import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useUpdatePrinter } from '../hooks/usePrinters'
import type { Printer, PrinterPricingConfig } from '../types'

const currencies = ['BRL', 'USD', 'EUR', 'GBP', 'CAD', 'AUD', 'JPY', 'CHF', 'NZD', 'MXN', 'ARS', 'CLP', 'COP', 'INR', 'CNY', 'SEK', 'NOK', 'DKK', 'PLN', 'CZK']
const emptyConfig: PrinterPricingConfig = {
  currency: '', rate_mode: 'direct', direct_rate_per_hour: '', purchase_price: '',
  residual_value: '', productive_lifetime_hours: '', maintenance_per_hour: '',
  average_power_watts: '', energy_tariff: '', includes_energy: false, includes_maintenance: false,
}
const decimalFields = [
  ['direct_rate_per_hour', 'Direct hourly rate'], ['purchase_price', 'Pricing purchase value'],
  ['residual_value', 'Residual value'], ['productive_lifetime_hours', 'Productive lifetime (hours)'],
  ['maintenance_per_hour', 'Maintenance per hour'], ['average_power_watts', 'Average power (W)'],
  ['energy_tariff', 'Energy tariff (per kWh)'],
] as const

// Preserve hidden values in the stored profile; the calculator adapter must
// select the active mode and omit separate charges identified as included.
function visiblePricingField(config: PrinterPricingConfig, field: string) {
  if (field === 'direct_rate_per_hour') return config.rate_mode === 'direct'
  if (['purchase_price', 'residual_value', 'productive_lifetime_hours'].includes(field)) return config.rate_mode === 'derived'
  if (field === 'maintenance_per_hour') return !config.includes_maintenance
  return !config.includes_energy
}

function pricingPayload(config: PrinterPricingConfig): PrinterPricingConfig {
  const payload = { ...config }
  for (const [field] of decimalFields) {
    payload[field] = config[field].trim().replace(',', '.')
  }
  return payload
}

export default function PrinterPricingSettings({ printer }: { printer: Printer }) {
  const { t } = useTranslation('fleet')
  const update = useUpdatePrinter()
  const [config, setConfig] = useState<PrinterPricingConfig>(() => ({ ...(printer.pricing_config ?? emptyConfig) }))
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [dirty, setDirty] = useState(false)
  const edit = (next: PrinterPricingConfig) => {
    setConfig(next)
    setSaved(false)
    setError('')
    setDirty(true)
  }
  useEffect(() => {
    if (!dirty) return
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  const save = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError('')
    setSaved(false)
    try {
      await update.mutateAsync({ id: printer.id, data: { pricing_config: pricingPayload(config) } })
      setSaved(true)
      setDirty(false)
    } catch (err) { setError(err instanceof Error ? err.message : t('Failed to save pricing')) }
  }
  return (
    <details className="card p-4">
      <summary className="cursor-pointer font-medium">{t('Pricing (optional)')}</summary>
      <details className="mt-3 text-sm text-surface-400">
        <summary className="cursor-pointer">{t('Pricing currency')} / {t('Hourly rate mode')}</summary>
        <p>{t('Independent calculator profile. Legacy hourly cost and ROI purchase price are not changed. No profile means unknown costs, not free printing.')}</p>
      </details>
      <form onSubmit={save} className="mt-4 space-y-3">
        <fieldset disabled={update.isPending} className="space-y-3">
        <label className="block text-sm">{t('Pricing currency')}
          <select required value={config.currency} onChange={e => edit({ ...config, currency: e.target.value })} className="input mt-1 w-full">
            <option value="">{t('Select currency explicitly')}</option>
            {currencies.map(currency => <option key={currency} value={currency}>{currency}</option>)}
          </select>
        </label>
        <label className="block text-sm">{t('Hourly rate mode')}
          <select value={config.rate_mode} onChange={e => edit({ ...config, rate_mode: e.target.value as PrinterPricingConfig['rate_mode'], includes_energy: false, includes_maintenance: false })} className="input mt-1 w-full">
            <option value="direct">{t('Direct rate')}</option>
            <option value="derived">{t('Derived depreciation')}</option>
          </select>
        </label>
        <div className="grid gap-3 sm:grid-cols-2">
          {decimalFields.filter(([field]) => visiblePricingField(config, field)).map(([field, label]) => (
            <label key={field} className="block text-sm">{t(label)}
              <input className="input mt-1 w-full" type="text" inputMode="decimal" maxLength={19}
                pattern="(0|[1-9][0-9]{0,8})([.,][0-9]{1,9})?"
                required={config.rate_mode === 'direct' ? field === 'direct_rate_per_hour' : ['purchase_price', 'residual_value', 'productive_lifetime_hours'].includes(field)}
                value={config[field]} onChange={e => edit({ ...config, [field]: e.target.value })} />
            </label>
          ))}
        </div>
        <p className="text-xs text-surface-400">0.5 / 0,5</p>
        <fieldset disabled={config.rate_mode === 'derived'} className="space-y-2">
          <legend className="text-sm">{t('Direct rate already includes')}</legend>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={config.includes_energy} onChange={e => edit({ ...config, includes_energy: e.target.checked })} />{t('Energy included in rate')}</label>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={config.includes_maintenance} onChange={e => edit({ ...config, includes_maintenance: e.target.checked })} />{t('Maintenance included in rate')}</label>
        </fieldset>
        {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
        {saved && <p role="status" className="text-sm">{t('Pricing saved')}</p>}
        <div className="flex gap-2">
          <button type="submit" className="btn btn-primary" disabled={update.isPending}>{t('Save pricing')}</button>
          <button type="button" className="btn btn-secondary" disabled={update.isPending || !printer.pricing_config} onClick={async () => {
            setError(''); setSaved(false)
            try {
              await update.mutateAsync({ id: printer.id, data: { pricing_config: null } })
              setConfig({ ...emptyConfig })
              setDirty(false)
            } catch (err) { setError(err instanceof Error ? err.message : t('Failed to save pricing')) }
          }}>{t('Clear pricing')}</button>
        </div>
        </fieldset>
      </form>
    </details>
  )
}
