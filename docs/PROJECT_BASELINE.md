# PicoFarm Project Baseline

This document defines the current operational baseline for PicoFarm. Use it before and after improvement cycles to verify the project is healthy enough to continue.

## Current project identity

- Repository path: local clone (path varies per machine; never commit absolute home paths into docs)
- Primary branch: `main`
- Normal push target: `picofarm main`
- Version file: `VERSION`
- Current application name in code/docs: PicoFarm. The legacy Daedalus name only survives in the marketing/docs site under `site/` (tracked for cleanup); the runtime path has none left.

## Stack

| Layer | Current implementation |
| --- | --- |
| Backend | Go module `github.com/Brook-sys/picofarm`, Go `1.26.11` in `go.mod` (mínimo: stdlib corrigida para GO-2026-6859/GO-2026-6809) |
| HTTP API | `go-chi/chi` router in `internal/api` |
| Database | SQLite via `github.com/glebarez/go-sqlite`, with the schema embedded in the binary (`internal/database/schema.sql`) applied on startup, plus idempotent in-Go compatibility migrations for older databases |
| Frontend | React 19, TypeScript 6.0, Vite 8 (Rolldown), Tailwind CSS 4 |
| Interface languages | `i18next` + `react-i18next`, bundled JSON catalogs for `en` and `pt-BR`; per-browser/device preference, see `docs/I18N.md` |
| Realtime | WebSocket hub in `internal/realtime`; browser hook in `web/src/hooks/useWebSocket.ts` |
| Integrations | Etsy, Squarespace, Bambu Lab, OctoPrint, Moonraker/Klipper |
| Build | `Makefile`, `web/package.json`, `Dockerfile`; Node.js 22.12+ (Docker/CI use Node 22) |

The Docker publication job has a 30-minute timeout. Its frontend stage runs on `BUILDPLATFORM`, generating shared static assets without running Node under target-architecture emulation. Backend and runtime stages still use the target platform; ARM64 publication still requires QEMU. This configuration must be validated by a multi-platform build, not just a local frontend build.

## Runtime defaults

Backend defaults are defined from `cmd/server/main.go` and database helpers:

| Setting | Default | Notes |
| --- | --- | --- |
| `PORT` | `8084` | Backend HTTP server and static frontend host |
| `DATABASE_PATH` | `~/.picofarm/picofarm.db` | Used when no explicit database path is provided |
| `UPLOAD_DIR` | `./uploads` | Runtime uploads and generated files |
| `STATIC_DIR` | `./web/dist` | Static frontend assets in production build |
| `ALLOWED_ORIGINS` | unset | Comma-separated CORS origins; unset allows local development origins only. Also extends the WebSocket origin allowlist |
| `API_TOKEN` | unset | Optional shared token for `/api/*`; when set, requests need `Authorization: Bearer <token>`. Named tokens can be managed at runtime via `/api/api-tokens` |
| `REQUIRE_API_TOKEN` | unset | Enables authentication without a shared token (`1`/`true`/`yes`/`on`/`enabled`). Fails open with a log warning if no token service is available |
| `SENTRY_DSN` | unset | Optional error reporting |
| `ETSY_CLIENT_ID` | unset | Optional Etsy keystring; requires `ETSY_SHARED_SECRET` for environment-based configuration |
| `ETSY_SHARED_SECRET` | unset | Etsy application shared secret; distinct from the webhook signing secret; stored encrypted when configured |
| `ETSY_WEBHOOK_SECRET` | unset | Optional Etsy webhook signing secret. When set, events with a missing or invalid `X-Etsy-Signature` are rejected with `401` instead of accepted |
| `ETSY_REDIRECT_URI` | `http://localhost:8084/api/integrations/etsy/callback` | Same single default the backend uses (`etsy.DefaultRedirectURI`). The value must also be registered as a Callback URL on the Etsy app, or Etsy refuses the authorization with `The requested redirect URL is not permitted.`; override it for any host, port, or reverse-proxy path difference |
| `SLICER_CONNECTION_URL` | unset | Overrides the stored `slicer_connection_url` setting. Use it to pin the orca-slicer-api container address from the deployment (compose/systemd/k8s) instead of relying on what an earlier version wrote into the database. `GET /api/slicer/config` reports `connection_url_source` (`env` or `settings`) so the UI can explain why a saved URL has no effect |
| `PICOFARM_SCAN_NETWORK` | unset | CIDR to scan for printers (`192.0.2.0/24` style value; use your own LAN subnet). Required whenever the backend runs in a container on a bridge network: the first non-loopback interface belongs to the container, not to the LAN where the printers live. Host networking is the alternative. Scans honor the real prefix length and are capped at 1024 hosts per pass, so pin a small subnet rather than a /8 |

