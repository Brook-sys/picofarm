import { useTranslation } from 'react-i18next'
import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useQuery } from '@tanstack/react-query'
import { Maximize2, Pause, Play, RefreshCw, X } from 'lucide-react'
import { camerasApi } from '../api/client'
import { usePrinters, usePrinterStates } from '../hooks/usePrinters'
import { CameraWallCard } from '../components/cameras/CameraWallCard'
import { CAMERA_WALL_STORAGE_KEY, normalizeCameraWallPreferences, orderCameraIds, readCameraWallPreferences } from '../components/cameras/cameraWallPreferences'
import type { CameraWallLayout, CameraWallSize } from '../components/cameras/cameraWallPreferences'
import { cn } from '../lib/utils'

const field = 'rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-surface-200'
const minimumWidth: Record<CameraWallSize, number> = { small: 240, medium: 360, large: 520, auto: 300 }

export default function Cameras() {
  const { t } = useTranslation('fleet')

  const [preferences, setPreferences] = useState(readCameraWallPreferences)
  const [filter, setFilter] = useState<'all' | 'printing' | 'favorites'>('all')
  const [printerFilter, setPrinterFilter] = useState('all')
  const [paused, setPaused] = useState<string[]>([])
  const [allPaused, setAllPaused] = useState(false)
  const [failures, setFailures] = useState<Record<string, string>>({})
  const [focusId, setFocusId] = useState<string | null>(null)
  const [wall, setWall] = useState(false)
  const wallButton = useRef<HTMLButtonElement>(null)
  const wallDialog = useRef<HTMLDialogElement>(null)
  const { data: printers = [] } = usePrinters()
  const { data: printerStates = {} } = usePrinterStates()
  const { data: cameras = [], isLoading, isFetching, error, refetch } = useQuery({ queryKey: ['cameras', 'all'], queryFn: () => camerasApi.list() })

  useEffect(() => {
    try { localStorage.setItem(CAMERA_WALL_STORAGE_KEY, JSON.stringify(preferences)) } catch { /* Storage can be unavailable or full; in-memory controls still work. */ }
  }, [preferences])

  const wasWall = useRef(false)
  useEffect(() => {
    if (!wall) {
      if (wasWall.current) wallButton.current?.focus()
      wasWall.current = false
      return
    }
    wasWall.current = true
    const dialog = wallDialog.current
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    dialog?.showModal()
    return () => {
      dialog?.close()
      document.body.style.overflow = previousOverflow
    }
  }, [wall])

  const byId = useMemo(() => new Map(cameras.map(camera => [camera.id, camera])), [cameras])
  const printerById = useMemo(() => new Map(printers.map(printer => [printer.id, printer])), [printers])
  const orderedIds = orderCameraIds(cameras.map(camera => camera.id), preferences.order)
  const visibleIds = orderedIds.filter(id => {
    const camera = byId.get(id)!
    const status = printerStates[camera.printer_id || '']?.status || printerById.get(camera.printer_id || '')?.status
    return (preferences.selected === null || preferences.selected.includes(id))
      && (printerFilter === 'all' || camera.printer_id === printerFilter)
      && (filter !== 'favorites' || preferences.favorites.includes(id))
      && (filter !== 'printing' || status === 'printing')
  })
  const focusedId = preferences.layout === 'focus' ? (visibleIds.includes(focusId || '') ? focusId : visibleIds[0]) : null

  function move(id: string, direction: -1 | 1) {
    const index = visibleIds.indexOf(id)
    const neighbor = visibleIds[index + direction]
    if (!neighbor) return
    const next = [...orderedIds]
    const from = next.indexOf(id)
    const to = next.indexOf(neighbor)
    ;[next[from], next[to]] = [next[to], next[from]]
    setPreferences(previous => ({ ...previous, order: next }))
  }

  const content = <div className="space-y-4 p-4 sm:p-6">
    <header className="flex flex-wrap items-center justify-between gap-3">
      <div><h1 id="camera-wall-title" className="font-display text-2xl font-bold text-surface-100">{t("Camera wall")}</h1><p className="mt-1 text-sm text-surface-400">{t("{{visible}} of {{total}} cameras · Monitoring only", { visible: visibleIds.length, total: cameras.length })}</p></div>
      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn btn-secondary text-sm" onClick={() => { setPreferences(normalizeCameraWallPreferences(null)); setFilter('all'); setPrinterFilter('all'); setFocusId(null) }}>{t("Reset layout")}</button>
        <button type="button" className="btn btn-secondary text-sm" onClick={() => setAllPaused(value => !value)}>{allPaused ? <Play className="mr-2 h-4 w-4" /> : <Pause className="mr-2 h-4 w-4" />}{allPaused ? t("Resume all") : t("Pause all")}</button>
        <button type="button" className="btn btn-secondary text-sm" onClick={() => void refetch()} disabled={isFetching}><RefreshCw className={cn('mr-2 h-4 w-4', isFetching && 'animate-spin')} />{t("Refresh list")}</button>
        <button ref={wall ? undefined : wallButton} type="button" className="btn btn-secondary text-sm" onClick={() => setWall(value => !value)}>{wall ? <X className="mr-2 h-4 w-4" /> : <Maximize2 className="mr-2 h-4 w-4" />}{wall ? t("Exit wall") : t("Wall mode")}</button>
      </div>
    </header>
    <div className="flex flex-wrap items-end gap-3">
      <label className="flex flex-col gap-1 text-xs text-surface-400">{t("Show")}<select className={field} value={filter} onChange={event => setFilter(event.target.value as typeof filter)}><option value="all">{t("All cameras")}</option><option value="printing">{t("Printing")}</option><option value="favorites">{t("Favorites")}</option></select></label>
      <label className="flex flex-col gap-1 text-xs text-surface-400">{t("Printer")}<select className={field} value={printerFilter} onChange={event => setPrinterFilter(event.target.value)}><option value="all">{t("All printers")}</option>{printers.map(printer => <option key={printer.id} value={printer.id}>{printer.name}</option>)}</select></label>
      <label className="flex flex-col gap-1 text-xs text-surface-400">{t("Card size")}<select className={field} value={preferences.size} onChange={event => setPreferences(previous => ({ ...previous, size: event.target.value as CameraWallSize }))}>{(['auto', 'small', 'medium', 'large'] as const).map(size => <option key={size} value={size}>{t(size[0].toUpperCase() + size.slice(1))}</option>)}</select></label>
      <label className="flex flex-col gap-1 text-xs text-surface-400">{t("Layout")}<select className={field} value={preferences.layout} onChange={event => setPreferences(previous => ({ ...previous, layout: event.target.value as CameraWallLayout }))}><option value="grid">{t("Grid")}</option><option value="focus">{t("Focus")}</option></select></label>
    </div>
    <details className="rounded-lg border border-surface-800 p-3 text-sm text-surface-400">
      <summary className="cursor-pointer">{t("Choose cameras ({{selection}})", { selection: preferences.selected === null ? t('All cameras') : preferences.selected.length })}</summary>
      <div className="mt-3 flex gap-3"><button type="button" className="text-accent-300 underline" onClick={() => setPreferences(previous => ({ ...previous, selected: null }))}>{t("Select all (including new)")}</button><button type="button" className="text-accent-300 underline" onClick={() => setPreferences(previous => ({ ...previous, selected: [] }))}>{t("Clear selection")}</button></div>
      <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">{cameras.map(camera => <label key={camera.id} className="flex items-center gap-2"><input type="checkbox" checked={preferences.selected === null || preferences.selected.includes(camera.id)} onChange={event => setPreferences(previous => {
        const current = previous.selected ?? cameras.map(item => item.id)
        return { ...previous, selected: event.target.checked ? [...current, camera.id] : current.filter(id => id !== camera.id) }
      })} />{camera.name} <span className="text-xs text-surface-500">{printerById.get(camera.printer_id || '')?.name}</span></label>)}</div>
    </details>
    {error && <div role="alert" className="rounded-lg border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200">{t("Failed to load camera list.")} {error instanceof Error ? error.message : t("Try refreshing the list.")}</div>}
    {isLoading && <p role="status" className="p-8 text-center text-surface-400">{t("Loading cameras…")}</p>}
    {!isLoading && !visibleIds.length && <div className="rounded-xl border border-dashed border-surface-700 p-8 text-center text-surface-400">{cameras.length ? t("No cameras match this selection. Adjust filters or choose more cameras.") : t("No cameras found. Configure cameras on your printers or add them in printer settings.")}</div>}
    <div className="grid items-start gap-4" style={{ gridTemplateColumns: `repeat(auto-fit, minmax(min(100%, ${minimumWidth[preferences.size]}px), 1fr))` }}>
      {visibleIds.map((id, index) => {
        const camera = byId.get(id)!
        return <div key={id} style={focusedId === id ? { gridColumn: '1 / -1', gridRow: 1 } : undefined}>
          <CameraWallCard failed={failures[id] === camera.url} onFailure={() => setFailures(previous => ({ ...previous, [id]: camera.url }))} onRetry={() => setFailures(previous => { const next = { ...previous }; delete next[id]; return next })} camera={camera} printer={printerById.get(camera.printer_id || '')} state={printerStates[camera.printer_id || '']} favorite={preferences.favorites.includes(id)} paused={paused.includes(id)} allPaused={allPaused} focused={focusedId === id} canMoveUp={index > 0} canMoveDown={index < visibleIds.length - 1}
            onFavorite={() => setPreferences(previous => ({ ...previous, favorites: previous.favorites.includes(id) ? previous.favorites.filter(item => item !== id) : [...previous.favorites, id] }))}
            onPause={() => setPaused(previous => previous.includes(id) ? previous.filter(item => item !== id) : [...previous, id])}
            onFocus={() => { setFocusId(id); setPreferences(previous => ({ ...previous, layout: focusedId === id ? 'grid' : 'focus' })) }}
            onMove={direction => move(id, direction)} />
        </div>
      })}
    </div>
    <p className="text-xs text-surface-500">{t("Preferences are saved in this browser when storage is available. Hidden and paused previews disconnect. Resuming all preserves individual pauses. Refresh list does not retry failed previews.")}</p>
  </div>

  return wall ? createPortal(<dialog ref={wallDialog} aria-labelledby="camera-wall-title" onCancel={() => setWall(false)} onClose={() => setWall(false)} className="fixed inset-0 m-0 h-[100dvh] max-h-none w-screen max-w-none overflow-y-auto border-0 bg-surface-950 p-0 text-surface-100 backdrop:bg-surface-950">{content}</dialog>, document.body) : content
}
