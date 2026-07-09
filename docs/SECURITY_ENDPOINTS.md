# Sensitive Endpoint Inventory

This inventory classifies PicoFarm routes that carry security or operational risk. It is based on the current router in `internal/api/router.go` and should be updated whenever routes, authorization, hardware control, file handling, backups, settings, integrations, or public access change.

## Current policy

PicoFarm currently has CORS restrictions and baseline browser security headers, but it does not implement an application-level authentication/authorization boundary in the router. Treat every route below as available to any client that can reach the backend process unless a trusted reverse proxy or network boundary protects it.

Before exposing PicoFarm outside a trusted local network, protect these routes with one of:

1. a documented trusted reverse proxy with authentication and TLS; or
2. the optional application-level auth middleware (set `API_TOKEN` in the environment).

## Risk classes

| Class | Risk | Required care when changing |
| --- | --- | --- |
| Public | Intentionally unauthenticated or browser entrypoint | Keep payloads minimal; avoid leaking private/customer data |
| Read | Operational or business data disclosure | Check pagination/filtering and avoid exposing secrets |
| Write | Persistent data mutation | Add handler/service tests for validation and error paths |
| Hardware control | Real printer/device action | Use fake/manual printers in CI; avoid real hardware unless explicitly requested |
| File operation | Upload, download, delete, move, or generated files | Validate path confinement, cleanup, size handling, and content-type behavior |
| Backup/restore | Data export, deletion, or rollback | Test in temp directories/DBs; avoid unsafe paths and stale restores |
| Settings/secrets | Runtime configuration or credentials | Never log/return raw secrets; redact examples as `[REDACTED]` |
| Integration/webhook | External account access or inbound event processing | Use fake clients; verify webhook signatures where provider supports them |

## Public and unauthenticated surface

| Routes | Class | Notes |
| --- | --- | --- |
| `GET /health` | Public | Status/version endpoint. Do not add secrets, environment dumps, or internal paths. |
| `GET /ws` | Public/realtime | Realtime channel. Treat as sensitive until auth policy exists. |
| `GET /api/public/quotes/{token}` | Public | Share-token quote access. Tokens should remain unguessable and responses should stay scoped to the quote. |
| `GET /api/public/business-info` | Public | Business profile fields only; do not include private settings or credentials. |
| `GET /*` | Public/frontend | Static SPA fallback when `STATIC_DIR` exists. |

## Hardware and printer-control endpoints

| Routes | Class | Notes |
| --- | --- | --- |
| `POST /api/printers/discover` | Hardware control | Network scan/discovery behavior; avoid broad scans in tests. |
| `POST /api/printers/emergency-stop` | Hardware control | Global emergency stop; high-impact physical action. |
| `POST /api/printers/{id}/reconnect` | Hardware control | Reconnects to device/network service. |
| `POST /api/printers/{id}/maintenance` | Hardware control | Changes printer availability/safety state. |
| `POST /api/printers/{id}/default` | Hardware control | Changes dispatch target defaults. |
| `POST /api/printers/{id}/macro` | Hardware control | Runs configured printer macro; validate macro source/config. |
| `POST /api/printers/{id}/emergency-stop` | Hardware control | Per-printer emergency stop; high-impact physical action. |
| `POST /api/printers/{id}/speed` | Hardware control | Changes runtime printer behavior. |
| `POST /api/printers/{id}/fan` | Hardware control | Changes runtime printer behavior. |
| `POST /api/printers/{id}/led` | Hardware control | Device action. |
| `POST /api/printers/{id}/skip-object` | Hardware control | Affects active print. |
| `POST /api/printers/{id}/jog` | Hardware control | Moves hardware axes. |
| `POST /api/printers/{id}/temperature` | Hardware control | Thermal control; safety-sensitive. |
| `POST /api/printers/{id}/plate-cleared` | Hardware control | Releases/continues automation assumptions. |
| `POST /api/printers/{id}/ams/load` | Hardware control | Material handling action. |
| `POST /api/printers/{id}/ams/unload` | Hardware control | Material handling action. |
| `POST /api/printers/{id}/ams/refresh` | Hardware control | Device query/action. |
| `POST /api/printers/{id}/ams/backup` | Hardware control/settings | Changes material fallback behavior. |