### Printer reconnection configuration

`PRINTER_RECONNECT_INTERVAL=10s` sets the minimum interval between attempt starts for one printer. Attempt timeout is derived as interval minus one second (`9s` by default); minimum supported interval is `2s`. `PRINTER_RECONNECT_MAX_CONCURRENT=4` limits simultaneous attempts (accepted range `1`–`256`; upper bound is an allocation guard, not a measured safe capacity). Invalid configuration prevents server startup rather than silently using defaults. Values are read at startup; restart to apply changes.

These values bound scheduling, not a promise of recovery within 10 seconds: booting devices, network availability and queued printers add latency. The concurrency default is conservative and has not been certified as a hardware-specific maximum. Reconnection must not replay start/resume commands. See the printer connection contract and regression matrix for behavior and validation limits.

### Etsy connection checklist

The Etsy application credentials and registered callback URLs are managed in the Etsy Developer Portal. In order to connect a shop:

1. Register the exact URI (scheme, host, port, and path) of your deployment as a **Callback URL** on the app in the Etsy Developer Portal. The default is `http://localhost:8084/api/integrations/etsy/callback`; anything reachable through a proxy or another port must be registered too.
2. Set the Client ID (Keystring) and separate Shared Secret either through `ETSY_CLIENT_ID`/`ETSY_SHARED_SECRET` or in **Settings → Integrations → Etsy**. Older installations must add the Shared Secret before reconnecting; never put it in Client ID. When it is configured but wrong, the form can be reopened with *Edit credentials* — the keystring and the redirect URI are both editable after configuration.
3. Start the connection and authorize on Etsy. If Etsy refuses, the Settings page now shows its own `error_description`, which names the URI that was rejected.
4. When stored settings exist and `ETSY_CLIENT_ID` is empty, they take precedence over the environment at startup; a redirect URI persisted by an older build (port 8080) is rewritten automatically on boot and logged as `repaired legacy Etsy redirect URI`.

Local helper script defaults from `AGENTS.md` and `scripts/dev.sh`:

| Variable | Default |
| --- | --- |
| `PICOFARM_BIN` | `/tmp/picofarm-current` |
| `PICOFARM_LOG` | `/tmp/opencode/picofarm-backend.log` |
| `PICOFARM_PORT` | `8084` |
| `PICOFARM_REMOTE` | `picofarm` |
| `PICOFARM_BRANCH` | `main` |

## Canonical validation commands

Run the relevant subset during development. Before finishing a consolidation cycle, prefer the full gate.

```sh
git status --short
go test -v ./...
node --test web/tests/*.test.mjs
cd web && npm run lint
cd web && npm run build
make build
```

Frontend copy/i18n checks (diagnostic, not proof of translation quality):

```sh
node --test web/tests/i18n-catalogs.test.mjs web/tests/i18n-source.test.mjs
node web/scripts/audit-ui-text.mjs
```

Desktop-specific validation (module internals are in `desktop/README.md`):

```sh
node scripts/desktop-build.mjs stage
cd desktop && GOTOOLCHAIN=auto go test ./...
cd desktop && GOTOOLCHAIN=auto go test -race ./...
```

