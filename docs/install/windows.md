# Install Raven AI on Windows

Raven AI ships as a `.msi` for x64 Windows. Microsoft's SmartScreen Defender flags new MSIs from unverified publishers — the binary isn't signed with a Microsoft-trusted certificate yet, so first launch shows the *"Windows protected your PC"* dialog.

We're applying for **free OSS code-signing via [SignPath.io](https://signpath.io/open-source)**, which provides Microsoft-trusted certs at no cost for open-source projects. Once that's approved (typically a few days), the release CI signs every MSI and SmartScreen stops complaining. Tracked as [#426](https://github.com/ravencloak-org/Raven/issues/426).

## Install path (while signing is pending)

1. Download the `.msi` from the [latest release](https://github.com/ravencloak-org/Raven/releases?q=raven-ai).
2. Double-click. The SmartScreen dialog appears: *"Windows protected your PC"*.
3. Click **More info**.
4. A **Run anyway** button appears. Click it.
5. The MSI installer dialog opens. Click through → Install → Finish.
6. Launch *Raven AI* from the Start Menu.

Windows remembers the decision; subsequent launches don't prompt.

## After signing lands

Once the SignPath certificate is in place, the installer is signed at build time and the SmartScreen warning never appears. No user action required; just download and install normally.

## Uninstall

**Settings → Apps → Installed apps → Raven AI → Uninstall.**

The uninstall removes the program files and Start Menu shortcut but **leaves docker volumes intact** so your workspace data survives. If you want a full wipe, also run:

```powershell
docker volume rm $(docker volume ls -q --filter "label=com.docker.compose.project=desktop")
```

## Troubleshooting

- **MSI fails with error 1603** — Windows Installer service may be stuck; try **Services → Windows Installer → Restart**, then retry.
- **App launches but window is blank** — WebView2 runtime missing (rare on Win 11; common on stripped Win 10). Install [Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (free Microsoft installer).
- **App opens but compose doesn't come up** — Docker Desktop for Windows isn't running. Start it from the Start Menu, then re-launch Raven AI.