## Printer files, library files, queue, and print jobs

| Routes | Class | Notes |
| --- | --- | --- |
| `POST /api/printers/{id}/files/upload` | File operation/hardware | Uploads to printer storage. Validate path/name/size. |
| `GET /api/printers/{id}/files/download` | File operation/read | Can disclose printer files. Validate path scoping. |
| `DELETE /api/printers/{id}/files` | File operation | Deletes printer-side files. Validate target path. |
| `POST /api/printers/{id}/files/mkdir` | File operation | Creates printer-side directories. |
| `POST /api/printers/{id}/files/rename` | File operation | Validate old/new path confinement. |
| `POST /api/printers/{id}/files/move` | File operation | Validate source/destination path confinement. |
| `POST /api/printers/{id}/files/print` | Hardware control/file operation | Starts print from file; high-impact action. |
| `POST /api/gcode-library/upload` | File operation | Uploads production files into local library. |
| `POST /api/gcode-library/save-from-printer` | File operation | Imports from printer into local storage. |
| `DELETE /api/gcode-library/{id}` | File operation | Deletes local library records/files. |
| `POST /api/gcode-library/{id}/add-to-queue` | Write/queue | Queues a production file. |
| `POST /api/gcode-library/{id}/send-to-printer` | Hardware control/file operation | Sends file to printer. |
| `POST /api/stl-library/upload` | File operation | Uploads model files. Validate 3MF/STL behavior. |
| `POST /api/stl-library/{id}/thumbnail` | File operation | Generated file/write path. |
| `DELETE /api/stl-library/{id}` | File operation | Deletes library object. |
| `POST /api/files` | File operation | Generic upload endpoint. |
| `GET /api/files/{id}` | File operation/read | Generic file download/read endpoint. |
| `POST /api/queue/upload` | File operation/queue | Uploads print queue item. |
| `POST /api/queue/from-print-job/{id}` | Queue/write | Re-queues from existing job. |
| `POST /api/queue/{id}/preflight` | Queue/hardware | Checks readiness against operational state. |
| `POST /api/queue/{id}/start` | Hardware control/queue | Starts queued print; high-impact action. |
| `POST /api/queue/{id}/status` | Queue/write | Changes queue state. |
| `POST /api/print-jobs/{id}/start` | Hardware control | Starts a print job. |
| `POST /api/print-jobs/{id}/pause` | Hardware control | Affects active print. |
| `POST /api/print-jobs/{id}/resume` | Hardware control | Affects active print. |
| `POST /api/print-jobs/{id}/cancel` | Hardware control | Stops/cancels production. |
| `POST /api/print-jobs/{id}/retry` | Queue/hardware | Creates repeat attempt. |
| `POST /api/print-jobs/{id}/failure` | Write/reporting | Records failure; affects metrics/retry flow. |
| `POST /api/print-jobs/{id}/scrap` | Write/reporting | Records material/business loss. |

## Backup, settings, notifications, and dispatch

