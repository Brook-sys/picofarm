import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { slicerApi } from '../api/client'
import { createRequestGate, slicePreviewKey } from '../lib/gcodeLibraryView'
import { slicerSubmissionKey } from '../lib/slicerJobSubmission'
import { slicerFormError, slicerFormPayload, type SlicerForm } from '../lib/slicerForm'
import type { GCodeLibraryFile, STLLibraryFile } from '../types'
import SlicerToolpathPreview from './SlicerToolpathPreview'

export interface SliceSTLModalProps {
  file: STLLibraryFile
  onClose: () => void
  /** Called after a job is accepted, not when slicing finishes. A file is supplied only by an already-succeeded idempotent replay. */
  onDone: (createdFile?: GCodeLibraryFile) => void
}

export default function SliceSTLModal({ file, onClose, onDone }: SliceSTLModalProps) {
  const { t } = useTranslation('library')
  const [profiles, setProfiles] = useState<Record<'printers' | 'presets' | 'filaments', Array<Record<string, unknown>>>>({ printers: [], presets: [], filaments: [] })
  const [form, setForm] = useState<SlicerForm>({ printer: '', preset: '', filament: '', quantity: 1, rotate: '', rotate_x: '', rotate_y: '', arrange: true, orient: true, enable_support: false, brim_type: false, print_sequence_by_object: false, display_name: `${file.display_name || file.file_name}.gcode` })
  const [busy, setBusy] = useState('')
  const [message, setMessage] = useState('')
  const [previewResult, setPreviewResult] = useState<Awaited<ReturnType<typeof slicerApi.preview>> | null>(null)
  const formError = slicerFormError(form)
  const canSubmit = !busy && !!form.printer && !!form.preset && !!form.filament && !formError
  const [previewKey, setPreviewKey] = useState('')
  const requestGate = useRef(createRequestGate())
  const dialogRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    dialogRef.current?.focus()
    return () => previous?.focus()
  }, [])
  const currentPreviewKey = slicePreviewKey(file.id, form)
  const previewIsStale = !!previewResult && previewKey !== currentPreviewKey

  const closeModal = () => {
    requestGate.current.invalidate()
    onClose()
  }

  useEffect(() => {
    let ignore = false
    const gate = requestGate.current
    Promise.all([slicerApi.getConfig(), slicerApi.profiles('printers'), slicerApi.profiles('presets'), slicerApi.profiles('filaments')]).then(([config, printers, presets, filaments]) => {
      if (ignore) return
      setProfiles({ printers, presets, filaments })
      const defaults = config.default_profiles || {}
      setForm(current => ({ ...current, printer: defaults.printers || String(printers.find(p => p.default)?.name || printers[0]?.name || ''), preset: defaults.presets || String(presets.find(p => p.default)?.name || presets[0]?.name || ''), filament: defaults.filaments || String(filaments.find(p => p.default)?.name || filaments[0]?.name || '') }))
    }).catch(err => { if (!ignore) setMessage(err instanceof Error ? err.message : t("Failed to load slicer profiles")) })
    return () => { ignore = true; gate.invalidate() }
  }, [t])

  const generatePreview = async () => {
    if (!canSubmit) return
    const request = requestGate.current.begin()
    const requestedKey = currentPreviewKey
    setBusy('preview')
    setMessage(t("Generating preview. Large models can take a while..."))
    setPreviewResult(null)
    try {
      const result = await slicerApi.preview(slicerFormPayload(file.id, form))
      if (!requestGate.current.isCurrent(request)) return
      setPreviewResult(result)
      setPreviewKey(requestedKey)
      setMessage(t("Preview generated for the submitted parameters."))
    } catch (err) {
      if (requestGate.current.isCurrent(request)) setMessage(err instanceof Error ? err.message : t("Failed to generate preview"))
    } finally {
      if (requestGate.current.isCurrent(request)) setBusy('')
    }
  }

  const slice = async () => {
    if (!canSubmit) return
    const request = requestGate.current.begin()
    setBusy('slice')
    setMessage(t("Submitting a background job. If it fails, retry without changing parameters to reuse the same key."))
    try {
      const payload = slicerFormPayload(file.id, form)
      const job = await slicerApi.submitJob(payload, slicerSubmissionKey(payload))
      if (!requestGate.current.isCurrent(request)) return
      setMessage(t("Job registered. Follow it in the library panel."))
      onDone(job.result?.gcode as GCodeLibraryFile | undefined)
    } catch (err) {
      if (requestGate.current.isCurrent(request)) setMessage(err instanceof Error ? err.message : t("Failed to slice STL"))
    } finally {
      if (requestGate.current.isCurrent(request)) setBusy('')
    }
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
      <div ref={dialogRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="slice-stl-title" onKeyDown={event => {
        if (event.key === 'Escape') { event.stopPropagation(); closeModal() }
        if (event.key === 'Tab') {
          const elements = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), summary')).filter(element => element.getClientRects().length > 0)
          const first = elements[0], last = elements[elements.length - 1]
          if (event.shiftKey && (document.activeElement === first || document.activeElement === event.currentTarget)) { event.preventDefault(); last?.focus() }
          else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
        }
      }} className="card p-6 w-full max-w-5xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4"><div><h2 id="slice-stl-title" className="text-xl font-semibold text-surface-100">{t("Slice STL with Orca")}</h2><p className="text-sm text-surface-500 truncate">{file.display_name}</p></div><button type="button" onClick={closeModal} className="btn btn-ghost text-sm">{t("Close")}</button></div>
        {message && <div className="mb-4 rounded-lg border border-surface-800 bg-surface-900/70 p-3 text-sm text-surface-300">{message}</div>}
        {previewIsStale && <div role="status" className="mb-4 rounded-lg border border-amber-500/40 p-3 text-sm text-amber-300">{t("Preview out of date — parameters changed. The image and estimates below are not valid for the current configuration. Generate another preview.")}</div>}
        {busy && <p className="mb-3 text-xs text-surface-400">{t("Closing ignores the response in this window but does not cancel server work.")}</p>}
        <div className="grid min-w-0 grid-cols-1 gap-5 md:grid-cols-[minmax(0,1fr)_340px]">
          <div className="space-y-4">
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("Printer profile")}</span><select className="input" value={form.printer} onChange={e => setForm({ ...form, printer: e.target.value })}>{profiles.printers.map(p => <option key={String(p.name)} value={String(p.name)}>{String(p.name)}</option>)}</select></label>
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("Preset profile")}</span><select className="input" value={form.preset} onChange={e => setForm({ ...form, preset: e.target.value })}>{profiles.presets.map(p => <option key={String(p.name)} value={String(p.name)}>{String(p.name)}</option>)}</select></label>
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("Filament profile")}</span><select className="input" value={form.filament} onChange={e => setForm({ ...form, filament: e.target.value })}>{profiles.filaments.map(p => <option key={String(p.name)} value={String(p.name)}>{String(p.name)}</option>)}</select></label>
            <section className="rounded-xl border border-surface-800 bg-surface-900/60 p-4 space-y-2">
              <label className="flex items-center justify-between gap-4"><span className="text-sm font-medium text-surface-200">{t("Pieces on one plate")}</span><input type="number" min={1} max={512} step={1} className="input w-24" value={Number.isNaN(form.quantity) ? '' : form.quantity} disabled={!!busy} onChange={e => setForm({ ...form, quantity: e.target.valueAsNumber, arrange: e.target.valueAsNumber > 1 ? true : form.arrange })} /></label>
              <p className="text-xs text-surface-400">{t("Includes the original. Orca must fit every piece on plate 1 or return an error; quantity is never reduced.")}</p>
              {form.quantity > 1 && <p className="text-xs text-accent-300">{t("Auto-arrange is required for multiple pieces.")}</p>}
            </section>
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("G-code name")}</span><input className="input" value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })} /></label>
            <div className="rounded-xl border border-surface-800 bg-surface-900/60 p-4"><div className="text-xs font-medium text-surface-400 mb-3">{t("Slicing options")}</div><div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm"><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.arrange} disabled={!!busy || form.quantity > 1} onChange={e => setForm({ ...form, arrange: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>{t("Auto-arrange")}</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.orient} disabled={!!busy || [form.rotate, form.rotate_x, form.rotate_y].some(value => value.trim() !== '')} onChange={e => setForm({ ...form, orient: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>{t("Auto-orient")}</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.enable_support} onChange={e => setForm({ ...form, enable_support: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>{t("Support")}</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.brim_type} onChange={e => setForm({ ...form, brim_type: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>{t("Brim")}</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.print_sequence_by_object} onChange={e => setForm({ ...form, print_sequence_by_object: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>{t("Print by object")}</span></label>{form.print_sequence_by_object && <div className="col-span-2 mt-1 text-xs text-amber-400">{t("Warning: May cause a collision")}</div>}</div></div>
            <details className="rounded-xl border border-surface-800 p-4">
              <summary className="cursor-pointer text-sm font-medium text-surface-300">{t("Advanced positioning")}</summary>
              <div className="mt-3 space-y-3">
                <p className="text-xs text-surface-400">{t("Leave angles empty for automatic orientation. Entering an angle, including zero, turns auto-orient off.")}</p>
                <div className="grid grid-cols-3 gap-3">{(['rotate_x', 'rotate_y', 'rotate'] as const).map((axis, index) => <label key={axis} className="text-xs text-surface-400">{['X (°)', 'Y (°)', 'Z (°)'][index]}<input className="input mt-1" type="number" step="any" value={form[axis]} disabled={!!busy} placeholder="—" onChange={e => setForm({ ...form, [axis]: e.target.value, orient: e.target.value.trim() !== '' ? false : form.orient })} /></label>)}</div>
                <p className="text-xs text-surface-500">{t("Orca controls the final arrangement. Custom spacing is not supported.")}</p>
              </div>
            </details>
            {formError && <p role="alert" className="text-sm text-amber-300">{formError === 'quantity' ? t("Enter a whole number of pieces between 1 and 512.") : formError === 'rotation' ? t("Rotation angles must be finite numbers.") : formError === 'arrange' ? t("Auto-arrange is required for multiple pieces.") : t("Clear explicit angles before enabling auto-orient.")}</p>}
            <div className="flex flex-wrap gap-2"><button className="btn btn-secondary" disabled={!canSubmit} onClick={generatePreview} type="button">{busy === 'preview' ? <><Loader2 className="h-4 w-4 mr-2 animate-spin" />{t("Generating preview...")}</> : t("Generate preview")}</button><button className="btn btn-primary" disabled={!canSubmit} onClick={slice} type="button">{busy === 'slice' ? <><Loader2 className="h-4 w-4 mr-2 animate-spin" />{t("Generating G-code...")}</> : t("Generate in background")}</button></div>
            <details className="text-xs text-surface-400"><summary className="cursor-pointer">{t("Background generation and retries")}</summary><p className="mt-2">{t("You can navigate after submitting. Repeating the same parameters looks up the same job, even after a network failure; to rerun it, explicitly remove the finished history entry. Changing parameters may create another job. After reloading the page, check the panel before resubmitting: the local key does not survive a reload.")}</p></details>
          </div>
          <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-3"><div className="mb-2 text-sm font-medium text-surface-200">{t("Preview")}</div>{busy === 'preview' ? <SlicerLoadingState title={t("Generating preview")} description={t("Preparing the model and rendering the preview. Large files may take a few minutes.")} /> : busy === 'slice' ? <SlicerLoadingState title={t("Generating G-code")} description={t("Registering the job in PicoFarm. Closing this window does not cancel an accepted job; follow it in the library panel.")} /> : previewResult ? <div className="space-y-3">{previewResult.toolpaths ? <SlicerToolpathPreview toolpaths={previewResult.toolpaths} /> : <p className="rounded border border-amber-500/30 p-3 text-xs text-amber-300">{t("Toolpaths are unavailable. The thumbnail does not show final placement or supports.")}</p>}{previewResult.thumbnail && <details className="text-xs text-surface-400"><summary className="cursor-pointer">{t("Source model thumbnail")}</summary><img src={`data:image/png;base64,${previewResult.thumbnail}`} alt={t("Source model thumbnail")} className="mt-2 max-h-40 rounded border border-surface-800" /></details>}<p className="text-sm font-medium text-surface-200">{t("Verified pieces: {{quantity}} · Plate 1", { quantity: previewResult.quantity ?? '—' })}</p><div className="text-xs space-y-1 text-surface-300"><div>{t("Time:")}{' '}{Math.floor(previewResult.printTime / 3600)}{t("h")}{Math.floor((previewResult.printTime % 3600) / 60)}{t("m")}</div><div>{t("Filament:")}{' '}{previewResult.filamentUsedG.toFixed(1)} {t("g /")}{previewResult.filamentUsedMm.toFixed(0)} {t("mm")}</div>{previewResult.usesSupport && <div className="text-emerald-400">{t("Uses support")}</div>}{form.print_sequence_by_object && <div className="text-amber-400">{t("Warning: May cause a collision")}</div>}</div></div> : <div className="flex h-64 items-center justify-center rounded-lg border border-dashed border-surface-700 bg-surface-950/40 text-sm text-surface-600">{t("Click Generate preview")}</div>}</div>
        </div>
      </div>
    </div>
  )
}

function SlicerLoadingState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-64 flex-col items-center justify-center rounded-lg border border-accent-500/30 bg-accent-500/10 px-6 text-center">
      <Loader2 className="mb-4 h-10 w-10 animate-spin text-accent-300" />
      <div className="text-base font-semibold text-surface-100">{title}</div>
      <p className="mt-2 max-w-xs text-sm leading-5 text-surface-400">{description}</p>
      <div className="mt-5 h-1.5 w-48 overflow-hidden rounded-full bg-surface-800">
        <div className="h-full w-1/2 animate-pulse rounded-full bg-accent-400" />
      </div>
    </div>
  )
}
