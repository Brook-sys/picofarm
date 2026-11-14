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
SQLite (`internal/database`: embedded `schema.sql` + compatibility bootstrap)
```

Runtime storage is configurable, not automatically outside the checkout. The database defaults to `~/.picofarm/picofarm.db`, database backups live in a `backups` directory beside it, and file storage defaults to `./uploads` relative to the working directory. For persistent installations, configure explicit data paths outside build-cleanup locations. The Docker image uses `/data/picofarm.db` and `/data/uploads`.

The production frontend is served from `STATIC_DIR` (default `./web/dist`) on disk, not embedded in the Go binary. Development uses Vite on port 5174 with `/api` and `/ws` proxies to the backend on 8084; see [Getting started](GETTING_STARTED.md).

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
| `internal/saleschannel/` | Implemented provider-neutral contracts, registry, capability types, and redaction helpers; current service-backed adapters live in `internal/service/sales_channel_adapters.go` |
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
- The schema lives in `internal/database/schema.sql`, embedded in the binary (`//go:embed`) and applied on startup by `RunMigrations`.
- `internal/database/sqlite.go` opens the database, configures SQLite behavior, and applies schema/migration logic at startup.
- There is historical compatibility logic in the database bootstrap path that performs idempotent `ALTER TABLE`/`CREATE` work. Treat this area carefully: do not change it without tests for both a fresh database and an upgrade/partial-upgrade database.
- The old numbered SQL files under `migrations/` were removed: no code ever read them, and the schema file is the single source of truth. If you need that history, read it from git (`git log --diff-filter=D --name-only -- migrations/`).

Recommended schema-change policy:

1. Make schema changes in `internal/database/schema.sql` for new databases.
2. Add the matching idempotent statement (`CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... ADD COLUMN` guarded by the compatibility helper) for databases created by an older build.
3. Do not rewrite history: never edit a statement that an existing production database has already applied without a plan for the upgrade path.
4. Add tests that open a fresh DB and validate required tables/columns/foreign keys.
5. Add upgrade tests when touching legacy compatibility paths.
6. Log or surface unexpected migration failures; do not hide real errors as harmless idempotency.

## Realtime/event flow

- Backend realtime events flow through `internal/realtime` and are exposed over `/ws`.
- The frontend uses `web/src/hooks/useWebSocket.ts` and app-level status handling.
- WebSocket changes should be validated for reconnect, cleanup, and browser unmount behavior.

## Integrations

External integrations should not be required for the normal test suite.

Sales channels have a dedicated architecture guide in `docs/SALES_CHANNELS.md`. Use that document before adding or changing Etsy, Squarespace, Shopify, product/listing sync, external order processing, product links, or future marketplace/storefront providers. A provider registry with capability-driven adapters and generic `/api/sales-channels/*` descriptor/status, sync, read-model, order-processing, and product-link routes is implemented. Generic connection/OAuth routes remain planned; legacy `/api/integrations/{provider}` routes remain supported. Mercado Livre and Shopee are registered descriptors with fake-client-tested adapters, but the production registry does not wire their credentials/clients, so they are not live-connectable. See [API contracts](API_CONTRACTS.md#integrations) for current boundaries.

| Integration | Area |
| --- | --- |
| Etsy | OAuth, listings, receipts/orders, webhooks, inventory/template linking |
| Squarespace | Orders/products sync |
| Shopify | OAuth/orders/product links; existing support is partial and should be represented by capabilities |
| Bambu Lab | Cloud/LAN printer integration and status/control |
| OctoPrint | Printer control/file/status integration |
| Moonraker/Klipper | Printer control/status integration |

Use mocks/fakes or isolated unit tests when adding regression coverage for external systems.

## Where to make common changes

| Change | Start here | Also check |
| --- | --- | --- |
| New backend endpoint | `internal/api/` | service, repository, model, frontend API client, `docs/API_CONTRACTS.md` |
| New business workflow | `internal/service/` | repository transactions, model tests, API handler tests |
| New database field/table | `internal/database/schema.sql`, `internal/database/sqlite.go` | repository code, fresh/upgrade tests, TS types, `docs/API_CONTRACTS.md` when JSON changes |
| New frontend server data | `web/src/api/`, `web/src/types/` | route/page, hooks, backend JSON tests, `docs/API_CONTRACTS.md` |
| Printer behavior | `internal/printer/`, relevant service | safety docs, regression matrix, mocks |
| Upload/file behavior | `internal/storage/`, API handlers | security model, cleanup/retention behavior |
| Notifications | notification service/API/types/pages | test delivery, templates, regression matrix |
| Sales channel/provider | `docs/SALES_CHANNELS.md`, `internal/saleschannel/` | provider client/adapters, repository migrations, API contracts, frontend feature types/components, security docs |
| Build/dev command | `Makefile`, `web/package.json`, scripts | `README.md`, `docs/PROJECT_BASELINE.md`, `AGENTS.md` |

## Architectural guardrails for future agents

- Keep API handlers thin: parse/validate input, call services, return responses.
- Keep business rules in services, not repositories or React components.
- Keep repositories focused on SQL/data mapping.
- Use transactions for workflows that mutate multiple tables.
- Prefer testable pure helpers for validation and calculations.
- Keep frontend API response types synchronized with Go JSON shapes.
- Avoid adding more behavior to already-large files when a small extracted helper/component would make the next change safer.
