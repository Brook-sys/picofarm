## PicoFarm v1.3.0-beta.1 — first desktop packages

This beta adds desktop packages for Windows x64, Linux x64, macOS Intel and
macOS Apple Silicon. Desktop and web use the same React screens, HTTP API and
business rules; the headless/Docker product keeps no native GUI dependencies.

### Included

- Local backend or remote-server mode, one active mode per installation.
- Single instance, tray menu, configurable native notifications, opt-in
  autostart, and informational update checks (no automatic installation).
- Authenticated loopback gateway serving only bundled UI, with upstream
  credential isolation and hardened redirect validation.
- Shared runtime lifecycle and WebSocket shutdown fixes.
- Native archives, installation notes and SHA-256 checksums attached below.

### Validation and limitations

Go tests, selected race tests, frontend tests/build/lint, and native packaging
CI are validation gates. The web revision preceding this release was deployed
and smoke-tested across 14 screens plus API and WebSocket checks. **Native GUI
operation on all supported systems is not certified.**

Packages are unsigned on Windows and ad-hoc signed/not notarized on macOS.
Linux requires Ubuntu 24.04-compatible GTK4/WebKitGTK 6 libraries. OAuth/public
webhooks require explicit server-side setup; autostart/tray/notifications need
per-OS verification. Read `INSTALL.md` before running. Back up existing data
before testing, and validate printer operations on non-production equipment.

### Versioning

PicoFarm uses Semantic Versioning: `vMAJOR.MINOR.PATCH`, with prereleases such as
`v1.3.0-beta.1`. Breaking compatibility increments MAJOR, backward-compatible
features increment MINOR, fixes increment PATCH. Beta iteration increments the
final counter (`beta.2`, etc.). The Git tag must exactly match `VERSION`.
Prereleases do not advance Docker's stable `latest` tag or GitHub's latest
stable-release marker. Web and desktop share one product version.
