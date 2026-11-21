# Slicer refinements and validation boundaries

The refinements below describe local implementation, not deployment or physical printing. Strict quantity/rotation and upstream toolpath-preview integration is implemented locally in PicoFarm; see [API_CONTRACTS.md](API_CONTRACTS.md#strict-geometry-and-toolpath-preview--integration-in-progress). [SLICING_FEASIBILITY.md](SLICING_FEASIBILITY.md) preserves the earlier investigation; its snapshot findings are not newly verified current behavior.

Local validation: Go tests (`go test ./...`), targeted service/API race tests, 64 frontend tests and production frontend build passed. Lint reports zero errors and the existing 29 warnings. Chromium smoke with mocked APIs passed quantity constraints, explicit-zero rotation, SVG layer navigation, partial warnings, stale previews, 390px dialog width, keyboard focus/Escape and existing job/editor flows (34 intercepted requests, no page errors). WebGL2 was unavailable: only the source-STL viewer error fallback was checked. Real end-to-end PicoFarm→Orca slicing, browser peak memory, screen-reader use and sustained load were not validated. The main frontend chunk remains large (~1126 kB minified / 261 kB gzip).

## Strict integration acceptance gates

These are required checks, not recorded passing results. Upstream live validation and PicoFarm proxy/UI validation must be reported separately.

- Quantity defaults to one; accept only integers `1`–`512`. Cover omitted/explicit one, `512`, zero, negative, fractional and oversized values. Multiple instances require arrangement and remain on one plate; surface no-fit failure without silent quantity reduction or plate splitting.
- Preserve optional finite `rotate`, `rotate_x`, `rotate_y`, including explicit zero, through preview, synchronous slice and queued slice. Any supplied angle disables UI auto-orient; reject contradictory `orient: true`. Verify upstream `rotateX`/`rotateY` names and omit unset values.
- Keep preview/slice geometry and overrides identical. Changing quantity, positioning, model or profiles invalidates stale preview results; an older response must not replace the active configuration's preview.
- Render upstream layer segments as SVG 2D, not inferred STL placements. Exercise layer navigation, empty/null geometry, warnings, `sourceComplete: false`, `truncated: true` and omitted-segment counts. Partial results must remain labeled partial; thumbnail is not a substitute for unavailable toolpaths.
- Keep thumbnail and advanced positioning separately collapsed by default; expanding/collapsing must not change settings or silently trigger slicing. Check labels, keyboard use and narrow-screen overflow in both supported languages.
- Test typed proxy response preservation, explicit-zero forwarding, upstream failure feedback and queue persistence with controlled HTTP fixtures. Measure large bounded previews and UI responsiveness separately; no resource/load benchmark is established here.
- Use isolated compute/import fixtures only. Preview must not create jobs, consume inventory or send hardware commands; slicing imports only and must not start printing.

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
