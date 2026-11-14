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
| Global currency display | One Settings choice formats internal costs, revenue, profit, expenses, sales, analytics, and public quotes without converting stored values | Settings service/API tests for default, normalization, validation, persistence, and public exposure; frontend lint/build plus hardcoded-currency audit | Change USD to BRL in Settings, verify dashboard/materials/projects/printers/quotes/expenses/sales and a public quote update; confirm external-channel orders retain their own currency |
| Task checklist and progress | Operational tracking | Service tests for status transitions and checklist updates | Start task, update checklist, complete/cancel |
| Print job lifecycle | Printer queue safety and reporting | Model/service tests for allowed transitions and outcome/retry | Create job, assign printer, pause/resume/cancel/record outcome |
| Print archive production | Terminal jobs are the only source of the print history; a missed or duplicated archive corrupts the Archives page, comparison, and CSV export | Service tests for archiving on every terminal transition (outcome, failure, cancel, upload failure), one-archive-per-job idempotency, backfill of pre-existing terminal jobs, and archives surviving job deletion | Record a failure, cancel a job, record a successful outcome, reload the Archives page, export CSV; restart the server and confirm the startup backfill reports nothing new |
| Queue dispatch | Automates what printer should run next | Service tests for priority, pending requests, disabled printers, compatible jobs | Add queue item, preflight/start/status |
| GCode/STL library | Files feed production and printer actions | Storage/API tests for upload, tags, default selection, delete | Upload file, tag it, send/add to queue |
| Files screen material label | Each G-code shows its filament type as a colored chip; a NULL `material_type`/`material_color` row (restored backup, hand edit) used to fail the whole listing with a scan error, and a type outside the hardcoded filter list was unreachable by search | Repository test asserting `List` survives NULL material columns; settings service tests for `filament_type_colors` validation and key normalization; frontend build plus color-resolution checks (setting > built-in palette > neutral) | Upload G-codes with PLA, PETG and an unusual type, confirm three chips with distinct colors, filter by the unusual type in the Material dropdown, change one color in Settings → Other and confirm the chip changes without a reload |
| Slicer connection | The UI must not hang on a slicer that is down, and a deployment must be able to pin the container address | Service tests for environment precedence, fallback, malformed environment value, persistence while overridden, the quick-call deadline against an unresponsive server, and the three distinct client timeouts | With the slicer stopped, open Slicer Settings and confirm health/profiles fail in seconds instead of hanging; set `SLICER_CONNECTION_URL`, reload and confirm the page shows the override notice and the stored URL is still kept |
| Slicer profile backup/restore | The slicer has no export endpoint and no way to delete a profile, so the bundle is the only recovery path for a recreated container or lost volume; a partial or silently-empty export looks like a complete backup, and a restore that overwrites without being asked destroys working profiles | Service tests for bundle composition from list+read (exact JSON bytes, version, source, slicer version), export failure on an unreadable profile, unknown-category rejection, skip-without-overwrite, overwrite, bundle validation (invalid JSON, missing name/json, unknown category, duplicates) rejecting before any upload, and per-profile failure reporting; handler tests for the empty-body export, the malformed-bundle `400`, and failures reported with `200` | Export a backup, confirm the file lists every category and the JSON matches what Edit shows for one profile; recreate the slicer container (or rename a profile), restore without overwrite and confirm existing profiles are skipped; restore with overwrite and confirm the bytes land; feed a truncated file and confirm it is refused before anything is uploaded |
| Printer connection/status | Hardware-facing and safety-sensitive | Fake printer client tests; no real hardware in CI | Register manual/fake printer, observe state UI |
| Automatic printer reconnect | An unreachable printer must not stall the rest: `Manager.Connect` used to hold the manager mutex for the whole network call, so one slow printer froze every status read and every other connection attempt; the retry rate was also flat (a printer that is switched off was retried every ~15s forever, and `chitu` printers, which have no client, were retried every pass) | Manager lock tests asserting a second `Connect` and `GetAllStates` return while another printer is blocked inside network I/O, plus replacement/no-leak, unsupported type, maintenance, and post-`DisconnectAll` behavior; service tests for the skip rules, the exponential/jittered/capped delay, backoff deferral, deferral expiry, and backoff clearing on success | Take a farm printer offline, watch it come back on its own and check the log shows decaying retries instead of a line every few seconds; with the printer still offline, confirm the files/queue screens keep loading instantly |
| Network scan for printers | The scan used to always emit .1–.254 regardless of the mask, so a /16 missed every printer outside one /24 and a /28 probed addresses that are not on the local link; the handler used `context.Background()`, so a cancelled request kept probing the LAN; and in a container on a bridge network the first non-loopback interface belongs to the container, so the scan found nothing | Range tests across /24, /25, /26, /27, /28, /29, /30 plus empty results for /31 and /32, a cap test asserting a truncated /8 stays inside the network and is ordered, duplicate detection, and CIDR parsing for `PICOFARM_SCAN_NETWORK` (including rejection of a malformed value) | Run the backend with `PICOFARM_SCAN_NETWORK` set to the LAN subnet and confirm Discover finds printers; cancel the request mid-scan and check the log reports an early stop instead of probing to the end |
| Notifications | Customer/operator communication | Template rendering, delivery recording, and multipart media tests | Preview/test delivery; for print lifecycle events verify the G-code thumbnail and text arrive in one message |
| Printer default upload folder | Remote G-code placement | Path normalization, migration/repository round-trip, Moonraker multipart path, OctoPrint path/start tests | Configure `/sda1`, start from Queue, and verify the file is uploaded and started as `sda1/<file>`; empty configuration must use root |
| Profit under mixed cost basis | Cost sources (materials, printers, supplies) carry no currency, so profit is only meaningful when the cost basis matches the configured currency | `FinancialSummary` currency/cost-basis/flag test; repository currency-inference test | Set `currency_code` to BRL with USD expenses, confirm the dashboard shows `—` for net profit and names both currencies instead of a plausible-looking number |
| Printer preparation analytics | Setup overhead per printer job, derived from `phase_changed` events | Repository test covering the 90-day window, `completed_at` filter, and `job_name` = design file name; service test for phase aggregation | Run several jobs on a printer, open its analytics, and confirm the preparation breakdown and coverage percentage reflect the window |
| Printer analytics failure surfacing | A broken query must not look like "no data" | Service test asserting `500` instead of a zeroed metric on repository error | Point the printer analytics query at a failing repository and confirm the endpoint errors |
| Backup/restore | Data recovery | Temp DB tests for backup creation, retention, restore safety | Create backup, verify restore procedure in temp data |
| Sales-channel sync and product links | External source of orders/listings/products from Etsy, Squarespace, Shopify, and future providers | Provider registry tests, fake-client conversion/sync tests, repository idempotency tests, and handler tests for generic `/api/sales-channels/*` routes, including sync-run redaction | Sync with sandbox/mock only; do not require credentials; verify the Channels page shows status/errors without secrets |
| WebSocket status | UI freshness | Hook/unit or integration test for reconnect/cleanup when feasible | Load app, observe connected status and live updates |
| Auth/security boundary | Prevent unsafe remote exposure | Router/CORS tests and endpoint policy tests | Verify allowed/bad origins and sensitive actions |
| API token lifecycle | Named credentials are how the optional auth is used; a broken revoke leaves a live credential | `internal/service/api_token_test.go` (hash-only storage, immediate revoke, scope matching), `internal/api/api_token_handler_test.go` (auth matrix), schema tests for the partial unique index | Create a token, use it on its scope, revoke it, confirm the next request is `401`/`403` |
| WebSocket origin allowlist | `/ws` is outside `/api`; the allowlist is its only always-on check | `internal/realtime/hub_test.go` covers real handshakes: loopback, configured, same-origin, no-Origin and rejected origins, plus token-on/token-off | Connect from the configured origin, confirm a disallowed origin fails the handshake |

