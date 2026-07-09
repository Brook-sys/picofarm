# PicoFarm Regression Matrix

Use this matrix to decide what to manually verify or automate when changing PicoFarm. Prefer turning high-risk manual checks into Go API tests, TypeScript tests, or small integration tests over time.

## Validation levels

| Level | Use when | Examples |
| --- | --- | --- |
| Smoke | Docs/scripts/build-only changes or low-risk UI copy | `go test -v ./...`, `cd web && npm run lint`, `make build` |
| Targeted | One domain changed | Relevant Go package tests, one browser/API workflow, build/lint |
| Full | Cross-domain, database, API contracts, printer control, queue, auth/security | Full validation gate plus targeted regression workflows |

## Critical workflows

| Workflow | Why it matters | Suggested automated coverage | Manual smoke check |
| --- | --- | --- | --- |
| Project catalog | Core product definition drives tasks, jobs, costing, files | API tests for create/update/delete/list and summary responses | Create project, edit SKU/pricing, view summary |
| Parts and design versions | File uploads and 3MF parsing affect print jobs and cost estimates | Multipart handler tests with fake files and invalid 3MF | Upload a design, confirm metadata/failure handling |
| Order to task | Main fulfillment pipeline | API tests for manual order, add item, process item into task | Create order, process item, inspect generated task |
| Quote to order | Sales flow with customer/address/line item state | Service/API tests for accept/reject and generated order | Create quote, send/accept, verify order |
| Task checklist and progress | Operational tracking | Service tests for status transitions and checklist updates | Start task, update checklist, complete/cancel |
| Print job lifecycle | Printer queue safety and reporting | Model/service tests for allowed transitions and outcome/retry | Create job, assign printer, pause/resume/cancel/record outcome |
| Queue dispatch | Automates what printer should run next | Service tests for priority, pending requests, disabled printers, compatible jobs | Add queue item, preflight/start/status |
| GCode/STL library | Files feed production and printer actions | Storage/API tests for upload, tags, default selection, delete | Upload file, tag it, send/add to queue |
| Printer connection/status | Hardware-facing and safety-sensitive | Fake printer client tests; no real hardware in CI | Register manual/fake printer, observe state UI |
| Notifications | Customer/operator communication | Template rendering tests and delivery recording tests | Preview/test delivery for a template |
| Backup/restore | Data recovery | Temp DB tests for backup creation, retention, restore safety | Create backup, verify restore procedure in temp data |
| Sales-channel sync and product links | External source of orders/listings/products from Etsy, Squarespace, Shopify, and future providers | Provider registry tests, fake-client conversion/sync tests, repository idempotency tests, and handler tests for generic `/api/sales-channels/*` routes, including sync-run redaction | Sync with sandbox/mock only; do not require credentials; verify the Channels page shows status/errors without secrets |
| WebSocket status | UI freshness | Hook/unit or integration test for reconnect/cleanup when feasible | Load app, observe connected status and live updates |
| Auth/security boundary | Prevent unsafe remote exposure | Router/CORS tests and endpoint policy tests | Verify allowed/bad origins and sensitive actions |

## Regression selection guide

- Database schema or migration change: run fresh DB tests, upgrade tests if available, `go test -v ./internal/database ./internal/repository ./internal/service`, then full Go tests.
- API route/response change: add/update handler tests and check matching TypeScript types.
- Frontend API client change: run `cd web && npm run lint`, `cd web && npm run build`, and manually exercise the affected page.
- Sales-channel change: read `docs/SALES_CHANNELS.md`, use fake clients/fixtures, test idempotent sync and secret redaction, verify generic read-model routes omit provider `raw_json`, verify `GET /api/sales-channels/sync-runs` returns sanitized diagnostic errors, verify generic webhook events are idempotent and `GET /api/sales-channels/{channel}/webhook-events` omits stored payload/signature, verify process/link/unlink routes mutate canonical storage idempotently or reject duplicates clearly, then update API/security docs if routes or JSON change.
- Sales-channel connect/auth change: test unsupported capability paths, missing provider-specific fields, OAuth state mismatch/replay where applicable, sanitized callback errors, and redaction for `api_key`, `client_secret`, `code`, access/refresh tokens, webhook secrets, and signed headers. Mercado Livre QA must use fake-client fixtures in CI and optional test users for manual checks; there is no separate sandbox. Shopee QA must prove HMAC base-string/signature behavior with fake values, use sandbox/manual seller authorization only outside CI, and confirm Brazil/account partner access before enabling live capabilities.
- Shopee MVP implementation: keep descriptor plus `orders_read`/`products_read` only, keep `inventory_write` and `webhooks` hidden until their gates pass, validate item/model/SKU mapping, idempotent canonical order/product sync, Shopee variant link/unlink via generic product-link routes, linked filtering, and `raw_json` omission with fake fixtures; verify pagination/rate-limit errors are sanitized through sync-run diagnostics.
- Printer control or queue change: use fake/manual printers only unless the user explicitly asks for real hardware testing.
- Upload/delete/backup change: test in temp directories and verify cleanup behavior.
- Security/CORS/endpoint-policy change: add router tests and document expected local/self-hosted behavior, including `docs/SECURITY_ENDPOINTS.md` for sensitive routes.

## Current automation gaps to close

1. Fresh database schema assertion covering all required current tables/columns.
2. Upgrade/partial-upgrade database tests around legacy compatibility logic.
3. HTTP JSON contract tests for project/task/order/queue/printer/notification responses.
4. Frontend tests or smaller component seams for large pages currently validated mostly through build/lint.
5. Fake printer integration tests for unsafe actions such as print/start/cancel/emergency stop.

## Reporting template

When finishing a cycle, include:

- Workflows affected.
- Validation level used: smoke, targeted, or full.
- Commands run and real outcome.
- Any manual check performed.
- Remaining automation gap, if any.
