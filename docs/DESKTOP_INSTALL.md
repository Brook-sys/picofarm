# Install PicoFarm desktop

Download a package from the GitHub Releases page for your OS and architecture.
Desktop builds are beta software. Back up valuable data before upgrading.

| OS | Asset suffix | Instructions |
| --- | --- | --- |
| Windows x64 | `windows-amd64.zip` | Extract the entire folder, install Microsoft WebView2 Evergreen Runtime if missing, run `PicoFarm.exe`. |
| Linux x64 | `linux-amd64.tar.gz` | Extract and run `PicoFarm` from a graphical desktop. Ubuntu 24.04-compatible libraries required (see below). |
| macOS Intel | `darwin-amd64.zip` | Extract, move `PicoFarm.app` to Applications, then open it. |
| macOS Apple Silicon | `darwin-arm64.zip` | Extract, move `PicoFarm.app` to Applications, then open it. |

Windows binaries are unsigned. macOS apps are ad-hoc signed, not notarized;
SmartScreen/Gatekeeper may block them. Review the source and publisher before
using the operating system's explicit approval mechanism. Never disable system
security globally to run this app.

Linux is dynamically linked, not a static binary or AppImage. On Ubuntu 24.04:

```sh
sudo apt install libgtk-4-1 libwebkitgtk-6.0-4 libsoup-3.0-0 libx11-6
```

macOS packaging declares macOS 12 as the minimum; actual GUI behavior on older
supported systems has not been verified. Build success is not a runtime test.

## Verify the download

Download `SHA256SUMS.txt` alongside the archive and compare its SHA-256 digest:

```sh
# Linux
sha256sum YOUR_ARCHIVE
# macOS
shasum -a 256 YOUR_ARCHIVE
```

```powershell
# Windows PowerShell
Get-FileHash .\YOUR_ARCHIVE.zip -Algorithm SHA256
```

Checksums detect accidental corruption; they are not a publisher signature.

## First run and updates

The app starts in local mode using its own database and uploads folder under the
OS configuration directory (`picofarm-desktop`). Remote mode can be selected
from the tray's Desktop settings menu; configure an HTTPS server origin and its
API token, then quit and reopen. Remote mode does not start a local backend.
There is no offline command queue or command replay after reconnecting.

Autostart is opt-in. Closing the window hides it on Windows/macOS; on Linux it
quits to avoid leaving an inaccessible background app. Native notifications
require OS permission. Update checks only display release information; download
and replace the app manually. Quit the app before replacing its files. Preserve
the configuration directory, database, uploads, and encryption key together.
Never delete the encryption key when updating or restoring data.

## Known beta limitations

- Native GUI startup, tray, notifications and autostart have not been manually
  certified on all target operating systems; native CI compiles and packages.
- OAuth callbacks and public webhooks need a separately reachable server;
  desktop loopback addresses are not a public integration endpoint.
- Invalid saved configuration can require manual correction of `config.json`.
- Secrets use local files, not an OS credential store. Windows inherits profile
  ACLs rather than installing an owner-only DACL.
- Do not assume safe printer operation from a successful UI load. Validate with
  non-production devices first; no real printer commands are exercised in CI.