## Regression selection guide

- Database schema or migration change: run fresh DB tests, upgrade tests if available, `go test -v ./internal/database ./internal/repository ./internal/service`, then full Go tests.
- API route/response change: add/update handler tests and check matching TypeScript types.
- Frontend API client change: run `cd web && npm run lint`, `cd web && npm run build`, and manually exercise the affected page.
- Sales-channel change: read `docs/SALES_CHANNELS.md`, use fake clients/fixtures, test idempotent sync and secret redaction, verify generic read-model routes omit provider `raw_json`, verify `GET /api/sales-channels/sync-runs` returns sanitized diagnostic errors, verify generic webhook events are idempotent and `GET /api/sales-channels/{channel}/webhook-events` omits stored payload/signature, verify process/link/unlink routes mutate canonical storage idempotently or reject duplicates clearly, then update API/security docs if routes or JSON change.
- Sales-channel connect/auth change: test unsupported capability paths, missing provider-specific fields, OAuth state mismatch/replay where applicable, sanitized callback errors, and redaction for `api_key`, `client_secret`, `code`, access/refresh tokens, webhook secrets, and signed headers. Mercado Livre QA must use fake-client fixtures in CI and optional test users for manual checks; there is no separate sandbox. Shopee QA must prove HMAC base-string/signature behavior with fake values, use sandbox/manual seller authorization only outside CI, and confirm Brazil/account partner access before enabling live capabilities.
- Shopee MVP implementation: keep descriptor plus `orders_read`/`products_read` only, keep `inventory_write` and `webhooks` hidden until their gates pass, validate item/model/SKU mapping, idempotent canonical order/product sync, Shopee variant link/unlink via generic product-link routes, linked filtering, and `raw_json` omission with fake fixtures; verify pagination/rate-limit errors are sanitized through sync-run diagnostics.
- OLX implementation: validate official integrator access and target category/scope before code, use only official APIs or manual/import fallback, start with ads/products plus lead webhook/inbox semantics, do not advertise `orders_read` unless order semantics are documented for the account, and cover OAuth/ad-import/lead-webhook paths with fake fixtures plus secret/PII redaction.
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
