import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Folder, FileText, Upload, Trash2, HardDrive, PlaySquare, ArrowLeft, FolderPlus, Pencil, Download, MoveRight, CheckSquare, Square } from 'lucide-react'
import { printersApi } from '../api/client'
import { formatBytes, formatRelativeTime } from '../lib/utils'
import type { PrinterFileEntry } from '../types'

interface PrinterFileBrowserProps {
  printerId: string
  connectionType: string
}

export function PrinterFileBrowser({ printerId, connectionType }: PrinterFileBrowserProps) {
  const queryClient = useQueryClient()
  const [currentPath, setCurrentPath] = useState('')
  const [uploading, setUploading] = useState(false)
  const [selected, setSelected] = useState<string[]>([])

  const { data: fileList, isLoading } = useQuery({
    queryKey: ['printer-files', printerId, currentPath],
    queryFn: () => printersApi.listFiles(printerId, currentPath),
    enabled: connectionType === 'moonraker',
  })

  const invalidateFiles = () => {
    setSelected([])
    queryClient.invalidateQueries({ queryKey: ['printer-files', printerId] })
  }

  const uploadMutation = useMutation({ mutationFn: (file: File) => printersApi.uploadFile(printerId, currentPath, file), onSuccess: invalidateFiles })
  const deleteMutation = useMutation({ mutationFn: (path: string) => printersApi.deleteFile(printerId, path), onSuccess: invalidateFiles })
  const mkdirMutation = useMutation({ mutationFn: (path: string) => printersApi.createDirectory(printerId, path), onSuccess: invalidateFiles })
  const renameMutation = useMutation({ mutationFn: ({ path, newPath }: { path: string; newPath: string }) => printersApi.renameFile(printerId, path, newPath), onSuccess: invalidateFiles })
  const moveMutation = useMutation({ mutationFn: ({ path, newPath }: { path: string; newPath: string }) => printersApi.moveFile(printerId, path, newPath), onSuccess: invalidateFiles })
  const printMutation = useMutation({ mutationFn: (path: string) => printersApi.printFile(printerId, path) })

  if (connectionType !== 'moonraker') {
    return (
      <div className="flex flex-col items-center justify-center p-8 text-center bg-surface-900 rounded-lg border border-surface-800">
        <HardDrive className="h-10 w-10 text-surface-500 mb-3" />
        <h3 className="text-sm font-medium text-surface-200">Not Supported</h3>
        <p className="text-sm text-surface-500 mt-1">File browsing is currently only available for Moonraker/Fluidd/Mainsail connected printers.</p>
      </div>
    )
  }

  const entries = [...(fileList?.files || [])].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'dir' ? -1 : 1
    return a.name.localeCompare(b.name)
  })

  const fileByPath = new Map(entries.map(file => [file.path, file]))
  const allSelected = entries.length > 0 && selected.length === entries.length

  const joinPath = (dir: string, name: string) => [dir, name].filter(Boolean).join('/')
  const parentPath = (filePath: string) => filePath.split('/').slice(0, -1).join('/')
  const navigateUp = () => setCurrentPath(currentPath.split('/').filter(Boolean).slice(0, -1).join('/'))
  const navigateTo = (dirName: string) => {
    setSelected([])
    setCurrentPath(joinPath(currentPath, dirName))
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    if (!files.length) return
    setUploading(true)
    try {
      for (const file of files) await uploadMutation.mutateAsync(file)
    } finally {
      setUploading(false)
      e.target.value = ''
    }
  }

  const createDirectory = async () => {
    const name = prompt('Folder name')?.trim()
    if (!name) return
    await mkdirMutation.mutateAsync(joinPath(currentPath, name))
  }

  const renameEntry = async (file: PrinterFileEntry) => {
    const name = prompt('New name', file.name)?.trim()
    if (!name || name === file.name) return
    await renameMutation.mutateAsync({ path: file.path, newPath: joinPath(parentPath(file.path), name) })
  }

  const moveEntries = async (paths: string[]) => {
    const targetDir = prompt('Move to folder path', currentPath)?.trim()
    if (targetDir === undefined || targetDir === null) return
    for (const filePath of paths) {
      const file = fileByPath.get(filePath)
      if (!file) continue
      await moveMutation.mutateAsync({ path: file.path, newPath: joinPath(targetDir, file.name) })
    }
  }

  const deleteEntries = async (paths: string[]) => {
    if (!paths.length || !confirm(`Delete ${paths.length} item(s)?`)) return
    for (const filePath of paths) await deleteMutation.mutateAsync(filePath)
  }

  const toggleSelect = (path: string) => {
    setSelected(prev => prev.includes(path) ? prev.filter(item => item !== path) : [...prev, path])
  }

  return (
    <div className="flex flex-col h-full bg-surface-900 rounded-lg border border-surface-800 overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-3 p-3 border-b border-surface-800 bg-surface-800/30">
        <div className="flex items-center gap-2 min-w-0">
          <button disabled={!currentPath} onClick={navigateUp} className="p-1.5 rounded-md hover:bg-surface-700 disabled:opacity-30">
            <ArrowLeft className="h-4 w-4" />
          </button>
          <span className="text-sm font-medium text-surface-300 truncate">gcodes/{currentPath}</span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {selected.length > 0 && (
            <>
              <span className="text-xs text-surface-400">{selected.length} selected</span>
              <button onClick={() => moveEntries(selected)} className="btn btn-secondary text-xs"><MoveRight className="h-3.5 w-3.5 mr-1.5" />Move</button>
              <button onClick={() => deleteEntries(selected)} className="btn btn-secondary text-xs text-red-300"><Trash2 className="h-3.5 w-3.5 mr-1.5" />Delete</button>
            </>
          )}
          <button onClick={createDirectory} className="btn btn-secondary text-xs"><FolderPlus className="h-3.5 w-3.5 mr-1.5" />New Folder</button>
          <label className="btn btn-primary text-xs cursor-pointer">
            <Upload className="h-3.5 w-3.5 mr-1.5" />{uploading ? 'Uploading...' : 'Upload'}
            <input type="file" multiple className="hidden" onChange={handleUpload} disabled={uploading} accept=".gcode,.bgcode,.3mf" />
          </label>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="p-8 text-center text-surface-500">Loading files...</div>
        ) : entries.length === 0 ? (
          <div className="p-8 text-center text-surface-500">Empty directory</div>
        ) : (
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="sticky top-0 bg-surface-900 shadow-sm z-10 text-surface-400">
              <tr>
                <th className="px-4 py-2 font-medium w-10">
                  <button onClick={() => setSelected(allSelected ? [] : entries.map(file => file.path))} className="text-surface-400 hover:text-surface-200">
                    {allSelected ? <CheckSquare className="h-4 w-4" /> : <Square className="h-4 w-4" />}
                  </button>
                </th>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Size</th>
                <th className="px-4 py-2 font-medium">Modified</th>
                <th className="px-4 py-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-800">
              {entries.map((file) => (
                <tr key={file.path} className="hover:bg-surface-800/50 group transition-colors">
                  <td className="px-4 py-2">
                    <button onClick={() => toggleSelect(file.path)} className="text-surface-400 hover:text-surface-200">
                      {selected.includes(file.path) ? <CheckSquare className="h-4 w-4" /> : <Square className="h-4 w-4" />}
                    </button>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-2">
                      {file.type === 'dir' ? <Folder className="h-4 w-4 text-blue-400" /> : <FileText className="h-4 w-4 text-surface-400" />}
                      {file.type === 'dir' ? (
                        <button onClick={() => navigateTo(file.name)} className="font-medium text-blue-400 hover:underline">{file.name}</button>
                      ) : (
                        <span className="font-medium text-surface-200">{file.name}</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-2 text-surface-400">{file.type === 'file' ? formatBytes(file.size || 0) : '--'}</td>
                  <td className="px-4 py-2 text-surface-400">{file.modified ? formatRelativeTime(new Date(file.modified * 1000).toISOString()) : '--'}</td>
                  <td className="px-4 py-2 text-right">
                    <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      {file.type === 'file' && (
                        <>
                          <a href={printersApi.downloadFileUrl(printerId, file.path)} className="p-1.5 text-surface-300 hover:bg-surface-700 rounded" title="Download"><Download className="h-4 w-4" /></a>
                          <button onClick={() => confirm(`Print ${file.name}?`) && printMutation.mutate(file.path)} className="p-1.5 text-accent-400 hover:bg-accent-500/10 rounded" title="Print"><PlaySquare className="h-4 w-4" /></button>
                        </>
                      )}
                      <button onClick={() => renameEntry(file)} className="p-1.5 text-surface-300 hover:bg-surface-700 rounded" title="Rename"><Pencil className="h-4 w-4" /></button>
                      <button onClick={() => moveEntries([file.path])} className="p-1.5 text-surface-300 hover:bg-surface-700 rounded" title="Move"><MoveRight className="h-4 w-4" /></button>
                      <button onClick={() => deleteEntries([file.path])} className="p-1.5 text-red-400 hover:bg-red-500/10 rounded" title="Delete"><Trash2 className="h-4 w-4" /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
