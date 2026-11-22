import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { gcodeLibraryApi, pricingApi } from '../api/client'
import { usePrinters } from '../hooks/usePrinters'
import { useMaterials } from '../hooks/useMaterials'
import { useCurrency } from '../hooks/useCurrency'
import { PRICING_CURRENCIES, formatPricingMoney, syncPricingTargets, pricingCurrencyMatches, legacyPrinterDraft, catalogSecondsToHours, fullBatchRemainder, normalizePricingDecimal, pricingCount } from '../lib/pricingCalculator'
import type { PricingExtra, PricingFee, PricingRequest, PricingResult, PricingScenario } from '../lib/pricingCalculator'

const initial = { quantity: '1', units: '', grams: '', hours: '', material: '', rate: '', purchase: '', residual: '', lifetime: '', maintenance: '', watts: '', tariff: '', minutes: '', labor: '', reserve: '', shippingCost: '', shippingCharged: '', price: '', gain: '', margin: '30', discount: '', minimum: '' }
type Draft = typeof initial
const warningMessages: Record<string, string> = {
  proportional_production_estimate: 'Proportional production is an estimate.', full_batch_remainder: 'The remainder uses a full plate.',
  missing_grams: 'Mass is missing.', missing_hours: 'Production time is missing.', missing_material_cost_per_kg: 'Material rate is missing.',
  missing_machine: 'Machine cost is missing.', missing_machine_hourly_rate: 'Machine hourly rate is missing.', missing_maintenance_hourly: 'Maintenance is missing.',
  missing_energy: 'Energy cost is missing.', missing_power_watts: 'Power is missing.', missing_tariff_per_kwh: 'Energy tariff is missing.',
  missing_labor: 'Labor cost is missing.', missing_labor_minutes: 'Labor time is missing.', missing_labor_hourly_rate: 'Labor rate is missing.',
  missing_reserve: 'Reserve is missing.', missing_shipping: 'Shipping is missing.', missing_shipping_cost: 'Shipping cost is missing.', missing_shipping_charged: 'Shipping charged is missing.',
  missing_extras: 'Extra costs are missing.', missing_fees: 'Sales fees are missing.', minimum_charge_applied: 'Minimum service charge was applied.',
  price_below_minimum_charge: 'Price is below the minimum service charge.', negative_gain: 'The base scenario has a loss.', discounted_loss: 'The discounted scenario has a loss.',
}
const inputClass = 'w-full min-w-0 rounded-lg border border-surface-700 bg-surface-950 p-2 text-surface-100 focus:ring-2 focus:ring-accent-500'
const panelClass = 'min-w-0 rounded-xl border border-surface-800 bg-surface-900 p-4 space-y-4'

