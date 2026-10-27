// providers.go — cấu hình model đa provider (Claude / GPT / custom openai-compat).
// Engine dùng litellm: type "anthropic" được hỗ trợ NATIVE (providers/anthropic.go),
// nên kind=anthropic map thẳng type=anthropic trong engine-config.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type providerEntry struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"` // openai | anthropic | custom
	BaseURL string   `json:"base_url"`
	APIKey  string   `json:"api_key"`
	Models  []string `json:"models"`
}

type roleRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type modelConfigV2 struct {
	Version         int                `json:"version"` // 2
	Providers       []providerEntry    `json:"providers"`
	DefaultProvider string             `json:"default_provider"`
	Roles           map[string]roleRef `json:"roles"` // coordinator/architect/writer/editor
}

var engineRoles = []string{"coordinator", "architect", "writer", "editor"}

// legacy v1 (1 provider) — để migration
type modelConfigV1 struct {
	ProviderName string            `json:"provider_name"`
	BaseURL      string            `json:"base_url"`
	APIKey       string            `json:"api_key"`
	Models       []string          `json:"models"`
	Roles        map[string]string `json:"roles"`
}

// loadModelConfigV2 đọc config; tự migrate từ v1 hoặc từ engine-config.json.
func loadModelConfigV2() (modelConfigV2, bool) {
	var mc modelConfigV2
	raw, err := os.ReadFile(modelConfigPath())
	if err == nil {
		if json.Unmarshal(raw, &mc) == nil && mc.Version == 2 && len(mc.Providers) > 0 {
			return mc, true
		}
		// thử v1
		var v1 modelConfigV1
		if json.Unmarshal(raw, &v1) == nil && v1.ProviderName != "" {
			mc = migrateV1(v1)
			if err := saveModelConfigV2(mc); err != nil {
				log.Printf("migrate model-config v1→v2: %v", err)
			} else {
				log.Printf("đã migrate model-config.json v1 → v2 (provider %q)", v1.ProviderName)
			}
			return mc, true
		}
	}
	// fallback: dựng từ engine-config.json hiện có
	if mc2, ok := fromEngineConfig(); ok {
		return mc2, true
	}
	return modelConfigV2{Version: 2, Roles: map[string]roleRef{}}, false
}

func migrateV1(v1 modelConfigV1) modelConfigV2 {
	mc := modelConfigV2{
		Version: 2,
		Providers: []providerEntry{{
			Name: v1.ProviderName, Kind: "custom",
			BaseURL: v1.BaseURL, APIKey: v1.APIKey, Models: v1.Models,
		}},
		DefaultProvider: v1.ProviderName,
		Roles:           map[string]roleRef{},
	}
	for role, m := range v1.Roles {
		if m != "" {
			mc.Roles[role] = roleRef{Provider: v1.ProviderName, Model: m}
		}
	}
	return mc
}

// fromEngineConfig dựng modelConfigV2 từ engine-config.json (giữ nguyên dữ liệu cũ).
func fromEngineConfig() (modelConfigV2, bool) {
	raw, err := os.ReadFile(engineConfigPath())
	if err != nil {
		return modelConfigV2{}, false
	}
	var ec struct {
		Provider  string `json:"provider"`
		Providers map[string]struct {
			Type    string   `json:"type"`
			APIKey  string   `json:"api_key"`
			BaseURL string   `json:"base_url"`
			Models  []string `json:"models"`
		} `json:"providers"`
		Roles map[string]struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		} `json:"roles"`
	}
	if json.Unmarshal(raw, &ec) != nil || len(ec.Providers) == 0 {
		return modelConfigV2{}, false
	}
	mc := modelConfigV2{Version: 2, DefaultProvider: ec.Provider, Roles: map[string]roleRef{}}
	// giữ thứ tự ổn định: default trước, còn lại theo alphabet
	var names []string
	for n := range ec.Providers {
		if n != ec.Provider {
			names = append(names, n)
		}
	}
	sortStrings(names)
	if _, ok := ec.Providers[ec.Provider]; ok {
		names = append([]string{ec.Provider}, names...)
	}
	for _, n := range names {
		p := ec.Providers[n]
		kind := "custom"
		switch p.Type {
		case "openai":
			kind = "openai"
		case "anthropic":
			kind = "anthropic"
		}
		mc.Providers = append(mc.Providers, providerEntry{
			Name: n, Kind: kind, BaseURL: p.BaseURL, APIKey: p.APIKey, Models: p.Models,
		})
	}
	for role, rc := range ec.Roles {
		mc.Roles[role] = roleRef{Provider: rc.Provider, Model: rc.Model}
	}
	return mc, true
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func saveModelConfigV2(mc modelConfigV2) error {
	mc.Version = 2
	raw, _ := json.MarshalIndent(mc, "", "  ")
	if err := os.WriteFile(modelConfigPath(), raw, 0o600); err != nil {
		return err
	}
	return writeEngineConfigV2(mc)
}

