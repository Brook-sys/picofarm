import { ArrowDownRight, ArrowUpRight, Info, Layers } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatPricingDecimal, type PricingResult, type PricingScenario } from '../../lib/pricingCalculator'
import { PricingChip, PricingNotice } from './PricingFields'

const pricingWarningMessages: Record<string, string> = {
  proportional_production_estimate: 'Proportional production is an estimate.',
  full_batch_remainder: 'The remainder uses a full plate.',
  missing_grams: 'Mass is missing.',
  missing_hours: 'Production time is missing.',
  missing_material_cost_per_kg: 'Material rate is missing.',
  missing_machine: 'Machine cost is missing.',
  missing_machine_hourly_rate: 'Machine hourly rate is missing.',
  missing_maintenance_hourly: 'Maintenance is missing.',
  missing_energy: 'Energy cost is missing.',
  missing_power_watts: 'Power is missing.',
  missing_tariff_per_kwh: 'Energy tariff is missing.',
  missing_labor: 'Labor cost is missing.',
  missing_labor_minutes: 'Labor time is missing.',
  missing_labor_hourly_rate: 'Labor rate is missing.',
  missing_reserve: 'Reserve is missing.',
  missing_shipping: 'Shipping is missing.',
  missing_shipping_cost: 'Shipping cost is missing.',
  missing_shipping_charged: 'Shipping charged is missing.',
  missing_extras: 'Extra costs are missing.',
  missing_fees: 'Sales fees are missing.',
  minimum_charge_applied: 'Minimum service charge was applied.',
  price_below_minimum_charge: 'Price is below the minimum service charge.',
  negative_gain: 'The base scenario has a loss.',
  discounted_loss: 'The discounted scenario has a loss.',
}

type Money = (value: string, currency: string) => string

function MoneyRow({ label, value, currency, money, emphasis = false }: {
  label: string
  value: string
  currency: string
  money: Money
  emphasis?: boolean
}) {
  return (
    <div className="flex min-w-0 items-baseline justify-between gap-3 py-1.5">
      <dt className="shrink-0 text-sm text-surface-400">{label}</dt>
      <dd className={`min-w-0 break-words text-right tabular-nums ${emphasis ? 'font-semibold text-surface-100' : 'text-sm text-surface-200'}`}>
        {money(value, currency)}
      </dd>
    </div>
  )
}

function Scenario({ value, currency, money, title, compact = false, incomplete = false }: {
  value: PricingScenario
  currency: string
  money: Money
  title: string
  compact?: boolean
  incomplete?: boolean
}) {
  const { t, i18n } = useTranslation('pricing')
  const loss = value.gain.startsWith('-')
  const tone = loss || incomplete ? 'amber' : 'green'
  const outcome = loss ? 'Loss scenario' : incomplete ? 'Incomplete estimate' : 'Estimated outcome'
  return (
    <section aria-label={t(title)} className={`rounded-2xl border ${loss ? 'border-red-700/60' : 'border-surface-700'} bg-surface-950/80 p-4`}>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold text-surface-100">{t(title)}</h3>
        <PricingChip tone={tone}>
          {loss ? <ArrowDownRight className="h-3.5 w-3.5" /> : <ArrowUpRight className="h-3.5 w-3.5" />}
          {t(outcome)}
        </PricingChip>
      </div>
      <dl className="min-w-0 border-b border-surface-800 pb-3">
        <dt className="text-xs font-medium uppercase tracking-wide text-surface-400">{t('Goods per unit')}</dt>
        <dd className={`${compact ? 'text-xl' : 'text-3xl'} mt-1 break-words font-bold tracking-tight text-accent-300 tabular-nums`}>
          {money(value.price_per_unit, currency)}
        </dd>
        <MoneyRow label={t('Goods total')} value={value.price} currency={currency} money={money} emphasis />
      </dl>
      <div className="my-2 grid grid-cols-1 min-[380px]:grid-cols-2 gap-2">
        <dl className={`rounded-xl p-3 ${loss ? 'bg-red-500/10' : 'bg-emerald-500/10'}`}>
          <dt className="text-xs text-surface-400">{t('Estimated order gain')}</dt>
          <dd className={`mt-1 break-words font-semibold tabular-nums ${loss ? 'text-red-300' : 'text-emerald-300'}`}>
            {money(value.gain, currency)}
          </dd>
        </dl>
        <dl className="rounded-xl bg-surface-900 p-3">
          <dt className="text-xs text-surface-400">{t('Margin on goods')}</dt>
          <dd className="mt-1 break-words font-semibold text-surface-100 tabular-nums">
            {value.margin_percent === null ? t('Undefined at zero goods price') : `${formatPricingDecimal(value.margin_percent, i18n.language)}%`}
          </dd>
        </dl>
      </div>
      {!compact && (
        <dl className="divide-y divide-surface-800/70">
          <MoneyRow label={t('Gain per unit')} value={value.gain_per_unit} currency={currency} money={money} />
          <MoneyRow label={t('Included costs and fees')} value={value.cost_total} currency={currency} money={money} />
          <MoneyRow label={t('Cost per unit')} value={value.cost_per_unit} currency={currency} money={money} />
          <MoneyRow label={t('Fees')} value={value.fees} currency={currency} money={money} />
          <MoneyRow label={t('Revenue including shipping')} value={value.revenue_total} currency={currency} money={money} />
        </dl>
      )}
    </section>
  )
}

