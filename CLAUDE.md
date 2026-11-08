# Byrgenwerth

Game support app for Windows: death count and real-time EN→VI dialogue translation with Vietnamese narration. Logic in `engine.go`, `core.go`, `label.go`, `translate.go`, `stabilizer.go`, `piper.go`; UI in `ui_windows.go` (walk), with `display_windows.go` (label), `translate_windows.go` and `narration_windows.go`. Formerly Death Counter. The macOS version, the per-game presets and the health-bar mode were removed in v0.12.0.

Project info (status, decisions, research) lives in the Obsidian vault, not in this repo:
@/Volumes/KAFKA/Vault/10-projects/death-counter/_context.md

- Before writing to the vault, read `/Volumes/KAFKA/Vault/CLAUDE.md` (frontmatter, `_context.md`, `_system/agent-log.md`).
- User-facing docs: `README.md` (EN + VI), `docs/user-guide.md`, `docs/huong-dan-su-dung.md`. Keep all of them in sync with the code.
- UI layout: every input and button goes through `row()`/`sized()` in `ui_windows.go`, which give them the shared widths (`inputW`, `buttonW`). Do not force heights: walk sizes rows from native heights and cuts a taller button.
- Narration downloads Piper and two voices at runtime; their URLs and SHA-256 hashes are pinned in `piper.go`. The man's voice is speaker 45 of `vivos`, picked by pitch: Piper has no single-speaker Vietnamese man.
- Test: `go test ./...`. Live checks: `BYR_PIPER=<folder> go test -run TestPiperLive` (downloads ~110 MB). Build: `GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o Byrgenwerth.exe .`
- Release: push a `v*` tag or run the release workflow from the Actions tab (`.github/workflows/release.yml`).
