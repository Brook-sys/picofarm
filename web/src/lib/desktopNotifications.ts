import { desktopRuntime } from './platform'

export const notificationDefaults = { finished: true, failed: true, critical: true }
export type NotificationPreferences = typeof notificationDefaults
const storageKey = 'picofarm.desktop.notifications'

export function notificationPreferences(): NotificationPreferences {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || '{}')
    return Object.fromEntries(Object.entries(notificationDefaults).map(([key, value]) =>
      [key, typeof saved?.[key] === 'boolean' ? saved[key] : value]
    )) as NotificationPreferences
  } catch { return { ...notificationDefaults } }
}

export function saveNotificationPreferences(preferences: NotificationPreferences): void {
  localStorage.setItem(storageKey, JSON.stringify(preferences))
}

// Bounded, session-only identities. Never persist event/customer payloads.
const seen = new Map<string, number>()
const retention = 24 * 60 * 60 * 1000
function send(kind: keyof NotificationPreferences, id: unknown) {
  const notify = desktopRuntime()?.notify
  if (!notify || !notificationPreferences()[kind] || typeof id !== 'string' || !id) return
  const now = Date.now()
  for (const [key, timestamp] of seen) if (now - timestamp > retention) seen.delete(key)
  const key = `${kind}:${id}`
  if (seen.has(key)) return
  if (seen.size >= 500) seen.delete(seen.keys().next().value!)
  seen.set(key, now)
  const body = { finished: 'A print job finished.', failed: 'A print job failed.', critical: 'A critical alert needs attention.' }[kind]
  // Tags contain no backend identity; all user-visible text is fixed, not event content.
  try { void Promise.resolve(notify({ title: 'PicoFarm', body, tag: `picofarm-${kind}` })).catch(() => {}) } catch { /* Native delivery must not interrupt cache updates. */ }
}

export function notifyDesktopEvent(event: { type: string; data: unknown }): void {
  if (!event.data || typeof event.data !== 'object') return
  const data = event.data as Record<string, unknown>
  if (event.type === 'job_completed') send('finished', data.id)
  if (event.type === 'job_failed') send('failed', data.id)
  if (event.type === 'alerts_updated' && Array.isArray(data.alerts)) {
    for (const alert of data.alerts) {
      if (alert && typeof alert === 'object' && alert.severity === 'critical') send('critical', alert.id)
    }
  }
}