// engineType map kind UI → type engine. Engine (litellm) hỗ trợ anthropic native.
func engineType(kind string) string {
	switch kind {
	case "anthropic":
		return "anthropic"
	default:
		return "openai"
	}
}

func (mc modelConfigV2) provider(name string) *providerEntry {
	for i := range mc.Providers {
		if mc.Providers[i].Name == name {
			return &mc.Providers[i]
		}
	}
	return nil
}

func (mc modelConfigV2) defaultEntry() *providerEntry {
	if p := mc.provider(mc.DefaultProvider); p != nil {
		return p
	}
	if len(mc.Providers) > 0 {
		return &mc.Providers[0]
	}
	return nil
}

// writeEngineConfigV2 sinh engine-config.json: providers{} đủ entry,
// roles có fallbacks = các provider còn lại theo thứ tự khai báo.
func writeEngineConfigV2(mc modelConfigV2) error {
	def := mc.defaultEntry()
	if def == nil {
		return fmt.Errorf("chưa có provider nào")
	}
	defModel := ""
	if len(def.Models) > 0 {
		defModel = def.Models[0]
	}
	provMap := map[string]any{}
	for _, p := range mc.Providers {
		provMap[p.Name] = map[string]any{
			"type":     engineType(p.Kind),
			"api_key":  p.APIKey,
			"base_url": p.BaseURL,
			"models":   p.Models,
		}
	}
	roles := map[string]any{}
	for _, role := range engineRoles {
		rr := mc.Roles[role]
		if rr.Provider == "" || mc.provider(rr.Provider) == nil {
			rr.Provider = def.Name
		}
		if rr.Model == "" {
			pe := mc.provider(rr.Provider)
			if pe != nil && len(pe.Models) > 0 {
				rr.Model = pe.Models[0]
			} else {
				rr.Model = defModel
			}
		}
		// fallbacks: các provider còn lại theo thứ tự khai báo
		var fallbacks []map[string]string
		for _, p := range mc.Providers {
			if p.Name == rr.Provider || len(p.Models) == 0 {
				continue
			}
			fallbacks = append(fallbacks, map[string]string{
				"provider": p.Name,
				"model":    p.Models[0],
			})
		}
		rm := map[string]any{"provider": rr.Provider, "model": rr.Model}
		if len(fallbacks) > 0 {
			rm["fallbacks"] = fallbacks
		}
		roles[role] = rm
	}
	cfg := map[string]any{
		"provider":       def.Name,
		"model":          defModel,
		"providers":      provMap,
		"roles":          roles,
		"style":          "default",
		"context_window": 200000,
	}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(engineConfigPath(), raw, 0o600)
}

// ---------- Settings HTTP (v2) ----------

type providerView struct {
	Name      string
	Kind      string
	BaseURL   string
	KeyMasked string
	Models    string
	IsDefault bool
}

