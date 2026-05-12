# Raven Local — Release QA Checklist

Manual pre-release validation for every `raven-local-v*` tag. Used until [#515 (Playwright e2e)](https://github.com/ravencloak-org/Raven/issues/515) and [#516 (Installer smoke)](https://github.com/ravencloak-org/Raven/issues/516) land.

Each release PR links here. The tester(s) tick every applicable box on every platform and fill in the sign-off table at the bottom before the draft Release is promoted to published.

---

## 1. Per-platform install + uninstall

### macOS (arm64 + x86_64 universal)

- [ ] Download `Raven Local_<version>_universal.dmg` from the draft release
- [ ] Double-click → DMG mounts without errors
- [ ] Drag `Raven Local.app` to `/Applications`
- [ ] Eject the DMG
- [ ] First launch from `/Applications` triggers Gatekeeper warning (unsigned in M11; expected until #426)
- [ ] Right-click → Open → "Open anyway" launches the app
- [ ] App appears in the menu bar (tray icon) and as a dock icon
- [ ] **Uninstall:** quit the app, drag `Raven Local.app` from `/Applications` to the Trash, confirm no leftover background processes (`pgrep -fl raven-local`)

### Windows (x64)

- [ ] Download `Raven_Local_<version>_x64_en-US.msi` from the draft release
- [ ] Double-click → SmartScreen warning (unsigned in M11; expected until #426)
- [ ] "More info" → "Run anyway" → MSI installer dialog appears
- [ ] Install completes without errors; entry added to Start Menu
- [ ] Launch from Start Menu → main window opens
- [ ] App appears in the system tray
- [ ] **Uninstall:** Settings → Apps → "Raven Local" → Uninstall → completes without leftover services or scheduled tasks

### Linux (x86_64)

- [ ] Download `Raven_Local_<version>_amd64.AppImage`
- [ ] `chmod +x Raven_Local_*.AppImage`
- [ ] `./Raven_Local_<version>_amd64.AppImage` → main window opens
- [ ] App appears in the system indicator area (varies by DE)
- [ ] **AppImage uninstall:** delete the file
- [ ] **Alternative .deb:** `sudo dpkg -i raven-local_<version>_amd64.deb` → entry in app launcher → launch succeeds
- [ ] **.deb uninstall:** `sudo apt remove raven-local`

## 2. System-requirements precheck

- [ ] On a host with ≥ 12 GB RAM and ≥ 20 GB free disk: app launches without warning
- [ ] On a host with < 8 GB RAM (use a VM with 4 GB): launch shows the precheck warning modal with actionable text
- [ ] `RAVEN_LOCAL_SKIP_REQS=true ./Raven_Local_*.AppImage` (or env-set equivalent on macOS/Windows): warning still emitted, app proceeds anyway

## 3. First-run wizard

- [ ] Compose comes up (splash visible) → first launch routes to `/onboarding` automatically
- [ ] **Model picker** shows three Ollama options; default pre-selection matches host RAM tier:
  - 8 GB → `llama3.2:3b` pre-selected
  - 12–15 GB → `llama3.1:8b` pre-selected
  - 16 GB+ → `llama3.1:8b` pre-selected, `llama3.1:13b` also visible
- [ ] Clicking "Download model" streams progress; bar reaches 100%
- [ ] **Optional BYOK** step appears; "Skip" advances without an API call
- [ ] **Workspace name** step defaults to "Personal"; "Get Started" routes to `/dashboard`
- [ ] **Relaunch the app:** onboarding is skipped; goes straight to `/dashboard`

## 4. Post-install smoke

### Chat round-trip

- [ ] On `/dashboard`, open the chat / sandbox interface
- [ ] Send a simple prompt (e.g. "Say hello in one sentence.")
- [ ] Streaming response appears within 10s and is non-empty
- [ ] No errors in the developer console or compose logs

### Knowledge-base upload

- [ ] Create a new knowledge base (any name)
- [ ] Upload a small PDF (≤ 5 MB)
- [ ] Document appears in the KB's documents list with status `processed` within 60s
- [ ] Send a chat message that references the document; the assistant cites it

### Settings tabs

- [ ] Visit `/settings` → all four tabs render (General / Models / API Keys / Privacy)
- [ ] **General:** rename the workspace → "Saved" toast appears; reload → new name persists
- [ ] **Models:** the pulled model from the wizard is listed
- [ ] **API Keys:** "Manage providers" link routes to `/llm-providers`
- [ ] **Privacy:** telemetry toggle persists across reload

### Tray / menubar

- [ ] Tray icon present on launch
- [ ] Tooltip shows `Raven Local — Ready` once compose is healthy
- [ ] Tray menu: Open Raven, Pause, Resume, Quit
- [ ] **Pause:** containers stop (verify via `docker ps`); tooltip updates to `Paused`
- [ ] **Resume:** containers restart; tooltip returns to `Ready`
- [ ] Left-click the tray icon toggles the main window visibility

## 5. Quit + relaunch

- [ ] Quit via tray menu → containers come down cleanly (no zombie containers in `docker ps -a` after 10s)
- [ ] Force-quit the app process (Activity Monitor / Task Manager / `kill`)
- [ ] Relaunch → orphaned containers are reaped or replaced; app reaches Ready
- [ ] Second relaunch (clean exit) → wizard not shown; dashboard loads with previous workspace + KB intact

## 6. Tear-down

- [ ] Uninstall per Section 1 for the platform being tested
- [ ] Verify no leftover docker volumes if that's the intended behaviour (or document that volumes persist by design)

---

## Sign-off

| Tester | Platform | Build | Date | Result | Notes |
|--------|----------|-------|------|--------|-------|
|        |          |       |      |        |       |
|        |          |       |      |        |       |
|        |          |       |      |        |       |

Promote the draft Release to published once **all three platforms have a "Pass" row**. Any "Fail" requires either a fix-and-retag or a documented known-issue called out in the release notes.

---

## Known limitations until automation lands

- Manual GUI clicking is error-prone; expect 30–60 min per platform per release.
- Gatekeeper / SmartScreen warnings are part of the experience until #426 ships signing.
- The first-run wizard's model download depends on Ollama Hub availability; if the registry is down, the wizard fails — that's an upstream issue, not a regression.

See also: [Raven-Local architecture](../wiki/Raven-Local.md), [milestone M12](https://github.com/ravencloak-org/Raven/milestone/15).
