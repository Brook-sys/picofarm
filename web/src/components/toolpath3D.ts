import * as THREE from 'three'
import type { ToolpathLayer, ToolpathSegment } from '../lib/slicerToolpaths'

export const EXTRUSION_CATEGORIES = ['walls', 'infill', 'bridge', 'support', 'brim', 'other'] as const
export const EXTRUSION_TRIANGLES_PER_SEGMENT = 12
export type ExtrusionCategory = typeof EXTRUSION_CATEGORIES[number]
export type Visibility = Record<ExtrusionCategory, boolean>

export interface ExtrusionGeometryData {
  category: ExtrusionCategory
  /** Column-major instance matrices: one oriented rectangular prism per segment. */
  matrices: Float32Array
  segments: number
  estimatedSegments: boolean
}

export function extrusionCategory(feature: string | undefined): ExtrusionCategory {
  const value = feature?.toLowerCase() ?? ''
  if (value === 'brim') return 'brim'
  if (['support', 'support interface', 'support transition'].includes(value)) return 'support'
  if (value === 'bridge' || value === 'internal bridge') return 'bridge'
  if (['sparse infill', 'internal solid infill', 'gap infill', 'top surface', 'bottom surface', 'solid infill'].includes(value)) return 'infill'
  if (['outer wall', 'inner wall', 'overhang wall'].includes(value)) return 'walls'
  return 'other'
}

function finitePositive(value: unknown, fallback: number) {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 && value <= 100 ? value : fallback
}

function hasExtrusionLength(segment: ToolpathSegment) {
  const dx = segment.to[0] - segment.from[0]
  const dy = segment.to[1] - segment.from[1]
  const dz = segment.to[2] - segment.from[2]
  return dx * dx + dy * dy + dz * dz > 1e-14
}

// The unit box is transformed into an oriented prism. Unlike a loose center
// line, this preserves nominal bead width/height. It is not a flow simulation.
function writePrism(segment: ToolpathSegment, matrices: Float32Array, offset: number) {
  const [ax, ay, az] = segment.from
  const dx = segment.to[0] - ax, dy = segment.to[1] - ay, dz = segment.to[2] - az
  const length = Math.hypot(dx, dy, dz)
  const horizontal = Math.hypot(dx, dy)
  const sx = horizontal > 1e-7 ? -dy / horizontal : 1
  const sy = horizontal > 1e-7 ? dx / horizontal : 0
  const ux = dx / length, uy = dy / length, uz = dz / length
  const width = finitePositive(segment.widthMm, finitePositive(segment.lineWidthMm, 0.4))
  const height = finitePositive(segment.heightMm, finitePositive(segment.layerHeightMm, 0.2))
  // Basis X follows the segment; basis Y is its horizontal perpendicular;
  // basis Z = X × Y keeps the transform right-handed, including Z lifts.
  matrices.set([
    dx, dy, dz, 0,
    sx * width, sy * width, 0, 0,
    -uz * sy * height, uz * sx * height, (ux * sy - uy * sx) * height, 0,
    ax + dx / 2, ay + dy / 2, az + dz / 2, 1,
  ], offset)
}

/**
 * Build at most six instanced meshes, never one Three.js object per segment.
 * Two passes allocate exact typed arrays: 64 bytes per segment, without large
 * temporary JavaScript number arrays or per-segment vertex/index buffers.
 */
export function buildExtrusionMeshes(layers: ToolpathLayer[], maxLayer: number, visible: Visibility): ExtrusionGeometryData[] {
  const groups = new Map<ExtrusionCategory, { count: number; cursor: number; estimated: boolean; matrices: Float32Array | null }>()
  for (const category of EXTRUSION_CATEGORIES) groups.set(category, { count: 0, cursor: 0, estimated: false, matrices: null })
  for (const layer of layers) {
    if (layer.index > maxLayer) continue
    for (const segment of layer.segments) {
      if (!(segment.extrusion > 0) || !hasExtrusionLength(segment)) continue
      const category = extrusionCategory(segment.feature)
      if (!visible[category]) continue
      const group = groups.get(category)!
      group.count++
      if (!finitePositive(segment.widthMm, 0) || !finitePositive(segment.heightMm, 0)) group.estimated = true
    }
  }
  for (const group of groups.values()) group.matrices = new Float32Array(group.count * 16)
  for (const layer of layers) {
    if (layer.index > maxLayer) continue
    for (const segment of layer.segments) {
      if (!(segment.extrusion > 0) || !hasExtrusionLength(segment)) continue
      const category = extrusionCategory(segment.feature)
      if (!visible[category]) continue
      const group = groups.get(category)!
      writePrism(segment, group.matrices!, group.cursor * 16)
      group.cursor++
    }
  }
  return [...groups].filter(([, group]) => group.count > 0).map(([category, group]) => ({
    category,
    matrices: group.matrices!,
    segments: group.count,
    estimatedSegments: group.estimated,
  }))
}

export function extrusionInstances(data: ExtrusionGeometryData, material: THREE.Material) {
  const geometry = new THREE.BoxGeometry(1, 1, 1)
  // Avoid allocating and then discarding the constructor's full matrix buffer.
  const mesh = new THREE.InstancedMesh(geometry, material, 0)
  mesh.count = data.segments
  mesh.instanceMatrix = new THREE.InstancedBufferAttribute(data.matrices, 16)
  mesh.instanceMatrix.setUsage(THREE.StaticDrawUsage)
  mesh.computeBoundingBox()
  mesh.computeBoundingSphere()
  return mesh
}
