# PicoFarm API and Data Contracts

This document records the current HTTP/JSON contract conventions between the Go backend and the React/TypeScript frontend. It is based on the current router in `internal/api/router.go`, Go models in `internal/model/`, and browser client/types in `web/src/api/client.ts` and `web/src/types/index.ts`.

## Contract sources of truth

| Layer | Source |
| --- | --- |
| Route table | `internal/api/router.go` |
| Sensitive endpoint risk inventory | `docs/SECURITY_ENDPOINTS.md` |
| Backend JSON shapes | `internal/model/*.go` plus handler-local request/response structs |
| Frontend API calls | `web/src/api/client.ts` |
| Frontend domain/API types | `web/src/types/index.ts` |
| Database schema | `internal/database/schema.sql` (embedded in the binary) plus the idempotent compatibility statements in `internal/database/sqlite.go` |

When an endpoint or JSON shape changes, update the backend model/handler, frontend API client, frontend type, tests, and this document in the same cycle. If the route can read private data, mutate state, touch files, control hardware, change settings, use credentials, or interact with an external service, update `docs/SECURITY_ENDPOINTS.md` too.

## Base URLs and protocol

- Backend health: `GET /health`.
- JSON API prefix: `/api`.
- Public unauthenticated API prefix: `/api/public`.
- Realtime endpoint: `GET /ws`.
- Frontend API base selection lives in `web/src/api/client.ts`:
  - `VITE_API_URL` when set;
  - otherwise the browser window origin;
  - otherwise `http://localhost:8084` for non-browser fallback.
- Browser CORS origins are configured by backend `ALLOWED_ORIGINS`; when unset, only local development origins are allowed.
- All routes include baseline browser hardening headers from `internal/api/middleware.go`; clients should not depend on these headers for application-level auth.

## JSON conventions

- Field names are snake_case JSON keys from Go struct tags.
- Go `uuid.UUID` values are serialized as strings; TypeScript represents them as `string`.
- Go `time.Time` and `*time.Time` values are serialized as timestamp strings; TypeScript represents them as `string` and optional `string` respectively.
- Go `omitempty` fields may be absent from responses. Frontend types should mark those as optional or nullable to match actual JSON.
- Monetary integer fields ending in `_cents` are minor units with two decimal places, not decimal dollars. The configured display currency changes formatting only; it does not convert stored values.
- Durations are seconds unless the field name says otherwise, for example `timeout_minutes`.
- Most list endpoints return bare arrays. Aggregated endpoints may return objects with `items`, `summary`, `counts`, or similarly named fields.
- Error responses should be JSON objects with at least `error: string`. The frontend fetch wrapper throws `new Error(error.error || HTTP_STATUS)` for non-2xx responses.
- `204 No Content` maps to `undefined` in the frontend wrapper.

## Shared enum contract

Keep these literal sets synchronized between Go and TypeScript.