func handleSettingsPageV2(w http.ResponseWriter, r *http.Request) {
	mc, _ := loadModelConfigV2()
	var pv []providerView
	for _, p := range mc.Providers {
		pv = append(pv, providerView{
			Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL,
			KeyMasked: maskKey(p.APIKey), Models: strings.Join(p.Models, "\n"),
			IsDefault: p.Name == mc.DefaultProvider,
		})
	}
	if mc.Roles == nil {
		mc.Roles = map[string]roleRef{}
	}
	data := struct {
		Providers []providerView
		Default   string
		Roles     map[string]roleRef
		Flash     flashData
	}{pv, mc.DefaultProvider, mc.Roles, flashData{Msg: r.URL.Query().Get("ok"), Err: r.URL.Query().Get("err")}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Printf("template settings: %v", err)
	}
}

func validKind(k string) bool { return k == "openai" || k == "anthropic" || k == "custom" }

// handleSettingsSaveV2 xử lý: action=save_provider (thêm/sửa), delete_provider, save_roles.
func handleSettingsSaveV2(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?err="+urlQ("form không hợp lệ"), http.StatusSeeOther)
		return
	}
	mc, _ := loadModelConfigV2()
	if mc.Roles == nil {
		mc.Roles = map[string]roleRef{}
	}
	action := r.FormValue("action")
	switch action {
	case "delete_provider":
		name := strings.TrimSpace(r.FormValue("name"))
		var kept []providerEntry
		for _, p := range mc.Providers {
			if p.Name != name {
				kept = append(kept, p)
			}
		}
		if len(kept) == len(mc.Providers) {
			http.Redirect(w, r, "/settings?err="+urlQ("không tìm thấy provider"), http.StatusSeeOther)
			return
		}
		mc.Providers = kept
		if mc.DefaultProvider == name {
			mc.DefaultProvider = ""
			if len(kept) > 0 {
				mc.DefaultProvider = kept[0].Name
			}
		}
		for role, rr := range mc.Roles {
			if rr.Provider == name {
				delete(mc.Roles, role)
			}
		}
		if len(mc.Providers) == 0 {
			// không còn provider — vẫn lưu file v2, engine-config giữ nguyên bản cũ
			raw, _ := json.MarshalIndent(mc, "", "  ")
			_ = os.WriteFile(modelConfigPath(), raw, 0o600)
			http.Redirect(w, r, "/settings?ok="+urlQ("đã xóa provider cuối — thêm provider mới để dùng"), http.StatusSeeOther)
			return
		}
	case "save_provider":
		name := strings.TrimSpace(r.FormValue("name"))
		orig := strings.TrimSpace(r.FormValue("orig_name")) // rename support
		kind := r.FormValue("kind")
		baseURL := strings.TrimSpace(r.FormValue("base_url"))
		key := strings.TrimSpace(r.FormValue("api_key"))
		var models []string
		for _, l := range strings.Split(r.FormValue("models"), "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				models = append(models, l)
			}
		}
		if name == "" || !validKind(kind) || baseURL == "" || len(models) == 0 {
			http.Redirect(w, r, "/settings?err="+urlQ("thiếu tên/kind/base_url/models"), http.StatusSeeOther)
			return
		}
		if len(name) > 40 || !slugRe.MatchString(name) {
			http.Redirect(w, r, "/settings?err="+urlQ("tên provider chỉ gồm chữ-số-gạch, tối đa 40 ký tự"), http.StatusSeeOther)
			return
		}
		lookup := orig
		if lookup == "" {
			lookup = name
		}
		existing := mc.provider(lookup)
		if key == "" || strings.HasPrefix(key, "••") {
			if existing != nil {
				key = existing.APIKey
			}
		}
		if key == "" {
			http.Redirect(w, r, "/settings?err="+urlQ("thiếu API key"), http.StatusSeeOther)
			return
		}
		entry := providerEntry{Name: name, Kind: kind, BaseURL: baseURL, APIKey: key, Models: models}
		if existing != nil {
			// đổi tên: cập nhật roles + default
			if lookup != name {
				if mc.provider(name) != nil {
					http.Redirect(w, r, "/settings?err="+urlQ("tên mới đã tồn tại"), http.StatusSeeOther)
					return
				}
				for role, rr := range mc.Roles {
					if rr.Provider == lookup {
						rr.Provider = name
						mc.Roles[role] = rr
					}
				}
				if mc.DefaultProvider == lookup {
					mc.DefaultProvider = name
				}
			}
			*existing = entry
			// khi rename, existing trỏ đúng phần tử trong slice nên gán trực tiếp OK
		} else {
			if mc.provider(name) != nil {
				http.Redirect(w, r, "/settings?err="+urlQ("provider đã tồn tại"), http.StatusSeeOther)
				return
			}
			mc.Providers = append(mc.Providers, entry)
		}
		if mc.DefaultProvider == "" {
			mc.DefaultProvider = name
		}
		if r.FormValue("make_default") == "on" {
			mc.DefaultProvider = name
		}
	case "save_roles":
		def := strings.TrimSpace(r.FormValue("default_provider"))
		if def != "" && mc.provider(def) != nil {
			mc.DefaultProvider = def
		}
		for _, role := range engineRoles {
			p := strings.TrimSpace(r.FormValue("role_" + role + "_provider"))
			m := strings.TrimSpace(r.FormValue("role_" + role + "_model"))
			if p == "" && m == "" {
				delete(mc.Roles, role)
				continue
			}
			if p != "" && mc.provider(p) == nil {
				http.Redirect(w, r, "/settings?err="+urlQ("role "+role+": provider không tồn tại"), http.StatusSeeOther)
				return
			}
			mc.Roles[role] = roleRef{Provider: p, Model: m}
		}
	default:
		http.Redirect(w, r, "/settings?err="+urlQ("action không hợp lệ"), http.StatusSeeOther)
		return
	}
	if err := saveModelConfigV2(mc); err != nil {
		http.Redirect(w, r, "/settings?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?ok="+urlQ("đã lưu cấu hình + sinh engine-config.json"), http.StatusSeeOther)
}

