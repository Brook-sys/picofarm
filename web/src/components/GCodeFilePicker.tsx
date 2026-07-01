import { useMemo, useState } from 'react'
import { FileCode, Search, X } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { fileLibraryApi } from '../api/client'
import type { GCodeLibraryFile, STLLibraryFile } from '../types'

type RootLibraryFile = { type: 'stl'; file: STLLibraryFile } | { type: 'gcode'; file: GCodeLibraryFile }

interface GCodeFilePickerProps {
  onSelect: (item: RootLibraryFile) => void
  onSelectMany?: (items: RootLibraryFile[]) => void
  multiple?: boolean
  stlWithGCodeOnly?: boolean
  onClose: () => void
}

export default function GCodeFilePicker({ onSelect, onSelectMany, multiple = false, stlWithGCodeOnly = false, onClose }: GCodeFilePickerProps) {
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<RootLibraryFile[]>([])

  const { data, isLoading } = useQuery({
    queryKey: ['root-file-library-picker'],
    queryFn: () => fileLibraryApi.get(),
  })

  const items = useMemo<RootLibraryFile[]>(() => {
    const q = search.trim().toLowerCase()
    const rootItems: RootLibraryFile[] = stlWithGCodeOnly
      ? (data?.stl_files || []).filter(file => (file.gcodes || []).length > 0).map(file => ({ type: 'stl' as const, file }))
      : [
        ...(data?.stl_files || []).map(file => ({ type: 'stl' as const, file })),
        ...(data?.root_gcode_files || []).map(file => ({ type: 'gcode' as const, file })),
      ]
    return rootItems
      .filter(item => {
        if (!q) return true
        const haystack = item.type === 'gcode'
          ? [item.file.display_name, item.file.file_name, item.file.material_type, item.file.filament_name].filter(Boolean).join(' ').toLowerCase()
          : [item.file.display_name, item.file.file_name].filter(Boolean).join(' ').toLowerCase()
        return haystack.includes(q)
      })
      .sort((a, b) => a.file.display_name.localeCompare(b.file.display_name))
  }, [data, search, stlWithGCodeOnly])

  const isSelected = (item: RootLibraryFile) => selected.some(entry => entry.type === item.type && entry.file.id === item.file.id)
  const toggleSelected = (item: RootLibraryFile) => {
    setSelected(current => isSelected(item) ? current.filter(entry => !(entry.type === item.type && entry.file.id === item.file.id)) : [...current, item])
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="w-full max-w-3xl rounded-2xl border border-surface-800 bg-surface-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-surface-800 p-4">
          <div>
            <div className="font-semibold text-surface-100">Selecionar arquivo da biblioteca</div>
            <div className="text-xs text-surface-500">Apenas arquivos na raiz da aba Files: STL ou G-code</div>
          </div>
          <div className="flex items-center gap-2">
            {multiple && <button disabled={selected.length === 0} onClick={() => onSelectMany?.(selected)} className="btn btn-primary text-xs disabled:opacity-50">Adicionar {selected.length || ''}</button>}
            <button onClick={onClose} className="text-surface-400 hover:text-surface-200">
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        <div className="p-4 space-y-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-surface-500" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Buscar por nome, material, perfil..."
              className="input w-full pl-9"
            />
          </div>

          <div className="max-h-[520px] overflow-y-auto space-y-1 rounded-lg border border-surface-800 bg-surface-900/40 p-1">
            {isLoading ? (
              <div className="p-8 text-center text-surface-500">Carregando...</div>
            ) : items.length === 0 ? (
              <div className="p-8 text-center text-surface-500">Nenhum arquivo raiz encontrado.</div>
            ) : (
              items.map((item) => (
                  <button
                    key={`${item.type}-${item.file.id}`}
                    onClick={() => multiple ? toggleSelected(item) : onSelect(item)}
                    className={`w-full flex items-center gap-3 rounded-lg p-3 text-left transition-colors ${isSelected(item) ? 'bg-accent-500/10 ring-1 ring-accent-500/50' : 'hover:bg-surface-800/60'}`}
                  >
                    {multiple && <span className={`flex h-5 w-5 shrink-0 items-center justify-center rounded border ${isSelected(item) ? 'border-accent-500 bg-accent-500 text-white' : 'border-surface-700 bg-surface-900'}`}>{isSelected(item) ? '✓' : ''}</span>}
                    <div className="w-14 h-14 flex-shrink-0 rounded-lg bg-surface-800 overflow-hidden flex items-center justify-center">
                      {item.file.thumbnail_file_id ? (
                        <img src={`/api/files/${item.file.thumbnail_file_id}`} alt="" className="w-full h-full object-cover" />
                      ) : (
                        <FileCode className="h-6 w-6 text-surface-600" />
                      )}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-surface-100 truncate">{item.file.display_name}</span>
                        <span className={item.type === 'stl' ? 'rounded-full border border-sky-500/30 bg-sky-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase text-sky-300' : 'rounded-full border border-accent-500/30 bg-accent-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase text-accent-300'}>{item.type}</span>
                      </div>
                      <div className="mt-0.5 text-xs text-surface-500 flex items-center gap-2">
                        {item.type === 'gcode' ? <>
                          {item.file.material_type && <span className="uppercase">{item.file.material_type}</span>}
                          {(item.file.filament_grams || 0) > 0 && <span>{item.file.filament_grams!.toFixed(0)}g</span>}
                          {(item.file.estimated_seconds || 0) > 0 && <span>{Math.floor(item.file.estimated_seconds! / 3600)}h {Math.floor((item.file.estimated_seconds! % 3600) / 60)}m</span>}
                        </> : <span>{item.file.gcodes?.length || 0} G-code linked</span>}
                      </div>
                    </div>
                    <div className="text-xs text-surface-500 text-right truncate max-w-48">{item.file.file_name}</div>
                  </button>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