| Domain | Go source | TypeScript source | Values |
| --- | --- | --- | --- |
| Task status | `model.TaskStatus` | `TaskStatus` | `pending`, `in_progress`, `completed`, `cancelled` |
| Part status | `model.PartStatus` | `PartStatus` | `design`, `printing`, `complete` |
| File type | `model.FileType` | `FileType` | `stl`, `3mf`, `gcode` |
| Connection type | `model.ConnectionType` | `ConnectionType` | `manual`, `octoprint`, `bambu_lan`, `bambu_cloud`, `moonraker`, `chitu` |
| Printer status | `model.PrinterStatus` | `PrinterStatus` | `idle`, `printing`, `paused`, `error`, `offline` |
| Material type | `model.MaterialType` | `MaterialType` | `pla`, `petg`, `abs`, `asa`, `tpu`, `supply` |
| Spool status | `model.SpoolStatus` | `SpoolStatus` | `new`, `in_use`, `low`, `empty`, `archived` |
| Print job status | `model.PrintJobStatus` | `PrintJobStatus` | `queued`, `assigned`, `uploaded`, `printing`, `paused`, `completed`, `failed`, `cancelled` |
| Failure category | `model.FailureCategory` | `FailureCategory` | `mechanical`, `filament`, `adhesion`, `thermal`, `network`, `user_cancelled`, `unknown` |
| Queue item status | `model.QueueItemStatus` | `QueueItemStatus` | `draft`, `queued`, `ready`, `blocked`, `printing`, `paused`, `done`, `failed`, `cancelled` |
| Queue source type | `model.QueueSourceType` | `QueueSourceType` | `upload`, `print_job`, `manual`, `library`, `project` |
| Dispatch request status | `model.DispatchRequestStatus` | `DispatchRequestStatus` | `pending`, `confirmed`, `rejected`, `expired` |
| Order status | `model.OrderStatus` | `OrderStatus` | `pending`, `in_progress`, `completed`, `shipped`, `cancelled` |
| Order source | `model.OrderSource` | `OrderSource` | `manual`, `etsy`, `squarespace`, `shopify`, `quote` |
| Quote status | `model.QuoteStatus` | `QuoteStatus` | `draft`, `sent`, `accepted`, `rejected`, `expired` |
| Quote line item type | `model.QuoteLineItemType` | `QuoteLineItemType` | `printing`, `post_processing`, `consulting`, `design`, `other`, `labor`, `consumables`, `shipping`, `finishing` |
| Discount type | `model.DiscountType` | `DiscountType` | `none`, `flat`, `percent` |
| Sales-channel ID | `saleschannel.ChannelID` | `SalesChannelID` | `etsy`, `squarespace`, `shopify`, `mercado_livre`, `shopee` |
| Sales-channel auth type | `saleschannel.ProviderDescriptor.AuthType` | `SalesChannelAuthType` | `oauth`, `api_key`, `manual` |
| Sales-channel capability | `saleschannel.Capability` | `SalesChannelCapability` | `oauth`, `api_key`, `orders_read`, `products_read`, `inventory_write`, `webhooks` |
| Sales-channel connection state | `saleschannel.ConnectionState` | pending feature-local type | `disconnected`, `connected`, `needs_attention` |
| Planned sales-channel connect request | future `saleschannel.ConnectRequest` | future `SalesChannelConnectRequest` | Provider-specific config accepted, secrets never echoed |
| Planned sales-channel auth URL response | future `saleschannel.AuthURLResult` | future `SalesChannelAuthUrlResult` | `auth_url`, `state`, `expires_at`; no tokens/secrets |
| Planned sales-channel disconnect request | future `saleschannel.DisconnectRequest` | future `SalesChannelDisconnectRequest` | `connection_id`, optional future `revoke_remote` |
| Sales-channel sync kind | `saleschannel.SyncKind` | pending feature-local type | `orders`, `products`, `all` |
| Sales-channel sync run status | `saleschannel.SyncRunStatus` | pending feature-local type | `pending`, `running`, `succeeded`, `failed` |

Do not add frontend-only enum values unless the backend explicitly returns them or the type is clearly marked as UI-local.

## Currency configuration

- `GET /api/settings/currency_code` returns the persisted setting when present. A missing setting is interpreted by the application as `USD`.
- `PUT /api/settings/currency_code` accepts `{ "value": "<ISO code>" }`, normalizes case/whitespace, and rejects unsupported codes with `400`.
- Supported codes are `USD`, `BRL`, `EUR`, `GBP`, `CAD`, `AUD`, `JPY`, `CNY`, `INR`, `MXN`, `ARS`, `CLP`, and `COP`.
- `GET /api/public/business-info` always includes `currency_code`; it returns `USD` when no setting exists. Public quotes use this field.
- Changing `currency_code` changes symbols and number formatting only. PicoFarm performs no exchange-rate lookup or automatic currency conversion.
- Internal costs, revenue, profit, expenses, sales, quotes, and analytics are scoped to the configured currency. New internal records inherit `currency_code` when their payload omits `currency`.
- **Cost sources carry no currency of their own.** `materials.cost_per_kg`, `printers.cost_per_hour_cents`,
  `printers.purchase_price_cents` and `project_supplies` have no `currency` column. `GET /api/stats/financial`
  therefore reports the currency its cost figures were entered in via `cost_basis_currency`, and sets
  `profit_mixed_currency: true` when that differs from the configured `currency`. In that case `net_profit_cents`
  is returned as `0` — subtracting costs entered in one currency from revenue in another yields no unit, so the
  API refuses to report it as profit. Buffering costs to a different currency requires re-entering them, not a
  conversion.
