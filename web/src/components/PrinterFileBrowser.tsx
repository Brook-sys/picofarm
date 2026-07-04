import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Folder, FileText, Upload, Trash2, HardDrive, PlaySquare, ArrowLeft } from 'lucide-react'
import { printersApi } from '../api/client'
import { formatBytes, formatRelativeTime } from '../lib/utils'

interface PrinterFileBrowserProps {
  printerId: string
  connectionType: string
}

export function PrinterFileBrowser({ printerId, connectionType }: PrinterFileBrowserProps) {
  const queryClient = useQueryClient()
  const [currentPath, setCurrentPath] = useState('')
  const [uploading, setUploading] = useState(false)

  const { data: fileList, isLoading } = useQuery({
    queryKey: ['printer-files', printerId, currentPath],
    queryFn: () => printersApi.listFiles(printerId, currentPath),
    enabled: connectionType === 'moonraker',
  })

  const uploadMutation = useMutation({
    mutationFn: (file: File) => printersApi.uploadFile(printerId, currentPath, file),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['printer-files', printerId] }),
  })

  const deleteMutation = useMutation({
    mutationFn: (path: string) => printersApi.deleteFile(printerId, path),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['printer-files', printerId] }),
  })

  const printMutation = useMutation({
    mutationFn: (path: string) => printersApi.printFile(printerId, path),
  })

  if (connectionType !== 'moonraker') {
    return (
      <div className="flex flex-col items-center justify-center p-8 text-center bg-surface-900 rounded-lg border border-surface-800">
        <HardDrive className="h-10 w-10 text-surface-500 mb-3" />
        <h3 className="text-sm font-medium text-surface-200">Not Supported</h3>
        <p className="text-sm text-surface-500 mt-1">
          File browsing is currently only available for Moonraker/Fluidd/Mainsail connected printers.
        </p>
      </div>
    )
  }

  const navigateUp = () => {
    const parts = currentPath.split('/').filter(Boolean)
    parts.pop()
    setCurrentPath(parts.join('/'))
  }

  const navigateTo = (dirName: string) => {
    setCurrentPath(currentPath ? `${currentPath}/${dirName}` : dirName)
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      await uploadMutation.mutateAsync(file)
    } finally {
      setUploading(false)
      e.target.value = ''
    }
  }

  return (
    <div className="flex flex-col h-full bg-surface-900 rounded-lg border border-surface-800 overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center justify-between p-3 border-b border-surface-800 bg-surface-800/30">
        <div className="flex items-center gap-2">
          <button
            disabled={!currentPath}
            onClick={navigateUp}
            className="p-1.5 rounded-md hover:bg-surface-700 disabled:opacity-30"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <span className="text-sm font-medium text-surface-300">
            gcodes/{currentPath}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <label className="btn btn-primary text-xs cursor-pointer">
            <Upload className="h-3.5 w-3.5 mr-1.5" />
            {uploading ? 'Uploading...' : 'Upload'}
            <input
              type="file"
              className="hidden"
              onChange={handleUpload}
              disabled={uploading}
              accept=".gcode,.bgcode,.3mf"
            />
          </label>
        </div>
      </div>

      {/* File List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="p-8 text-center text-surface-500">Loading files...</div>
        ) : !fileList?.files?.length ? (
          <div className="p-8 text-center text-surface-500">Empty directory</div>
        ) : (
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="sticky top-0 bg-surface-900 shadow-sm z-10 text-surface-400">
              <tr>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Size</th>
                <th className="px-4 py-2 font-medium">Modified</th>
                <th className="px-4 py-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-800">
              {fileList.files
                .sort((a, b) => {
                  if (a.type !== b.type) return a.type === 'dir' ? -1 : 1
                  return a.name.localeCompare(b.name)
                })
                .map((file) => (
                  <tr key={file.path} className="hover:bg-surface-800/50 group transition-colors">
                    <td className="px-4 py-2">
                      <div className="flex items-center gap-2">
                        {file.type === 'dir' ? (
                          <Folder className="h-4 w-4 text-blue-400" />
                        ) : (
                          <FileText className="h-4 w-4 text-surface-400" />
                        )}
                        {file.type === 'dir' ? (
                          <button
                            onClick={() => navigateTo(file.name)}
                            className="font-medium text-blue-400 hover:underline"
                          >
                            {file.name}
                          </button>
                        ) : (
                          <span className="font-medium text-surface-200">{file.name}</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-2 text-surface-400">
                      {file.type === 'file' ? formatBytes(file.size || 0) : '--'}
                    </td>
                    <td className="px-4 py-2 text-surface-400">
                      {file.modified ? formatRelativeTime(new Date(file.modified * 1000).toISOString()) : '--'}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        {file.type === 'file' && (
                          <button
                            onClick={() => {
                              if (confirm(`Print ${file.name}?`)) {
                                printMutation.mutate(file.path)
                              }
                            }}
                            className="p-1.5 text-accent-400 hover:bg-accent-500/10 rounded"
                            title="Print"
                          >
                            <PlaySquare className="h-4 w-4" />
                          </button>
                        )}
                        <button
                          onClick={() => {
                            if (confirm(`Delete ${file.name}?`)) {
                              deleteMutation.mutate(file.path)
                            }
                          }}
                          className="p-1.5 text-red-400 hover:bg-red-500/10 rounded"
                          title="Delete"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
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
