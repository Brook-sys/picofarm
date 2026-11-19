import { useTranslation } from 'react-i18next'
import { useEffect, useMemo, useState } from 'react'
import { archivesApi } from '../api/client'
import type { PrintArchive } from '../types'
import { useCurrency } from '../hooks/useCurrency'
import { usePrinters } from '../hooks/usePrinters'
import { formatDuration, getStatusBadge } from '../lib/utils'

// Shape returned by GET /archives/compare.
type ArchiveComparison = {
  a: PrintArchive
  b: PrintArchive
  differences: Record<string, boolean>
}

const JOB_STATUS_LABELS: Record<string, string> = {"queued":"Queued","assigned":"Assigned","uploaded":"Uploaded","started":"Started","progress":"Progress","paused":"Paused","resumed":"Resumed","completed":"Completed","failed":"Failed","cancelled":"Cancelled","retried":"Retried","idle":"Idle","printing":"Printing","offline":"Offline","error":"Error","unassigned":"Unassigned","available":"Available","empty":"Empty","in_use":"In use","running":"Running","succeeded":"Succeeded","interrupted":"Interrupted","user":"User","system":"System","printer":"Printer","api":"Api"}

const STATUS_FILTERS = [
  { value: '', label: 'All statuses' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'cancelled', label: 'Cancelled' },
]

