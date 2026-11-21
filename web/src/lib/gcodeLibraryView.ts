import type { GCodeLibraryFile, STLLibraryFile } from '../types'

export type LibrarySortMode = 'created_desc' | 'created_asc' | 'name_asc' | 'name_desc' | 'prints_desc' | 'time_asc' | 'grams_asc' | 'grams_desc'
export type LibraryItem = { type: 'stl'; file: STLLibraryFile } | { type: 'gcode'; file: GCodeLibraryFile }

// STL metric sorting uses only its explicitly default G-code, never a guessed variant.
export function sortLibraryItems(items: LibraryItem[], mode: LibrarySortMode): LibraryItem[] {
  const metricFile = (item: LibraryItem) => item.type === 'gcode' ? item.file : item.file.gcodes?.find(file => file.default_for_stl)
  const metric = (item: LibraryItem) => {
    const file = metricFile(item)
    const value = mode === 'prints_desc' ? file?.print_count : mode === 'time_asc' ? file?.estimated_seconds : file?.filament_grams
    return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : undefined
  }
  return [...items].sort((a, b) => {
    let result: number
    if (mode.startsWith('name_')) result = a.file.display_name.localeCompare(b.file.display_name) * (mode === 'name_desc' ? -1 : 1)
    else if (mode.startsWith('created_')) result = ((Date.parse(a.file.created_at) || 0) - (Date.parse(b.file.created_at) || 0)) * (mode === 'created_desc' ? -1 : 1)
    else {
      const av = metric(a), bv = metric(b)
      if (av === undefined || bv === undefined) result = av === bv ? 0 : av === undefined ? 1 : -1
      else result = (av - bv) * (mode === 'time_asc' || mode === 'grams_asc' ? 1 : -1)
    }
    return result || `${a.type}:${a.file.id}`.localeCompare(`${b.type}:${b.file.id}`)
  })
}

export function paginateLibrary<T>(items: T[], page: number, size: number) {
  const pages = Math.max(1, Math.ceil(items.length / size))
  const current = Math.max(1, Math.min(page, pages))
  return { current, pages, items: items.slice((current - 1) * size, current * size) }
}

// Invalidating on close/unmount prevents any pending response publishing state.
export function createRequestGate() {
  let generation = 0
  return {
    begin: () => ++generation,
    invalidate: () => { generation += 1 },
    isCurrent: (request: number) => request === generation,
  }
}

export function slicePreviewKey(stlId: string, form: { printer: string; preset: string; filament: string; arrange: boolean; orient: boolean; enable_support: boolean; brim_type: boolean; print_sequence_by_object: boolean; quantity?: number; rotate?: string; rotate_x?: string; rotate_y?: string }) {
  return JSON.stringify([stlId, form.printer, form.preset, form.filament, form.arrange, form.orient, form.enable_support, form.brim_type, form.print_sequence_by_object, form.quantity ?? 1, form.rotate ?? '', form.rotate_x ?? '', form.rotate_y ?? ''])
}