| Routes | Class | Notes |
| --- | --- | --- |
| `POST /api/backups` | Backup/restore | Creates data export. Avoid leaking location or content. |
| `PUT /api/backups/config` | Backup/restore/settings | Changes backup policy/paths. |
| `DELETE /api/backups/{name}` | Backup/restore | Deletes backup artifacts. Validate name/path. |
| `POST /api/backups/{name}/restore` | Backup/restore | Restores database/state; highest data-loss risk. |
| `GET /api/settings` | Settings/secrets/read | Ensure sensitive values are redacted if secrets are stored as settings. |
| `PUT /api/settings/{key}` | Settings/secrets | Persistent runtime config mutation; may store credentials. |
| `DELETE /api/settings/{key}` | Settings/secrets | Removes runtime config; can disable integrations. |
| `GET /api/settings/thingiverse_token` | Settings/secrets/read | Token-specific endpoint; should not leak secret material unnecessarily. |
| `PUT /api/settings/thingiverse_token` | Settings/secrets | Stores token material. |
| `POST /api/notifications` | Settings/integration | Creates external delivery channel. |
| `POST /api/notifications/templates` | Settings/integration | Template rendering can leak data if misconfigured. |
| `POST /api/notifications/templates/preview` | Integration/read | Preview may render sensitive variables. |
| `POST /api/notifications/{id}/test` | Integration | Sends test notification externally. |
| `PUT /api/printers/{id}/dispatch-settings` | Hardware control/settings | Changes per-printer automation. |
| `PUT /api/dispatch/settings` | Hardware control/settings | Changes global dispatch automation. |
| `POST /api/dispatch/requests/{id}/confirm` | Hardware control/queue | Confirms automated dispatch. |
| `POST /api/dispatch/requests/{id}/reject` | Queue/write | Rejects dispatch request. |
| `POST /api/dispatch/requests/{id}/skip` | Queue/write | Skips dispatch candidate. |

## Customer, sales, orders, quotes, and business data

| Routes | Class | Notes |
| --- | --- | --- |
| `/api/customers/*` | Read/write/business data | Customer PII/business data. |
| `/api/orders/*` | Read/write/business data | Order and fulfillment data. `POST /api/orders/{id}/ship` changes fulfillment state. |
| `/api/quotes/*` | Read/write/business data | Quote pricing/customer data. Send/accept/reject actions can affect sales workflow. |
| `/api/sales/*` | Read/write/business data | Sales and financial data. |
| `/api/expenses/*` | Read/write/business data/file operation | Receipt uploads and expense data. |
| `/api/stats/*` | Read/business data | Aggregated operational/financial data. |
| `/api/feedback/*` | Read/write/business data | User feedback data. |

## Integration and webhook endpoints

Sales-channel routes are designed in `docs/SALES_CHANNELS.md`. Any new generic `/api/sales-channels/*` endpoint that connects accounts, handles OAuth/API keys, syncs external data, links products, updates inventory, processes orders, or receives webhooks belongs in this section and should follow the same secret-redaction and fake-client testing rules as the provider-specific routes below.

