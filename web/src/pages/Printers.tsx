import { useTranslation } from 'react-i18next'
import { externalLinkClick } from '../lib/platform'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Plus,
  Printer as PrinterIcon,
  Wifi,
  WifiOff,
  Trash2,
  Radar,
  Loader2,
  Check,
  Key,
  Cloud,
  AlertTriangle,
  XCircle,
  ExternalLink,
  RefreshCw
} from 'lucide-react'
import { usePrinters, useCreatePrinter, useDeletePrinter, usePrinterStates, useDefaultPrinter, useSetDefaultPrinter } from '../hooks/usePrinters'
import { printersApi, bambuCloudApi, type DiscoveredPrinter } from '../api/client'
import { cn, getStatusBadge, PRINTER_STATUS_LABELS } from '../lib/utils'
import { useCurrency } from '../hooks/useCurrency'
import type { ConnectionType, CloudDevice } from '../types'

export default function Printers() {
  const { t } = useTranslation('fleet')

  const { formatAmount, symbol } = useCurrency()
  const { data: printers = [], isLoading, refetch } = usePrinters()
  const { data: printerStates = {} } = usePrinterStates()
  const { data: defaultPrinter } = useDefaultPrinter()
  const setDefaultPrinter = useSetDefaultPrinter()
  const createPrinter = useCreatePrinter()
  const deletePrinter = useDeletePrinter()

  const [showAdd, setShowAdd] = useState(false)
  const [addConnectionType, setAddConnectionType] = useState<ConnectionType>('manual')
  const [showDiscover, setShowDiscover] = useState(false)
  const [discovering, setDiscovering] = useState(false)
  const [discovered, setDiscovered] = useState<DiscoveredPrinter[]>([])
  const [addingPrinter, setAddingPrinter] = useState<string | null>(null)
  const [bambuSetup, setBambuSetup] = useState<string | null>(null) // printer.id being set up
  const [accessCode, setAccessCode] = useState('')
  const [serialNumber, setSerialNumber] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [emergencyTarget, setEmergencyTarget] = useState<{ id: string; name: string } | null>(null)
  const [reconnectingPrinter, setReconnectingPrinter] = useState<string | null>(null)
  const [createError, setCreateError] = useState('')

  // Bambu Cloud state
  const [showCloud, setShowCloud] = useState(false)
  const [cloudStep, setCloudStep] = useState<'login' | 'verify' | 'devices'>('login')
  const [cloudEmail, setCloudEmail] = useState('')
  const [cloudPassword, setCloudPassword] = useState('')
  const [cloudCode, setCloudCode] = useState('')
  const [cloudDevices, setCloudDevices] = useState<CloudDevice[]>([])
  const [cloudLoading, setCloudLoading] = useState(false)
  const [cloudError, setCloudError] = useState('')
  const [addingCloudDevice, setAddingCloudDevice] = useState<string | null>(null)
  const [addedCloudDevices, setAddedCloudDevices] = useState<Set<string>>(new Set())

  const handleCloudLogin = async () => {
    setCloudLoading(true)
    setCloudError('')
    try {
      const res = await bambuCloudApi.login(cloudEmail, cloudPassword)
      if (res.status === 'verify_code_required') {
        setCloudStep('verify')
      } else {
        // Direct login succeeded, fetch devices
        const devices = await bambuCloudApi.devices()
        setCloudDevices(devices || [])
        setCloudStep('devices')
      }
    } catch (err) {
      setCloudError(err instanceof Error ? err.message : t("Login failed"))
    } finally {
      setCloudLoading(false)
    }
  }

  const handleCloudVerify = async () => {
    setCloudLoading(true)
    setCloudError('')
    try {
      await bambuCloudApi.verify(cloudEmail, cloudCode)
      const devices = await bambuCloudApi.devices()
      setCloudDevices(devices || [])
      setCloudStep('devices')
    } catch (err) {
      setCloudError(err instanceof Error ? err.message : t("Verification failed"))
    } finally {
      setCloudLoading(false)
    }
  }

  const handleAddCloudDevice = async (devId: string) => {
    setAddingCloudDevice(devId)
    setCloudError('')
    try {
      await bambuCloudApi.addDevice(devId)
      setAddedCloudDevices(prev => new Set(prev).add(devId))
      refetch()
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("Failed to add device")
      setCloudError(msg)
      console.error('Failed to add cloud device:', err)
    } finally {
      setAddingCloudDevice(null)
    }
  }

  const resetCloudFlow = () => {
    setShowCloud(false)
    setCloudStep('login')
    setCloudEmail('')
    setCloudPassword('')
    setCloudCode('')
    setCloudDevices([])
    setCloudError('')
    setAddedCloudDevices(new Set())
  }

  const handleDiscover = async () => {
    setShowDiscover(true)
    setDiscovering(true)
    setDiscovered([])

    try {
      console.log('[Printers] Starting discovery...')
      const found = await printersApi.discover()
      console.log('[Printers] Discovery complete, found:', found)
      setDiscovered(found || [])
    } catch (err) {
      console.error('[Printers] Discovery failed:', err)
    } finally {
      console.log('[Printers] Discovery finished')
      setDiscovering(false)
    }
  }

  const handleAddDiscovered = async (printer: DiscoveredPrinter) => {
    // Bambu printers need an access code before adding
    if (printer.type === 'bambu_lan') {
      setBambuSetup(printer.id)
      setAccessCode('')
      return
    }

    setAddingPrinter(printer.id)
    try {
      await createPrinter.mutateAsync({
        name: printer.name,
        model: printer.model || '',
        manufacturer: printer.manufacturer || '',
        connection_type: printer.type,
        connection_uri: `http://${printer.host}:${printer.port}`,
      })
      // Mark as added
      setDiscovered(prev =>
        prev.map(p => p.id === printer.id ? { ...p, already_added: true } : p)
      )
      refetch()
    } catch (err) {
      console.error('Failed to add printer:', err)
    } finally {
      setAddingPrinter(null)
    }
  }

  const handleAddBambu = async (printer: DiscoveredPrinter) => {
    if (!serialNumber.trim()) return
    setAddingPrinter(printer.id)
    try {
      const req: Record<string, unknown> = {
        name: printer.name,
        model: printer.model || '',
        manufacturer: printer.manufacturer || '',
        connection_type: printer.type,
        connection_uri: printer.host,
        serial_number: serialNumber.trim(),
      }
      if (accessCode.trim()) {
        req.api_key = accessCode.trim()
      }
      await createPrinter.mutateAsync(req as Parameters<typeof createPrinter.mutateAsync>[0])
      setDiscovered(prev =>
        prev.map(p => p.id === printer.id ? { ...p, already_added: true } : p)
      )
      setBambuSetup(null)
      setAccessCode('')
      setSerialNumber('')
      refetch()
    } catch (err) {
      console.error('Failed to add Bambu printer:', err)
    } finally {
      setAddingPrinter(null)
    }
  }

  const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setCreateError('')
    const formData = new FormData(e.currentTarget)

    try {
      await createPrinter.mutateAsync({
        name: formData.get('name') as string,
        model: formData.get('model') as string,
        manufacturer: formData.get('manufacturer') as string,
        connection_type: formData.get('connection_type') as ConnectionType,
        connection_uri: formData.get('connection_uri') as string,
        fluidd_url: formData.get('fluidd_url') as string,
        location: formData.get('location') as string,
        cost_per_hour_cents: Math.round(parseFloat(formData.get('cost_per_hour') as string || '0') * 100),
        restrict_gcode_model: formData.get('restrict_gcode_model') === 'on',
        default_print_folder: formData.get('default_print_folder') as string,
      })
      setShowAdd(false)
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : t("Failed to add printer"))
    }
  }

  const handleDelete = async (id: string) => {
    if (confirmDelete !== id) {
      setConfirmDelete(id)
      return
    }
    await deletePrinter.mutateAsync(id)
    setConfirmDelete(null)
  }

  const handleMaintenance = async (id: string, enabled: boolean) => {
    await printersApi.setMaintenanceMode(id, enabled)
    refetch()
  }

  const handleReconnect = async (id: string) => {
    setReconnectingPrinter(id)
    try {
      await printersApi.reconnect(id)
      window.setTimeout(() => refetch(), 1500)
    } finally {
      window.setTimeout(() => setReconnectingPrinter(null), 1500)
    }
  }

  const confirmEmergencyStop = async () => {
    if (!emergencyTarget) return
    await printersApi.runMacro(emergencyTarget.id, 'M112')
    setEmergencyTarget(null)
    refetch()
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8">
      {emergencyTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
          <div className="w-full max-w-lg rounded-2xl border border-red-500/40 bg-surface-950 shadow-2xl">
            <div className="border-b border-red-500/20 bg-red-500/10 p-5">
              <div className="flex items-center gap-3">
                <div className="rounded-xl bg-red-500/20 p-3 text-red-300"><XCircle className="h-7 w-7" /></div>
                <div>
                  <h2 className="text-xl font-semibold text-red-200">{t("Confirm emergency stop")}</h2>
                  <p className="text-sm text-red-300/80">{emergencyTarget.name}</p>
                </div>
              </div>
            </div>
            <div className="space-y-3 p-5 text-sm text-surface-300">
              <p>{t("Are you sure you want to perform an emergency stop?")}</p>
              <p>{t("This attempts to stop the printer immediately. The current print job may be lost. Inspect the machine manually; a restart or homing may be needed before operating again.")}</p>
              <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-amber-200">
                {t("Use only for danger, critical failure, or a real need to stop the machine immediately.")}
              </div>
            </div>
            <div className="flex justify-end gap-2 border-t border-surface-800 p-5">
              <button onClick={() => setEmergencyTarget(null)} className="btn btn-secondary cursor-pointer">{t("Cancel")}</button>
              <button onClick={confirmEmergencyStop} className="inline-flex cursor-pointer items-center rounded-lg border border-red-500/70 bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-500">{t("Yes, stop now")}</button>
            </div>
          </div>
        </div>
      )}
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-display font-bold text-surface-100">
            {t("Printers")}
          </h1>
          <p className="text-surface-400 mt-1">
            {t("Manage your print farm")}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowCloud(true)}
            className="btn btn-secondary"
          >
            <Cloud className="h-4 w-4 mr-2" />
            Bambu Cloud
          </button>
          <button
            onClick={handleDiscover}
            className="btn btn-secondary"
          >
            <Radar className="h-4 w-4 mr-2" />
            {t("Scan Network")}
          </button>
          <button
            onClick={() => { setCreateError(''); setShowAdd(true) }}
            className="btn btn-primary"
          >
            <Plus className="h-4 w-4 mr-2" />
            {t("Add Manually")}
          </button>
        </div>
      </div>

      {/* Printers Grid */}
      {isLoading ? (
        <div className="text-surface-500">{t("Loading...")}</div>
      ) : printers.length === 0 ? (
        <div className="text-center py-16">
          <PrinterIcon className="h-16 w-16 mx-auto mb-4 text-surface-600" />
          <h3 className="text-xl font-semibold text-surface-300 mb-2">
            {t("No printers configured")}
          </h3>
          <p className="text-surface-500 mb-4">
            {t("Add your first printer to start managing your farm")}
          </p>
          <button
            onClick={() => { setCreateError(''); setShowAdd(true) }}
            className="btn btn-primary"
          >
            <Plus className="h-4 w-4 mr-2" />
            {t("Add Printer")}
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {printers.map((printer) => {
            const state = printerStates[printer.id]
            const status = state?.status || 'offline'
            const cardClass = printer.maintenance_mode
              ? 'border-2 border-amber-400/90 bg-amber-500/15 shadow-[0_0_28px_rgba(245,158,11,0.24)]'
              : status === 'printing'
                ? 'border-2 border-emerald-400/90 bg-emerald-500/15 shadow-[0_0_32px_rgba(16,185,129,0.28)]'
                : status === 'idle'
                  ? 'border-2 border-blue-400/90 bg-blue-500/15 shadow-[0_0_28px_rgba(59,130,246,0.25)]'
                  : status === 'error'
                    ? 'border-2 border-red-400/90 bg-red-500/15 shadow-[0_0_30px_rgba(239,68,68,0.26)]'
                    : 'border-2 border-surface-700 bg-surface-900/30 opacity-60 grayscale-[0.45]'
            const stateBarClass = printer.maintenance_mode
              ? 'bg-amber-400'
              : status === 'printing'
                ? 'bg-emerald-400 animate-pulse'
                : status === 'idle'
                  ? 'bg-blue-400'
                  : status === 'error'
                    ? 'bg-red-400'
                    : 'bg-surface-600'
            const iconClass = printer.maintenance_mode
              ? 'bg-amber-500/30 text-amber-200 ring-2 ring-amber-400/50'
              : status === 'printing'
                ? 'bg-emerald-500/30 text-emerald-200 ring-2 ring-emerald-400/50 animate-pulse'
                : status === 'idle'
                  ? 'bg-blue-500/30 text-blue-200 ring-2 ring-blue-400/50'
                  : status === 'error'
                    ? 'bg-red-500/30 text-red-200 ring-2 ring-red-400/50'
                    : 'bg-surface-800 text-surface-500 ring-1 ring-surface-700'
            return (
              <Link key={printer.id} to={`/printers/${printer.id}`} className={cn('card relative overflow-hidden p-5 block border shadow-lg transition-all hover:scale-[1.01]', cardClass)}>
                <div className={cn('absolute inset-y-0 left-0 w-1.5', stateBarClass)} />
                <div className="flex flex-col gap-4 mb-4 pl-1">
                  <div className="flex min-w-0 items-center gap-3">
                    <div className={cn('p-2 rounded-lg', iconClass)}>
                      <PrinterIcon className="h-5 w-5" />
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-semibold text-surface-100 flex items-center gap-2 flex-wrap">
                        {printer.name}
                        {defaultPrinter?.id === printer.id && <span className="badge bg-accent-500/20 text-accent-300 border-accent-500/30">{t("Default")}</span>}
                      </h3>
                      <p className="text-sm text-surface-500">
                        {printer.model || printer.manufacturer || t("Unknown model")}
                      </p>
                      {printer.maintenance_mode && (
                        <span className="inline-flex mt-1 text-[10px] px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-400">
                          {t("Maintenance")}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2" onClick={(e) => { e.preventDefault(); e.stopPropagation() }}>
                      {(status === 'offline' || status === 'error') && !printer.maintenance_mode && printer.connection_type !== 'manual' && (
                        <button
                          onClick={() => handleReconnect(printer.id)}
                          disabled={reconnectingPrinter === printer.id}
                          className="inline-flex cursor-pointer items-center rounded-lg border border-blue-500/50 bg-blue-500/20 px-2.5 py-1 text-xs font-semibold text-blue-300 hover:bg-blue-500/30 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          <RefreshCw className={cn('h-3.5 w-3.5 mr-1', reconnectingPrinter === printer.id && 'animate-spin')} />
                          {t("Reconnect")}
                        </button>
                      )}
                      <button
                        onClick={() => handleMaintenance(printer.id, !printer.maintenance_mode)}
                       className={cn(
                         'inline-flex cursor-pointer items-center rounded-lg border px-2.5 py-1 text-xs font-semibold transition-colors',
                         printer.maintenance_mode ? 'border-emerald-500/40 bg-emerald-500/15 text-emerald-300 hover:bg-emerald-500/25' : 'border-amber-500/60 bg-amber-500/20 text-amber-200 hover:bg-amber-500/30'
                       )}
                     >
                       <AlertTriangle className="h-3.5 w-3.5 mr-1" />
                       {printer.maintenance_mode ? t("Exit maintenance") : t("Maintenance")}
                     </button>
                      <button onClick={() => setEmergencyTarget({ id: printer.id, name: printer.name })} className="inline-flex cursor-pointer items-center rounded-lg border border-red-500/60 bg-red-500/20 px-2.5 py-1 text-xs font-semibold text-red-300 hover:bg-red-500/30">
                        <XCircle className="h-3.5 w-3.5 mr-1" /> {t("Emergency")}
                      </button>
                      {printer.fluidd_url && (
                        <a href={printer.fluidd_url} onClick={externalLinkClick} target="_blank" rel="noreferrer" className="inline-flex cursor-pointer items-center rounded-lg border border-blue-500/50 bg-blue-500/20 px-2.5 py-1 text-xs font-semibold text-blue-300 hover:bg-blue-500/30">
                          <ExternalLink className="h-3.5 w-3.5 mr-1" /> Fluidd
                        </a>
                      )}
                      {defaultPrinter?.id !== printer.id && (
                       <button onClick={() => setDefaultPrinter.mutate(printer.id)} className="cursor-pointer text-xs rounded px-2 py-0.5 bg-accent-500/20 text-accent-300 hover:bg-accent-500/30">
                         {t("Set default")}
                       </button>
                     )}
                   </div>
                  {confirmDelete === printer.id ? (
                    <div className="flex items-center gap-1" onClick={(e) => {
                     const t = e.target as HTMLElement
                     if (t.closest('a') || t.closest('button')) {
                       e.stopPropagation()
                       return
                     }
                     e.preventDefault()
                     e.stopPropagation()
                   }}>
                      <button
                        onClick={() => setConfirmDelete(null)}
                        className="text-xs text-surface-500 hover:text-surface-300 px-1.5 py-0.5"
                      >
                        {t("Cancel")}
                      </button>
                      <button
                        onClick={() => handleDelete(printer.id)}
                        className="text-xs bg-red-500/20 text-red-400 hover:bg-red-500/30 rounded px-2 py-0.5"
                      >
                        {t("Delete")}
                      </button>
                    </div>
                  ) : (
                    <button
                      aria-label={t("Delete printer")}
                      onClick={(e) => { e.preventDefault(); e.stopPropagation(); handleDelete(printer.id) }}
                      className="p-1.5 rounded hover:bg-surface-800 text-surface-500 hover:text-red-400 transition-colors"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                </div>

                <div className="space-y-3">
                  {/* Status */}
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-surface-500">{t("Status")}</span>
                    <span className={cn('badge', getStatusBadge(state?.status || 'offline'))}>
                      {t(PRINTER_STATUS_LABELS[state?.status || 'offline'] || state?.status || 'offline')}
                    </span>
                  </div>

                  {/* Progress (if printing) */}
                  {state?.status === 'printing' && (
                    <div>
                      <div className="flex items-center justify-between text-sm mb-1">
                        <span className="text-surface-500">{t("Progress")}</span>
                        <span className="text-surface-300">{state.progress.toFixed(1)}%</span>
                      </div>
                      <div className="h-2 bg-surface-800 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-emerald-500 transition-all"
                          style={{ width: `${state.progress}%` }}
                        />
                      </div>
                      {state.current_file && (
                        <p className="text-xs text-surface-500 mt-1 truncate">
                          {state.current_file}
                        </p>
                      )}
                    </div>
                  )}

                  {/* Temperatures */}
                  {state && (state.bed_temp || state.nozzle_temp) && (
                    <div className="flex gap-4 text-sm">
                      {state.nozzle_temp !== undefined && (
                        <div>
                          <span className="text-surface-500">{t("Nozzle:")} </span>
                          <span className="text-surface-300">{state.nozzle_temp.toFixed(0)}°C</span>
                        </div>
                      )}
                      {state.bed_temp !== undefined && (
                        <div>
                          <span className="text-surface-500">{t("Bed:")} </span>
                          <span className="text-surface-300">{state.bed_temp.toFixed(0)}°C</span>
                        </div>
                      )}
                    </div>
                  )}

                  {/* Connection */}
                  <div className="flex items-center gap-2 text-sm">
                    {printer.connection_type === 'manual' ? (
                      <WifiOff className="h-4 w-4 text-surface-500" />
                    ) : (
                      <Wifi className={cn(
                        'h-4 w-4',
                        state?.status && state.status !== 'offline'
                          ? 'text-emerald-400'
                          : 'text-surface-500'
                      )} />
                    )}
                    <span className="text-surface-500">
                      {printer.connection_type === 'manual'
                        ? t("Manual")
                        : printer.connection_type.replace('_', ' ')}
                    </span>
                  </div>

                  {/* Location */}
                  {printer.location && (
                    <div className="text-sm text-surface-500">
                      📍 {printer.location}
                    </div>
                  )}
                </div>
              </Link>
            )
          })}
        </div>
      )}

      {/* Discover Printers Modal */}
      {showDiscover && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="card w-full max-w-2xl p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-xl font-semibold text-surface-100">
                {t("Network Discovery")}
              </h2>
              <button aria-label={t("Close")}
                onClick={() => setShowDiscover(false)}
                className="text-surface-500 hover:text-surface-300"
              >
                ✕
              </button>
            </div>

            {discovering ? (
              <div className="text-center py-12">
                <Loader2 className="h-12 w-12 mx-auto mb-4 text-accent-500 animate-spin" />
                <p className="text-surface-300">{t("Scanning your network...")}</p>
                <p className="text-surface-500 text-sm mt-1">
                  {t("Looking for OctoPrint, Moonraker, and Bambu printers")}
                </p>
                <p className="text-surface-600 text-xs mt-3">
                  {t("This can take 15-20 seconds. Please wait...")}
                </p>
              </div>
            ) : discovered.length === 0 ? (
              <div className="text-center py-12">
                <Radar className="h-12 w-12 mx-auto mb-4 text-surface-600" />
                <p className="text-surface-300">{t("No printers found")}</p>
                <p className="text-surface-500 text-sm mt-1">
                  {t("Make sure your printers are powered on and connected to the network")}
                </p>
                <button
                  onClick={handleDiscover}
                  className="btn btn-secondary mt-4"
                >
                  <Radar className="h-4 w-4 mr-2" />
                  {t("Scan Again")}
                </button>
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-surface-400 text-sm mb-4">
                  {t("Found {{count}} printers on your network", { count: discovered.length })}
                </p>
                {discovered.map((printer) => {
                  const isBambu = printer.type === 'bambu_lan'
                  return (
                  <div
                    key={printer.id}
                    className="rounded-lg bg-surface-800/50 border border-surface-700"
                  >
                    <div className="flex items-center justify-between p-4">
                      <div className="flex items-center gap-4">
                        <div className={cn(
                          'p-2 rounded-lg',
                          printer.type === 'octoprint' ? 'bg-green-500/20' :
                          printer.type === 'moonraker' ? 'bg-purple-500/20' :
                          isBambu ? 'bg-blue-500/20' :
                          'bg-surface-700'
                        )}>
                          <PrinterIcon className={cn(
                            'h-5 w-5',
                            printer.type === 'octoprint' ? 'text-green-400' :
                            printer.type === 'moonraker' ? 'text-purple-400' :
                            isBambu ? 'text-blue-400' :
                            'text-surface-400'
                          )} />
                        </div>
                        <div>
                          <div className="font-medium text-surface-100">
                            {printer.name}
                          </div>
                          <div className="text-sm text-surface-500">
                            {printer.host}:{printer.port} • {printer.type.replace('_', ' ')}
                            {printer.model && ` • ${printer.model}`}
                            {printer.version && ` • ${printer.version}`}
                          </div>
                        </div>
                      </div>
                      <div>
                        {printer.already_added ? (
                          <span className="flex items-center gap-1 text-emerald-400 text-sm">
                            <Check className="h-4 w-4" />
                            {t("Added")}
                          </span>
                        ) : !isBambu ? (
                          <button
                            onClick={() => handleAddDiscovered(printer)}
                            disabled={addingPrinter === printer.id}
                            className="btn btn-primary text-sm py-1.5 px-3"
                          >
                            {addingPrinter === printer.id ? (
                              <>
                                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                                {t("Adding...")}
                              </>
                            ) : (
                              <>
                                <Plus className="h-4 w-4 mr-1" />
                                {t("Add")}
                              </>
                            )}
                          </button>
                        ) : null}
                      </div>
                    </div>

                    {/* Bambu access code form — always visible for Bambu printers */}
                    {isBambu && !printer.already_added && (
                      <div className="px-4 pb-4 pt-0 border-t border-surface-700">
                        <div className="mt-3 space-y-3">
                          <div>
                            <label className="block text-sm font-medium text-surface-300 mb-1">
                              <Key className="h-3.5 w-3.5 inline mr-1" />
                              {t("LAN Access Code")} <span className="text-surface-500 font-normal">{t("(optional)")}</span>
                            </label>
                            <input
                              type="text"
                              value={bambuSetup === printer.id ? accessCode : ''}
                              onChange={(e) => { setBambuSetup(printer.id); setAccessCode(e.target.value) }}
                              onFocus={() => {
                                if (bambuSetup !== printer.id) {
                                  setBambuSetup(printer.id)
                                  setAccessCode('')
                                  setSerialNumber(printer.serial_number || '')
                                }
                              }}
                              className="input"
                              placeholder={t("Enter 8-digit access code")}
                            />
                            <p className="text-xs text-surface-500 mt-1">
                              {t("Not all models require this. Check Bambu Handy app, Bambu Studio, or printer LCD.")}
                            </p>
                          </div>
                          <div>
                            <label className="block text-sm font-medium text-surface-300 mb-1">
                              {t("Serial Number")}
                            </label>
                            <input
                              type="text"
                              value={bambuSetup === printer.id ? serialNumber : (printer.serial_number || '')}
                              onChange={(e) => { setBambuSetup(printer.id); setSerialNumber(e.target.value) }}
                              onFocus={() => {
                                if (bambuSetup !== printer.id) {
                                  setBambuSetup(printer.id)
                                  setAccessCode('')
                                  setSerialNumber(printer.serial_number || '')
                                }
                              }}
                              className="input"
                              placeholder={t("e.g. 01P00A000000000")}
                            />
                            <p className="text-xs text-surface-500 mt-1">
                              {printer.serial_number
                                ? t("Auto-detected. Edit if incorrect.")
                                : t("Find on the printer sticker, in Bambu Handy, or Bambu Studio")}
                            </p>
                          </div>
                          <div className="flex justify-end">
                            <button
                              onClick={() => handleAddBambu(printer)}
                              disabled={
                                !(bambuSetup === printer.id && serialNumber.trim()) ||
                                addingPrinter === printer.id
                              }
                              className="btn btn-primary text-sm py-1.5 px-3"
                            >
                              {addingPrinter === printer.id ? (
                                <>
                                  <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                                  {t("Connecting...")}
                                </>
                              ) : (
                                <>
                                  <Check className="h-4 w-4 mr-1" />
                                  {t("Connect")}
                                </>
                              )}
                            </button>
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                  )
                })}
                <div className="flex justify-between items-center mt-4 pt-4 border-t border-surface-800">
                  <button
                    onClick={handleDiscover}
                    className="btn btn-ghost text-sm"
                  >
                    <Radar className="h-4 w-4 mr-2" />
                    {t("Scan Again")}
                  </button>
                  <button aria-label={t("Close")}
                onClick={() => setShowDiscover(false)}
                    className="btn btn-secondary"
                  >
                    {t("Done")}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Bambu Cloud Modal */}
      {showCloud && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="card w-full max-w-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-xl font-semibold text-surface-100">
                {cloudStep === 'login' && t("Connect Bambu Cloud")}
                {cloudStep === 'verify' && t("Enter Verification Code")}
                {cloudStep === 'devices' && t("Your Bambu Printers")}
              </h2>
              <button aria-label={t("Close")}
                onClick={resetCloudFlow}
                className="text-surface-500 hover:text-surface-300"
              >
                ✕
              </button>
            </div>

            {cloudError && (
              <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 mb-4 text-sm text-red-400">
                {cloudError}
              </div>
            )}

            {cloudStep === 'login' && (
              <div className="space-y-4">
                <p className="text-sm text-surface-400">
                  {t("Sign in with your Bambu Lab account to connect printers via cloud MQTT. This works even when printers are not in LAN-only mode.")}
                </p>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    {t("Email")}
                  </label>
                  <input
                    type="email"
                    value={cloudEmail}
                    onChange={e => setCloudEmail(e.target.value)}
                    className="input"
                    placeholder="your@email.com"
                    autoFocus
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    {t("Password")}
                  </label>
                  <input
                    type="password"
                    value={cloudPassword}
                    onChange={e => setCloudPassword(e.target.value)}
                    className="input"
                    placeholder={t("Password")}
                    onKeyDown={e => e.key === 'Enter' && cloudEmail && cloudPassword && handleCloudLogin()}
                  />
                </div>
                <div className="flex justify-end gap-3">
                  <button aria-label={t("Close")}
                onClick={resetCloudFlow} className="btn btn-ghost">
                    {t("Cancel")}
                  </button>
                  <button
                    onClick={handleCloudLogin}
                    disabled={cloudLoading || !cloudEmail || !cloudPassword}
                    className="btn btn-primary"
                  >
                    {cloudLoading ? (
                      <>
                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                        {t("Signing in...")}
                      </>
                    ) : (
                      t("Sign In")
                    )}
                  </button>
                </div>
              </div>
            )}

            {cloudStep === 'verify' && (
              <div className="space-y-4">
                <p className="text-sm text-surface-400">
                  {t("A verification code has been sent to {{email}}. Enter the 6-digit code below.", { email: cloudEmail })}
                </p>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    {t("Verification Code")}
                  </label>
                  <input
                    type="text"
                    value={cloudCode}
                    onChange={e => setCloudCode(e.target.value)}
                    className="input text-center text-2xl tracking-widest"
                    placeholder="000000"
                    maxLength={6}
                    autoFocus
                    onKeyDown={e => e.key === 'Enter' && cloudCode.length >= 6 && handleCloudVerify()}
                  />
                </div>
                <div className="flex justify-end gap-3">
                  <button onClick={() => setCloudStep('login')} className="btn btn-ghost">
                    {t("Back")}
                  </button>
                  <button
                    onClick={handleCloudVerify}
                    disabled={cloudLoading || cloudCode.length < 6}
                    className="btn btn-primary"
                  >
                    {cloudLoading ? (
                      <>
                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                        {t("Verifying...")}
                      </>
                    ) : (
                      t("Verify")
                    )}
                  </button>
                </div>
              </div>
            )}

            {cloudStep === 'devices' && (
              <div className="space-y-3">
                {cloudDevices.length === 0 ? (
                  <p className="text-surface-400 text-center py-8">
                    {t("No printers found on your Bambu account.")}
                  </p>
                ) : (
                  <>
                    <p className="text-sm text-surface-400">
                      {t("Found {{count}} printers on your account. Click Add to connect via cloud MQTT.", { count: cloudDevices.length })}
                    </p>
                    {cloudDevices.map(device => (
                      <div
                        key={device.dev_id}
                        className="flex items-center justify-between p-4 rounded-lg bg-surface-800/50 border border-surface-700"
                      >
                        <div className="flex items-center gap-3">
                          <div className={cn(
                            'p-2 rounded-lg',
                            device.online ? 'bg-emerald-500/20' : 'bg-surface-700'
                          )}>
                            <PrinterIcon className={cn(
                              'h-5 w-5',
                              device.online ? 'text-emerald-400' : 'text-surface-500'
                            )} />
                          </div>
                          <div>
                            <div className="font-medium text-surface-100">
                              {device.name}
                            </div>
                            <div className="text-sm text-surface-500">
                              {device.dev_product_name || device.dev_model_name}
                              {device.online ? (
                                <span className="text-emerald-400 ml-2">{t("Online")}</span>
                              ) : (
                                <span className="text-surface-600 ml-2">{t("Offline")}</span>
                              )}
                            </div>
                          </div>
                        </div>
                        {addedCloudDevices.has(device.dev_id) ? (
                          <span className="flex items-center gap-1 text-emerald-400 text-sm">
                            <Check className="h-4 w-4" />
                            {t("Added")}
                          </span>
                        ) : (
                          <button
                            onClick={() => handleAddCloudDevice(device.dev_id)}
                            disabled={addingCloudDevice === device.dev_id}
                            className="btn btn-primary text-sm py-1.5 px-3"
                          >
                            {addingCloudDevice === device.dev_id ? (
                              <>
                                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                                {t("Adding...")}
                              </>
                            ) : (
                              <>
                                <Plus className="h-4 w-4 mr-1" />
                                {t("Add")}
                              </>
                            )}
                          </button>
                        )}
                      </div>
                    ))}
                  </>
                )}
                <div className="flex justify-end pt-4 border-t border-surface-800">
                  <button aria-label={t("Close")}
                onClick={resetCloudFlow} className="btn btn-secondary">
                    {t("Done")}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Add Printer Modal */}
      {showAdd && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="card w-full max-w-lg p-6">
            <h2 className="text-xl font-semibold text-surface-100 mb-4">
              {t("Add Printer")}
            </h2>
            <form onSubmit={handleCreate}>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      {t("Name *")}
                    </label>
                    <input
                      type="text"
                      name="name"
                      required
                      className="input"
                      placeholder="Prusa MK4 #1"
                      autoFocus
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      {t("Model")}
                    </label>
                    <input
                      type="text"
                      name="model"
                      className="input"
                      placeholder="Prusa MK4"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      {t("Manufacturer")}
                    </label>
                    <input
                      type="text"
                      name="manufacturer"
                      className="input"
                      placeholder="Prusa Research"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      {t("Location")}
                    </label>
                    <input
                      type="text"
                      name="location"
                      className="input"
                      placeholder={t("Workshop, Desk 3")}
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    {t("Cost per hour ({{symbol}})", { symbol })}
                  </label>
                  <input
                    type="number"
                    step="0.01"
                    name="cost_per_hour"
                    className="input"
                    placeholder="0.50"
                    defaultValue="0.50"
                  />
                  <p className="text-xs text-surface-500 mt-1">
                    {t("Suggested: A1 {{a1}} / P1S {{p1s}} / X1 {{x1}} — covers electricity, depreciation, maintenance, and utilization. You can adjust this later from the printer detail page.", { a1: formatAmount(0.50), p1s: formatAmount(0.75), x1: formatAmount(1) })}
                  </p>
                </div>
                 <label className="flex items-start gap-3 rounded-lg border border-surface-800 bg-surface-900/50 p-3">
                   <input type="checkbox" name="restrict_gcode_model" defaultChecked className="mt-1" />
                   <span>
                     <span className="block text-sm font-medium text-surface-200">{t("Only allow G-code for this printer model")}</span>
                     <span className="text-xs text-surface-500">{t("Block printing when the G-code printer_model does not match this printer model.")}</span>
                   </span>
                 </label>
                 <div>
                   <label className="block text-sm font-medium text-surface-300 mb-1">
                     {t("Connection Type")}
                  </label>
                  <select name="connection_type" value={addConnectionType} onChange={e => setAddConnectionType(e.target.value as ConnectionType)} className="input">
                    <option value="manual">{t("Manual (No Integration)")}</option>
                    <option value="octoprint">OctoPrint</option>
                    <option value="bambu_lan">Bambu Lab (LAN)</option>
                    <option value="moonraker">Moonraker (Klipper)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    {t("Connection URL")}
                  </label>
                  <input
                    type="text"
                    name="connection_uri"
                    className="input"
                    placeholder="http://192.168.1.100:7125"
                  />
                  <p className="text-xs text-surface-500 mt-1">
                    {t("Leave empty for manual printers")}
                  </p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    {t("Fluidd URL")}
                  </label>
                  <input
                    type="text"
                    name="fluidd_url"
                    className="input"
                    placeholder="http://192.168.1.100"
                  />
                  <p className="text-xs text-surface-500 mt-1">
                    {t("Optional. Moonraker defaults to the same host on port 80.")}
                  </p>
                </div>
                {(addConnectionType === 'moonraker' || addConnectionType === 'octoprint') && (
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      {t("Default print folder")}
                    </label>
                    <input
                      type="text"
                      name="default_print_folder"
                      className="input"
                      placeholder="/sda1"
                    />
                    <p className="text-xs text-surface-500 mt-1">
                      {t("Leave empty to use the printer root folder.")}
                    </p>
                  </div>
                )}
          {createError && <div className="text-sm text-red-400">{createError}</div>}
        </div>
              <div className="flex justify-end gap-3 mt-6">
                <button
                  type="button"
                  onClick={() => setShowAdd(false)}
                  className="btn btn-ghost"
                >
                  {t("Cancel")}
                </button>
                <button
                  type="submit"
                  disabled={createPrinter.isPending}
                  className="btn btn-primary"
                >
                  {createPrinter.isPending ? t("Adding...") : t("Add Printer")}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

