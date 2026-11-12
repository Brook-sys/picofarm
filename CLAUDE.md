# PicoFarm

A print farm management platform for makers — product catalogs, multi-channel order tracking, printer fleet control, and profitability analytics.

## Tech Stack

- **Backend:** Go 1.24, Chi router, SQLite via `glebarez/go-sqlite` (WAL mode), `log/slog` (JSON to stdout)
- **Frontend:** React 19, TypeScript 5.9, Vite 7, Tailwind CSS 4, TanStack Query v5
- **Delivery:** self-hosted/local web app. The backend binary serves the API and the built frontend from `web/dist` on the same port; there is no desktop shell
- **Integrations:** Etsy (OAuth + PKCE, webhooks), Squarespace, Shopify, Mercado Livre, Shopee (see `docs/SALES_CHANNELS.md`), Bambu Lab (MQTT + cloud), OctoPrint, Moonraker/Klipper
- **Real-time:** WebSocket hub in `internal/realtime` (`nhooyr.io/websocket`)

## Project Structure

```
cmd/server/          # Standalone HTTP server entry point (the only binary)
internal/
  api/               # HTTP handlers & chi router
  service/           # Business logic layer
  repository/        # Data access (SQLite)
  model/             # Domain types & interfaces
  database/          # DB init, embedded schema, DBTX interface
  printer/           # Printer integrations
  realtime/          # WebSocket hub
  storage/           # File storage abstraction
  etsy/              # Etsy API client
  squarespace/       # Squarespace API client
  shopee/            # Shopee API client
  mercadolivre/      # Mercado Livre API client
  saleschannel/      # Provider-neutral sales channel contracts
  olx/               # OLX client (not registered in production)
  bambu/             # Bambu Lab cloud integration
  gcode/             # G-code inspection
  receipt/           # Receipt OCR parsing
  threemf/           # 3MF file parsing
  crypto/            # Encryption for settings at rest
  validation/        # Input validation
  version/           # Build version metadata
web/src/
  api/               # API client functions
  components/        # React components
  hooks/             # Custom hooks (useWebSocket, useMaterials, etc.)
  pages/             # Route components
  contexts/          # React contexts
  types/             # TypeScript type definitions
  lib/               # Utilities
migrations/          # Historical SQL. NOT executed by any code — see "Database & Migrations"
site/                # Docs/marketing site (Next.js; Vercel deploy is disabled)
```

## Architecture

- **Layered:** API handlers → Services → Repositories → SQLite
- **Repository pattern** with `DBTX` interface for transaction support
- **Dependency injection** — services injected into API handlers
- **`WithTransaction` helper** for ACID operations across repositories
- **WebSocket hub** broadcasts printer status and job state changes in real-time

## Common Commands

```bash
make dev              # Start backend (8084) + frontend (5173) together
make backend          # Go server only
make frontend         # Vite dev server only
make test             # go test -v ./...
make test-coverage    # Tests with coverage report
make build            # Production build (backend binary + frontend assets)
make lint             # golangci-lint + ESLint
make site             # Docs/marketing site dev server
```

There is no `make migrate`: the schema is embedded in the binary and applied on startup.

## Testing — MANDATORY

**Every change must include tests and pass the existing suite.**

1. **Always write tests** for new or modified code — no exceptions. If adding a new repository method, service function, or API endpoint, add corresponding test cases.
2. **Run tests before and after making changes** to confirm nothing is broken. If any test fails, fix it before moving on.
3. **Run tests incrementally** — after each meaningful change, run at least the relevant package tests (`go test ./internal/repository/...`) to catch regressions early rather than waiting until the end.
4. **Do not consider a task complete until all tests pass**, including both new and existing tests.

### Test conventions

- Go standard `testing` package with table-driven tests
- In-memory SQLite (`:memory:`) via an `openTestDB()` helper, defined per test package (`internal/database`, `internal/repository`)
- `internal/api` mounts the real router with an in-memory database through `newTestEnv(t)` in `testhelper_test.go`
- Tests live alongside source: `*_test.go`
- Always use `t.Helper()` in test helpers and `t.Cleanup()` for teardown
- Test function names: `TestFunctionName_scenario` (e.g., `TestCreateOrder_duplicateSKU`)

## Database & Migrations

- SQLite with WAL mode. Default path `~/.picofarm/picofarm.db`, overridable with `DATABASE_PATH`
- The schema lives in `internal/database/schema.sql`, embedded in the binary (`//go:embed`) and applied on startup by `RunMigrations`
- Older databases are brought up to date by idempotent in-Go compatibility statements in `internal/database/sqlite.go`
- `migrations/` holds 42 numbered SQL files from the pre-Go lineage. **No code reads them.** They are history, not a migration path; do not add files there expecting them to run

## Code Conventions

### Go
- Standard Go formatting (`gofmt`)
- Explicit error returns — no `panic` except during startup
- Structured logging with `log/slog` (JSON to stdout)
- CamelCase exported, camelCase unexported
- Doc comments on exported functions
- Interfaces defined near consumers, not implementors

### TypeScript/React
- Strict mode enabled (`noUnusedLocals`, `noUnusedParameters`, `verbatimModuleSyntax`)
- Functional components with hooks only — no class components
- Custom hooks prefixed with `use`
- PascalCase components, camelCase functions/variables
- Tailwind utility classes with `cn()` helper for conditional styling
- TanStack Query for all server state — no manual fetch/useEffect patterns
- Dark theme: `surface-*` (grayscale) and `accent-*` (orange) scales; new UI must match it

### Git
- Conventional commits with a scope: `fix(web):`, `feat(channels):`, `docs:`, `chore:`
- Imperative mood, concise subject
- Never commit generated artifacts (`bin/`, `web/dist/`, `uploads/`) or local-only scripts

## Key Patterns to Follow

- New API endpoints: add handler in `internal/api/`, service in `internal/service/`, repository in `internal/repository/`, and register the route in `internal/api/router.go`
- New frontend pages: add page component in `web/src/pages/`, route in `web/src/App.tsx`, API client in `web/src/api/client.ts`
- Types shared between front/back: Go structs in `internal/model/`, mirrored TypeScript types in `web/src/types/index.ts`
- Prefer editing existing files over creating new ones
- Keep the layered architecture clean — handlers should not contain business logic, repositories should not contain business logic
- When removing a feature, remove its frontend callers in the same change: routes deleted from the router have already silently broken pages once (`docs/API_CONTRACTS.md` tracks the wired surface)
