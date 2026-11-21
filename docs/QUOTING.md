# Quotation and Costing Decisions

This document records the approved product decisions for quotation, cost estimation and commercial pricing.

**Status:** decision record and implementation plan. **It does not mean the pricing engine, calculator fields or workflow changes are already implemented.** Current PicoFarm still has the quote/order behavior described in [API contracts](API_CONTRACTS.md#orders-customers-and-quotes) and the costing gaps listed below.

Last update: 2026-11-20.

## 1. Scope and priority

- FDM is the only process in the current scope.
- Resin/SLA is out of planning. It may be analyzed in the future with its own consumables, wash/cure steps, machine rate and failure model.
- The quotation system is operational and commercial: it estimates manufacturing, proposes a price, sends a customer-facing proposal, and can generate an order. Fiscal issuance, electronic invoicing and tax compliance remain future integrations and cannot be derived from this document.
- Language, currency, transport/delivery enums and fiscal rules stay independent from each other.

## 2. Approved commercial decisions

| Area | Decision |
| --- | --- |
| Process scope | FDM first; resin out of scope for this cycle |
| Project composition | One quote can contain several projects, standalone printed pieces, non-printed components, services and order-level costs |
| Non-printed components | Magnets, metal parts, screws, electronics etc. have quantity, unit and cost; no stock control in the first delivery |
| Stock control | Future goal after a dedicated design decision; not part of the first quotation scope |
| Reusable cost settings | Yes: material, printer rate, hourly labor, energy and other defaults are reusable across quotes |
| Per-quote overrides | Yes; adjustments in a quote must not silently rewrite the original reusable record |
| Activity pricing | Both time × hourly rate and fixed amount, selected per activity |
| Quantity comparison | Yes; a quote can offer alternatives such as 1/5/20 units or different materials/finishes |
| Payment/marketplace rules | Configurable fixed amount, percentage, base, calculation method and frequency; large-marketplace presets may exist but must be editable |
| Below-cost price | Warn only; never block the operator. Show the calculated loss and require confirmation when it is material |
| Customer proposal | First delivery: printable PDF and manual sharing |
| Public quote link | Existing read-only presentation can be improved later |
| Customer acceptance | Internal approval to order for now |
| Public customer acceptance, email and WhatsApp integration | Explicit future work, not in the first delivery |
| Offer validity / production deadline / payment terms | Simple configurable recorded conditions |
| Payment installments/deposits | Conditions recorded on the proposal; no automatic billing system in the first delivery |
| Estimated vs actual cost | Future improvement, not essential now |
| In-app slicer | Do not require slicing inside PicoFarm for quotation; prefer a selected G-code |

## 3. One engine, two presentation modes

Use one calculation engine with different depth:

- **Basic:** quick pricing for occasional sellers. It must not silently treat omitted labor, wear, fees or overhead as zero. It must disclose what was not included.
- **Commercial:** same engine with batch/project costs, overhead allocation, fee rules, channels, margin scenarios and reusable commercial settings.

The mode changes information density and analysis, not the underlying units or formulas.

Recommended outputs are kept distinct:

1. **Estimated cost** of the resources considered.
2. **Minimum price to cover the considered items**, including the entered selling expenses.
3. **Selling price**, chosen by the operator.

Terms must not be conflated:

- markup is a price above cost;
- the percentage input is margin on selling price (`gain / price`), not markup on cost (`gain / cost`);
- contribution margin is variable revenue minus variable costs;
- profit/net result depends on the full cost basis and fixed coverage;
- “minimum price” is not proof that the whole month is profitable.

## 4. Cost inputs and provenance

Every input must record its provenance: slicer output, catalog record, measurement or manual entry.

### Process scope

- Basic can quote without a customer or saved project.
- Commercial reuses the existing quote/order structure.
- Printable estimates and simulation must not create print jobs, consume filament, enqueue work or send printer commands.

### Material

For filament:

```text
material_cost = sum(grams_used × package_price / package_grams)
```

Include pieces, supports, brim/raft, purge and tower when the slicer reports them in the same total. Do not add them twice.

Machine-time-related measurements:

- Prefer measured `kWh × tariff`.
- Without measurement, use `average_W / 1000 × hours × tariff`.
- Nominal power supply wattage is not average consumption.
- Resin/SLA is out of scope until its own model is designed.

### Machine wear and maintenance

Printer rate should be explicit and configurable. A possible managerial formula is:

```text
machine_rate = ((investment − residual) / expected_productive_hours) + maintenance_per_hour
machine_cost = machine_rate × occupied_time
```

Do not divide the investment by 24-hour continuous use automatically. Utilization changes recovered cost significantly.

Do not stack two full recovery methods for the same machine. Investment payback and economic depreciation are alternative managerial choices; choose and document one treatment.

The printer rate must declare whether it already includes energy, maintenance and/or overhead. Otherwise adding a separate energy line can duplicate cost.

### Human labor

```text
labor_cost = sum((active_minutes / 60) × hourly_rate)
```

Use active preparation, removal, cleaning, finishing, inspection, assembly and packing time. An unattended four-hour print is not four hours of human work.

Modeling/development must be separated from repeated production.

### Other cost bases

Each item must declare whether it occurs:

- once per project;
- once per plate/batch/setup;
- once per unit;
- once per order.

Examples: development charge and one-time assembly fixtures are project-level; plate setup is batch-level; magnets per assembly are unit-level; freight and packaging can be unit-level or order-level depending on the operation.

### Failure reserve

Do not hardcode a universal failure percentage.

For independent repeated attempts until one approved result:

```text
expected_cost = successful_attempt_cost + (f / (1 − f)) × mean_failed_attempt_cost
```

`C/(1−f)` is only a narrow shorthand and generally wrong when a failed partial print does not cost the same as a full successful attempt.

Allow an explicit manual reserve when the operator has no reliable history. Mark it as an estimate, show its amount, and do not imitate precision.

### Selling expenses and fees

Fee rules must store at least:

- percentage and/or fixed amount;
- base (goods, freight, total or other explicit scope);
- occurrence unit (per item, per order or per transaction);
- bands, limits and conditions when applicable;
- currency;
- source and review date.

Etsy-style processing fees are country/order dependent and may include shipping/applicable tax in their base. Mercado Livre fees vary by category, listing and price band. There is no safe universal marketplace percentage. Validate a calculated price against the actual rule before promising a net result.

### Overhead allocation

Commercial mode may allocate monthly fixed costs through an explicit periodic or hourly rule.

Overhead allocation helps price formation. It does not prove the month will be profitable: if forecast productive usage is not realized, the same fixed cost remains. Keep contribution margin separate from managerial result after allocation.

## 5. Price calculation and dynamic fields

The operator works in one form. It shows at least:

- estimated cost;
- minimum price for considered costs;
- selling price;
- estimated gain in currency;
- estimated margin on selling price;
- fee and cost details behind those numbers.

There is no separate selector for “how to calculate”.

### Linked fields

The three fields are editable views of the same calculation:

| Last field edited | Field used as the anchor | Other fields recalculated |
| --- | --- | --- |
| Estimated gain in currency | Gain | Selling price and margin |
| Estimated margin on selling price | Margin | Selling price and gain |
| Selling price | Price | Gain and margin |

When costs, quantities, taxes or fees change, keep the same anchor that was last edited and update the other fields.

Simplified formulas when selling fees use the selling-price base:

```text
price = (cost + fixed_selling_fee + desired_gain) / (1 − percentage_fee)
price = (cost + fixed_selling_fee) / (1 − percentage_fee − target_margin)
gain = price − cost − fixed_selling_fee − percentage_fee × price
```

Rules:

- Reject zero or negative denominator cases; do not return `Infinity` or divide by zero.
- Percentages use explicit base/scope; do not silently share a base across unrelated fee rules.
- Define one deterministic backend numeric representation, intermediate precision and rounding mode; canonical monetary totals must not depend on client/server binary-float differences. Round only for display/final proposal, except for an explicitly documented currency rounding step.
- Avoid feedback loops when a derived field changes while the user is typing.
- Permit negative gain and show it as a loss estimate rather than rounding the presentation to look profitable.
- If known costs were omitted, label the result partial.

The term “gain” must be qualified when required costs are absent.

## 6. Quote structure for projects, parts and batches

A quotation can contain:

```text
Quotation
├── Project A — quantity
│   ├── development/modeling (once)
│   ├── printed pieces (batches)
│   └── unit finishing
├── Standalone printed piece B — quantity
├── Non-printed components (magnets, metal parts etc.)
├── Assembly/services
└── Packaging / delivery / order-level items
```

Alternatives in the same quotation compare real quantities and production batches. Changing quantity must recalculate shared costs, not just multiply everything linearly. Development is not automatically multiplied by quantity.

Batch production needs explicit semantics:

- quantity per plate;
- number of plates;
- actual sliced batch time/material when a valid G-code exists;
- setup/support/purge treatment;
- failure/reprint effects.

A selected G-code is the preferred input for material and time. Manual input is allowed and must be identified as manual. We do not require slicing inside PicoFarm for quotation in this cycle.

Each submitted quote revision stores a snapshot of:

- every calculation input and its provenance, including reusable defaults and per-quote overrides;
- prices, discounts and totals;
- quantities and alternatives;
- selected G-code library identity, content identity/revision or manual input provenance;
- material and printer assumptions and all applicable rates;
- labor/finishing inputs;
- fee rules, bases and occurrence scopes;
- unit, plate/batch, project and order cost scopes/allocation rules;
- expenses/overhead;
- calculation version;
- currency;
- validity, production lead time, production start event and payment terms.

Later catalog changes must not silently rewrite a quote already sent or accepted.

## 7. Existing technical debt to fix before considering the pricing engine complete

These are current limitations or planned fixes, not new product goals:

- A quote needs an explicit recorded currency. Cost-source records currently do not carry their own currency in all cases.
- Commercial calculations must be centralized in the backend rather than trusting the frontend to send canonical totals.
- Quote acceptance must be safe under repetition/concurrency and avoid duplicated orders.
- Sent/accepted quote items should be protected from casual editing.
- The public quote response must be reviewed and restricted before broader sharing.
- Quote-to-order conversion should transfer the commercial/operational context that was accepted.
- Quote issue must remain a state change unless an explicit notification integration is enabled and validated.

## 8. Customer proposal and operations workflow

### First customer-facing scope

- Printable PDF and printing are the first required output.
- Manual sharing by email, WhatsApp or file transfer is acceptable.
- Public links are future work. A later implementation may reuse the existing public quote presentation only after restricting it to customer-safe data.
- Public acceptance, notification automation and quote email delivery are future.

Customer-visible items include scope, quantities, prices, total, validity, production lead time, payment conditions and terms. Internal costs, machine settings, labor rates, margins and detailed fee breakdown must not be exposed.

### Approval and production

- Accepting a quotation creates/updates the associated order safely.
- Approval must not start a printer.
- Print start remains a separate operational action with preflight, confirmation and existing printer-safety boundaries.

### Conditions on the proposal

The first delivery requires simple configurable fields:

- offer validity date;
- production deadline;
- event that starts production (approval, payment confirmation or final file receipt);
- payment method(s);
- payment terms, including deposit/saldo as recorded conditions.

This is a registration of commercial conditions, not a billing/payment reconciliation system.

## 9. UI rules

- Desktop is primary; mobile must preserve essential creation, review and approval actions.
- “Not provided” is different from zero.
- Every input should make its provenance and cost basis visible.
- The form warns before leaving unsaved edits: **Continue editing** and **Discard**.
- No automatic save of commercial drafts and no sensitive persistence in ordinary browser storage.
- Internal estimates are separated from the customer proposal.
- Physical printer actions remain explicit; quotation or acceptance cannot trigger hardware commands.
- JavaScript error/raw-key checks and bounded G-code/3D preview budgets continue to apply.

## 10. Sequential implementation plan

1. **Foundation:** centralized cost units, currency capture, rounding rules, validation and tests.
2. **Basic estimator:** reusable defaults, selected G-code/manual inputs, plain composition, linked price fields, minimum-price alerts, quote draft integration, revisions.
3. **Commercial mode:** projects and batches, components, labor/development/finishing, overhead allocation, failure reserve, fee rules and quantity alternatives.
4. **Customer proposal hardening:** restricted public projection, printable output, confirmed acceptance, transactional quote-to-order conversion.
5. **Future improvement:** estimated vs actual production feedback and stock control for printing components.

No implementation step may create a print job, consume stock or send a physical command.

## 11. Testing gates

Targeted tests must cover at least:

- duplicated material/energy/machine/labor lines;
- manual vs slicer provenance;
- per-unit, per-batch, project-level and order-level allocations;
- division by zero/negative denominator;
- last-edited anchor behavior;
- invalid and negative fee/margin combinations;
- edited fee presets;
- discounts below minimum;
- multi-project and quantity alternatives;
- revision snapshots and immutability after send/accept;
- mixed/missing currency cases;
- quote acceptance idempotency without duplicated orders;
- public quote field restriction;
- mobile and unsaved-edit warnings.

## Future backlog (not part of first quotation delivery)

- Component stock management.
- Estimated vs actual comparison as a reporting feature.
- Public customer proposal links and public customer acceptance.
- Email/WhatsApp integration and notification automation; manually sending an exported proposal is already the chosen initial workflow.
- Sales-channel integrations producing orders directly.
- Resin/SLA support.
- Fiscal/electronic invoice classification and issuance.
- Advanced payment installments, billing or reconciliation.

## Sources consulted for the research

External sources informed the decomposition and operational cautions. Example values and proposed PicoFarm equations are design decisions, not instructions from these sources.

- Prusa — How to calculate 3D printing costs: https://blog.prusa3d.com/how-to-calculate-printing-costs_38650/
- Prusa — 3D Printing Price Calculator: https://blog.prusa3d.com/3d-printing-price-calculator_38905/
- Formlabs — How to calculate 3D printer cost: https://formlabs.com/blog/how-to-calculate-3d-printer-cost/
- Sebrae MG — Como definir o preço de venda: https://sebraemg.com.br/como-definir-preco-venda-seu-produto-servico/
- Etsy — Payment processing fees: https://help.etsy.com/hc/en-us/articles/115015628847-What-are-Payment-Processing-Fees-for-Selling-on-Etsy
- Mercado Livre — Como funcionam as taxas: https://vendedores.mercadolivre.com.br/nota/como-funcionam-as-taxas-do-mercado-livre
- Brazil Consumer Defense Code: https://www.planalto.gov.br/ccivil_03/leis/l8078compilado.htm
- Brazil national NFS-e portal and technical documentation: https://www.gov.br/nfse/pt-br