# Slicer refinements and validation boundaries

These are implemented local changes, not evidence of deployment or physical printing.

## API contracts

- `POST /api/slicer/profiles/validate` takes `{category,name,json}` and returns `{valid,errors,warnings}`. `json` is a string containing a profile object. Preflight checks common structural/types/range errors, not full Orca inheritance or physical safety.
- `GET /api/slicer/profiles/{category}/{name}/history` returns `{current_json,current_hash,revisions}`. Revision fields: `{id,hash,json,created_at}`. Response: latest 50 revisions, at most 4 MiB.
- `POST /api/slicer/profiles/revisions` takes `{category,name,json,expected_hash}` and returns refreshed history. Hash conflict is `409`; invalid preflight is `400`. Maximum snapshot: 1 MiB; global stored JSON quota: 64 MiB.
- Background job routes, statuses, limits and restart semantics: [SLICER_JOBS.md](SLICER_JOBS.md). CORS permits `Idempotency-Key`.
- Preview and slice forward explicit booleans, camelCase upstream brim/sequence fields and overrides. Slice imports G-code only. Outgoing multipart is spooled to a request-scoped temporary file, not a second model-sized heap buffer.

## Architecture and security

`internal/service/slicer.go` owns upstream calls/import. `slicer_jobs.go` wraps this path in a persisted local worker; server startup/close owns its lifetime before database shutdown. It does not serialize synchronous preview callers or use upstream async jobs.

`slicer_profile_history.go` and its repository counterpart store immutable recovery JSON in the canonical `slicer_profile_history` table. Snapshots can contain custom machine G-code; treat database, backups and history endpoints as sensitive. Source URLs are hashed for partitioning. Only editor saves create snapshots; imports, source sync and direct upstream edits are not automatically versioned. Attempted saves are included, not only confirmed uploads. Current profile must remain readable for this recovery UI; a recreated volume still requires the profile bundle.

There is no automatic history deletion or quota-maintenance endpoint. Exhausting the quota blocks editor saves rather than silently deleting recovery data. All PicoFarm profile mutations share a process-local lock; each operation pins its target connection. The editor stores the preimage/proposal pair transactionally before upload. The lock serializes even different profiles and waiting for it is not context-cancellable. Multiple instances and direct upstream edits remain outside this protection.

The queue can send models to the configured slicer and import files after browser navigation. Keep the slicer private and run one worker instance per database. Generic settings endpoints cannot read/write/delete the internal job snapshot. Removing terminal history removes its deduplication key, not the G-code.

Viewer budgets (32 MiB / 500,000 triangles) are preview limits, not upload hardening. STL has no unit; mm is explicitly assumed. Parsing remains synchronous; large ASCII inputs may stall the main thread. Render is on demand with cleanup and canonical views.

## Regression gates

| Workflow | Automated evidence |
| --- | --- |
| Preview/slice | `slicer_contract_test.go`: true/false, parity, overrides, upstream keys, G-code-only output |
| Library | `gcode-library-view.test.mjs`: global sorting, missing/default metadata, group pagination, stale preview and response gates |
| STL | `stl-viewer.test.mjs`: dimensions, invalid geometry, byte/triangle limits, camera poses |
| Profiles | Service/repository/handler/schema tests: validation, conflict, restore, failed upload, quota, fresh/upgrade DB; `slicer-settings-editor.test.mjs`: actual handlers without DOM, no autosave and draft preservation |
| Jobs | `TestSlicerJobs*`: queue/concurrency, idempotency, cancel refusal, restart interruption, persistence failure, redaction and terminal deletion; settings isolation and frontend retry/polling tests |

`web/tests/slicer-browser-smoke.cjs` also exercises comparison, stale preview, queued cancellation/terminal removal and editor save/conflict in real Chromium with mocked APIs. Configure `PLAYWRIGHT_MODULE`, `CHROMIUM_EXECUTABLE` and `BASE_URL` for an isolated server. It requires WebGL by default; `ALLOW_NO_WEBGL=1` explicitly skips geometry/camera assertions if unavailable, checking the error fallback instead. In the local validation WebGL2 was unavailable; dimensions/cameras were not browser-validated.

Tests use temporary databases, synthetic models and controlled HTTP fixtures. Real Orca compatibility, physical output, RSS/disk/load benchmarks and device-specific WebGL behavior are not established by these tests. Cache changes in the companion slicer repository were exercised with a synthetic CLI. The frontend still emits a large-chunk warning; route-level optimization remains pending.
