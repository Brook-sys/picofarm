# PicoFarm desktop app

The desktop product is an optional, isolated Go submodule under `desktop/` that
reuses the same backend runtime (`internal/appruntime`), the same React screens
in `web/`, and the same integration code as the self-hosted server. It is not a
rewrite and it does not fork pages.

Source of truth for the module internals: `desktop/README.md`. This document
records the product contract, security posture, and validation status that the
rest of the repository must stay aligned with.

## Products and modes

- Web app: the standalone server (`cmd/server`) serving API + `web/dist`.
- Desktop app: one active mode per installation, chosen in Desktop settings:
  - **Local** — starts the shared backend in-process on `127.0.0.1:0` with a
    generated API token. Data lives in the OS user config directory
    (`<config>/picofarm-desktop/picofarm.db`, `uploads/`, `encryption.key`).
  - **Remote** — connects to an HTTPS server origin with an API token. No
    local backend, no local printers, no local database.
- Only one instance runs: Wails' `SingleInstanceOptions` lock in
  `application.New` is acquired before the database, encryption key, services,
  or listener are touched. A second launch focuses the existing window.
- Changing mode (or remote target) requires quitting and reopening the app.
  There is no offline queue and no replay of physical commands after a
  restart or reconnection.

## Build and test contract

```sh
# stage frontend assets (web/dist -> desktop/assets/web, git-ignored)
node scripts/desktop-build.mjs stage

# headless core tests (GUI entrypoint excluded by build tag)
cd desktop && GOTOOLCHAIN=auto go test ./...

# native binaries per platform (run on native runners or locally with deps)
cd desktop && GOTOOLCHAIN=auto go build -tags desktop,production .
```

`make desktop-stage`, `make desktop-test`, and `make desktop-build` wrap the
same steps. The root module keeps zero native dependencies: Docker and headless
servers never compile `desktop/`.

## Gateway architecture and contract

The webview never loads remote code with native privileges. The desktop host
runs a loopback-only HTTP server that:

- serves the embedded bundled frontend from `desktop/assets/web`;
- reverse-proxies `/api/*`, `/uploads/*`, `/ws`, and `/health` to the local
  runtime or the configured remote origin;
- injects the configured backend token upstream (`Authorization: Bearer`), and
  additionally a `token` query parameter for `/ws`, which is where the hub
  accepts tokens;
- removes browser cookies and authorization headers before proxying, and strips
  upstream `Set-Cookie` from responses;
- forces a `sandbox` CSP plus `Content-Disposition: attachment` on any upstream
  document response, so remote servers cannot serve executable UI;
- allows only same-origin redirects: relative app paths pass through unchanged
  (integration callbacks such as `/settings?...` depend on this), absolute URLs
  pointing back at the gateway collapse to their path, and any other target is
  refused.

Session bootstrap: one-shot `GET /desktop/bootstrap?key=<random>` sets an
`HttpOnly`, `SameSite=Strict` session cookie and expires on first use (or after
60 seconds). Every other request requires an exact `Host` and `Origin` match,
rejects cross-site posts, and native mutations (`POST /desktop/*`) additionally
require the exact `Origin`.

Native capabilities are exposed as narrow endpoints, not per-endpoint bindings:

| Endpoint | Purpose |
| --- | --- |
| `GET /desktop/config` | Current settings without the remote token |
| `POST /desktop/config` | Save settings (mode, remote URL/token, tray, notifications, autostart); returns `restartRequired` |
| `POST /desktop/open-external` | Open an HTTP(S) link in the OS browser (used for OAuth authorization) |
| `POST /desktop/notify` | Native notification with server-side preference gate |
| `GET /desktop/setup` | Minimal settings page served by the shell |

The frontend bridge contract (`window.__PICOFARM_DESKTOP__`) is injected before
module boot and contains no secrets:

```ts
window.__PICOFARM_DESKTOP__ = {
  apiBaseURL: location.origin,
  wsBaseURL: location.origin + '/ws',
  openExternal: async (url) => {},
  notify: async ({ title, body, tag }) => {},
}
```

`platform.ts` treats the bridge as optional; a plain browser run is unchanged.
Notification tags `picofarm-finished`, `picofarm-failed`, `picofarm-critical`
map to native categories gated by local preferences (default on) with generic,
customer-free text and bounded in-memory deduplication.

## Native features

- System tray: open, Desktop settings, check for updates (informational dialog
  with the exact GitHub release link; never installs anything), quit.
- Close-to-tray hides the window on macOS/Windows. On Linux the window close
  quits instead, because status-area availability cannot be detected reliably
  and an unreachable application is worse than quitting. This is a deliberate
  fail-safe, documented here rather than faked.
- Autostart (`desktop/internal/autostart`): disabled by default, one
  Picofarm-owned entry per platform, atomic write with rollback. Requires
  per-OS validation in CI before being considered proven.
- Encryption key: generated per install as a 0600 private file under the OS
  config dir and set before the runtime constructs services. No machine-derived
  fallback is accepted. OS credential stores are not used; the file fallback is
  a documented limitation, not a keychain implementation.

## Limitations (honest status)

- Invalid saved configuration currently aborts startup before the settings UI
  is available; recovery requires correcting the local config file. A blank
  remote token is allowed for unauthenticated servers, but protected servers
  require a valid token; saving does not prove connectivity.
- File permissions described as `0600` are Unix permissions. On Windows the
  files inherit the user's profile-directory ACLs; Go `Chmod` does not install
  an owner-only DACL. Do not place the profile in a shared directory. Explicit
  Windows ACL enforcement or OS credential storage remains pending.

- Desktop builds were validated headlessly: `go test ./...` and
  `go test -race ./...` in `desktop`, plus Windows cross-compile and macOS
  purego cross-compile. Native Linux compilation and any GUI launch were not
  executed on the development host (GTK4/WebKitGTK 6 absent); CI runners must
  prove packaging. Artifacts are not signed or notarized.
- OAuth inside desktop mode is not fully supported. Same-origin redirects work,
  but an authorization callback registered against the server address will not
  land inside the desktop gateway; the user must complete authorization in the
  OS browser. Etsy in particular also has an unresolved server-side shop lookup
  issue independent of desktop.
- Linux tray hide is intentionally disabled (see above).
- WebView2 runtime is required on Windows; Linux requires GTK4/WebKitGTK 6
  runtime libraries; macOS builds are ad-hoc signed and will warn under
  Gatekeeper.
- Autostart actual registration per desktop environment and OS policy is
  unverified.