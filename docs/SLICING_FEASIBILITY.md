# STL multiplication, arrangement and G-code preview feasibility

**Current status (2026-11-21):** strict single-plate quantity (`1`–`512`), mandatory arrangement for multiple instances, optional finite rotations and upstream-derived full 3D toolpath preview are **implemented locally in PicoFarm**. Current contracts, measurements and acceptance gates: [API_CONTRACTS.md](API_CONTRACTS.md#strict-geometry-and-toolpath-preview--local-implementation) and [SLICER_REFINEMENTS.md](SLICER_REFINEMENTS.md). Preview alone requests `toolpathMode=full`; v2/full complete coherent geometry is limited to 250,000 segments / 4,096 layers, with backend `422` rather than partial overflow output. v1 remains an explicitly partial SVG fallback without 3D. Source thumbnail and advanced positioning remain separate collapsed UI sections. Synthetic Chromium WebGL2/SwiftShader smoke passed; real-payload browser tests remain in progress, and mobile/physical GPU behavior is unproven. Upstream HTTP measurements are not PicoFarm end-to-end, whole-container or browser-memory proof. No deployment, physical printing or hardware commands are part of this work.

**Historical investigation boundary:** the sections below preserve the sourced investigation last updated **2026-11-20**, its pinned source snapshots and proposed POC gates. Phrases such as “current code”, “today”, “not implemented” and “no renderer selected” below refer to that investigation snapshot, not the local implementation summarized above. Historical findings have not been re-executed or newly verified by this documentation update. The original no-deployment-approval boundary remains; spacing, maximum-fit, placement editing and multi-plate splitting are not implied by the current implementation.

## 1. Conclusions and boundaries

The requested capabilities should be investigated as separate, measurable increments; feasibility remains conditional on the gates below:

1. **Post-import toolpath preview:** render an existing stored G-code after an explicit user action. This is the lowest-coupling first POC because PicoFarm already stores the artifact. It does not require a new slicing engine, but it does require authenticated, bounded delivery and proven parser/renderer behavior.
2. **STL multiplication, orientation and arrangement:** keep OrcaSlicer as the authority. A browser may display source geometry or returned placements, but duplicated client-side meshes are not proof of the final Orca layout.
3. **Pre-import toolpath preview:** this requires actually slicing the proposed configuration and returning a G-code artifact or a bounded derived toolpath. The current preview metadata is insufficient. It must not be presented as an instant geometry-only preview.
4. **Spacing and maximum-fit:** the tagged Orca CLI does not establish a general user-configurable spacing argument or a globally optimal packing guarantee. These require CLI experiments and possibly an API/service-level implementation; success must report the effective result rather than imply the request was satisfied exactly.

No renderer, final HTTP schema, interactive bed architecture or Orca fork is selected here. JSON names below are illustrative data requirements for a spike, not published endpoints.

All preview, quotation and slicing investigations are read/compute operations only. They must not create a print job, enqueue production, consume stock or send printer commands. Reconnection, restart and redeployment must never replay a physical command.

## 2. Evidence classes and inspected versions

| Class | Meaning in this document |
| --- | --- |
| **Current code** | Behavior directly inspected in PicoFarm HEAD `92e99ffcb2c73689d47df771d9fb50cac980b6bf` or sibling `slicer-api` HEAD `138888776d16a9b0976171f845895ce02cb6aa03`. The latter is a local source snapshot, not proof of publication or deployment. |
| **Tagged source** | Behavior in OrcaSlicer tag `v2.5.0`, commit `40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19`. Source inspection is not an executed compatibility test. |
| **Upstream documentation/package metadata** | Claims made by the referenced project at the pinned release. These still require integration and adversarial testing in PicoFarm. |
| **Proposal/gate** | Work or acceptance criteria to validate later; not current behavior or an observed benchmark. |

The companion Dockerfile defaults to OrcaSlicer `2.5.0` and packages matching vendor JSON profiles. The actually running image, AppImage banner and profile tree were not verified in this investigation. Do not substitute the Dockerfile argument for runtime evidence.

## 3. What exists today

### 3.1 PicoFarm — current code

- `internal/service/slicer.go` has one `stl_file_id` in both slice and preview requests. It forwards `arrange`, `orient`, support, brim, by-object sequence, multicolor and overrides. There is no quantity, repetition, spacing, explicit instance transform or placement field.
- `SlicerPreviewResult` contains support use, print time, filament amount and a base64 thumbnail. It returns neither G-code nor a temporary artifact identifier.
- The browser slicing flow in `web/src/pages/GCodeFiles.tsx` consumes those estimates/PNG data. It has no layer model or toolpath parser.
- Stored library G-code can be downloaded through `GET /api/files/{id}` using `GCodeLibraryFile.file_id`; `internal/api/handler_file.go` streams the complete file. PicoFarm authentication is optional and applies to `/api/*` when enabled. This is not an `/api/public/*` route; with authentication disabled, the surrounding network boundary is the only access control.
- `GCodeLibraryFile` exposes file identity and extracted estimates/settings/thumbnail metadata, but not a G-code `size_bytes` field. The current download has no preview-specific byte ceiling or Range handling. A one-layer browser view therefore still implies receiving the entire artifact unless a bounded transport is added or the client aborts at a hard cap.
- Scalar `layer_height` metadata is not layer geometry. Existing G-code extraction does not create a navigable list of moves/layers.
- `web/src/components/STLViewer3D.tsx` is a source-STL viewer, not a sliced toolpath viewer. The web lockfile currently resolves Three.js `0.187.0` from the declared `^0.187.0` dependency.

### 3.2 `slicer-api` — inspected source, not deployed-runtime proof

- Sync and async slice handlers accept multiple model files; preview accepts one `file`. Accepting several models is not equivalent to cloning one model into a requested number of instances. Temporary inputs use each upload's basename: same-named parts overwrite one another, so repeated multipart files are not a safe quantity workaround.
- `parseSettings` and `buildArgsForInputs` forward arrange/orient and the other existing options. There is no forwarding for `--repetitions`, clone lists, assemble lists, instance transforms or a generic spacing argument.
- Preview forces G-code/image generation, slices once and returns only support/time/filament/thumbnail metadata. Its temporary workdir is removed by the handler. A generated model thumbnail is not evidence of final arrangement, supports or toolpaths.
- Result collection selects one extension. Sync returns a ZIP when multiple files match, async download serves only the first result path, and PicoFarm currently accepts a single G-code rather than ZIP/3MF delivery. Returning one plate's G-code therefore cannot prove that every requested instance was included.
- The per-model/profile limit is 100,000,000 bytes. The operations documentation explicitly says that individual limits do not bound aggregate body size, file count, RAM, disk or geometric complexity.
- One process-local `sync.Mutex` admits one slicing operation; a concurrent caller receives `409 Slicer is busy`. This is not distributed admission control or a durable queue.
- The default CLI timeout is 1800 seconds. Async job records do not provide durable artifact delivery after restart. Cache files, job files and temporary directories have different lifecycles; complete cleanup on every path is not established.
- The service is unauthenticated and has permissive CORS. The browser must not bypass PicoFarm and call a network-exposed slicer directly.
- Inspected cache code includes ordered model inputs, settings, effective resolved profiles and a SHA-256 of the executable. New multiplication/orientation/spacing inputs still need explicit cache-miss tests; existing coverage does not prove those future fields are represented correctly.

Sources: `internal/slicer/handler.go`, `internal/slicer/service.go`, `internal/config/config.go`, `internal/slicer/cache.go`, `docs/api/operations.md` and `Dockerfile` in the inspected sibling source snapshot.

### 3.3 OrcaSlicer `v2.5.0` — tagged-source findings

- On a compatible selected-plate path, `--repetitions N` is the **total requested instance count**, not the number of additional copies. The CLI sets `duplicate_count = repetitions_count - 1`; values at or below one do not create duplicates. Repetitions rejects `plate_to_slice == 0`. [O1]
- This is not a viable assumption for the current ordinary-STL wrapper path: tagged control flow resets a positive plate to `0` for non-BBL input before transform actions, where repetitions then rejects the request. The later slice action rereads its configured plate only if processing reaches it. Treat repetitions on the current STL route as a negative-control experiment, not an implementation candidate. [O1] [O8]
- `--clone-objects N` is a separate spike candidate, not verified behavior. It requires one count per input file and loaded filaments, rejects a first input ending in `.3mf`, and uses a total count including the original. Its loop clones every object in the input model, so a multi-object STL does not yield N total objects. Neither application currently forwards this option, and fit/artifact behavior remains untested. [O9]
- On the compatible repetitions path, arrangement failure can reduce `duplicate_count` and retry. The code logs a “max arrangeable count” after its bounded search. This does not prove a global geometric optimum, and a successful CLI exit alone does not prove that all requested instances survived. [O2]
- In the CLI arrangement setup, `arrange_cfg.min_obj_distance` is assigned `0`. Subsequent arrangement code can still apply inflation/clearance, skirt/brim, sequential-print and exclusion rules. Zero at that assignment must not be interpreted as a promise that physical parts or all auxiliary material may touch. [O3] [O4]
- The arrangement structure supports minimum object distance internally, but the inspected CLI/API path does not expose a general user spacing control. A patch, different CLI route or service-side geometry operation would need its own correctness and maintenance review. [O3] [O4]
- Official CLI documentation describes arrangement/transform actions and `--export-3mf`. Transform flags are applied in command-line order. Exporting a 3MF is available upstream, but this investigation did not establish that the current API can return an arranged-instance 3MF with the required identity/placement metadata. [O5] [O6] [O7]
- Orca's desktop preview is coupled to GUI/OpenGL components. Direct reuse of that renderer in the browser is not established and is not part of the proposed POC.

## 4. Recommended responsibility split for the spikes

| Component | Responsibility | Must not claim |
| --- | --- | --- |
| Browser | Request a preview explicitly; fetch through an authenticated bounded transport; parse/render existing G-code; expose layer/toolpath warnings; dispose resources | That a source-STL mockup is the final arrangement, that unsupported commands are safe, or that a visual inspection certifies a print |
| PicoFarm backend | Authorize library access; enforce artifact byte limits or bounded ranges; proxy slicer calls; persist immutable result identity/metadata when later implemented | Canonical totals from an untrusted browser, a requested count as the effective count, or hardware preflight solely from rendering |
| `slicer-api` | Translate validated options into a version-pinned CLI contract; retain and return authorized artifacts; report effective count/placements/warnings/statistics; enforce timeout/cleanup and cache identity | Exact requested fit when Orca reduced it, durable async delivery after restart, or authentication it does not have |
| OrcaSlicer CLI | Own final orientation, duplication, arrangement, slicing and exported model/G-code geometry | Globally optimal nesting, arbitrary spacing or browser-renderer compatibility without specific tests |

A returned placement list should be derived from Orca output, not predicted by the browser. If stable mapping between 3MF instances and G-code object comments cannot be demonstrated, the list must be labeled informational and cannot support per-instance claims in the UI.

For quotation integration later, the selected artifact and **effective** plate quantity/time/material are the costing inputs. A reduced batch requires operator review; multiplying a one-piece estimate by the requested quantity is not an equivalent substitute. See [quotation decisions](QUOTING.md), whose pricing engine is also planned rather than implemented.

## 5. POC A — bounded preview of an existing G-code

### Scope

Start from the stored G-code details view. On an explicit **Preview toolpath** action, fetch the selected `file_id`, show parse/render progress and provide layer/segment navigation. Do not slice again, modify the file or contact a printer. Plain `.gcode` is the initial format; `.gcode.3mf`, `.bgcode`, live progress and multi-file overlays are separate follow-ups.

The current full-file endpoint can support a constrained experiment only with a dedicated authenticated `fetch`, `AbortController`, streamed byte counting and a hard stop at the cap. It must not use a token query parameter or buffer the whole response before enforcing the limit. Before production, prefer an explicit size/Range-aware artifact contract with bounded ranges and consistent authorization. Aborting a response is not a substitute for server-side resource governance.

### Candidate, not a selection

`@chestnutlabs/gcode-preview-react@0.24.0` is the most complete candidate inspected:

- MIT-licensed, React 18/19 adapter, Node `>=22`, worker parsing, streaming, cancellation, byte/segment/allocation limits and structured partial results. [C1] [C2] [C3]
- Layer/segment scrubbing uses draw-range updates; Three.js tube/line rendering has disclosed quality degradation and an optional Canvas 2D layer renderer. A 2D-only path can avoid Three.js. [C1] [C2]
- Its Three.js renderer declares peer dependency `three: ^0.178.0`; PicoFarm resolves `0.187.0`. This range mismatch is an integration risk, not a proven runtime failure. Test one shared compatible version versus an isolated nested version, bundle duplication and behavior of both viewers. Do not silently downgrade the existing STL viewer. [C4]
- Its published compatibility matrix says current dialect/container fixtures are synthetic constructions. Real exported Orca fixtures remain a POC gate even if the package tests pass. [C5]
- The release is pre-1.0. Browser policy distinguishes tested Chromium from expected Firefox/Safari behavior; module workers and WebGL2 assumptions require actual target-browser tests. [C3]

The legacy alternatives also have version friction: `gcode-preview@2.18.0` depends on Three.js `^0.159.0`, while `3.0.0-alpha.7` declares `>=0.166.0 <0.186.0`. Neither matches PicoFarm's current `0.187.0`; an alpha tag is not a stable adoption recommendation. [C6]

On 2026-11-20, isolated lockfile-only npm audits reported zero known advisories for the inspected ChestnutLabs and legacy dependency sets. This is neither an integrated PicoFarm audit nor evidence of parser correctness, license compatibility, maintenance quality, resource safety or absence of future vulnerabilities. No dependency was added to PicoFarm.

### POC acceptance gates

These are **proposed initial budgets**, not measured results. Freeze the reference hardware/browser and any budget adjustments before implementation; report worst of at least three cold runs rather than selecting the best run.

| Area | Required evidence to pass |
| --- | --- |
| Real Orca corpus | At least three redistributable G-codes exported by the pinned Orca path across different printer/process profiles, plus targeted cases for comments/thumbnails, travel, retraction, variable layers, arcs if emitted, multiple objects and missing/unknown metadata. Record Orca version, profile identifiers, SHA-256 and fixture provenance without private paths or customer data. Synthetic unit fixtures remain additional, not replacements. |
| Parser fidelity | Independently calculated layer/move expectations match within an agreed numeric tolerance; unknown/unsupported commands produce visible diagnostics and are never silently drawn as extrusion. Compare full geometry and selected layer ranges, not only a screenshot. Incomplete/truncated input is labeled partial. A pretty thumbnail is not a fidelity test. |
| Byte/work limits | POC cap: 32 MiB of plain G-code, 2,000,000 parsed motion segments and the library's explicit allocation budget. A 32 MiB + 1 byte response is aborted/rejected before complete buffering or parsing; segment/allocation exhaustion returns a bounded partial/error state. Limits are configurable in code, with no “unlimited” production option. |
| Time budget | On the recorded reference setup, the <=5 MiB functional fixture reaches first useful layer feedback within 5 s and interactive ready state within 15 s; the near-cap valid fixture completes or returns its bounded result within 60 s. Parsing produces no main-thread task longer than 200 ms. An inconclusive measurement is not a pass. |
| Memory budget | For the near-cap fixture, peak browser-process RSS increase is <=512 MiB over the measured warm idle baseline and returns near baseline after close plus browser GC opportunity. Also record renderer-reported segments/vertices/decimation and JS heap where available. RSS attribution limitations must be disclosed rather than hidden. |
| Cancellation/lifecycle | Closing or switching files aborts download, terminates worker work and disposes geometry/materials/GPU resources. No state update occurs after unmount. Network/worker activity stops within 1 s in the automated lifecycle test; repeated open/close cycles do not accumulate canvases, workers or listeners. |
| WebGL and 2D | Exercise WebGL2, context loss and WebGL-unavailable cases. The proposed requirement includes an explicitly selected working Canvas 2D layer fallback; an accessible unsupported-message alone does not satisfy a promised 2D fallback. Verify that 2D results disclose their lack of 3D/orbit capability. |
| Worker/CSP/build | Vite emits and loads workers in development and production; test the strictest intended CSP with an explicit worker policy and no unsafe-eval added merely to make the dependency run. Dynamic import prevents the viewer cost on unrelated routes. Existing headers are not a CSP and must not be described as one. |
| Three.js coexistence | `npm ls three` and the built bundle expose exactly which versions are used. Both current STL and new toolpath viewers pass geometry/camera/cleanup tests. Any duplicated Three.js copy is measured and justified; unresolved peer/runtime incompatibility rejects the candidate rather than being hidden by package-manager overrides. |
| Accessibility/UI | Layer controls are keyboard-operable and labeled; progress, partial output, decimation, unsupported dialect and errors are textual as well as visual. Mobile cannot trigger an unbounded automatic preview. |
| License/supply chain | Review MIT/NOTICE and all transitive licenses, lock integrity/provenance, release source and install scripts. Run the integrated lockfile audit and license check. Zero npm advisories alone is insufficient. |

If these gates fail, the outcome is a rejected or narrowed POC, not permission to raise limits until the test passes.

## 6. POC B — Orca CLI multiplication and arrangement spike

Run in an isolated environment with synthetic/redistributable models and no printer configured. Record the exact executable hash/banner, command, profiles, plate, stdout/stderr, exit code, result JSON, artifacts and durations. Do not use the sibling repository's untracked user files as fixtures.

### Cases and required outputs

Start with one single-object STL and the `--clone-objects` candidate. Include a multi-object input to expose its per-object semantics and a `--repetitions` negative control for the current STL/selected-plate route. None of these commands has been run in this investigation.

| Investigation | Acceptance evidence |
| --- | --- |
| Exact copies | Request totals 1, 2, a fitting N and an impossible N. Export/reopen 3MF and verify the actual effective instance count, identical source geometry modulo transforms and no accidental mesh merging. A fitting N must produce N; an impossible N must produce an explicit bounded failure or a reduced result with warnings, never apparent exact-count success. Report both requested and effective totals. |
| Effective placements | Extract each instance's plate, object/instance identity and transform from the arranged 3MF or another proven Orca artifact. Verify coordinate units, origin, transform order and bed coordinates. Do not infer final positions from duplicated browser meshes. |
| Orientation | Test disabled/forced orientation, explicit rotation order where supported, `allow_rotations` on/off, by-layer versus by-object sequence and brim/support combinations. Confirm the final artifact rather than trusting requested flags. |
| Configurable spacing | Test at least 0, 5 and 20 mm requests. First define whether spacing means minimum XY polygon clearance, bounding-box gap, or clearance including brim/skirt. Measure the resulting geometry with the same definition. If the stock CLI cannot honor it, record “unsupported” instead of displaying a fabricated setting. |
| Bounded maximum-fit | Compare requested N with effective N on fitting, boundary and impossible models. Require a finite request cap (initial spike: 64) and a case deadline (initial spike: 120 s); no unbounded incremental duplication loop. Describe the result as “largest found by this run”, not a global maximum. |
| 3MF instance output | Validate ZIP/container safety, object/instance counts, transforms and successful reopening by the same Orca version. Establish whether one operation can export both G-code and arranged 3MF or needs separate commands; include both artifacts in identity/cache design if retained. |
| Plate/artifact completeness | Test fitting and overflowing counts. Record every plate and object/instance assignment, and prove that delivered G-code covers the reported effective count rather than only plate 1. Explicitly reject or deliberately retrieve additional plates; never silently discard them through first-file/ZIP handling. |
| Statistics | Derive layers/time/material from the actual effective G-code. Compare against preview metadata and the Orca statistics option where available. Detect a reduced instance count and invalidate any quotation based on stale requested totals. |
| Warnings | Surface reduced counts, off-plate/unprintable objects, rotation/arrangement changes and unsupported settings in structured form. HTTP 200/CLI exit 0 is insufficient when the requested count was not produced. |

The repetition arrangement retry/search behavior and possible plate overflow make deterministic tests especially important: fixture expectations must assert effective artifact contents, not a presumed count from a CLI option.

## 7. POC C — `slicer-api` contract spike, after CLI evidence

Do not add a PicoFarm UI field until the CLI spike proves its semantics. The future logical result needs at least:

```text
requested_instances
effective_instances
placements[] = { plate, source_object, instance, transform, status }
warnings[] = { code, message, related_instance? }
artifacts[] = { kind, file_id_or_job_reference, size_bytes, sha256 }
slicer = { version, executable_sha256, calculation_contract_version }
statistics = { time, material, layers, source_artifact_sha256 }
cache = { hit, request_identity_sha256 }
```

This is a data checklist, not a commitment to expose executable hashes, internal paths or debug data publicly. Public responses need sanitized stable identifiers; sensitive executable/profile details belong in protected diagnostics.

Contract gates:

1. **Validation:** quantity is a bounded integer; spacing is finite, non-negative and unit-explicit; transforms have a defined coordinate system/order; conflicting auto/manual options are rejected. Aggregate model/profile bytes and file counts are bounded before expensive processing.
2. **Identity/cache:** identical requests can hit only after all inputs are identical. Changing source bytes/name semantics, requested/effective-fit mode, quantity, spacing, orientation, arrange/sequence/brim/support, profiles, overrides, plate or executable must miss. Cache identity covers retained G-code/3MF/manifest artifacts together. A cache hit must not erase a warning or effective count.
3. **Atomic result:** manifests and artifacts are published only after complete writes and validation. Partial/missing G-code, 3MF or placement data cannot appear as success. Hash/size are computed from delivered bytes.
4. **Concurrency:** retain the explicit `409` behavior or use a separately designed bounded queue; do not allow multiplication requests to bypass the existing slicing lock. Cross-replica locking remains unresolved.
5. **Timeout/cancel:** each spike case has a deadline. Cancellation kills the actual CLI process tree where possible, waits for exit and prevents partial publication. Test client disconnect, explicit cancel, timeout and a child that ignores normal termination.
6. **Cleanup:** after success, error, cache hit/miss, cancel, timeout and restart, only explicitly retained artifacts remain. No temporary path is handed to another request with ambiguous ownership. Disk usage is bounded and measured.
7. **Recovery:** restart/redeploy does not resume a stale slice as a print action or make a non-durable async artifact downloadable. Physical commands are outside this API flow.
8. **Compatibility:** run the same real Orca fixtures through HTTP and compare artifact hashes/manifests with the direct CLI baseline. Existing synthetic-CLI unit tests do not satisfy this gate.

Pre-import toolpath preview is allowed only after this track can return an authorized, bounded G-code artifact (or a separately specified bounded derivative) tied to the exact request identity. Otherwise the UI may show source geometry and estimates, but not claim to show the future sliced toolpath.

## 8. Security and operational gates

- Treat G-code, 3MF, profile content, comments and thumbnails as untrusted and potentially sensitive. Never execute file-provided commands in the browser, log full artifact content or persist it in ordinary browser storage.
- Preview uses the same PicoFarm authorization boundary as file download. Do not create unauthenticated temporary URLs, expose slicer debug paths or place bearer tokens in artifact query strings/worker source URLs.
- Keep the unauthenticated slicer on its private service network. PicoFarm performs upstream calls; the browser does not need direct slicer access.
- Enforce transport and parser limits independently. A library allocation budget does not bound HTTP download, backend temporary storage, CLI RSS or container disk.
- Do not relax CSP, CORS or authentication to accommodate a viewer. Add router/security tests for any new file/preview route and update the sensitive endpoint inventory when implementation occurs.
- A rendered path is not a physical-safety preflight. Temperature, machine limits, profile compatibility, collisions and printer-specific behavior require separate validation.
- Quote calculation, preview generation and artifact inspection remain detached from Queue/Send/Start controls. No automatic print follows approval, slicing, preview, reconnect or redeploy.
- Real-hardware validation is not part of the initial POC. Passing visual/API tests does not establish physical print safety or output quality.

## 9. Decision gates and deferred work

Proceed from POC A to a production design only if real Orca fidelity, bounded transport, cancellation, memory/time, lifecycle, worker/CSP, Three.js coexistence and license review all pass. Otherwise retain the existing thumbnail/metadata behavior and record the failed constraint.

Proceed with multiplication UI only if the CLI/API tracks prove effective instance reporting, final placements, exact artifact identity, bounded search, cleanup and warning propagation. Configurable spacing remains unsupported until measured clearance matches its documented definition. “Maximum” remains “largest found within declared bounds” unless stronger evidence exists.

Explicitly deferred:

- exact client-only replication of Orca nesting;
- a full interactive bed editor or drag/drop placement system;
- maintaining an Orca fork solely to expose spacing without a maintenance/security review;
- live printer-nozzle overlays and multi-G-code scenes;
- `.gcode.3mf`/`.bgcode` browser ingestion in the first preview POC;
- automatic quotation recalculation from unverified placements;
- any physical print trigger.

## 10. Sources

### OrcaSlicer

- **[O1]** Tagged CLI repetitions, selected-plate requirement and duplicate count: [`src/OrcaSlicer.cpp` lines 4938–4959](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L4938-L4959); option definition: [`src/libslic3r/PrintConfig.cpp` lines 12940–12944](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/libslic3r/PrintConfig.cpp#L12940-L12944).
- **[O2]** Arrangement retry/reduction and “max arrangeable count”: [`src/OrcaSlicer.cpp` lines 6005–6266](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L6005-L6266).
- **[O3]** CLI arrangement parameters and `min_obj_distance = 0`: [`src/OrcaSlicer.cpp` lines 5963–5987](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L5963-L5987).
- **[O4]** Internal minimum-distance semantics and subsequent inflation: [`Arrange.hpp` lines 128–155](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/libslic3r/Arrange.hpp#L128-L155), [`Arrange.cpp` lines 150–259](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/libslic3r/Arrange.cpp#L150-L259).
- **[O5]** Official [CLI transforms](https://www.orcaslicer.com/wiki/cli/cli_transform), including arrangement and command-line transform ordering.
- **[O6]** Official [CLI actions](https://www.orcaslicer.com/wiki/cli/cli_actions.html), including `--export-3mf` and slicing actions.
- **[O7]** Official [CLI miscellaneous options](https://www.orcaslicer.com/wiki/cli/cli_misc), including profile, plate and arrangement-related controls. Moving official documentation is secondary to the pinned `v2.5.0` source for version-specific claims.
- **[O8]** Non-BBL plate reset before transforms and the later slice-action plate read: [`src/OrcaSlicer.cpp` lines 2042–2046](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L2042-L2046) and [lines 6441–6445](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L6441-L6445).
- **[O9]** Clone-count validation, first-input 3MF restriction and per-object cloning loop: [`src/OrcaSlicer.cpp` lines 1672–1690](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L1672-L1690), [lines 1742–1747](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L1742-L1747) and [lines 1940–1966](https://github.com/OrcaSlicer/OrcaSlicer/blob/40ecc66f78ed72dc368e0c1ebe40d77f00cc8a19/src/OrcaSlicer.cpp#L1940-L1966).

### Browser preview candidates

- **[C1]** ChestnutLabs pinned release [README](https://github.com/ChestnutLabs/gcode-preview/blob/950d3810ee083320c85a6ee2a24899399ab80633/README.md): formats, React adapter, Three.js/Canvas 2D, layers and cancellation overview.
- **[C2]** [Workers, streaming and performance](https://github.com/ChestnutLabs/gcode-preview/blob/950d3810ee083320c85a6ee2a24899399ab80633/docs/manual/concept-workers.md): resource limits, partial results, worker construction and rendering lifecycle claims.
- **[C3]** [Support policy](https://github.com/ChestnutLabs/gcode-preview/blob/950d3810ee083320c85a6ee2a24899399ab80633/docs/reference/support-policy.md) and [npm registry package metadata](https://registry.npmjs.org/@chestnutlabs/gcode-preview-react/0.24.0): runtime/browser support, React and Node requirements, MIT license.
- **[C4]** [Three.js renderer package at the pinned release](https://github.com/ChestnutLabs/gcode-preview/blob/950d3810ee083320c85a6ee2a24899399ab80633/packages/gcode-renderer-three/package.json): `three: ^0.178.0` peer requirement.
- **[C5]** [Dialect/container compatibility matrix](https://github.com/ChestnutLabs/gcode-preview/blob/950d3810ee083320c85a6ee2a24899399ab80633/docs/compatibility/dialects-and-containers.md): support tiers and synthetic-fixture limitation.
- **[C6]** npm registry metadata for legacy [`gcode-preview@2.18.0`](https://registry.npmjs.org/gcode-preview/2.18.0) and [`3.0.0-alpha.7`](https://registry.npmjs.org/gcode-preview/3.0.0-alpha.7). Version-range statements were also checked through `npm view` on 2026-11-20.