// handleSettingsTestV2 test kết nối 1 provider:
// anthropic → GET {base}/v1/models với x-api-key; openai/custom → GET {base}/models Bearer.
func handleSettingsTestV2(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, 400, map[string]string{"error": "form không hợp lệ"})
		return
	}
	kind := r.FormValue("kind")
	if !validKind(kind) {
		kind = "custom"
	}
	baseURL := strings.TrimSpace(r.FormValue("base_url"))
	key := strings.TrimSpace(r.FormValue("api_key"))
	if key == "" || strings.HasPrefix(key, "••") {
		// lấy key đã lưu theo tên provider
		mc, _ := loadModelConfigV2()
		if p := mc.provider(strings.TrimSpace(r.FormValue("name"))); p != nil {
			key = p.APIKey
			if baseURL == "" {
				baseURL = p.BaseURL
			}
		}
	}
	if baseURL == "" {
		writeJSON(w, 400, map[string]string{"error": "thiếu base_url"})
		return
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		writeJSON(w, 400, map[string]string{"error": "base_url phải là http(s) URL"})
		return
	}
	base := strings.TrimRight(baseURL, "/")
	var url string
	if kind == "anthropic" {
		url = base + "/v1/models"
		if strings.HasSuffix(base, "/v1") {
			url = base + "/models"
		}
	} else {
		url = base + "/models"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if kind == "anthropic" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": "không kết nối được: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != 200 {
		writeJSON(w, 200, map[string]any{"ok": false, "error": fmt.Sprintf("HTTP %d", resp.StatusCode)})
		return
	}
	var mr struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	n := 0
	if json.Unmarshal(body, &mr) == nil {
		n = len(mr.Data)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "models": n})
}
