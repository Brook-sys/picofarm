import { useEffect, useMemo, useState } from 'react'
import { HardDrive } from 'lucide-react'
import { printersApi } from '../api/client'
import { PrinterFileBrowser } from '../components/PrinterFileBrowser'
import type { Printer } from '../types'

export default function PrinterFiles() {
  const [printers, setPrinters] = useState<Printer[]>([])
  const [selectedPrinterId, setSelectedPrinterId] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    printersApi.list()
      .then(items => {
        if (cancelled) return
        const supported = items.filter(printer => printer.connection_type === 'moonraker' || printer.connection_type === 'octoprint')
        setPrinters(supported)
        setSelectedPrinterId(current => current || supported[0]?.id || '')
      })
      .catch(err => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load printers')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [])

  const selectedPrinter = useMemo(() => printers.find(printer => printer.id === selectedPrinterId), [printers, selectedPrinterId])

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-3xl font-bold text-surface-100 flex items-center gap-3">
            <HardDrive className="h-8 w-8 text-accent-400" />
            Printer Files
          </h1>
          <p className="mt-2 text-surface-500">Manage internal print files stored directly on Moonraker/Fluidd/Mainsail or OctoPrint printers.</p>
        </div>
        <label className="block min-w-64">
          <span className="text-xs text-surface-500 mb-1 block">Printer</span>
          <select className="input" value={selectedPrinterId} onChange={e => setSelectedPrinterId(e.target.value)} disabled={loading || printers.length === 0}>
            {printers.length === 0 && <option value="">No supported printers</option>}
            {printers.map(printer => <option key={printer.id} value={printer.id}>{printer.name} · {printer.connection_type}</option>)}
          </select>
        </label>
      </div>

      {error && <div className="rounded-lg border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</div>}

      {loading ? (
        <div className="card p-8 text-center text-surface-500">Loading printers...</div>
      ) : selectedPrinter ? (
        <PrinterFileBrowser printerId={selectedPrinter.id} connectionType={selectedPrinter.connection_type} />
      ) : (
        <div className="card p-8 text-center text-surface-500">No Moonraker or OctoPrint printers configured.</div>
      )}
    </div>
  )
}
