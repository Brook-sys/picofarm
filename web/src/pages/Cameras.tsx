import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  Camera as CameraIcon,
  CheckCircle,
  ExternalLink,
  Loader2,
  MonitorPlay,
  Printer as PrinterIcon,
  RefreshCw,
  Video,
  WifiOff,
} from 'lucide-react'
import { camerasApi } from '../api/client'
import { usePrinters, usePrinterStates } from '../hooks/usePrinters'
import { cn, formatRelativeTime, getStatusBadge } from '../lib/utils'
import type { Camera, Printer } from '../types'

function cameraTypeLabel(type: string) {
  switch (type) {
    case 'mjpeg':
      return 'MJPEG stream'
    case 'snapshot':
      return 'Snapshot'
    case 'rtsp':
      return 'RTSP'
    case 'webrtc':
      return 'WebRTC'
    default:
      return type.toUpperCase()
  }
}

function canPreviewInline(camera: Camera) {
  return camera.type === 'mjpeg' || camera.type === 'snapshot'
}

function printerLabel(printer?: Printer) {
  if (!printer) return 'Unassigned camera'
  return [printer.name, printer.model].filter(Boolean).join(' • ')
}

export default function Cameras() {
  const [selectedPrinterId, setSelectedPrinterId] = useState('all')
  const [failedPreviews, setFailedPreviews] = useState<Record<string, boolean>>({})

  const { data: printers = [], isLoading: printersLoading } = usePrinters()
  const { data: printerStates = {} } = usePrinterStates()
  const {
    data: cameras = [],
    isLoading: camerasLoading,
    isFetching: camerasFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: ['cameras', selectedPrinterId],
    queryFn: () => camerasApi.list(selectedPrinterId === 'all' ? undefined : selectedPrinterId),
  })

  const printerById = useMemo(() => new Map(printers.map(printer => [printer.id, printer])), [printers])
  const camerasByPrinter = useMemo(() => {
    const grouped = new Map<string, Camera[]>()
    cameras.forEach(camera => {
      const key = camera.printer_id || 'unassigned'
      grouped.set(key, [...(grouped.get(key) || []), camera])
    })
    return grouped
  }, [cameras])

  const onlinePrinters = printers.filter(printer => {
    const status = printerStates[printer.id]?.status || printer.status
    return status !== 'offline'
  }).length
  const enabledCameras = cameras.filter(camera => camera.enabled).length
  const discoveredCameras = cameras.filter(camera => camera.printer_id).length
  const selectedPrinter = selectedPrinterId === 'all' ? undefined : printerById.get(selectedPrinterId)
  const loading = printersLoading || camerasLoading

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-accent-300">
            <MonitorPlay className="h-4 w-4" />
            Live monitoring
          </div>
          <h1 className="font-display text-3xl font-bold text-surface-100">Cameras</h1>
          <p className="mt-2 max-w-3xl text-sm text-surface-400">
            Veja os streams configurados no Moonraker por impressora, junto com estado atual,
            origem do stream e links rápidos para diagnóstico.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={selectedPrinterId}
            onChange={event => setSelectedPrinterId(event.target.value)}
            className="rounded-xl border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-surface-200 outline-none transition-colors focus:border-accent-500"
          >
            <option value="all">All printers</option>
            {printers.map(printer => (
              <option key={printer.id} value={printer.id}>{printer.name}</option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => refetch()}
            className="btn btn-secondary text-sm"
            disabled={camerasFetching}
          >
            <RefreshCw className={cn('mr-2 h-4 w-4', camerasFetching && 'animate-spin')} />
            Refresh
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <SummaryCard label="Active cameras" value={`${enabledCameras}/${cameras.length}`} helper="enabled / total" icon={<CameraIcon className="h-5 w-5" />} />
        <SummaryCard label="Mapped printers" value={`${discoveredCameras}`} helper="streams linked to printers" icon={<PrinterIcon className="h-5 w-5" />} />
        <SummaryCard label="Printers online" value={`${onlinePrinters}/${printers.length}`} helper="current fleet state" icon={<CheckCircle className="h-5 w-5" />} />
      </div>

      {selectedPrinter && (
        <PrinterContextCard printer={selectedPrinter} status={printerStates[selectedPrinter.id]?.status || selectedPrinter.status} />
      )}

      {error && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200">
          <div className="flex items-center gap-2 font-semibold">
            <AlertTriangle className="h-4 w-4" />
            Failed to load cameras
          </div>
          <p className="mt-1 text-red-200/80">{error instanceof Error ? error.message : 'Unknown error'}</p>
        </div>
      )}

      {loading ? (
        <div className="card flex min-h-64 items-center justify-center p-8 text-surface-400">
          <Loader2 className="mr-3 h-5 w-5 animate-spin" />
          Loading printers and camera streams...
        </div>
      ) : cameras.length === 0 ? (
        <div className="card border-dashed p-8 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-surface-800 text-surface-400">
            <WifiOff className="h-7 w-7" />
          </div>
          <h2 className="text-lg font-semibold text-surface-200">No cameras found</h2>
          <p className="mx-auto mt-2 max-w-xl text-sm text-surface-500">
            Nenhuma câmera foi retornada para {selectedPrinter ? selectedPrinter.name : 'a frota'}.
            Verifique se o Moonraker responde em <code className="text-surface-300">/server/webcams/list</code>
            ou cadastre uma câmera manual apontando para o stream MJPEG.
          </p>
        </div>
      ) : selectedPrinterId === 'all' ? (
        <div className="space-y-5">
          {Array.from(camerasByPrinter.entries()).map(([printerId, group]) => {
            const printer = printerById.get(printerId)
            const status = printer ? printerStates[printer.id]?.status || printer.status : 'offline'
            return (
              <section key={printerId} className="card overflow-hidden">
                <div className="flex flex-col gap-3 border-b border-surface-800 p-5 md:flex-row md:items-center md:justify-between">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="text-lg font-semibold text-surface-100">{printerLabel(printer)}</h2>
                      <span className={cn('badge', getStatusBadge(status))}>{status}</span>
                    </div>
                    <p className="mt-1 text-xs text-surface-500">
                      {printer ? printer.connection_uri : 'No printer attached'}
                    </p>
                  </div>
                  {printer && (
                    <Link to={`/printers/${printer.id}`} className="btn btn-secondary text-xs">
                      Open printer
                      <ExternalLink className="ml-2 h-3.5 w-3.5" />
                    </Link>
                  )}
                </div>
                <div className="grid grid-cols-1 gap-4 p-5 xl:grid-cols-2">
                  {group.map(camera => (
                    <CameraCard
                      key={camera.id}
                      camera={camera}
                      printer={printer}
                      previewFailed={!!failedPreviews[camera.id]}
                      onPreviewError={() => setFailedPreviews(prev => ({ ...prev, [camera.id]: true }))}
                    />
                  ))}
                </div>
              </section>
            )
          })}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {cameras.map(camera => (
            <CameraCard
              key={camera.id}
              camera={camera}
              printer={printerById.get(camera.printer_id || '')}
              previewFailed={!!failedPreviews[camera.id]}
              onPreviewError={() => setFailedPreviews(prev => ({ ...prev, [camera.id]: true }))}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function SummaryCard({ label, value, helper, icon }: { label: string; value: string; helper: string; icon: React.ReactNode }) {
  return (
    <div className="card p-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-surface-500">{label}</p>
          <p className="mt-2 text-3xl font-bold text-surface-100">{value}</p>
          <p className="mt-1 text-xs text-surface-500">{helper}</p>
        </div>
        <div className="rounded-2xl bg-accent-500/15 p-3 text-accent-300">{icon}</div>
      </div>
    </div>
  )
}

function PrinterContextCard({ printer, status }: { printer: Printer; status: string }) {
  return (
    <div className="card p-5">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <PrinterIcon className="h-5 w-5 text-accent-300" />
            <h2 className="text-lg font-semibold text-surface-100">{printer.name}</h2>
            <span className={cn('badge', getStatusBadge(status))}>{status}</span>
            <span className="badge bg-surface-800 text-surface-400">{printer.connection_type.replace('_', ' ')}</span>
          </div>
          <p className="mt-2 text-sm text-surface-400">
            {[printer.manufacturer, printer.model, printer.location].filter(Boolean).join(' • ') || 'No printer metadata'}
          </p>
        </div>
        <div className="grid grid-cols-1 gap-2 text-xs text-surface-400 sm:grid-cols-2 lg:min-w-96">
          <InfoRow label="Moonraker" value={printer.connection_uri || 'Not configured'} />
          <InfoRow label="Fluidd" value={printer.fluidd_url || 'Not configured'} />
        </div>
      </div>
    </div>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-surface-800 bg-surface-950/60 px-3 py-2">
      <div className="text-[10px] font-semibold uppercase tracking-wider text-surface-600">{label}</div>
      <div className="mt-1 truncate text-surface-300">{value}</div>
    </div>
  )
}

function CameraCard({
  camera,
  printer,
  previewFailed,
  onPreviewError,
}: {
  camera: Camera
  printer?: Printer
  previewFailed: boolean
  onPreviewError: () => void
}) {
  const previewable = canPreviewInline(camera)
  return (
    <article className="overflow-hidden rounded-2xl border border-surface-800 bg-surface-900/70 shadow-lg shadow-black/10">
      <div className="relative aspect-video bg-black">
        {previewable && !previewFailed ? (
          <img
            src={camera.url}
            alt={`${camera.name} stream`}
            className="h-full w-full object-contain"
            onError={onPreviewError}
          />
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-surface-500">
            <Video className="h-10 w-10" />
            <div>
              <div className="text-sm font-medium text-surface-300">
                {previewFailed ? 'Preview failed to load' : `${cameraTypeLabel(camera.type)} preview unavailable`}
              </div>
              <div className="mt-1 text-xs">Open the stream directly to verify connectivity.</div>
            </div>
          </div>
        )}
        <div className="absolute left-3 top-3 flex flex-wrap gap-2">
          <span className={cn('badge backdrop-blur', camera.enabled ? 'bg-emerald-500/25 text-emerald-200' : 'bg-surface-800/90 text-surface-400')}>
            {camera.enabled ? 'Enabled' : 'Disabled'}
          </span>
          <span className="badge bg-black/60 text-surface-200 backdrop-blur">{cameraTypeLabel(camera.type)}</span>
        </div>
      </div>

      <div className="space-y-4 p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h3 className="text-lg font-semibold text-surface-100">{camera.name}</h3>
            <p className="mt-1 text-sm text-surface-400">{printerLabel(printer)}</p>
          </div>
          {printer && (
            <Link to={`/printers/${printer.id}`} className="btn btn-secondary text-xs">
              Printer
              <ExternalLink className="ml-2 h-3.5 w-3.5" />
            </Link>
          )}
        </div>

        <div className="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
          <InfoRow label="Stream URL" value={camera.url} />
          <InfoRow label="Updated" value={formatRelativeTime(camera.updated_at)} />
        </div>

        <a
          href={camera.url}
          target="_blank"
          rel="noreferrer"
          className="inline-flex w-full items-center justify-center rounded-xl border border-accent-500/40 bg-accent-500/10 px-3 py-2 text-sm font-semibold text-accent-200 transition-colors hover:bg-accent-500/20"
        >
          Open stream directly
          <ExternalLink className="ml-2 h-4 w-4" />
        </a>
      </div>
    </article>
  )
}
