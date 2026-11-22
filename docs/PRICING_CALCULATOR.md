# Pricing calculator — scope and validation

## Delivery boundary

Approved scope: [QUOTING.md](QUOTING.md), implementation decision dated 2026-11-22. This increment targets a standalone calculator with quick and complete presentations of one backend engine. It does not replace the existing quotation engine or retroactively recalculate quotations. PDF proposals, saved commercial revisions and order conversion are subsequent work.

Implemented locally: standalone `/pricing` page, stateless decimal backend, optional reusable printer costing, quick/full presentations and conditional discount scenario. Validation evidence and limitations are recorded below. No PDF, saved commercial revisions or order handoff are implemented by this increment.

## User workflow

1. Start with a ready G-code (including files associated with STLs), or enter mass manually. The alternative STL view retains associated variants and explicit slicing. Choosing a file only loads metadata; it does not slice or print.
2. Enter filament cost per kg. A file with valid mass plus that rate is sufficient for a basic estimate: no printer record, piece count or advanced-cost setup is required. Extracted mass/time appear in a compact editable summary. Durations accept decimal hours or `hh:mm[:ss]`; time is required when hourly costs are used.
3. The default scope is **one entire print run**, not one known physical piece. The UI maps this to one batch containing one calculation unit representing the run. An explicitly visible editable 30% margin is the starting sales target, not an inferred profit recommendation, material markup or net margin. Other target anchors, minimum charges and discounts remain refinements.
4. Add optional printer, energy, labor, reserve, shipping, extras and fee information when needed. Quick/full presentation and collapsed editors preserve all entered values and their effects. Selecting a printer/material retains its ID and local-edit provenance. Catalog denomination and configured-printer mismatch protections still apply when catalog costs are explicitly loaded; no currency conversion is performed.
5. Choose **Price individual pieces or batches** only when a per-piece result is wanted. Enter real demand/capacity and verify how the file totals relate to that basis. Scope and basis changes are financial changes, independent of presentation. Unit-scoped labor/extras/fees refer to a whole run in whole-print scope and requested physical units in piece scope.
6. Calculate and inspect included cost, suggested price and surplus over included costs. Whole-print results never claim a known physical per-piece price. Optional missing costs remain inspectable without blocking a material-only estimate; losses and minimum-charge warnings remain prominent. Editing financial inputs invalidates the previous result until recalculated.

Design rationale and primary references: [PRICING_UX_REFERENCES.md](PRICING_UX_REFERENCES.md).

No commercial drafts are automatically persisted in ordinary browser storage. No calculation creates a quote, order, print job, inventory movement or hardware action. A separate explicit **Slice a new G-code** action opens the shared slicer modal without leaving the calculator. Submission uses the existing background job/import workflow and can create library files; it does not dispatch a print. A filtered jobs panel exposes status and lets the operator load a completed result after a fresh library lookup. Closing a modal or navigating does not cancel accepted slicing work.

## Calculation principles

- Use deterministic decimal arithmetic in the backend; decimal strings cross the API boundary. Binary-floating frontend formatting is not the source of canonical totals.
- Currency is explicit. Reusing legacy currency-less cost records requires confirmation of denomination, not implicit conversion.
- Cost omissions differ from explicit zero and produce qualified estimates. Complete presentation does not prove complete cost coverage or net profit.
- Proportional input describes one unit. Batch input describes an entire sliced plate; incomplete final batches conservatively use a full plate and must be identified as an estimate rather than measured partial-plate consumption.
- A printer's direct hourly rate and a derived rate are alternatives, not additive acquisition-recovery methods. Included energy/maintenance must not also be added separately.
- Active human work is separate from unattended machine time.
- Additional costs declare occurrence: per unit, per batch, per project or per order. The first calculator represents a single production scenario, not a multi-project quotation composer.
- Shipping cost and customer shipping charge are separate. Selling-price anchors refer to goods; shipping revenue and applicable fee bases are explicit.
- Discount results are conditional scenarios, not silently applied to the base offer. Discounting must recalculate fees and gain rather than merely display a reduced price.
- A minimum service charge is a commercial floor, not an additional production cost; fractional-minor-unit floors are rounded upward before enforcement for gain/margin targets. Manual prices below it must remain explicit and warned, not silently rewritten.