- External sales-channel orders/products with an explicit `currency` continue to use that source currency; the configured currency is only their fallback when the provider omits it.
- Existing sales and expenses retain their recorded currency. Lists format each record using that currency; totals and analytics include only records matching the current `currency_code`, because PicoFarm does not convert or combine unrelated currencies.
- Frontend monetary formatting is centralized in `web/src/lib/currency.ts` and `CurrencyProvider`. Fields ending in `_cents` use `formatCents`; decimal-unit fields such as `cost_per_kg` and `material_cost` use `formatAmount`.

## High-value domain contracts

### Projects and production

Representative routes:

- `GET /api/projects`
- `POST /api/projects`
- `GET/PATCH/DELETE /api/projects/{id}`
- `GET /api/projects/{id}/jobs`
- `DELETE /api/projects/{id}/jobs`
- `GET /api/projects/{id}/job-stats`
- `GET /api/projects/{id}/summary`
- `POST /api/projects/{id}/start-production`
- `GET/POST /api/projects/{id}/parts`
- `GET/POST /api/projects/{id}/supplies`

Primary response types:

- `Project`
- `PrintJob[]`
- `JobStats`
- `ProjectSummary`
- `StartProductionResult`
- `Part[]` / `Part`
- `ProjectSupply[]` / `ProjectSupply`

### Tasks

Representative routes:

- `GET /api/tasks?project_id=&order_id=&status=`
- `POST /api/tasks`
- `GET/PATCH/DELETE /api/tasks/{id}`
- `PATCH /api/tasks/{id}/status`
- `GET /api/tasks/{id}/progress`
- `POST /api/tasks/{id}/start|complete|cancel`
- `GET /api/tasks/{id}/checklist`
- `POST /api/tasks/{id}/checklist/regenerate`
- `PATCH /api/tasks/{id}/checklist/{itemId}`
- `POST /api/tasks/{id}/checklist/{itemId}/print`

Primary response types:

- `Task`
- `TaskChecklistItem[]`
- `{ progress: number }`
- `PrintJob`

### Parts, designs, and files

Representative routes:

- `GET/PATCH/DELETE /api/parts/{id}`
- `GET/POST /api/parts/{id}/designs`
- `GET/DELETE /api/designs/{id}`
- `GET /api/designs/{id}/download`
- `GET /api/designs/{id}/print-jobs`
- `POST /api/designs/{id}/open-external`
- `GET /api/files/{id}`
- `POST /api/files`

Primary response types:

- `Part`
- `Design`
- `Design[]`
- `PrintJob[]`
- `FileRecord`

Uploads use `multipart/form-data`; do not force `Content-Type: application/json` for `FormData` bodies.

### Printers and hardware control

Representative routes:

- `GET/POST /api/printers`
- `GET /api/printers/states`
- `POST /api/printers/discover`
- `GET /api/printers/default`
- `POST /api/printers/emergency-stop`
- `GET/PATCH/DELETE /api/printers/{id}`
- `GET /api/printers/{id}/state`
- `GET /api/printers/{id}/capabilities`
- `POST /api/printers/{id}/reconnect`
- `POST /api/printers/{id}/maintenance`
- `POST /api/printers/{id}/default`
- `POST /api/printers/{id}/macro`
- `POST /api/printers/{id}/emergency-stop`
- `POST /api/printers/{id}/speed|fan|led|skip-object|jog|temperature|plate-cleared`
- `POST /api/printers/{id}/ams/load|unload|refresh|backup`
- `GET /api/printers/{id}/analytics`
- `GET /api/printers/{id}/job-stats`

