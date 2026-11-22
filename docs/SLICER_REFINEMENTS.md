# Slicer refinements and validation boundaries

The refinements below describe local implementation, not deployment or physical printing. Strict quantity/rotation and full 3D upstream toolpath preview are implemented locally in PicoFarm; see [API_CONTRACTS.md](API_CONTRACTS.md#strict-geometry-and-toolpath-preview--local-implementation). [SLICING_FEASIBILITY.md](SLICING_FEASIBILITY.md) preserves the earlier investigation; its snapshot findings are not newly verified current behavior.

Local validation: the full Go/frontend test helper, targeted Go service/API race tests, `go vet ./...`, 71 frontend tests, production frontend build and backend health check passed. Lint passed with zero errors and the existing 29 warnings. Synthetic Chromium smoke passed both WebGL2/SwiftShader rendering and the explicit no-WebGL 2D fallback, including 390px dialog width, filters, layer cuts, camera controls and existing dialog/job/editor flows. Three real upstream payloads also passed the isolated browser smoke described below. These are software-rendered observations, not physical-GPU or mobile-device validation. Browser process/GPU peak memory, screen-reader use and sustained/concurrent load remain unvalidated. The main frontend chunk remains large (about 1,140 kB minified / 266 kB gzip).

## Strict integration acceptance gates

These are required checks, not recorded passing results. Upstream live validation and PicoFarm proxy/UI validation must be reported separately.

- Quantity defaults to one; accept only integers `1`–`512`. Cover omitted/explicit one, `512`, zero, negative, fractional and oversized values. Multiple instances require arrangement and remain on one plate; surface no-fit failure without silent quantity reduction or plate splitting.
- Preserve optional finite `rotate`, `rotate_x`, `rotate_y`, including explicit zero, through preview, synchronous slice and queued slice. Any supplied angle disables UI auto-orient; reject contradictory `orient: true`. Verify upstream `rotateX`/`rotateY` names and omit unset values.
- Keep preview/slice geometry and overrides identical. Changing quantity, positioning, model or profiles invalidates stale preview results; an older response must not replace the active configuration's preview.
- Render complete upstream v2/full geometry in 3D, not inferred STL placements. Require `geometryComplete: true`, complete source, no truncation/omissions and coherent actual/declared counts within 250,000 segments / 4,096 layers. Exercise rejection of malformed/over-budget/incoherent payloads and upstream `422` overflow without partial geometry. Legacy v1 remains an explicitly partial SVG fallback; test the 20,000-segment per-layer 2D display warning. Thumbnail is not a substitute for unavailable toolpaths.
- Keep thumbnail and advanced positioning separately collapsed by default; expanding/collapsing must not change settings or silently trigger slicing. Check labels, keyboard use and narrow-screen overflow in both supported languages.
- Test typed proxy response preservation, preview-only `toolpathMode=full` forwarding (slice unchanged), explicit-zero forwarding, upstream failure feedback and queue persistence with controlled HTTP fixtures. Measure large bounded previews and UI responsiveness separately; the sequential upstream measurements below are not a concurrency/load certification.
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

## Full toolpath rendering and measurements

The frontend and backend share a 250,000-segment / 4,096-layer ceiling. Full-mode backend overflow fails with `422`, not a sampled success. Only complete, count-coherent v2/full data enables 3D. v1 remains an explicitly partial SVG fallback, with a separate 20,000-segment per-layer display cap and warning.

The renderer uses `InstancedMesh` segment boxes: 64 bytes per matrix and 12 triangles per segment, grouped into up to six feature groups. At the segment ceiling, matrices alone require 16,000,000 bytes and geometry represents 3,000,000 triangles; JSON, parsed objects, temporary matrices and GPU/driver overhead are additional. Missing width/height uses labeled estimates. The source thumbnail is separate from the sliced geometry. Sanitization and matrix construction are synchronous main-thread work, so bounded geometry can still stall the UI.

Sequential fresh-process local Orca 2.5.0 HTTP measurements at the final cap:

| Fixture | Segments / layers | JSON bytes | HTTP elapsed | API peak RSS (KiB) |
| --- | --- | --- | --- | --- |
| Single instance | 76,160 / 240 | 14,156,412 | 4.16 s | 149,728 |
| Three instances, explicit zero rotation | 228,444 / 240 | 42,253,033 | 8.16 s | 259,572 |
| Single instance, multi-shell support/brim | 79,718 / 240 | 14,801,567 | 4.86 s | 184,356 |

All three returned v2/full complete geometry with coherent counts and no omitted segments. RSS is the **upstream Go API process VmHWM only**, not Orca CLI, whole container, PicoFarm proxy or browser peak memory. HTTP elapsed includes slicing and transport; it is not renderer latency. A synthetic 250,000-segment parser/JSON-encoder run produced 38,746,814 JSON bytes in 1.02 s, with 432,486,688 cumulative allocated bytes and 191,068 KiB process peak RSS. These single-run observations do not establish safe concurrent capacity. Earlier one-million-segment experiments used a superseded temporary ceiling and are not current supported limits.

JSON transport remains costly; the PicoFarm proxy decode has no response-byte bound and there is no global preview concurrency guard. The geometry ceilings are not an end-to-end memory bound. Physical GPUs and real mobile devices are not yet proven.

The isolated `web/tests/slicer-real-preview-smoke.cjs` accepts an operator-supplied `REAL_PREVIEW_FILE`; never commit private payloads. Three real responses passed in Chromium with SwiftShader: every source segment rendered, support filters removed exactly the expected counts, layer cuts restored, 2D/3D switching worked, actual WebGL context loss showed an actionable fallback, and close detached/lost all created contexts. No page errors or unexpected API writes were observed.

| Real response | Time to rendered | Observed rendered JS heap |
| --- | --- | --- |
| Single instance, 76,160 segments | 2.83 s | 59.9 MiB |
| Three instances, 228,444 segments | 6.78 s | 169.4 MiB |
| Multi-shell support/brim, 79,718 segments | 2.84 s | 63.1 MiB |

Time includes click, mocked response delivery/JSON parse, geometry generation/upload and two animation frames; it excludes upstream slicing. These are single-run observations, not thresholds. `performance.memory` measures JS heap, not process/GPU memory. Heap remained higher immediately after close without forced garbage collection; context cleanup passing does not establish absence of memory leaks. These browser runs replay actual upstream responses through mocked PicoFarm APIs; a live end-to-end proxy benchmark is still absent.

## Regression gates

| Workflow | Automated evidence |
| --- | --- |
| Preview/slice | `slicer_contract_test.go`: true/false, parity, overrides, upstream keys, G-code-only output |
| Library | `gcode-library-view.test.mjs`: global sorting, missing/default metadata, group pagination, stale preview and response gates |
| STL | `stl-viewer.test.mjs`: dimensions, invalid geometry, byte/triangle limits, camera poses |
| Profiles | Service/repository/handler/schema tests: validation, conflict, restore, failed upload, quota, fresh/upgrade DB; `slicer-settings-editor.test.mjs`: actual handlers without DOM, no autosave and draft preservation |
| Jobs | `TestSlicerJobs*`: queue/concurrency, idempotency, cancel refusal, restart interruption, persistence failure, redaction and terminal deletion; settings isolation and frontend retry/polling tests |

`web/tests/slicer-browser-smoke.cjs` also exercises comparison, stale preview, queued cancellation/terminal removal and editor save/conflict in real Chromium with mocked APIs. Configure `PLAYWRIGHT_MODULE`, `CHROMIUM_EXECUTABLE` and `BASE_URL` for an isolated server. It requires WebGL by default; `ALLOW_NO_WEBGL=1` explicitly skips geometry/camera assertions if unavailable, checking the error fallback instead. The latest synthetic full smoke passed with WebGL2 via SwiftShader, including the mobile-width grid correction; it does not establish real-device GPU behavior or real-payload browser performance.

Tests use temporary databases, synthetic models and controlled HTTP fixtures. The separate upstream HTTP measurements above do not prove PicoFarm end-to-end compatibility, physical output, browser memory, concurrent load or device-specific WebGL behavior. The real-payload replay results above do not certify sustained load or leak-free behavior. Cache changes in the companion slicer repository were exercised with a synthetic CLI. The frontend still emits a large-chunk warning; route-level optimization remains pending.
