import { useEffect, useState } from 'react'
import { archivesApi } from '../api/client'
import type { PrintArchive } from '../types'

export default function Archives() {
  const [items, setItems] = useState<PrintArchive[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    archivesApi.list().then(setItems).finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="p-6">Loading archives...</div>

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-semibold">Print Archives</h1>
        <a className="btn btn-secondary" href="/api/archives/log/export">Export CSV</a>
      </div>
      <div className="card overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-surface-800 text-surface-400">
            <tr><th className="p-3 text-left">Status</th><th className="p-3 text-left">Duration</th><th className="p-3 text-left">Filament</th><th className="p-3 text-left">Cost</th></tr>
          </thead>
          <tbody>
            {items.map(item => (
              <tr key={item.id} className="border-t border-surface-800">
                <td className="p-3">{item.status}</td>
                <td className="p-3">{item.duration_seconds}s</td>
                <td className="p-3">{item.filament_used_grams.toFixed(1)}g</td>
                <td className="p-3">${(item.cost_cents / 100).toFixed(2)}</td>
              </tr>
            ))}
            {items.length === 0 && <tr><td className="p-3 text-surface-400" colSpan={4}>No archives yet.</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  )
}
