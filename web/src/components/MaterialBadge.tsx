import { materialBadgeStyle, normalizeMaterialTypeKey, type MaterialColorMap } from '../lib/materialColors'

/**
 * Filament type label used on the Files screen and in Settings.
 *
 * The color comes from the `filament_type_colors` setting and falls back to
 * the built-in palette in lib/materialColors, so it still renders on a fresh
 * install and for types nobody has configured.
 *
 * Inline style on purpose: Tailwind scans static source strings, so a class
 * built from the type name (bg-${type}-500) would never be generated.
 */
export function MaterialBadge({ material, colors }: { material?: string; colors: MaterialColorMap }) {
  const type = normalizeMaterialTypeKey(material ?? '')
  if (!type) {
    return (
      <span className="inline-flex items-center rounded-full border border-surface-700 bg-surface-800/60 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-surface-500">
        No material
      </span>
    )
  }
  return (
    <span
      title={`${type.toUpperCase()} filament`}
      className="inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide"
      style={materialBadgeStyle(material, colors)}
    >
      {type.toUpperCase()}
    </span>
  )
}