import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Calculator, FileCode, Pencil, Printer, RefreshCw } from 'lucide-react'
import { pricingApi } from '../api/client'
import { usePrinters } from '../hooks/usePrinters'
import { useMaterials } from '../hooks/useMaterials'
import { useCurrency } from '../hooks/useCurrency'
import { PRICING_CURRENCIES, formatPricingMoney, syncPricingTargets, pricingCurrencyMatches, legacyPrinterDraft, catalogSecondsToDuration, fullBatchRemainder, normalizePricingDecimal, normalizePricingDuration, pricingCount } from '../lib/pricingCalculator'
import type { PricingRequest, PricingResult } from '../lib/pricingCalculator'
import type { GCodeLibraryFile } from '../types'
import PricingModelPicker from '../components/pricing/PricingModelPicker'
import PricingResults from '../components/pricing/PricingResults'
import PricingAdvancedCosts, { type PricingExtraRow, type PricingFeeRow } from '../components/pricing/PricingAdvancedCosts'
import { CalculatorSection, PricingChip, PricingField, PricingNotice, PricingSegments, PricingSelect } from '../components/pricing/PricingFields'

const initial = { quantity: '1', units: '', grams: '', hours: '', material: '', rate: '', purchase: '', residual: '', lifetime: '', maintenance: '', watts: '', tariff: '', minutes: '', labor: '', reserve: '', shippingCost: '', shippingCharged: '', price: '', gain: '', margin: '30', discount: '', minimum: '' }
type Draft = typeof initial
type Anchor = PricingRequest['target']['anchor']
const printerFields: (keyof Draft)[] = ['rate', 'purchase', 'residual', 'lifetime', 'maintenance', 'watts', 'tariff']

