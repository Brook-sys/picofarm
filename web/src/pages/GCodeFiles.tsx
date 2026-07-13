import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, FileCode, Grid3X3, Info as InfoIcon, Link2Off, List, Loader2, Plus, Search, Star, Upload, Send } from 'lucide-react'
import { fileLibraryApi, gcodeLibraryApi, printersApi, slicerApi, stlLibraryApi } from '../api/client'
import AppToast, { type AppToastState } from '../components/AppToast'
import { STLViewer3D } from '../components/STLViewer3D'
import type { GCodeLibraryFile, STLLibraryFile, Tag } from '../types'
import { cn, formatDuration } from '../lib/utils'
import { renderSTLThumbnailWithTimeout } from '../lib/stlThumbnail'

const MATERIAL_OPTIONS = ['pla', 'petg', 'abs', 'asa', 'tpu'] as const

type SortMode = 'created_desc' | 'created_asc' | 'name_asc' | 'name_desc' | 'prints_desc' | 'time_asc' | 'grams_asc' | 'grams_desc'
type LibraryItem = { type: 'stl'; file: STLLibraryFile } | { type: 'gcode'; file: GCodeLibraryFile }

export default function GCodeFiles() {
  const [files, setFiles] = useState<GCodeLibraryFile[]>([])
  const [stlFiles, setSTLFiles] = useState<STLLibraryFile[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [printers, setPrinters] = useState<import('../types').Printer[]>([])
  const [tagFilter, setTagFilter] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [material, setMaterial] = useState('')
  const [printProfile, setPrintProfile] = useState('')
  const [filamentProfile, setFilamentProfile] = useState('')
  const [printerProfile, setPrinterProfile] = useState('')
  const [nozzle, setNozzle] = useState('')
  const [layer, setLayer] = useState('')
  const [timeBucket, setTimeBucket] = useState('')
  const [usage, setUsage] = useState('')
  const [sortMode, setSortMode] = useState<SortMode>('created_desc')
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [showFilters, setShowFilters] = useState(false)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const pageSize = 50
  const [uploading, setUploading] = useState(false)
  const [uploadStatus, setUploadStatus] = useState('')
  const [busy, setBusy] = useState('')
  const [confirmDelete, setConfirmDelete] = useState('')
  const [viewingFile, setViewingFile] = useState<GCodeLibraryFile | null>(null)
  const [sendingFile, setSendingFile] = useState<GCodeLibraryFile | null>(null)
  const [viewingSTL, setViewingSTL] = useState<STLLibraryFile | null>(null)
  const [slicingSTL, setSlicingSTL] = useState<STLLibraryFile | null>(null)
  const [toast, setToast] = useState<AppToastState | null>(null)

  const loadTags = async () => {
    try {
      setTags(await gcodeLibraryApi.listTags())
    } catch {
      // ignore
    }
  }

  const loadFiles = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await fileLibraryApi.get()
      const nextSTLs = res.stl_files || []
      const nextRootGCodes = res.root_gcode_files || []
      setSTLFiles(nextSTLs)
      setFiles(nextRootGCodes)
      setViewingSTL(current => current ? nextSTLs.find(file => file.id === current.id) || current : current)
      setSlicingSTL(current => current ? nextSTLs.find(file => file.id === current.id) || current : current)
      setViewingFile(current => current ? [...nextRootGCodes, ...nextSTLs.flatMap(file => file.gcodes || [])].find(file => file.id === current.id) || current : current)
      setTotal(nextSTLs.length + nextRootGCodes.length)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load files')
    } finally {
      setLoading(false)
    }
  }

  const showToast = (next: AppToastState) => {
    setToast(next)
    window.setTimeout(() => setToast(null), 3500)
  }

  useEffect(() => { loadTags() }, [])
  useEffect(() => { printersApi.list().then(setPrinters).catch(() => setPrinters([])) }, [])
  useEffect(() => { loadFiles() }, [])

  const allGCodeFiles = useMemo(() => [...files, ...stlFiles.flatMap(file => file.gcodes || [])], [files, stlFiles])
  const printProfileOptions = useMemo(() => Array.from(new Set(allGCodeFiles.map(f => f.metadata?.print_settings_id).filter(Boolean))).sort(), [allGCodeFiles])
  const filamentProfileOptions = useMemo(() => Array.from(new Set(allGCodeFiles.map(f => f.metadata?.filament_settings_id || f.filament_name).filter(Boolean))).sort(), [allGCodeFiles])
  const printerProfileOptions = useMemo(() => Array.from(new Set(allGCodeFiles.map(f => f.metadata?.printer_settings_id).filter(Boolean))).sort(), [allGCodeFiles])
  const nozzleOptions = useMemo(() => Array.from(new Set(allGCodeFiles.map(f => f.nozzle_diameter).filter((v): v is number => typeof v === 'number'))).sort((a, b) => a - b), [allGCodeFiles])
  const layerOptions = useMemo(() => Array.from(new Set(allGCodeFiles.map(f => f.layer_height).filter((v): v is number => typeof v === 'number'))).sort((a, b) => a - b), [allGCodeFiles])

  const activeFilters = [material, printProfile, filamentProfile, printerProfile, nozzle, layer, timeBucket, usage, tagFilter].filter(Boolean).length

  const getFilterChips = () => {
    const chips: { label: string; onRemove: () => void }[] = []
    if (material) chips.push({ label: `Material: ${material.toUpperCase()}`, onRemove: () => setMaterial('') })
    if (printProfile) chips.push({ label: `Impressão: ${printProfile}`, onRemove: () => setPrintProfile('') })
    if (filamentProfile) chips.push({ label: `Filamento: ${filamentProfile}`, onRemove: () => setFilamentProfile('') })
    if (printerProfile) chips.push({ label: `Impressora: ${printerProfile}`, onRemove: () => setPrinterProfile('') })
    if (nozzle) chips.push({ label: `Nozzle: ${nozzle}mm`, onRemove: () => setNozzle('') })
    if (layer) chips.push({ label: `Layer: ${layer}mm`, onRemove: () => setLayer('') })
    if (timeBucket) {
      const map: Record<string, string> = { lt_30: '<30m', '30_60': '30-60m', '1_3h': '1-3h', gt_3h: '3h+' }
      chips.push({ label: `Tempo: ${map[timeBucket]}`, onRemove: () => setTimeBucket('') })
    }
    if (usage) chips.push({ label: usage === 'never' ? 'Nunca impresso' : 'Já impresso', onRemove: () => setUsage('') })
    if (tagFilter) {
      const tagName = tags.find(t => t.id === tagFilter)?.name || 'Tag'
      chips.push({ label: `Tag: ${tagName}`, onRemove: () => setTagFilter('') })
    }
    return chips
  }

  const clearFilters = () => {
    setQuery('')
    setMaterial('')
    setPrintProfile('')
    setFilamentProfile('')
    setPrinterProfile('')
    setNozzle('')
    setLayer('')
    setTimeBucket('')
    setUsage('')
    setTagFilter('')
    setSortMode('created_desc')
    setPage(1)
  }

  const gcodeMatchesFilters = useCallback((file: GCodeLibraryFile) => {
    const q = query.trim().toLowerCase()
    if (material && file.material_type?.toLowerCase() !== material.toLowerCase()) return false
    if (printProfile && file.metadata?.print_settings_id !== printProfile) return false
    if (filamentProfile && (file.metadata?.filament_settings_id || file.filament_name) !== filamentProfile) return false
    if (printerProfile && file.metadata?.printer_settings_id !== printerProfile) return false
    if (nozzle && String(file.nozzle_diameter ?? '') !== nozzle) return false
    if (layer && String(file.layer_height ?? '') !== layer) return false
    if (usage === 'never' && file.print_count > 0) return false
    if (usage === 'printed' && file.print_count === 0) return false
    if (tagFilter && !(file.tags || []).some(t => t.id === tagFilter)) return false
    if (timeBucket) {
      const seconds = file.estimated_seconds ?? 0
      if (timeBucket === 'lt_30' && !(seconds > 0 && seconds < 1800)) return false
      if (timeBucket === '30_60' && !(seconds >= 1800 && seconds < 3600)) return false
      if (timeBucket === '1_3h' && !(seconds >= 3600 && seconds < 10800)) return false
      if (timeBucket === 'gt_3h' && seconds < 10800) return false
    }
    if (!q) return true
    const haystack = [file.display_name, file.file_name, file.material_type, file.filament_name, file.metadata?.print_settings_id, file.metadata?.filament_settings_id, file.metadata?.printer_settings_id].filter(Boolean).join(' ').toLowerCase()
    return haystack.includes(q)
  }, [query, material, printProfile, filamentProfile, printerProfile, nozzle, layer, usage, tagFilter, timeBucket])

  const hasActiveGCodeFilter = activeFilters > 0 || query.trim().length > 0

  const displayedSTLs = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!hasActiveGCodeFilter) return stlFiles
    return stlFiles.filter(file => {
      const selfQuery = q ? [file.display_name, file.file_name].filter(Boolean).join(' ').toLowerCase().includes(q) : true
      const selfTag = tagFilter ? (file.tags || []).some(tag => tag.id === tagFilter) : true
      const hasGCodeOnlyFilter = Boolean(material || printProfile || filamentProfile || printerProfile || nozzle || layer || timeBucket || usage)
      const self = !hasGCodeOnlyFilter && selfQuery && selfTag
      const child = (file.gcodes || []).some(gcodeMatchesFilters)
      return self || child
    })
  }, [stlFiles, query, tagFilter, material, printProfile, filamentProfile, printerProfile, nozzle, layer, timeBucket, usage, hasActiveGCodeFilter, gcodeMatchesFilters])

  const filtered = useMemo(() => {
    return files.filter(gcodeMatchesFilters).sort((a, b) => {
      switch (sortMode) {
        case 'created_asc': return new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
        case 'name_asc': return a.display_name.localeCompare(b.display_name)
        case 'name_desc': return b.display_name.localeCompare(a.display_name)
        case 'prints_desc': return b.print_count - a.print_count
        case 'time_asc': return (a.estimated_seconds ?? Number.MAX_SAFE_INTEGER) - (b.estimated_seconds ?? Number.MAX_SAFE_INTEGER)
        case 'grams_asc': return (a.filament_grams ?? Number.MAX_SAFE_INTEGER) - (b.filament_grams ?? Number.MAX_SAFE_INTEGER)
        case 'grams_desc': return (b.filament_grams ?? 0) - (a.filament_grams ?? 0)
        default: return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      }
    })
  }, [files, gcodeMatchesFilters, sortMode])

  const libraryItems = useMemo<LibraryItem[]>(() => {
    const items: LibraryItem[] = [
      ...displayedSTLs.map(file => ({ type: 'stl' as const, file })),
      ...filtered.map(file => ({ type: 'gcode' as const, file })),
    ]
    return items.sort((a, b) => {
      if (sortMode === 'name_asc') return a.file.display_name.localeCompare(b.file.display_name)
      if (sortMode === 'name_desc') return b.file.display_name.localeCompare(a.file.display_name)
      if (sortMode === 'created_asc') return new Date(a.file.created_at).getTime() - new Date(b.file.created_at).getTime()
      return new Date(b.file.created_at).getTime() - new Date(a.file.created_at).getTime()
    })
  }, [displayedSTLs, filtered, sortMode])

  const uploadFiles = async (selected: FileList | File[] | undefined | null, type: 'stl' | 'gcode', parentSTLId?: string | null) => {
    const selectedFiles = Array.from(selected || [])
    if (selectedFiles.length === 0) return
    setUploading(true)
    setError('')
    try {
      for (let i = 0; i < selectedFiles.length; i += 1) {
        const file = selectedFiles[i]
        const progress = selectedFiles.length > 1 ? ` (${i + 1}/${selectedFiles.length})` : ''
        if (type === 'stl') {
          setUploadStatus(`Generating preview${progress}...`)
          const thumbnail = await renderSTLThumbnailWithTimeout(file)
          setUploadStatus(`Uploading STL${progress}...`)
          await stlLibraryApi.upload(file, thumbnail)
        } else {
          setUploadStatus(`Uploading G-code${progress}...`)
          await gcodeLibraryApi.upload(file, parentSTLId)
        }
      }
      await loadFiles()
      showToast({ title: 'Upload complete', message: `${selectedFiles.length} file${selectedFiles.length === 1 ? '' : 's'} added to the library.`, tone: 'success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploadStatus('')
      setUploading(false)
    }
  }

  const setGCodeParent = async (gcodeId: string, parentSTLId: string | null) => {
    setBusy(gcodeId)
    setError('')
    try {
      await gcodeLibraryApi.setParentSTL(gcodeId, parentSTLId)
      await loadFiles()
      showToast({ title: parentSTLId ? 'G-code linked' : 'G-code moved to root', message: 'Library organization updated.', tone: 'success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to move G-code')
    } finally {
      setBusy('')
    }
  }

  const makeDefaultGCode = async (id: string) => {
    setBusy(id)
    try {
      await gcodeLibraryApi.setDefaultForSTL(id)
      await loadFiles()
      showToast({ title: 'Default G-code updated', message: 'This STL will use the selected G-code by default.', tone: 'success' })
    } finally {
      setBusy('')
    }
  }

  const addToQueue = async (id: string) => {
    setBusy(id)
    setError('')
    try {
      await gcodeLibraryApi.addToQueue(id)
      showToast({ title: 'Added to queue', message: 'G-code is ready in the print queue.', tone: 'success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add to queue')
    } finally {
      setBusy('')
    }
  }

  const sendToPrinter = async (file: GCodeLibraryFile, printerId: string, remotePath: string, startPrint: boolean) => {
    setBusy(file.id)
    setError('')
    try {
      const res = await gcodeLibraryApi.sendToPrinter(file.id, { printer_id: printerId, remote_path: remotePath, start_print: startPrint })
      setSendingFile(null)
      showToast({ title: startPrint ? 'Print started' : 'Sent to printer', message: `Remote file: ${res.remote_path}`, tone: 'success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send to printer')
    } finally {
      setBusy('')
    }
  }

  const renameSTLFile = async (file: STLLibraryFile, displayName: string) => {
    const next = displayName.trim()
    if (!next || next === file.display_name) return
    setBusy(file.id)
    setError('')
    try {
      await stlLibraryApi.update(file.id, { display_name: next })
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rename file')
    } finally {
      setBusy('')
    }
  }

  const retrySTLThumbnail = async (file: STLLibraryFile) => {
    setBusy(file.id)
    setError('')
    try {
      const response = await fetch(`/api/files/${file.file_id}`)
      if (!response.ok) throw new Error('Failed to load STL file')
      const blob = await response.blob()
      const stlFile = new File([blob], file.file_name || `${file.display_name}.stl`, { type: 'model/stl' })
      const thumbnail = await renderSTLThumbnailWithTimeout(stlFile)
      if (!thumbnail) throw new Error('Failed to generate thumbnail')
      await stlLibraryApi.updateThumbnail(file.id, thumbnail)
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate STL thumbnail')
    } finally {
      setBusy('')
    }
  }

  const deleteSTLFile = async (id: string) => {
    if (confirmDelete !== id) {
      setConfirmDelete(id)
      return
    }
    setBusy(id)
    setError('')
    try {
      await stlLibraryApi.delete(id)
      setConfirmDelete('')
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete file')
    } finally {
      setBusy('')
    }
  }

  const renameFile = async (file: GCodeLibraryFile, displayName: string) => {
    const next = displayName.trim()
    if (!next || next === file.display_name) return
    setBusy(file.id)
    setError('')
    try {
      await gcodeLibraryApi.update(file.id, { display_name: next })
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rename file')
    } finally {
      setBusy('')
    }
  }

  const createTag = async (name: string, color?: string) => {
    const next = name.trim()
    if (!next) return
    setError('')
    try {
      await gcodeLibraryApi.createTag({ name: next, color })
      await loadTags()
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create tag')
    }
  }

  const addTagToFile = async (fileId: string, tagId: string) => {
    if (!tagId) return
    setError('')
    try {
      await gcodeLibraryApi.addTag(fileId, tagId)
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add tag')
    }
  }

  const removeTagFromFile = async (fileId: string, tagId: string) => {
    setError('')
    try {
      await gcodeLibraryApi.removeTag(fileId, tagId)
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove tag')
    }
  }

  const addTagToSTL = async (fileId: string, tagId: string) => {
    if (!tagId) return
    setError('')
    try {
      await stlLibraryApi.addTag(fileId, tagId)
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add tag')
    }
  }

  const removeTagFromSTL = async (fileId: string, tagId: string) => {
    setError('')
    try {
      await stlLibraryApi.removeTag(fileId, tagId)
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove tag')
    }
  }

  const deleteFile = async (id: string) => {
    if (confirmDelete !== id) {
      setConfirmDelete(id)
      return
    }
    setBusy(id)
    setError('')
    try {
      await gcodeLibraryApi.delete(id)
      setConfirmDelete('')
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete file')
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8">
      <div className="flex items-start justify-between gap-4 mb-8">
        <div>
          <h1 className="text-3xl font-display font-bold text-surface-100 flex items-center gap-3">
            <FileCode className="h-8 w-8 text-accent-500" />
            Files
          </h1>
          <p className="text-surface-400 mt-1">Organize STL source files with linked printable G-code files</p>
        </div>
        <div className="flex gap-2">
          <label className="btn btn-secondary cursor-pointer">
            <Upload className="h-4 w-4 mr-2" />{uploading && uploadStatus ? uploadStatus : 'Upload STL'}
            <input type="file" accept=".stl" multiple className="hidden" disabled={uploading} onChange={e => uploadFiles(e.target.files, 'stl')} />
          </label>
          <label className="btn btn-primary cursor-pointer">
            <Upload className="h-4 w-4 mr-2" />{uploading && uploadStatus ? uploadStatus : 'Upload G-code'}
            <input type="file" accept=".gcode" multiple className="hidden" disabled={uploading} onChange={e => uploadFiles(e.target.files, 'gcode')} />
          </label>
        </div>
      </div>

      <div className="card p-2 mb-4">
        <div className="grid grid-cols-[minmax(220px,1fr)_auto_auto_auto] gap-2 items-center">
          <div className="relative min-w-0">
            <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
            <input value={query} onChange={e => setQuery(e.target.value)} className="input input-with-icon py-1.5 text-sm h-9 w-full" placeholder="Buscar arquivos..." />
          </div>

          <button onClick={() => setShowFilters(!showFilters)} className={cn('btn btn-secondary h-9 text-sm px-3 whitespace-nowrap', (activeFilters > 0 || showFilters) && 'border-accent-500/40 text-accent-300')}>
            Filtros{activeFilters > 0 ? ` (${activeFilters})` : ''}
          </button>

          <select value={sortMode} onChange={e => setSortMode(e.target.value as SortMode)} className="input w-36 py-1.5 text-xs h-9">
            <option value="created_desc">Recentes</option>
            <option value="created_asc">Antigos</option>
            <option value="name_asc">A-Z</option>
            <option value="name_desc">Z-A</option>
            <option value="prints_desc">Usados</option>
            <option value="time_asc">Tempo↑</option>
            <option value="grams_asc">Filamento↑</option>
            <option value="grams_desc">Filamento↓</option>
          </select>

          <div className="flex items-center border-l border-surface-800 pl-2">
            <button onClick={() => setViewMode('grid')} className={cn('btn btn-ghost px-2 py-1 h-9', viewMode === 'grid' && 'bg-surface-800 text-surface-100')}><Grid3X3 className="h-4 w-4" /></button>
            <button onClick={() => setViewMode('list')} className={cn('btn btn-ghost px-2 py-1 h-9', viewMode === 'list' && 'bg-surface-800 text-surface-100')}><List className="h-4 w-4" /></button>
          </div>
        </div>

        {activeFilters > 0 && !showFilters && (
          <div className="flex gap-1.5 mt-2 overflow-x-auto whitespace-nowrap pb-0.5">
            {getFilterChips().map(chip => (
              <span key={chip.label} className="inline-flex items-center gap-1 rounded-full bg-surface-800 px-2 py-0.5 text-[11px] text-surface-300 shrink-0">
                {chip.label}<button onClick={chip.onRemove} className="text-surface-500 hover:text-red-400">×</button>
              </span>
            ))}
            <button onClick={clearFilters} className="text-[11px] text-surface-500 hover:text-surface-300 px-1 shrink-0">limpar tudo</button>
          </div>
        )}

        {showFilters && (
          <div className="mt-2 pt-2 border-t border-surface-800">
            <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-9 gap-2">
              <Field label="Material"><select value={material} onChange={e => setMaterial(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{MATERIAL_OPTIONS.map(m => <option key={m} value={m}>{m.toUpperCase()}</option>)}</select></Field>
              <Field label="Perfil de Impressão"><select value={printProfile} onChange={e => setPrintProfile(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{printProfileOptions.map(p => <option key={p} value={p}>{p}</option>)}</select></Field>
              <Field label="Perfil de Filamento"><select value={filamentProfile} onChange={e => setFilamentProfile(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{filamentProfileOptions.map(p => <option key={p} value={p}>{p}</option>)}</select></Field>
              <Field label="Perfil de Impressora"><select value={printerProfile} onChange={e => setPrinterProfile(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{printerProfileOptions.map(p => <option key={p} value={p}>{p}</option>)}</select></Field>
              <Field label="Tag"><select value={tagFilter} onChange={e => setTagFilter(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{tags.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}</select></Field>
              <Field label="Nozzle"><select value={nozzle} onChange={e => setNozzle(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{nozzleOptions.map(n => <option key={n} value={String(n)}>{n}mm</option>)}</select></Field>
              <Field label="Layer"><select value={layer} onChange={e => setLayer(e.target.value)} className="input text-xs py-1"><option value="">Any</option>{layerOptions.map(l => <option key={l} value={String(l)}>{l}mm</option>)}</select></Field>
              <Field label="Tempo"><select value={timeBucket} onChange={e => setTimeBucket(e.target.value)} className="input text-xs py-1"><option value="">Any</option><option value="lt_30">&lt;30m</option><option value="30_60">30-60m</option><option value="1_3h">1-3h</option><option value="gt_3h">3h+</option></select></Field>
              <Field label="Uso"><select value={usage} onChange={e => setUsage(e.target.value)} className="input text-xs py-1"><option value="">Any</option><option value="never">Nunca impresso</option><option value="printed">Já impresso</option></select></Field>
            </div>
            <div className="flex justify-end gap-2 mt-2">
              <button onClick={clearFilters} className="btn btn-ghost text-xs">Limpar</button>
              <button onClick={() => setShowFilters(false)} className="btn btn-primary text-xs">Aplicar</button>
            </div>
          </div>
        )}
      </div>

      {error && <div className="mb-4 rounded-lg border border-red-500/20 bg-red-500/10 text-red-400 px-4 py-3 text-sm">{error}</div>}
      {toast && <AppToast toast={toast} onClose={() => setToast(null)} />}
      {viewingFile && <FileDetailsModal file={viewingFile} tags={tags} onCreateTag={createTag} onAddTag={addTagToFile} onRemoveTag={removeTagFromFile} onClose={() => setViewingFile(null)} />}
      {sendingFile && <SendToPrinterModal file={sendingFile} printers={printers} busy={busy === sendingFile.id} onClose={() => setSendingFile(null)} onSend={(printerId, remotePath, startPrint) => sendToPrinter(sendingFile, printerId, remotePath, startPrint)} />}
      {viewingSTL && <STLDetailsModal file={viewingSTL} tags={tags} onCreateTag={createTag} onAddTag={addTagToSTL} onRemoveTag={removeTagFromSTL} onClose={() => setViewingSTL(null)} onRetryThumbnail={() => retrySTLThumbnail(viewingSTL)} onSlice={() => setSlicingSTL(viewingSTL)} />}
      {slicingSTL && <SliceSTLModal file={slicingSTL} onClose={() => setSlicingSTL(null)} onDone={() => { setSlicingSTL(null); loadFiles() }} />}

      {loading ? <div className="text-surface-500">Loading files...</div> : libraryItems.length === 0 ? (
        <div className="text-center py-16">
          <FileCode className="h-16 w-16 mx-auto mb-4 text-surface-600" />
          <h3 className="text-xl font-semibold text-surface-300 mb-2">No stored files</h3>
          <p className="text-surface-500">Upload STL source files and G-code files to build your library.</p>
        </div>
      ) : (
        <div onDragOver={e => e.preventDefault()} onDrop={e => { e.preventDefault(); const id = e.dataTransfer.getData('text/gcode-id'); if (id) setGCodeParent(id, null) }} className="rounded-xl border border-dashed border-surface-800 p-3">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-surface-400">Library files</h2>
            <span className="text-xs text-surface-500">STL em azul · G-code em laranja · drop no vazio desvincula</span>
          </div>
          {viewMode === 'grid' ? (
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {libraryItems.map(item => item.type === 'stl'
                ? <STLFileCard key={`stl-${item.file.id}`} file={item.file} busy={busy === item.file.id} confirmDelete={confirmDelete === item.file.id} onRename={(name) => renameSTLFile(item.file, name)} onDelete={() => deleteSTLFile(item.file.id)} onDropGCode={(gcodeId) => setGCodeParent(gcodeId, item.file.id)} onUploadGCode={(gcodes) => uploadFiles(gcodes, 'gcode', item.file.id)} onViewGCode={setViewingFile} onRenameGCode={renameFile} onAddGCode={addToQueue} onSendGCode={setSendingFile} onUnlinkGCode={(gcodeId) => setGCodeParent(gcodeId, null)} onMakeDefaultGCode={makeDefaultGCode} onRetryThumbnail={() => retrySTLThumbnail(item.file)} onViewSTL={() => setViewingSTL(item.file)} onSlice={() => setSlicingSTL(item.file)} isGCodeMatch={(gcode) => hasActiveGCodeFilter && gcodeMatchesFilters(gcode)} />
                : <FileCard key={`gcode-${item.file.id}`} file={item.file} busy={busy === item.file.id} confirmDelete={confirmDelete === item.file.id} onView={() => setViewingFile(item.file)} onRename={(name) => renameFile(item.file, name)} onAdd={() => addToQueue(item.file.id)} onSend={() => setSendingFile(item.file)} onDelete={() => deleteFile(item.file.id)} />)}
            </div>
          ) : (
            <div className="card overflow-hidden">
              <div className="grid grid-cols-[56px_minmax(220px,1.7fr)_0.7fr_minmax(120px,1fr)_minmax(130px,1fr)_0.8fr_0.7fr_220px] gap-3 px-4 py-2 text-xs text-surface-500 border-b border-surface-800">
                <span>Preview</span><span>Name</span><span>Type</span><span>Link</span><span>Material / Profile</span><span>Size / Time</span><span>Usage</span><span>Ações</span>
              </div>
              {libraryItems.map(item => item.type === 'stl'
                ? <div key={`stl-group-${item.file.id}`}>
                    <STLFileRow file={item.file} busy={busy === item.file.id} confirmDelete={confirmDelete === item.file.id} onView={() => setViewingSTL(item.file)} onRename={(name) => renameSTLFile(item.file, name)} onDelete={() => deleteSTLFile(item.file.id)} onRetryThumbnail={() => retrySTLThumbnail(item.file)} onDropGCode={(gcodeId) => setGCodeParent(gcodeId, item.file.id)} onSlice={() => setSlicingSTL(item.file)} />
                    {(item.file.gcodes || []).map(gcode => <FileRow key={`linked-gcode-${gcode.id}`} file={gcode} busy={busy === gcode.id} confirmDelete={confirmDelete === gcode.id} indented matched={hasActiveGCodeFilter && gcodeMatchesFilters(gcode)} parentName={item.file.display_name} onView={() => setViewingFile(gcode)} onRename={(name) => renameFile(gcode, name)} onAdd={() => addToQueue(gcode.id)} onSend={() => setSendingFile(gcode)} onDelete={() => deleteFile(gcode.id)} />)}
                  </div>
                : <FileRow key={`gcode-${item.file.id}`} file={item.file} busy={busy === item.file.id} confirmDelete={confirmDelete === item.file.id} matched={hasActiveGCodeFilter} onView={() => setViewingFile(item.file)} onRename={(name) => renameFile(item.file, name)} onAdd={() => addToQueue(item.file.id)} onSend={() => setSendingFile(item.file)} onDelete={() => deleteFile(item.file.id)} />)}
            </div>
          )}
        </div>
      )}

      {total > pageSize && (
        <div className="flex items-center justify-between mt-4 text-sm text-surface-400">
          <span>{libraryItems.length} of {total} files · page {page}</span>
          <div className="flex gap-2">
            <button className="btn btn-secondary text-sm" disabled={page === 1} onClick={() => setPage(p => Math.max(1, p - 1))}>Previous</button>
            <button className="btn btn-secondary text-sm" disabled={page * pageSize >= total} onClick={() => setPage(p => p + 1)}>Next</button>
          </div>
        </div>
      )}
    </div>
  )
}

function SliceSTLModal({ file, onClose, onDone }: { file: STLLibraryFile; onClose: () => void; onDone: () => void }) {
  const [profiles, setProfiles] = useState<Record<'printers' | 'presets' | 'filaments', Array<Record<string, unknown>>>>({ printers: [], presets: [], filaments: [] })
  const [form, setForm] = useState({ printer: '', preset: '', filament: '', arrange: true, orient: true, enable_support: false, brim_type: false, print_sequence_by_object: false, display_name: `${file.display_name || file.file_name}.gcode` })
  const [busy, setBusy] = useState('')
  const [message, setMessage] = useState('')
  const [previewResult, setPreviewResult] = useState<{ usesSupport: boolean; printTime: number; filamentUsedG: number; filamentUsedMm: number; thumbnail: string } | null>(null)

  useEffect(() => {
    Promise.all([slicerApi.getConfig(), slicerApi.profiles('printers'), slicerApi.profiles('presets'), slicerApi.profiles('filaments')]).then(([config, printers, presets, filaments]) => {
      setProfiles({ printers, presets, filaments })
      const defaults = config.default_profiles || {}
      setForm(current => ({ ...current, printer: defaults.printers || String(printers.find(p => p.default)?.name || printers[0]?.name || ''), preset: defaults.presets || String(presets.find(p => p.default)?.name || presets[0]?.name || ''), filament: defaults.filaments || String(filaments.find(p => p.default)?.name || filaments[0]?.name || '') }))
    }).catch(err => setMessage(err instanceof Error ? err.message : 'Failed to load slicer profiles'))
  }, [])

  const generatePreview = async () => {
    setBusy('preview')
    setMessage('Generating preview. Large models can take a while...')
    setPreviewResult(null)
    try {
      const result = await slicerApi.preview({ stl_file_id: file.id, printer: form.printer, preset: form.preset, filament: form.filament, arrange: form.arrange, orient: form.orient, enable_support: form.enable_support, brim_type: form.brim_type, print_sequence_by_object: form.print_sequence_by_object })
      setPreviewResult(result)
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to generate preview')
    } finally {
      setBusy('')
    }
  }

  const slice = async () => {
    setBusy('slice')
    setMessage('Slicing in Orca container. This can take a while...')
    try {
      await slicerApi.sliceSTL({ stl_file_id: file.id, ...form, export_type: 'gcode' })
      setMessage('G-code generated and linked to STL.')
      onDone()
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to slice STL')
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
      <div className="card p-6 w-full max-w-4xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4"><div><h2 className="text-xl font-semibold text-surface-100">Fatiar STL com Orca</h2><p className="text-sm text-surface-500 truncate">{file.display_name}</p></div><button onClick={onClose} className="btn btn-ghost text-sm">Close</button></div>
        {message && <div className="mb-4 rounded-lg border border-surface-800 bg-surface-900/70 p-3 text-sm text-surface-300">{message}</div>}
        <div className="grid gap-5 md:grid-cols-[1fr_340px]">
          <div className="space-y-4">
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">Printer profile</span><select className="input" value={form.printer} onChange={e => setForm({ ...form, printer: e.target.value })}>{profiles.printers.map(p => <option key={String(p.name)} value={String(p.name)}>{String(p.name)}</option>)}</select></label>
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">Preset profile</span><select className="input" value={form.preset} onChange={e => setForm({ ...form, preset: e.target.value })}>{profiles.presets.map(p => <option key={String(p.name)} value={String(p.name)}>{String(p.name)}</option>)}</select></label>
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">Filament profile</span><select className="input" value={form.filament} onChange={e => setForm({ ...form, filament: e.target.value })}>{profiles.filaments.map(p => <option key={String(p.name)} value={String(p.name)}>{String(p.name)}</option>)}</select></label>
            <label className="block"><span className="text-xs text-surface-500 mb-1 block">G-code name</span><input className="input" value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })} /></label>
            <div className="rounded-xl border border-surface-800 bg-surface-900/60 p-4"><div className="text-xs font-medium text-surface-400 mb-3">Opções de fatiamento</div><div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm"><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.arrange} onChange={e => setForm({ ...form, arrange: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>Auto-arrange</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.orient} onChange={e => setForm({ ...form, orient: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>Auto-orient</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.enable_support} onChange={e => setForm({ ...form, enable_support: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>Suporte</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.brim_type} onChange={e => setForm({ ...form, brim_type: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>Borda</span></label><label className="inline-flex items-center gap-3 cursor-pointer select-none"><input type="checkbox" className="sr-only peer" checked={form.print_sequence_by_object} onChange={e => setForm({ ...form, print_sequence_by_object: e.target.checked })} /><span className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full bg-surface-700 transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500/50 peer-checked:bg-accent-500 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-all peer-checked:after:translate-x-5" /><span>Sequência por objeto</span></label>{form.print_sequence_by_object && <div className="col-span-2 mt-1 text-xs text-amber-400">Atenção: Pode causar colisão</div>}</div></div>
            <div className="flex gap-2"><button className="btn btn-secondary" disabled={!!busy} onClick={generatePreview} type="button">{busy === 'preview' ? <><Loader2 className="h-4 w-4 mr-2 animate-spin" />Gerando preview...</> : 'Gerar Preview'}</button><button className="btn btn-primary" disabled={!!busy || !form.printer || !form.preset || !form.filament} onClick={slice} type="button">{busy === 'slice' ? <><Loader2 className="h-4 w-4 mr-2 animate-spin" />Gerando G-code...</> : 'Gerar G-code'}</button></div>
          </div>
          <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-3"><div className="mb-2 text-sm font-medium text-surface-200">Preview</div>{busy === 'preview' ? <SlicerLoadingState title="Gerando preview" description="Preparando o modelo e renderizando a prévia. Arquivos grandes podem demorar alguns minutos." /> : busy === 'slice' ? <SlicerLoadingState title="Gerando G-code" description="O Orca está fatiando o STL no container. Não feche esta janela até o processo terminar." /> : previewResult ? <div className="space-y-3"><img src={`data:image/png;base64,${previewResult.thumbnail}`} alt="Preview" className="rounded border border-surface-800" /><div className="text-xs space-y-1 text-surface-300"><div>Tempo: {Math.floor(previewResult.printTime / 3600)}h {Math.floor((previewResult.printTime % 3600) / 60)}m</div><div>Filamento: {previewResult.filamentUsedG.toFixed(1)} g / {previewResult.filamentUsedMm.toFixed(0)} mm</div>{previewResult.usesSupport && <div className="text-emerald-400">Usa suporte</div>}{form.print_sequence_by_object && <div className="text-amber-400">Atenção: Pode causar colisão</div>}</div></div> : <div className="flex h-64 items-center justify-center rounded-lg border border-dashed border-surface-700 bg-surface-950/40 text-sm text-surface-600">Clique em Gerar Preview</div>}</div>
        </div>
      </div>
    </div>
  )
}

function SlicerLoadingState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-64 flex-col items-center justify-center rounded-lg border border-accent-500/30 bg-accent-500/10 px-6 text-center">
      <Loader2 className="mb-4 h-10 w-10 animate-spin text-accent-300" />
      <div className="text-base font-semibold text-surface-100">{title}</div>
      <p className="mt-2 max-w-xs text-sm leading-5 text-surface-400">{description}</p>
      <div className="mt-5 h-1.5 w-48 overflow-hidden rounded-full bg-surface-800">
        <div className="h-full w-1/2 animate-pulse rounded-full bg-accent-400" />
      </div>
    </div>
  )
}

function STLDetailsModal({ file, tags, onCreateTag, onAddTag, onRemoveTag, onClose, onSlice }: { file: STLLibraryFile; tags: Tag[]; onCreateTag: (name: string, color?: string) => void; onAddTag: (fileId: string, tagId: string) => void; onRemoveTag: (fileId: string, tagId: string) => void; onClose: () => void; onRetryThumbnail: () => void; onSlice: () => void }) {
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
      <div className="card p-6 w-full max-w-3xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-surface-100">STL Details</h2>
          <button onClick={onClose} className="btn btn-ghost text-sm">Close</button>
        </div>
        <div className="grid gap-5 md:grid-cols-[280px_1fr]">
          <div className="h-72 rounded-xl border border-surface-800 bg-surface-800 flex flex-col overflow-hidden">
            <STLViewer3D url={`/api/files/${file.file_id}`} />
          </div>
          <div className="space-y-4">
            <div><span className="text-xs text-surface-500">Display name</span><div className="text-surface-100 text-lg font-medium">{file.display_name}</div></div>
            <div><span className="text-xs text-surface-500">Source file</span><div className="text-surface-100 break-all">{file.file_name}</div></div>
            <div className="grid grid-cols-2 gap-3">
              <div><span className="text-xs text-surface-500">Size</span><div className="text-surface-100">{formatBytes(file.size_bytes)}</div></div>
              <div><span className="text-xs text-surface-500">Linked G-code</span><div className="text-surface-100">{file.gcodes?.length || 0}</div></div>
              <div><span className="text-xs text-surface-500">Created</span><div className="text-surface-100">{new Date(file.created_at).toLocaleString()}</div></div>
              <div><span className="text-xs text-surface-500">Updated</span><div className="text-surface-100">{new Date(file.updated_at).toLocaleString()}</div></div>
            </div>
            <FileTagManager itemId={file.id} currentTags={file.tags} tags={tags} onCreateTag={onCreateTag} onAddTag={onAddTag} onRemoveTag={onRemoveTag} />
            <div className="flex gap-2">
              <button onClick={onSlice} className="btn btn-primary w-fit">Fatiar</button>
              <a href={`/api/files/${file.file_id}`} target="_blank" rel="noreferrer" className="btn btn-secondary w-fit">Open STL</a>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function STLFileCard({ file, busy, confirmDelete, onRename, onDelete, onDropGCode, onUploadGCode, onViewGCode, onRenameGCode, onAddGCode, onSendGCode, onUnlinkGCode, onMakeDefaultGCode, onRetryThumbnail, onViewSTL, onSlice, isGCodeMatch }: { file: STLLibraryFile; busy: boolean; confirmDelete: boolean; onRename: (name: string) => void; onDelete: () => void; onDropGCode: (gcodeId: string) => void; onUploadGCode: (files?: FileList | null) => void; onViewGCode: (file: GCodeLibraryFile) => void; onRenameGCode: (file: GCodeLibraryFile, name: string) => void; onAddGCode: (id: string) => void; onSendGCode: (file: GCodeLibraryFile) => void; onUnlinkGCode: (id: string) => void; onMakeDefaultGCode: (id: string) => void; onRetryThumbnail: () => void; onViewSTL: () => void; onSlice: () => void; isGCodeMatch: (file: GCodeLibraryFile) => boolean }) {
  const [editingName, setEditingName] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const [name, setName] = useState(file.display_name)
  const children = file.gcodes || []
  const visibleChildren = expanded ? children : children.slice(0, 3)
  const saveName = () => {
    setEditingName(false)
    onRename(name)
  }
  return (
    <div onDragOver={e => { e.preventDefault(); setDragOver(true) }} onDragLeave={() => setDragOver(false)} onDrop={e => { e.preventDefault(); e.stopPropagation(); setDragOver(false); const id = e.dataTransfer.getData('text/gcode-id'); if (id) onDropGCode(id) }} className={cn('card relative overflow-hidden p-4 border-sky-500/45 bg-sky-500/[0.075] shadow-[0_0_0_1px_rgba(14,165,233,0.08)] hover:border-sky-400/75 hover:bg-sky-500/[0.11] transition-colors before:absolute before:inset-y-0 before:left-0 before:w-1 before:bg-sky-400', dragOver && 'border-sky-400/90 bg-sky-500/15')}>
      <div className="flex gap-3 mb-3 pl-1">
        <div className="w-16 h-16 rounded-lg border border-sky-400/35 bg-sky-500/10 flex items-center justify-center shrink-0 overflow-hidden">
          {file.thumbnail_file_id ? <img src={`/api/files/${file.thumbnail_file_id}`} alt="STL preview" className="h-full w-full object-contain" /> : <button type="button" disabled={busy} onClick={onRetryThumbnail} className="flex h-full w-full flex-col items-center justify-center text-[10px] text-sky-300 hover:bg-sky-500/10 disabled:opacity-60"><Upload className="mb-1 h-4 w-4" />Retry</button>}
        </div>
        <div className="min-w-0 flex-1">
          {editingName ? (
            <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(file.display_name); setEditingName(false) } }} className="input py-1 text-sm" />
          ) : (
            <button type="button" onClick={() => setEditingName(true)} title={file.display_name} className="font-medium text-sky-100 truncate block max-w-full text-left hover:text-sky-300">{file.display_name}</button>
          )}
          <div className="mb-1 inline-flex rounded-full border border-sky-500/30 bg-sky-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-sky-300">STL</div>
          <div className="text-xs text-surface-500 truncate" title={file.file_name}>{file.file_name}</div>
          <div className="text-xs text-surface-500 mt-1">{formatBytes(file.size_bytes)} · {children.length} G-code{children.length === 1 ? '' : 's'}</div>
          <TagChips tags={file.tags} />
        </div>
      </div>
      <div className="mb-3 rounded-lg border border-dashed border-sky-500/25 bg-surface-950/60 p-2">
        <div className="mb-2 flex items-center justify-between text-xs text-surface-500">
          <span>Linked G-code</span>
          <label className="cursor-pointer text-accent-300 hover:text-accent-200">
            Upload here
            <input type="file" accept=".gcode" multiple className="hidden" onChange={e => onUploadGCode(e.target.files)} />
          </label>
        </div>
        {visibleChildren.length === 0 ? <div className="py-4 text-center text-xs text-surface-600">Drop G-code here to link</div> : <div className="space-y-2">{visibleChildren.map(gcode => <LinkedGCodeRow key={gcode.id} file={gcode} matched={isGCodeMatch(gcode)} onView={() => onViewGCode(gcode)} onRename={(name) => onRenameGCode(gcode, name)} onAdd={() => onAddGCode(gcode.id)} onSend={() => onSendGCode(gcode)} onUnlink={() => onUnlinkGCode(gcode.id)} onMakeDefault={() => onMakeDefaultGCode(gcode.id)} />)}</div>}
        {children.length > 3 && <button onClick={() => setExpanded(v => !v)} className="mt-2 text-xs text-accent-300 hover:text-accent-200">{expanded ? 'Show less' : `+${children.length - 3} more`}</button>}
      </div>
      <div className="flex gap-2">
        <button onClick={onViewSTL} className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-surface-700 bg-surface-800 text-surface-200 hover:border-accent-500/60 hover:bg-accent-500/15 hover:text-accent-200" title="Details"><InfoIcon className="h-4 w-4" /></button>
        <button onClick={onSlice} className="btn btn-primary text-sm">Fatiar</button>
        <a href={`/api/files/${file.file_id}`} target="_blank" rel="noreferrer" className="btn btn-secondary text-sm flex-1">Open STL</a>
        <button disabled={busy} onClick={onDelete} className={cn('btn text-sm', confirmDelete ? 'border-red-500/50 bg-red-500/20 text-red-200' : 'btn-ghost text-red-400')}>{confirmDelete ? 'Confirm?' : 'Delete'}</button>
      </div>
    </div>
  )
}

function LinkedGCodeRow({ file, matched, onView, onRename, onAdd, onSend, onUnlink, onMakeDefault }: { file: GCodeLibraryFile; matched: boolean; onView: () => void; onRename: (name: string) => void; onAdd: () => void; onSend: () => void; onUnlink: () => void; onMakeDefault: () => void }) {
  const [editingName, setEditingName] = useState(false)
  const [name, setName] = useState(file.display_name)
  const saveName = () => {
    setEditingName(false)
    onRename(name)
  }
  const actionClass = 'inline-flex h-8 w-8 items-center justify-center rounded-lg border border-surface-700/80 bg-surface-900/80 text-surface-300 transition hover:border-accent-500/60 hover:bg-accent-500/15 hover:text-accent-200'
  return <div draggable={!editingName} onDragStart={e => e.dataTransfer.setData('text/gcode-id', file.id)} className={cn('group flex items-center gap-3 rounded-xl border border-accent-500/25 border-l-4 border-l-accent-400 bg-gradient-to-r from-accent-500/[0.12] to-surface-900/75 p-3 text-xs shadow-sm transition hover:border-accent-400/45 hover:from-accent-500/[0.16] cursor-grab', editingName && 'cursor-default', matched && 'ring-1 ring-emerald-400/60 from-emerald-500/15')}>
    <div className="h-10 w-10 rounded-lg border border-surface-700 bg-surface-800 overflow-hidden shrink-0 flex items-center justify-center shadow-inner">{file.thumbnail_file_id ? <img src={`/api/files/${file.thumbnail_file_id}`} className="h-full w-full object-cover" /> : <FileCode className="h-5 w-5 text-accent-300" />}</div>
    <div className="min-w-0 flex-1">
      {editingName ? <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(file.display_name); setEditingName(false) } }} className="input py-1 text-xs" /> : <div className="flex items-center gap-2"><button type="button" onClick={() => setEditingName(true)} className="max-w-full truncate text-left font-medium text-surface-100 hover:text-accent-300" title="Rename G-code">{file.display_name}</button>{file.default_for_stl && <span className="rounded-full border border-emerald-500/40 bg-emerald-500/15 px-2 py-0.5 text-[10px] font-bold uppercase text-emerald-300">Default</span>}{matched && <span className="rounded-full border border-emerald-500/30 bg-emerald-500/15 px-2 py-0.5 text-[10px] font-semibold uppercase text-emerald-300">Match</span>}</div>}
      <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-surface-500">
        <span className="truncate">{file.metadata?.print_settings_id || 'No profile'}</span>
        <span>Layer {file.layer_height ? `${file.layer_height}mm` : '—'}</span>
        <span>{file.estimated_seconds ? formatDuration(file.estimated_seconds) : 'No ETA'}</span>
      </div>
    </div>
    <div className="flex shrink-0 items-center gap-1.5">
      {!file.default_for_stl && <button onClick={onMakeDefault} className={actionClass} title="Set this G-code as the default for this STL"><Star className="h-4 w-4" /></button>}
      <button onClick={onView} className={actionClass} title="View G-code details"><InfoIcon className="h-4 w-4" /></button>
      <button onClick={onAdd} className={actionClass} title="Add this G-code to the print queue"><Plus className="h-4 w-4" /></button>
      <button onClick={onSend} className={actionClass} title="Send this G-code to printer storage"><Send className="h-4 w-4" /></button>
      <button onClick={onUnlink} className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-red-500/30 bg-red-500/10 text-red-300 transition hover:border-red-400/60 hover:bg-red-500/20 hover:text-red-200" title="Unlink this G-code from the STL"><Link2Off className="h-4 w-4" /></button>
    </div>
  </div>
}

function TagChips({ tags, limit = 3 }: { tags?: Tag[]; limit?: number }) {
  const visible = (tags || []).slice(0, limit)
  if (visible.length === 0) return null
  const extra = (tags || []).length - visible.length
  return (
    <div className="mt-2 flex flex-wrap gap-1.5">
      {visible.map(tag => (
        <span key={tag.id} className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium" style={{ backgroundColor: tag.color + '16', borderColor: tag.color + '44', color: tag.color }}>
          <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: tag.color }} />
          {tag.name}
        </span>
      ))}
      {extra > 0 && <span className="rounded-full border border-surface-700 bg-surface-800 px-2 py-0.5 text-[10px] text-surface-400">+{extra}</span>}
    </div>
  )
}

function SendToPrinterModal({ file, printers, busy, onClose, onSend }: { file: GCodeLibraryFile; printers: import('../types').Printer[]; busy: boolean; onClose: () => void; onSend: (printerId: string, remotePath: string, startPrint: boolean) => void }) {
  const moonrakerPrinters = printers.filter(printer => printer.connection_type === 'moonraker')
  const [printerId, setPrinterId] = useState(moonrakerPrinters[0]?.id || '')
  const [remotePath, setRemotePath] = useState('')
  const [startPrint, setStartPrint] = useState(false)
  const [printerStatus, setPrinterStatus] = useState<string>('')
  const selectedPrinter = moonrakerPrinters.find(printer => printer.id === printerId)
  const isOffline = printerStatus === 'offline'

  useEffect(() => {
    let active = true
    if (!printerId) {
      const timeout = window.setTimeout(() => {
        if (active) setPrinterStatus('')
      }, 0)
      return () => {
        active = false
        window.clearTimeout(timeout)
      }
    }
    printersApi.getState(printerId).then(state => {
      if (active) setPrinterStatus(state.status)
    }).catch(() => {
      if (active) setPrinterStatus('offline')
    })
    return () => { active = false }
  }, [printerId])

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
      <div className="card p-6 w-full max-w-lg">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-surface-100">Send to Printer</h2>
          <button onClick={onClose} className="btn btn-ghost text-sm">Close</button>
        </div>
        <div className="space-y-4">
          <div>
            <div className="text-sm font-medium text-surface-100">{file.display_name}</div>
            <div className="text-xs text-surface-500">{file.file_name}</div>
          </div>
          <label className="block">
            <span className="text-xs text-surface-500 mb-1 block">Moonraker printer</span>
            <select className="input" value={printerId} onChange={e => setPrinterId(e.target.value)}>
              {moonrakerPrinters.length === 0 && <option value="">No Moonraker printers available</option>}
              {moonrakerPrinters.map(printer => <option key={printer.id} value={printer.id}>{printer.name}</option>)}
            </select>
          </label>
          {selectedPrinter && (
            <div className={cn('rounded-lg border p-3 text-sm', isOffline ? 'border-red-500/30 bg-red-500/10 text-red-200' : 'border-surface-800 bg-surface-900/70 text-surface-400')}>
              <div className="flex items-center gap-2">
                {isOffline && <AlertTriangle className="h-4 w-4 text-red-300" />}
                <span>{isOffline ? `${selectedPrinter.name} is offline. Sending is disabled until it comes back online.` : `${selectedPrinter.name} is ${printerStatus || 'checking status...'}.`}</span>
              </div>
            </div>
          )}
          <label className="block">
            <span className="text-xs text-surface-500 mb-1 block">Remote folder under gcodes</span>
            <input className="input" value={remotePath} onChange={e => setRemotePath(e.target.value)} placeholder="Optional, e.g. projects/calibration" />
          </label>
          <label className="flex items-center gap-3 rounded-lg border border-surface-800 bg-surface-900/70 p-3">
            <input type="checkbox" checked={startPrint} onChange={e => setStartPrint(e.target.checked)} />
            <span>
              <span className="block text-sm text-surface-200">Start print immediately</span>
              <span className="text-xs text-surface-500">Uploads the file and then starts it on the printer.</span>
            </span>
          </label>
          <div className="flex justify-end gap-2">
            <button onClick={onClose} className="btn btn-secondary">Cancel</button>
            <button disabled={!printerId || busy || isOffline} onClick={() => onSend(printerId, remotePath, startPrint)} className="btn btn-primary">
              <Send className="h-4 w-4 mr-2" />{busy ? 'Sending...' : startPrint ? 'Send & Print' : 'Send'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function FileTagManager({ itemId, currentTags, tags, onCreateTag, onAddTag, onRemoveTag }: { itemId: string; currentTags?: Tag[]; tags: Tag[]; onCreateTag: (name: string, color?: string) => void; onAddTag: (fileId: string, tagId: string) => void; onRemoveTag: (fileId: string, tagId: string) => void }) {
  const [query, setQuery] = useState('')
  const [color, setColor] = useState('#1883FF')
  const current = currentTags || []
  const available = tags.filter(tag => !current.some(active => active.id === tag.id) && tag.name.toLowerCase().includes(query.trim().toLowerCase()))
  const exactMatch = tags.some(tag => tag.name.toLowerCase() === query.trim().toLowerCase())
  const canCreate = query.trim().length > 0 && !exactMatch

  return (
    <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-3">
      <div className="mb-3 flex items-center justify-between gap-2">
        <div>
          <div className="text-sm font-medium text-surface-200">Tags</div>
          <div className="text-xs text-surface-500">Organize root files with reusable labels</div>
        </div>
        <span className="rounded-full bg-surface-800 px-2 py-0.5 text-xs text-surface-400">{current.length}</span>
      </div>

      <div className="mb-3 flex min-h-10 flex-wrap gap-2 rounded-lg border border-surface-800 bg-surface-950/40 p-2">
        {current.length === 0 && <span className="text-sm text-surface-600">No tags yet</span>}
        {current.map(tag => (
          <span key={tag.id} className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium" style={{ backgroundColor: tag.color + '18', borderColor: tag.color + '55', color: tag.color }}>
            <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: tag.color }} />
            {tag.name}
            <button onClick={() => onRemoveTag(itemId, tag.id)} className="ml-1 text-surface-400 hover:text-red-300">×</button>
          </span>
        ))}
      </div>

      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-500" />
          <input value={query} onChange={e => setQuery(e.target.value)} className="input input-with-icon text-sm" placeholder="Search or create tag..." />
        </div>
        <input type="color" value={color} onChange={e => setColor(e.target.value)} className="h-10 w-12 cursor-pointer rounded-lg border border-surface-700 bg-surface-900 p-1" title="Tag color" />
      </div>

      {(available.length > 0 || canCreate) && (
        <div className="mt-2 flex flex-wrap gap-2">
          {available.slice(0, 8).map(tag => (
            <button key={tag.id} onClick={() => { onAddTag(itemId, tag.id); setQuery('') }} className="inline-flex items-center gap-1.5 rounded-full border border-surface-700 bg-surface-800 px-2.5 py-1 text-xs text-surface-200 hover:border-accent-500/50 hover:bg-surface-700">
              <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: tag.color }} />
              {tag.name}
            </button>
          ))}
          {canCreate && (
            <button onClick={() => { onCreateTag(query, color); setQuery('') }} className="inline-flex items-center gap-1.5 rounded-full border border-accent-500/40 bg-accent-500/10 px-2.5 py-1 text-xs text-accent-200 hover:bg-accent-500/20">
              <Plus className="h-3 w-3" /> Create “{query.trim()}”
            </button>
          )}
        </div>
      )}
    </div>
  )
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block"><span className="text-[10px] uppercase tracking-wide text-surface-500 mb-1 block">{label}</span>{children}</label>
}

function FileCard({ file, busy, confirmDelete, onView, onRename, onAdd, onSend, onDelete }: { file: GCodeLibraryFile; busy: boolean; confirmDelete: boolean; onView: () => void; onRename: (name: string) => void; onAdd: () => void; onSend: () => void; onDelete: () => void }) {
  const [editingName, setEditingName] = useState(false)
  const [name, setName] = useState(file.display_name)
  const saveName = () => {
    setEditingName(false)
    onRename(name)
  }
  return (
    <div draggable onDragStart={e => e.dataTransfer.setData('text/gcode-id', file.id)} className="card relative overflow-hidden p-4 border-accent-500/45 bg-accent-500/[0.075] shadow-[0_0_0_1px_rgba(249,115,22,0.08)] hover:border-accent-400/75 hover:bg-accent-500/[0.11] transition-colors cursor-grab before:absolute before:inset-y-0 before:left-0 before:w-1 before:bg-accent-400">
      <div className="flex gap-3 mb-3 pl-1">
        <div className="w-16 h-16 rounded-lg border border-accent-400/35 bg-accent-500/10 flex items-center justify-center shrink-0 overflow-hidden">
          {file.thumbnail_file_id ? <img src={`/api/files/${file.thumbnail_file_id}`} alt="G-code preview" className="w-full h-full object-cover" /> : <FileCode className="h-8 w-8 text-accent-300" />}
        </div>
        <div className="min-w-0 flex-1">
          {editingName ? (
            <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(file.display_name); setEditingName(false) } }} className="input py-1 text-sm" />
          ) : (
            <button type="button" onClick={() => setEditingName(true)} className="font-medium text-surface-100 truncate text-left hover:text-accent-300" title={file.display_name}>{file.display_name}</button>
          )}
          <div className="mb-1 inline-flex rounded-full border border-accent-500/30 bg-accent-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-accent-300">G-code</div>
          <div className="text-xs text-surface-500 mt-1">{file.material_type?.toUpperCase() || 'Material unknown'}</div>
          {file.filament_name && <div className="text-xs text-surface-400 truncate">Filamento: {file.filament_name}</div>}
          <div className="text-xs text-surface-500">{file.print_count} successful prints</div>
          {!file.parent_stl_id && <TagChips tags={file.tags} />}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2 text-xs mb-4">
        <Info label={file.estimated_seconds ? formatDuration(file.estimated_seconds) : 'No ETA'} />
        <Info label={file.filament_grams ? `${Math.round(file.filament_grams)}g filament` : 'No grams'} />
        <Info label={file.layer_height ? `${file.layer_height}mm layer` : 'No layer'} />
        <Info label={file.nozzle_diameter ? `${file.nozzle_diameter}mm nozzle` : 'No nozzle'} />
      </div>
      <div className="flex gap-2">
        <button onClick={onView} className="btn btn-secondary text-sm flex-1" title="Details"><InfoIcon className="h-4 w-4" /></button>
        <button onClick={onAdd} disabled={busy} className="btn btn-primary text-sm flex-1"><Plus className="h-4 w-4 mr-2" />{busy ? 'Adding...' : 'Add to Queue'}</button>
        <button onClick={onSend} disabled={busy} className="btn btn-secondary text-sm flex-1"><Send className="h-4 w-4 mr-2" />Send to Printer</button>
        {confirmDelete ? (
          <button onClick={onDelete} disabled={busy} className="btn btn-secondary text-sm text-red-400">Confirm</button>
        ) : (
          <button onClick={onDelete} disabled={busy} className="btn btn-ghost text-sm text-red-400">Delete</button>
        )}
      </div>
    </div>
  )
}

function STLFileRow({ file, busy, confirmDelete, onView, onRename, onDelete, onRetryThumbnail, onDropGCode, onSlice }: { file: STLLibraryFile; busy: boolean; confirmDelete: boolean; onView: () => void; onRename: (name: string) => void; onDelete: () => void; onRetryThumbnail: () => void; onDropGCode: (gcodeId: string) => void; onSlice: () => void }) {
  const [editingName, setEditingName] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const [name, setName] = useState(file.display_name)
  const saveName = () => {
    setEditingName(false)
    onRename(name)
  }
  return (
    <div onDragOver={e => { e.preventDefault(); setDragOver(true) }} onDragLeave={() => setDragOver(false)} onDrop={e => { e.preventDefault(); e.stopPropagation(); setDragOver(false); const id = e.dataTransfer.getData('text/gcode-id'); if (id) onDropGCode(id) }} className={cn('grid grid-cols-[56px_minmax(220px,1.7fr)_0.7fr_minmax(120px,1fr)_minmax(130px,1fr)_0.8fr_0.7fr_220px] gap-3 items-center border-l-4 border-l-sky-400 px-4 py-3 border-b border-sky-500/20 last:border-b-0 bg-sky-500/[0.065] hover:bg-sky-500/[0.11]', dragOver && 'bg-sky-500/15 border-sky-400/60')}>
      <div className="w-10 h-10 rounded-lg bg-sky-500/10 border border-sky-500/25 flex items-center justify-center overflow-hidden">
        {file.thumbnail_file_id ? <img src={`/api/files/${file.thumbnail_file_id}`} alt="STL preview" className="w-full h-full object-contain" /> : <button type="button" disabled={busy} onClick={onRetryThumbnail} className="text-[9px] text-sky-300 disabled:opacity-60">Retry</button>}
      </div>
      <div className="min-w-0">
        {editingName ? <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(file.display_name); setEditingName(false) } }} className="input py-1 text-sm" /> : <button type="button" onClick={() => setEditingName(true)} className="text-left text-surface-100 hover:text-sky-300 truncate max-w-full">{file.display_name}</button>}
        <div className="text-xs text-surface-500 truncate">{file.file_name}</div>
        <TagChips tags={file.tags} limit={2} />
      </div>
      <div className="text-sm text-sky-300 font-medium">STL</div>
      <div className="text-sm text-surface-300">Parent model</div>
      <div className="text-sm text-surface-400">Source geometry</div>
      <div className="text-sm text-surface-300">{formatBytes(file.size_bytes)}</div>
      <div className="text-sm text-surface-300">{file.gcodes?.length || 0} G-code</div>
      <div className="flex gap-2 justify-end">
        <button onClick={onView} className="btn btn-secondary text-xs"><InfoIcon className="h-3.5 w-3.5" /></button>
        <button onClick={onSlice} className="btn btn-primary text-xs">Fatiar</button>
        <a href={`/api/files/${file.file_id}`} target="_blank" rel="noreferrer" className="btn btn-secondary text-xs">Open</a>
        <button onClick={onDelete} disabled={busy} className={cn('btn text-xs', confirmDelete ? 'btn-secondary text-red-400' : 'btn-ghost text-red-400')}>{confirmDelete ? 'Confirm' : 'Delete'}</button>
      </div>
    </div>
  )
}