const costGroups: { label: string; keys: (keyof PricingResult['breakdown'])[] }[] = [
  { label: 'Production costs', keys: ['material', 'machine'] },
  { label: 'Operating costs', keys: ['maintenance', 'energy', 'labor'] },
  { label: 'Order costs', keys: ['extras', 'reserve', 'shipping'] },
]

function CostGroups({ result, money }: { result: PricingResult; money: Money }) {
  const { t, i18n } = useTranslation('pricing')
  const plates = Number(result.effective_plates)
  return (
    <div className="mt-3 space-y-4">
      <p className="text-xs leading-5 text-surface-400">
        {t('Production totals: {{grams}} g / {{hours}} h / {{plates}} full plates.', {
          grams: formatPricingDecimal(result.total_grams, i18n.language),
          hours: formatPricingDecimal(result.total_hours, i18n.language),
          plates: formatPricingDecimal(result.effective_plates, i18n.language),
          count: Number.isSafeInteger(plates) ? plates : 0,
        })}
      </p>
      {costGroups.map(group => (
        <section key={group.label}>
          <h4 className="mb-1 text-xs font-semibold uppercase tracking-wide text-surface-400">{t(group.label)}</h4>
          <dl className="divide-y divide-surface-800/70">
            {group.keys.map(key => (
              <MoneyRow key={key} label={t(`Cost: ${key}`)} value={result.breakdown[key]} currency={result.currency} money={money} />
            ))}
          </dl>
        </section>
      ))}
      <p className="text-xs text-surface-500">{t('Selling fees are shown separately above.')}</p>
    </div>
  )
}

function missingCostTarget(warnings: string[]): string {
  if (warnings.some(code => ['missing_grams', 'missing_hours'].includes(code))) return '#production'
  if (warnings.some(code => ['missing_machine', 'missing_machine_hourly_rate', 'missing_material_cost_per_kg'].includes(code))) return '#machine'
  return '#additional-costs'
}

