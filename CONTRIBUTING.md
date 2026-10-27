# Contributing to OUMStudio-Novel

Cảm ơn bạn quan tâm. Tài liệu này mô tả quy ước đóng góp cho Omni Novel Suite.

## Môi trường
- Go ≥ 1.25 (engine)
- Node.js (novel-guardian / novel-master)
- Python 3 (verify scripts)

## Build & test
```bash
go build -o bin/oum-novel ./cmd/ainovel-cli/
go test ./...
```
Mọi PR phải giữ `go test ./...` xanh (hiện 19 package pass).

## Cấu trúc
- Engine: `cmd/` + `internal/`
- Assets embed: `assets/`
- Lớp prose Việt: `oumstudio/`
- Skill quy trình: `skills/` (mỗi skill có `SKILL.md` riêng)

## Quy ước prose tiếng Việt (bắt buộc)
- 0 em dash (—), 0 tiếng Anh lọt prose, 0 markdown trong văn truyện.
- Tôn trọng 30+ rule continuity trong `assets/references/continuity-rules-vi.md`.
```bash
python3 oumstudio/oum-prose-verify.py --strict <file>
```

## Pull request
1. Fork + branch `feature/...` hoặc `fix/...`.
2. Commit nhỏ, message rõ.
3. Test xanh.
4. Mở PR mô tả thay đổi + lý do.

## License
Đóng góp được phát hành dưới [Apache License 2.0](LICENSE). Sub-component `skills/novel-guardian/` giữ MIT riêng.