export default function PricingCalculator() {
  const { t, i18n } = useTranslation('pricing')
  const money = (value: string, code: string) => formatPricingMoney(value, code, i18n.language)
  const { currencyCode, loading: currencyLoading } = useCurrency()
  const [currencyOverride, setCurrencyOverride] = useState<string | null>(null)
  const currency = currencyOverride ?? currencyCode
  const [draft, setDraft] = useState<Draft>(initial)
  const [full, setFull] = useState(false)
  const [production, setProduction] = useState<'proportional' | 'batches'>('proportional')
  const [machineMode, setMachineMode] = useState<'direct' | 'derived'>('direct')
  const [includesEnergy, setIncludesEnergy] = useState(false)
  const [includesMaintenance, setIncludesMaintenance] = useState(false)
  const [anchor, setAnchor] = useState<'price' | 'gain' | 'margin'>('margin')
  const [extras, setExtras] = useState<PricingExtra[]>([])
  const [fees, setFees] = useState<PricingFee[]>([])
  const [result, setResult] = useState<PricingResult | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [provenance, setProvenance] = useState({ printer: '', material: '', gcode: '' })
  const [catalogConfirmed, setCatalogConfirmed] = useState(false)
  const [legacyPrinter, setLegacyPrinter] = useState(false)
  const [printerCurrency, setPrinterCurrency] = useState('')
  const [gcodeConfirmed, setGcodeConfirmed] = useState(false)
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const generation = useRef(0)
  const controller = useRef<AbortController | null>(null)
  const printers = usePrinters()
  const materials = useMaterials()
  const gcodes = useQuery({ queryKey: ['pricing-gcodes', search, page], queryFn: () => gcodeLibraryApi.list({ q: search, page, page_size: 25 }) })
  function invalidate() { generation.current++; controller.current?.abort(); setResult(null); setBusy(false); setError(''); setDirty(true) }
  function edit(key: keyof Draft, value: string) { invalidate(); setDraft(prev => ({ ...prev, [key]: value })) }
  useEffect(() => () => { generation.current++; controller.current?.abort() }, [])
  useEffect(() => {
    if (!dirty) return
    const unload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    const navigate = (event: MouseEvent) => {
      const link = (event.target as Element).closest?.('a[href]')
      if (link && !link.getAttribute('href')?.startsWith('#') && !window.confirm(t('Leave this unsaved calculation?'))) { event.preventDefault(); event.stopPropagation() }
    }
    window.addEventListener('beforeunload', unload)
    document.addEventListener('click', navigate, true)
    return () => { window.removeEventListener('beforeunload', unload); document.removeEventListener('click', navigate, true) }
  }, [dirty, t])
  // Currency setting changes also invalidate an in-flight result; no settings writes.
  useEffect(() => {
    generation.current++; controller.current?.abort()
    setResult(null); setBusy(false); setCatalogConfirmed(false)
  }, [currency])
  function field(key: keyof Draft, label: string, help?: string) {
    return <label className="block min-w-0 text-sm text-surface-300">{t(label)}<input className={`${inputClass} mt-1`} inputMode={key === 'quantity' || key === 'units' ? 'numeric' : 'decimal'} value={draft[key]} onChange={e => edit(key, e.target.value)} />{help && <span className="block text-xs text-surface-400 mt-1">{t(help)}</span>}</label>
  }
  function select(label: string, value: string, change: (value: string) => void, options: [string, string][]) {
    return <label className="block text-sm text-surface-300">{t(label)}<select className={`${inputClass} mt-1`} value={value} onChange={e => { invalidate(); change(e.target.value) }}>{options.map(([key, text]) => <option value={key} key={key}>{t(text)}</option>)}</select></label>
  }
  async function calculate() {
    invalidate()
    const version = generation.current
    try {
      if (!pricingCurrencyMatches(printerCurrency, currency)) throw new Error(t('Printer currency differs. Choose its currency before loading; no conversion is performed.'))
      if ((provenance.material || legacyPrinter) && !catalogConfirmed) throw new Error(t('Confirm catalog amounts are in the selected currency. No conversion is performed.'))
      if (provenance.gcode && !gcodeConfirmed) throw new Error(t('Confirm what the G-code quantities represent before calculating.'))
      const d = (key: keyof Draft, required = false) => normalizePricingDecimal(draft[key], required)
      const direct = machineMode === 'direct'
      const request: PricingRequest = {
        currency, quantity: pricingCount(draft.quantity),
        production: { mode: production, ...(production === 'batches' ? { units_per_batch: pricingCount(draft.units) } : {}), grams: d('grams'), hours: d('hours') },
        material_cost_per_kg: d('material'),
        machine: { mode: machineMode, ...(direct ? { hourly_rate: d('rate'), includes_energy: includesEnergy, includes_maintenance: includesMaintenance } : { acquisition: d('purchase', true), residual: d('residual', true), lifetime_hours: d('lifetime', true) }), ...(!direct || !includesMaintenance ? { maintenance_hourly: d('maintenance') } : {}) },
        ...(!direct || !includesEnergy ? { energy: { power_watts: d('watts'), tariff_per_kwh: d('tariff') } } : {}),
        labor: { minutes: d('minutes'), hourly_rate: d('labor') }, reserve: d('reserve'), shipping: { cost: d('shippingCost'), charged: d('shippingCharged') },
        extras: extras.map(extra => ({ ...extra, amount: normalizePricingDecimal(extra.amount, true) })),
        fees: fees.map(fee => ({ ...fee, percent: normalizePricingDecimal(fee.percent, true), fixed: normalizePricingDecimal(fee.fixed, true) })),
        target: { anchor, value: normalizePricingDecimal(draft[anchor], true, anchor !== 'price') }, discount_percent: d('discount'), min_charge: d('minimum'),
      }
      controller.current = new AbortController()
      setBusy(true)
      const response = await pricingApi.calculate(request, controller.current.signal)
      if (generation.current === version) {
        setDraft(d => syncPricingTargets(d, anchor, response.base))
        setResult(response); setBusy(false)
      }
    } catch (failure) {
      if (generation.current !== version || (failure instanceof Error && failure.name === 'AbortError')) return
      setError(failure instanceof Error ? failure.message : t('Calculation failed.'))
      setBusy(false)
    }
  }
  function scenario(value: PricingScenario, label: string) {
    return <section className={panelClass}><h3 className="font-semibold">{t(label)}</h3><dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">{([
      ['Goods total', value.price], ['Goods per unit', value.price_per_unit], ['Revenue including shipping', value.revenue_total], ['Included costs and fees', value.cost_total], ['Cost per unit', value.cost_per_unit], ['Fees', value.fees], ['Estimated gain', value.gain], ['Gain per unit', value.gain_per_unit],
    ] as [string, string][]).map(([name, amount]) => <div key={name}><dt className="text-surface-400">{t(name)}</dt><dd className="break-words tabular-nums">{money(amount, result?.currency ?? currency)}</dd></div>)}<div><dt className="text-surface-400">{t('Margin on goods')}</dt><dd>{value.margin_percent === null ? t('Undefined at zero goods price') : `${value.margin_percent}%`}</dd></div></dl></section>
  }
  return <div className="max-w-6xl mx-auto space-y-5 min-w-0 pb-24 text-surface-100">
    <header className="flex flex-wrap justify-between gap-3"><div><h1 className="text-2xl font-semibold">{t('Pricing calculator')}</h1><p className="text-sm text-surface-400">{t('Estimate only. Unentered costs are not profit.')}</p></div><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={full} onChange={e => setFull(e.target.checked)} />{t('Full mode')}</label></header>
    <details className={panelClass}><summary className="cursor-pointer">{t('How this estimate works')}</summary><p className="text-sm text-surface-300">{t('Quick and full modes use the same engine and retain all entered costs. Price and gain targets are order goods totals. Margin is gain divided by goods revenue. Shipping charged is separate. Empty costs produce warnings; enter zero only when intentionally free. Drafts stay in memory only.')}</p></details>
    <form onSubmit={e => { e.preventDefault(); void calculate() }} className="space-y-5">
      <section className={panelClass}><h2 className="font-semibold">{t('Sources and currency')}</h2><div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <label className="text-sm">{t('Currency')}<select aria-label={t('Currency')} className={`${inputClass} mt-1`} value={currency} onChange={e => { invalidate(); setCurrencyOverride(e.target.value); setCatalogConfirmed(false) }}>{PRICING_CURRENCIES.map(code => <option key={code}>{code}</option>)}</select></label>
        <label className="text-sm">{t('Load printer rates')}<select className={`${inputClass} mt-1`} value="" onChange={e => {
          const printer = printers.data?.find(p => p.id === e.target.value); if (!printer) return
          invalidate()
          const config = printer.pricing_config
          if (config && config.currency !== currency) { setError(t('Printer currency differs. Choose its currency before loading; no conversion is performed.')); return }
          setCatalogConfirmed(false); setLegacyPrinter(!config); setPrinterCurrency(config?.currency ?? ''); setProvenance(p => ({ ...p, printer: printer.name }))
          if (config) {
            setMachineMode(config.rate_mode === 'derived' ? 'derived' : 'direct'); setIncludesEnergy(config.includes_energy); setIncludesMaintenance(config.includes_maintenance)
            setDraft(d => ({ ...d, rate: config.direct_rate_per_hour, purchase: config.purchase_price, residual: config.residual_value, lifetime: config.productive_lifetime_hours, maintenance: config.maintenance_per_hour, watts: config.average_power_watts, tariff: config.energy_tariff }))
          } else { setMachineMode('direct'); setIncludesEnergy(false); setIncludesMaintenance(false); setDraft(d => ({ ...d, ...legacyPrinterDraft(printer.cost_per_hour_cents) })) }
        }}><option value="">{t('Choose a printer')}</option>{printers.data?.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
        <label className="text-sm">{t('Load material rate')}<select className={`${inputClass} mt-1`} value="" onChange={e => { const material = materials.data?.find(m => m.id === e.target.value); if (!material) return; invalidate(); setCatalogConfirmed(false); setProvenance(p => ({ ...p, material: material.name })); setDraft(d => ({ ...d, material: String(material.cost_per_kg) })) }}><option value="">{t('Choose a printing material')}</option>{materials.data?.filter(m => m.type !== 'supply').map(m => <option key={m.id} value={m.id}>{m.name}</option>)}</select></label>
      </div>
      {(printers.isError || materials.isError || gcodes.isError) && <p role="alert" className="text-amber-400 text-sm">{t('Some catalog sources could not load. Manual entry is available.')}</p>}
      {(provenance.printer || provenance.material) && <div className="text-sm space-y-2"><p className="break-words">{t('Loaded rates from {{printer}} / {{material}}; fields remain editable.', { printer: provenance.printer || '—', material: provenance.material || '—' })}</p>{(legacyPrinter || provenance.material) && <label className="flex gap-2"><input type="checkbox" checked={catalogConfirmed} onChange={e => { invalidate(); setCatalogConfirmed(e.target.checked) }} />{t('I confirm loaded amounts use {{currency}}. Legacy currency is unknown; no conversion.', { currency })}</label>}</div>}
      <details><summary className="cursor-pointer text-sm">{t('Load G-code estimates')}</summary><div className="mt-3 space-y-3"><label className="block text-sm">{t('Search G-code')}<input className={inputClass} value={search} onChange={e => { setSearch(e.target.value); setPage(1) }} /></label><select aria-label={t('Choose G-code')} className={inputClass} value="" onChange={e => {
        const file = gcodes.data?.items.find(f => f.id === e.target.value); if (!file) return
        invalidate(); setGcodeConfirmed(false); setProvenance(p => ({ ...p, gcode: file.display_name || file.file_name })); setProduction('batches')
        setDraft(d => ({ ...d, units: '', grams: file.filament_grams == null ? '' : String(file.filament_grams), hours: file.estimated_seconds == null ? '' : catalogSecondsToHours(file.estimated_seconds) }))
      }}><option value="">{t('Choose G-code')}</option>{gcodes.data?.items.map(f => <option key={f.id} value={f.id}>{f.display_name || f.file_name}</option>)}</select><div className="flex gap-3 items-center"><button type="button" className="btn btn-secondary" disabled={page === 1} onClick={() => setPage(p => p - 1)}>{t('Previous page')}</button><span>{page}</span><button type="button" className="btn btn-secondary" disabled={!gcodes.data || page * 25 >= gcodes.data.total} onClick={() => setPage(p => p + 1)}>{t('Next page')}</button></div></div></details>
      {provenance.gcode && <div className="space-y-2 text-sm"><p className="break-words">{t('G-code source: {{name}}. Copy count is unknown.', { name: provenance.gcode })}</p><label className="flex gap-2"><input type="checkbox" checked={gcodeConfirmed} onChange={e => { invalidate(); setGcodeConfirmed(e.target.checked) }} />{t('I verified the mass, time and units per plate, or changed them to per-unit values.')}</label></div>}
      </section>
      <div className="grid lg:grid-cols-2 gap-5"><section className={panelClass}><h2 className="font-semibold">{t('Production')}</h2><div className="grid sm:grid-cols-2 gap-4">{field('quantity', 'Requested units')}{select('Production model', production, value => setProduction(value as typeof production), [['proportional', 'Proportional per unit'], ['batches', 'Full plate batches']])}{production === 'batches' && field('units', 'Units per full plate')}{field('grams', production === 'batches' ? 'Grams per full plate' : 'Grams per unit')}{field('hours', production === 'batches' ? 'Hours per full plate' : 'Hours per unit')}{field('material', 'Material cost per kg')}</div>{production === 'batches' && fullBatchRemainder(draft.quantity, draft.units) && <p className="text-sm text-amber-400">{t('Remainder charged as a full plate, including its full mass and time.')}</p>}</section>
      <section className={panelClass}><h2 className="font-semibold">{t('Machine')}</h2>{select('Machine rate model', machineMode, v => setMachineMode(v as typeof machineMode), [['direct', 'Direct hourly rate'], ['derived', 'Derived depreciation']])}<div className="grid sm:grid-cols-2 gap-4">{machineMode === 'direct' ? field('rate', 'Machine rate per hour') : <>{field('purchase', 'Purchase price')}{field('residual', 'Residual value')}{field('lifetime', 'Productive lifetime hours')}</>}</div>{machineMode === 'direct' && <div className="space-y-2 text-sm"><label className="flex gap-2"><input type="checkbox" checked={includesEnergy} onChange={e => { invalidate(); setIncludesEnergy(e.target.checked) }} />{t('Hourly rate already includes energy')}</label><label className="flex gap-2"><input type="checkbox" checked={includesMaintenance} onChange={e => { invalidate(); setIncludesMaintenance(e.target.checked) }} />{t('Hourly rate already includes maintenance')}</label></div>}</section></div>
      <section className={panelClass}><h2 className="font-semibold">{t('Additional costs')}</h2>{!full && <details className="text-sm text-surface-400"><summary className="cursor-pointer">{t('Hidden costs still apply')}</summary><p>{t('Full mode reveals energy, labor, extras, fees and shipping. Entered values still apply when hidden.')}</p></details>}<div hidden={!full} className="space-y-4"><div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">{(machineMode !== 'direct' || !includesMaintenance) && field('maintenance', 'Maintenance per hour')}{(machineMode !== 'direct' || !includesEnergy) && <>{field('watts', 'Average power watts')}{field('tariff', 'Energy tariff per kWh')}</>}{field('minutes', 'Labor minutes per requested unit')}{field('labor', 'Labor rate per hour')}{field('reserve', 'Fixed order reserve')}{field('shippingCost', 'Shipping cost')}{field('shippingCharged', 'Shipping charged')}</div>
      <details><summary className="cursor-pointer">{t('Scoped extra costs')}</summary><div className="space-y-3 mt-3">{extras.map((extra, index) => <fieldset key={index} className="grid sm:grid-cols-2 gap-3 border border-surface-700 rounded-lg p-3"><legend>{t('Extra cost')} {index + 1}</legend><label className="text-sm">{t('Cost label')}<input maxLength={120} className={inputClass} value={extra.label} onChange={e => { invalidate(); setExtras(all => all.map((v, i) => i === index ? { ...v, label: e.target.value } : v)) }} /></label><label className="text-sm">{t('Amount')}<input className={inputClass} inputMode="decimal" value={extra.amount} onChange={e => { invalidate(); setExtras(all => all.map((v, i) => i === index ? { ...v, amount: e.target.value } : v)) }} /></label>{select('Cost occurrence', extra.scope, scope => setExtras(all => all.map((v, i) => i === index ? { ...v, scope: scope as PricingExtra['scope'] } : v)), [['unit', 'Per requested unit'], ['batch', 'Per full plate'], ['project', 'Once per project'], ['order', 'Once per order']])}<button type="button" className="btn btn-secondary" onClick={() => { invalidate(); setExtras(all => all.filter((_, i) => i !== index)) }}>{t('Remove extra')}</button></fieldset>)}<button type="button" className="btn btn-secondary" disabled={extras.length >= 50} onClick={() => { invalidate(); setExtras(all => [...all, { label: '', amount: '', scope: 'order' }]) }}>{t('Add extra cost')}</button></div></details>
      <details><summary className="cursor-pointer">{t('Sales fees')}</summary><p className="text-xs text-surface-400 mt-2">{t('Percentage applies to goods or goods plus shipping charged. Occurrence multiplies the fixed amount only.')}</p><div className="space-y-3 mt-3">{fees.map((fee, index) => <fieldset key={index} className="grid sm:grid-cols-2 gap-3 border border-surface-700 rounded-lg p-3"><legend>{t('Fee')} {index + 1}</legend>{(['percent', 'fixed'] as const).map(key => <label key={key} className="text-sm">{t(key === 'percent' ? 'Fee percent' : 'Fixed fee')}<input className={inputClass} inputMode="decimal" value={fee[key]} onChange={e => { invalidate(); setFees(all => all.map((v, i) => i === index ? { ...v, [key]: e.target.value } : v)) }} /></label>)}{select('Fee base', fee.base, base => setFees(all => all.map((v, i) => i === index ? { ...v, base: base as PricingFee['base'] } : v)), [['goods', 'Goods only'], ['total', 'Goods plus shipping charged']])}{select('Fixed fee occurrence', fee.scope, scope => setFees(all => all.map((v, i) => i === index ? { ...v, scope: scope as PricingFee['scope'] } : v)), [['unit', 'Per requested unit'], ['order', 'Once per order']])}<button type="button" className="btn btn-secondary" onClick={() => { invalidate(); setFees(all => all.filter((_, i) => i !== index)) }}>{t('Remove fee')}</button></fieldset>)}<button type="button" className="btn btn-secondary" disabled={fees.length >= 50} onClick={() => { invalidate(); setFees(all => [...all, { percent: '0', fixed: '0', scope: 'order', base: 'goods' }]) }}>{t('Add fee')}</button></div></details></div></section>
      <section className={panelClass}><h2 className="font-semibold">{t('Selling target')}</h2><details className="text-sm text-surface-400"><summary className="cursor-pointer">{t('Target help')}</summary><p>{t('The last edited target is the anchor. Calculate to update linked values. Price and gain are goods totals for this order, not per-unit values.')}</p><p>{t('Margin uses goods revenue as denominator and includes shipping gain. It is not accounting net profit.')}</p></details><div className="grid sm:grid-cols-3 gap-4">{(['price', 'gain', 'margin'] as const).map(key => <label key={key} className="text-sm">{t(key === 'price' ? 'Goods price target' : key === 'gain' ? 'Gain target' : 'Margin target percent')}{anchor === key && <span className="ml-2 text-accent-400">{t('Active anchor')}</span>}<input className={`${inputClass} mt-1`} inputMode="decimal" value={result && anchor !== key ? (key === 'price' ? result.base.price : key === 'gain' ? result.base.gain : result.base.margin_percent ?? '') : draft[key]} onChange={e => { edit(key, e.target.value); setAnchor(key) }} /></label>)}</div>{field('minimum', 'Minimum service charge for goods') }<details><summary className="cursor-pointer text-sm">{t('Conditional discount, for example Pix')}</summary><div className="mt-3">{field('discount', 'Conditional discount percent')}<p className="mt-2 text-xs text-surface-400">{t('Same costs and fees, lower goods price. This is a scenario, not a guaranteed payment-method saving.')}</p></div></details></section>
      {error && <p role="alert" className="text-red-400 break-words">{t(error)}</p>}<button type="submit" disabled={busy || currencyLoading} className="btn btn-primary">{busy ? t('Calculating…') : t('Calculate estimate')}</button>
    </form>
    {!result && <p role="status" className="text-sm text-surface-400">{t('No current result. Calculate after editing to refresh the estimate.')}</p>}
    {result && <section aria-live="polite" className="space-y-4"><h2 className="text-xl font-semibold">{t('Estimate results')}</h2><details className="text-sm text-surface-400"><summary className="cursor-pointer">{t('Estimate limitations')}</summary><p>{t('Not complete profit: only entered costs are included. Rounded rows may not sum exactly.')}</p></details>{result.warnings.some(w => w === 'negative_gain' || w === 'discounted_loss') && <p role="alert" className="text-red-400 text-sm">{t('This estimate includes a loss scenario.')}</p>}<div className="grid lg:grid-cols-2 gap-4">{scenario(result.base, 'Base scenario')}{result.discounted && scenario(result.discounted, 'Conditional discount scenario')}</div><p className="text-sm">{t('Break-even goods total: {{total}} {{currency}}; per unit: {{unit}} {{currency}}.', { total: result.minimum_sale_price, unit: result.minimum_sale_price_per_unit, currency: result.currency })}</p><details className={panelClass}><summary className="cursor-pointer">{t('Cost breakdown and assumptions')}</summary><p className="text-sm">{t('Production totals: {{grams}} g / {{hours}} h / {{plates}} full plates.', { grams: result.total_grams, hours: result.total_hours, plates: result.effective_plates })}</p><dl className="grid sm:grid-cols-2 gap-2 text-sm">{Object.entries(result.breakdown).map(([key, value]) => <div key={key}><dt>{t(`Cost: ${key}`)}</dt><dd>{money(value, result.currency)}</dd></div>)}</dl></details>{result.warnings.length > 0 && <details className="rounded-xl border border-amber-700 p-4 text-sm text-amber-300"><summary className="cursor-pointer">{t('Missing costs and estimate warnings')} ({result.warnings.length})</summary><ul className="list-disc pl-5 space-y-1 mt-3">{result.warnings.map(code => <li key={code}>{t(warningMessages[code] ?? 'Review this estimate assumption.')}</li>)}</ul></details>}</section>}
  </div>
}
