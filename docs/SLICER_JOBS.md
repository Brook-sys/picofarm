# Local background slicer jobs

Implementation: `internal/service/slicer_jobs.go`, `internal/api/slicer_jobs_handler.go`, `web/src/components/SliceSTLModal.tsx`, and `web/src/components/SlicerJobsPanel.tsx`. The slicing modal is shared by the Files library and other explicit STL-to-G-code workflows. Lifecycle/router wiring is in server startup and the authenticated slicer route group; the jobs component takes an injected authenticated transport rather than maintaining credentials.

## Contract

- `POST /api/slicer/jobs`: the existing `SlicerSliceRequest` JSON, limited to 64 KiB, with required `Idempotency-Key` header (1–128 characters). Only single G-code export is supported. Returns the local job with `202`. Repeating the same key and payload returns the same job; changing payload with that key is `400`.
- `GET /api/slicer/jobs`: newest-first bare array, including the persisted request and optional result.
- `GET /api/slicer/jobs/{id}`: job, or `404`.
- `POST /api/slicer/jobs/{id}/cancel`: cancel a queued job, returning the job with `200`; repeating cancellation is safe. Running or terminal non-cancelled jobs return `409`. This does not abort remote execution.
- `DELETE /api/slicer/jobs/{id}`: explicitly discard a terminal record and its idempotency key, returning `204`. Queued/running records return `409`; missing records return `404`. Library G-code is not deleted. A later submission of the forgotten key can execute again; the UI warns before removal and drops its matching retained key after success.
- Errors follow `{error: message}`. Invalid input is `400`, queue/history capacity is `429`, and a persistence failure or closed service is `503`.
- Fields: `id`, `status`, `request`, `result?` (existing `SlicerSliceResult`), `error?`, `created_at`, `updated_at`, `idempotency_key`, `persistence_error?`.
- Status: `queued`, `running`, `succeeded`, `failed`, `cancelled`, `interrupted`. There is no invented progress percentage.

## Execution and durability boundaries

One worker per service instance executes existing `SliceSTL` with its own lifecycle context and a 60-minute deadline. Browser navigation/request cancellation does not own that context. The queue allows eight waiting jobs; history is capped at 100 records, with no automatic deletion (deleting idempotency records weakens retry safety). The panel displays usage and allows explicit confirmed removal of terminal records to free capacity. Do not run multiple instances against one database.

The bounded snapshot lives in setting `slicer_jobs_v1`. Constructor errors must be surfaced; do not silently replace an unreadable snapshot. On startup, queued/running jobs become `interrupted`, never automatically replayed. Initialization persists that recovery state before starting the worker. `Close()` cancels local I/O and waits for the worker; it must run before closing the database. Local cancellation is not evidence that the remote Orca process stopped.

`SliceSTL` performs import once during execution; GET/list are read-only and cannot import duplicate results. Idempotent submission prevents duplicate execution for the same retained key. This is **not** a transaction spanning slicing, storage, G-code import, and snapshot update. A crash or failed final snapshot after import can leave a G-code in the library with an interrupted job on restart. Inspect the library before submitting another job; there is no automatic retry, reconciliation, or durable exactly-once guarantee. Result objects are snapshots, not proof a library file still exists after deletion.

Persistence errors stop new execution/submission. If final persistence fails, the in-memory job exposes `persistence_error`; a restart can only recover the last saved snapshot. Raw upstream errors are not retained or returned by the job service, since they can contain URLs or credentials. A worker panic is recorded as a generic failure rather than taking down the server.

## Upstream audit and limitations

The locally available upstream implementation already contains async job creation/status/download/cancel infrastructure. Its persisted job structure omits result paths/work directory; persistence errors are ignored. Consequently it cannot currently provide reliable result recovery across upstream restart. This wrapper reuses the synchronous `SliceSTL` import path rather than introducing another upload/download/import pipeline or advertising upstream durability.

Local concurrency one does not serialize the existing synchronous slice/preview endpoints or other users of the remote service. `SliceSTL` spools outgoing multipart to a request-scoped temporary file; remote buffering/resource usage remains outside this queue. Queue bounding is not a disk, memory or file-size limit for the running job. Remote URL/config/profile changes and source-file deletion while queued affect execution; a queued request snapshots names/options, not STL bytes or profile contents.

## Security and integration gates

Register all five routes under the normal authenticated `/api/slicer` group, with the same sensitive endpoint policy as slice/import. Submission can read stored STL files, send them to the configured slicer, write library files, and change a default G-code. List/get expose model/profile names and results. Cancellation mutates state. Keep the remote port private.

Treat `slicer_jobs_v1` as service-owned state: generic settings GET/SET/DELETE return `403` and generic listing excludes it. The worker accesses it through the service layer; direct database modifications remain outside this isolation boundary. Do not put credentials in profile names, display names, or override values. The worker does not store connection URLs or raw upstream error messages.

The shared slicing modal submits a background job with one stable idempotency key per payload, retained in module memory across modal/library remounts for ambiguous network retries. Changing parameters selects a different key; returning to previous parameters reuses their old key. Explicit successful history removal forgets its matching key. Full page reload loses this browser-memory mapping: check the server-side panel before resubmitting after a reload; cross-reload deduplication is not guaranteed by this UI. `SlicerJobsPanel` appears at the top of the Files library and resolves result opening against a freshly fetched library; removed results report an error. Its injected `list/cancel` transport must use the existing shared authenticated client. A button can submit in the background and close the modal after `202`; polling remounts recover server-side state. Do not regenerate keys silently on a retry.

## Regression coverage

`TestSlicerJobs*` covers bounded concurrent submission, one running local operation, queued cancellation and running cancellation refusal, repeated submission identity, conflicting key payloads, defensive snapshots, restart interruption without replay, failure/panic recovery and error redaction, and persistence failure preventing execution. Handler tests cover input/body limits, unknown/trailing JSON, missing key, missing jobs and accepted submissions. Tests use fake storage/settings or temporary databases, never real slicers/printers.

Frontend polling is serialized (next poll starts three seconds after completion); unmount cancels timers/state updates, not backend jobs. Browser integration still needs modal navigation/remount, result opening, authenticated endpoint policy, queue-full feedback and ambiguous submission retry checks after main wiring.
