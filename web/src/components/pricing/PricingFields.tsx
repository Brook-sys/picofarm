import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react'
import { CheckCircle2, Info, TriangleAlert } from 'lucide-react'

export const pricingInputClass = 'input min-h-11 w-full min-w-0 bg-surface-950 text-base sm:text-sm'

interface FieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'id' | 'className'> {
  id: string
  label: ReactNode
  help?: string
  error?: string
  suffix?: string
  invalid?: boolean
}

export function PricingField({ id, label, help, error, suffix, invalid = false, ...inputProps }: FieldProps) {
  const helpId = `${id}-help`
  const errorId = `${id}-error`
  const suffixId = `${id}-unit`
  const describedBy = [inputProps['aria-describedby'], help ? helpId : '', error ? errorId : '', suffix ? suffixId : '']
    .filter(Boolean)
    .join(' ') || undefined
  return (
    <div className="min-w-0">
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-surface-200">
        {label}
      </label>
      <div className="relative min-w-0">
        <input
          {...inputProps}
          id={id}
          aria-invalid={invalid || !!error || undefined}
          aria-describedby={describedBy}
          className={`${pricingInputClass} ${suffix ? 'pr-14' : ''} ${invalid || error ? 'border-red-500' : ''}`}
        />
        {suffix && (
          <span id={suffixId} className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-surface-400">
            {suffix}
          </span>
        )}
      </div>
      {help && <p id={helpId} className="mt-1.5 text-xs leading-5 text-surface-400">{help}</p>}
      {error && <p id={errorId} role="alert" className="mt-1.5 text-xs leading-5 text-red-300">{error}</p>}
    </div>
  )
}

interface SelectOption {
  value: string
  label: string
}

interface SelectFieldProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id' | 'className' | 'children'> {
  id: string
  label: ReactNode
  options: SelectOption[]
  help?: string
  invalid?: boolean
}

export function PricingSelect({ id, label, options, help, invalid = false, ...selectProps }: SelectFieldProps) {
  const helpId = `${id}-help`
  const describedBy = [selectProps['aria-describedby'], help ? helpId : ''].filter(Boolean).join(' ') || undefined
  return (
    <div className="min-w-0">
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-surface-200">
        {label}
      </label>
      <select
        {...selectProps}
        id={id}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        className={`${pricingInputClass} ${invalid ? 'border-red-500' : ''}`}
      >
        {options.map(option => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      {help && <p id={helpId} className="mt-1.5 text-xs leading-5 text-surface-400">{help}</p>}
    </div>
  )
}

interface SegmentOption<T extends string> {
  value: T
  label: string
  icon?: ReactNode
}

interface SegmentedControlProps<T extends string> {
  label: string
  value: T
  options: SegmentOption<T>[]
  onChange: (value: T) => void
  disabled?: boolean
  compact?: boolean
}

export function PricingSegments<T extends string>({
  label,
  value,
  options,
  onChange,
  disabled = false,
  compact = false,
}: SegmentedControlProps<T>) {
  return (
    <div
      role="group"
      aria-label={label}
      className="grid gap-1 rounded-xl border border-surface-700 bg-surface-950 p-1"
      style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}
    >
      {options.map(option => {
        const active = option.value === value
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={active}
            disabled={disabled}
            onClick={() => onChange(option.value)}
            className={`btn min-h-11 min-w-0 justify-center gap-1.5 whitespace-normal border px-3 py-2 text-center leading-4 ${compact ? 'text-xs' : ''} ${
              active
                ? 'border-accent-500 bg-accent-500/15 text-accent-300 shadow-sm'
                : 'border-transparent bg-transparent text-surface-400 hover:bg-surface-800 hover:text-surface-100'
            }`}
          >
            {option.icon}
            <span className="min-w-0 break-words">{option.label}</span>
          </button>
        )
      })}
    </div>
  )
}

interface CalculatorSectionProps {
  id?: string
  step?: string
  title: string
  icon?: ReactNode
  action?: ReactNode
  children: ReactNode
  className?: string
}

export function CalculatorSection({ id, step, title, icon, action, children, className = '' }: CalculatorSectionProps) {
  return (
    <section id={id} className={`min-w-0 rounded-2xl border border-surface-800 bg-surface-900 p-4 shadow-sm sm:p-5 ${className}`}>
      <div className="mb-5 flex min-w-0 items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          {step && (
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent-500/15 text-sm font-semibold text-accent-300">
              {step}
            </span>
          )}
          {!step && icon && <span className="shrink-0 text-accent-400">{icon}</span>}
          <h2 className="min-w-0 text-base font-semibold text-surface-100">{title}</h2>
        </div>
        {action}
      </div>
      {children}
    </section>
  )
}

interface NoticeProps {
  tone?: 'amber' | 'red' | 'green' | 'neutral'
  role?: 'alert' | 'status'
  children: ReactNode
  className?: string
}

export function PricingNotice({ tone = 'neutral', role, children, className = '' }: NoticeProps) {
  const styles = {
    amber: 'border-amber-600/50 bg-amber-500/10 text-amber-200',
    red: 'border-red-600/50 bg-red-500/10 text-red-200',
    green: 'border-emerald-600/50 bg-emerald-500/10 text-emerald-200',
    neutral: 'border-surface-700 bg-surface-950/70 text-surface-300',
  }
  const Icon = tone === 'green' ? CheckCircle2 : tone === 'neutral' ? Info : TriangleAlert
  return (
    <div role={role} className={`flex items-start gap-2.5 rounded-xl border p-3 text-sm leading-5 ${styles[tone]} ${className}`}>
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  )
}

export function PricingChip({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'accent' | 'amber' | 'green' }) {
  const styles = {
    neutral: 'border-surface-700 bg-surface-800/80 text-surface-300',
    accent: 'border-accent-600/40 bg-accent-500/10 text-accent-300',
    amber: 'border-amber-600/40 bg-amber-500/10 text-amber-300',
    green: 'border-emerald-600/40 bg-emerald-500/10 text-emerald-300',
  }
  return (
    <span className={`inline-flex max-w-full items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium ${styles[tone]}`}>
      {children}
    </span>
  )
}
