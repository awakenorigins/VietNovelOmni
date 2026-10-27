#!/usr/bin/env bash
# OmniNovel post-commit hook orchestrator.
# Engine gọi script này SAU KHI commit mỗi chương thành công.
# Chạy TẤT CẢ skill verify của bộ OmniNovel trên chương vừa commit, gom kết quả thành 1 JSON
# (meta/skill-audit/ch{N}.json). Best-effort: không chặn commit, exit luôn 0.
#
# Env từ engine: OMNI_CHAPTER, OMNI_CHAPTER_FILE, OMNI_OUTPUT_DIR, OMNI_SKILLS_DIR

set -uo pipefail

HD="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export OMNI_HOOK_DIR="$HD"

# Toàn bộ logic + sinh JSON nằm trong Python để bảo đảm JSON hợp lệ tuyệt đối.
python3 - <<'PY'
import os, json, subprocess, shutil, datetime, glob

CH       = os.environ.get("OMNI_CHAPTER", "0")
CHFILE   = os.environ.get("OMNI_CHAPTER_FILE", "")
OUTDIR   = os.environ.get("OMNI_OUTPUT_DIR", "output/novel")
HOOKDIR  = os.environ.get("OMNI_HOOK_DIR", ".")
SKILLS   = os.environ.get("OMNI_SKILLS_DIR", "")

# Suy ra skills dir nếu trống
if not SKILLS:
    for c in [os.path.join(HOOKDIR,"..","skills"), "/opt/omni/skills",
              "/workspace/skills", "/root/.openclaw/workspace/skills"]:
        if os.path.isdir(c): SKILLS = os.path.abspath(c); break

try: chn = int(CH)
except: chn = 0

audit_dir = os.path.join(OUTDIR, "meta", "skill-audit")
os.makedirs(audit_dir, exist_ok=True)
report_path = os.path.join(audit_dir, "ch%02d.json" % chn)

def run(cmd, cwd=None, timeout=90):
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=timeout)
        return {"exit": p.returncode, "output": (p.stdout + p.stderr).strip()[:8000]}
    except Exception as e:
        return {"exit": -1, "output": "hook-error: %s" % e}

results = {}
pass_all = True

have_node = shutil.which("node") is not None
have_chfile = bool(CHFILE) and os.path.isfile(CHFILE)

# --- SKILL 1: oum-prose-verify (CHÍNH XÁC — blacklist tiếng Anh) ---
opv = None
for c in [os.path.join(HOOKDIR,"oum-prose-verify.py"),
          os.path.join(HOOKDIR,"..","scripts","oum-prose-verify.py"),
          os.path.join(SKILLS,"oumstudio-novel","scripts","oum-prose-verify.py")]:
    if os.path.isfile(c): opv = c; break
if opv and have_chfile:
    r = run(["python3", opv, "--json", CHFILE])
    # parse JSON output nếu được
    parsed = None
    try: parsed = json.loads(r["output"])
    except: pass
    results["oum_prose_verify"] = {"available": True, "exit": r["exit"],
                                   "parsed": parsed, "output": None if parsed else r["output"]}
    # hard errors thật sự (em dash / tiếng Anh) → fail; markdown không tính
    if parsed and parsed.get("hard_errors"):
        real = [e for e in parsed["hard_errors"] if not e.startswith("markdown")]
        if real: pass_all = False
else:
    results["oum_prose_verify"] = {"available": False}

# --- SKILL 2: forge-novel-guard verify-prose (CHỈ tham khảo em dash/markdown/AI) ---
fng = os.path.join(SKILLS, "forge-novel-guard", "scripts", "verify-prose.py")
if os.path.isfile(fng) and have_chfile:
    r = run(["python3", fng, CHFILE])
    results["forge_novel_guard_prose"] = {"available": True, "exit": r["exit"],
        "note": "english-detector co the bao nham tieng Viet khong dau; chi tham khao em dash/markdown/AI",
        "output": r["output"]}
else:
    results["forge_novel_guard_prose"] = {"available": False}

# --- SKILL 3: novel-guardian scan (continuity 10 rule) ---
ng = os.path.join(SKILLS, "novel-guardian", "scripts", "novel-guardian.mjs")
if os.path.isfile(ng) and have_node:
    gdir = os.path.join(OUTDIR, "guardian")
    os.makedirs(os.path.join(gdir, "chapters"), exist_ok=True)
    # novel-guardian scanner nhận tên file khớp /(ch|chuong|chương)[._-]?(\d+)/i
    # → đổi tên chapters/NN.md → chNN.md khi copy
    for f in glob.glob(os.path.join(OUTDIR, "chapters", "*.md")):
        base = os.path.basename(f)
        num = "".join(ch for ch in base if ch.isdigit()) or "0"
        dest = os.path.join(gdir, "chapters", "ch%02d.md" % int(num))
        try: shutil.copy(f, dest)
        except: pass
    if not os.path.isdir(os.path.join(gdir, "data")):
        run(["node", ng, "init"], cwd=gdir)
    r = run(["node", ng, "scan", "--chapter", str(chn)], cwd=gdir)
    results["novel_guardian_scan"] = {"available": True, "exit": r["exit"], "output": r["output"]}