Native desktop builds (`cd desktop && GOTOOLCHAIN=auto go build -tags desktop,production .`)
require platform GUI dependencies (GTK4/WebKitGTK 6 on Linux, Xcode/WebKit on
macOS, WebView2 runtime on Windows) and were not executed on the headless
development host; use `make desktop-build` or the desktop CI workflow on native
runners and do not claim native artifacts work without that evidence.

When both backend and frontend can be affected, `make build` is the canonical build gate because it compiles the Go server and production frontend assets.

When backend runtime behavior changes, also use the local helper when available:

```sh
scripts/dev.sh doctor
scripts/dev.sh status
scripts/dev.sh backend
scripts/dev.sh health
scripts/dev.sh logs
```

`scripts/dev.sh` is intentionally local-only in this workspace and must not be committed unless the project policy changes.

## Current known validation output

On the baseline collection pass for this plan:

- `go test -v ./...` passed.
- `go test -run '^$' ./...` passed.
- `cd web && npm run build` passed.
- `make build` passed.
- `cd web && npm run lint` passed with zero errors.

Current validation output after the interface-localization cycle (2026-11-19):

- `node --test web/tests/*.test.mjs` passed: 57 tests, 0 failures (includes catalog parity/interpolation, literal-key coverage and currency/locale independence).
- `GOTOOLCHAIN=auto go test ./...` passed (the pinned local toolchain is older than the `go.mod` minimum; `scripts/dev.sh test` with `GOTOOLCHAIN=local` fails on the version check, not on tests).
- `cd web && npm run build` passed (`tsc -b` clean).
- `cd web && npm run lint` passed with zero errors and 29 warnings.
- Browser smoke visited 20 routes in each language: no JavaScript errors or raw interpolation placeholders. Browser-language fallback, blocked-storage language switching and keyboard skip-link focus passed. This does not cover every modal, mobile viewport, screen reader or native desktop workflow.

Known validation warnings after the dependency compatibility update:

- `cd web && npm run lint` passes with 29 React Hooks warnings (27 before localization); the same pre-existing families remain (`set-state-in-effect`, declaration order and `exhaustive-deps` in effects that already used empty dependency arrays). This is follow-up work, not a clean lint baseline.
- The production frontend main bundle grew from approximately 786 kB minified (162 kB gzip) to 1108.92 kB (255.88 kB gzip) after localization: both languages and every namespace are bundled eagerly in the entry chunk. This is a real loading-cost regression on slow links. Follow-up: load the inactive language and non-critical namespaces on demand, or split catalogs into separately cached chunks. Do not describe this as a neutral change.
- TypeScript is held at `~6.0.3`: `typescript-eslint@8.73.0` requires TypeScript `<6.1.0`, so the proposed TypeScript 7 update cannot yet be adopted without an unsupported peer combination.
- Vite 8 chunk groups use `rolldownOptions.output.codeSplitting.groups`; the former object-form `manualChunks` is not supported.

Do not silence warnings without fixing or documenting the underlying reason.

## Generated/local files that must not be committed

Expected generated or local-only paths include:

- `bin/`
- `web/dist/`
- `desktop/assets/web/` (staged copy of `web/dist` for the desktop embed)
- `desktop/dist/` (native packaging output)
- `uploads/`
- `web/node_modules/`
- `.hermes/` local plans/session artifacts
- local helper scripts intentionally excluded through `.git/info/exclude`
- logs, temp files, coverage output, and local credential files

Always inspect `git status --short` before committing.

## Commit and push policy

- Use small commits per cycle or subcycle.
- Do not mix broad refactors with behavior changes.
- Do not force-push.
- Push normal commits with:

```sh
git push picofarm main
```

Before each commit, report the validation commands actually run and their real results.

## Baseline success criteria

A cycle can be called complete when:

1. The relevant tests/build/lint pass, or any remaining issue is documented with evidence and a follow-up plan.
2. Documentation affected by the change is updated.
3. `git status --short` contains only intentional changes.
4. No generated files, secrets, or local-only scripts are staged.
5. The user receives a concise Portuguese report with changed files, validations, risks, and next step.
