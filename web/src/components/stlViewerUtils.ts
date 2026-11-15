import * as THREE from 'three'

// Preview budgets, not upload limits. STL parsing still runs on the main thread.
export const MAX_STL_BYTES = 32 * 1024 * 1024
export const MAX_STL_TRIANGLES = 500_000
export type STLView = 'reset' | 'top' | 'front' | 'side'

export function inspectSTLGeometry(geometry: THREE.BufferGeometry) {
  const positions = geometry.getAttribute('position')
  const triangles = (geometry.index?.count ?? positions?.count ?? 0) / 3
  if (!positions || !Number.isInteger(triangles) || triangles <= 0) {
    throw new Error('The STL contains no valid triangles.')
  }
  if (triangles > MAX_STL_TRIANGLES) throw new Error('The STL exceeds the 500,000 triangle preview limit.')
  for (let i = 0; i < positions.count; i++) {
    if (!Number.isFinite(positions.getX(i)) || !Number.isFinite(positions.getY(i)) || !Number.isFinite(positions.getZ(i))) {
      throw new Error('The STL contains invalid coordinates.')
    }
  }
  geometry.computeBoundingBox()
  const box = geometry.boundingBox!
  const dimensions = box.getSize(new THREE.Vector3())
  const radius = dimensions.length() / 2
  if (!Number.isFinite(radius) || radius <= 0) throw new Error('The STL has invalid or zero dimensions.')
  return { dimensions, center: box.getCenter(new THREE.Vector3()), radius, triangles }
}

// Mesh rotation maps STL Z up to world Y up. Views refer to original STL axes.
export function stlViewPose(view: STLView) {
  switch (view) {
    case 'top': return { direction: new THREE.Vector3(0, 1, 0), up: new THREE.Vector3(0, 0, -1) }
    case 'front': return { direction: new THREE.Vector3(0, 0, 1), up: new THREE.Vector3(0, 1, 0) }
    case 'side': return { direction: new THREE.Vector3(1, 0, 0), up: new THREE.Vector3(0, 1, 0) }
    default: return { direction: new THREE.Vector3(0.85, 0.72, 1).normalize(), up: new THREE.Vector3(0, 1, 0) }
  }
}

export function stlCameraDistance(radius: number, aspect: number, fov: number) {
  const vertical = THREE.MathUtils.degToRad(fov / 2)
  const horizontal = Math.atan(Math.tan(vertical) * aspect)
  return radius / Math.sin(Math.min(vertical, horizontal)) * 1.1
}

export async function readSTLResponse(response: Response, signal: AbortSignal) {
  if (!response.ok) throw new Error(`Could not download the STL (HTTP ${response.status}).`)
  if (Number(response.headers.get('content-length')) > MAX_STL_BYTES) {
    throw new Error('The STL exceeds the 32 MiB preview limit.')
  }
  if (!response.body) throw new Error('The STL download returned no data.')
  const reader = response.body.getReader()
  const chunks: Uint8Array[] = []
  let size = 0
  try {
    while (true) {
      signal.throwIfAborted()
      const { done, value } = await reader.read()
      if (done) break
      size += value.byteLength
      if (size > MAX_STL_BYTES) throw new Error('The STL exceeds the 32 MiB preview limit.')
      chunks.push(value)
    }
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
  signal.throwIfAborted()
  const data = new Uint8Array(size)
  let offset = 0
  for (const chunk of chunks) {
    data.set(chunk, offset)
    offset += chunk.byteLength
  }
  // Reject oversized binary STL before STLLoader allocates its vertex arrays.
  if (size >= 84) {
    const count = new DataView(data.buffer).getUint32(80, true)
    if (84 + count * 50 === size && count > MAX_STL_TRIANGLES) {
      throw new Error('The STL exceeds the 500,000 triangle preview limit.')
    }
  }
  return data.buffer
}