function FileRow({ file, busy, confirmDelete, indented = false, matched = false, parentName, onView, onRename, onAdd, onSend, onDelete }: { file: GCodeLibraryFile; busy: boolean; confirmDelete: boolean; indented?: boolean; matched?: boolean; parentName?: string; onView: () => void; onRename: (name: string) => void; onAdd: () => void; onSend: () => void; onDelete: () => void }) {
  const [editingName, setEditingName] = useState(false)
  const [name, setName] = useState(file.display_name)
  const saveName = () => {
    setEditingName(false)
    onRename(name)
  }
  return (
    <div draggable onDragStart={e => e.dataTransfer.setData('text/gcode-id', file.id)} className={cn('grid grid-cols-[56px_minmax(220px,1.7fr)_0.7fr_minmax(120px,1fr)_minmax(130px,1fr)_0.8fr_0.7fr_220px] gap-3 items-center border-l-4 border-l-accent-400 px-4 py-3 border-b border-accent-500/20 last:border-b-0 bg-accent-500/[0.055] hover:bg-accent-500/[0.1] cursor-grab', indented && 'ml-8 border-l-sky-500/35 bg-accent-500/[0.07]', matched && 'ring-1 ring-emerald-400/60 bg-emerald-500/[0.08]')}>
      <div className="w-10 h-10 rounded-lg bg-surface-800 flex items-center justify-center overflow-hidden">
        {file.thumbnail_file_id ? <img src={`/api/files/${file.thumbnail_file_id}`} alt="G-code preview" className="w-full h-full object-cover" /> : <FileCode className="h-5 w-5 text-surface-500" />}
      </div>
      <div className="min-w-0">
        {editingName ? <input value={name} autoFocus onChange={e => setName(e.target.value)} onBlur={saveName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setName(file.display_name); setEditingName(false) } }} className="input py-1 text-sm" /> : <button type="button" onClick={() => setEditingName(true)} className="text-left text-surface-100 hover:text-accent-300 truncate max-w-full">{file.display_name}</button>}
        <div className="flex items-center gap-2 text-xs text-surface-500 truncate"><span className="truncate">{file.file_name}</span>{matched && <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] text-emerald-300">match</span>}</div>
        {!file.parent_stl_id && <TagChips tags={file.tags} limit={2} />}
      </div>
      <div className="text-sm text-accent-300 font-medium">G-code</div>
      <div className="text-sm text-surface-300 truncate" title={parentName || 'Root'}>{parentName ? `Inside ${parentName}` : 'Root file'}</div>
      <div className="min-w-0 text-sm text-surface-300">
        <div>{file.material_type?.toUpperCase() || '—'}</div>
        <div className="text-xs text-surface-500 truncate">{file.metadata?.print_settings_id || 'No profile'}</div>
      </div>
      <div className="min-w-0 text-sm text-surface-300">
        <div>{file.estimated_seconds ? formatDuration(file.estimated_seconds) : 'No ETA'}</div>
        <div className="text-xs text-surface-500">Layer {file.layer_height ? `${file.layer_height}mm` : '—'}</div>
      </div>
      <div className="text-sm text-surface-300">{file.print_count} prints</div>
      <div className="flex gap-2 justify-end">
        <button onClick={onView} className="btn btn-secondary text-xs" title="Details"><InfoIcon className="h-3.5 w-3.5" /></button>
        <button onClick={onAdd} disabled={busy} className="btn btn-primary text-xs">{busy ? 'Adding...' : 'Queue'}</button>
        <button onClick={onSend} disabled={busy} className="btn btn-secondary text-xs"><Send className="h-3.5 w-3.5 mr-1" />Send</button>
        <button onClick={onDelete} disabled={busy} className={cn('btn text-xs', confirmDelete ? 'btn-secondary text-red-400' : 'btn-ghost text-red-400')}>{confirmDelete ? 'Confirm' : 'Delete'}</button>
      </div>
    </div>
  )
}

