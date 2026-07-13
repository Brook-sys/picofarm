import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle, Clock, FileCode, Gauge, History, Info, Layers, Play, Printer, RefreshCw, Search, SlidersHorizontal, Trash2, Upload, XCircle } from 'lucide-react'
import { queueApi } from '../api/client'
import AppToast, { type AppToastState } from '../components/AppToast'
import { usePrinters } from '../hooks/usePrinters'
import { useSpoolsWithMaterials } from '../hooks/useMaterials'
import type { GCodeQueueItem, Material, MaterialSpool, Printer as PrinterRecord, QueueItem, QueueResponse } from '../types'
import { cn, formatDuration, getStatusBadge } from '../lib/utils'


type QueueViewMode = 'normal' | 'compact' | 'thumb'

function getQueueSourceBadge(sourceType: GCodeQueueItem['source_type']) {
  if (sourceType === 'project' || sourceType === 'print_job') return { label: 'Projects', className: 'border-blue-500/30 bg-blue-500/15 text-blue-200' }
  if (sourceType === 'library') return { label: 'Files', className: 'border-orange-500/30 bg-orange-500/15 text-orange-200' }
  if (sourceType === 'upload') return { label: 'Upload', className: 'border-purple-500/30 bg-purple-500/15 text-purple-200' }
  return { label: 'Manual', className: 'border-surface-600 bg-surface-800 text-surface-300' }
}

const columnConfig = {
  ready: { title: 'Ready to Print', description: 'G-code files that passed preflight', icon: CheckCircle, accent: 'text-emerald-400', border: 'border-emerald-500/20' },

  active: { title: 'Printing / Active', description: 'Printing or paused G-code files', icon: Gauge, accent: 'text-blue-400', border: 'border-blue-500/20' },
} as const

