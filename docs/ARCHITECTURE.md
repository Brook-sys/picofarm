# PicoFarm Architecture

PicoFarm is a print farm management and order fulfillment application. It combines product catalog management, sales/order workflows, printer fleet coordination, file libraries, costing, notifications, and integrations with external commerce/printer systems.

## High-level shape

```text
React/Vite frontend
        |
        | HTTP JSON + WebSocket
        v
Go chi router (`internal/api`)
        |
        v
Services (`internal/service`)
        |
        v
Repositories (`internal/repository`)
        |
        v
SQLite (`internal/database`, `migrations/`)
```

Runtime files such as uploads, generated thumbnails, backups, and downloaded/processed model files live outside the source tree during normal operation.

## Backend layout

| Path | Responsibility |
| --- | --- |
| `cmd/server/main.go` | Standalone server entry point, environment defaults, service wiring, scheduler/printer startup |
| `internal/api/` | HTTP routes, request parsing, response formatting, middleware, static serving |
| `internal/service/` | Business workflows and coordination across repositories/integrations |
| `internal/repository/` | SQLite persistence and transactions |
| `internal/model/` | Domain models and shared data shapes |
| `internal/database/` | SQLite open/configuration, default path, schema bootstrap/migration execution |
| `internal/realtime/` | WebSocket hub for status and domain events |
| `internal/storage/` | Local file storage helpers |
| `internal/printer/` | Printer abstractions and protocol implementations |
| `internal/bambu/` | Bambu-specific integration logic |
| `internal/etsy/` | Etsy API/OAuth/webhook-facing logic |
| `internal/squarespace/` | Squarespace API logic |
| `internal/threemf/` | 3MF parsing and metadata extraction |
| `internal/receipt/` | Receipt parsing/OCR-adjacent behavior |
| `internal/validation/` | Shared validation helpers |

## Frontend layout

| Path | Responsibility |
| --- | --- |
| `web/src/App.tsx` | Route shell and app-level WebSocket use |
| `web/src/api/` | Browser API client functions for backend endpoints |
| `web/src/pages/` | Route/page components |
| `web/src/components/` | Shared UI components |
| `web/src/hooks/` | Custom hooks and server-state helpers |
| `web/src/contexts/` | React contexts |
| `web/src/types/` | TypeScript domain and API types |
| `web/src/lib/` | Shared utilities |

Current complexity hotspots:

- `web/src/api/client.ts` is large and should be split by domain while preserving imports through a compatibility barrel.
- `web/src/pages/ProjectDetail.tsx` is very large and should be decomposed into section components gradually.
- Several pages still use manual `useEffect` loading patterns; the project convention in `CLAUDE.md` prefers TanStack Query for server state.

## Database and migrations

- The default database path is `~/.picofarm/picofarm.db`.
- Versioned SQL migrations live under `migrations/`.
- `internal/database/sqlite.go` opens the database, configures SQLite behavior, and applies schema/migration logic at startup.
- There is historical compatibility logic in the database bootstrap path that performs idempotent `ALTER TABLE`/`CREATE` work. Treat this area carefully: do not change it without tests for both a fresh database and an upgrade/partial-upgrade database.

Recommended migration policy:

1. Add new schema changes as a new numbered migration whenever possible.
2. Do not destructively edit old migrations without a clear upgrade strategy.
3. Add tests that open a fresh DB and validate required tables/columns/foreign keys.
4. Add upgrade tests when touching legacy compatibility paths.
5. Log or surface unexpected migration failures; do not hide real errors as harmless idempotency.

## Realtime/event flow

- Backend realtime events flow through `internal/realtime` and are exposed over `/ws`.
- The frontend uses `web/src/hooks/useWebSocket.ts` and app-level status handling.
- WebSocket changes should be validated for reconnect, cleanup, and browser unmount behavior.

## Integrations

External integrations should not be required for the normal test suite.

| Integration | Area |
| --- | --- |
| Etsy | OAuth, listings, receipts/orders, webhooks, inventory/template linking |
| Squarespace | Orders/products sync |
| Bambu Lab | Cloud/LAN printer integration and status/control |
| OctoPrint | Printer control/file/status integration |
| Moonraker/Klipper | Printer control/status integration |

Use mocks/fakes or isolated unit tests when adding regression coverage for external systems.

## Where to make common changes

| Change | Start here | Also check |
| --- | --- | --- |
| New backend endpoint | `internal/api/` | service, repository, model, frontend API client, `docs/API_CONTRACTS.md` |
| New business workflow | `internal/service/` | repository transactions, model tests, API handler tests |
| New database field/table | `migrations/`, `internal/database` | repository code, fresh/upgrade tests, TS types, `docs/API_CONTRACTS.md` when JSON changes |
| New frontend server data | `web/src/api/`, `web/src/types/` | route/page, hooks, backend JSON tests, `docs/API_CONTRACTS.md` |
| Printer behavior | `internal/printer/`, relevant service | safety docs, regression matrix, mocks |
| Upload/file behavior | `internal/storage/`, API handlers | security model, cleanup/retention behavior |
| Notifications | notification service/API/types/pages | test delivery, templates, regression matrix |
| Build/dev command | `Makefile`, `web/package.json`, scripts | `README.md`, `docs/PROJECT_BASELINE.md`, `AGENTS.md` |

## Architectural guardrails for future agents

- Keep API handlers thin: parse/validate input, call services, return responses.
- Keep business rules in services, not repositories or React components.
- Keep repositories focused on SQL/data mapping.
- Use transactions for workflows that mutate multiple tables.
- Prefer testable pure helpers for validation and calculations.
- Keep frontend API response types synchronized with Go JSON shapes.
- Avoid adding more behavior to already-large files when a small extracted helper/component would make the next change safer.
