export type CameraWallSize = 'small' | 'medium' | 'large' | 'auto'
export type CameraWallLayout = 'grid' | 'focus'
export interface CameraWallPreferences {
  size: CameraWallSize
  layout: CameraWallLayout
  /** null selects all cameras, including cameras discovered later. */
  selected: string[] | null
  order: string[]
  favorites: string[]
}

export const CAMERA_WALL_STORAGE_KEY = 'picofarm.camera-wall.v1'

function ids(value: unknown): string[] {
  return Array.isArray(value)
    ? [...new Set(value.filter((id): id is string => typeof id === 'string' && id.length > 0 && id.length <= 512))].slice(0, 10000)
    : []
}

/** Accepts untrusted parsed storage. Unknown fields and invalid values are discarded. */
export function normalizeCameraWallPreferences(value: unknown): CameraWallPreferences {
  const input = value && typeof value === 'object' ? value as Record<string, unknown> : {}
  return {
    size: typeof input.size === 'string' && ['small', 'medium', 'large', 'auto'].includes(input.size) ? input.size as CameraWallSize : 'auto',
    layout: input.layout === 'focus' ? 'focus' : 'grid',
    selected: Array.isArray(input.selected) ? ids(input.selected) : null,
    order: ids(input.order),
    favorites: ids(input.favorites),
  }
}

/** Known saved IDs first; newly discovered IDs retain API order. Never mutates inputs. */
export function orderCameraIds(available: readonly string[], saved: readonly string[]): string[] {
  const known = new Set(available)
  return [...new Set([...saved.filter(id => known.has(id)), ...available])]
}

export function readCameraWallPreferences(): CameraWallPreferences {
  try {
    return normalizeCameraWallPreferences(JSON.parse(localStorage.getItem(CAMERA_WALL_STORAGE_KEY) || 'null'))
  } catch {
    return normalizeCameraWallPreferences(null)
  }
}
