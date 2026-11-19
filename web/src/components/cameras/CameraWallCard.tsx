import { useTranslation } from 'react-i18next'
import { externalLinkClick, resourceURL } from '../../lib/platform'
import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router-dom'
import { ArrowDown, ArrowUp, Maximize2, Minimize2, Pause, Play, Star, X } from 'lucide-react'
import { cn, formatRelativeTime, getStatusBadge } from '../../lib/utils'
import type { Camera, Printer, PrinterState } from '../../types'

const JOB_STATUS_LABELS = {"queued":"Queued","assigned":"Assigned","uploaded":"Uploaded","started":"Started","progress":"Progress","paused":"Paused","resumed":"Resumed","completed":"Completed","failed":"Failed","cancelled":"Cancelled","retried":"Retried","idle":"Idle","printing":"Printing","offline":"Offline","error":"Error","unassigned":"Unassigned","available":"Available","empty":"Empty","in_use":"In use","running":"Running","succeeded":"Succeeded","interrupted":"Interrupted","user":"User","system":"System","printer":"Printer","api":"Api"}

const control = 'rounded-lg border border-surface-700 p-2 text-surface-300 hover:bg-surface-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent-400 disabled:opacity-30'

/** Removing src before unmount also terminates browser-managed MJPEG requests. */
function CameraImage({ url, name, snapshot, onError }: { url: string; name: string; snapshot: boolean; onError: () => void }) {
  const { t } = useTranslation('components')
  const [loaded, setLoaded] = useState(false)
  const ref = useRef<HTMLImageElement>(null)
  useEffect(() => {
    const image = ref.current
    if (!image) return
    image.src = resourceURL(url)
    return () => { image.removeAttribute('src') }
  }, [url])
  return <><img ref={ref} alt={t("{{name}} camera preview", { name: name })} onLoad={() => setLoaded(true)} onError={onError} className="absolute inset-0 h-full w-full object-contain" /><div className="absolute left-2 top-2 rounded bg-black/80 px-2 py-1 text-xs text-surface-200" role="status">{loaded ? snapshot ? t("Snapshot · not live") : t("Stream connected") : t("Connecting…")}</div></>
}

interface Props {
  camera: Camera
  printer?: Printer
  state?: PrinterState
  favorite: boolean
  paused: boolean
  allPaused: boolean
  failed: boolean
  onFailure: () => void
  onRetry: () => void
  focused: boolean
  canMoveUp: boolean
  canMoveDown: boolean
  onFavorite: () => void
  onPause: () => void
  onFocus: () => void
  onMove: (direction: -1 | 1) => void
}

