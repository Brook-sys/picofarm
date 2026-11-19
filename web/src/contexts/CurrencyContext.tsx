import { useMemo, useState, useEffect, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { settingsApi } from '../api/client'
import {
  currencySymbol,
  formatMoney,
  formatMoneyCents,
  normalizeCurrencyCode,
} from '../lib/currency'
import i18n, { getLocale } from '../i18n'
import { CurrencyContext, type CurrencyContextValue } from './currency-context'

export function CurrencyProvider({ children }: { children: ReactNode }) {
  const { data, isLoading } = useQuery({
    queryKey: ['settings', 'currency_code'],
    queryFn: () => settingsApi.get('currency_code'),
    retry: false,
  })
  const currencyCode = normalizeCurrencyCode(data?.value)

  // Bind the memoized formatters to one display locale. Changing language
  // re-creates them; it never changes the currency code or stored amounts.
  const [locale, setLocale] = useState(getLocale())
  useEffect(() => {
    const changed = () => setLocale(getLocale())
    i18n.on('languageChanged', changed)
    return () => i18n.off('languageChanged', changed)
  }, [])

  const value = useMemo<CurrencyContextValue>(() => ({
    currencyCode,
    symbol: currencySymbol(currencyCode, locale),
    formatAmount: (amount, overrideCurrency) => formatMoney(amount, overrideCurrency || currencyCode, locale),
    formatCents: (cents, overrideCurrency) => formatMoneyCents(cents, overrideCurrency || currencyCode, locale),
    loading: isLoading,
  }), [currencyCode, isLoading, locale])

  return <CurrencyContext.Provider value={value}>{children}</CurrencyContext.Provider>
}