## Numerical reference cases

These cases define independent expected results to verify against the implementation; they are not recorded passing tests.

| Case | Inputs | Expected result |
| --- | --- | --- |
| Material | 100 g at 100 currency units/kg | Material cost 10.00 |
| Machine | 2 h at 3.00/h | Machine cost 6.00 |
| Energy | 100 W average, 2 h, tariff 1.00/kWh | Energy cost 0.20 |
| Labor | 30 active minutes at 20.00/h | Labor cost 10.00 |
| Gain anchor | Cost 20.00, fixed selling fee 2.00, fee 10%, desired gain 10.00 | Goods price 35.56 after currency rounding; rational target 32/0.9 |
| Margin anchor | Cost 20.00, fixed fee 2.00, fee 10%, margin 20% | Goods price 31.43 after currency rounding; rational target 22/0.7 |
| Discount loss | Cost 20.00, base price 25.00, discount 30%, no fees | Discounted price 17.50, gain −2.50 |
| Full batches | Demand 10, capacity 4, plate material 100 g, plate time 2 h | 3 full batches, 300 g, 6 h; final partial-demand batch estimate flagged |
| Unit/project costs | Demand 5, unit component 2.00, project development 10.00 | Additional cost 20.00, not 60.00 |
| Invalid target | Fee 80%, target margin 20% | Reject zero denominator; no Infinity or arbitrary price |

Round scenario goods prices to currency precision using halves away from zero, then evaluate actual gain from that rounded charge. Round break-even minimum upward to the next attainable minor unit, so the advertised minimum does not produce a sub-cent loss. Negative gain/margin targets are permitted, but negative goods prices are rejected. Round final customer-facing money using the selected currency precision, with an explicit rounding policy. Displayed rounded components may differ from a separately rounded total by minor units; do not claim greater precision than inputs justify.

## Validation gates

- Unit: decimal parsing, bounds, rounding, zero/negative denominators, negative gain, missing costs, duplicate energy/maintenance, both machine rate modes, scopes, quantities, batch remainder, shipping fee bases and conditional discounts.
- API: strict input contract, bounded body/arrays/decimal lengths, malformed/unknown/trailing JSON, auth inherited from the existing API group, no persistence side effects.
- Printer config: fresh/upgrade compatibility, legacy rate preservation and validation before persistence; no printer connection required in tests.
- Frontend: comma-decimal normalization, raw draft preservation, last-anchor behavior, mode preservation, old-response exclusion, missing inputs and explicit source-currency confirmation.
- Browser: quick/full flow, source selection, pricing result and discount scenario, keyboard operation, responsive width, no raw translation keys or page errors, no unexpected API mutations.
- Regression: project test helper, frontend lint/build, targeted race tests and backend health after changes.

## Local engine evidence (2026-11-22)

The engine/API implementation passed targeted Go tests; `go vet ./...` passed. Independent arithmetic checks verify that reported minimum prices cover exact rational costs after fees in BRL, JPY and CLP across fractional costs and percentage rules, and that cost changes preserve each target's meaning.

One bounded-input microbenchmark (50 extra-cost rows and 50 fee rows, 1,000 units in batches) measured approximately 0.57 ms/op and 182 kB allocated/op on the local amd64 runtime. This is a single engine microbenchmark, not HTTP latency, browser performance, process peak memory or concurrent-load certification. An isolated loopback HTTP check completed 100 calculation requests at concurrency 10, all with the expected 26.20 cost / 13.80 gain and `Cache-Control: no-store`; observed p50 9.2 ms, p95 54.0 ms and maximum 104.3 ms including client overhead. This short run is not sustained-load certification or a production capacity promise.