export default function Archives() {
  const { t, i18n } = useTranslation('library')
  const { formatCents } = useCurrency()
  const { data: printers } = usePrinters()
  const [items, setItems] = useState<PrintArchive[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [comparison, setComparison] = useState<ArchiveComparison | null>(null)
  const [compareError, setCompareError] = useState<string | null>(null)
  const [comparing, setComparing] = useState(false)

  const printerNames = useMemo(() => {
    const names = new Map<string, string>()
    for (const printer of printers ?? []) names.set(printer.id, printer.name)
    return names
  }, [printers])

  useEffect(() => {
    let active = true
    setLoading(true)
    archivesApi.list(undefined, statusFilter || undefined)
      .then((data) => {
        if (!active) return
        setItems(data)
        setError(null)
      })
      .catch((err: unknown) => {
        if (!active) return
        setError(err instanceof Error ? err.message : t("Failed to load archives"))
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [statusFilter, t])

  // A new filter invalidates both the selection and an open comparison.
  useEffect(() => {
    setSelected([])
    setComparison(null)
    setCompareError(null)
  }, [statusFilter])

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      if (prev.includes(id)) return prev.filter((entry) => entry !== id)
      // Only two archives can be compared: the newest pick replaces the oldest.
      if (prev.length >= 2) return [prev[1], id]
      return [...prev, id]
    })
  }

  const runCompare = async () => {
    if (selected.length !== 2) return
    setComparing(true)
    setCompareError(null)
    try {
      const result = (await archivesApi.compare(selected[0], selected[1])) as ArchiveComparison
      setComparison(result)
    } catch (err: unknown) {
      setComparison(null)
      setCompareError(err instanceof Error ? err.message : t("Failed to compare archives"))
    } finally {
      setComparing(false)
    }
  }

  const printerLabel = (id?: string) => (id ? printerNames.get(id) ?? id : '—')

  const formatEndTime = (value?: string | null) => {
    if (!value) return '—'
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString(i18n.resolvedLanguage)
  }

  const comparisonRows = comparison
    ? [
        {
          field: 'status',
          label: t("Status"),
          a: t(comparison.a.status === 'completed' ? 'Completed' : comparison.a.status === 'failed' ? 'Failed' : 'Cancelled'),
          b: t(comparison.b.status === 'completed' ? 'Completed' : comparison.b.status === 'failed' ? 'Failed' : 'Cancelled'),
        },
        {
          field: 'printer_id',
          label: t("Printer"),
          a: printerLabel(comparison.a.printer_id),
          b: printerLabel(comparison.b.printer_id),
        },
        {
          field: 'duration_seconds',
          label: t("Duration"),
          a: formatDuration(comparison.a.duration_seconds),
          b: formatDuration(comparison.b.duration_seconds),
        },
        {
          field: 'filament_used_grams',
          label: t("Filament"),
          a: `${comparison.a.filament_used_grams.toFixed(1)}g`,
          b: `${comparison.b.filament_used_grams.toFixed(1)}g`,
        },
        {
          field: 'cost_cents',
          label: t("Cost"),
          a: formatCents(comparison.a.cost_cents),
          b: formatCents(comparison.b.cost_cents),
        },
      ]
    : []

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-semibold">{t("Print Archives")}</h1>
        <div className="flex items-center gap-2">
          <select
            className="input"
            aria-label={t("Filter by status")}
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            {STATUS_FILTERS.map((option) => (
              <option key={option.value} value={option.value}>{t(option.label)}</option>
            ))}
          </select>
          <button
            className="btn btn-primary"
            disabled={selected.length !== 2 || comparing}
            onClick={runCompare}
          >
            {comparing ? t("Comparing...") : t("Compare selected")}
          </button>
          <a className="btn btn-secondary" href={archivesApi.exportCSV()}>{t("Export CSV")}</a>
        </div>
      </div>

      {error && (
        <div className="card p-3 mb-4 text-sm text-red-400 border border-red-500/40">
          {t("Failed to load archives:")}{' '}{error}
        </div>
      )}

      {comparison && (
        <div className="card p-4 mb-4">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-lg font-medium">{t("Comparison")}</h2>
            <button className="btn btn-secondary" onClick={() => setComparison(null)}>{t("Close")}</button>
          </div>
          <table className="w-full text-sm">
            <thead className="bg-surface-800 text-surface-400">
              <tr>
                <th className="p-3 text-left">{t("Metric")}</th>
                <th className="p-3 text-left">{t("A")}</th>
                <th className="p-3 text-left">{t("B")}</th>
              </tr>
            </thead>
            <tbody>
              {comparisonRows.map((row) => {
                const differs = comparison.differences[row.field]
                return (
                  <tr key={row.field} className={`border-t border-surface-800 ${differs ? 'bg-amber-500/10' : ''}`}>
                    <td className="p-3 text-surface-400">{row.label}</td>
                    <td className="p-3">{row.a}</td>
                    <td className="p-3">{row.b}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
          <p className="mt-2 text-xs text-surface-500">{t("Highlighted rows are the fields that differ between the two archives.")}</p>
        </div>
      )}

      {compareError && (
        <div className="card p-3 mb-4 text-sm text-red-400 border border-red-500/40">
          {t("Failed to compare archives:")}{' '}{compareError}
        </div>
      )}

      <div className="card overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-surface-800 text-surface-400">
            <tr>
              <th className="p-3 w-8" />
              <th className="p-3 text-left">{t("Status")}</th>
              <th className="p-3 text-left">{t("Printer")}</th>
              <th className="p-3 text-left">{t("Duration")}</th>
              <th className="p-3 text-left">{t("Filament")}</th>
              <th className="p-3 text-left">{t("Cost")}</th>
              <th className="p-3 text-left">{t("Ended")}</th>
              <th className="p-3 text-left">{t("Notes")}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className={`border-t border-surface-800 ${selected.includes(item.id) ? 'bg-surface-800/40' : ''}`}>
                <td className="p-3">
                  <input
                    type="checkbox"
                    aria-label={t("Select archive {{name}}", { name: item.id })}
                    checked={selected.includes(item.id)}
                    onChange={() => toggleSelect(item.id)}
                  />
                </td>
                <td className="p-3">
                  <span className={`px-2 py-0.5 rounded text-xs ${getStatusBadge(item.status)}`}>{t(JOB_STATUS_LABELS[item.status] || item.status)}</span>
                </td>
                <td className="p-3">{printerLabel(item.printer_id)}</td>
                <td className="p-3">{item.duration_seconds > 0 ? formatDuration(item.duration_seconds) : '—'}</td>
                <td className="p-3">{item.filament_used_grams.toFixed(1)}{t("g")}</td>
                <td className="p-3">{formatCents(item.cost_cents)}</td>
                <td className="p-3 text-surface-400">{formatEndTime(item.end_time)}</td>
                <td className="p-3 text-surface-400 max-w-64 truncate" title={item.notes}>{item.notes || '—'}</td>
              </tr>
            ))}
            {items.length === 0 && !loading && (
              <tr><td className="p-3 text-surface-400" colSpan={8}>{t("No archives yet. Finished prints are archived automatically.")}</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {loading && <div className="p-3 text-surface-400">{t("Loading archives...")}</div>}
    </div>
  )
}
