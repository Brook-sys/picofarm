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

function resolveDisplayCurrency(value?: string | null): string {
  const normalized = value?.trim().toUpperCase() || DEFAULT_CURRENCY
  if (!/^[A-Z]{3}$/.test(normalized)) return DEFAULT_CURRENCY
  try {
    new Intl.NumberFormat(undefined, { style: 'currency', currency: normalized }).format(0)
    return normalized
  } catch {
    return DEFAULT_CURRENCY
  }
}

export function formatMoney(amount: number, currencyCode: string = DEFAULT_CURRENCY): string {
  const currency = resolveDisplayCurrency(currencyCode)
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number.isFinite(amount) ? amount : 0)
}

export function formatMoneyCents(cents: number, currencyCode: string = DEFAULT_CURRENCY): string {
  return formatMoney(cents / 100, currencyCode)
}

export function currencySymbol(currencyCode: string = DEFAULT_CURRENCY): string {
  const currency = resolveDisplayCurrency(currencyCode)
  const part = new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    currencyDisplay: 'narrowSymbol',
  }).formatToParts(0).find(item => item.type === 'currency')
  return part?.value || currency
}
