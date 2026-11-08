# Byrgenwerth

Game support app for Windows and macOS: death count and real-time EN→VI dialogue translation. Shared logic in `engine.go`, `translate.go`, `stabilizer.go`; Windows UI in `ui_windows.go` (walk); macOS in `app_darwin.go`, `platform_darwin.m`, `webui/app.html`. Formerly Death Counter; the Mac translation prototype stays in the separate `byrgenwerth` repo.

Project info (status, decisions, research) lives in the Obsidian vault, not in this repo:
@/Volumes/KAFKA/Vault/10-projects/death-counter/_context.md

- Before writing to the vault, read `/Volumes/KAFKA/Vault/CLAUDE.md` (frontmatter, `_context.md`, `_system/agent-log.md`).
- User-facing docs: `README.md` (EN + VI), `docs/user-guide.md`, `docs/huong-dan-su-dung.md`. Keep all of them in sync with the code.
- Death-screen research behind the presets: `docs/death-screens-top100.csv`; presets in `presets.json`.
- Test: `go test ./...`. Build: `GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o Byrgenwerth.exe .` Mac: `scripts/build-mac.sh` (put `dist` on APFS: codesign fails on exFAT).
- Release: push a `v*` tag or run the release workflow from the Actions tab (`.github/workflows/release.yml`). The Mac build is signed with a self-signed certificate from the secrets `MAC_CERT_P12`/`MAC_CERT_PASSWORD` (made once by `scripts/make-signing-cert.sh`) so Screen Recording survives updates; a release without them fails. Do not regenerate the certificate unless it leaked: every Mac user would have to grant the permission again.