export function CameraWallCard({ camera, printer, state, favorite, paused, allPaused, failed, onFailure, onRetry, focused, canMoveUp, canMoveDown, onFavorite, onPause, onFocus, onMove }: Props) {
  const { t } = useTranslation('components')
  const [enlarged, setEnlarged] = useState(false)
  const dialogRef = useRef<HTMLDialogElement>(null)
  const expandRef = useRef<HTMLButtonElement>(null)
  const titleId = useId()
  const supported = camera.type === 'mjpeg' || camera.type === 'snapshot'
  const status = state?.status || printer?.status || 'unassigned'
  const active = camera.enabled && supported && !paused && !allPaused && !failed && !!camera.url
  const previewStatus = !camera.enabled ? t("Disabled") : paused || allPaused ? t("Paused") : !supported ? t("External preview only") : !camera.url ? t("No stream URL") : failed ? t("Preview failed") : t("Connecting…")

  const wasEnlarged = useRef(false)
  useEffect(() => {
    if (!enlarged) {
      if (wasEnlarged.current) expandRef.current?.focus()
      wasEnlarged.current = false
      return
    }
    wasEnlarged.current = true
    const dialog = dialogRef.current
    dialog?.showModal()
    return () => { dialog?.close() }
  }, [enlarged])

  const content = (
    <>
      <header className="flex items-start justify-between gap-2 p-3">
        <div className="min-w-0">
          <h2 id={titleId} className="truncate font-semibold text-surface-100" title={camera.name}>{camera.name}</h2>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-surface-400">
            <span className="truncate">{printer?.name || t("Unassigned camera")}</span>
            <span className={cn('badge', getStatusBadge(status))}>{t(JOB_STATUS_LABELS[status] || status)}</span>
            {state && status === 'printing' && <span>{Math.round(Math.min(100, Math.max(0, Number.isFinite(state.progress) ? state.progress : 0)))}%</span>}
          </div>
        </div>
        {enlarged && <button type="button" className={control} onClick={() => setEnlarged(false)} aria-label={t("Close enlarged camera")}><X className="h-4 w-4" /></button>}
      </header>
      <div className={cn('relative bg-black', enlarged ? 'h-[65dvh]' : 'aspect-video')}>
        {active && <CameraImage key={camera.url} url={camera.url} name={camera.name} snapshot={camera.type === 'snapshot'} onError={onFailure} />}
        {!active && <div className="absolute left-2 top-2 rounded bg-black/80 px-2 py-1 text-xs text-surface-200" role="status">{previewStatus}</div>}
        {camera.type === 'snapshot' && <span className="absolute bottom-2 left-2 rounded bg-black/80 px-2 py-1 text-xs text-surface-300">{t("Snapshot · not live")}</span>}
        {!active && <div className="flex h-full flex-col items-center justify-center gap-3 p-4 text-center text-sm text-surface-400">
          <span>{previewStatus}</span>
          {failed && camera.enabled && !paused && !allPaused && <button type="button" className="btn btn-secondary text-xs" onClick={onRetry}>{t("Retry preview")}</button>}
        </div>}
      </div>
      <div className="flex flex-wrap items-center gap-2 p-3">
        <button type="button" className={control} aria-label={t("{{name}} {{value1}}", { name: favorite ? t("Unfavorite") : t("Favorite"), value1: camera.name })} aria-pressed={favorite} onClick={onFavorite}><Star className={cn('h-4 w-4', favorite && 'fill-amber-400 text-amber-400')} /></button>
        <button type="button" className={control} aria-label={t("{{name}} {{value1}}", { name: paused ? t("Resume") : t("Pause"), value1: camera.name })} aria-pressed={paused} disabled={!camera.enabled || !supported || allPaused} onClick={onPause}>{paused ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}</button>
        <button ref={enlarged ? undefined : expandRef} type="button" className={control} aria-label={t("{{name}} {{value1}}", { name: enlarged ? t("Reduce") : t("Enlarge"), value1: camera.name })} onClick={() => setEnlarged(value => !value)}>{enlarged ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}</button>
        <button type="button" className={cn(control, 'text-xs', focused && 'border-accent-500 text-accent-300')} onClick={onFocus} aria-pressed={focused}>{t("Focus")}</button>
        <div className="ml-auto flex gap-1">
          <button type="button" className={control} disabled={!canMoveUp} aria-label={t("Move {{name}} earlier", { name: camera.name })} onClick={() => onMove(-1)}><ArrowUp className="h-4 w-4" /></button>
          <button type="button" className={control} disabled={!canMoveDown} aria-label={t("Move {{name}} later", { name: camera.name })} onClick={() => onMove(1)}><ArrowDown className="h-4 w-4" /></button>
        </div>
      </div>
      <details className="border-t border-surface-800 p-3 text-xs text-surface-400">
        <summary className="cursor-pointer">{t("Diagnostics")}</summary>
        <dl className="mt-3 space-y-2 break-all"><div><dt className="text-surface-500">{t("Type")}</dt><dd>{camera.type}</dd></div><div><dt className="text-surface-500">{t("Stream URL")}</dt><dd>{camera.url || t("Not configured")}</dd></div><div><dt className="text-surface-500">{t("Updated")}</dt><dd>{formatRelativeTime(camera.updated_at)}</dd></div></dl>
        <div className="mt-3 flex flex-wrap gap-4">
          {camera.url && <a className="text-accent-300 underline" href={camera.url} onClick={externalLinkClick} target="_blank" rel="noreferrer">{t("Open source externally")}</a>}
          {printer && <Link className="text-accent-300 underline" to={`/printers/${printer.id}`}>{t("Open printer")}</Link>}
        </div>
        <p className="mt-3">{t("Browser-managed previews. MJPEG may keep reporting connecting until the browser emits a load event. No automatic retries; use Retry preview after a failure. Snapshots are not refreshed automatically.")}</p>
      </details>
    </>
  )
  return <article className="overflow-hidden rounded-xl border border-surface-800 bg-surface-900" aria-label={t("{{name}} camera", { name: camera.name })}>
    {enlarged ? <>
      <div className="flex aspect-video items-center justify-center text-sm text-surface-400">{camera.name} {t("is enlarged")}</div>
      {createPortal(<dialog ref={dialogRef} aria-labelledby={titleId} onCancel={event => { event.stopPropagation(); setEnlarged(false) }} onClose={() => setEnlarged(false)} className="m-auto max-h-[95dvh] w-[min(96vw,1200px)] overflow-y-auto rounded-xl border border-surface-700 bg-surface-900 p-0 text-surface-100 backdrop:bg-black/80">{content}</dialog>, document.body)}
    </> : content}
  </article>
}
