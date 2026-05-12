# Install Raven AI on Linux

Linux has no centralised app-signing requirement, so installs are friction-free. Two artefacts ship per release: a portable `.AppImage` (works on any glibc-based distro) and a `.deb` for Debian/Ubuntu.

## AppImage (any distro)

```bash
APPIMAGE_URL=$(curl -fsSL https://api.github.com/repos/ravencloak-org/Raven/releases/latest \
  | grep -oE '"browser_download_url": "[^"]*\.AppImage"' \
  | head -1 \
  | cut -d'"' -f4)
curl -fsSL -o raven-ai.AppImage "$APPIMAGE_URL"
chmod +x raven-ai.AppImage
./raven-ai.AppImage
```

Move it somewhere on `PATH` (e.g. `~/.local/bin/`) if you want a stable name. The AppImage is self-contained — no install step, no package manager.

## .deb (Debian / Ubuntu)

```bash
DEB_URL=$(curl -fsSL https://api.github.com/repos/ravencloak-org/Raven/releases/latest \
  | grep -oE '"browser_download_url": "[^"]*\.deb"' \
  | head -1 \
  | cut -d'"' -f4)
curl -fsSL -o raven-ai.deb "$DEB_URL"
sudo apt install -y ./raven-ai.deb
```

The `.deb` registers an `.desktop` file, an icon, and the `raven-ai` binary on `PATH`. Launch from your DE's app launcher or from a terminal.

## Runtime dependencies

The AppImage bundles everything; the `.deb` declares them in its `Depends:`. Both need:

- `libwebkit2gtk-4.1-0` (Tauri WebView)
- `libgtk-3-0`
- `libayatana-appindicator3-1` (for the tray icon)
- `librsvg2-2`
- Docker Engine (or Docker Desktop for Linux) — for the bundled compose stack

On Wayland sessions, AppImage may fall back to XWayland — that's expected and works.

## Uninstall

- **AppImage**: delete the file. `rm -rf ~/.config/io.ravencloak.ai ~/.local/share/io.ravencloak.ai` to also wipe app data.
- **.deb**: `sudo apt remove raven-ai`. Same `rm` commands to wipe user data.

## Why Linux is the easy path

No centralised signing authority, no quarantine attributes, no SmartScreen equivalent. The same `.AppImage` runs unchanged on Ubuntu, Fedora, Arch, Debian — distro divergence happens at the package-format level, not the trust-and-install level.

## Troubleshooting

- **`./raven-ai.AppImage` fails with "FUSE not available"** — install `fuse` (Ubuntu 22.04+ defaults to FUSE 3 which needs `libfuse2` explicitly). On Ubuntu 24.04: `sudo apt install libfuse2`. Or extract via `./raven-ai.AppImage --appimage-extract` and run `./squashfs-root/AppRun`.
- **Tray icon missing on GNOME** — install the AppIndicator extension. KDE / XFCE / Cinnamon support it natively.
- **App opens but compose doesn't come up** — Docker daemon isn't running: `systemctl --user start docker-desktop` (Docker Desktop) or `sudo systemctl start docker` (Docker Engine).
