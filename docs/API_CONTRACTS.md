# PicoFarm API and Data Contracts

This document records the current HTTP/JSON contract conventions between the Go backend and the React/TypeScript frontend. It is based on the current router in `internal/api/router.go`, Go models in `internal/model/`, and browser client/types in `web/src/api/client.ts` and `web/src/types/index.ts`.

## Contract sources of truth

| Layer | Source |
| --- | --- |
| Route table | `internal/api/router.go` |
| Backend JSON shapes | `internal/model/*.go` plus handler-local request/response structs |
| Frontend API calls | `web/src/api/client.ts` |
| Frontend domain/API types | `web/src/types/index.ts` |
| Database schema | `migrations/*.sql` plus compatibility logic in `internal/database/sqlite.go` |

When an endpoint or JSON shape changes, update the backend model/handler, frontend API client, frontend type, tests, and this document in the same cycle.

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

## JSON conventions

- Field names are snake_case JSON keys from Go struct tags.
- Go `uuid.UUID` values are serialized as strings; TypeScript represents them as `string`.
- Go `time.Time` and `*time.Time` values are serialized as timestamp strings; TypeScript represents them as `string` and optional `string` respectively.
- Go `omitempty` fields may be absent from responses. Frontend types should mark those as optional or nullable to match actual JSON.
- Monetary integer fields ending in `_cents` are cents, not decimal dollars.
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

Do not add frontend-only enum values unless the backend explicitly returns them or the type is clearly marked as UI-local.

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

Primary response types:

- `Printer`
- `PrinterState`
- `PrinterCapabilities`
- `PrinterMacro`

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

### Integrations

Representative route groups:

- `/api/integrations/etsy/*`
- `/api/integrations/squarespace/*`
- `/api/integrations/shopify/*`
- `/api/bambu-cloud/*`

Primary contracts live in:

- `internal/model/etsy.go`
- `internal/model/squarespace.go`
- Shopify/Bambu model shapes in `internal/model/models.go`
- matching TypeScript types in `web/src/types/index.ts`

Integration endpoints must be testable without real credentials. Use fake clients or conversion tests for CI.

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
