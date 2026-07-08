import { useEffect, useMemo, useState } from 'react'
import { Activity, FolderTree, HardDrive, Server, Wifi } from 'lucide-react'
import { printersApi } from '../api/client'
import { PrinterFileBrowser } from '../components/PrinterFileBrowser'
import { cn } from '../lib/utils'
import type { Printer } from '../types'

export default function PrinterFiles() {
  const [printers, setPrinters] = useState<Printer[]>([])
  const [selectedPrinterId, setSelectedPrinterId] = useState('')
  const [loading, setLoading] = useState(true)
  const [stateLoading, setStateLoading] = useState(false)
  const [printerState, setPrinterState] = useState<import('../types').PrinterState | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
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

  useEffect(() => {
    if (!selectedPrinterId) {
      const timeout = window.setTimeout(() => setPrinterState(null), 0)
      return () => window.clearTimeout(timeout)
    }
    let cancelled = false
    const timeout = window.setTimeout(() => {
      if (cancelled) return
      setStateLoading(true)
      printersApi.getState(selectedPrinterId)
        .then(state => { if (!cancelled) setPrinterState(state) })
        .catch(() => { if (!cancelled) setPrinterState(null) })
        .finally(() => { if (!cancelled) setStateLoading(false) })
    }, 0)
    return () => {
      cancelled = true
      window.clearTimeout(timeout)
    }
  }, [selectedPrinterId])

  const effectiveConnection = selectedPrinter?.connection_type === 'moonraker' && selectedPrinter.connection_uri && !/:\d+($|\/)/.test(selectedPrinter.connection_uri.replace(/^https?:\/\//, ''))
    ? `${selectedPrinter.connection_uri.replace(/\/$/, '')}:7125`
    : selectedPrinter?.connection_uri || 'Configured'

  return (
    <div className="mx-auto flex h-[calc(100vh-3rem)] max-w-[1600px] flex-col gap-5 px-4 py-2 sm:px-6 lg:px-8">
      <div className="relative overflow-hidden rounded-2xl border border-surface-800 bg-gradient-to-br from-surface-900 via-surface-900 to-surface-950 p-6 shadow-xl shadow-black/20">
        <div className="absolute inset-y-0 right-0 w-1/2 bg-[radial-gradient(circle_at_top_right,rgba(249,115,22,0.14),transparent_45%)]" />
        <div className="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-3xl">
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-accent-500/30 bg-accent-500/10 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-accent-300">
              <HardDrive className="h-3.5 w-3.5" />
              Printer Storage
            </div>
            <h1 className="text-3xl font-bold tracking-tight text-surface-100 sm:text-4xl">Printer Files</h1>
            <p className="mt-2 text-sm leading-6 text-surface-400 sm:text-base">
              Browse, upload, download, organize and print files stored directly on Moonraker/Fluidd/Mainsail or OctoPrint printers.
            </p>
          </div>

          <div className="w-full max-w-sm rounded-xl border border-surface-800 bg-surface-950/70 p-4 backdrop-blur">
            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase tracking-wide text-surface-500">Active printer</span>
              <select className="input" value={selectedPrinterId} onChange={e => setSelectedPrinterId(e.target.value)} disabled={loading || printers.length === 0}>
                {printers.length === 0 && <option value="">No supported printers</option>}
                {printers.map(printer => <option key={printer.id} value={printer.id}>{printer.name} · {printer.connection_type}</option>)}
              </select>
            </label>
          </div>
        </div>
      </div>

      {selectedPrinter && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <InfoCard icon={Server} label="Protocol" value={selectedPrinter.connection_type === 'moonraker' ? 'Moonraker / Klipper' : 'OctoPrint'} />
          <InfoCard icon={Wifi} label="Effective connection" value={effectiveConnection} />
          <InfoCard icon={Activity} label="Live status" value={stateLoading ? 'checking...' : (printerState?.status || selectedPrinter.status || 'unknown')} status={printerState?.status || selectedPrinter.status} />
        </div>
      )}

      {error && <div className="rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</div>}

      <div className="min-h-0 flex-1 rounded-2xl border border-surface-800 bg-surface-950/60 p-3 shadow-xl shadow-black/10">
        {loading ? (
          <div className="flex h-full min-h-[560px] items-center justify-center rounded-xl border border-dashed border-surface-800 bg-surface-900/40 text-surface-500">
            Loading printers...
          </div>
        ) : selectedPrinter ? (
          <PrinterFileBrowser printerId={selectedPrinter.id} connectionType={selectedPrinter.connection_type} />
        ) : (
          <div className="flex h-full min-h-[560px] flex-col items-center justify-center rounded-xl border border-dashed border-surface-800 bg-surface-900/40 text-center text-surface-500">
            <FolderTree className="mb-3 h-10 w-10 text-surface-600" />
            <div className="text-base font-medium text-surface-300">No supported printers configured</div>
            <p className="mt-1 text-sm text-surface-500">Add a Moonraker or OctoPrint printer to manage printer-side files.</p>
          </div>
        )}
      </div>
    </div>
  )
}

function InfoCard({ icon: Icon, label, value, status }: { icon: typeof Server; label: string; value: string; status?: string }) {
  return (
    <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-4 shadow-lg shadow-black/10">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-surface-700 bg-surface-800/80 text-accent-300">
          <Icon className="h-5 w-5" />
        </div>
        <div className="min-w-0">
          <div className="text-xs font-semibold uppercase tracking-wide text-surface-500">{label}</div>
          <div className={cn('mt-0.5 truncate text-sm font-medium text-surface-100', status === 'offline' && 'text-red-300', status === 'idle' && 'text-emerald-300', status === 'printing' && 'text-accent-300')}>
            {value}
          </div>
        </div>
      </div>
    </div>
  )
}