else:
    results["novel_guardian_scan"] = {"available": False}

# --- SKILL 4: novel-master check (5 lớp) ---
nm = os.path.join(SKILLS, "novel-master", "scripts", "novel-master.mjs")
if os.path.isfile(nm) and have_node and have_chfile:
    # novel-master cần tên file suy ra được số chương (chNN.md). Tạo bản sao chNN.md tạm.
    nmtmp = os.path.join(OUTDIR, "meta", "nm-tmp")
    os.makedirs(nmtmp, exist_ok=True)
    nmfile = os.path.join(nmtmp, "ch%02d.md" % chn)
    try: shutil.copy(CHFILE, nmfile)
    except: nmfile = CHFILE
    # novel-master: check scan <chapter.md> (positional)
    r = run(["node", nm, "check", "scan", nmfile, "--project", "omni"])
    results["novel_master_check"] = {"available": True, "exit": r["exit"], "output": r["output"]}
else:
    results["novel_master_check"] = {"available": False}

# --- SKILL 5..11: bộ script advisory (KHÔNG ảnh hưởng pass_all) ---
def run_py_skill(key, script_rel, args):
    """Chạy 1 script python skill, parse JSON output. Advisory-only."""
    sp = os.path.join(SKILLS, *script_rel)
    if not os.path.isfile(sp):
        results[key] = {"available": False}
        return
    r = run(["python3", sp, "--json"] + args)
    parsed = None
    try: parsed = json.loads(r["output"])
    except: pass
    results[key] = {"available": True, "exit": r["exit"], "advisory": True,
                    "parsed": parsed, "output": None if parsed else r["output"]}

# --- SKILL 5: novelcore-ai check (blueprint + AI-sáo) ---
if have_chfile:
    run_py_skill("novelcore_check", ["novelcore-ai","scripts","novelcore-check.py"], [CHFILE])
else:
    results["novelcore_check"] = {"available": False}

# --- SKILL 6: cw-prose-writing metrics (văn phong) ---
if have_chfile:
    run_py_skill("cw_prose_metrics", ["cw-prose-writing","scripts","prose-metrics.py"], [CHFILE])
else:
    results["cw_prose_metrics"] = {"available": False}

# --- SKILL 7: cw-story-critique heuristics (hook/mở chương/telling) ---
if have_chfile:
    run_py_skill("cw_story_critique", ["cw-story-critique","scripts","critique-heuristics.py"], [CHFILE])
else:
    results["cw_story_critique"] = {"available": False}

# --- SKILL 8: cw-brainstorming tbd-scan (placeholder sót) ---
if have_chfile:
    run_py_skill("cw_tbd_scan", ["cw-brainstorming","scripts","tbd-scan.py"], [CHFILE])
else:
    results["cw_tbd_scan"] = {"available": False}

# --- SKILL 9: cw-style-skill-creator fingerprint (+ drift vs chương trước) ---
if have_chfile:
    fp_dir = os.path.join(OUTDIR, "meta", "style-fingerprint")
    os.makedirs(fp_dir, exist_ok=True)
    prev_fp = os.path.join(fp_dir, "ch%02d.json" % (chn - 1))
    fp_args = [CHFILE] + (["--baseline", prev_fp] if os.path.isfile(prev_fp) else [])
    run_py_skill("cw_style_fingerprint", ["cw-style-skill-creator","scripts","style-fingerprint.py"], fp_args)
    # lưu fingerprint chương này làm baseline cho chương sau
    fpres = results.get("cw_style_fingerprint", {})
    if fpres.get("parsed"):
        try:
            with open(os.path.join(fp_dir, "ch%02d.json" % chn), "w", encoding="utf-8") as f:
                json.dump(fpres["parsed"], f, ensure_ascii=False, indent=2)
        except: pass
else:
    results["cw_style_fingerprint"] = {"available": False}

# --- SKILL 10: cw-official-docs wiki-extract (ứng viên tên riêng) ---
if have_chfile:
    run_py_skill("cw_wiki_extract", ["cw-official-docs","scripts","wiki-extract.py"], [CHFILE])
else:
    results["cw_wiki_extract"] = {"available": False}

# --- SKILL 11: cw-router route-check (self-test router) ---
if have_chfile:
    outline = os.path.join(OUTDIR, "outline.md")
    run_py_skill("cw_route_check", ["cw-router","scripts","route-check.py"],
                 ["--chapter", str(chn), "--file", CHFILE,
                  "--has-outline", "yes" if os.path.isfile(outline) else "no"])
else:
    results["cw_route_check"] = {"available": False}

report = {
    "chapter": chn,
    "chapter_file": CHFILE,
    "audited_at": datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
    "skills_dir": SKILLS,
    "results": results,
    "pass_all": pass_all,
}
with open(report_path, "w", encoding="utf-8") as f:
    json.dump(report, f, ensure_ascii=False, indent=2)

import sys
print("[omni-hook] chương %d audited → %s (pass_all=%s)" % (chn, report_path, pass_all), file=sys.stderr)
PY
exit 0