Primary response types:

- `Printer`
- `PrinterState`
- `PrinterCapabilities`
- `PrinterMacro`
- `PrinterAnalytics`

`GET /api/printers/{id}/analytics` returns `utilization` (7d/30d/90d), `roi`, `health`, and
`preparation`. `preparation` measures the setup overhead between a job's first
`phase_changed` event and its first phase named `Printing`, aggregated per phase, and is
scoped to jobs completed in the last 90 days. It reports `total_jobs` and
`data_coverage_percent` so a low-coverage result is distinguishable from real data:
`recent_jobs[].job_name` is the design's file name, and `measured_jobs` counts only jobs
that had both a start event and a `Printing` marker.

A failure in any of the underlying repository queries returns `500`, not a zeroed metric.
The previous behavior degraded `revenue_cents` to `0` and `failure_breakdown` to an empty
map on error, which made an unavailable database indistinguishable from a printer with no
history.

`Printer.default_print_folder` is an optional remote directory relative to the printer's G-code root. Empty means the protocol's normal root. Inputs such as `/sda1` are canonicalized to `sda1`; traversal and URL-like values are rejected. Queue, direct print-job, and project-production starts use this value for Moonraker and OctoPrint uploads.

These endpoints can affect real hardware. Preserve safety checks, confirmation flows, and fake/manual-printer test coverage when changing them.

### Printer files and G-code library

Representative routes:

- `GET /api/printers/{id}/files`
- `POST /api/printers/{id}/files/upload`
- `GET /api/printers/{id}/files/metadata|thumbnail|download`
- `DELETE /api/printers/{id}/files`
- `POST /api/printers/{id}/files/mkdir|rename|move|print`
- `GET/POST /api/gcode-library`
- `POST /api/gcode-library/save-from-printer`
- `GET/POST /api/gcode-library/tags`
- `PATCH/DELETE /api/gcode-library/{id}`
- `POST /api/gcode-library/{id}/add-to-queue|send-to-printer`

Primary response types:

- `PrinterFileList`
- `PrinterFileMetadata`
- `GCodeLibraryFile`
- `QueueItem`

### Queue and print jobs

Representative routes:

- `GET /api/queue`
- `POST /api/queue/upload`
- `POST /api/queue/from-print-job/{id}`
- `PATCH/DELETE /api/queue/{id}`
- `POST /api/queue/{id}/preflight|start|status`
- `PATCH /api/queue/{id}/priority`
- `GET/POST /api/print-jobs`
- `GET/PATCH/DELETE /api/print-jobs/{id}`
- `GET /api/print-jobs/{id}/preflight`
- `POST /api/print-jobs/{id}/start|pause|resume|cancel|outcome|retry|failure|scrap`
- `GET /api/print-jobs/{id}/events|with-events|retry-chain`
- `PATCH /api/print-jobs/{id}/priority`

Primary response types:

- `QueueResponse`
- `GCodeQueueItem` / queue item wrappers
- `PrintJob`
- `JobEvent`
- `PrintOutcome`

### Orders, customers, and quotes

Representative routes:

- `GET/POST /api/orders`
- `GET /api/orders/counts`
- `GET/PATCH/DELETE /api/orders/{id}`
- `PATCH /api/orders/{id}/status`
- `GET /api/orders/{id}/progress`
- `POST /api/orders/{id}/ship`
- `POST /api/orders/{id}/items`
- `DELETE /api/orders/{id}/items/{itemId}`
- `POST /api/orders/{id}/items/{itemId}/process`
- `GET/POST /api/customers`
- `GET/PATCH/DELETE /api/customers/{id}`
- `GET/POST /api/quotes`
- `GET/PATCH/DELETE /api/quotes/{id}`
- `POST /api/quotes/{id}/send|accept|reject`
- `POST /api/quotes/{id}/options`
- `PATCH/DELETE /api/quotes/{id}/options/{optionId}`
- `POST /api/quotes/{id}/options/{optionId}/items`
- `PATCH/DELETE /api/quotes/{id}/options/{optionId}/items/{itemId}`
- `GET /api/public/quotes/{token}`