export default function PricingCalculator() {
  const { t, i18n } = useTranslation('pricing')
  const { currencyCode, loading: currencyLoading } = useCurrency()
  const [currencyOverride, setCurrencyOverride] = useState<string | null>(null)
  const currency = currencyOverride ?? currencyCode
  const [draft, setDraft] = useState<Draft>(initial)
  const [full, setFull] = useState(false)
  const [sourceMode, setSourceMode] = useState<'manual' | 'library'>('library')
  const [pickerOpen, setPickerOpen] = useState(true)
  const [piecePricing, setPiecePricing] = useState(false)
  const [production, setProduction] = useState<'proportional' | 'batches'>('proportional')
  const [pendingBasis, setPendingBasis] = useState<typeof production | null>(null)
  const [machineMode, setMachineMode] = useState<'direct' | 'derived'>('direct')
  const [includesEnergy, setIncludesEnergy] = useState(false)
  const [includesMaintenance, setIncludesMaintenance] = useState(false)
  const [anchor, setAnchor] = useState<Anchor>('margin')
  const [extras, setExtras] = useState<PricingExtraRow[]>([])
  const [fees, setFees] = useState<PricingFeeRow[]>([])
  const [result, setResult] = useState<PricingResult | null>(null)
  const [error, setError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [printerId, setPrinterId] = useState('')
  const [materialId, setMaterialId] = useState('')
  const [printerEdited, setPrinterEdited] = useState(false)
  const [materialEdited, setMaterialEdited] = useState(false)
  const [printerCurrency, setPrinterCurrency] = useState('')
  const [legacyPrinter, setLegacyPrinter] = useState(false)
  const [catalogConfirmed, setCatalogConfirmed] = useState(false)
  const [currencyPending, setCurrencyPending] = useState(false)
  const [gcode, setGcode] = useState<GCodeLibraryFile | null>(null)
  const [gcodeConfirmed, setGcodeConfirmed] = useState(false)
  const [costsOpen, setCostsOpen] = useState(false)
  const scopeDrafts = useRef<Partial<Record<'file' | 'unit', { quantity: string; units: string; grams: string; hours: string; production: 'proportional' | 'batches' }>>>({})
  const generation = useRef(0)
  const rowId = useRef(0)
  const controller = useRef<AbortController | null>(null)
  const errorRef = useRef<HTMLDivElement>(null)
  const previousCurrency = useRef(currency)
  const dirtyRef = useRef(false)
  const printers = usePrinters()
  const materials = useMaterials()
  const selectedPrinter = printers.data?.find(item => item.id === printerId)
  const selectedMaterial = materials.data?.find(item => item.id === materialId)
  const money = (value: string, code: string) => formatPricingMoney(value, code, i18n.language)
  const needsCatalogConfirmation = legacyPrinter || !!materialId
  const currencyMismatch = !pricingCurrencyMatches(printerCurrency, currency)
  const activePrinterFields: (keyof Draft)[] = [
    ...(machineMode === 'direct' ? ['rate'] as const : ['purchase', 'residual', 'lifetime'] as const),
    ...(machineMode !== 'direct' || !includesMaintenance ? ['maintenance'] as const : []),
    ...(machineMode !== 'direct' || !includesEnergy ? ['watts', 'tariff'] as const : []),
  ]
  const printerCosts = activePrinterFields.filter(key => draft[key].trim()).length
  const otherCosts = [draft.minutes, draft.labor, draft.reserve, draft.shippingCost, draft.shippingCharged].filter(value => value.trim()).length + extras.length + fees.length
  const hourlyCosts = machineMode === 'derived' || activePrinterFields.some(key => /[1-9]/.test(draft[key]))
  const essentialsMissing = !draft.grams.trim() || !draft.material.trim() || (hourlyCosts && !draft.hours.trim())
  const pieceCountsMissing = piecePricing && (!draft.quantity.trim() || (production === 'batches' && !draft.units.trim()))
  const ready = !essentialsMissing && !pieceCountsMissing && (!piecePricing || !gcode || gcodeConfirmed) && !pendingBasis && !currencyPending && !currencyMismatch && (!needsCatalogConfirmation || catalogConfirmed)

  function invalidate() {
    generation.current++
    controller.current?.abort()
    setResult(null)
    setBusy(false)
    setError('')
    setFieldErrors({})
    dirtyRef.current = true
    setDirty(true)
  }
  function edit(key: keyof Draft, value: string) {
    invalidate()
    setDraft(previous => ({ ...previous, [key]: value }))
    if (printerFields.includes(key) && printerId) setPrinterEdited(true)
    if (key === 'material' && materialId) setMaterialEdited(true)
    if (gcode && ['grams', 'hours', 'units'].includes(key)) setGcodeConfirmed(false)
  }
  useEffect(() => () => { generation.current++; controller.current?.abort() }, [])
  useEffect(() => {
    if (previousCurrency.current === currency) return
    previousCurrency.current = currency
    generation.current++
    controller.current?.abort()
    const needsConfirmation = dirtyRef.current
    const version = generation.current
    queueMicrotask(() => {
      if (generation.current !== version) return
      setResult(null)
      setBusy(false)
      setError('')
      setCatalogConfirmed(false)
      if (needsConfirmation) setCurrencyPending(true)
    })
  }, [currency])
  useEffect(() => {
    if (!dirty) return
    const unload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    const navigate = (event: MouseEvent) => {
      const link = (event.target as Element).closest?.('a[href]')
      if (link && !link.getAttribute('href')?.startsWith('#') && !window.confirm(t('Leave this unsaved calculation?'))) {
        event.preventDefault(); event.stopPropagation()
      }
    }
    window.addEventListener('beforeunload', unload)
    document.addEventListener('click', navigate, true)
    return () => { window.removeEventListener('beforeunload', unload); document.removeEventListener('click', navigate, true) }
  }, [dirty, t])

  function loadPrinter(id: string) {
    invalidate()
    setPrinterId(id)
    setPrinterEdited(false)
    setCatalogConfirmed(false)
    const printer = printers.data?.find(item => item.id === id)
    const config = printer?.pricing_config
    setPrinterCurrency(config?.currency ?? '')
    setLegacyPrinter(!!printer && !config)
    if (!printer) return // Keep local values when detaching a source.
    if (config) {
      setMachineMode(config.rate_mode === 'derived' ? 'derived' : 'direct')
      setIncludesEnergy(config.includes_energy)
      setIncludesMaintenance(config.includes_maintenance)
      setDraft(previous => ({ ...previous, rate: config.direct_rate_per_hour, purchase: config.purchase_price, residual: config.residual_value, lifetime: config.productive_lifetime_hours, maintenance: config.maintenance_per_hour, watts: config.average_power_watts, tariff: config.energy_tariff }))
    } else {
      setMachineMode('direct'); setIncludesEnergy(false); setIncludesMaintenance(false)
      setDraft(previous => ({ ...previous, ...legacyPrinterDraft(printer.cost_per_hour_cents) }))
    }
  }
  function loadMaterial(id: string) {
    invalidate(); setMaterialId(id); setMaterialEdited(false); setCatalogConfirmed(false)
    const material = materials.data?.find(item => item.id === id && item.type !== 'supply')
    if (material) setDraft(previous => ({ ...previous, material: String(material.cost_per_kg) }))
  }
  function loadGCode(file: GCodeLibraryFile) {
    if (piecePricing && !window.confirm(t('Load this whole-file estimate? Your requested quantity is kept, but units per plate must be entered and verified again.'))) return
    invalidate(); setGcode(file); setGcodeConfirmed(false); setProduction('batches'); setPendingBasis(null); setPickerOpen(false)
    // A different source invalidates saved mass/time bases, not optional costs.
    scopeDrafts.current = {}
    setDraft(previous => ({ ...previous, quantity: piecePricing ? previous.quantity : '1', units: piecePricing ? '' : '1', grams: file.filament_grams == null ? '' : String(file.filament_grams), hours: file.estimated_seconds == null ? '' : catalogSecondsToDuration(file.estimated_seconds) }))
    focusControl('pricing-material')
  }
  function changeScope(pieces: boolean) {
    const scopedCosts = !!draft.minutes.trim() || extras.some(row => row.scope === 'unit' && row.amount.trim()) || fees.some(row => row.scope === 'unit' && (row.fixed.trim() || row.percent.trim()))
    if (scopedCosts) {
      if (!window.confirm(t(pieces ? 'Labor minutes, per-unit extras and per-unit fixed fees will now apply to EACH requested piece, not the whole print. Their values are kept and may multiply your costs. Continue and review them?' : 'Labor minutes, per-unit extras and per-unit fixed fees will now apply ONCE to the whole print, not each physical piece. Their values are kept. Continue and review them?'))) return
    } else if (!pieces && !window.confirm(t('Price one whole print run instead? Mass and time must describe the entire run. Other costs are kept.'))) return
    scopeDrafts.current[piecePricing ? 'unit' : 'file'] = { quantity: draft.quantity, units: draft.units, grams: draft.grams, hours: draft.hours, production }
    const saved = scopeDrafts.current[pieces ? 'unit' : 'file']
    invalidate(); setPiecePricing(pieces); setPendingBasis(null); setGcodeConfirmed(false)
    setProduction(saved?.production ?? 'batches')
    setDraft(previous => ({ ...previous, quantity: saved?.quantity ?? (pieces ? '' : '1'), units: saved?.units ?? (pieces ? '' : '1'), grams: saved?.grams ?? previous.grams, hours: saved?.hours ?? previous.hours }))
  }
  function chooseBasis(value: typeof production) {
    if (value === production) return
    invalidate()
    if (draft.grams || draft.hours) setPendingBasis(value)
    else setProduction(value)
  }
  function applyBasis(keep: boolean) {
    if (!pendingBasis) return
    invalidate(); setProduction(pendingBasis); setPendingBasis(null); setGcodeConfirmed(false)
    if (!keep) setDraft(previous => ({ ...previous, grams: '', hours: '', units: '' }))
  }
  function focusControl(id: string) {
    requestAnimationFrame(() => {
      const control = document.getElementById(id)
      if (!control) { errorRef.current?.focus(); return }
      let parent = control.parentElement
      while (parent) { if (parent instanceof HTMLDetailsElement) parent.open = true; parent = parent.parentElement }
      control.focus(); control.scrollIntoView({ block: 'center', behavior: 'smooth' })
    })
  }
  function field(key: keyof Draft, label: string, suffix?: string, help?: string) {
    return <PricingField id={`pricing-${key}`} label={t(label)} value={draft[key]} suffix={suffix} help={help ? t(help) : undefined} inputMode={key === 'hours' ? 'text' : key === 'quantity' || key === 'units' ? 'numeric' : 'decimal'} error={fieldErrors[`pricing-${key}`] ? t(fieldErrors[`pricing-${key}`]) : undefined} onChange={event => edit(key, event.target.value)} />
  }
  async function calculate() {
    generation.current++; controller.current?.abort(); setError(''); setFieldErrors({}); setResult(null)
    const version = generation.current
    const errors: Record<string, string> = {}
    const parse = (id: string, value: string, required = false, signed = false, duration = false) => {
      try { return duration ? normalizePricingDuration(value, required) : normalizePricingDecimal(value, required, signed) }
      catch (failure) { errors[id] = failure instanceof Error ? failure.message : 'Calculation failed.'; return '' }
    }
    const d = (key: keyof Draft, required = false) => parse(`pricing-${key}`, draft[key], required, false, key === 'hours')
    const count = (key: 'quantity' | 'units') => {
      try { return pricingCount(draft[key]) }
      catch (failure) { errors[`pricing-${key}`] = failure instanceof Error ? failure.message : 'Calculation failed.'; return 1 }
    }
    try {
      if (pendingBasis) throw new Error('Confirm the production basis change before calculating.')
      if (currencyMismatch) throw new Error('Printer currency differs. Choose its currency before loading; no conversion is performed.')
      if (currencyPending) throw new Error('Confirm the new currency. Existing amounts are not converted.')
      if (needsCatalogConfirmation && !catalogConfirmed) throw new Error('Confirm catalog amounts are in the selected currency. No conversion is performed.')
      if (piecePricing && gcode && !gcodeConfirmed) throw new Error('Confirm what the G-code quantities represent before calculating.')
      const direct = machineMode === 'direct'
      const request: PricingRequest = {
        currency, quantity: piecePricing ? count('quantity') : 1,
        production: { mode: piecePricing ? production : 'batches', ...(!piecePricing || production === 'batches' ? { units_per_batch: piecePricing ? count('units') : 1 } : {}), grams: d('grams', true), hours: d('hours', hourlyCosts) },
        material_cost_per_kg: d('material', true),
        machine: { mode: machineMode, ...(direct ? { hourly_rate: d('rate'), includes_energy: includesEnergy, includes_maintenance: includesMaintenance } : { acquisition: d('purchase', true), residual: d('residual', true), lifetime_hours: d('lifetime', true) }), ...(!direct || !includesMaintenance ? { maintenance_hourly: d('maintenance') } : {}) },
        ...(!direct || !includesEnergy ? { energy: { power_watts: d('watts'), tariff_per_kwh: d('tariff') } } : {}),
        labor: { minutes: d('minutes'), hourly_rate: d('labor') }, reserve: d('reserve'), shipping: { cost: d('shippingCost'), charged: d('shippingCharged') },
        extras: extras.map(({ id, ...extra }) => ({ ...extra, amount: parse(`extra-${id}-amount`, extra.amount, true) })),
        fees: fees.map(({ id, ...fee }) => ({ ...fee, percent: parse(`fee-${id}-percent`, fee.percent, true), fixed: parse(`fee-${id}-fixed`, fee.fixed, true) })),
        target: { anchor, value: parse(`pricing-${anchor}`, draft[anchor], true, anchor !== 'price') }, discount_percent: d('discount'), min_charge: d('minimum'),
      }
      if (Object.keys(errors).length) {
        setFieldErrors(errors); setCostsOpen(true)
        focusControl(Object.keys(errors)[0])
        throw new Error('Review the highlighted fields before calculating.')
      }
      controller.current = new AbortController(); setBusy(true)
      const response = await pricingApi.calculate(request, controller.current.signal)
      if (generation.current !== version) return
      setDraft(previous => syncPricingTargets(previous, anchor, response.base))
      setResult(response); setBusy(false)
    } catch (failure) {
      if (generation.current !== version || (failure instanceof Error && failure.name === 'AbortError')) return
      setError(failure instanceof Error ? failure.message : 'Calculation failed.'); setBusy(false)
      if (!Object.keys(errors).length) requestAnimationFrame(() => errorRef.current?.focus())
    }
  }

  const checkboxClass = 'flex min-h-11 items-center gap-3 text-sm text-surface-300'
  return (
    <div className="mx-auto max-w-[1440px] min-w-0 space-y-5 pb-24 text-surface-100">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="mb-1 flex items-center gap-2 text-xs font-medium uppercase tracking-widest text-accent-400"><Calculator className="h-4 w-4" />{t('Estimate before you produce')}</div>
          <h1 className="text-2xl font-semibold tracking-tight">{t('Pricing calculator')}</h1>
          <p className="mt-1 max-w-xl text-sm text-surface-400">{t('Choose a G-code and enter your filament price. Add other costs only when you need them.')}</p>
        </div>
        <div className="grid w-full gap-3 sm:w-auto sm:grid-cols-[minmax(180px,1fr)_110px]">
          <PricingSegments label={t('Calculator mode')} value={full ? 'full' : 'quick'} options={[{ value: 'quick', label: t('Quick mode') }, { value: 'full', label: t('Full mode') }]} onChange={value => setFull(value === 'full')} compact />
          <PricingSelect id="pricing-currency" label={<span className="sr-only">{t('Currency')}</span>} value={currency} options={PRICING_CURRENCIES.map(code => ({ value: code, label: code }))} onChange={event => { invalidate(); setCurrencyOverride(event.target.value); setCurrencyPending(true); setCatalogConfirmed(false) }} />
        </div>
      </header>
      {currencyPending && <PricingNotice tone="amber"><p>{t('Existing amounts will be interpreted in {{currency}}. No conversion is performed.', { currency })}</p><button type="button" className="btn btn-secondary mt-2 min-h-11" onClick={() => { invalidate(); setCurrencyPending(false) }}>{t('Confirm this currency')}</button></PricingNotice>}
      <form onSubmit={event => { event.preventDefault(); void calculate() }} noValidate className="grid min-w-0 items-start gap-5 xl:grid-cols-[minmax(0,1fr)_370px]">
        <div className="min-w-0 space-y-5 xl:col-start-1 xl:row-start-1">
          <CalculatorSection id="production" step="1" title={t('Production')}>
            <div className="space-y-5">
              {(!gcode || pickerOpen) && <>
                <PricingSegments label={t('Production estimate method')} value={sourceMode} options={[{ value: 'library', label: t('From an STL or G-code'), icon: <FileCode className="h-4 w-4 shrink-0" /> }, { value: 'manual', label: t('Enter manually'), icon: <Pencil className="h-4 w-4 shrink-0" /> }]} onChange={value => { setSourceMode(value); if (value === 'manual' && gcode) { invalidate(); setGcode(null); setGcodeConfirmed(false); setPickerOpen(true) } }} />
                {sourceMode === 'library' && <PricingModelPicker key={piecePricing ? 'pieces' : 'whole-print'} selectedGCodeId={gcode?.id} onSelectGCode={loadGCode} />}
                {gcode && <button type="button" className="btn btn-ghost min-h-11" onClick={() => setPickerOpen(false)}>{t('Keep selected file')}</button>}
              </>}
              {gcode && <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-surface-700 p-3"><div className="min-w-0"><p className="break-words text-sm font-medium">{gcode.display_name || gcode.file_name}</p><p className="mt-1 text-xs text-surface-400">{t('{{grams}} g · {{duration}}', { grams: draft.grams || '—', duration: draft.hours ? t('{{duration}} h', { duration: draft.hours }) : '—' })}</p></div><button type="button" className="btn btn-secondary min-h-11" onClick={() => { setSourceMode('library'); setPickerOpen(true) }}>{t('Change file')}</button></div>}
              {gcode && !pickerOpen && <button type="button" className="min-h-11 text-sm text-surface-400 underline" onClick={() => { invalidate(); setGcode(null); setGcodeConfirmed(false); setSourceMode('manual'); setPickerOpen(true); focusControl('pricing-grams') }}>{t('Use manual estimates')}</button>}
              {!piecePricing && <p className="text-sm text-surface-400">{t(gcode ? 'Estimate for the entire G-code. It may contain several pieces.' : 'Estimate for one whole print run, not one physical piece.')}</p>}
              {piecePricing && <>
                <PricingNotice tone="amber">{t('Enter the actual piece counts. File estimates describe the whole plate; do not treat them as per-piece values without adjusting them.')}</PricingNotice>
                <div className="grid gap-4 sm:grid-cols-2">
                  {field('quantity', 'Requested units')}
                  <PricingSelect id="pricing-basis" label={t('Production model')} value={production} options={[{ value: 'proportional', label: t('Proportional per unit') }, { value: 'batches', label: t('Full plate batches') }]} onChange={event => chooseBasis(event.target.value as typeof production)} />
                </div>
                {production === 'batches' && field('units', 'Units per full plate')}
                {gcode && <label className={checkboxClass}><input type="checkbox" checked={gcodeConfirmed} onChange={event => { invalidate(); setGcodeConfirmed(event.target.checked) }} />{t('I verified the mass, time and units per plate, or changed them to per-unit values.')}</label>}
              </>}
              {pendingBasis && <PricingNotice tone="amber" role="status"><p>{t('Changing the basis changes what mass and time mean. Keep the numbers only if they already describe the new basis.')}</p><div className="mt-2 flex flex-wrap gap-2"><button type="button" className="btn btn-primary min-h-11" onClick={() => applyBasis(false)}>{t('Clear mass and time')}</button><button type="button" className="btn btn-secondary min-h-11" onClick={() => applyBasis(true)}>{t('Keep and reinterpret values')}</button><button type="button" className="btn btn-ghost min-h-11" onClick={() => setPendingBasis(null)}>{t('Cancel change')}</button></div></PricingNotice>}
              <details open={!gcode || piecePricing || full || !draft.grams.trim() || (hourlyCosts && !draft.hours.trim()) || undefined}>
                <summary className="cursor-pointer text-sm text-surface-300">{t(gcode ? 'Edit file estimates' : 'Print mass and duration')}</summary>
                <div className="mt-4 grid gap-4 sm:grid-cols-2">
                  {field('grams', !piecePricing ? 'Grams for the whole print' : production === 'batches' ? 'Grams per full plate' : 'Grams per unit', 'g')}
                  {(!gcode || draft.grams.trim() || hourlyCosts || piecePricing) && field('hours', !piecePricing ? 'Duration for the whole print' : production === 'batches' ? 'Duration per full plate' : 'Duration per unit', undefined, hourlyCosts ? 'Duration is required for hourly costs.' : 'Optional until hourly costs are added. Use 1:30 or decimal hours such as 1.5.')}
                </div>
              </details>
              <button type="button" className="min-h-11 text-sm text-accent-400 underline underline-offset-4" onClick={() => changeScope(!piecePricing)}>{t(piecePricing ? 'Price one whole print run instead' : 'Price individual pieces or batches')}</button>
              {piecePricing && production === 'batches' && fullBatchRemainder(draft.quantity, draft.units) && <PricingNotice tone="amber">{t('Remainder charged as a full plate, including its full mass and time.')}</PricingNotice>}
            </div>
          </CalculatorSection>

          <CalculatorSection id="material" step="2" title={t('Material')}>
            {field('material', 'Material cost per kg', `${currency}/kg`)}
            <details className="mt-3" open={full || undefined}>
              <summary className="cursor-pointer text-sm text-surface-400">{t('Load material rate')}</summary>
              <div className="mt-3 space-y-3">
                <PricingSelect id="pricing-material-source" label={t('Load material rate')} value={materialId} options={[{ value: '', label: t('Manual material cost') }, ...(materials.data ?? []).filter(material => material.type !== 'supply').map(material => ({ value: material.id, label: material.name }))]} disabled={materials.isPending} onChange={event => loadMaterial(event.target.value)} />
                {materials.isError && <p className="text-sm text-surface-400">{t('Materials could not load. Enter the rate manually or retry.')} <button type="button" className="underline" onClick={() => void materials.refetch()}>{t('Try again')}</button></p>}
                {selectedMaterial && <PricingChip tone={materialEdited ? 'amber' : 'accent'}>{t(materialEdited ? 'Modified for this estimate' : 'Loaded from material')}</PricingChip>}
              </div>
            </details>
            {needsCatalogConfirmation && <label className={`${checkboxClass} mt-3`}><input type="checkbox" checked={catalogConfirmed} onChange={event => { invalidate(); setCatalogConfirmed(event.target.checked) }} />{t('I confirm loaded amounts use {{currency}}. Legacy currency is unknown; no conversion.', { currency })}</label>}
          </CalculatorSection>
        </div>
        <aside className="min-w-0 space-y-5 xl:col-start-2 xl:row-span-2 xl:row-start-1 xl:sticky xl:top-5" aria-label={t('Selling target and estimate')}>
          <CalculatorSection title={t('Selling target')} icon={<Calculator className="h-5 w-5" />}>
            <div className="space-y-4">
              {field(anchor, anchor === 'price' ? 'Goods price target' : anchor === 'gain' ? 'Gain target' : 'Margin target percent', anchor === 'margin' ? '%' : currency)}
              {anchor === 'margin' && <p className="text-xs text-surface-400">{t('The starting margin is 30%. Change it to your target; it is not markup or net profit.')}</p>}
              <details open={full || undefined} className="rounded-xl border border-surface-700 p-3">
                <summary className="cursor-pointer text-sm text-surface-300">{t('Customize selling target')} · {t('{{count}} entered adjustments', { count: [draft.minimum, draft.discount].filter(value => value.trim()).length })}</summary>
                <div className="mt-4 space-y-4">
                  <PricingSegments label={t('Active anchor')} value={anchor} options={[{ value: 'margin', label: t('Margin') }, { value: 'price', label: t('Price') }, { value: 'gain', label: t('Gain') }]} onChange={value => { invalidate(); setAnchor(value) }} compact />
                  <p className="text-xs text-surface-400">{t('Price and gain targets apply to the whole order. Margin is not markup or accounting net profit.')}</p>
                  {field('minimum', 'Minimum service charge for goods', currency)}{field('discount', 'Conditional discount percent', '%')}
                  <p className="text-xs text-surface-400">{t('Same costs and fees, lower goods price. This is a scenario, not a guaranteed payment-method saving.')}</p>
                </div>
              </details>
              {error && <div ref={errorRef} tabIndex={-1}><PricingNotice tone="red" role="alert">{t(error)}</PricingNotice></div>}
              <p role="status" className="text-sm text-surface-400">{t(essentialsMissing ? 'Enter mass and material cost per kg. Add duration if using hourly costs.' : !ready ? 'Complete the piece counts and any pending source or currency confirmations.' : 'Ready to calculate. Optional costs can be added below.')}</p>
              <button type="submit" disabled={busy || currencyLoading || !ready} className="btn btn-primary min-h-12 w-full gap-2">{busy ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Calculator className="h-4 w-4" />}{busy ? t('Calculating…') : t('Calculate estimate')}</button>
            </div>
          </CalculatorSection>
          {result ? <PricingResults result={result} full={full} basic={!full} scope={piecePricing ? 'unit' : 'file'} money={money} /> : <div role="status" className="rounded-2xl border border-dashed border-surface-700 p-5 text-sm leading-6 text-surface-400">{t('Your estimate will appear here.')}</div>}
        </aside>
        <details className="min-w-0 rounded-2xl border border-surface-700 p-4 xl:col-start-1 xl:row-start-2" open={full || undefined}>
          <summary className="cursor-pointer text-sm font-medium text-surface-300">{t('Refine with optional costs')} · {t('{{count}} entered cost inputs', { count: printerCosts + otherCosts })}</summary>
          <p className="mt-2 text-xs text-surface-400">{t('Entered costs remain included when collapsed.')}</p>
          <div className="mt-4 space-y-4">
          <CalculatorSection id="machine" title={t('Printer and energy')}>
            <p className="mb-3 text-xs text-surface-400">{t('Entered costs always apply, even when this section is collapsed. Empty is unknown; zero means intentionally free.')}</p>
            <details open={full || undefined}>
              <summary className="cursor-pointer text-sm font-medium text-surface-300">{t('Printer and energy')} · {t('{{count}} entered cost inputs', { count: printerCosts })}</summary>
              <div className="mt-4 space-y-3">
                <PricingSelect id="pricing-printer" label={t('Load printer rates')} value={printerId} options={[{ value: '', label: t('Manual printer costs') }, ...(printers.data ?? []).map(printer => ({ value: printer.id, label: printer.name }))]} disabled={printers.isPending} onChange={event => loadPrinter(event.target.value)} />
                {printers.isError && <PricingNotice tone="amber"><p>{t('Printer rates could not load. Enter costs manually or retry.')}</p><button type="button" className="underline" onClick={() => void printers.refetch()}>{t('Try again')}</button></PricingNotice>}
                {selectedPrinter && <PricingChip tone={printerEdited ? 'amber' : 'accent'}><Printer className="h-3.5 w-3.5" />{t(printerEdited ? 'Modified for this estimate' : 'Loaded from printer')}</PricingChip>}
                {machineMode === 'direct' ? field('rate', 'Machine rate per hour', `${currency}/h`) : <PricingNotice>{t('The hourly machine cost is derived from acquisition, residual value and productive lifetime below.')}</PricingNotice>}
                <div className="grid gap-4 sm:grid-cols-2">
                  {(machineMode !== 'direct' || !includesMaintenance) && field('maintenance', 'Maintenance per hour', `${currency}/h`)}
                  {(machineMode !== 'direct' || !includesEnergy) && <>{field('watts', 'Average power watts', 'W')}{field('tariff', 'Energy tariff per kWh', currency)}</>}
                </div>
              </div>
            {currencyMismatch && <div className="mt-4"><PricingNotice tone="red">{t('This printer uses {{currency}}. Match the calculator currency or choose manual printer costs; no conversion is performed.', { currency: printerCurrency })}</PricingNotice></div>}
            <details className="mt-5 rounded-xl border border-surface-700 p-3" open={full || undefined}>
              <summary className="cursor-pointer text-sm font-medium text-surface-300">{t('Machine cost settings')} · {t(machineMode === 'direct' ? 'Direct hourly rate' : 'Derived depreciation')}</summary>
              <div className="mt-4 space-y-4">
                <PricingSelect id="pricing-machine-mode" label={t('Machine rate model')} value={machineMode} options={[{ value: 'direct', label: t('Direct hourly rate') }, { value: 'derived', label: t('Derived depreciation') }]} onChange={event => { invalidate(); setMachineMode(event.target.value as typeof machineMode); setPrinterEdited(!!printerId) }} />
                {machineMode === 'derived' && <div className="grid gap-4 sm:grid-cols-2">{field('purchase', 'Purchase price', currency)}{field('residual', 'Residual value', currency)}{field('lifetime', 'Productive lifetime hours', 'h')}</div>}
                {machineMode === 'direct' && <div><label className={checkboxClass}><input type="checkbox" checked={includesEnergy} onChange={event => { invalidate(); setIncludesEnergy(event.target.checked); setPrinterEdited(!!printerId) }} />{t('Hourly rate already includes energy')}</label><label className={checkboxClass}><input type="checkbox" checked={includesMaintenance} onChange={event => { invalidate(); setIncludesMaintenance(event.target.checked); setPrinterEdited(!!printerId) }} />{t('Hourly rate already includes maintenance')}</label></div>}
                <p className="text-xs text-surface-400">{t('Changes here apply only to this estimate, not to the printer or material catalog.')}</p>
              </div>
            </details>
            </details>
          </CalculatorSection>

          <CalculatorSection id="additional-costs" title={t('Labor, shipping and sales fees')} action={<PricingChip tone="accent">{t('{{count}} entered cost inputs', { count: otherCosts })}</PricingChip>}>
            <details open={full || costsOpen} onToggle={event => { if (!full) setCostsOpen(event.currentTarget.open) }}>
              <summary className="cursor-pointer text-sm text-surface-300">{t('Review energy, labor, shipping and sales fees')}</summary>
              <p className="mt-2 text-xs text-surface-400">{t('Entered costs always apply, even when this section is collapsed. Empty is unknown; zero means intentionally free.')}</p>
              <div className="mt-4 space-y-5">
                <div className="grid gap-4 sm:grid-cols-2">
                  {field('minutes', piecePricing ? 'Labor minutes per requested unit' : 'Labor minutes for the whole print', 'min')}{field('labor', 'Labor rate per hour', `${currency}/h`)}
                  {field('reserve', 'Fixed order reserve', currency)}{field('shippingCost', 'Shipping cost', currency)}{field('shippingCharged', 'Shipping charged', currency)}
                </div>
                <PricingAdvancedCosts scope={piecePricing ? 'unit' : 'file'} currency={currency} extras={extras} fees={fees} errors={fieldErrors}
                  onExtraChange={(index, patch) => { invalidate(); setExtras(rows => rows.map((row, i) => i === index ? { ...row, ...patch } : row)) }}
                  onExtraAdd={() => { invalidate(); const id = String(++rowId.current); setExtras(rows => [...rows, { id, label: '', amount: '', scope: 'order' }]); focusControl(`extra-${id}-label`) }}
                  onExtraRemove={index => { invalidate(); setExtras(rows => rows.filter((_, i) => i !== index)); focusControl('add-extra-cost') }}
                  onFeeChange={(index, patch) => { invalidate(); setFees(rows => rows.map((row, i) => i === index ? { ...row, ...patch } : row)) }}
                  onFeeAdd={() => { invalidate(); const id = String(++rowId.current); setFees(rows => [...rows, { id, percent: '0', fixed: '0', scope: 'order', base: 'goods' }]); focusControl(`fee-${id}-percent`) }}
                  onFeeRemove={index => { invalidate(); setFees(rows => rows.filter((_, i) => i !== index)); focusControl('add-fee') }} />
              </div>
            </details>
          </CalculatorSection>
          </div>
        </details>
      </form>
      <details className="text-xs text-surface-400"><summary className="cursor-pointer">{t('How this estimate works')}</summary><p className="mt-2 max-w-3xl leading-6">{t('Quick and full modes use the same engine and retain all entered costs. Price and gain targets are order goods totals. Margin is gain divided by goods revenue. Shipping charged is separate. Empty costs produce warnings; enter zero only when intentionally free. Drafts stay in memory only.')}</p></details>
    </div>
  )
}
