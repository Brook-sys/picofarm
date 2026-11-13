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
| Frontend | React 19, TypeScript 5.9, Vite 7, Tailwind CSS 4 |
| Realtime | WebSocket hub in `internal/realtime`; browser hook in `web/src/hooks/useWebSocket.ts` |
| Integrations | Etsy, Squarespace, Bambu Lab, OctoPrint, Moonraker/Klipper |
| Build | `Makefile`, `web/package.json`, `Dockerfile` |

## Runtime defaults

Backend defaults are defined from `cmd/server/main.go` and database helpers:

| Setting | Default | Notes |
| --- | --- | --- |
| `PORT` | `8084` | Backend HTTP server and static frontend host |
| `DATABASE_PATH` | `~/.picofarm/picofarm.db` | Used when no explicit database path is provided |
| `UPLOAD_DIR` | `./uploads` | Runtime uploads and generated files |
| `STATIC_DIR` | `./web/dist` | Static frontend assets in production build |
| `ALLOWED_ORIGINS` | unset | Comma-separated CORS origins; unset allows local development origins only |
| `SENTRY_DSN` | unset | Optional error reporting |
| `ETSY_CLIENT_ID` | unset | Optional Etsy integration |
| `ETSY_WEBHOOK_SECRET` | unset | Optional Etsy webhook signing secret. When set, events with a missing or invalid `X-Etsy-Signature` are rejected with `401` instead of accepted |
| `ETSY_REDIRECT_URI` | `http://localhost:8084/api/integrations/etsy/callback` | Same single default the backend uses (`etsy.DefaultRedirectURI`). The value must also be registered as a Callback URL on the Etsy app, or Etsy refuses the authorization with `The requested redirect URL is not permitted.`; override it for any host, port, or reverse-proxy path difference |

### Etsy connection checklist

The redirect URI is the only piece of the Etsy OAuth flow that lives outside this repository. In order to connect a shop:

1. Register the exact URI (scheme, host, port, and path) of your deployment as a **Callback URL** on the app in the Etsy Developer Portal. The default is `http://localhost:8084/api/integrations/etsy/callback`; anything reachable through a proxy or another port must be registered too.
2. Set the Client ID (Keystring) either through `ETSY_CLIENT_ID` or in **Settings → Integrations → Etsy**. When it is configured but wrong, the form can be reopened with *Edit credentials* — the keystring and the redirect URI are both editable after configuration.
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
cd web && npm run lint
cd web && npm run build
make build
```

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

Known validation warnings to burn down in follow-up cycles:

- `cd web && npm run build` reports stale Browserslist/caniuse-lite data.
- The production frontend bundle is still larger than Vite's default warning threshold.

Do not silence warnings without fixing or documenting the underlying reason.

## Generated/local files that must not be committed

Expected generated or local-only paths include:

- `bin/`
- `web/dist/`
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
