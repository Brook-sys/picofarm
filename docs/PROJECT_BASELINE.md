# PicoFarm Project Baseline

This document defines the current operational baseline for PicoFarm. Use it before and after improvement cycles to verify the project is healthy enough to continue.

## Current project identity

- Repository path: `/home/alpine/picofarm`
- Primary branch: `main`
- Normal push target: `picofarm main`
- Version file: `VERSION`
- Current application name in code/docs: PicoFarm, with some legacy Daedalus references still present in older docs and helper paths.

## Stack

| Layer | Current implementation |
| --- | --- |
| Backend | Go module `github.com/Brook-sys/picofarm`, Go `1.24.0` in `go.mod` |
| HTTP API | `go-chi/chi` router in `internal/api` |
| Database | SQLite via `github.com/glebarez/go-sqlite`, migrations in `migrations/` and bootstrap logic in `internal/database` |
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
| `SENTRY_DSN` | unset | Optional error reporting |
| `ETSY_CLIENT_ID` | unset | Optional Etsy integration |
| `ETSY_REDIRECT_URI` | `http://localhost:8080/api/integrations/etsy/callback` in `Makefile` | Check this before production/self-hosted use because backend defaults to port `8084` |

Local helper script defaults from `AGENTS.md` and `scripts/dev.sh`:

| Variable | Default |
| --- | --- |
| `PICOFARM_BIN` | `/tmp/daedalus-current` |
| `PICOFARM_LOG` | `/tmp/opencode/daedalus-backend.log` |
| `PICOFARM_PORT` | `8084` |
| `PICOFARM_REMOTE` | `picofarm` |
| `PICOFARM_BRANCH` | `main` |

`/tmp/daedalus-current` and related log paths are legacy names. Treat them as local helper details, not product identity.

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

`script/dev.sh` is intentionally local-only in this workspace and must not be committed unless the project policy changes.

## Current known validation output

On the baseline collection pass for this plan:

- `go test -v ./...` passed.
- `go test -run '^$' ./...` passed.
- `cd web && npm run build` passed.
- `make build` passed.
- `cd web && npm run lint` passed with warnings and zero errors.

Known frontend lint warnings to burn down in follow-up cycles:

- `web/src/components/Tooltip.tsx`: reads a React ref during render.
- `web/src/hooks/useWebSocket.ts`: state update pattern inside effect/reconnect setup.
- Several route pages have incomplete `useEffect` dependency arrays.
- `GCodeFiles.tsx`, `PrinterFiles.tsx`, and `Notifications.tsx` contain synchronous state updates inside effects.

Do not silence these warnings without fixing or documenting the underlying reason.

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
