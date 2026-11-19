/** Injected by the desktop host before mounting the shared frontend. No native imports. */
export interface DesktopRuntime {
  apiBaseURL: string
  wsBaseURL?: string
  openExternal?: (url: string) => void | Promise<void>
  notify?: (notification: { title: string; body: string; tag: string }) => void | Promise<void>
}

declare global {
  interface Window { __PICOFARM_DESKTOP__?: DesktopRuntime }
}

export function desktopRuntime(): DesktopRuntime | undefined {
  return typeof window === 'undefined' ? undefined : window.__PICOFARM_DESKTOP__
}

export function apiBaseURL(): string {
  const base = desktopRuntime()?.apiBaseURL ?? import.meta.env?.VITE_API_URL
  if (base) {
    const url = new URL(base)
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) {
      throw new Error('API base URL must be HTTP(S) without credentials')
    }
    return url.href.replace(/\/$/, '')
  }
  return typeof window === 'undefined' ? 'http://localhost:8084' : window.location.origin
}

/** Rebase backend resources only; never rewrite third-party, blob or data URLs. */
export function resourceURL(path: string): string {
  return /^\/(?:api(?:\/|\?|$)|uploads(?:\/|$))/.test(path) ? `${apiBaseURL()}${path}` : path
}

/** Storage may be blocked by privacy settings or desktop policies; never throw. */
export function readStorage(key: string): string | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage.getItem(key)
  } catch {
    return null
  }
}

export function writeStorage(key: string, value: string | null): boolean {
  try {
    if (typeof window === 'undefined') return false
    if (value === null) window.localStorage.removeItem(key)
    else window.localStorage.setItem(key, value)
    return true
  } catch {
    return false
  }
}

// Session-only fallback so authentication still works when persistence is
// blocked. The token is kept in memory, never logged, and is lost on reload.
let sessionToken: string | null = null
let sessionTokenOverride = false

export function readApiToken(): string | null {
  return sessionTokenOverride ? sessionToken : readStorage('API_TOKEN')
}

/** Returns true when the token was also persisted across reloads. */
export function writeApiToken(token: string | null): boolean {
  sessionToken = token
  const persisted = writeStorage('API_TOKEN', token)
  sessionTokenOverride = !persisted
  return persisted
}

export function webSocketURL(): string {
  const configured = desktopRuntime()?.wsBaseURL ?? import.meta.env?.VITE_WS_URL
  const url = new URL(configured || `${apiBaseURL()}/ws`)
  if (url.protocol === 'http:') url.protocol = 'ws:'
  if (url.protocol === 'https:') url.protocol = 'wss:'
  if (!['ws:', 'wss:'].includes(url.protocol) || url.username || url.password) {
    throw new Error('WebSocket URL must be WS(S) without credentials')
  }
  if (url.pathname === '/') url.pathname = '/ws'
  const token = readApiToken()
  if (token) url.searchParams.set('token', token)
  return url.href
}

/** Attach to third-party anchors: browser target/modifier behavior stays unchanged. */
export function externalLinkClick(event: { preventDefault(): void; currentTarget: { href: string } }): void {
  if (!desktopRuntime()?.openExternal) return
  event.preventDefault()
  void openExternal(event.currentTarget.href).catch(() => {
    console.warn('Could not open external browser')
  })
}

/** Web OAuth retains navigation behavior; desktop delegates to the OS browser. */
export async function openExternal(url: string): Promise<void> {
  const parsed = new URL(url)
  if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) {
    throw new Error('External URL must be HTTP(S) without credentials')
  }
  const open = desktopRuntime()?.openExternal
  if (open) await open(parsed.href)
  else window.location.assign(parsed.href)
}
