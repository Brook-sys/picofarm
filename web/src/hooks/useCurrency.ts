import { useContext } from 'react'
import { CurrencyContext, type CurrencyContextValue } from '../contexts/currency-context'

export function useCurrency(): CurrencyContextValue {
  const context = useContext(CurrencyContext)
  if (!context) throw new Error('useCurrency must be used inside CurrencyProvider')
  return context
}
