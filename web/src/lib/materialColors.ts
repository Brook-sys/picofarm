// Filament type colors — the label shown next to each G-code on the Files
// screen, one color per material type.
//
// IMPORTANT: keys must match what the backend stores in
// gcode_files.material_type. internal/service/queue.go normalizeMaterialType
// lowercases, cuts at '#', strips spaces/'-'/'_' and then maps substrings onto
// the canonical set (so "PLA-CF" and "PLA+ Beige" both arrive here as "pla").
//
// normalizeMaterialTypeKey below applies the string rules only. That is enough
// because the backend also runs its normalization on the keys of the
// `filament_type_colors` setting (see validateFilamentTypeColors in
// internal/service), so a stored key is already canonical ("pla", not "PLA-CF").
// The rules are repeated here to survive a setting written by hand or by an
// older build straight into the database.

/** Types that always appear as an editable row in Settings. */
export const CANONICAL_MATERIAL_TYPES = ['pla', 'petg', 'abs', 'asa', 'tpu'] as const

/** The settings key consulted by the Files screen and written by Settings. */
export const FILAMENT_TYPE_COLORS_SETTING_KEY = 'filament_type_colors'

/** Extra types offered in Settings for convenience. */
export const SUGGESTED_MATERIAL_TYPES = ['pc', 'pa', 'pva', 'hips', 'wood'] as const

/**
 * Built-in colors, used when nothing is configured for a type. They are also
 * the seed for the Settings editor. Neutral gray is the fallback for any type
 * that is neither here nor in the setting, which happens because
 * normalizeMaterialType passes unknown values through unchanged (e.g. "pc"
 * or a slicer-specific name).
 */
export const DEFAULT_MATERIAL_COLORS: Record<string, string> = {
  pla: '#22c55e', // green
  petg: '#3b82f6', // blue
  abs: '#f97316', // orange
  asa: '#a855f7', // purple
  tpu: '#ec4899', // pink
  pc: '#ef4444',
  pa: '#eab308',
  pva: '#14b8a6',
  hips: '#64748b',
  wood: '#a16207',
}

/** Used when a file carries no material type at all. */
export const UNKNOWN_MATERIAL_COLOR = '#64748b'

export type MaterialColorMap = Record<string, string>

export const HEX_COLOR = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

/** Same normalization the backend applies to material_type values. */
export function normalizeMaterialTypeKey(value: string): string {
  let v = value.trim().toLowerCase()
  const hash = v.indexOf('#')
  if (hash >= 0) v = v.slice(0, hash).trim()
  return v.replace(/[\s_-]/g, '')
}

/**
 * Parse the raw JSON stored in the `filament_type_colors` setting. Never
 * throws: the setting is written by hand through the API as often as it is
 * written by the UI, and a bad value must degrade to defaults instead of a
 * blank screen.
 */
export function parseMaterialColors(raw: string | undefined | null): MaterialColorMap {
  if (!raw || !raw.trim()) return {}
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return {}
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
  const colors: MaterialColorMap = {}
  for (const [key, color] of Object.entries(parsed as Record<string, unknown>)) {
    const type = normalizeMaterialTypeKey(key)
    const hex = typeof color === 'string' ? color.trim().toLowerCase() : ''
    if (type && HEX_COLOR.test(hex)) colors[type] = hex
  }
  return colors
}

/** Resolve the hex color for a material type: setting, then default, then neutral. */
export function resolveMaterialColor(material: string | undefined | null, colors: MaterialColorMap): string {
  const type = material ? normalizeMaterialTypeKey(material) : ''
  if (!type) return UNKNOWN_MATERIAL_COLOR
  return colors[type] || DEFAULT_MATERIAL_COLORS[type] || UNKNOWN_MATERIAL_COLOR
}

function hexToRgb(hex: string): [number, number, number] {
  let h = hex.replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const n = Number.parseInt(h, 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

/** Relative luminance (WCAG), used to decide whether the chip needs dark text. */
function luminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex).map(c => {
    const s = c / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

const CHIP_DARK_TEXT = '#0f172a'
const CHIP_LIGHT_TEXT = '#ffffff'

export type MaterialBadgeStyle = {
  backgroundColor: string
  borderColor: string
  color: string
}

/**
 * Style for the material chip. The chip is opaque so that the color reads as
 * an identifier at a glance (the reason the feature exists) rather than a
 * tint; the text color is picked from the background luminance, which is what
 * keeps low-luminance colors readable against the app's dark surfaces.
 */
export function materialBadgeStyle(material: string | undefined | null, colors: MaterialColorMap): MaterialBadgeStyle {
  const hex = resolveMaterialColor(material, colors)
  return {
    backgroundColor: hex,
    // Same-color border is invisible, so the edge is a faint white outline:
    // it keeps the chip's bounds readable when the chosen color sits close to
    // the card background behind it.
    borderColor: 'rgba(255, 255, 255, 0.18)',
    color: luminance(hex) >= 0.35 ? CHIP_DARK_TEXT : CHIP_LIGHT_TEXT,
  }
}