function FileDetailsModal({ file, tags, onCreateTag, onAddTag, onRemoveTag, onClose }: { file: GCodeLibraryFile; tags: Tag[]; onCreateTag: (name: string, color?: string) => void; onAddTag: (fileId: string, tagId: string) => void; onRemoveTag: (fileId: string, tagId: string) => void; onClose: () => void }) {
  const isRoot = !file.parent_stl_id
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
      <div className="card p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-surface-100">File Details</h2>
          <button onClick={onClose} className="btn btn-ghost text-sm">Close</button>
        </div>
        <div className="space-y-4">
          {file.thumbnail_file_id && <img src={`/api/files/${file.thumbnail_file_id}`} alt="G-code preview" className="w-28 h-28 rounded-xl object-cover bg-surface-800" />}
           <div className="font-medium text-surface-100 text-lg">{file.display_name}</div>
           <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <label className="block"><span className="text-xs text-surface-500 mb-1 block">Perfil de Impressão</span><input value={file.metadata?.print_settings_id || 'Não encontrado'} disabled className="input opacity-70" /></label>
              <label className="block"><span className="text-xs text-surface-500 mb-1 block">Perfil de Impressora</span><input value={file.metadata?.printer_settings_id || 'Não encontrado'} disabled className="input opacity-70" /></label>
              <label className="block"><span className="text-xs text-surface-500 mb-1 block">Perfil de Filamento</span><input value={file.metadata?.filament_settings_id || file.filament_name || 'Não encontrado'} disabled className="input opacity-70" /></label>
              <label className="block"><span className="text-xs text-surface-500 mb-1 block">Impressora</span><input value={file.metadata?.printer_model || 'Não encontrado'} disabled className="input opacity-70" /></label>
             <label className="block"><span className="text-xs text-surface-500 mb-1 block">Material</span><input value={(file.material_type || '').toUpperCase()} disabled className="input opacity-70" /></label>
             <div className="text-xs text-surface-500">Filament grams<div className="text-surface-100 text-base mt-0.5">{file.filament_grams ? Math.round(file.filament_grams) : '—'}</div></div>
             <div className="text-xs text-surface-500">Estimated seconds<div className="text-surface-100 text-base mt-0.5">{file.estimated_seconds ? formatDuration(file.estimated_seconds) : '—'}</div></div>
             <div className="text-xs text-surface-500">Layer height<div className="text-surface-100 text-base mt-0.5">{file.layer_height || '—'}</div></div>
             <div className="text-xs text-surface-500">Nozzle diameter<div className="text-surface-100 text-base mt-0.5">{file.nozzle_diameter || '—'}</div></div>
             <div className="text-xs text-surface-500">Bed temp<div className="text-surface-100 text-base mt-0.5">{file.bed_temp || '—'}</div></div>
             <div className="text-xs text-surface-500">Nozzle temp<div className="text-surface-100 text-base mt-0.5">{file.nozzle_temp || '—'}</div></div>
           </div>

           {isRoot ? <FileTagManager itemId={file.id} currentTags={file.tags} tags={tags} onCreateTag={onCreateTag} onAddTag={onAddTag} onRemoveTag={onRemoveTag} /> : <div className="rounded-lg border border-surface-800 bg-surface-900/70 px-3 py-2 text-sm text-surface-500">Tags only apply to root files. This G-code inherits organization from its parent STL.</div>}

           <div className="flex gap-2 justify-end">
             <button onClick={onClose} className="btn btn-secondary">Close</button>
           </div>
        </div>
      </div>
    </div>
  )
}

function Info({ label }: { label: string }) {
  return <div className="rounded-lg bg-surface-800/60 px-2 py-1.5 text-surface-300 truncate">{label}</div>
}
