import { useMemo, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { settingsApi } from '../api/client'
import {
  currencySymbol,
  formatMoney,
  formatMoneyCents,
  normalizeCurrencyCode,
} from '../lib/currency'
import { CurrencyContext, type CurrencyContextValue } from './currency-context'

export function CurrencyProvider({ children }: { children: ReactNode }) {
  const { data, isLoading } = useQuery({
    queryKey: ['settings', 'currency_code'],
    queryFn: () => settingsApi.get('currency_code'),
    retry: false,
  })
  const currencyCode = normalizeCurrencyCode(data?.value)

  const value = useMemo<CurrencyContextValue>(() => ({
    currencyCode,
    symbol: currencySymbol(currencyCode),
    formatAmount: (amount, overrideCurrency) => formatMoney(amount, overrideCurrency || currencyCode),
    formatCents: (cents, overrideCurrency) => formatMoneyCents(cents, overrideCurrency || currencyCode),
    loading: isLoading,
  }), [currencyCode, isLoading])

  return <CurrencyContext.Provider value={value}>{children}</CurrencyContext.Provider>
}
