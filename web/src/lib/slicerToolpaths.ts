// Wire schema from slicer-api/internal/slicer/toolpath.go (v1 legacy, v2 full).
export type ToolpathPoint = [number, number, number]
export interface ToolpathSegment {
  from: ToolpathPoint
  to: ToolpathPoint
  extrusion: number
  feedrateMmSec?: number
  widthMm?: number
  heightMm?: number
  /** Compatibility with alternate Orca producers; absent dimensions are estimated. */
  lineWidthMm?: number
  layerHeightMm?: number
  feature?: string
  object?: string
  tool: number
}
export interface ToolpathLayer { index: number; z: number; segments: ToolpathSegment[] }
export interface ToolpathPreview {
  version: number
  mode?: 'full' | 'legacy'
  geometryComplete?: boolean
  positionUnit: string
  extrusionUnit: string
  feedrateUnit: string
  coordinateSystem: string
  sourceComplete: boolean
  truncated: boolean
  sourceBytes: number
  sourceLines: number
  totalLayers: number
  totalSegments: number
  returnedLayers: number
  returnedSegments: number
  omittedSegments: number
  supportSegments: number
  brimSegments: number
  layerSemantics: string
  warnings: string[]
  limits: {
    sourceBytes: number; lineBytes: number; lines: number; layers: number; segments: number
    arcSegmentsPerMove: number; arcSegmentsTotal: number; arcChordToleranceMm: number
  }
  bounds?: { min: ToolpathPoint; max: ToolpathPoint }
  layers: ToolpathLayer[]
}
export type ToolpathKind = 'model' | 'support' | 'brim' | 'travel'
export const MAX_TOOLPATH_SEGMENTS = 20_000
const MAX_LAYERS = 512
// Aligned with slicer-api's full-mode retention budget.
export const MAX_COMPLETE_TOOLPATH_SEGMENTS = 250_000
const MAX_COMPLETE_LAYERS = 4096
const record = (value: unknown): Record<string, unknown> =>
  value !== null && typeof value === 'object' ? value as Record<string, unknown> : {}
const coordinate = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value) && Math.abs(value) <= 1e9
const point = (value: unknown): value is ToolpathPoint =>
  Array.isArray(value) && value.length === 3 && value.every(coordinate)

export function toolpathKind(segment: Pick<ToolpathSegment, 'extrusion' | 'feature'>): ToolpathKind {
  if (segment.extrusion <= 0) return 'travel'
  if (segment.feature === 'Brim') return 'brim'
  if (['Support', 'Support interface', 'Support transition'].includes(segment.feature ?? '')) return 'support'
  return 'model'
}

