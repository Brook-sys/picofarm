import { createContext } from 'react'
import type { CurrencyCode } from '../lib/currency'

export interface CurrencyContextValue {
  currencyCode: CurrencyCode
  symbol: string
  formatAmount: (amount: number, overrideCurrency?: string) => string
  formatCents: (cents: number, overrideCurrency?: string) => string
  loading: boolean
}

export const CurrencyContext = createContext<CurrencyContextValue | null>(null)
