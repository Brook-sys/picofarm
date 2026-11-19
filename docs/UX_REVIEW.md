# PicoFarm UX review: language, feedback and operational safety

## Scope and evidence

This is a source-based heuristic review, not user research, a browser usability test, or a WCAG conformance assessment. No printers were commanded. The only deliverable of this review is this document; recommendations below are **proposals**, not implemented features.

Reviewed on 2026-11-19: `Layout`, `Settings`, `PrintQueue` and the language initialization/persistence helpers. The existing knowledge graph was queried for navigation and these components, then findings were checked against source. Translation work was happening concurrently: line ranges below are review-time anchors and may shift. Locate the named component/function before treating a finding as still open. Catalog completeness and other pages were not exhaustively verified.

### Guidance consulted

All five sources were fetched and read for this review on 2026-11-19:

1. [Nielsen Norman Group — 10 usability heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/): visibility of status; familiar language; user control; consistency; error prevention; recognition; efficiency; minimalist presentation; recovery; contextual help. These are evaluation principles, not evidence about PicoFarm users.
2. [GOV.UK Design System — Error message](https://design-system.service.gov.uk/components/error-message/): identify the field and corrective action, retain entered information, associate messages with controls, and distinguish input errors from service failures. Its research concerns GOV.UK services, not PicoFarm.
3. [W3C — Understanding SC 3.1.1, Language of Page](https://www.w3.org/WAI/WCAG22/Understanding/language-of-page.html): expose the predominant page language programmatically so assistive technology can select pronunciation rules.
4. [W3C APG — Modal dialog pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/): initial focus inside the dialog, contained Tab sequence, Escape, visible close action, accessible title, inert background and focus restoration. Merely adding `aria-modal` is insufficient.
5. [W3C — Understanding SC 2.4.7, Focus Visible](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html): keyboard users must be able to see focus, without a time-limited indicator. Rendered styles and contrast still require testing.

## What is already present in inspected source

These are implementation observations, **not validated outcomes**:

- `web/src/i18n/index.ts:13–27` configures English and pt-BR resources, English fallback and the initial document language. `web/src/i18n/locale.ts:13–25` detects a stored/browser language, tolerates blocked storage and updates `document.documentElement.lang` when persisting a language change. This is a foundation; it does not demonstrate complete translations or correctly spoken content.
- `web/src/components/Layout.tsx:31–59,109–169` groups navigation into production and commerce, separates settings, and gives the navigation region vertical scrolling. The collapse/expand buttons have translated accessible labels at lines 79–100.
- `web/src/pages/PrintQueue.tsx:62–86,231–238` keeps the prior queue on refresh failure, records the last successful update and displays an alert with a **read-only refresh retry**. Preserve this distinction from resending a physical command.
- `PrintQueue.tsx:191–198,242–250` provides summary counts and an empty state with an upload next step. Status text accompanies several visual indicators rather than relying solely on color.
- `Settings.tsx:1454–1470,1519–1522` disables the business-information save button while saving and temporarily renders a saved label. The error/recovery gaps below still apply.

## Prioritized observations and proposals

Priorities are reviewer judgments: **P1** = address before expanding risky workflows; **P2** = next usability increment. None is based on measured user frequency.

| Priority | Source evidence at review time | Risk / interpretation | Proposed change and acceptance check |
| --- | --- | --- | --- |
| P1 | `PrintQueue.tsx`, `ItemDetailsModal`, lines 328–359: overlay and card divs, visible Close buttons; no dialog role/title association or focus lifecycle in this function | Keyboard focus can remain outside the visible overlay; assistive technology lacks explicit dialog semantics | Use a shared modal implementation with actual background inertness, named dialog, initial focus, Tab/Shift+Tab containment, Escape and restoration. Test the full cycle with keyboard and a screen reader. Do not add ARIA without behavior. |
| P1 | `Settings.tsx`, `BusinessInfoSettings`, lines 1432–1469: rejected reads are omitted from `Promise.allSettled`; missing values become empty strings; save errors only reach the console | A failed read can look like intentionally blank settings; saving can overwrite values. Five independent writes can partially succeed | Show a distinct load-failure state, prevent saving unknown fields, retain the draft and show actionable save failures. Prefer an atomic backend update; until then explicitly reconcile partial outcomes rather than claiming everything was saved. Inject one read failure and one write failure and verify no silent blank overwrite or false success. |
| P1 | `PrintQueue.tsx:130–140,261–274`: commands flow through a generic action wrapper; start errors become a generic error display; cancel/delete handlers directly invoke API actions | A timeout does not prove the printer rejected a command. A user may resend start or cancel after an ambiguous response. Frontend review cannot establish backend deduplication guarantees | Separate “request failed”, “outcome unknown” and confirmed printer state. On unknown outcomes offer **Refresh printer status**, not an automatic resend. Show printer/file before start; require an appropriate confirmation for irreversible deletion/cancellation. Reconcile backend/device state and document idempotency guarantees before enabling command retry. Test against a simulator, never a live print for fault injection. |
| P1 | `Settings.tsx:1487–1516`: several labels are sibling elements without `htmlFor`/input `id`; address inputs rely on placeholders. `PrintQueue.tsx:203–225`: search and status filter lack explicit labels in this markup | Visible text is not necessarily a programmatic name; placeholders disappear during entry | Give every input/select a persistent label and association; individually name address subfields. Add field-specific error descriptions and invalid state when applicable. Verify names in the accessibility tree and that clicking a label focuses its input. |
| P2 | `Layout.tsx:66–72,109,174`: fixed 256px expanded sidebar in a horizontal full-height layout; no small-screen alternate layout in this component. `Settings.tsx:473–489`: five tab-like buttons in a nonwrapping row | Narrow viewports and translated labels may crowd or overflow the work area. This is a source-based risk, not an observed screenshot defect | At small widths propose a dismissible navigation drawer with a clearly named menu button, current location and independent scrolling. Keep essential work area available; make Settings sections wrap or scroll with a visible affordance. Test 320/375/768px and 200%/400% zoom before choosing breakpoints. |
| P2 | `Layout.tsx:112–145,156–168`: collapsed links remove visible text and depend on `title`; `Layout.tsx:128` renders `section.label` without `t()` | Tooltip-only discovery is weak on touch; translated item names can coexist with untranslated group names | Keep an explicit accessible name on each collapsed link and keyboard/touch-friendly label discovery; retain visible expanded mode. Translate group headings, not merely their map keys. Verify all labels in both languages after concurrent translation work lands. |
| P2 | `Settings.tsx`, `BusinessInfoSettings`, local drafts at lines 1423–1430 and explicit save at 1454–1470; section changes at 477 and conditional panels | Drafts can be lost when a panel unmounts or a route changes; no dirty-state guard in the inspected component | Track dirty fields and show “Unsaved changes” / “Alterações não salvas”. On leaving offer Stay/Discard, with an explicit Save option only where the operation is safe and defined. Browser-unload warnings are supplementary, not the sole strategy. Never store secret drafts in local storage. |
| P2 | `Settings.tsx:491–508`: success/error panels use ordinary divs; business saved feedback expires after 2 seconds at 1465. `PrintQueue.tsx:75–78`: toast expires after 3.5 seconds | Feedback may be missed, especially off-screen or with assistive technology | Use restrained live status announcements for confirmed saves; persistent actionable alerts for failures. Keep errors until resolved/dismissed, and preserve drafts. Do not announce the entire 3-second polling queue repeatedly. Test one announcement per relevant state transition. |
| P2 | `PrintQueue.tsx:16–25,165,210–211,343–346`: source badges/config labels and interpolated English plural text coexist with translation calls, including Portuguese source keys | Translation wrappers alone do not ensure a single language; concatenation fails plural rules; English fallback can mask missing catalog entries | Audit actual rendered text, attributes, helper output and API-derived messages per locale. Use plural-aware complete messages, not `file` plus `s`. Translate display labels, never wire enums, filenames or profile identifiers. Treat these exact examples as review-time checks, not proof they remain untranslated after parallel edits. |
| P2 | `Settings.tsx:473–489`: tab-styled buttons communicate selection visually, without explicit tab semantics in this markup | Screen-reader users may not know which section is active | Either implement the complete tabs interaction pattern (named tablist, selected state, associated panels and keyboard behavior) or use ordinary section navigation with clear current-state semantics. Do not assign tab roles without implementing their keyboard model. |

### Modal and keyboard implementation contract (proposed)

Apply first to queue details, then inventory other dialogs separately:

- A visible title supplies the dialog name. For long structured content, initially focus the title with temporary programmatic focusability rather than announcing the entire content as one description.
- Make the background truly inert; prevent interaction and contain keyboard focus. Keep the dialog's own content scrollable without hiding the title/close path.
- Tab and Shift+Tab wrap within the dialog; Escape and a visible, localized Close/Cancel action exit. If a dirty form needs confirmation, explicitly handle that case rather than silently discarding edits.
- On close, return focus to the invoking control, or a logical surviving control if the row was removed. Never default to an unrelated destructive action.
- Maintain a visible focus indicator on links, inputs, upload triggers and icon buttons. Current queue uploads use a label around a hidden input (`PrintQueue.tsx:183–186,247–250`); specifically test keyboard activation and provide a real focusable trigger if needed.

## Canonical glossary (proposed copy standard)

Use these as the English/pt-BR baseline for catalogs and review. Validate terminology with operators later; no interviews were conducted. Preserve product names (PicoFarm, Etsy), file extensions, G-code, PLA/PETG/ABS, API/OAuth and user-entered names. API enums remain stable regardless of display language.

| English | pt-BR | Meaning / distinction |
| --- | --- | --- |
| Dashboard | Painel | Operational overview |
| Production | Produção | Manufacturing navigation group |
| Commerce | Comercial | Commercial navigation group |
| Print queue / Queue | Fila de impressão / Fila | Printable work waiting or running |
| Queue item | Item da fila | Not automatically a customer order |
| Print job | Trabalho de impressão | One execution, not an order or project |
| Printer / Printer files | Impressora / Arquivos da impressora | Device versus files stored on the device |
| Files / G-code files | Arquivos / Arquivos G-code | Use the specific name when ambiguity matters |
| Slicer / Slice | Fatiador / Fatiar | Tool versus action |
| Print profile | Perfil de impressão | Distinguish printer and filament profiles |
| Material / Filament / Spool | Material / Filamento / Carretel | Material definition, printing feedstock, physical stock unit |
| Ready / Queued | Pronto para imprimir / Na fila | Eligibility versus waiting state |
| Printing / Paused | Imprimindo / Pausado | Device/job states |
| Completed / Failed / Cancelled | Concluído / Falhou / Cancelado | Distinct terminal outcomes |
| Offline / Disconnected | Offline / Desconectado | Do not imply the physical printer is powered off |
| Preflight checks | Verificações antes da impressão | Explain blockers and corrective action |
| Start print / Resume print | Iniciar impressão / Retomar impressão | Starting and resuming are different commands |
| Refresh status / Retry | Atualizar status / Tentar novamente | Reading state versus repeating an operation |
| Project / Task | Projeto / Tarefa | Design/production grouping versus work item |
| Order / Quote | Pedido / Orçamento | Confirmed demand versus proposal |
| Customer / Sales channel | Cliente / Canal de vendas | Customer record versus external provider |
| Expenses / Sales | Despesas / Vendas | Financial records |
| Settings / Integrations | Configurações / Integrações | Preferences versus external connections |
| Notifications / Cameras | Notificações / Câmeras | Keep consistent across navigation and page titles |
| Archives / Timeline | Histórico / Linha do tempo | What the page holds: completed print history versus the schedule view |

### Microcopy rules (proposed)

1. Use sentence case and verb + object for actions: “Save settings” / “Salvar configurações”. Prefer explicit “Delete queue item” over an ambiguous “OK”. Use the same domain term in navigation, headings, dialogs and errors.
2. Localize visible text **and** accessible names, hints, empty states, confirmations, tooltips and status labels. Language selection uses “English” and “Português (Brasil)”, not flags. Keep document language synchronized; test initial load and in-place switching.
3. Use complete plural-aware messages with named values. Format dates/numbers with the selected locale, money with the configured currency, and always show units (g, mm, °C). “Estimated print duration” is clearer than “ETA” when showing a duration rather than a clock time.
4. Distinguish empty, loading, filtered-empty, stale and failed states. A failed request must not look like an empty queue. Show last successful data time when serving stale information.
5. Input error example: “Enter the business email address” / “Informe o e-mail da empresa”. Associate the message with the named field and keep all entered values. Do not blame users for service failures.
6. Service failure example: “Could not save business information. Your changes are still on this page.” / “Não foi possível salvar os dados da empresa. Suas alterações continuam nesta página.” Use this only if the draft really remains available; disclose partial saves when applicable.
7. Ambiguous physical outcome example: “Print request sent; confirmation was not received. Check printer status before trying again.” / “Solicitação de impressão enviada; a confirmação não foi recebida. Verifique o status da impressora antes de tentar novamente.” Use only when request delivery is known; otherwise say the outcome is unknown.
8. Avoid raw exception text as the only explanation. Offer a safe next action and an optional sanitized diagnostic identifier; never expose credentials or connection strings. Do not promise undo for physical actions.

## Measurable validation plan (not executed)

Use synthetic data and simulated printer responses. Record build/revision, locale, browser, viewport/zoom, assistive technology, task outcome, elapsed time, number of errors and screenshots/accessibility-tree evidence. These are proposed acceptance gates, not measured results.

| Scenario | Procedure | Proposed pass criterion |
| --- | --- | --- |
| Language consistency | In English and pt-BR visit navigation, Settings and queue; open details, empty/error/loading states, and switch language without reload; repeat after reload and with storage blocked | Zero unintended mixed-language UI strings or raw translation keys in the scoped walkthrough; selected document `lang` matches displayed primary language. Product names/data are excluded. Blocked storage does not prevent the session working. |
| Modal keyboard cycle | Open details using Enter; inspect initial focus; Tab and Shift+Tab through all controls; press Escape; repeat with long content | Every focus stop visible; zero escapes into background while modal is open; accessible dialog name announced; closure restores focus in all repetitions. |
| Forms and recovery | Navigate business fields by keyboard/screen reader; inject read failure, validation failure and one failed save request | Every field has a persistent accessible name; no silently blanked unknown data; draft retained; failure visible/announced; no false full-save confirmation after partial failure. |
| Unsaved edits | Edit a field, change section, follow a route, use browser Back, then return | No silent loss: a defined warning or retained draft exists for each path; discard is deliberate. No secrets persisted to browser storage. |
| Physical-command uncertainty | Simulate accepted start with lost response, repeated activation, a second item action during the first, and printer disconnection | Record server and simulated device command counts: at most one accepted start for one intended execution; unknown state is explicit; no automatic start resend; refresh only reads state. If backend guarantees cannot satisfy this, block the retry UX and record the limitation. |
| Stale queue | Fail polling after successful load; restore the connection | Last known data stays visible with timestamp and warning; manual refresh is safe; recovery clears warning after success; no repeated announcement of unchanged polling data. |
| Mobile and zoom | Check 320, 375 and 768px widths, 200% and 400% browser zoom, both languages, longest labels and filenames | All navigation destinations and essential actions reachable; no clipped controls or page-wide horizontal scrolling for ordinary forms; no overlapping sticky/overlay content. Document unavoidable large-content scrolling separately. |
| Status/error recognition | With color suppressed, distinguish ready/blocked/active/failed and explain next action for a blocked item | Each state has a readable text cue; failure and corrective action can be identified without relying on color or remembering another page. |

For later user research, recruit actual novice and frequent operators for the same safe tasks. Measure completion, wrong-command attempts and recovery time; ask participants to explain “queue item”, “print job” and “order”. Establish baseline results before setting improvement percentages. A small qualitative study can uncover problems but cannot establish population-wide success rates.

## Delivery sequence and residual risk

1. Complete and verify bilingual copy/catalog coverage across the parallel translation work. Re-check this review's examples against the merged result; translation is not a substitute for interaction fixes.
2. Prioritize unknown printer-command outcomes, silent settings failures and shared modal behavior.
3. Add named controls, persistent feedback and unsaved-edit handling.
4. Validate responsive navigation and information grouping with tasks before reorganizing all menus.
5. Turn reproducible checks into regression tests and update `docs/REGRESSION_MATRIX.md` when behavior changes are actually implemented.

This review did not validate backend command idempotency, real-device behavior, focus styles in a rendered browser, contrast ratios, every modal, all language catalogs or assistive-technology compatibility. A successful build, catalog parity check or automated accessibility scan alone cannot close those gaps. No compliance or production-readiness claim follows from this document.
