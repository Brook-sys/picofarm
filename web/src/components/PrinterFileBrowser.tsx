import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Folder, FileText, Upload, Trash2, HardDrive, PlaySquare, ArrowLeft, FolderPlus, Pencil, Download, MoveRight, CheckSquare, Square, Save } from 'lucide-react'
import { printersApi, gcodeLibraryApi } from '../api/client'
import AppToast, { type AppToastState } from './AppToast'
import { formatBytes, formatDuration, formatRelativeTime } from '../lib/utils'
import type { PrinterFileEntry } from '../types'

interface PrinterFileBrowserProps {
  printerId: string
  connectionType: string
}

export function PrinterFileBrowser({ printerId, connectionType }: PrinterFileBrowserProps) {
  const queryClient = useQueryClient()
  const [currentPath, setCurrentPath] = useState(connectionType === 'moonraker' ? 'sda1' : '')
  const [uploading, setUploading] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [previewFile, setPreviewFile] = useState<PrinterFileEntry | null>(null)
  const [moveRequest, setMoveRequest] = useState<PrinterFileEntry[] | null>(null)
  const [toast, setToast] = useState<AppToastState | null>(null)

  const showToast = (next: AppToastState) => {
    setToast(next)
    window.setTimeout(() => setToast(null), 3500)
  }

  useEffect(() => {
    setCurrentPath(connectionType === 'moonraker' ? 'sda1' : '')
    setSelected([])
    setPreviewFile(null)
  }, [printerId, connectionType])

  const { data: fileList, isLoading } = useQuery({
    queryKey: ['printer-files', printerId, currentPath],
    queryFn: () => printersApi.listFiles(printerId, currentPath),
    enabled: connectionType === 'moonraker' || connectionType === 'octoprint',
  })

  const { data: metadata, isLoading: metadataLoading } = useQuery({
    queryKey: ['printer-file-metadata', printerId, previewFile?.path],
    queryFn: () => printersApi.getFileMetadata(printerId, previewFile!.path),
    enabled: (connectionType === 'moonraker' || connectionType === 'octoprint') && previewFile?.type === 'file',
    retry: false,
  })

  const invalidateFiles = () => {
    setSelected([])
    queryClient.invalidateQueries({ queryKey: ['printer-files', printerId] })
  }

  const uploadMutation = useMutation({ mutationFn: (file: File) => printersApi.uploadFile(printerId, currentPath, file), onSuccess: invalidateFiles })
  const deleteMutation = useMutation({ mutationFn: (file: PrinterFileEntry) => printersApi.deleteFile(printerId, file.path, file.type), onSuccess: invalidateFiles })
  const mkdirMutation = useMutation({ mutationFn: (path: string) => printersApi.createDirectory(printerId, path), onSuccess: invalidateFiles })
  const renameMutation = useMutation({ mutationFn: ({ path, newPath }: { path: string; newPath: string }) => printersApi.renameFile(printerId, path, newPath), onSuccess: invalidateFiles })
  const moveMutation = useMutation({ mutationFn: ({ path, newPath }: { path: string; newPath: string }) => printersApi.moveFile(printerId, path, newPath), onSuccess: invalidateFiles })
  const printMutation = useMutation({ mutationFn: (path: string) => printersApi.printFile(printerId, path) })
  const saveToLibraryMutation = useMutation({
    mutationFn: (path: string) => gcodeLibraryApi.saveFromPrinter({ printer_id: printerId, remote_path: path }),
    onSuccess: () => showToast({ title: 'Saved to Library', message: 'File imported to G-code library successfully.', tone: 'success' }),
    onError: (err) => showToast({ title: 'Import failed', message: err instanceof Error ? err.message : 'Unknown error', tone: 'error' })
  })

  if (connectionType !== 'moonraker' && connectionType !== 'octoprint') {
    return (
      <div className="flex flex-col items-center justify-center p-8 text-center bg-surface-900 rounded-lg border border-surface-800">
        <HardDrive className="h-10 w-10 text-surface-500 mb-3" />
        <h3 className="text-sm font-medium text-surface-200">Not Supported</h3>
        <p className="text-sm text-surface-500 mt-1">File browsing is currently only available for Moonraker/Fluidd/Mainsail and OctoPrint connected printers.</p>
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
  const navigateUp = () => {
    const parent = currentPath.split('/').filter(Boolean).slice(0, -1).join('/')
    setCurrentPath(connectionType === 'moonraker' && parent === '' ? 'sda1' : parent)
  }
  const navigateTo = (dirName: string) => {
    setSelected([])
    setPreviewFile(null)
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

  const openMoveDialog = (paths: string[]) => {
    const files = paths.map(path => fileByPath.get(path)).filter(Boolean) as PrinterFileEntry[]
    if (!files.length) return
    setMoveRequest(files)
  }

  const moveEntries = async (files: PrinterFileEntry[], targetDir: string) => {
    try {
      for (const file of files) {
        await moveMutation.mutateAsync({ path: file.path, newPath: joinPath(targetDir, file.name) })
      }
      setMoveRequest(null)
      setSelected([])
      setPreviewFile(null)
      setCurrentPath(targetDir)
      queryClient.invalidateQueries({ queryKey: ['printer-files', printerId] })
      showToast({ title: 'Moved', message: `${files.length} item${files.length === 1 ? '' : 's'} moved to /${targetDir}.`, tone: 'success' })
    } catch (err) {
      showToast({ title: 'Move failed', message: err instanceof Error ? err.message : 'Failed to move file', tone: 'error' })
    }
  }

  const deleteEntries = async (paths: string[]) => {
    const files = paths.map(path => fileByPath.get(path)).filter(Boolean) as PrinterFileEntry[]
    if (!files.length || !confirm(`Delete ${files.length} item(s)?`)) return
    try {
      for (const file of files) await deleteMutation.mutateAsync(file)
      showToast({ title: 'Deleted', message: `${files.length} item${files.length === 1 ? '' : 's'} deleted.`, tone: 'success' })
    } catch (err) {
      showToast({ title: 'Delete failed', message: err instanceof Error ? err.message : 'Failed to delete item', tone: 'error' })
    }
  }

  const toggleSelect = (path: string) => {
    setSelected(prev => prev.includes(path) ? prev.filter(item => item !== path) : [...prev, path])
  }

  return (
    <div className="flex h-full min-h-[620px] flex-col overflow-hidden rounded-xl border border-surface-800 bg-surface-900 shadow-inner shadow-black/20">
      <div className="flex flex-wrap items-center justify-between gap-3 p-3 border-b border-surface-800 bg-surface-800/30">
        <div className="flex items-center gap-2 min-w-0">
          <button disabled={!currentPath || (connectionType === 'moonraker' && currentPath === 'sda1')} onClick={navigateUp} className="p-1.5 rounded-md hover:bg-surface-700 disabled:opacity-30">
            <ArrowLeft className="h-4 w-4" />
          </button>
          <span className="text-sm font-medium text-surface-300 truncate">/{currentPath || 'root'}</span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {selected.length > 0 && (
            <>
              <span className="text-xs text-surface-400">{selected.length} selected</span>
              <button onClick={() => openMoveDialog(selected)} className="btn btn-secondary text-xs"><MoveRight className="h-3.5 w-3.5 mr-1.5" />Move</button>
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

      <div className="flex min-h-[520px] flex-1 overflow-hidden">
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
                <tr key={file.path} onClick={() => setPreviewFile(file)} className="hover:bg-surface-800/50 group transition-colors cursor-pointer">
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
                      <button onClick={() => openMoveDialog([file.path])} className="p-1.5 text-surface-300 hover:bg-surface-700 rounded" title="Move"><MoveRight className="h-4 w-4" /></button>
                      <button onClick={() => deleteEntries([file.path])} className="p-1.5 text-red-400 hover:bg-red-500/10 rounded" title="Delete"><Trash2 className="h-4 w-4" /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        </div>
        <div className="hidden w-80 shrink-0 border-l border-surface-800 bg-surface-950/35 p-4 lg:block">
          {previewFile ? (
            <div className="space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-surface-100">
                    {previewFile.type === 'dir' ? <Folder className="h-5 w-5 text-blue-400" /> : <FileText className="h-5 w-5 text-accent-300" />}
                    <span className="truncate font-semibold">{previewFile.name}</span>
                  </div>
                  <div className="mt-1 break-all text-xs text-surface-500">{previewFile.path}</div>
                </div>
                <button onClick={() => setPreviewFile(null)} className="text-surface-500 hover:text-surface-200">×</button>
              </div>

              {previewFile.type === 'file' && metadata?.thumbnail_relative_path && (
                <img src={printersApi.thumbnailUrl(printerId, metadata.thumbnail_relative_path)} alt="Preview" className="aspect-square w-full rounded-xl border border-surface-800 bg-surface-900 object-contain" />
              )}

              <div className="grid grid-cols-2 gap-3 text-xs">
                <PreviewMetric label="Size" value={formatBytes(metadata?.size || previewFile.size || 0)} />
                <PreviewMetric label="Modified" value={(metadata?.modified || previewFile.modified) ? formatRelativeTime(new Date((metadata?.modified || previewFile.modified || 0) * 1000).toISOString()) : '—'} />
                <PreviewMetric label="ETA" value={metadata?.estimated_time ? formatDuration(Math.round(metadata.estimated_time)) : '—'} />
                <PreviewMetric label="Filament" value={metadata?.filament_total ? `${Math.round(metadata.filament_total / 1000)}m` : '—'} />
                <PreviewMetric label="Layer" value={metadata?.layer_height ? `${metadata.layer_height}mm` : '—'} />
                <PreviewMetric label="Height" value={metadata?.object_height ? `${metadata.object_height}mm` : '—'} />
                <PreviewMetric label="Bed" value={metadata?.first_layer_bed_temp ? `${metadata.first_layer_bed_temp}°C` : '—'} />
                <PreviewMetric label="Nozzle" value={metadata?.first_layer_extr_temp ? `${metadata.first_layer_extr_temp}°C` : '—'} />
              </div>

              <div className="rounded-lg border border-surface-800 bg-surface-900/70 p-3 text-xs text-surface-400">
                <div className="text-surface-500">Slicer</div>
                <div className="mt-1 text-surface-200">{metadataLoading ? 'Loading...' : metadata?.slicer ? `${metadata.slicer} ${metadata.slicer_version || ''}` : '—'}</div>
              </div>

              {previewFile.type === 'file' && (
                <div className="flex flex-col gap-2">
                  <div className="flex gap-2">
                    <a href={printersApi.downloadFileUrl(printerId, previewFile.path)} className="btn btn-secondary flex-1 text-xs"><Download className="mr-1.5 h-3.5 w-3.5" />Download</a>
                    <button onClick={() => confirm(`Print ${previewFile.name}?`) && printMutation.mutate(previewFile.path)} className="btn btn-primary flex-1 text-xs"><PlaySquare className="mr-1.5 h-3.5 w-3.5" />Print</button>
                  </div>
                  <button onClick={() => saveToLibraryMutation.mutate(previewFile.path)} disabled={saveToLibraryMutation.isPending} className="btn btn-secondary text-xs w-full"><Save className="mr-1.5 h-3.5 w-3.5" />{saveToLibraryMutation.isPending ? 'Saving...' : 'Save to G-code Library'}</button>
                </div>
              )}
            </div>
          ) : (
            <div className="flex h-full flex-col items-center justify-center text-center text-sm text-surface-500">
              <HardDrive className="mb-3 h-8 w-8 text-surface-600" />
              Select a file to view metadata
            </div>
          )}
        </div>
      </div>
      {moveRequest && (
        <MoveFilesModal
          printerId={printerId}
          currentPath={currentPath}
          files={moveRequest}
          onClose={() => setMoveRequest(null)}
          onMove={(targetDir) => moveEntries(moveRequest, targetDir)}
          busy={moveMutation.isPending}
        />
      )}
      {toast && <AppToast toast={toast} onClose={() => setToast(null)} />}
    </div>
  )
}

function MoveFilesModal({ printerId, currentPath, files, busy, onClose, onMove }: { printerId: string; currentPath: string; files: PrinterFileEntry[]; busy: boolean; onClose: () => void; onMove: (targetDir: string) => void }) {
  const [browsePath, setBrowsePath] = useState(currentPath)
  const [manualPath, setManualPath] = useState(currentPath)
  const { data, isLoading } = useQuery({
    queryKey: ['printer-files-move-folders', printerId, browsePath],
    queryFn: () => printersApi.listFiles(printerId, browsePath),
  })
  const folders = (data?.files || []).filter(file => file.type === 'dir').sort((a, b) => a.name.localeCompare(b.name))
  const parts = browsePath.split('/').filter(Boolean)
  const goUp = () => {
    const next = parts.slice(0, -1).join('/')
    setBrowsePath(next)
    setManualPath(next)
  }
  const chooseFolder = (folder: PrinterFileEntry) => {
    const next = [browsePath, folder.name].filter(Boolean).join('/')
    setBrowsePath(next)
    setManualPath(next)
  }
  const chooseCrumb = (index: number) => {
    const next = parts.slice(0, index + 1).join('/')
    setBrowsePath(next)
    setManualPath(next)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-2xl overflow-hidden rounded-2xl border border-surface-800 bg-surface-950 shadow-2xl shadow-black/40">
        <div className="border-b border-surface-800 bg-surface-900/80 p-5">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-xl font-semibold text-surface-100">Move files</h2>
              <p className="mt-1 text-sm text-surface-500">Choose the destination folder inside the printer storage.</p>
            </div>
            <button onClick={onClose} className="rounded-lg px-2 py-1 text-surface-500 hover:bg-surface-800 hover:text-surface-100">×</button>
          </div>
        </div>

        <div className="grid gap-0 md:grid-cols-[1fr_260px]">
          <div className="p-5">
            <div className="mb-3 flex flex-wrap items-center gap-2 text-sm">
              <button onClick={() => { setBrowsePath(''); setManualPath('') }} className="rounded-lg border border-surface-700 bg-surface-900 px-2.5 py-1 text-surface-300 hover:bg-surface-800">root</button>
              {parts.map((part, index) => (
                <button key={`${part}-${index}`} onClick={() => chooseCrumb(index)} className="rounded-lg border border-surface-700 bg-surface-900 px-2.5 py-1 text-surface-300 hover:bg-surface-800">/{part}</button>
              ))}
            </div>

            <div className="mb-3 flex gap-2">
              <button disabled={!browsePath} onClick={goUp} className="btn btn-secondary text-xs disabled:opacity-40"><ArrowLeft className="mr-1.5 h-3.5 w-3.5" />Up</button>
              <input value={manualPath} onChange={e => setManualPath(e.target.value)} className="input flex-1 text-sm" placeholder="Destination folder" />
            </div>

            <div className="h-72 overflow-y-auto rounded-xl border border-surface-800 bg-surface-900/40">
              {isLoading ? (
                <div className="p-6 text-center text-sm text-surface-500">Loading folders...</div>
              ) : folders.length === 0 ? (
                <div className="p-6 text-center text-sm text-surface-500">No subfolders here. You can move directly to this folder.</div>
              ) : (
                folders.map(folder => (
                  <button key={folder.path} onClick={() => chooseFolder(folder)} className="flex w-full items-center gap-3 border-b border-surface-800 px-4 py-3 text-left text-sm text-surface-200 last:border-b-0 hover:bg-surface-800/70">
                    <Folder className="h-4 w-4 text-blue-400" />
                    <span className="truncate">{folder.name}</span>
                  </button>
                ))
              )}
            </div>
          </div>

          <div className="border-t border-surface-800 bg-surface-900/50 p-5 md:border-l md:border-t-0">
            <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-surface-500">Moving {files.length} item{files.length === 1 ? '' : 's'}</div>
            <div className="mb-5 max-h-48 space-y-2 overflow-y-auto">
              {files.map(file => (
                <div key={file.path} className="flex items-center gap-2 rounded-lg border border-surface-800 bg-surface-950/60 px-3 py-2 text-xs text-surface-300">
                  {file.type === 'dir' ? <Folder className="h-3.5 w-3.5 text-blue-400" /> : <FileText className="h-3.5 w-3.5 text-accent-300" />}
                  <span className="truncate">{file.name}</span>
                </div>
              ))}
            </div>
            <div className="rounded-lg border border-surface-800 bg-surface-950/70 p-3 text-xs text-surface-500">
              Destination
              <div className="mt-1 break-all font-mono text-surface-200">gcodes/{manualPath || ''}</div>
            </div>
          </div>
        </div>

        <div className="flex justify-end gap-2 border-t border-surface-800 bg-surface-900/80 p-4">
          <button onClick={onClose} className="btn btn-secondary">Cancel</button>
          <button disabled={busy} onClick={() => onMove(manualPath.trim())} className="btn btn-primary"><MoveRight className="mr-2 h-4 w-4" />{busy ? 'Moving...' : 'Move here'}</button>
        </div>
      </div>
    </div>
  )
}

function PreviewMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-surface-800 bg-surface-900/70 p-3">
      <div className="text-surface-500">{label}</div>
      <div className="mt-1 font-medium text-surface-100">{value}</div>
    </div>
  )
}