Primary response types:

- `Order`, `OrderItem`, `OrderProgress`, order counts
- `Customer`
- `Quote`, `QuoteOption`, `QuoteLineItem`, `QuoteEvent`
- public quote response for share-token access

### Financial stats

Representative routes:

- `GET /api/stats/financial`
- `GET /api/stats/time-series`
- `GET /api/stats/expenses-by-category`
- `GET /api/stats/sales-by-channel`
- `GET /api/stats/sales-by-project`
- `GET /api/sales/profit-summary`
- `GET /api/sales/weekly-insights`
- `GET /api/stats/usage`

Primary response types:

- `FinancialSummary`
- `TimeSeriesData`
- `CategoryBreakdown`, `ChannelBreakdown`
- `ProjectSales`
- `WeeklyInsights`

`FinancialSummary` carries `currency` (the configured currency the revenue and expense
aggregates are scoped to), `cost_basis_currency`, and `profit_mixed_currency`. Cost
figures have no currency column of their own — see the currency configuration section
above for what the API does about it. `confirmed_expense_count` and
`pending_expense_count` are both scoped to the same currency, so they always describe
the same population.

`WeeklyInsights.PendingRevenueCents` is an estimate: it uses `projects.price_cents`
when set and falls back to the average gross of past sales for that project, scoped to
the configured currency. `projects.price_cents` has no currency, so this figure stays
in whatever currency the project price was entered in.

### Integrations

Representative route groups:

- Generic sales-channel routes implemented so far: `GET /api/sales-channels`, `GET /api/sales-channels/{channel}`, `POST /api/sales-channels/{channel}/sync`, `GET /api/sales-channels/sync-runs`, `GET /api/sales-channels/orders`, `POST /api/sales-channels/orders/{id}/process`, `GET /api/sales-channels/products`, `POST /api/sales-channels/products/{id}/link`, `DELETE /api/sales-channels/products/{id}/link`, `POST /api/sales-channels/{channel}/webhook`, and `GET /api/sales-channels/{channel}/webhook-events` (see `docs/SALES_CHANNELS.md`). Webhook event listings return metadata only and omit stored `payload`/`signature`.
- Planned generic sales-channel connection routes: `POST /api/sales-channels/{channel}/connect`, `POST /api/sales-channels/{channel}/disconnect`, `GET /api/sales-channels/{channel}/auth-url`, and `GET /api/sales-channels/{channel}/callback`. These must preserve snake_case, capability errors, OAuth state validation, and secret-redaction semantics before implementation.
- `/api/integrations/etsy/*`
- `/api/integrations/squarespace/*`
- `/api/integrations/shopify/*`
- `/api/bambu-cloud/*`
- `GET`/`PUT /api/dispatch/printers/{id}/dispatch-settings` — per-printer auto-dispatch settings (enabled, require_confirmation, auto_start, timeout_minutes, macro_auto_dispatch_enabled, macro_empty_queue_gcode). Registered because the handlers and tests existed while the routes did not; the frontend calls this exact path.

Primary contracts live in:

- `internal/saleschannel/*.go` for provider-neutral descriptors, capabilities, connection status, sync results, external orders/products, sync runs, and product links
- `internal/service/sales_channel_adapters.go` for the current legacy-backed Etsy/Squarespace/Shopify provider adapters
- `internal/api/sales_channel_handler.go` for generic HTTP wrappers covering descriptors/status, sync, sync-run listing, read-model lists, process-order, and product link/unlink
- `internal/model/etsy.go`
- `internal/model/squarespace.go`
- Shopify/Bambu model shapes in `internal/model/models.go`
- matching TypeScript types in `web/src/types/index.ts`

