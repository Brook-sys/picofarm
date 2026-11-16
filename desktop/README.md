# Picofarm desktop host

Isolated Go submodule. The root module keeps zero native dependencies; Docker
and headless servers never compile this directory.

## Build

- Stage frontend: `scripts/desktop-build.mjs stage` (copies `web/dist` to
  `desktop/assets/web`, git-ignored, required for embedding).
- Core: `GOTOOLCHAIN=auto go test ./...` (build-tag `desktop` root files are
  excluded; no GUI deps needed).
- Native: `GOTOOLCHAIN=auto go build -tags desktop,production .` from
  `desktop/` (single tagged `main.go`; cgo per platform: GTK4/WebKitGTK 6 on
  Linux via Wails v3.1.0, Xcode WebKit on macOS, WebView2 runtime on Windows).
- Verified here: Windows cross-compile (`GOOS=windows CGO_ENABLED=0`) and
  macOS purego cross-compile (`GOOS=darwin -tags purego`). Native Linux build
  and GUI launch were NOT executed on the development host (GTK4/WebKit6
  absent), so packaging needs CI evidence before claiming it works.

## Architecture

One loopback-only HTTP gateway (`internal/shell`) serves embedded `web/dist`
assets and reverse-proxies `/api/*`, `/uploads/*`, `/ws`, `/health` to the
runtime — local (`appruntime.Start` on `127.0.0.1:0`, generated token) or a
validated remote origin. Only bundled UI executes; upstream documents get a
sandboxing CSP and forced download, upstream `Set-Cookie` and redirects are
stripped, browser cookies/auth headers are replaced with the configured token,
and `/ws` receives the token as a query parameter (Wails backend ignores
`Authorization`).

Auth: `GET /desktop/bootstrap?key=<single-use random>` sets an `HttpOnly`,
`SameSite=Strict` session cookie and expires on first use (or after 60s).
Everything requires an exact `Origin`/Host match and forbids cross-site posts.
Bootstrap URL is never logged and is not the windowed origin's state.

Native access through `/desktop/open-external`, `/desktop/notify`,
`/desktop/config` (GET/POST), `/desktop/setup` web settings page. The frontend
bridge `window.__PICOFARM_DESKTOP__` is injected before module boot and only
exposes `apiBaseURL`, `wsBaseURL`, `openExternal`, `notify` (tag
`picofarm-<kind>` mapped to `finished/failed/critical`, server-side preference
gate). No secret reaches the browser.

## Hardening / protection

- Encryption key: generated once per install in a 0600 private file under the
  OS config dir (keychain verification absent; no hostname-fallback fake).
  Root runtime owns key semantics; desktop only sets it before construction.
- Secrets: config 0600, no delete of key on reinstall (keeps DB decryptable),
  no API token in `GET /desktop/config` responses.
- Single instance before runtime: Wails `SingleInstanceOptions` lock in
  `application.New` precedes DB/key/service startup.

## Explicitly pending (not promised)

- Autostart exists via `internal/autostart` but is off by default and must be
  validated per OS in CI (settings toggle writes atomically, rollback on fail).
- Update checks: tray menu "Check for updates" via `internal/updates` shows
  latest stable release link only (10s timeout, no redirects). No installation
  path is implemented.
- OAuth/redirect flows inside desktop mode: only same-origin redirects are
  allowed — relative app paths (e.g. `/settings?...` integration callbacks)
  pass through and absolute URLs pointing at the gateway collapse to their
  path. Redirects to any other host are refused, so authorization must be
  completed by manually re-entering the callback URL. Full OAuth inside
  desktop mode is not supported yet.
- Linux tray fail-safe: closing the window quits unless/until status-area
  availability can be detected; on Linux this was chosen over an unreachable
  application. macOS/Windows use tray-hide.