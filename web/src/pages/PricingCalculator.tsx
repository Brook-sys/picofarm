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
  const [sourceMode, setSourceMode] = useState<'manual' | 'library'>('manual')
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
  const activeCosts = [draft.maintenance, draft.watts, draft.tariff, draft.minutes, draft.labor, draft.reserve, draft.shippingCost, draft.shippingCharged].filter(value => value.trim()).length + extras.length + fees.length

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
    invalidate(); setGcode(file); setGcodeConfirmed(false); setProduction('batches'); setPendingBasis(null)
    setDraft(previous => ({ ...previous, units: '', grams: file.filament_grams == null ? '' : String(file.filament_grams), hours: file.estimated_seconds == null ? '' : catalogSecondsToDuration(file.estimated_seconds) }))
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
      if (gcode && !gcodeConfirmed) throw new Error('Confirm what the G-code quantities represent before calculating.')
      const direct = machineMode === 'direct'
      const request: PricingRequest = {
        currency, quantity: count('quantity'),
        production: { mode: production, ...(production === 'batches' ? { units_per_batch: count('units') } : {}), grams: d('grams'), hours: d('hours') },
        material_cost_per_kg: d('material'),
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
          <p className="mt-1 max-w-xl text-sm text-surface-400">{t('Choose how to estimate production, review your costs and set your selling target.')}</p>
        </div>
        <div className="grid w-full gap-3 sm:w-auto sm:grid-cols-[minmax(180px,1fr)_110px]">
          <PricingSegments label={t('Calculator mode')} value={full ? 'full' : 'quick'} options={[{ value: 'quick', label: t('Quick mode') }, { value: 'full', label: t('Full mode') }]} onChange={value => setFull(value === 'full')} compact />
          <PricingSelect id="pricing-currency" label={<span className="sr-only">{t('Currency')}</span>} value={currency} options={PRICING_CURRENCIES.map(code => ({ value: code, label: code }))} onChange={event => { invalidate(); setCurrencyOverride(event.target.value); setCurrencyPending(true); setCatalogConfirmed(false) }} />
        </div>
      </header>
      {currencyPending && <PricingNotice tone="amber"><p>{t('Existing amounts will be interpreted in {{currency}}. No conversion is performed.', { currency })}</p><button type="button" className="btn btn-secondary mt-2 min-h-11" onClick={() => { invalidate(); setCurrencyPending(false) }}>{t('Confirm this currency')}</button></PricingNotice>}
      <form onSubmit={event => { event.preventDefault(); void calculate() }} noValidate className="grid min-w-0 items-start gap-5 xl:grid-cols-[minmax(0,1fr)_370px]">
        <div className="min-w-0 space-y-5">
          <CalculatorSection id="production" step="1" title={t('Production')}>
            <div className="space-y-5">
              <PricingSegments label={t('Production estimate method')} value={sourceMode} options={[{ value: 'manual', label: t('Enter manually'), icon: <Pencil className="h-4 w-4 shrink-0" /> }, { value: 'library', label: t('From an STL or G-code'), icon: <FileCode className="h-4 w-4 shrink-0" /> }]} onChange={value => { setSourceMode(value); if (value === 'manual') { invalidate(); setGcode(null); setGcodeConfirmed(false) } }} />
              {sourceMode === 'library' && <PricingModelPicker selectedGCodeId={gcode?.id} onSelectGCode={loadGCode} />}
              {gcode && <PricingNotice tone="neutral"><p className="font-medium break-words">{t('G-code source: {{name}}. Copy count is unknown.', { name: gcode.display_name || gcode.file_name })}</p><label className={checkboxClass}><input type="checkbox" checked={gcodeConfirmed} onChange={event => { invalidate(); setGcodeConfirmed(event.target.checked) }} />{t('I verified the mass, time and units per plate, or changed them to per-unit values.')}</label></PricingNotice>}
              <div className="grid gap-4 sm:grid-cols-2">
                {field('quantity', 'Requested units')}
                <PricingSelect id="pricing-basis" label={t('Production model')} value={production} options={[{ value: 'proportional', label: t('Proportional per unit') }, { value: 'batches', label: t('Full plate batches') }]} onChange={event => chooseBasis(event.target.value as typeof production)} />
              </div>
              {pendingBasis && <PricingNotice tone="amber" role="status"><p>{t('Changing the basis changes what mass and time mean. Keep the numbers only if they already describe the new basis.')}</p><div className="mt-2 flex flex-wrap gap-2"><button type="button" className="btn btn-primary min-h-11" onClick={() => applyBasis(false)}>{t('Clear mass and time')}</button><button type="button" className="btn btn-secondary min-h-11" onClick={() => applyBasis(true)}>{t('Keep and reinterpret values')}</button><button type="button" className="btn btn-ghost min-h-11" onClick={() => setPendingBasis(null)}>{t('Cancel change')}</button></div></PricingNotice>}
              <div className="grid gap-4 sm:grid-cols-2">
                {production === 'batches' && field('units', 'Units per full plate')}
                {field('grams', production === 'batches' ? 'Grams per full plate' : 'Grams per unit', 'g')}
                {field('hours', production === 'batches' ? 'Duration per full plate' : 'Duration per unit', undefined, 'Use 1:30 for one hour and thirty minutes, or decimal hours such as 1.5.')}
              </div>
              {production === 'batches' && fullBatchRemainder(draft.quantity, draft.units) && <PricingNotice tone="amber">{t('Remainder charged as a full plate, including its full mass and time.')}</PricingNotice>}
            </div>
          </CalculatorSection>

          <CalculatorSection id="machine" step="2" title={t('Printer and material')}>
            <div className="grid gap-5 md:grid-cols-2">
              <div className="space-y-3">
                <PricingSelect id="pricing-printer" label={t('Load printer rates')} value={printerId} options={[{ value: '', label: t('Manual printer costs') }, ...(printers.data ?? []).map(printer => ({ value: printer.id, label: printer.name }))]} disabled={printers.isPending} onChange={event => loadPrinter(event.target.value)} />
                {printers.isError && <PricingNotice tone="amber"><p>{t('Printer rates could not load. Enter costs manually or retry.')}</p><button type="button" className="underline" onClick={() => void printers.refetch()}>{t('Try again')}</button></PricingNotice>}
                {selectedPrinter && <PricingChip tone={printerEdited ? 'amber' : 'accent'}><Printer className="h-3.5 w-3.5" />{t(printerEdited ? 'Modified for this estimate' : 'Loaded from printer')}</PricingChip>}
                {machineMode === 'direct' ? field('rate', 'Machine rate per hour', `${currency}/h`) : <PricingNotice>{t('The hourly machine cost is derived from acquisition, residual value and productive lifetime below.')}</PricingNotice>}
                {!draft.rate && machineMode === 'direct' && <p className="text-xs text-amber-300">{t('Machine hourly rate is missing.')}</p>}
              </div>
              <div className="space-y-3">
                <PricingSelect id="pricing-material-source" label={t('Load material rate')} value={materialId} options={[{ value: '', label: t('Manual material cost') }, ...(materials.data ?? []).filter(material => material.type !== 'supply').map(material => ({ value: material.id, label: material.name }))]} disabled={materials.isPending} onChange={event => loadMaterial(event.target.value)} />
                {materials.isError && <PricingNotice tone="amber"><p>{t('Materials could not load. Enter the rate manually or retry.')}</p><button type="button" className="underline" onClick={() => void materials.refetch()}>{t('Try again')}</button></PricingNotice>}
                {selectedMaterial && <PricingChip tone={materialEdited ? 'amber' : 'accent'}>{t(materialEdited ? 'Modified for this estimate' : 'Loaded from material')}</PricingChip>}
                {field('material', 'Material cost per kg', `${currency}/kg`)}
                {!draft.material && <p className="text-xs text-amber-300">{t('Material rate is missing.')}</p>}
              </div>
            </div>
            {currencyMismatch && <div className="mt-4"><PricingNotice tone="red">{t('This printer uses {{currency}}. Match the calculator currency or choose manual printer costs; no conversion is performed.', { currency: printerCurrency })}</PricingNotice></div>}
            {needsCatalogConfirmation && <label className={`${checkboxClass} mt-4 rounded-xl border border-amber-600/40 bg-amber-500/5 p-3`}><input type="checkbox" checked={catalogConfirmed} onChange={event => { invalidate(); setCatalogConfirmed(event.target.checked) }} />{t('I confirm loaded amounts use {{currency}}. Legacy currency is unknown; no conversion.', { currency })}</label>}
            <details className="mt-5 rounded-xl border border-surface-700 p-3" open={full || undefined}>
              <summary className="cursor-pointer text-sm font-medium text-surface-300">{t('Machine cost settings')} · {t(machineMode === 'direct' ? 'Direct hourly rate' : 'Derived depreciation')}</summary>
              <div className="mt-4 space-y-4">
                <PricingSelect id="pricing-machine-mode" label={t('Machine rate model')} value={machineMode} options={[{ value: 'direct', label: t('Direct hourly rate') }, { value: 'derived', label: t('Derived depreciation') }]} onChange={event => { invalidate(); setMachineMode(event.target.value as typeof machineMode); setPrinterEdited(!!printerId) }} />
                {machineMode === 'derived' && <div className="grid gap-4 sm:grid-cols-2">{field('purchase', 'Purchase price', currency)}{field('residual', 'Residual value', currency)}{field('lifetime', 'Productive lifetime hours', 'h')}</div>}
                {machineMode === 'direct' && <div><label className={checkboxClass}><input type="checkbox" checked={includesEnergy} onChange={event => { invalidate(); setIncludesEnergy(event.target.checked); setPrinterEdited(!!printerId) }} />{t('Hourly rate already includes energy')}</label><label className={checkboxClass}><input type="checkbox" checked={includesMaintenance} onChange={event => { invalidate(); setIncludesMaintenance(event.target.checked); setPrinterEdited(!!printerId) }} />{t('Hourly rate already includes maintenance')}</label></div>}
                <p className="text-xs text-surface-400">{t('Changes here apply only to this estimate, not to the printer or material catalog.')}</p>
              </div>
            </details>
          </CalculatorSection>

          <CalculatorSection id="additional-costs" step="3" title={t('Additional costs')} action={<PricingChip tone={activeCosts ? 'accent' : 'amber'}>{t('{{count}} entered cost inputs', { count: activeCosts })}</PricingChip>}>
            <details open={full || costsOpen} onToggle={event => { if (!full) setCostsOpen(event.currentTarget.open) }}>
              <summary className="cursor-pointer text-sm text-surface-300">{t('Review energy, labor, shipping and sales fees')}</summary>
              <p className="mt-2 text-xs text-surface-400">{t('Entered costs always apply, even when this section is collapsed. Empty is unknown; zero means intentionally free.')}</p>
              <div className="mt-4 space-y-5">
                <div className="grid gap-4 sm:grid-cols-2">
                  {(machineMode !== 'direct' || !includesMaintenance) && field('maintenance', 'Maintenance per hour', `${currency}/h`)}
                  {(machineMode !== 'direct' || !includesEnergy) && <>{field('watts', 'Average power watts', 'W')}{field('tariff', 'Energy tariff per kWh', currency)}</>}
                  {field('minutes', 'Labor minutes per requested unit', 'min')}{field('labor', 'Labor rate per hour', `${currency}/h`)}
                  {field('reserve', 'Fixed order reserve', currency)}{field('shippingCost', 'Shipping cost', currency)}{field('shippingCharged', 'Shipping charged', currency)}
                </div>
                <PricingAdvancedCosts currency={currency} extras={extras} fees={fees} errors={fieldErrors}
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

        <aside className="min-w-0 space-y-5 xl:sticky xl:top-5" aria-label={t('Selling target and estimate')}>
          <CalculatorSection title={t('Selling target')} icon={<Calculator className="h-5 w-5" />}>
            <div className="space-y-4">
              <PricingSegments label={t('Active anchor')} value={anchor} options={[{ value: 'margin', label: t('Margin') }, { value: 'price', label: t('Price') }, { value: 'gain', label: t('Gain') }]} onChange={value => { invalidate(); setAnchor(value) }} compact />
              {field(anchor, anchor === 'price' ? 'Goods price target' : anchor === 'gain' ? 'Gain target' : 'Margin target percent', anchor === 'margin' ? '%' : currency)}
              <p className="text-xs leading-5 text-surface-400">{t('Price and gain targets apply to the whole order. Margin is not markup or accounting net profit.')}</p>
              <details open={full || undefined} className="rounded-xl border border-surface-700 p-3">
                <summary className="cursor-pointer text-sm text-surface-300">{t('Minimum charge and discount')}{draft.minimum || draft.discount ? ' •' : ''}</summary>
                <div className="mt-4 space-y-4">{field('minimum', 'Minimum service charge for goods', currency)}{field('discount', 'Conditional discount percent', '%')}<p className="text-xs text-surface-400">{t('Same costs and fees, lower goods price. This is a scenario, not a guaranteed payment-method saving.')}</p></div>
              </details>
              {error && <div ref={errorRef} tabIndex={-1}><PricingNotice tone="red" role="alert">{t(error)}</PricingNotice></div>}
              <button type="submit" disabled={busy || currencyLoading} className="btn btn-primary min-h-12 w-full gap-2">{busy ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Calculator className="h-4 w-4" />}{busy ? t('Calculating…') : t('Calculate estimate')}</button>
            </div>
          </CalculatorSection>
          {result ? <PricingResults result={result} full={full} money={money} /> : <div role="status" className="rounded-2xl border border-dashed border-surface-700 p-5 text-sm leading-6 text-surface-400">{t('No current result. Calculate after editing to refresh the estimate.')}</div>}
        </aside>
      </form>
      <details className="text-xs text-surface-400"><summary className="cursor-pointer">{t('How this estimate works')}</summary><p className="mt-2 max-w-3xl leading-6">{t('Quick and full modes use the same engine and retain all entered costs. Price and gain targets are order goods totals. Margin is gain divided by goods revenue. Shipping charged is separate. Empty costs produce warnings; enter zero only when intentionally free. Drafts stay in memory only.')}</p></details>
    </div>
  )
}