Full Go tests, targeted race tests (service/API/repository/database), 85 frontend tests, lint and production build passed during integration. The project helper passed and the local backend was rebuilt/restarted with `/health` returning `status: ok`. Existing frontend lint warnings and oversized-bundle build warning remain. The final isolated Chromium smoke passed 22 checks across EN/PT-BR, with 28 real calculation requests and no page/proxy errors or unexpected requests. It covers desktop/mobile widths, ordinary mobile warning clicks, source guards, quick/full retention, configured-printer currency changes, configured-to-legacy source replacement, active-anchor preservation and stale responses. External resources and WebSockets were blocked; no hardware endpoints were invoked. A separate Chromium check saved/reloaded direct and derived printer profiles through the real isolated API, including comma-decimal normalization; its final run made exactly two fixture-only PATCH requests and had zero page errors. This demonstrates save/reload persistence, not restart durability or hardware behavior. Local evidence artifacts are intentionally not versioned.

## First visual refactor validation (historical, before progressive simplification)

The redesigned page and shared slicing modal compile; 86 frontend tests pass, including exact `hh:mm[:ss]` normalization. The project helper's repository/service/API tests and frontend production build pass with the required Go toolchain. Targeted calculator/slicer component lint has zero errors or warnings. The existing large-bundle warning remains. The redesigned workflow passed 28 isolated Chromium scenarios across EN/PT-BR, with 34 real calculation requests and two mocked slicer submissions. Checks cover retained printer/material IDs, explicit target segments, human duration entry, basis/currency guards, hidden-cost retention, visual STL/G-code selection, accepted-job result reuse, stale calculation suppression, prominent missing-cost notices, unique DOM IDs and modal Close not submitting either form. At 1440px and 390px there was no horizontal overflow or clipped control label; mobile content width was 326px after collapsing the shared sidebar. No unexpected requests or browser/proxy errors were recorded. Catalogs, slicer jobs and thumbnails were fixtures; these checks do not establish real slicer availability, thumbnail quality or production-load performance. Full Go tests and `go vet ./...` also pass.

## Progressive workflow validation

The basic whole-print redesign leaves the backend calculation contract and engine unchanged. The 86 frontend tests and full frontend lint pass (existing warnings outside the changed pricing files remain); targeted pricing lint is clean. The project helper's repository/service/API tests and production build pass. The pre-existing oversized-bundle build warning remains. The updated isolated Chromium smoke passed 22 checks across EN/PT-BR: G-code + one manually entered filament price without advanced setup; whole-file cost/price equality; missing-metadata fallback; associated/root selection; retained optional inputs and identical payloads across disclosure changes; scope reinterpretation acceptance/cancellation; excluded maintenance not requiring duration; stale calculations and deferred slicer-result selection across scope changes; explicit mocked slicing. Captures at 1440px and 390px showed no horizontal overflow or clipped labels. Catalogs and slicing were fixtures; calculation requests reached only a temporary loopback backend. Screenshot inspection covered the basic mobile/desktop layouts. These checks do not establish newcomer comprehension, real slicer availability, production-load behavior or thumbnail quality.

## Residual limitations

The model picker uses the existing grouped library snapshot (up to 1,000 STLs, with per-model association queries), not a fully paginated server-side search. Large libraries may incur response/rendering costs and require a dedicated bounded association API. Search currently filters that snapshot locally. Online slicing still requires configured profiles/service availability and uses the existing background-job recovery limitations.

On narrow screens the shared application sidebar must be collapsed to leave usable calculator width; the calculator does not redesign global navigation. Unsaved calculator warnings cover unload/reload and clicked in-app links, but not the browser Back button. Drafts are memory-only and are lost on navigation. Catalog material prices and legacy rates retain their existing numeric representation and denomination limitations; explicit acknowledgment is not currency conversion. Gain/margin are estimates from supplied costs, not accounting net profit. Input accuracy remains operator-dependent. Slicer time/material are estimates. Generic manual fee rules do not certify any marketplace's current fees or tax treatment. Printer cost assumptions do not establish monthly profitability. Sustained concurrent calculator load and localization beyond supported currencies require separate evidence. Production commands and fiscal/billing integrations are outside this increment.