/** Bound inspection as well as output: even invalid entries consume the segment budget. */
export function sanitizeToolpaths(value: unknown, complete = false) {
  const raw = record(value)
  const segmentLimit = complete ? MAX_COMPLETE_TOOLPATH_SEGMENTS : MAX_TOOLPATH_SEGMENTS
  const layerLimit = complete ? MAX_COMPLETE_LAYERS : MAX_LAYERS
  const supported = (raw.version === 1 || (raw.version === 2 && raw.mode === 'full')) && raw.positionUnit === 'mm' && raw.coordinateSystem === 'gcode'
  const layers: ToolpathLayer[] = []
  let limited = false
  let inspected = 0
  let minX = Infinity, minY = Infinity, minZ = Infinity, maxX = -Infinity, maxY = -Infinity, maxZ = -Infinity
  if (supported && Array.isArray(raw.layers)) {
    limited = raw.layers.length > layerLimit
    for (let i = 0; i < Math.min(raw.layers.length, layerLimit); i++) {
      const layer = record(raw.layers[i])
      if (!coordinate(layer.z) || !Number.isSafeInteger(layer.index) || (layer.index as number) < 0 || !Array.isArray(layer.segments)) {
        limited = true
        continue
      }
      const segments: ToolpathSegment[] = []
      for (let j = 0; j < layer.segments.length; j++) {
        if (inspected >= segmentLimit) { limited = true; break }
        inspected++
        const segment = record(layer.segments[j])
        if (!point(segment.from) || !point(segment.to) || typeof segment.extrusion !== 'number' || !Number.isFinite(segment.extrusion)) {
          limited = true
          continue
        }
        const safe: ToolpathSegment = {
          from: [...segment.from], to: [...segment.to], extrusion: segment.extrusion,
          tool: Number.isSafeInteger(segment.tool) && (segment.tool as number) >= 0 ? segment.tool as number : 0,
          feature: typeof segment.feature === 'string' ? segment.feature.slice(0, 80) : undefined,
          object: typeof segment.object === 'string' ? segment.object.slice(0, 80) : undefined,
          feedrateMmSec: coordinate(segment.feedrateMmSec) && segment.feedrateMmSec >= 0 ? segment.feedrateMmSec : undefined,
          widthMm: coordinate(segment.widthMm) && segment.widthMm > 0 ? segment.widthMm : undefined,
          heightMm: coordinate(segment.heightMm) && segment.heightMm > 0 ? segment.heightMm : undefined,
        }
        segments.push(safe)
        for (const p of [safe.from, safe.to]) {
          minX = Math.min(minX, p[0]); maxX = Math.max(maxX, p[0])
          minY = Math.min(minY, p[1]); maxY = Math.max(maxY, p[1])
          minZ = Math.min(minZ, p[2]); maxZ = Math.max(maxZ, p[2])
        }
      }
      if (layer.index !== i) limited = true
      layers.push({ index: layer.index as number, z: layer.z, segments })
    }
  }
  const warnings = Array.isArray(raw.warnings)
    ? raw.warnings.slice(0, 20).filter((warning): warning is string => typeof warning === 'string').map(warning => warning.slice(0, 500))
    : []
  const coherentCounts = raw.returnedSegments === raw.totalSegments && raw.returnedLayers === raw.totalLayers &&
    raw.totalSegments === inspected && raw.totalLayers === layers.length
  // Legacy XY-only data cannot certify complete XYZ geometry, even if counters match.
  const partial = raw.version === 1 || limited || raw.sourceComplete !== true || raw.truncated === true ||
    (typeof raw.omittedSegments === 'number' && raw.omittedSegments > 0) ||
    (raw.mode === 'full' && (raw.geometryComplete !== true || raw.omittedSegments !== 0 || !coherentCounts))
  const completeGeometry = supported && !partial && raw.version === 2 && raw.mode === 'full' &&
    raw.geometryComplete === true && coherentCounts
  return {
    supported, layers, warnings, completeGeometry,
    partial,
    // Derive bounds from sanitized geometry, never trust remote bounds for sizing.
    bounds: Number.isFinite(minX) ? {
      minX, minY, minZ,
      width: Math.max(maxX - minX, 1), height: Math.max(maxY - minY, 1), depth: Math.max(maxZ - minZ, 0.01),
    } : null,
  }
}

/** Four SVG paths rather than one DOM element per segment; coordinates stay in a fixed viewport. */
export function toolpathPaths(layer: ToolpathLayer | undefined, bounds: ReturnType<typeof sanitizeToolpaths>['bounds']) {
  const paths: Record<ToolpathKind, string> = { model: '', support: '', brim: '', travel: '' }
  if (!bounds || !layer) return paths
  const scale = 960 / Math.max(bounds.width, bounds.height)
  const x = (v: number) => (20 + (1000 - 40 - bounds.width * scale) / 2 + (v - bounds.minX) * scale).toFixed(2)
  const y = (v: number) => (980 - (1000 - 40 - bounds.height * scale) / 2 - (v - bounds.minY) * scale).toFixed(2)
  for (const segment of layer.segments.slice(0, MAX_TOOLPATH_SEGMENTS)) {
    paths[toolpathKind(segment)] += `M${x(segment.from[0])},${y(segment.from[1])}L${x(segment.to[0])},${y(segment.to[1])}`
  }
  return paths
}