Integration endpoints must be testable without real credentials. Use fake clients or conversion tests for CI.

Sales-channel work should move toward provider descriptors, capabilities, generic connection status, external orders/products, product links, sync runs, and optional webhook events as documented in `docs/SALES_CHANNELS.md`. Keep legacy integration route contracts working until a dedicated migration/deprecation cycle explicitly changes them. Mercado Livre uses channel ID `mercado_livre`, OAuth + `offline_access` for refresh tokens, test users instead of a sandbox, and fake-client fixtures for CI. Shopee uses channel ID `shopee`, OAuth shop authorization plus signed HMAC-SHA256 V2 API calls, initial `orders_read` and `products_read` capabilities, fake-client-backed canonical order/product sync, post-MVP `inventory_write`/`webhooks`, fake-client/sandbox validation, and no real credentials in CI.

**Channels actually wired in the production registry** (`internal/service/service.go`, `mustNewSalesChannelRegistry`): Etsy, Squarespace, Shopify, Mercado Livre and Shopee. Of those, Etsy, Squarespace and Shopify have a real client or settings-backed credential, so `Status` can connect. Mercado Livre and Shopee are registered as descriptor-only shells: their providers have no credential source in this build, so `Status` reports disconnected and `Sync`/`LinkProduct`/`ProcessOrder` fail closed with `errSalesChannelReadModelPending`. Treat them as non-connectable until a client and settings keys are wired into the registry. OLX is deliberately **not** registered — `docs/SALES_CHANNELS.md` records the product decision to keep it hidden until OLX partner/integrator API access is practical. Consequence to be aware of: the `olx_api_key` setting and its Settings screen field currently feed nothing. The parity test `TestProductionRegistry_ExposesWiredChannels` pins this set so a channel cannot silently appear or disappear again.

### Notifications, alerts, backups, settings, and feedback

Representative route groups:

- `/api/notifications/*`
- `/api/alerts/*`
- `/api/backups/*`
- `/api/settings/*`
- `/api/feedback/*`

Primary response types:

- `NotificationChannel`, `NotificationDelivery`, `NotificationTemplate`, `NotificationPreview`
- `Alert`, alert counts
- backup list/config responses
- settings key/value responses
- `Feedback`

Backups and settings are operationally sensitive; document validation in `docs/REGRESSION_MATRIX.md` and security assumptions in `docs/SECURITY_MODEL.md` when changing them.

## Contract drift found during this cycle

This cycle inspected the router, Go models, frontend client, and TypeScript types and corrected two concrete drift points:

1. `ConnectionType` now includes backend value `chitu` in TypeScript.
2. `Printer` now includes backend field `min_material_percent` in TypeScript.
3. Stale frontend-only `projectsApi.markReadyToShip` and `projectsApi.ship` helpers were removed because there are no matching `/api/projects/{id}/ready-to-ship` or `/api/projects/{id}/ship` routes. Shipping is currently modeled on orders through `POST /api/orders/{id}/ship`.

Known follow-up area: the frontend `PrintJobStatus` type still includes `sending`, which appears to be used as UI-only state in presentation utilities. If it should not be a backend contract value, split it into a backend `PrintJobStatus` and a UI display status in a later cleanup.

## Change checklist

Before merging an API/data-contract change:

1. Update Go model or handler-local request/response struct.
2. Update TypeScript type in `web/src/types/index.ts`.
3. Update frontend API helper in `web/src/api/client.ts`.
4. Add or update backend handler/service tests for behavior and JSON shape.
5. Run `go test -v ./...`.
6. Run `cd web && npm run lint`.
7. Run `cd web && npm run build`.
8. Update this document and `docs/REGRESSION_MATRIX.md` if the affected workflow is critical.
9. Update `docs/SECURITY_ENDPOINTS.md` if the route is security-sensitive.
