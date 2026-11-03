# Death Counter

Project info (status, decisions, research) lives in the Obsidian vault, not in this repo:
@/Volumes/KAFKA/Vault/10-projects/death-counter/_context.md

- Before writing to the vault, read `/Volumes/KAFKA/Vault/CLAUDE.md` (frontmatter, `_context.md`, `_system/agent-log.md`).
- User-facing docs: `README.md` (EN + VI), `docs/user-guide.md`, `docs/huong-dan-su-dung.md`. Keep all of them in sync with the code.
- Death-screen research behind the presets: `docs/death-screens-top100.csv`; presets in `presets.json`.
- Test: `go test ./...`. Build: `GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o DeathCounter.exe .` The app only runs on Windows.
- Release: push a `v*` tag or run the release workflow from the Actions tab (`.github/workflows/release.yml`).
