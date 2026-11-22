# Pricing UX references and design criteria

## Problem

The first visual redesign still exposed production accounting before the first useful estimate: printer costs, quantity/basis controls, acknowledgment checkboxes and a dense results checklist. Collapsing some fields did not make the initial task simpler.

The target journey is **choose a ready G-code → enter filament price → calculate**. Other information should refine the estimate rather than act as prerequisite setup.

## Primary references reviewed (2026-11-22)

These observations come from public page content and product documentation, not completed authenticated-product usability tests. No third-party files were uploaded. Vendor correctness/performance claims were not independently verified.

| Reference | Observed/documented pattern | Application and limits |
| --- | --- | --- |
| [Prusa 3D Printing Price Calculator](https://blog.prusa3d.com/3d-printing-price-calculator_38905/) | G-code import fills print time and mass; manual entry remains possible. Electricity, labor and machine/upkeep are explicitly optional. | File-first data reuse and optional cost categories. Do not reproduce its long visible form or confuse its material markup with margin on sales. |
| [SimplyPrint print cost calculations](https://help.simplyprint.io/en/article/all-about-the-print-cost-calculations-feature-19ck72x/) | Documentation separates basic material costing from advanced machine runtime, electricity and labor. Existing file/material information feeds estimates. | Strong precedent for a useful material-only starting point. Documentation of an authenticated product, not an interactively verified flow. |
| [3D PrintForce calculator](https://3dprintforce.com/3d-printing-calculator) | Separates recommended sale price and cost summary; groups labor/supplies/fees under advanced settings. Promotes sliced-file import to avoid typing. | Keep cost and selling price distinct and results close to inputs. Import demonstrations are paid-product promotional content, not proof of free-calculator behavior. Avoid silent generic overheads. |
| [AndreasReitberger/3D-Print-Cost-Calculator](https://github.com/AndreasReitberger/3D-Print-Cost-Calculator) | Desktop project README describes file loading and reusable printer/material management. | Reusable catalog data can reduce repetition. README review only; application was not run and is not evidence of minimal onboarding. |

## Proposed interaction criteria

- A valid G-code with mass plus a manually entered price/kg must be enough. No printer/material record creation, unknown copy-count acknowledgment or advanced section is necessary for whole-file costing.
- List all ready G-codes, including those associated with STLs. Requiring the user to understand library storage relationships adds unnecessary steps. Keep the STL/slicing route available separately.
- A whole-file estimate covers one complete print run, which may contain multiple physical objects. Never present that total as a known per-piece price.
- Expose extracted mass/time as a compact summary, with editing on demand. Missing mass needs a clear correction path; missing time need not block material-only costing.
- Clearly show any suggested sales margin and allow editing it. Do not silently invent machine, energy, labor or fee amounts. Material cost is not a recommended selling price or accounting profit.
- Optional details must preserve entered costs and their effects when collapsed. Applied costs need an accessible summary. Disclosure state and financial scope must be independent.
- Per-piece pricing is an explicit refinement requiring real piece counts/basis. Opening advanced presentation alone must not change the request.
- Preserve configured-currency mismatch protections and exact decimal payloads. Confirmation is justified when reinterpreting amounts, not as a routine requirement for basic manual input.
- Empty optional costs should be disclosed as not included, not presented as a wall of errors before the first price. Losses and genuine invalid input still require prominent feedback.

## Validation criteria

Measure the actual browser journey, not source tokens: choose one ready G-code, fill one price/kg, calculate without advanced setup; verify the request and whole-print result labels. Also cover missing metadata, manual fallback, associated G-code search, optional-cost retention, exact active targets, piece/batch transitions, stale responses, explicit slicing and mobile keyboard/overflow behavior.

Passing automated checks does not establish user comprehension. Follow-up usability observation should check whether an unfamiliar operator can reach a result without assistance and correctly explain which costs and physical quantity it covers.
