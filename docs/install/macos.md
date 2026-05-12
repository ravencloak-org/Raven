# Install Raven Local on macOS

Raven Local ships as a universal `.dmg` (Apple Silicon + Intel). Apple Developer Program membership ($99/year) is required to fully notarise a desktop app, and Raven Local — being free OSS — doesn't pay it. The DMG is **ad-hoc signed but not Apple-notarised**, so first-time macOS launch shows Gatekeeper's *"Apple could not verify…"* warning.

There are three install paths. Pick whichever matches your setup; **Homebrew is the cleanest** because it strips macOS's quarantine attribute automatically and you never see the warning.

## 1. Homebrew (recommended)

One-time tap setup:

```bash
brew tap ravencloak-org/Raven https://github.com/ravencloak-org/Raven.git
```

Install / upgrade:

```bash
brew install --cask raven-local
# later:
brew upgrade --cask raven-local
```

Uninstall:

```bash
brew uninstall --cask raven-local
brew untap ravencloak-org/Raven
```

`brew --cask` calls `xattr -d com.apple.quarantine` on the installed `.app` before launching, so Gatekeeper doesn't kick in.

## 2. Manual DMG, with quarantine stripped

Download the `.dmg` from the [latest release](https://github.com/ravencloak-org/Raven/releases?q=raven-local), mount it, drag `Raven Local.app` to `/Applications`, then run:

```bash
xattr -d com.apple.quarantine "/Applications/Raven Local.app"
open -a "Raven Local"
```

The `xattr` line removes the "downloaded from the internet" flag that triggers Gatekeeper. After this the app launches without the warning.

## 3. Manual DMG, click through the warning once

If you don't want to touch a terminal:

1. Download and mount the `.dmg`, drag the app to `/Applications`.
2. Double-click `Raven Local.app` → see the *"Apple could not verify…"* dialog → click **Cancel**.
3. Open **System Settings → Privacy & Security**. Near the bottom you'll see a line saying *"'Raven Local.app' was blocked…"* with an **Open Anyway** button. Click it.
4. Re-launch the app. A second dialog appears asking again — this time it has an **Open** button. Click **Open**.
5. macOS remembers the decision; subsequent launches don't prompt.

(Alternative: in Finder, **right-click → Open**, then click **Open** in the dialog. The right-click path surfaces a different dialog with a working Open button; double-click only shows the dead-end warning.)

## Why this is the situation

- Apple's notarisation service is gated on Apple Developer Program membership ($99/year). Raven Local would need to either pay it or rely on the workarounds above.
- Tracked as [#426](https://github.com/ravencloak-org/Raven/issues/426); the moment we have a Developer ID, the release CI signs + notarises and this warning disappears entirely.
- Until then, Homebrew is the cleanest no-friction path.

## Troubleshooting

- **Cannot remove quarantine attribute** — re-run `xattr -d com.apple.quarantine` with `sudo` if your user doesn't own `/Applications/Raven Local.app`.
- **Brew install fails with "sha256 mismatch"** — the Cask hash is bumped by an automated PR after each release; if a release was just published, give it a few minutes for the bump PR to merge, or fall back to the manual DMG path.
- **App opens but compose doesn't come up** — Docker Desktop isn't running. The app's startup splash shows a *"Docker daemon is not running"* message if so.
