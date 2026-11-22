import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { PricingExtra, PricingFee } from '../../lib/pricingCalculator'
import { PricingField, PricingSelect } from './PricingFields'

export interface PricingExtraRow extends PricingExtra { id: string }
export interface PricingFeeRow extends PricingFee { id: string }

interface AdvancedCostsProps {
  extras: PricingExtraRow[]
  fees: PricingFeeRow[]
  currency: string
  errors?: Record<string, string>
  invalid?: boolean
  onExtraChange: (index: number, patch: Partial<PricingExtra>) => void
  onExtraAdd: () => void
  onExtraRemove: (index: number) => void
  onFeeChange: (index: number, patch: Partial<PricingFee>) => void
  onFeeAdd: () => void
  onFeeRemove: (index: number) => void
}

export default function PricingAdvancedCosts({
  extras,
  fees,
  currency,
  errors = {},
  invalid = false,
  onExtraChange,
  onExtraAdd,
  onExtraRemove,
  onFeeChange,
  onFeeAdd,
  onFeeRemove,
}: AdvancedCostsProps) {
  const { t } = useTranslation('pricing')
  const occurrenceOptions = [
    { value: 'unit', label: t('Per requested unit') },
    { value: 'batch', label: t('Per full plate') },
    { value: 'project', label: t('Once per project') },
    { value: 'order', label: t('Once per order') },
  ]

  return (
    <div className="space-y-5">
      <section aria-labelledby="extra-costs-heading">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h3 id="extra-costs-heading" className="text-sm font-semibold text-surface-100">{t('Scoped extra costs')}</h3>
          <button id="add-extra-cost" type="button" className="btn btn-secondary min-h-11 gap-2" disabled={extras.length >= 50} onClick={onExtraAdd}>
            <Plus className="h-4 w-4" />{t('Add extra cost')}
          </button>
        </div>
        <div className="space-y-3">
          {extras.map((extra, index) => (
            <fieldset key={extra.id} className="min-w-0 rounded-xl border border-surface-700 bg-surface-950/60 p-3">
              <legend className="sr-only">{t('Extra cost')} {index + 1}</legend>
              <div className="mb-3 flex items-center justify-between gap-3">
                <span className="text-xs font-medium text-surface-400">{t('Extra cost')} {index + 1}</span>
                <button
                  type="button"
                  className="btn btn-ghost min-h-11 gap-2 px-3 text-red-300"
                  aria-label={`${t('Remove extra')} ${index + 1}${extra.label ? ` — ${extra.label}` : ''}`}
                  onClick={() => onExtraRemove(index)}
                >
                  <Trash2 className="h-4 w-4" />{t('Remove extra')}
                </button>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <PricingField
                  id={`extra-${extra.id}-label`}
                  label={t('Cost label')}
                  maxLength={120}
                  value={extra.label}
                  invalid={invalid}
                  onChange={event => onExtraChange(index, { label: event.target.value })}
                />
                <PricingField
                  id={`extra-${extra.id}-amount`}
                  label={t('Amount')}
                  inputMode="decimal"
                  suffix={currency}
                  value={extra.amount}
                  invalid={invalid}
                  error={errors[`extra-${extra.id}-amount`] ? t(errors[`extra-${extra.id}-amount`]) : undefined}
                  onChange={event => onExtraChange(index, { amount: event.target.value })}
                />
                <div className="sm:col-span-2">
                  <PricingSelect
                    id={`extra-${extra.id}-scope`}
                    label={t('Cost occurrence')}
                    value={extra.scope}
                    options={occurrenceOptions}
                    onChange={event => onExtraChange(index, { scope: event.target.value as PricingExtra['scope'] })}
                  />
                </div>
              </div>
            </fieldset>
          ))}
        </div>
      </section>

      <section aria-labelledby="sales-fees-heading">
        <div className="mb-1 flex flex-wrap items-center justify-between gap-2">
          <h3 id="sales-fees-heading" className="text-sm font-semibold text-surface-100">{t('Sales fees')}</h3>
          <button id="add-fee" type="button" className="btn btn-secondary min-h-11 gap-2" disabled={fees.length >= 50} onClick={onFeeAdd}>
            <Plus className="h-4 w-4" />{t('Add fee')}
          </button>
        </div>
        <p className="mb-3 text-xs leading-5 text-surface-400">
          {t('Percentage applies to goods or goods plus shipping charged. Occurrence multiplies the fixed amount only.')}
        </p>
        <div className="space-y-3">
          {fees.map((fee, index) => (
            <fieldset key={fee.id} className="min-w-0 rounded-xl border border-surface-700 bg-surface-950/60 p-3">
              <legend className="sr-only">{t('Fee')} {index + 1}</legend>
              <div className="mb-3 flex items-center justify-between gap-3">
                <span className="text-xs font-medium text-surface-400">{t('Fee')} {index + 1}</span>
                <button
                  type="button"
                  className="btn btn-ghost min-h-11 gap-2 px-3 text-red-300"
                  aria-label={`${t('Remove fee')} ${index + 1}${fee.percent || fee.fixed ? ` — ${fee.percent}% + ${fee.fixed} ${currency}` : ''}`}
                  onClick={() => onFeeRemove(index)}
                >
                  <Trash2 className="h-4 w-4" />{t('Remove fee')}
                </button>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <PricingField
                  id={`fee-${fee.id}-percent`}
                  label={t('Fee percent')}
                  inputMode="decimal"
                  suffix="%"
                  value={fee.percent}
                  invalid={invalid}
                  error={errors[`fee-${fee.id}-percent`] ? t(errors[`fee-${fee.id}-percent`]) : undefined}
                  onChange={event => onFeeChange(index, { percent: event.target.value })}
                />
                <PricingField
                  id={`fee-${fee.id}-fixed`}
                  label={t('Fixed fee')}
                  inputMode="decimal"
                  suffix={currency}
                  value={fee.fixed}
                  invalid={invalid}
                  error={errors[`fee-${fee.id}-fixed`] ? t(errors[`fee-${fee.id}-fixed`]) : undefined}
                  onChange={event => onFeeChange(index, { fixed: event.target.value })}
                />
                <PricingSelect
                  id={`fee-${fee.id}-base`}
                  label={t('Fee base')}
                  value={fee.base}
                  options={[
                    { value: 'goods', label: t('Goods only') },
                    { value: 'total', label: t('Goods plus shipping charged') },
                  ]}
                  onChange={event => onFeeChange(index, { base: event.target.value as PricingFee['base'] })}
                />
                <PricingSelect
                  id={`fee-${fee.id}-scope`}
                  label={t('Fixed fee occurrence')}
                  value={fee.scope}
                  options={[
                    { value: 'unit', label: t('Per requested unit') },
                    { value: 'order', label: t('Once per order') },
                  ]}
                  onChange={event => onFeeChange(index, { scope: event.target.value as PricingFee['scope'] })}
                />
              </div>
            </fieldset>
          ))}
        </div>
      </section>
    </div>
  )
}