export default function PrintQueue() {
  const { data: printers = [] } = usePrinters()
  const { data: spools = [] } = useSpoolsWithMaterials()
  const [queue, setQueue] = useState<QueueResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [busyItem, setBusyItem] = useState('')
  const [uploading, setUploading] = useState(false)
  const [toast, setToast] = useState<AppToastState | null>(null)
  const [viewingItem, setViewingItem] = useState<QueueItem | null>(null)
  const [viewMode, setViewMode] = useState<QueueViewMode>('normal')

  const renameItem = async (item: QueueItem, displayName: string) => {
    const next = displayName.trim()
    if (!next || next === item.item.display_name) return
    setBusyItem(item.item.id)
    setError('')
    try {
      await queueApi.update(item.item.id, { display_name: next })
      await loadQueue(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rename item')
    } finally {
      setBusyItem('')
    }
  }

  const loadQueue = async (silent = false) => {
    if (!silent) setLoading(true)
    setError('')
    try {
      setQueue(await queueApi.get())
    } catch (err) {
      if (!silent) setError(err instanceof Error ? err.message : 'Failed to load queue')
    } finally {
      if (!silent) setLoading(false)
    }
  }

  const showToast = (next: AppToastState) => {
    setToast(next)
    window.setTimeout(() => setToast(null), 3500)
  }

  useEffect(() => {
    loadQueue()
    const interval = window.setInterval(() => {
      loadQueue(true)
    }, 3000)
    return () => window.clearInterval(interval)
  }, [])

  const filteredItems = useMemo(() => {
    const items = queue?.items ?? []
    const q = query.trim().toLowerCase()
    return items.filter(item => {
      if (statusFilter && item.item.status !== statusFilter) return false
      if (!q) return true
      const haystack = [
        item.item.display_name,
        item.item.file_name,
        item.item.material_type,
        item.item.material_color,
        item.item.notes,
        item.printer?.name,
        item.material?.name,
        item.material?.type,
      ].filter(Boolean).join(' ').toLowerCase()
      return haystack.includes(q)
    })
  }, [queue, query, statusFilter])

  const byColumn = {
    ready: filteredItems.filter(i => i.column === 'ready' || i.column === 'blocked').sort(sortQueue),

    active: filteredItems.filter(i => i.column === 'active').sort(sortQueue),
  }
  const recentCompleted = useMemo(() => (
    (queue?.items ?? [])
      .filter(item => item.item.status === 'done')
      .sort((a, b) => new Date(b.item.updated_at).getTime() - new Date(a.item.updated_at).getTime())
      .slice(0, 3)
  ), [queue])
  const pendingTotals = useMemo(() => {
    const items = queue?.items ?? []
    const printable = items.filter(item => (item.column === 'ready' || item.column === 'active') && item.item.status !== 'failed' && item.item.status !== 'cancelled')
    return {
      estimatedSeconds: printable.reduce((sum, item) => sum + (item.item.estimated_seconds || 0), 0),
      filamentGrams: printable.reduce((sum, item) => sum + (item.item.filament_grams || 0), 0),
    }
  }, [queue])
  const availablePrinters = printers.filter(p => !p.maintenance_mode)
  const availableSpools = spools.filter(s => s.status !== 'empty' && s.status !== 'archived')

  const runItemAction = async (itemId: string, action: () => Promise<unknown>) => {
    setBusyItem(itemId)
    setError('')
    try {
      await action()
      await loadQueue()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Action failed')
    } finally {
      setBusyItem('')
    }
  }

  const reorderItems = async (items: QueueItem[]) => {
    setBusyItem('reorder')
    setError('')
    try {
      await Promise.all(items.map((item, index) => queueApi.updatePriority(item.item.id, (items.length - index) * 10)))
      await loadQueue(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reorder queue')
    } finally {
      setBusyItem('')
    }
  }

  const uploadGCodeFiles = async (files?: FileList | null) => {
    if (!files || files.length === 0) return
    setUploading(true)
    setError('')
    try {
      for (const file of Array.from(files)) {
        await queueApi.upload(file, {})
      }
      await loadQueue(true)
      showToast({ title: 'G-code uploaded', message: `${files.length} file${files.length === 1 ? '' : 's'} added to the queue.`, tone: 'success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8">
      <div className="flex items-start justify-between gap-4 mb-8">
        <div>
          <h1 className="text-3xl font-display font-bold text-surface-100 flex items-center gap-3">
            <Layers className="h-8 w-8 text-accent-500" />
            Print Queue
          </h1>
          <p className="text-surface-400 mt-1">Independent manual queue for printable .gcode files</p>
        </div>
        <div className="flex gap-2">
          <label className="btn btn-primary cursor-pointer">
            <Upload className="h-4 w-4 mr-2" />{uploading ? 'Uploading...' : 'Add G-code'}
            <input type="file" accept=".gcode" multiple disabled={uploading} className="hidden" onChange={e => uploadGCodeFiles(e.target.files)} />
          </label>
          <button onClick={() => loadQueue()} className="btn btn-secondary" disabled={loading}><RefreshCw className={cn('h-4 w-4 mr-2', loading && 'animate-spin')} />Refresh</button>
        </div>
      </div>

      {queue && (
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-6">
          <SummaryCard label="Ready" value={queue.summary.ready_count} tone="text-emerald-400" />
          <SummaryCard label="Attention" value={queue.summary.blocked_count} tone="text-amber-400" />
          <SummaryCard label="Active" value={queue.summary.active_count} tone="text-blue-400" />
          <SummaryCard label="Est. Time" value={formatDuration(pendingTotals.estimatedSeconds)} tone="text-surface-100" />
          <SummaryCard label="Filament" value={`${Math.round(pendingTotals.filamentGrams)}g`} tone="text-surface-100" />
        </div>
      )}

      <div className="card p-3 mb-6">
        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_auto] gap-3 items-center">
          <div className="relative min-w-0">
            <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
            <input value={query} onChange={e => setQuery(e.target.value)} className="input input-with-icon w-full" placeholder="Search G-code, display name, printer, material..." />
          </div>
          <div className="flex flex-col sm:flex-row sm:items-center gap-2 min-w-0">
            <div className="inline-grid grid-cols-3 rounded-lg border border-surface-700 bg-surface-900 p-1 w-full sm:w-auto">
              {(['normal', 'compact', 'thumb'] as QueueViewMode[]).map(mode => (
                <button key={mode} onClick={() => setViewMode(mode)} className={cn('rounded-md px-2.5 py-1.5 text-xs font-medium capitalize transition-colors whitespace-nowrap', viewMode === mode ? 'bg-accent-500 text-white' : 'text-surface-400 hover:bg-surface-800 hover:text-surface-100')}>
                  {mode === 'compact' ? 'Reduced' : mode}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-2 min-w-0">
              <SlidersHorizontal className="h-4 w-4 shrink-0 text-surface-500" />
              <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} className="input w-full sm:w-40">
                <option value="">All statuses</option>
                <option value="queued">Queued</option>
                <option value="ready">Ready</option>
                <option value="printing">Printing</option>
                <option value="paused">Paused</option>
                <option value="failed">Failed</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {error && <div className="mb-4 rounded-lg border border-red-500/20 bg-red-500/10 text-red-400 px-4 py-3 text-sm">{error}</div>}
      {toast && <AppToast toast={toast} onClose={() => setToast(null)} />}
      {viewingItem && <ItemDetailsModal item={viewingItem.item} onClose={() => setViewingItem(null)} />}

      {loading ? <div className="text-surface-500">Loading queue...</div> : !queue || queue.items.length === 0 ? (
        <div className="text-center py-16">
          <FileCode className="h-16 w-16 mx-auto mb-4 text-surface-600" />
          <h3 className="text-xl font-semibold text-surface-300 mb-2">No queued G-code</h3>
          <p className="text-surface-500 mb-4">Upload a .gcode file to build your manual print queue.</p>
          <label className="btn btn-primary cursor-pointer">
            {uploading ? 'Uploading...' : 'Add G-code'}
            <input type="file" accept=".gcode" multiple disabled={uploading} className="hidden" onChange={e => uploadGCodeFiles(e.target.files)} />
          </label>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 items-start xl:grid-cols-3">
            {(['ready', 'active'] as const).map(column => (
              <QueueColumn
                key={column}
                column={column}
                items={byColumn[column]}
                busyItem={busyItem}
                onPrintNow={(item) => runItemAction(item.item.id, async () => {
                  try {
                    await queueApi.start(item.item.id)
                  } catch (err) {
                    const msg = err instanceof Error ? err.message : 'Preflight failed'
                    throw new Error(msg)
                  }
                })}
                onReorder={reorderItems}
                onPause={(item) => runItemAction(item.item.id, () => queueApi.setStatus(item.item.id, 'paused'))}
                onResume={(item) => runItemAction(item.item.id, () => queueApi.setStatus(item.item.id, 'printing'))}
                onCancel={(item) => runItemAction(item.item.id, () => queueApi.setStatus(item.item.id, 'cancelled'))}
                onDelete={(item) => runItemAction(item.item.id, () => queueApi.delete(item.item.id))}
                onQuickAssign={(item, data) => runItemAction(item.item.id, () => queueApi.update(item.item.id, data))}
                onEdit={setViewingItem}
                onRename={renameItem}
                printers={availablePrinters}
                spools={availableSpools}
                viewMode={viewMode}
              />
            ))}
          </div>
          {recentCompleted.length > 0 && <RecentCompleted items={recentCompleted} onOpen={setViewingItem} />}
        </>
      )}
    </div>
  )
}

function RecentCompleted({ items, onOpen }: { items: QueueItem[]; onOpen: (item: QueueItem) => void }) {
  return (
    <section className="mt-6 rounded-xl border border-surface-800 bg-surface-900/50 overflow-hidden">
      <div className="flex items-center justify-between border-b border-surface-800 bg-surface-900 px-4 py-3">
        <div className="flex items-center gap-2">
          <History className="h-5 w-5 text-emerald-400" />
          <div>
            <h2 className="font-semibold text-surface-100">Recently completed</h2>
            <p className="text-xs text-surface-500">Last 3 successful prints</p>
          </div>
        </div>
        <span className="badge bg-surface-800 text-surface-400">{items.length}</span>
      </div>
      <div className="divide-y divide-surface-800">
        {items.map(item => (
          <button key={item.item.id} type="button" onClick={() => onOpen(item)} className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-800/50">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-surface-800">
              {item.item.thumbnail_file_id ? <img src={`/api/files/${item.item.thumbnail_file_id}`} alt="G-code preview" className="h-full w-full object-cover" /> : <CheckCircle className="h-5 w-5 text-emerald-400" />}
            </div>
            <div className="min-w-0 flex-1">
              <div className="truncate font-medium text-surface-100">{item.item.display_name || item.item.file_name}</div>
              <div className="mt-0.5 flex flex-wrap gap-x-2 text-xs text-surface-500">
                <span>{item.printer?.name || 'No printer'}</span>
                <span>{new Date(item.item.updated_at).toLocaleString()}</span>
                {item.item.filament_grams ? <span>{Math.round(item.item.filament_grams)}g</span> : null}
              </div>
            </div>
            <span className="badge border border-emerald-500/30 bg-emerald-500/15 text-emerald-300">done</span>
          </button>
        ))}
      </div>
    </section>
  )
}

function ItemDetailsModal({ item, onClose }: { item: GCodeQueueItem; onClose: () => void }) {
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
      <div className="card p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-surface-100">Queue Item Details</h2>
          <button onClick={onClose} className="btn btn-ghost text-sm">Close</button>
        </div>
        <div className="space-y-4">
          <div className="font-medium text-surface-100 text-lg">{item.display_name || item.file_name}</div>
          <div className="text-xs text-surface-500">File: {item.file_name}</div>
          <div className="grid grid-cols-2 gap-3">
            <div><span className="text-xs text-surface-500">Perfil de Impressão</span><div className="text-surface-100">{item.metadata?.print_settings_id || 'Não encontrado'}</div></div>
            <div><span className="text-xs text-surface-500">Perfil de Impressora</span><div className="text-surface-100">{item.metadata?.printer_settings_id || 'Não encontrado'}</div></div>
            <div><span className="text-xs text-surface-500">Perfil de Filamento</span><div className="text-surface-100">{item.metadata?.filament_settings_id || item.filament_name || 'Não encontrado'}</div></div>
            <div><span className="text-xs text-surface-500">Impressora do G-code</span><div className="text-surface-100">{item.metadata?.printer_model || 'Não encontrado'}</div></div>
            <div><span className="text-xs text-surface-500">Material</span><div className="text-surface-100">{item.material_type?.toUpperCase() || '—'}</div></div>
            <div><span className="text-xs text-surface-500">Filament grams</span><div className="text-surface-100">{item.filament_grams ? Math.round(item.filament_grams) : '—'}</div></div>
            <div><span className="text-xs text-surface-500">ETA</span><div className="text-surface-100">{item.estimated_seconds ? formatDuration(item.estimated_seconds) : '—'}</div></div>
            <div><span className="text-xs text-surface-500">Layer</span><div className="text-surface-100">{item.layer_height || '—'}</div></div>
            <div><span className="text-xs text-surface-500">Nozzle</span><div className="text-surface-100">{item.nozzle_diameter || '—'}</div></div>
            <div><span className="text-xs text-surface-500">Bed / Nozzle temp</span><div className="text-surface-100">{item.bed_temp || '—'} / {item.nozzle_temp || '—'}</div></div>
          </div>
          {item.notes && <div><span className="text-xs text-surface-500">Notes</span><div className="text-surface-100 whitespace-pre-wrap">{item.notes}</div></div>}
        </div>
        <div className="flex justify-end mt-4"><button onClick={onClose} className="btn btn-secondary">Close</button></div>
      </div>
    </div>
  )
}

function QueueColumn({ column, items, busyItem, onPrintNow, onReorder, onPause, onResume, onCancel, onDelete, onQuickAssign, onEdit, onRename, printers, spools, viewMode }: {
  column: keyof typeof columnConfig
  items: QueueItem[]
  busyItem: string
  onPrintNow: (item: QueueItem) => void
  onReorder: (items: QueueItem[]) => void
  onPause: (item: QueueItem) => void
  onResume: (item: QueueItem) => void
  onCancel: (item: QueueItem) => void
  onDelete: (item: QueueItem) => void
  onQuickAssign: (item: QueueItem, data: Partial<GCodeQueueItem>) => void
  onEdit: (item: QueueItem) => void
  onRename: (item: QueueItem, name: string) => void
  printers: PrinterRecord[]
  spools: (MaterialSpool & { material?: Material })[]
  viewMode: QueueViewMode
}) {
  const config = columnConfig[column]
  const Icon = config.icon
  const [draggedId, setDraggedId] = useState<string | null>(null)
  const [dragOverId, setDragOverId] = useState<string | null>(null)
  const reorder = (targetId: string) => {
    if (!draggedId || draggedId === targetId) return
    const from = items.findIndex(item => item.item.id === draggedId)
    const to = items.findIndex(item => item.item.id === targetId)
    if (from < 0 || to < 0) return
    const next = [...items]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    onReorder(next)
  }
  return (
    <div className={cn('rounded-xl border bg-surface-900/50 overflow-hidden', config.border, column === 'ready' && 'xl:col-span-2')}>
      <div className="p-4 border-b border-surface-800 bg-surface-900">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2"><Icon className={cn('h-5 w-5', config.accent)} /><h2 className="font-semibold text-surface-100">{config.title}</h2></div>
          <span className="badge bg-surface-800 text-surface-400">{items.length}</span>
        </div>
        <p className="text-xs text-surface-500 mt-1">{config.description}</p>
      </div>
      <div className={cn('p-3 min-h-[280px]', column === 'ready' && viewMode === 'normal' ? 'grid items-start gap-3 xl:grid-cols-2' : column === 'ready' && viewMode === 'compact' ? 'grid items-start gap-3 xl:grid-cols-3' : column === 'ready' && viewMode === 'thumb' ? 'grid items-start gap-3 xl:grid-cols-2' : 'space-y-3')}>
        {items.length === 0 ? <div className="text-center py-10 text-sm text-surface-600 xl:col-span-2">No items</div> : items.map((item, index) => (
          <QueueCard key={item.item.id} item={item} queuePosition={column === 'ready' ? index + 1 : undefined} busy={busyItem === item.item.id} dragging={draggedId === item.item.id} dragOver={dragOverId === item.item.id} onDragStart={() => setDraggedId(item.item.id)} onDragOver={() => setDragOverId(item.item.id)} onDrop={() => reorder(item.item.id)} onDragEnd={() => { setDraggedId(null); setDragOverId(null) }} onPrintNow={() => onPrintNow(item)} onPause={() => onPause(item)} onResume={() => onResume(item)} onCancel={() => onCancel(item)} onDelete={() => onDelete(item)} onQuickAssign={(data) => onQuickAssign(item, data)} onEdit={() => onEdit(item)} onRename={(name) => onRename(item, name)} printers={printers} spools={spools} viewMode={viewMode} />
        ))}
      </div>
    </div>
  )
}

function QueueCard({ item, queuePosition, busy, dragging, dragOver, onDragStart, onDragOver, onDrop, onDragEnd, onPrintNow, onPause, onResume, onCancel, onDelete, onQuickAssign, onEdit, onRename, printers, spools, viewMode }: {
  item: QueueItem
  queuePosition?: number
  busy: boolean
  dragging: boolean
  dragOver: boolean
  onDragStart: () => void
  onDragOver: () => void
  onDrop: () => void
  onDragEnd: () => void
  onPrintNow: () => void
  onPause: () => void
  onResume: () => void
  onCancel: () => void
  onDelete: () => void
  onQuickAssign: (data: Partial<GCodeQueueItem>) => void
  onEdit: () => void
  onRename: (name: string) => void
  printers: PrinterRecord[]
  spools: (MaterialSpool & { material?: Material })[]
  viewMode: QueueViewMode
}) {
  const queueItem = item.item
  const status = queueItem.status
  const [editingName, setEditingName] = useState(false)
  const [name, setName] = useState(queueItem.display_name || queueItem.file_name)
  const saveName = () => {
    setEditingName(false)
    onRename(name)
  }
  const materialType = queueItem.material_type?.trim().toLowerCase() || ''
  const isCompatibleSpool = (spool: MaterialSpool & { material?: Material }) => !materialType || spool.material?.type?.trim().toLowerCase() === materialType
  const compact = viewMode === 'compact'
  const thumb = viewMode === 'thumb'
  const sourceBadge = getQueueSourceBadge(queueItem.source_type)
  const needsRetry = status === 'failed' || status === 'cancelled'
  const printActionLabel = needsRetry ? 'Retry' : 'Print now'

  if (compact) {
    return (
      <div draggable onDragStart={onDragStart} onDragOver={e => { e.preventDefault(); onDragOver() }} onDrop={e => { e.preventDefault(); onDrop() }} onDragEnd={onDragEnd} className={cn('cursor-grab rounded-xl border border-surface-800 bg-surface-950/60 p-2.5 transition-colors hover:border-surface-700 active:cursor-grabbing', dragging && 'opacity-50', dragOver && !dragging && 'ring-2 ring-accent-500/60')}>
        <div className="flex items-center gap-2.5">
          <div className="w-16 h-12 rounded-lg bg-surface-800 flex items-center justify-center shrink-0 overflow-hidden">
            {queueItem.thumbnail_file_id ? <img src={`/api/files/${queueItem.thumbnail_file_id}`} alt="G-code preview" className="w-full h-full object-cover" /> : <FileCode className="h-6 w-6 text-surface-500" />}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5 mb-1">
              {queuePosition && <span className="inline-flex h-6 min-w-6 items-center justify-center rounded-full border border-emerald-500/50 bg-emerald-500/15 px-2 text-xs font-bold text-emerald-200">#{queuePosition}</span>}
              <span className={cn('badge', getStatusBadge(status))}>{status}</span>
              {needsRetry && <span className="badge border border-amber-500/40 bg-amber-500/15 text-amber-300">retry ready</span>}
              <span className={cn('badge border', sourceBadge.className)}>{sourceBadge.label}</span>
            </div>
            {editingName ? (
              <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(queueItem.display_name || queueItem.file_name); setEditingName(false) } }} className="input py-1 text-sm" />
            ) : (
              <button type="button" onClick={() => setEditingName(true)} className="block w-full min-w-0 overflow-hidden truncate text-left text-sm font-medium text-surface-100 hover:text-accent-300" title={queueItem.display_name || queueItem.file_name}>{queueItem.display_name || queueItem.file_name}</button>
            )}
            <div className="mt-1 flex flex-wrap gap-x-2 text-xs text-surface-500">
              <span>{item.printer?.name || 'No printer'}</span>
              <span>{queueItem.estimated_seconds ? formatDuration(queueItem.estimated_seconds) : 'No ETA'}</span>

            </div>
          </div>
        </div>
        {(status === 'printing' || status === 'paused') && (
          <div className="mt-2 h-1.5 rounded-full bg-surface-800 overflow-hidden">
            <div className="h-full bg-accent-500 transition-all" style={{ width: `${Math.min(100, Math.max(0, queueItem.progress || 0))}%` }} />
          </div>
        )}
        {item.blocked_by && item.blocked_by.length > 0 && <div className="mt-2 flex items-center gap-1.5 text-xs text-amber-400"><AlertTriangle className="h-3 w-3" /> {item.blocked_by[0]}</div>}
        <div className="mt-2 grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto_auto] gap-1.5">
          <select className="input h-7 min-w-0 py-0 text-xs" value={queueItem.assigned_printer_id || ''} onChange={async e => onQuickAssign({ assigned_printer_id: e.target.value || undefined })} title="Printer">
            <option value="" disabled hidden>Printer</option>
            {printers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
          <select className="input h-7 min-w-0 py-0 text-xs" value={queueItem.assigned_spool_id || ''} onChange={async e => {
            const sid = e.target.value || undefined
            const spool = spools.find(s => s.id === sid)
            if (spool && !isCompatibleSpool(spool)) return
            const patch: Partial<GCodeQueueItem> = { assigned_spool_id: sid || null }
            if (spool?.material) {
              patch.material_type = spool.material.type
              patch.material_color = spool.material.color_hex || spool.material.color
            }
            onQuickAssign(patch)
          }} title="Filament">
            <option value="">Filamento</option>
            {spools.map(s => <option key={s.id} value={s.id} disabled={!isCompatibleSpool(s)}>{s.material ? `${s.material.type.toUpperCase()} · ${s.material.color || s.material.name}` : 'Unknown'} · {Math.round(s.remaining_weight)}g</option>)}
          </select>
          <button disabled={busy} onClick={onEdit} title="Details" className="inline-flex h-7 w-8 shrink-0 items-center justify-center rounded-lg border border-surface-700 bg-surface-800 text-surface-200 transition-colors hover:border-accent-500/60 hover:bg-accent-500/15 hover:text-accent-200 disabled:cursor-not-allowed disabled:opacity-50"><Info className="h-4 w-4" /></button>
          <button disabled={busy} onClick={onDelete} title="Delete" className="inline-flex h-7 w-8 shrink-0 items-center justify-center rounded-lg border border-red-500/35 bg-red-500/10 text-red-300 transition-colors hover:border-red-400 hover:bg-red-500/20 hover:text-red-200 disabled:cursor-not-allowed disabled:opacity-50"><Trash2 className="h-4 w-4" /></button>
              {item.column === 'ready' && <button disabled={busy} onClick={onPrintNow} className="btn btn-primary col-span-4 text-xs py-1 px-2"><Play className="h-3.5 w-3.5 mr-1" />{needsRetry ? 'Retry' : 'Print'}</button>}
        </div>
        {busy && <div className="text-xs text-surface-500 mt-2">Working...</div>}
      </div>
    )
  }

  if (thumb) {
    return (
      <div draggable onDragStart={onDragStart} onDragOver={e => { e.preventDefault(); onDragOver() }} onDrop={e => { e.preventDefault(); onDrop() }} onDragEnd={onDragEnd} className={cn('relative h-80 cursor-grab overflow-hidden rounded-xl border border-surface-800 bg-surface-950 transition-colors hover:border-surface-700 active:cursor-grabbing', dragging && 'opacity-50', dragOver && !dragging && 'ring-2 ring-accent-500/60')}>
        {queueItem.thumbnail_file_id ? (
          <img src={`/api/files/${queueItem.thumbnail_file_id}`} alt="G-code preview" className="absolute inset-0 h-full w-full object-cover" />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center bg-surface-900"><FileCode className="h-20 w-20 text-surface-600" /></div>
        )}
        <div className="absolute inset-0 bg-gradient-to-t from-surface-950/80 via-surface-900/20 to-surface-900/55" />
        <div className="relative z-10 flex h-full flex-col justify-between p-4">
          <div className="flex flex-wrap items-center gap-2 drop-shadow">
            {queuePosition && <span className="inline-flex h-7 min-w-7 items-center justify-center rounded-full border border-emerald-400/60 bg-emerald-500/20 px-2 text-xs font-bold text-emerald-100">#{queuePosition}</span>}
            <span className={cn('badge', getStatusBadge(status))}>{status}</span>
            {needsRetry && <span className="badge border border-amber-500/40 bg-amber-500/15 text-amber-300">retry ready</span>}
            <span className={cn('badge border', sourceBadge.className)}>{sourceBadge.label}</span>
          </div>
          <div className="space-y-3 rounded-xl border border-white/10 bg-surface-950/55 p-3 shadow-2xl backdrop-blur-sm">
            <div>
              {editingName ? (
                <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(queueItem.display_name || queueItem.file_name); setEditingName(false) } }} className="input py-1 text-sm" />
              ) : (
                <button type="button" onClick={() => setEditingName(true)} className="line-clamp-2 w-full min-w-0 overflow-hidden text-left text-base font-semibold text-white hover:text-accent-200" title={queueItem.display_name || queueItem.file_name}>{queueItem.display_name || queueItem.file_name}</button>
              )}
              <div className="mt-1 flex flex-wrap gap-2 text-xs text-surface-200">
                <span>{item.printer?.name || 'No printer'}</span>
                <span>·</span>
                <span>{queueItem.estimated_seconds ? formatDuration(queueItem.estimated_seconds) : 'No ETA'}</span>
                {queueItem.filament_grams ? <><span>·</span><span>{Math.round(queueItem.filament_grams)}g</span></> : null}
              </div>
            </div>
            {(status === 'printing' || status === 'paused') && (
              <div>
                <div className="mb-1 flex items-center justify-between text-xs text-surface-200"><span>Progress</span><span>{Math.round(queueItem.progress || 0)}%</span></div>
                <div className="h-2 rounded-full bg-white/20 overflow-hidden"><div className="h-full bg-accent-500" style={{ width: `${Math.min(100, Math.max(0, queueItem.progress || 0))}%` }} /></div>
              </div>
            )}
            <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto_auto] gap-2">
              <select className="input h-8 min-w-0 border-white/15 bg-surface-900/70 py-0 text-xs text-white" value={queueItem.assigned_printer_id || ''} onChange={async e => onQuickAssign({ assigned_printer_id: e.target.value || undefined })} title="Printer">
                <option value="" disabled hidden>Printer</option>
                {printers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
              <select className="input h-8 min-w-0 border-white/15 bg-surface-900/70 py-0 text-xs text-white" value={queueItem.assigned_spool_id || ''} onChange={async e => {
                const sid = e.target.value || undefined
                const spool = spools.find(s => s.id === sid)
                if (spool && !isCompatibleSpool(spool)) return
                const patch: Partial<GCodeQueueItem> = { assigned_spool_id: sid || null }
                if (spool?.material) {
                  patch.material_type = spool.material.type
                  patch.material_color = spool.material.color_hex || spool.material.color
                }
                onQuickAssign(patch)
              }} title="Filament">
                <option value="">Filamento</option>
                {spools.map(s => <option key={s.id} value={s.id} disabled={!isCompatibleSpool(s)}>{s.material ? `${s.material.type.toUpperCase()} · ${s.material.color || s.material.name}` : 'Unknown'} · {Math.round(s.remaining_weight)}g</option>)}
              </select>
              <button disabled={busy} onClick={onEdit} title="Details" className="inline-flex h-8 w-9 shrink-0 items-center justify-center rounded-lg border border-white/15 bg-white/10 text-white transition-colors hover:border-accent-400 hover:bg-accent-500/20"><Info className="h-4 w-4" /></button>
              <button disabled={busy} onClick={onDelete} title="Delete" className="inline-flex h-8 w-9 shrink-0 items-center justify-center rounded-lg border border-red-400/30 bg-red-500/15 text-red-200 transition-colors hover:border-red-300 hover:bg-red-500/25"><Trash2 className="h-4 w-4" /></button>
          {item.column === 'ready' && <button disabled={busy} onClick={onPrintNow} className="btn btn-primary col-span-4 text-xs py-1 px-2"><Play className="h-3.5 w-3.5 mr-1" />{needsRetry ? 'Retry' : 'Print'}</button>}
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div draggable onDragStart={onDragStart} onDragOver={e => { e.preventDefault(); onDragOver() }} onDrop={e => { e.preventDefault(); onDrop() }} onDragEnd={onDragEnd} className={cn('cursor-grab rounded-xl border border-surface-800 bg-surface-950/60 transition-colors hover:border-surface-700 active:cursor-grabbing', dragging && 'opacity-50', dragOver && !dragging && 'ring-2 ring-accent-500/60', compact ? 'p-2.5' : thumb ? 'p-3' : 'p-4')}>
      <div className={cn(thumb ? 'space-y-3 mb-3' : 'flex items-start gap-3', compact ? 'mb-2' : 'mb-3')}>
        <div className={cn('rounded-lg bg-surface-800 flex items-center justify-center shrink-0 overflow-hidden', compact ? 'w-16 h-12' : thumb ? 'w-full h-52' : 'w-14 h-14')}>
          {queueItem.thumbnail_file_id ? <img src={`/api/files/${queueItem.thumbnail_file_id}`} alt="G-code preview" className="w-full h-full object-cover" /> : <FileCode className="h-7 w-7 text-surface-500" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className={cn('flex items-center gap-2', compact ? 'mb-0.5' : 'mb-1')}>
            {queuePosition && <span className="inline-flex h-6 min-w-6 items-center justify-center rounded-full border border-emerald-500/50 bg-emerald-500/15 px-2 text-xs font-bold text-emerald-200">#{queuePosition}</span>}
            <span className={cn('badge', getStatusBadge(status))}>{status}</span>
            {needsRetry && <span className="badge border border-amber-500/40 bg-amber-500/15 text-amber-300">retry ready</span>}

            {!compact && <span className={cn('badge border', sourceBadge.className)}>{sourceBadge.label}</span>}
          </div>
          {editingName ? (
            <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(queueItem.display_name || queueItem.file_name); setEditingName(false) } }} className="input py-1 text-sm" />
          ) : (
            <button type="button" onClick={() => setEditingName(true)} className="block w-full min-w-0 overflow-hidden truncate text-left font-medium text-surface-100 hover:text-accent-300" title={queueItem.display_name || queueItem.file_name}>{queueItem.display_name || queueItem.file_name}</button>
          )}
          <div className="overflow-hidden truncate text-xs text-surface-500" title={queueItem.file_name}>{queueItem.file_name}</div>
        </div>
      </div>

      <div className={cn('grid grid-cols-2 text-xs', compact ? 'gap-1.5 mb-2' : 'gap-2 mb-3')}>
        <InfoPill icon={<Printer className="h-3 w-3" />} label={item.printer?.name || 'No printer'} tone={!item.printer ? 'warn' : undefined} />
        <InfoPill icon={<Clock className="h-3 w-3" />} label={queueItem.estimated_seconds ? formatDuration(queueItem.estimated_seconds) : 'No ETA'} />
        <InfoPill label={item.material ? `${item.material.type.toUpperCase()} ${item.material.color}` : queueItem.material_type || 'No material'} tone={!item.material && !queueItem.material_type ? 'warn' : undefined} />
        <InfoPill label={queueItem.filament_grams ? `${Math.round(queueItem.filament_grams)}g` : 'No grams'} />
      </div>

      {/* Quick assign printer/filament */}
      <div className={cn('grid grid-cols-2 gap-2', compact ? 'mb-2' : 'mb-3')}>
        <select className="input text-xs py-1" value={queueItem.assigned_printer_id || ''} onChange={async e => {
          const pid = e.target.value || undefined
          onQuickAssign({ assigned_printer_id: pid })
        }}>
          <option value="" disabled hidden>Printer</option>
          {printers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
        <select className="input text-xs py-1" value={queueItem.assigned_spool_id || ''} onChange={async e => {
          const sid = e.target.value || undefined
          const spool = spools.find(s => s.id === sid)
          if (spool && !isCompatibleSpool(spool)) return
          const patch: Partial<GCodeQueueItem> = { assigned_spool_id: sid || null }
          if (spool?.material) {
            patch.material_type = spool.material.type
            patch.material_color = spool.material.color_hex || spool.material.color
          }
          onQuickAssign(patch)
        }}>
          <option value="">Não definido</option>
          {spools.map(s => <option key={s.id} value={s.id} disabled={!isCompatibleSpool(s)}>{s.material ? `${s.material.type.toUpperCase()} · ${s.material.color || s.material.name}` : 'Unknown'} · {Math.round(s.remaining_weight)}g</option>)}
        </select>
      </div>

      {(status === 'printing' || status === 'paused') && (
        <div className="mb-3">
          <div className="flex items-center justify-between text-xs mb-1">
            <span className="text-surface-500">Print progress</span>
            <span className="text-surface-200 font-medium">{Math.round(queueItem.progress || 0)}%</span>
          </div>
          <div className="h-2 rounded-full bg-surface-800 overflow-hidden">
            <div className="h-full bg-accent-500 transition-all" style={{ width: `${Math.min(100, Math.max(0, queueItem.progress || 0))}%` }} />
          </div>
        </div>
      )}

      {!compact && (queueItem.nozzle_temp || queueItem.bed_temp || queueItem.layer_height) && (
        <div className="mb-3 text-xs text-surface-500">{queueItem.nozzle_temp ? `Nozzle ${queueItem.nozzle_temp}°C` : ''} {queueItem.bed_temp ? `Bed ${queueItem.bed_temp}°C` : ''} {queueItem.layer_height ? `Layer ${queueItem.layer_height}mm` : ''}</div>
      )}

      {item.blocked_by && item.blocked_by.length > 0 && <div className="mb-3 space-y-1">{item.blocked_by.slice(0, 3).map(reason => <div key={reason} className="flex items-center gap-1.5 text-xs text-amber-400"><AlertTriangle className="h-3 w-3" /> {reason}</div>)}</div>}
      {item.preflight?.warnings && item.preflight.warnings.length > 0 && <div className="mb-3 text-xs text-amber-400">{item.preflight.warnings[0]}</div>}

      <div className={cn('flex flex-wrap', compact ? 'gap-1.5' : 'gap-2')}>
        {item.column === 'ready' && <button disabled={busy} onClick={onPrintNow} className={cn('btn btn-primary text-xs', compact ? 'py-1 px-2' : 'py-1.5 px-3')}><Play className="h-3.5 w-3.5 mr-1" />{printActionLabel}</button>}

        {status === 'paused' && <button disabled={busy} onClick={onResume} className={cn('btn btn-secondary text-xs', compact ? 'py-1 px-2' : 'py-1.5 px-3')}>Resume</button>}
        {status === 'printing' && <button disabled={busy} onClick={onPause} className={cn('btn btn-secondary text-xs', compact ? 'py-1 px-2' : 'py-1.5 px-3')}>Pause</button>}
        {(status === 'printing' || status === 'paused') && <button disabled={busy} onClick={onCancel} className={cn('btn btn-secondary text-xs text-red-400', compact ? 'py-1 px-2' : 'py-1.5 px-3')}><XCircle className="h-3.5 w-3.5 mr-1" />Cancel</button>}
        <button disabled={busy} onClick={onEdit} className={cn('btn btn-ghost text-xs', compact ? 'py-1 px-2' : 'py-1.5 px-2')}><Info className="h-3.5 w-3.5 mr-1" />Details</button>
        <button disabled={busy} onClick={onDelete} className={cn('btn btn-ghost text-xs text-red-400', compact ? 'py-1 px-2' : 'py-1.5 px-2')}>Delete</button>
      </div>
      {busy && <div className="text-xs text-surface-500 mt-2">Working...</div>}
    </div>
  )
}

function InfoPill({ icon, label, tone }: { icon?: React.ReactNode; label: string; tone?: 'warn' }) {
  return <div className={cn('rounded-lg bg-surface-800/60 px-2 py-1.5 flex items-center gap-1.5 truncate', tone === 'warn' ? 'text-amber-400' : 'text-surface-300')}>{icon}<span className="truncate">{label}</span></div>
}

function SummaryCard({ label, value, tone }: { label: string; value: string | number; tone: string }) {
  return <div className="card p-4"><div className="text-xs uppercase tracking-wider text-surface-500 mb-1">{label}</div><div className={cn('text-xl font-semibold', tone)}>{value}</div></div>
}

function sortQueue(a: QueueItem, b: QueueItem) {
  const pa = a.item.priority ?? 0
  const pb = b.item.priority ?? 0
  if (pa !== pb) return pb - pa
  return new Date(a.item.created_at).getTime() - new Date(b.item.created_at).getTime()
}
