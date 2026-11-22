// Compute-only pricing contract: monetary values are major-unit decimal strings.
export interface PricingMachine {
  mode: 'direct' | 'derived'
  hourly_rate?: string
  acquisition?: string
  residual?: string
  lifetime_hours?: string
  maintenance_hourly?: string
  includes_energy?: boolean
  includes_maintenance?: boolean
}
export interface PricingExtra { label: string; amount: string; scope: 'unit' | 'batch' | 'project' | 'order' }
export interface PricingFee { percent: string; fixed: string; scope: 'unit' | 'order'; base: 'goods' | 'total' }
export interface PricingRequest {
  currency: string
  quantity: number
  production: { mode: 'proportional' | 'batches'; units_per_batch?: number; grams?: string; hours?: string }
  material_cost_per_kg?: string
  machine?: PricingMachine
  energy?: { power_watts?: string; tariff_per_kwh?: string }
  labor?: { minutes?: string; hourly_rate?: string }
  extras: PricingExtra[]
  min_charge?: string
  reserve?: string
  shipping?: { cost?: string; charged?: string }
  fees: PricingFee[]
  target: { anchor: 'price' | 'gain' | 'margin'; value: string }
  discount_percent?: string
}
export interface PricingScenario {
  price: string; price_per_unit: string; fees: string; gain: string; gain_per_unit: string
  margin_percent: string | null; revenue_total: string; cost_total: string; cost_per_unit: string
}
export interface PricingResult {
  currency: string; quantity: number; effective_plates: string; production_multiplier: string
  total_grams: string; total_hours: string
  breakdown: Record<'material' | 'machine' | 'maintenance' | 'energy' | 'labor' | 'extras' | 'reserve' | 'shipping', string>
  minimum_sale_price: string; minimum_sale_price_per_unit: string
  base: PricingScenario; discounted?: PricingScenario; warnings: string[]; estimate: boolean
}

/** Achieved values may differ after minimum-charge/rounding: never replace intent. */
export function syncPricingTargets<T extends { price: string; gain: string; margin: string }>(draft: T, anchor: PricingRequest['target']['anchor'], scenario: PricingScenario): T {
  return { ...draft,
    price: anchor === 'price' ? draft.price : scenario.price,
    gain: anchor === 'gain' ? draft.gain : scenario.gain,
    margin: anchor === 'margin' ? draft.margin : scenario.margin_percent ?? '',
  }
}
export function pricingCurrencyMatches(sourceCurrency: string, selectedCurrency: string): boolean {
  return !sourceCurrency || sourceCurrency === selectedCurrency
}
/** Loading legacy rates replaces the entire printer source, not only its rate. */
export function legacyPrinterDraft(cents: number) {
  return { rate: catalogCentsToDecimal(cents), purchase: '', residual: '', lifetime: '', maintenance: '', watts: '', tariff: '' }
}

export const PRICING_CURRENCIES = ['BRL', 'USD', 'EUR', 'GBP', 'CAD', 'AUD', 'JPY', 'CHF', 'CNY', 'INR', 'MXN', 'ARS', 'CLP', 'COP', 'NZD', 'SEK', 'NOK', 'DKK', 'PLN', 'CZK'] as const

/** Preserve arbitrary response precision: Intl groups only the BigInt whole part. */
export function formatPricingMoney(value: string, currency: string, locale: string): string {
  if (!/^-?\d+(\.\d+)?$/.test(value)) return `${value} ${currency}`
  const negative = value.startsWith('-')
  const [whole, fraction] = value.replace(/^-/, '').split('.')
  const formatter = new Intl.NumberFormat(locale, { useGrouping: true, maximumFractionDigits: 0 })
  const separator = new Intl.NumberFormat(locale).formatToParts(1.1).find(p => p.type === 'decimal')?.value ?? '.'
  return `${negative ? '−' : ''}${formatter.format(BigInt(whole))}${fraction === undefined ? '' : separator + fraction} ${currency}`
}

/** Normalize only at submission; never rewrite an in-progress decimal input. */
export function normalizePricingDecimal(raw: string, required = false, signed = false): string {
  const value = raw.trim().replace(',', '.')
  if (signed && value.startsWith('-')) return `-${normalizePricingDecimal(value.slice(1), true)}`
  if (!value && !required) return ''
  if (!/^[0-9]{1,10}(\.[0-9]{1,9})?$/.test(value)) throw new Error('Enter a nonnegative decimal with up to 9 decimal places.')
  const [whole, fraction = ''] = value.split('.')
  const integer = BigInt(whole)
  if (integer > 1000000000n || (integer === 1000000000n && /[1-9]/.test(fraction))) throw new Error('The maximum value is 1000000000.')
  return `${integer}${fraction ? `.${fraction}` : ''}`
}
export function pricingCount(raw: string): number {
  if (!/^[0-9]{1,7}$/.test(raw) || BigInt(raw) < 1n || BigInt(raw) > 1000000n) throw new Error('Enter a whole quantity from 1 to 1000000.')
  return Number(raw)
}
/** Exact unit conversion of an existing numeric catalog value, not pricing math. */
export function catalogCentsToDecimal(cents: number): string {
  if (!Number.isSafeInteger(cents) || cents < 0) return ''
  const value = BigInt(cents)
  return `${value / 100n}.${String(value % 100n).padStart(2, '0')}`
}
/** Convert source seconds with integer arithmetic, rounded to 9 decimal hours. */
export function catalogSecondsToHours(seconds: number): string {
  if (!Number.isSafeInteger(seconds) || seconds < 0) return ''
  const scaled = (BigInt(seconds) * 1000000000n + 1800n) / 3600n
  return `${scaled / 1000000000n}.${String(scaled % 1000000000n).padStart(9, '0')}`
}
export function fullBatchRemainder(quantity: string, units: string): boolean {
  try { return pricingCount(quantity) % pricingCount(units) !== 0 } catch { return false }
}