| Routes | Class | Notes |
| --- | --- | --- |
| `GET /api/sales-channels` | Integration/read | Provider descriptors/capabilities plus connection status. Do not include secrets or private account data. |
| `GET /api/sales-channels/{channel}` | Integration/read | One provider descriptor/capabilities plus connection status. Redact credential details and external API errors. |
| `POST /api/sales-channels/{channel}/connect` | Integration/secrets/settings | Connects external account/API. Validate provider-specific body and never log credentials. |
| `POST /api/sales-channels/{channel}/disconnect` | Integration | Disconnects provider/connection. |
| `GET /api/sales-channels/{channel}/auth-url` | Integration/OAuth | OAuth start. Validate redirect config and state. |
| `GET /api/sales-channels/{channel}/callback` | Integration/OAuth | OAuth callback. Validate state and errors. |
| `POST /api/sales-channels/{channel}/sync` | Integration/write | Imports external orders/products. Use fake clients in tests and sanitize stored errors. |
| `POST /api/sales-channels/orders/{id}/process` | Integration/write | Converts external order into internal workflow. |
| `POST /api/sales-channels/products/{id}/link` | Integration/write | Links external product/variant to internal project/SKU. |
| `DELETE /api/sales-channels/products/{id}/link` | Integration/write | Unlinks external product/variant. |
| `POST /api/sales-channels/{channel}/webhook` | Integration/webhook | Inbound webhook. Verify signatures where provider supports them. |
| `POST /api/sales-channels/{channel}/webhook-events/{id}/reprocess` | Integration/write | Replays inbound event. Must be idempotent and auditable. |
| `POST /api/bambu-cloud/login` | Integration/secrets | Authenticates external account; never log credentials. |
| `POST /api/bambu-cloud/verify` | Integration/secrets | Verifies auth material. |
| `POST /api/bambu-cloud/devices/add` | Integration/hardware | Adds device from external account. |
| `DELETE /api/bambu-cloud/logout` | Integration/secrets | Clears/disconnects auth state. |
| `PUT /api/integrations/etsy/configure` | Integration/secrets/settings | Stores integration config. |
| `GET /api/integrations/etsy/auth` | Integration/OAuth | OAuth start. Validate redirect config. |
| `GET /api/integrations/etsy/callback` | Integration/OAuth | OAuth callback. Validate state and errors. |
| `POST /api/integrations/etsy/disconnect` | Integration | Disconnects account. |
| `POST /api/integrations/etsy/receipts/sync` | Integration/write | Imports external orders. |
| `POST /api/integrations/etsy/receipts/{id}/process` | Integration/write | Converts receipt into internal workflow. |
| `POST /api/integrations/etsy/listings/sync` | Integration/write | Imports listing data. |
| `POST /api/integrations/etsy/listings/{id}/link` | Integration/write | Links external product to internal project. |
| `DELETE /api/integrations/etsy/listings/{id}/link` | Integration/write | Unlinks external product. |
| `POST /api/integrations/etsy/listings/{id}/sync-inventory` | Integration/write | Pushes/syncs inventory. |
| `POST /api/integrations/etsy/webhook` | Integration/webhook | Inbound webhook. Signature verification should be documented/testable. |
| `POST /api/integrations/etsy/webhook/events/{id}/reprocess` | Integration/write | Replays inbound event. |
| `POST /api/integrations/squarespace/connect` | Integration/secrets | Connects external account/API. |
| `POST /api/integrations/squarespace/disconnect` | Integration | Disconnects account. |
| `POST /api/integrations/squarespace/orders/sync` | Integration/write | Imports external orders. |
| `POST /api/integrations/squarespace/orders/{id}/process` | Integration/write | Converts external order. |
| `POST /api/integrations/squarespace/products/sync` | Integration/write | Imports product data. |
| `POST /api/integrations/squarespace/products/{id}/link` | Integration/write | Links product. |
| `DELETE /api/integrations/squarespace/products/{id}/link` | Integration/write | Unlinks product. |
| `GET /api/integrations/shopify/auth-url` | Integration/OAuth | OAuth start URL. |
| `GET /api/integrations/shopify/callback` | Integration/OAuth | OAuth callback. Validate state and errors. |
| `DELETE /api/integrations/shopify` | Integration | Disconnects account. |
| `POST /api/integrations/shopify/sync` | Integration/write | Imports external orders/products. |
| `POST /api/integrations/shopify/orders/{id}/process` | Integration/write | Converts external order. |
| `POST /api/integrations/shopify/products/{productId}/link` | Integration/write | Links external product. |
| `DELETE /api/integrations/shopify/products/{productId}/link` | Integration/write | Unlinks external product. |

## Route policy checklist

When adding or changing a route:

1. Classify it in this document if it can read private data, mutate state, touch files, control hardware, change settings, use credentials, or interact with an external service.
2. Add or update handler/service tests for validation and failure paths.
3. Avoid real credentials, real printers, and live commerce accounts in tests.
4. Update `docs/API_CONTRACTS.md` if request/response JSON changes.
5. Update `docs/REGRESSION_MATRIX.md` if the route belongs to a critical workflow.
6. Re-run `go test -v ./...`, `make lint`, and `make build` before committing behavior changes.
