import { useEffect, useState } from 'react'
import { camerasApi } from '../api/client'
import type { Camera } from '../types'

export default function Cameras() {
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    camerasApi.list().then(setCameras).finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="p-6">Loading cameras...</div>

  return (
    <div className="p-6">
      <h1 className="text-2xl font-semibold mb-4">Cameras</h1>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {cameras.length === 0 && <div className="text-surface-400">No cameras configured.</div>}
        {cameras.map(c => (
          <div key={c.id} className="card p-4">
            <div className="font-medium">{c.name}</div>
            <div className="text-xs text-surface-400">{c.type} • {c.url}</div>
            <div className="mt-2 text-sm">Enabled: {c.enabled ? 'Yes' : 'No'}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
