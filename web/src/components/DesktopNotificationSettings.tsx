import { useState } from 'react'
import { desktopRuntime } from '../lib/platform'
import { notificationPreferences, saveNotificationPreferences } from '../lib/desktopNotifications'

export function DesktopNotificationSettings() {
  const [preferences, setPreferences] = useState(notificationPreferences)
  const [error, setError] = useState('')
  if (!desktopRuntime()?.notify) return null
  return <section className="card p-6 space-y-3">
    <h2 className="text-lg font-semibold">Desktop notifications</h2>
    <p className="text-sm text-surface-400">Stored on this device only. Notifications contain no customer or job details. Delivery requires the app to be running and OS permission.</p>
    {(['finished', 'failed', 'critical'] as const).map(kind => <label key={kind} className="flex items-center gap-3 text-sm">
      <input type="checkbox" checked={preferences[kind]} onChange={event => {
        const next = { ...preferences, [kind]: event.target.checked }
        try { saveNotificationPreferences(next); setPreferences(next); setError('') }
        catch { setError('Could not save preferences on this device.') }
      }} />
      {{ finished: 'Print finished', failed: 'Print failed', critical: 'Critical alerts' }[kind]}
    </label>)}
    {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
  </section>
}