export default function PricingResults({ result, full, money }: { result: PricingResult; full: boolean; money: Money }) {
  const { t } = useTranslation('pricing')
  const hasLoss = result.warnings.some(code => code === 'negative_gain' || code === 'discounted_loss')
  const missing = result.warnings.filter(code => code.startsWith('missing_'))
  const secondaryWarnings = result.warnings.filter(code => !code.startsWith('missing_'))

  return (
    <section aria-label={t('Estimate results')} className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-surface-100">{t('Estimate results')}</h2>
          <p className="mt-0.5 text-xs text-surface-400">{t('For {{count}} units', { count: result.quantity })}</p>
        </div>
        <PricingChip tone={missing.length ? 'amber' : 'accent'}>{t(missing.length ? 'Incomplete estimate' : 'Estimate')}</PricingChip>
      </div>
      <p role="status" className="sr-only">
        {t('Estimate updated: {{price}} per unit for {{count}} units.', { price: money(result.base.price_per_unit, result.currency), count: result.quantity })}
      </p>
      {hasLoss && (
        <PricingNotice tone="red" role="alert">{t('This estimate includes a loss scenario.')}</PricingNotice>
      )}
      {missing.length > 0 && (
        <PricingNotice tone="amber">
          <p className="font-medium">{t('Incomplete estimate: {{count}} cost inputs are missing.', { count: missing.length })}</p>
          <a href={missingCostTarget(missing)} className="mt-1 inline-block underline underline-offset-2" onClick={() => {
            const section = document.querySelector(missingCostTarget(missing))
            section?.querySelectorAll('details').forEach(details => { details.open = true })
            section?.querySelector<HTMLElement>('input, select, button')?.focus()
          }}>{t('Review missing costs')}</a>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-xs">{missing.map(code => <li key={code}>{t(pricingWarningMessages[code] ?? 'Review this estimate assumption.')}</li>)}</ul>
        </PricingNotice>
      )}
      <Scenario value={result.base} currency={result.currency} money={money} title="Base scenario" incomplete={missing.length > 0} />
      {result.discounted && (
        <Scenario value={result.discounted} currency={result.currency} money={money} title="Conditional discount scenario" compact incomplete={missing.length > 0} />
      )}

      <div className="rounded-xl border border-surface-700 bg-surface-900 p-3">
        <div className="flex items-center gap-2 text-sm font-medium text-surface-200">
          <Layers className="h-4 w-4 text-accent-400" />
          {t('Break-even goods price')}
        </div>
        <dl className="mt-2 divide-y divide-surface-800">
          <MoneyRow label={t('Goods total')} value={result.minimum_sale_price} currency={result.currency} money={money} emphasis />
          <MoneyRow label={t('Goods per unit')} value={result.minimum_sale_price_per_unit} currency={result.currency} money={money} />
        </dl>
      </div>

      {full ? (
        <section className="rounded-xl border border-surface-700 bg-surface-900 p-3">
          <h3 className="text-sm font-semibold text-surface-100">{t('Cost breakdown and assumptions')}</h3>
          <CostGroups result={result} money={money} />
        </section>
      ) : (
        <details className="rounded-xl border border-surface-700 bg-surface-900 p-3">
          <summary className="cursor-pointer text-sm font-medium text-surface-200">{t('Cost breakdown and assumptions')}</summary>
          <CostGroups result={result} money={money} />
        </details>
      )}

      {secondaryWarnings.length > 0 && (
        <details className="rounded-xl border border-amber-700/60 bg-amber-500/5 p-3 text-sm text-amber-200">
          <summary className="cursor-pointer font-medium">{t('Estimate assumptions and warnings')} ({secondaryWarnings.length})</summary>
          <ul className="mt-3 list-disc space-y-1.5 pl-5">
            {secondaryWarnings.map(code => <li key={code}>{t(pricingWarningMessages[code] ?? 'Review this estimate assumption.')}</li>)}
          </ul>
        </details>
      )}
      <details className="text-xs text-surface-400">
        <summary className="inline-flex cursor-pointer items-center gap-1.5">
          <Info className="h-3.5 w-3.5" />{t('Estimate limitations')}
        </summary>
        <p className="mt-2 leading-5">{t('Not complete profit: only entered costs are included. Rounded rows may not sum exactly.')}</p>
      </details>
    </section>
  )
}
