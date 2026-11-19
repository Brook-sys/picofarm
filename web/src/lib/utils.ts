import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
import i18n from '../i18n'

// Merge Tailwind classes with clsx.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Format bytes to human readable string.
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export function formatGrams(grams: number | null | undefined): string {
  if (grams == null) return '0g'
  if (grams < 1000) return `${grams.toFixed(2)}g`
  return `${(grams / 1000).toFixed(2)}kg`
}

// Format duration in seconds to human readable string. Unit abbreviations are
// translated so a mixed-language duration cannot be read as the wrong unit.
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds)) return '—'
  if (seconds < 60) return i18n.t('{{count}}s', { count: seconds, ns: 'common' })
  if (seconds < 3600) {
    return i18n.t('{{minutes}}m {{seconds}}s', { minutes: Math.floor(seconds / 60), seconds: seconds % 60, ns: 'common' })
  }
  return i18n.t('{{hours}}h {{minutes}}m', { hours: Math.floor(seconds / 3600), minutes: Math.floor((seconds % 3600) / 60), ns: 'common' })
}

// Format date to relative time.
export function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString)
  if (!Number.isFinite(date.getTime())) return '—'
  const locale = i18n.resolvedLanguage === 'pt-BR' ? 'pt-BR' : 'en-US'
  const seconds = (date.getTime() - Date.now()) / 1000
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  if (Math.abs(seconds) < 60) return formatter.format(0, 'second')
  if (Math.abs(seconds) < 3600) return formatter.format(Math.trunc(seconds / 60), 'minute')
  if (Math.abs(seconds) < 86400) return formatter.format(Math.trunc(seconds / 3600), 'hour')
  if (Math.abs(seconds) < 604800) return formatter.format(Math.trunc(seconds / 86400), 'day')
  return date.toLocaleDateString(locale)
}

// English catalog keys for the stable printer-state wire enums. Translate at
// render time in the page's namespace; never change the underlying status.
export const PRINTER_STATUS_LABELS: Record<string, string> = {
  idle: 'Idle',
  printing: 'Printing',
  paused: 'Paused',
  error: 'Error',
  offline: 'Offline',
}

export const PRINT_JOB_STATUS_LABELS: Record<string, string> = {
  queued: 'Queued',
  assigned: 'Assigned',
  uploaded: 'Uploaded',
  sending: 'Sending',
  printing: 'Printing',
  paused: 'Paused',
  completed: 'Completed',
  failed: 'Failed',
  cancelled: 'Cancelled',
}

// Get status color class.
export function getStatusColor(status: string): string {
  switch (status) {
    case 'idle':
    case 'draft':
    case 'new':
      return 'text-surface-400'
    case 'active':
    case 'printing':
    case 'in_use':
      return 'text-blue-400'
    case 'completed':
    case 'complete':
      return 'text-emerald-400'
    case 'paused':
    case 'low':
      return 'text-amber-400'
    case 'error':
    case 'failed':
    case 'cancelled':
      return 'text-red-400'
    case 'offline':
    case 'archived':
    case 'empty':
      return 'text-surface-500'
    default:
      return 'text-surface-400'
  }
}

// Get status badge classes.
export function getStatusBadge(status: string): string {
  switch (status) {
    case 'idle':
    case 'draft':
    case 'new':
      return 'bg-surface-700/50 text-surface-300'
    case 'active':
    case 'printing':
    case 'in_use':
    case 'sending':
      return 'bg-blue-500/20 text-blue-400'
    case 'completed':
    case 'complete':
      return 'bg-emerald-500/20 text-emerald-400'
    case 'paused':
    case 'low':
    case 'queued':
      return 'bg-amber-500/20 text-amber-400'
    case 'error':
    case 'failed':
    case 'cancelled':
      return 'bg-red-500/20 text-red-400'
    case 'offline':
    case 'archived':
    case 'empty':
      return 'bg-surface-800 text-surface-500'
    default:
      return 'bg-surface-700/50 text-surface-300'
  }
}

