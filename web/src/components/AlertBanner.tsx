import { useEffect, useState } from 'react'
import { AlertTriangle, X, Clock, Bell } from 'lucide-react'
import { alertsApi } from '../api/client'
import type { Alert, AlertSeverity } from '../types'

interface AlertBannerProps {
  className?: string
}

export function AlertBanner({ className = '' }: AlertBannerProps) {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)
  const [dismissingId, setDismissingId] = useState<string | null>(null)

  useEffect(() => {
    loadAlerts()
    // Poll for updates every 30 seconds
    const interval = setInterval(loadAlerts, 30000)
    return () => clearInterval(interval)
  }, [])

  const loadAlerts = async () => {
    try {
      const data = await alertsApi.list()
      setAlerts(data)
    } catch (err) {
      console.error('Failed to load alerts:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleDismiss = async (alert: Alert, duration: string) => {
    setDismissingId(alert.id)
    try {
      await alertsApi.dismiss(alert.type, alert.entity_id, duration)
      setAlerts(prev => prev.filter(a => a.id !== alert.id))
    } catch (err) {
      console.error('Failed to dismiss alert:', err)
    } finally {
      setDismissingId(null)
    }
  }

  if (loading || alerts.length === 0) {
    return null
  }

  // Group alerts by severity, showing critical first
  const criticalAlerts = alerts.filter(a => a.severity === 'critical')
  const warningAlerts = alerts.filter(a => a.severity === 'warning')
  const infoAlerts = alerts.filter(a => a.severity === 'info')
  const sortedAlerts = [...criticalAlerts, ...warningAlerts, ...infoAlerts]

  const severityStyles: Record<AlertSeverity, string> = {
    info: 'bg-blue-500/10 border-blue-500/30 text-blue-300',
    warning: 'bg-amber-500/10 border-amber-500/30 text-amber-300',
    critical: 'bg-red-500/10 border-red-500/30 text-red-300',
  }

  const severityIcons: Record<AlertSeverity, React.ReactNode> = {
    info: <Bell className="w-4 h-4" />,
    warning: <AlertTriangle className="w-4 h-4" />,
    critical: <AlertTriangle className="w-4 h-4" />,
  }

  const snoozeDurations = ['1h', '4h', '24h', 'permanent']

  return (
    <div className={`space-y-2 ${className}`} role="region" aria-label="Active alerts">
      {sortedAlerts.slice(0, 3).map(alert => (
        <div
          key={alert.id}
          className={`flex items-center justify-between px-4 py-2 rounded-lg border ${severityStyles[alert.severity]}`}
        >
          <div className="flex items-center gap-3">
            {severityIcons[alert.severity]}
            <span className="text-sm font-medium">{alert.message}</span>
          </div>

          <div className="flex items-center gap-2">
            {/* Snooze options */}
            <div className="relative group">
              <button
                onClick={() => handleDismiss(alert, '1h')}
                disabled={dismissingId === alert.id}
                className="p-1.5 rounded hover:bg-white/10 transition-colors"
                title="Snooze for 1 hour"
              >
                <Clock className="w-4 h-4" />
              </button>
              <div className="absolute right-0 top-full mt-1 hidden group-hover:block z-10">
                <div className="bg-surface-800 border border-surface-700 rounded-lg shadow-lg py-1 min-w-[120px]">
                  {snoozeDurations.map(duration => (
                    <button
                      key={duration}
                      onClick={() => handleDismiss(alert, duration)}
                      className="w-full px-3 py-1.5 text-left text-sm text-surface-300 hover:bg-surface-700 hover:text-surface-100"
                    >
                      {duration === 'permanent' ? 'Dismiss permanently' : duration.replace('h', ' hour(s)')}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Quick dismiss */}
            <button
              onClick={() => handleDismiss(alert, '1h')}
              disabled={dismissingId === alert.id}
              className="p-1.5 rounded hover:bg-white/10 transition-colors"
              title="Dismiss"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
      ))}
    </div>
  )
}

export default AlertBanner
