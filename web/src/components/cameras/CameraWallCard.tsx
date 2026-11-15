import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router-dom'
import { ArrowDown, ArrowUp, Maximize2, Minimize2, Pause, Play, Star, X } from 'lucide-react'
import { cn, formatRelativeTime, getStatusBadge } from '../../lib/utils'
import type { Camera, Printer, PrinterState } from '../../types'

const control = 'rounded-lg border border-surface-700 p-2 text-surface-300 hover:bg-surface-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent-400 disabled:opacity-30'

/** Removing src before unmount also terminates browser-managed MJPEG requests. */
function CameraImage({ url, name, snapshot, onError }: { url: string; name: string; snapshot: boolean; onError: () => void }) {
  const [loaded, setLoaded] = useState(false)
  const ref = useRef<HTMLImageElement>(null)
  useEffect(() => {
    const image = ref.current
    if (!image) return
    image.src = url
    return () => { image.removeAttribute('src') }
  }, [url])
  return <><img ref={ref} alt={`${name} camera preview`} onLoad={() => setLoaded(true)} onError={onError} className="absolute inset-0 h-full w-full object-contain" /><div className="absolute left-2 top-2 rounded bg-black/80 px-2 py-1 text-xs text-surface-200" role="status">{loaded ? snapshot ? 'Snapshot · not live' : 'Stream connected' : 'Connecting…'}</div></>
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
  const [enlarged, setEnlarged] = useState(false)
  const dialogRef = useRef<HTMLDialogElement>(null)
  const expandRef = useRef<HTMLButtonElement>(null)
  const titleId = useId()
  const supported = camera.type === 'mjpeg' || camera.type === 'snapshot'
  const status = state?.status || printer?.status || 'unassigned'
  const active = camera.enabled && supported && !paused && !allPaused && !failed && !!camera.url
  const previewStatus = !camera.enabled ? 'Disabled' : paused || allPaused ? 'Paused' : !supported ? 'External preview only' : !camera.url ? 'No stream URL' : failed ? 'Preview failed' : 'Connecting…'

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
            <span className="truncate">{printer?.name || 'Unassigned camera'}</span>
            <span className={cn('badge', getStatusBadge(status))}>{status}</span>
            {state && status === 'printing' && <span>{Math.round(Math.min(100, Math.max(0, Number.isFinite(state.progress) ? state.progress : 0)))}%</span>}
          </div>
        </div>
        {enlarged && <button type="button" className={control} onClick={() => setEnlarged(false)} aria-label="Close enlarged camera"><X className="h-4 w-4" /></button>}
      </header>
      <div className={cn('relative bg-black', enlarged ? 'h-[65dvh]' : 'aspect-video')}>
        {active && <CameraImage key={camera.url} url={camera.url} name={camera.name} snapshot={camera.type === 'snapshot'} onError={onFailure} />}
        {!active && <div className="absolute left-2 top-2 rounded bg-black/80 px-2 py-1 text-xs text-surface-200" role="status">{previewStatus}</div>}
        {camera.type === 'snapshot' && <span className="absolute bottom-2 left-2 rounded bg-black/80 px-2 py-1 text-xs text-surface-300">Snapshot · not live</span>}
        {!active && <div className="flex h-full flex-col items-center justify-center gap-3 p-4 text-center text-sm text-surface-400">
          <span>{previewStatus}</span>
          {failed && camera.enabled && !paused && !allPaused && <button type="button" className="btn btn-secondary text-xs" onClick={onRetry}>Retry preview</button>}
        </div>}
      </div>
      <div className="flex flex-wrap items-center gap-2 p-3">
        <button type="button" className={control} aria-label={`${favorite ? 'Unfavorite' : 'Favorite'} ${camera.name}`} aria-pressed={favorite} onClick={onFavorite}><Star className={cn('h-4 w-4', favorite && 'fill-amber-400 text-amber-400')} /></button>
        <button type="button" className={control} aria-label={`${paused ? 'Resume' : 'Pause'} ${camera.name}`} aria-pressed={paused} disabled={!camera.enabled || !supported || allPaused} onClick={onPause}>{paused ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}</button>
        <button ref={enlarged ? undefined : expandRef} type="button" className={control} aria-label={`${enlarged ? 'Reduce' : 'Enlarge'} ${camera.name}`} onClick={() => setEnlarged(value => !value)}>{enlarged ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}</button>
        <button type="button" className={cn(control, 'text-xs', focused && 'border-accent-500 text-accent-300')} onClick={onFocus} aria-pressed={focused}>Focus</button>
        <div className="ml-auto flex gap-1">
          <button type="button" className={control} disabled={!canMoveUp} aria-label={`Move ${camera.name} earlier`} onClick={() => onMove(-1)}><ArrowUp className="h-4 w-4" /></button>
          <button type="button" className={control} disabled={!canMoveDown} aria-label={`Move ${camera.name} later`} onClick={() => onMove(1)}><ArrowDown className="h-4 w-4" /></button>
        </div>
      </div>
      <details className="border-t border-surface-800 p-3 text-xs text-surface-400">
        <summary className="cursor-pointer">Diagnostics</summary>
        <dl className="mt-3 space-y-2 break-all"><div><dt className="text-surface-500">Type</dt><dd>{camera.type}</dd></div><div><dt className="text-surface-500">Stream URL</dt><dd>{camera.url || 'Not configured'}</dd></div><div><dt className="text-surface-500">Updated</dt><dd>{formatRelativeTime(camera.updated_at)}</dd></div></dl>
        <div className="mt-3 flex flex-wrap gap-4">
          {camera.url && <a className="text-accent-300 underline" href={camera.url} target="_blank" rel="noreferrer">Open source externally</a>}
          {printer && <Link className="text-accent-300 underline" to={`/printers/${printer.id}`}>Open printer</Link>}
        </div>
        <p className="mt-3">Browser-managed previews. MJPEG may keep reporting connecting until the browser emits a load event. No automatic retries; use Retry preview after a failure. Snapshots are not refreshed automatically.</p>
      </details>
    </>
  )
  return <article className="overflow-hidden rounded-xl border border-surface-800 bg-surface-900" aria-label={`${camera.name} camera`}>
    {enlarged ? <>
      <div className="flex aspect-video items-center justify-center text-sm text-surface-400">{camera.name} is enlarged</div>
      {createPortal(<dialog ref={dialogRef} aria-labelledby={titleId} onCancel={event => { event.stopPropagation(); setEnlarged(false) }} onClose={() => setEnlarged(false)} className="m-auto max-h-[95dvh] w-[min(96vw,1200px)] overflow-y-auto rounded-xl border border-surface-700 bg-surface-900 p-0 text-surface-100 backdrop:bg-black/80">{content}</dialog>, document.body)}
    </> : content}
  </article>
}
