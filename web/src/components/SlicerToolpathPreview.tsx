import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { sanitizeToolpaths, toolpathPaths } from '../lib/slicerToolpaths'
import type { ToolpathKind, ToolpathPreview } from '../lib/slicerToolpaths'

export type { ToolpathPreview, ToolpathLayer, ToolpathSegment } from '../lib/slicerToolpaths'

export interface SlicerToolpathPreviewProps {
  /** Missing in legacy slicer responses. Parent may instead render its legacy preview. */
  toolpaths?: ToolpathPreview | null
  className?: string
}

// The upstream v1 preview retains positive-extrusion XY moves, not travel moves.
const kinds: ToolpathKind[] = ['model', 'support', 'brim']
const colors: Record<ToolpathKind, string> = {
  model: '#38bdf8', support: '#c084fc', brim: '#fbbf24', travel: '#94a3b8',
}

export default function SlicerToolpathPreview({ toolpaths, className = '' }: SlicerToolpathPreviewProps) {
  const { t } = useTranslation('library')
  const id = useId()
  const preview = useMemo(() => sanitizeToolpaths(toolpaths), [toolpaths])
  const [selected, setSelected] = useState(0)
  const [visible, setVisible] = useState<Record<ToolpathKind, boolean>>({ model: true, support: true, brim: true, travel: false })
  const selectedIndex = Math.min(selected, Math.max(0, preview.layers.length - 1))
  const layer = preview.layers[selectedIndex]
  const paths = useMemo(() => toolpathPaths(layer, preview.bounds), [layer, preview.bounds])
  const layerText = layer
    ? t("Layer {{number}} · Z {{z}} mm", { defaultValue: 'Layer {{number}} · Z {{z}} mm', number: layer.index + 1, z: layer.z.toFixed(3) })
    : ''
  const hasVisible = kinds.some(kind => visible[kind] && paths[kind])

  return (
    <section className={`min-w-0 space-y-3 rounded-lg border border-slate-700 bg-slate-900/50 p-4 ${className}`} aria-labelledby={`${id}-heading`}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h4 id={`${id}-heading`} className="text-sm font-medium text-slate-100">
          {t("Toolpaths · top view", { defaultValue: 'Toolpaths · top view' })}
        </h4>
        {layer && <output htmlFor={`${id}-layer`} className="text-xs tabular-nums text-slate-300">{layerText}</output>}
      </div>
      {!toolpaths || !preview.supported ? (
        <p className="text-sm text-slate-400">{t("Toolpath preview unavailable for this response.", { defaultValue: 'Toolpath preview unavailable for this response.' })}</p>
      ) : <>
        <p className="text-xs text-surface-400">{t('Retained: {{layers}} layers · {{segments}} segments', { layers: preview.layers.length, segments: preview.layers.reduce((sum, item) => sum + item.segments.length, 0) })}</p>
        {preview.partial && <p role="status" className="text-xs text-amber-300">
          {t("Partial preview: some layers or segments are omitted, invalid, or truncated. Only retained geometry is shown.", { defaultValue: 'Partial preview: some layers or segments are omitted, invalid, or truncated. Only retained geometry is shown.' })}
        </p>}
        {layer && preview.bounds ? <>
          <div className="relative overflow-hidden rounded-md border border-slate-700 bg-slate-950">
            <svg viewBox="0 0 1000 1000" className="mx-auto block aspect-square max-h-96 w-full" role="img" aria-labelledby={`${id}-svg-title ${id}-svg-desc`}>
              <title id={`${id}-svg-title`}>{layerText}</title>
              <desc id={`${id}-svg-desc`}>{t("XY extrusion paths of the selected layer. Travel moves are not included. This is not a collision check.", { defaultValue: 'XY extrusion paths of the selected layer. Travel moves are not included. This is not a collision check.' })}</desc>
              {kinds.map(kind => visible[kind] && paths[kind] ? <path key={kind} d={paths[kind]} fill="none" stroke={colors[kind]} strokeWidth={kind === 'travel' ? 0.75 : 1.25} vectorEffect="non-scaling-stroke" strokeDasharray={kind === 'travel' ? '4 4' : undefined} /> : null)}
            </svg>
            {!hasVisible && <p className="absolute inset-0 flex items-center justify-center p-6 text-center text-sm text-slate-400">
              {t("No visible moves on this layer. Adjust the visibility controls or choose another layer.", { defaultValue: 'No visible moves on this layer. Adjust the visibility controls or choose another layer.' })}
            </p>}
          </div>
          <div className="space-y-1">
            <label htmlFor={`${id}-layer`} className="text-xs text-slate-300">{t("Select retained layer", { defaultValue: 'Select retained layer' })}</label>
            <input id={`${id}-layer`} type="range" min={0} max={preview.layers.length - 1} step={1} value={selectedIndex} disabled={preview.layers.length < 2} onChange={event => setSelected(Number(event.target.value))} aria-valuetext={layerText} className="block w-full accent-sky-400 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-sky-400" />
          </div>
        </> : <p className="text-sm text-slate-400">{t("No valid toolpath segments to display.", { defaultValue: 'No valid toolpath segments to display.' })}</p>}
        <details className="text-xs text-slate-300">
          <summary className="cursor-pointer py-1 focus-visible:outline-2 focus-visible:outline-sky-400">{t("Visibility & legend", { defaultValue: 'Visibility & legend' })}</summary>
          <div className="mt-2 flex flex-wrap gap-x-5 gap-y-2">
            {kinds.map(kind => <label key={kind} className="flex cursor-pointer items-center gap-2 py-1">
              <input type="checkbox" checked={visible[kind]} onChange={event => setVisible(previous => ({ ...previous, [kind]: event.target.checked }))} className="accent-sky-400" />
              <span aria-hidden="true" className="inline-block w-5 border-t-2" style={{ borderColor: colors[kind], borderStyle: kind === 'travel' ? 'dashed' : 'solid' }} />
              {({ model: t('Model / other extrusion'), support: t('Support'), brim: t('Brim'), travel: t('Travel / non-extruding') }[kind])}
            </label>)}
          </div>
        </details>
        {preview.warnings.length > 0 && <details className="text-xs text-amber-200">
          <summary className="cursor-pointer py-1">{t("Parser warnings", { defaultValue: 'Parser warnings' })}</summary>
          <ul className="mt-2 list-disc space-y-1 break-words pl-5">{preview.warnings.map((warning, index) => <li key={index}>{warning}</li>)}</ul>
        </details>}
      </>}
      <p className="text-xs text-slate-400">{t("Visualization only — not a collision or printability guarantee. Printer macros and physical machine behavior may not be represented.", { defaultValue: 'Visualization only — not a collision or printability guarantee. Printer macros and physical machine behavior may not be represented.' })}</p>
    </section>
  )
}
