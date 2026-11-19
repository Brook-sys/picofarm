import { getLocale } from '../i18n'

export const DEFAULT_CURRENCY = 'USD'

export const CURRENCY_OPTIONS = [
  { code: 'USD', label: 'US Dollar ($)' },
  { code: 'BRL', label: 'Brazilian Real (R$)' },
  { code: 'EUR', label: 'Euro (€)' },
  { code: 'GBP', label: 'British Pound (£)' },
  { code: 'CAD', label: 'Canadian Dollar (CA$)' },
  { code: 'AUD', label: 'Australian Dollar (A$)' },
  { code: 'JPY', label: 'Japanese Yen (¥)' },
  { code: 'CNY', label: 'Chinese Yuan (CN¥)' },
  { code: 'INR', label: 'Indian Rupee (₹)' },
  { code: 'MXN', label: 'Mexican Peso (MX$)' },
  { code: 'ARS', label: 'Argentine Peso (ARS)' },
  { code: 'CLP', label: 'Chilean Peso (CLP)' },
  { code: 'COP', label: 'Colombian Peso (COP)' },
] as const

export type CurrencyCode = typeof CURRENCY_OPTIONS[number]['code']

const supportedCurrencies = new Set<string>(CURRENCY_OPTIONS.map(option => option.code))

export function normalizeCurrencyCode(value?: string | null): CurrencyCode {
  const normalized = value?.trim().toUpperCase() || DEFAULT_CURRENCY
  return (supportedCurrencies.has(normalized) ? normalized : DEFAULT_CURRENCY) as CurrencyCode
}

// Locale is an explicit optional parameter so React contexts can bind their
// memoized formatters to one interface language. Stored amounts and currency
// codes are never converted or rewritten by formatting.
function resolveDisplayCurrency(value: string | null | undefined, locale: string): string {
  const normalized = value?.trim().toUpperCase() || DEFAULT_CURRENCY
  if (!/^[A-Z]{3}$/.test(normalized)) return DEFAULT_CURRENCY
  try {
    new Intl.NumberFormat(locale, { style: 'currency', currency: normalized }).format(0)
    return normalized
  } catch {
    return DEFAULT_CURRENCY
  }
}

export function formatMoney(amount: number, currencyCode: string = DEFAULT_CURRENCY, locale: string = getLocale()): string {
  const currency = resolveDisplayCurrency(currencyCode, locale)
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number.isFinite(amount) ? amount : 0)
}

export function formatNumber(amount: number, digits = 2): string {
  return new Intl.NumberFormat(getLocale(), {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(Number.isFinite(amount) ? amount : 0)
}

export function formatDateTime(dateString: string): string {
  const date = new Date(dateString)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleString(getLocale(), { dateStyle: 'short', timeStyle: 'short' })
}


export function formatMoneyCents(cents: number, currencyCode: string = DEFAULT_CURRENCY, locale: string = getLocale()): string {
  return formatMoney(cents / 100, currencyCode, locale)
}

export function currencySymbol(currencyCode: string = DEFAULT_CURRENCY, locale: string = getLocale()): string {
  const currency = resolveDisplayCurrency(currencyCode, locale)
  const part = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    currencyDisplay: 'narrowSymbol',
  }).formatToParts(0).find(item => item.type === 'currency')
  return part?.value || currency
}
