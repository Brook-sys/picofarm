import { useTranslation } from 'react-i18next'
import { externalLinkClick } from '../lib/platform'
import { resourceURL } from '../lib/platform'
import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  ArrowLeft,
  Plus,
  Printer,
  Play,
  FileCode,
  Box,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Star,
  History,
  Clock,
  RefreshCw,
  BarChart3,
  DollarSign,
  TrendingUp,
  Timer,
  ExternalLink,
  X,
  Scale,
  Trash2,
  ShoppingCart,
  Info,
} from 'lucide-react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useProject, useParts, useCreatePart, useUpdateProject } from '../hooks/useProjects'
import { usePrinters, usePrinterStates } from '../hooks/usePrinters'
import { useSpoolsWithMaterials } from '../hooks/useMaterials'
import { designsApi, printJobsApi, projectsApi, partsApi, suppliesApi, materialsApi, queueApi, gcodeLibraryApi, fileLibraryApi, fileApi, tagsApi, enqueueProjectParts } from '../api/client'
import { cn, getStatusBadge, formatBytes, formatRelativeTime, PRINT_JOB_STATUS_LABELS } from '../lib/utils'
import { useCurrency } from '../hooks/useCurrency'
import AppToast, { type AppToastState } from '../components/AppToast'
import { FailureModal } from '../components/FailureModal'
import { ExpandableJobEvents } from '../components/JobEventTimeline'
import { Tooltip } from '../components/Tooltip'
import GCodeFilePicker from '../components/GCodeFilePicker'
import TagSelector from '../components/TagSelector'
import { STLViewer3D } from '../components/STLViewer3D'
import type { Design, Part, Material, PrintJob, ProjectSummary, ProjectSupply, GCodeLibraryFile, STLLibraryFile } from '../types'

type RootProjectFile = { type: 'stl'; file: STLLibraryFile } | { type: 'gcode'; file: GCodeLibraryFile }

export default function ProjectDetail() {
  const { t } = useTranslation('production')

  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()

  const { data: project, isLoading: projectLoading } = useProject(id!)
  const { data: parts = [], isLoading: partsLoading } = useParts(id!)
  const { data: printers = [] } = usePrinters()
  const { data: printerStates = {} } = usePrinterStates()

  const createPart = useCreatePart()
  const updateProject = useUpdateProject()

  const [showAddPart, setShowAddPart] = useState(false)
  const [selectedPart, setSelectedPart] = useState<Part | null>(null)
  const [showUpload, setShowUpload] = useState(false)
  const [showSendToPrinter, setShowSendToPrinter] = useState<Design | null>(null)
  const [showOutcomeCapture, setShowOutcomeCapture] = useState<PrintJob | null>(null)
  const [showFailureModal, setShowFailureModal] = useState<PrintJob | null>(null)
  const [activeTab, setActiveTab] = useState<'parts' | 'history' | 'analytics'>('parts')
  const [editingProject, setEditingProject] = useState(false)
  const [projectForm, setProjectForm] = useState({ name: '', description: '', source_url: '', source_license: '', source_author: '', source_provider: '', cover_file_id: '', image_file_ids: [] as string[] })
  const [projectEditError, setProjectEditError] = useState('')
  const [uploadingCover, setUploadingCover] = useState(false)
  const [descriptionExpanded, setDescriptionExpanded] = useState(false)
  const [selectedRootFiles, setSelectedRootFiles] = useState<RootProjectFile[]>([])
  const [showGCodePicker, setShowGCodePicker] = useState(false)
  const [partDeleteError, setPartDeleteError] = useState('')
  const [projectToast, setProjectToast] = useState<AppToastState | null>(null)
  const [printingProject, setPrintingProject] = useState(false)

  const { data: projectSummary } = useQuery({
    queryKey: ['project-summary', id],
    queryFn: () => projectsApi.getSummary(id!),
    enabled: !!id,
  })

  const handlePrintProject = async () => {
    setPrintingProject(true)
    try {
      const { added, missing } = await enqueueProjectParts(id!)
      if (added > 0) {
        setProjectToast({ title: t("Project queued"), message: t("{{count}} item added to the print queue.", { count: added }), tone: 'success' })
        queryClient.invalidateQueries({ queryKey: ['queue'] })
      } else if (missing > 0) {
        setProjectToast({ title: t("No G-code found"), message: t("{{count}} part does not have valid G-code.", { count: missing }), tone: 'info' })
      } else {
        setProjectToast({ title: t("Nothing queued"), message: t("No items were added to the print queue."), tone: 'info' })
      }
    } catch (err) {
      setProjectToast({ title: t("Failed to queue project"), message: err instanceof Error ? err.message : String(err), tone: 'error' })
    } finally {
      setPrintingProject(false)
      setTimeout(() => setProjectToast(null), 3500)
    }
  }

  const handleAddPart = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (selectedRootFiles.length === 0) return

    for (const selected of selectedRootFiles) {
      await createPart.mutateAsync({
        projectId: id!,
        data: {
          name: selected.file.display_name || selected.file.file_name,
          description: '',
          quantity: 1,
          gcode_file_id: selected.type === 'gcode' ? selected.file.id : undefined,
          stl_file_id: selected.type === 'stl' ? selected.file.id : undefined,
        },
      })
    }

    setShowAddPart(false)
    setSelectedRootFiles([])
  }

  if (projectLoading) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="text-surface-500">{t("Loading...")}</div>
      </div>
    )
  }

  if (!project) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="text-surface-500">{t("Project not found")}</div>
      </div>
    )
  }

  const startProjectEdit = () => {
    setProjectForm({
      name: project.name || '',
      description: project.description || '',
      source_url: project.source_url || '',
      source_license: project.source_license || '',
      source_author: project.source_author || '',
      source_provider: project.source_provider || '',
      cover_file_id: project.cover_file_id || '',
      image_file_ids: Array.from(new Set([project.cover_file_id, ...((project.default_settings?.image_file_ids as string[] | undefined) || [])].filter(Boolean) as string[])),
    })
    setProjectEditError('')
    setEditingProject(true)
  }

  const saveProjectEdit = async () => {
    setProjectEditError('')
    try {
      await updateProject.mutateAsync({
        id: project.id,
        data: {
          ...projectForm,
          cover_file_id: projectForm.cover_file_id || undefined,
          default_settings: { ...(project.default_settings || {}), image_file_ids: projectForm.image_file_ids },
        },
      })
      setEditingProject(false)
    } catch (err) {
      setProjectEditError(err instanceof Error ? err.message : t("Failed to update project"))
    }
  }

  const uploadProjectImage = async (file?: File) => {
    if (!file) return
    setUploadingCover(true)
    setProjectEditError('')
    try {
      const uploaded = await fileApi.upload(file)
      setProjectForm(prev => ({
        ...prev,
        cover_file_id: prev.cover_file_id || uploaded.id,
        image_file_ids: Array.from(new Set([...prev.image_file_ids, uploaded.id])),
      }))
    } catch (err) {
      setProjectEditError(err instanceof Error ? err.message : t("Failed to upload project photo"))
    } finally {
      setUploadingCover(false)
    }
  }

  const projectImages = Array.from(new Set([project.cover_file_id, ...((project.default_settings?.image_file_ids as string[] | undefined) || [])].filter(Boolean) as string[]))
  const hasLongDescription = (project.description || '').length > 420
  const visibleDescription = !hasLongDescription || descriptionExpanded ? project.description : `${project.description.slice(0, 420).trim()}...`

  return (
    <div className="p-4 sm:p-6 lg:p-8">
      {projectToast && <AppToast toast={projectToast} onClose={() => setProjectToast(null)} />}
      {/* Header */}
      <div className="mb-8">
        <Link
          to="/projects"
          className="inline-flex items-center text-sm text-surface-500 hover:text-surface-300 mb-4"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          {t("Back to Projects")}</Link>

          <div className="card p-5">
          {editingProject ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Project name")}</label>
                <input value={projectForm.name} onChange={e => setProjectForm(prev => ({ ...prev, name: e.target.value }))} className="input text-xl font-semibold" autoFocus />
              </div>
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Description")}</label>
                <textarea value={projectForm.description} onChange={e => setProjectForm(prev => ({ ...prev, description: e.target.value }))} rows={2} className="input resize-none" placeholder={t("Optional description")} />
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Project / source link")}</label>
                  <input value={projectForm.source_url} onChange={e => setProjectForm(prev => ({ ...prev, source_url: e.target.value }))} className="input" placeholder={t("https://...")} />
                </div>
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("License")}</label>
                  <input value={projectForm.source_license} onChange={e => setProjectForm(prev => ({ ...prev, source_license: e.target.value }))} className="input" placeholder={t("CC BY, MIT, Commercial use...")} />
                </div>
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Author / creator")}</label>
                  <input value={projectForm.source_author} onChange={e => setProjectForm(prev => ({ ...prev, source_author: e.target.value }))} className="input" placeholder={t("Author name")} />
                </div>
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Platform")}</label>
                  <input value={projectForm.source_provider} onChange={e => setProjectForm(prev => ({ ...prev, source_provider: e.target.value }))} className="input" placeholder={t("MakerWorld, Printables, etc.")} />
                </div>
              </div>
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Cover photo")}</label>
                <div className="space-y-3">
                  <label className="btn btn-secondary text-sm cursor-pointer w-fit">
                    {uploadingCover ? t("Uploading...") : t("Add photo")}
                    <input type="file" accept="image/png,image/jpeg" disabled={uploadingCover} className="hidden" onChange={e => void uploadProjectImage(e.target.files?.[0])} />
                  </label>
                  {projectForm.image_file_ids.length > 0 && (
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                      {projectForm.image_file_ids.map(fileId => (
                        <div key={fileId} className={cn('relative overflow-hidden rounded-xl border bg-surface-900', projectForm.cover_file_id === fileId ? 'border-accent-500' : 'border-surface-800')}>
                          <img src={resourceURL(`/api/files/${fileId}`)} className="h-28 w-full object-cover" />
                          <div className="absolute inset-x-0 bottom-0 flex gap-1 bg-black/70 p-1">
                            <button type="button" onClick={() => setProjectForm(prev => ({ ...prev, cover_file_id: fileId }))} className="flex-1 rounded bg-surface-800/80 px-2 py-1 text-[10px] text-surface-200 hover:bg-accent-600">
                              {projectForm.cover_file_id === fileId ? t("Cover") : t("Set as cover")}
                            </button>
                            <button type="button" onClick={() => setProjectForm(prev => ({ ...prev, image_file_ids: prev.image_file_ids.filter(id => id !== fileId), cover_file_id: prev.cover_file_id === fileId ? '' : prev.cover_file_id }))} className="rounded bg-red-500/20 px-2 py-1 text-[10px] text-red-200 hover:bg-red-500/40">
                              {t("Remove")}</button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
              {projectEditError && <div className="text-sm text-red-400">{projectEditError}</div>}
              <div className="flex gap-2">
                <button onClick={saveProjectEdit} disabled={updateProject.isPending} className="btn btn-primary text-sm">{updateProject.isPending ? t("Saving...") : t("Save")}</button>
                <button onClick={() => setEditingProject(false)} className="btn btn-secondary text-sm">{t("Cancel")}</button>
              </div>
            </div>
          ) : (
            <div className="space-y-6">
              <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
                <div className="min-w-0">
                  <h1 className="text-3xl font-display font-bold text-surface-100">
                    {project.name}
                  </h1>
                  {(project.source_url || project.source_license || project.source_author || project.source_provider) && (
                    <div className="mt-3 flex flex-wrap gap-2 text-xs text-surface-500">
                      {project.source_url && <a href={project.source_url} onClick={externalLinkClick} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-accent-400 hover:underline"><ExternalLink className="h-3 w-3" />{t("Source")}</a>}
                      {project.source_license && <span className="rounded-full border border-surface-700 px-2 py-0.5">{t("License:")}{' '}{project.source_license}</span>}
                      {project.source_author && <span className="rounded-full border border-surface-700 px-2 py-0.5">{t("By")}{' '}{project.source_author}</span>}
                      {project.source_provider && <span className="rounded-full border border-surface-700 px-2 py-0.5">{project.source_provider}</span>}
                    </div>
                  )}
                  {project.description && (
                    <div className="mt-5 rounded-2xl border border-surface-800 bg-surface-950/40 p-4">
                      <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-surface-200">
                        <Info className="h-4 w-4 text-accent-400" />{t("Description")}</div>
                      <p className="whitespace-pre-line text-sm leading-6 text-surface-300">{visibleDescription}</p>
                      {hasLongDescription && (
                        <button onClick={() => setDescriptionExpanded(prev => !prev)} className="mt-3 text-sm font-medium text-accent-400 hover:text-accent-300">
                          {descriptionExpanded ? t("Show less") : t("Show more")}
                        </button>
                      )}
                    </div>
                  )}
                </div>
                <div className="space-y-3">
                  <div className="overflow-hidden rounded-2xl border border-surface-800 bg-surface-900/70">
                    {project.cover_file_id ? <img src={resourceURL(`/api/files/${project.cover_file_id}`)} className="h-56 w-full object-cover" /> : <div className="flex h-56 items-center justify-center text-surface-600"><Box className="h-10 w-10" /></div>}
                  </div>
                  {projectImages.length > 1 && (
                    <div className="grid grid-cols-4 gap-2">
                      {projectImages.map(fileId => <img key={fileId} src={resourceURL(`/api/files/${fileId}`)} className={cn('h-16 w-full rounded-lg object-cover border', project.cover_file_id === fileId ? 'border-accent-500' : 'border-surface-800')} />)}
                    </div>
                  )}
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-3">
                <button onClick={handlePrintProject} disabled={printingProject} className="btn btn-primary" aria-label={t("Print whole project")} title={t("Print whole project")}>
                  <Play className="h-4 w-4 mr-2" />
                  {printingProject ? t("Queueing...") : t("Print Project")}
                </button>
                <button onClick={startProjectEdit} className="btn btn-secondary text-sm">{t("Edit")}</button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Printer Control Panel */}
      <div className="card p-4 mb-6">
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-semibold text-surface-100 flex items-center gap-2">
            <Printer className="h-5 w-5 text-accent-500" />
            {t("Printer Fleet")}</h2>
        </div>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          {printers.length === 0 ? (
            <div className="col-span-4 text-center py-4 text-surface-500">
              <Link to="/printers" className="text-accent-400 hover:text-accent-300">
                {t("Add printers to get started")}</Link>
            </div>
          ) : (
            printers.map((printer) => {
              const state = printerStates[printer.id]
              return (
                <div
                  key={printer.id}
                  className="p-3 rounded-lg bg-surface-800/50 border border-surface-700"
                >
                  <div className="flex items-center gap-2 mb-2">
                    <div className={cn(
                      'w-2 h-2 rounded-full',
                      state?.status === 'printing' ? 'bg-emerald-400 animate-pulse' :
                      state?.status === 'idle' ? 'bg-blue-400' :
                      state?.status === 'error' ? 'bg-red-400' :
                      'bg-surface-600'
                    )} />
                    <span className="font-medium text-surface-100 text-sm truncate">
                      {printer.name}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className={cn('badge text-xs', getStatusBadge(state?.status || 'offline'))}>
                      {t(({ pending: 'Pending', in_progress: 'In Progress', completed: 'Completed', cancelled: 'Cancelled', printing: 'Printing', paused: 'Paused', failed: 'Failed', queued: 'Queued', idle: 'Idle', offline: 'Offline', online: 'Online', error: 'Error', ready: 'Ready', sent: 'Sent', info: 'Info', success: 'Success', warning: 'Warning', critical: 'Critical', channels: 'Channels', templates: 'Templates', deliveries: 'Deliveries' } as Record<string, string>)[state?.status || 'offline'] || state?.status || 'offline')}
                    </span>
                    {state?.status === 'printing' && (
                      <span className="text-xs text-surface-400">
                        {state.progress.toFixed(0)}%
                      </span>
                    )}
                  </div>
                  {state?.status === 'printing' && (
                    <div className="mt-2 h-1 bg-surface-700 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-emerald-500 transition-all"
                        style={{ width: `${state.progress}%` }}
                      />
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* Project Quick Stats */}
      {projectSummary && (
        <ProjectQuickStats summary={projectSummary} />
      )}

      {/* Tab Navigation */}
      <div className="flex items-center gap-4 border-b border-surface-800 mb-4">
        <button
          onClick={() => setActiveTab('parts')}
          className={cn(
            'flex items-center gap-2 px-4 py-2 border-b-2 -mb-px transition-colors',
            activeTab === 'parts'
              ? 'border-accent-500 text-accent-400'
              : 'border-transparent text-surface-400 hover:text-surface-200'
          )}
        >
          <Box className="h-4 w-4" />
          {t("Parts")}</button>
        <button
          onClick={() => setActiveTab('history')}
          className={cn(
            'flex items-center gap-2 px-4 py-2 border-b-2 -mb-px transition-colors',
            activeTab === 'history'
              ? 'border-accent-500 text-accent-400'
              : 'border-transparent text-surface-400 hover:text-surface-200'
          )}
        >
          <History className="h-4 w-4" />
          {t("Print History")}</button>
        <button
          onClick={() => setActiveTab('analytics')}
          className={cn(
            'flex items-center gap-2 px-4 py-2 border-b-2 -mb-px transition-colors',
            activeTab === 'analytics'
              ? 'border-accent-500 text-accent-400'
              : 'border-transparent text-surface-400 hover:text-surface-200'
          )}
        >
          <BarChart3 className="h-4 w-4" />
          {t("Analytics")}</button>
      </div>

      {/* Parts Tab */}
      {activeTab === 'parts' && (
        <>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-semibold text-surface-100">{t("Parts")}</h2>
            <button
              onClick={() => setShowAddPart(true)}
              className="btn btn-secondary"
            >
              <Plus className="h-4 w-4 mr-2" />
              {t("Add Part")}</button>
          </div>

          {partDeleteError && <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{partDeleteError}</div>}
          {partsLoading ? (
            <div className="text-surface-500">{t("Loading parts...")}</div>
          ) : parts.length === 0 ? (
            <div className="card p-8 text-center">
              <Box className="h-12 w-12 mx-auto mb-3 text-surface-600" />
              <h3 className="text-lg font-medium text-surface-300 mb-2">
                {t("No parts yet")}</h3>
              <p className="text-surface-500 mb-4">
                {t("Add parts to start organizing your project")}</p>
              <button
                onClick={() => setShowAddPart(true)}
                className="btn btn-primary"
              >
                <Plus className="h-4 w-4 mr-2" />
                {t("Add First Part")}</button>
            </div>
          ) : (
            <div className="space-y-4">
              {parts.map((part) => (
                <PartCard
                  key={part.id}
                  part={part}
                  onSendToPrinter={(design) => setShowSendToPrinter(design)}
                  onAddDesign={() => { setSelectedPart(part); setShowUpload(true) }}
                  onDelete={async () => {
                    if (!confirm(t("Delete part \"{{value1}}\"? This cannot be undone.", { value1: part.name }))) return
                    try {
                      setPartDeleteError('')
                      await partsApi.delete(part.id)
                      queryClient.invalidateQueries({ queryKey: ['parts', id] })
                    } catch (err) {
                      setPartDeleteError(err instanceof Error ? err.message : t("Failed to delete part"))
                    }
                  }}
                />
              ))}
            </div>
          )}

          {/* Supplies Section */}
          <SuppliesSection projectId={id!} />
        </>
      )}

      {/* History Tab */}
      {activeTab === 'history' && (
        <PrintHistoryTab
          projectId={id!}
          parts={parts}
          printers={printers}
          onRecordOutcome={(job) => setShowOutcomeCapture(job)}
          onHandleFailure={(job) => setShowFailureModal(job)}
        />
      )}

      {/* Analytics Tab */}
      {activeTab === 'analytics' && (
        <ProjectAnalyticsTab summary={projectSummary} />
      )}

      {/* Add Part Modal */}
      {showAddPart && (
        <Modal title={t("Add Part")} onClose={() => setShowAddPart(false)}>
          <form onSubmit={handleAddPart}>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-surface-300 mb-2">{t("Files from the Files page")}</label>
                {selectedRootFiles.length > 0 ? (
                  <div className="space-y-2 rounded-lg border border-accent-500/40 bg-accent-500/5 p-3">
                    {selectedRootFiles.map(selected => (
                      <div key={`${selected.type}-${selected.file.id}`} className="flex items-center justify-between gap-3 rounded-lg bg-surface-900/60 p-2">
                        <div className="min-w-0">
                          <div className="truncate font-medium text-surface-100">{selected.file.display_name}</div>
                          <div className="text-xs uppercase text-surface-500">{selected.type}</div>
                        </div>
                        <button type="button" onClick={() => setSelectedRootFiles(files => files.filter(file => !(file.type === selected.type && file.file.id === selected.file.id)))} className="text-surface-400 hover:text-surface-200"><X className="h-4 w-4" /></button>
                      </div>
                    ))}
                  </div>
                ) : (
                  <button type="button" onClick={() => setShowGCodePicker(true)} className="w-full rounded-lg border border-surface-700 bg-surface-900/40 px-4 py-6 text-sm text-surface-300 hover:border-accent-500 hover:text-accent-400">
                    {t("Select STL / G-code from Files")}</button>
                )}
                {selectedRootFiles.length > 0 && <button type="button" onClick={() => setShowGCodePicker(true)} className="mt-2 text-xs text-accent-400 hover:text-accent-300">{t("Change selection")}</button>}
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                type="button"
                onClick={() => { setShowAddPart(false); setSelectedRootFiles([]) }}
                className="btn btn-ghost"
              >
                {t("Cancel")}</button>
              <button
                type="submit"
                disabled={createPart.isPending || selectedRootFiles.length === 0}
                className="btn btn-primary"
              >
                {createPart.isPending ? t("Adding...") : selectedRootFiles.length > 1 ? t("Add Parts") : t("Add Part")}
              </button>
            </div>
          </form>
        </Modal>
      )}

      {/* GCode File Picker */}
      {showGCodePicker && (
        <GCodeFilePicker
          multiple
          stlWithGCodeOnly
          onSelect={(item) => {
            setSelectedRootFiles([item])
            setShowGCodePicker(false)
          }}
          onSelectMany={(items) => {
            setSelectedRootFiles(items)
            setShowGCodePicker(false)
          }}
          onClose={() => setShowGCodePicker(false)}
        />
      )}

      {/* Upload Design Modal */}
      {showUpload && selectedPart && (
        <UploadDesignModal
          part={selectedPart}
          onClose={() => {
            setShowUpload(false)
            setSelectedPart(null)
          }}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ['designs', selectedPart.id] })
            setShowUpload(false)
            setSelectedPart(null)
          }}
        />
      )}

      {/* Send to Printer Modal */}
      {showSendToPrinter && (
        <SendToPrinterModal
          design={showSendToPrinter}
          printers={printers}
          printerStates={printerStates}
          onClose={() => setShowSendToPrinter(null)}
        />
      )}

      {/* Outcome Capture Modal */}
      {showOutcomeCapture && (
        <OutcomeCaptureModal
          job={showOutcomeCapture}
          onClose={() => setShowOutcomeCapture(null)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ['print-jobs'] })
            queryClient.invalidateQueries({ queryKey: ['spools'] })
            setShowOutcomeCapture(null)
          }}
        />
      )}

      {/* Failure Modal */}
      {showFailureModal && (
        <FailureModal
          job={showFailureModal}
          printers={printers}
          onClose={() => setShowFailureModal(null)}
          onRetry={() => {
            queryClient.invalidateQueries({ queryKey: ['print-jobs'] })
            setShowFailureModal(null)
          }}
          onScrap={() => {
            queryClient.invalidateQueries({ queryKey: ['print-jobs'] })
            setShowFailureModal(null)
          }}
        />
      )}
    </div>
  )
}

function formatPrintTime(seconds: number, translate: (key: string, options: Record<string, unknown>) => string): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return translate('{{hours}}h {{minutes}}m', { hours: h, minutes: m })
  return translate('{{minutes}}m', { minutes: m })
}

function InfoRow({ label, value }: { label: string; value: string }) {
  useTranslation('production')

  return <div className="flex justify-between gap-4 border-b border-surface-800 pb-2"><span className="text-surface-500">{label}</span><span className="text-right text-surface-200">{value}</span></div>
}

// Part Card Component
function PartCard({
  part,
  onSendToPrinter,
  onAddDesign,
  onDelete,
}: {
  part: Part
  onSendToPrinter: (design: Design) => void
  onAddDesign: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation('production')

  const queryClient = useQueryClient()
  const [partBusy, setPartBusy] = useState('')
  const [editingPartName, setEditingPartName] = useState(false)
  const [partName, setPartName] = useState(part.name)
  const [partQuantity, setPartQuantity] = useState(part.quantity)
  const [selectedGCodeMaterials, setSelectedGCodeMaterials] = useState<Record<string, string>>({})
  const [partError, setPartError] = useState('')
  const [toast, setToast] = useState<AppToastState | null>(null)
  const [viewingSTL, setViewingSTL] = useState<STLLibraryFile | null>(null)
  const [viewingGCode, setViewingGCode] = useState<GCodeLibraryFile | null>(null)
  const { data: designs = [] } = useQuery({
    queryKey: ['designs', part.id],
    queryFn: () => designsApi.listByPart(part.id),
  })
  // Tags for this part. The tag API and the TagSelector component both
  // existed; nothing ever rendered the selector, so tags could only be managed
  // through direct API calls.
  const { data: partTags = [] } = useQuery({
    queryKey: ['part-tags', part.id],
    queryFn: () => tagsApi.getPartTags(part.id),
  })
  const { data: library } = useQuery({ queryKey: ['file-library'], queryFn: () => fileLibraryApi.get() })
  const { data: spools = [] } = useSpoolsWithMaterials()
  const availableSpools = spools.filter(spool => spool.status !== 'empty' && spool.status !== 'archived' && spool.material)

  useEffect(() => {
    setPartName(part.name)
    setPartQuantity(part.quantity)
  }, [part.name, part.quantity])

  const savePartName = async () => {
    setEditingPartName(false)
    const next = partName.trim()
    if (!next || next === part.name) return
    await partsApi.update(part.id, { name: next })
    queryClient.invalidateQueries({ queryKey: ['parts', part.project_id] })
  }

  const savePartQuantity = async (quantity: number) => {
    const next = Math.max(1, quantity)
    setPartQuantity(next)
    if (next === part.quantity) return
    await partsApi.update(part.id, { quantity: next })
    queryClient.invalidateQueries({ queryKey: ['parts', part.project_id] })
  }

  const getDesignSTL = (design: Design) => (library?.stl_files || []).find(file => file.file_id === design.file_id)
  const getDesignGCode = (design: Design) => [...(library?.root_gcode_files || []), ...(library?.stl_files || []).flatMap(file => file.gcodes || [])].find(file => file.file_id === design.file_id)
  const getGCodeProfile = (gcode: GCodeLibraryFile) => gcode.metadata?.print_settings_id || t("No profile")

  const makeDefaultGCode = async (gcode: GCodeLibraryFile) => {
    await gcodeLibraryApi.setDefaultForSTL(gcode.id)
    queryClient.invalidateQueries({ queryKey: ['file-library'] })
    queryClient.invalidateQueries({ queryKey: ['designs', part.id] })
  }

  const printGCode = async (gcode: GCodeLibraryFile) => {
    setPartError('')
    setPartBusy(gcode.id)
    try {
      const gcodeMaterialType = (gcode.material_type || '').toLowerCase()
      const spool = availableSpools.find(item => item.id === selectedGCodeMaterials[gcode.id]) || availableSpools.find(item => item.default_for_material && item.material?.type.toLowerCase() === gcodeMaterialType)
      await gcodeLibraryApi.addToQueue(gcode.id, spool?.material ? { assigned_spool_id: spool.id, project_id: part.project_id, material_type: spool.material.type, material_color: spool.material.color_hex || spool.material.color, filament_name: spool.material.name, source_type: 'project' } : { project_id: part.project_id, source_type: 'project' })
      setToast({ title: t("Print added to queue"), message: gcode.display_name || gcode.file_name, tone: 'success' })
      setTimeout(() => setToast(null), 3500)
    } catch (err) {
      setPartError(err instanceof Error ? err.message : t("Failed to add print to queue"))
    } finally {
      setPartBusy('')
    }
  }

  const printDesign = async (design: Design) => {
    if (design.file_type === 'stl') {
      const stl = getDesignSTL(design)
      const gcode = stl?.gcodes?.find(file => file.default_for_stl) || stl?.gcodes?.[0]
      if (!gcode) {
        setPartError(t("STL has no linked G-code"))
        return
      }
      await printGCode(gcode)
      return
    }
    if (design.file_type === 'gcode') {
      const gcode = getDesignGCode(design)
      if (!gcode) {
        setPartError(t("G-code not found in library"))
        return
      }
      await printGCode(gcode)
      return
    }
    onSendToPrinter(design)
  }

  const renderGCodeRow = (gcode: GCodeLibraryFile) => {
    const gcodeMaterialType = (gcode.material_type || '').toLowerCase()
    const matchingSpools = gcodeMaterialType ? availableSpools.filter(spool => spool.material?.type.toLowerCase() === gcodeMaterialType) : []
    const defaultSpool = matchingSpools.find(spool => spool.default_for_material)
    const selectedSpoolID = selectedGCodeMaterials[gcode.id] || defaultSpool?.id || ''
    return (
    <div key={gcode.id} className="ml-12 mt-2 flex items-center justify-between rounded-lg border border-surface-700/60 bg-surface-900/60 p-2">
      <div className="min-w-0">
        <div className="flex items-center gap-2 text-sm font-medium text-orange-200">
          <FileCode className="h-4 w-4 text-orange-400" />
          <span className="truncate">{gcode.display_name || gcode.file_name}</span>
          {gcode.default_for_stl && <span className="rounded-full border border-emerald-500/40 bg-emerald-500/15 px-2 py-0.5 text-[10px] font-bold uppercase text-emerald-300">{t("Default")}</span>}
        </div>
        <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-surface-500">
          <span>{t('Profile: ')}{getGCodeProfile(gcode)}</span>
          {gcode.material_type && <span>{gcode.material_type.toUpperCase()}</span>}
          {gcode.layer_height && <span>{t('{{value1}}mm', { value1: gcode.layer_height })}</span>}
          {gcode.estimated_seconds && <span>{formatPrintTime(gcode.estimated_seconds, t)}</span>}
          {gcode.filament_grams && <span>{t('{{value1}}g', { value1: Math.round(gcode.filament_grams) })}</span>}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <select value={selectedSpoolID} onChange={e => setSelectedGCodeMaterials(current => ({ ...current, [gcode.id]: e.target.value }))} className="input w-56 py-1 text-xs" disabled={matchingSpools.length === 0}>
          <option value="">{gcodeMaterialType ? t("Spool {{value1}}", { value1: gcodeMaterialType.toUpperCase() }) : t("Type not detected")}</option>
          {matchingSpools.map(spool => <option key={spool.id} value={spool.id}>{spool.material ? `${spool.material.name} · ${spool.material.type.toUpperCase()}` : t("Unknown")} · {t('{{value1}}g', { value1: Math.round(spool.remaining_weight) })}{spool.default_for_material ? t(" · Default") : ''}</option>)}
        </select>
        {!gcode.default_for_stl && gcode.parent_stl_id && <button onClick={() => void makeDefaultGCode(gcode)} className="btn btn-ghost text-xs py-1.5 px-2">{t("Make default")}</button>}
        <button onClick={() => setViewingGCode(gcode)} className="btn btn-ghost text-xs py-1.5 px-2" title={t("G-code info")}><Info className="h-3.5 w-3.5" /></button>
        <button onClick={() => void printGCode(gcode)} className="btn btn-primary text-xs py-1.5 px-3">
          <Play className="h-3.5 w-3.5 mr-1" />
          {partBusy === gcode.id ? t("Adding...") : t("Queue")}
        </button>
      </div>
    </div>
    )
  }

  return (
    <div className="card p-5 relative">
      {toast && <AppToast toast={toast} onClose={() => setToast(null)} />}
      <div className="flex items-start justify-between mb-4">
        <div>
          <div className="flex items-center gap-2">
            {editingPartName ? <input value={partName} autoFocus onChange={e => setPartName(e.target.value)} onBlur={savePartName} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') { setPartName(part.name); setEditingPartName(false) } }} className="input py-1 text-sm" /> : <button type="button" onClick={() => setEditingPartName(true)} className="font-semibold text-surface-100 hover:text-accent-300">{part.name}</button>}
            <div className="ml-2 inline-flex items-center rounded-lg border border-surface-700 bg-surface-900/70">
              <button type="button" onClick={() => void savePartQuantity(partQuantity - 1)} className="px-2 py-1 text-surface-400 hover:text-surface-100">−</button>
              <input value={partQuantity} onChange={e => setPartQuantity(parseInt(e.target.value) || 1)} onBlur={() => void savePartQuantity(partQuantity)} onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur() }} className="w-10 bg-transparent text-center text-sm text-surface-200 outline-none" />
              <button type="button" onClick={() => void savePartQuantity(partQuantity + 1)} className="px-2 py-1 text-surface-400 hover:text-surface-100">+</button>
            </div>
          </div>
          {part.description && (
            <p className="text-sm text-surface-500 mt-1">{part.description}</p>
          )}

          {/* Tags */}
          <div className="mt-2">
            <TagSelector
              entityType="part"
              entityId={part.id}
              selectedTags={partTags}
              onChange={(tags) => queryClient.setQueryData(['part-tags', part.id], tags)}
            />
          </div>

        </div>
        <div className="flex items-center gap-2">
          <span className={cn('badge', getStatusBadge(part.status))}>
            {t(({ pending: 'Pending', in_progress: 'In Progress', completed: 'Completed', cancelled: 'Cancelled', printing: 'Printing', paused: 'Paused', failed: 'Failed', queued: 'Queued', idle: 'Idle', offline: 'Offline', online: 'Online', error: 'Error', ready: 'Ready', sent: 'Sent', info: 'Info', success: 'Success', warning: 'Warning', critical: 'Critical', channels: 'Channels', templates: 'Templates', deliveries: 'Deliveries' } as Record<string, string>)[part.status] || part.status)}
          </span>
          <button
            onClick={onDelete}
            className="p-1.5 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
            aria-label={t("Delete part")} title={t("Delete part")}
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Designs */}
      <div className="border-t border-surface-800 pt-4">
        <div className="flex items-center justify-between mb-3">
          <span className="text-sm font-medium text-surface-400">
            {t('Designs ({{count}})', { count: designs.length })})
          </span>
          <button onClick={onAddDesign} className="btn btn-secondary text-xs py-1.5 px-3"><Plus className="h-3.5 w-3.5 mr-1" />{t("Add Design")}</button>
        </div>

        {partError && <div className="mb-3 rounded-lg border border-red-500/30 bg-red-500/10 p-2 text-sm text-red-300">{partError}</div>}
        {designs.length === 0 ? (
          <div className="text-center py-4 text-surface-500 text-sm">
            {t("No designs uploaded yet")}</div>
        ) : (
          <div className="space-y-2">
            {designs.slice(0, 3).map((design) => {
              const stl = design.file_type === 'stl' ? getDesignSTL(design) : undefined
              const gcode = design.file_type === 'gcode' ? getDesignGCode(design) : undefined
              return (
                <div key={design.id} className="rounded-lg bg-surface-800/50 p-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="h-10 w-10 rounded-lg bg-surface-900 overflow-hidden flex items-center justify-center">
                        {stl?.thumbnail_file_id ? <img src={resourceURL(`/api/files/${stl.thumbnail_file_id}`)} className="h-full w-full object-contain" /> : <FileCode className="h-5 w-5 text-surface-500" />}
                      </div>
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-surface-200 truncate">
                          {t('v{{version}}', { version: design.version })} — {stl?.display_name || gcode?.display_name || design.file_name}
                        </div>
                        <div className="text-xs text-surface-500 truncate">
                          {design.file_name} • {formatBytes(design.file_size_bytes)} • {formatRelativeTime(design.created_at)}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {design.file_type === '3mf' && (
                        <button
                          onClick={() => {
                            designsApi.openExternal(design.id, t("BambuStudio")).catch((err) => {
                              alert(t("Failed to open Bambu Studio: ") + err.message)
                            })
                          }}
                          className="btn btn-ghost text-xs py-1.5 px-3"
                          title={t("Open in Bambu Studio")}
                        >
                          <ExternalLink className="h-3.5 w-3.5 mr-1" />
                          {t("Bambu Studio")}</button>
                      )}
                      {stl && <button onClick={() => setViewingSTL(stl)} className="btn btn-ghost text-xs py-1.5 px-2" title={t("STL info")}><Info className="h-3.5 w-3.5" /></button>}
                      {design.file_type !== 'stl' && (
                        <button onClick={() => void printDesign(design)} className="btn btn-primary text-xs py-1.5 px-3">
                          <Play className="h-3.5 w-3.5 mr-1" />
                          {partBusy === design.id || partBusy === gcode?.id ? t("Adding...") : t("Queue")}
                        </button>
                      )}
                      <button
                        onClick={async () => { if (!confirm(t("Delete design {{value1}}?", { value1: design.file_name }))) return; await designsApi.delete(design.id); queryClient.invalidateQueries({ queryKey: ['designs', part.id] }) }}
                        className="btn btn-ghost text-xs py-1.5 px-2 text-red-400"
                        title={t("Delete design")}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                  {stl?.gcodes?.map(renderGCodeRow)}
                  {gcode && <div className="mt-2">{renderGCodeRow(gcode)}</div>}
                </div>
              )
            })}
          </div>
        )}
      </div>
      {viewingSTL && (
        <Modal title={t("STL Viewer")} onClose={() => setViewingSTL(null)}>
          <div className="space-y-4">
            <STLViewer3D url={resourceURL(`/api/files/${viewingSTL.file_id}`)} />
            <div className="grid grid-cols-2 gap-4 text-sm bg-surface-900/50 p-3 rounded-lg">
              <div>
                <div className="text-surface-500 mb-1">{t("Title")}</div>
                <div className="font-medium text-surface-200">{viewingSTL.display_name || viewingSTL.file_name}</div>
              </div>
              <div>
                <div className="text-surface-500 mb-1">{t("File")}</div>
                <div className="font-medium text-surface-200 truncate" title={viewingSTL.file_name}>{viewingSTL.file_name}</div>
              </div>
              <div>
                <div className="text-surface-500 mb-1">{t("Size")}</div>
                <div className="font-medium text-surface-200">{formatBytes(viewingSTL.size_bytes)}</div>
              </div>
              <div>
                <div className="text-surface-500 mb-1">{t("Linked G-codes")}</div>
                <div className="font-medium text-surface-200">{viewingSTL.gcodes?.length || 0}</div>
              </div>
            </div>
          </div>
        </Modal>
      )}
      {viewingGCode && (
        <Modal title={t("G-code info")} onClose={() => setViewingGCode(null)}>
          <div className="space-y-3 text-sm">
            {viewingGCode.thumbnail_file_id && <img src={resourceURL(`/api/files/${viewingGCode.thumbnail_file_id}`)} className="h-48 w-full rounded-lg bg-surface-900 object-contain" />}
            <InfoRow label={t("Title")} value={viewingGCode.display_name || viewingGCode.file_name} />
            <InfoRow label={t("File")} value={viewingGCode.file_name} />
            <InfoRow label={t("Print profile")} value={viewingGCode.metadata?.print_settings_id || '-'} />
            <InfoRow label={t("Printer profile")} value={viewingGCode.metadata?.printer_settings_id || '-'} />
            <InfoRow label={t("Filament profile")} value={viewingGCode.metadata?.filament_settings_id || viewingGCode.filament_name || '-'} />
            <InfoRow label={t("Printer")} value={viewingGCode.metadata?.printer_model || '-'} />
            <InfoRow label={t("Material")} value={viewingGCode.material_type?.toUpperCase() || '-'} />
            <InfoRow label={t("Weight")} value={viewingGCode.filament_grams ? t('{{value1}}g', { value1: Math.round(viewingGCode.filament_grams) }) : '-'} />
            <InfoRow label={t("Time")} value={viewingGCode.estimated_seconds ? formatPrintTime(viewingGCode.estimated_seconds, t) : '-'} />
            <InfoRow label={t("Layer")} value={viewingGCode.layer_height ? t('{{value1}}mm', { value1: viewingGCode.layer_height }) : '-'} />
            <InfoRow label={t("Nozzle")} value={viewingGCode.nozzle_diameter ? t('{{value1}}mm', { value1: viewingGCode.nozzle_diameter }) : '-'} />
          </div>
        </Modal>
      )}
    </div>
  )
}

// Supplies Section Component
function SuppliesSection({ projectId }: { projectId: string }) {
  const { t } = useTranslation('production')

  const { formatAmount, formatCents, symbol } = useCurrency()
  const queryClient = useQueryClient()
  const [showAddForm, setShowAddForm] = useState(false)
  const [addMode, setAddMode] = useState<'catalog' | 'manual'>('catalog')
  const [selectedMaterialId, setSelectedMaterialId] = useState('')
  const [newName, setNewName] = useState('')
  const [newCost, setNewCost] = useState('')
  const [newQuantity, setNewQuantity] = useState('1')

  const { data: supplies = [] } = useQuery({
    queryKey: ['project-supplies', projectId],
    queryFn: () => suppliesApi.listByProject(projectId),
  })

  const { data: supplyMaterials = [] } = useQuery({
    queryKey: ['materials', 'supply'],
    queryFn: () => materialsApi.listByType('supply'),
  })

  const handleCatalogAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedMaterialId) return
    try {
      await suppliesApi.create(projectId, {
        name: '', // auto-populated from material by backend
        unit_cost_cents: 0, // auto-populated from material by backend
        quantity: parseInt(newQuantity) || 1,
        material_id: selectedMaterialId,
      })
      queryClient.invalidateQueries({ queryKey: ['project-supplies', projectId] })
      queryClient.invalidateQueries({ queryKey: ['project-summary', projectId] })
      setSelectedMaterialId('')
      setNewQuantity('1')
      setShowAddForm(false)
    } catch (err) {
      console.error('Failed to add supply from catalog:', err)
    }
  }

  const handleManualAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    try {
      await suppliesApi.create(projectId, {
        name: newName.trim(),
        unit_cost_cents: Math.round(parseFloat(newCost || '0') * 100),
        quantity: parseInt(newQuantity) || 1,
      })
      queryClient.invalidateQueries({ queryKey: ['project-supplies', projectId] })
      queryClient.invalidateQueries({ queryKey: ['project-summary', projectId] })
      setNewName('')
      setNewCost('')
      setNewQuantity('1')
      setShowAddForm(false)
    } catch (err) {
      console.error('Failed to add supply:', err)
    }
  }

  const [confirmDeleteSupplyId, setConfirmDeleteSupplyId] = useState<string | null>(null)

  const handleDelete = async (supply: ProjectSupply) => {
    if (confirmDeleteSupplyId !== supply.id) {
      setConfirmDeleteSupplyId(supply.id)
      return
    }
    setConfirmDeleteSupplyId(null)
    try {
      await suppliesApi.delete(supply.id)
      queryClient.invalidateQueries({ queryKey: ['project-supplies', projectId] })
      queryClient.invalidateQueries({ queryKey: ['project-summary', projectId] })
    } catch (err) {
      console.error('Failed to delete supply:', err)
    }
  }

  // Auto-fill cost when selecting a catalog material
  const handleMaterialSelect = (materialId: string) => {
    setSelectedMaterialId(materialId)
    const mat = supplyMaterials.find((m) => m.id === materialId)
    if (mat) {
      setNewCost((mat.cost_per_kg).toFixed(2)) // cost_per_kg is repurposed as per-unit $ for supplies
    }
  }

  const totalCents = supplies.reduce((sum, s) => sum + s.unit_cost_cents * s.quantity, 0)

  return (
    <div className="mt-8">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-semibold text-surface-100 flex items-center gap-2">
          <ShoppingCart className="h-5 w-5 text-surface-400" />
          {t("Supplies")}</h2>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="btn btn-secondary"
        >
          <Plus className="h-4 w-4 mr-2" />
          {t("Add Supply")}</button>
      </div>

      {showAddForm && (
        <div className="card p-4 mb-4">
          {supplyMaterials.length > 0 && (
            <div className="flex gap-2 mb-3">
              <button
                type="button"
                onClick={() => setAddMode('catalog')}
                className={cn(
                  'text-xs px-3 py-1 rounded-full transition-colors',
                  addMode === 'catalog'
                    ? 'bg-accent-500/20 text-accent-400 border border-accent-500'
                    : 'bg-surface-800 text-surface-400 border border-surface-700 hover:text-surface-200'
                )}
              >
                {t("From Catalog")}</button>
              <button
                type="button"
                onClick={() => setAddMode('manual')}
                className={cn(
                  'text-xs px-3 py-1 rounded-full transition-colors',
                  addMode === 'manual'
                    ? 'bg-accent-500/20 text-accent-400 border border-accent-500'
                    : 'bg-surface-800 text-surface-400 border border-surface-700 hover:text-surface-200'
                )}
              >
                {t("Manual Entry")}</button>
            </div>
          )}

          {addMode === 'catalog' && supplyMaterials.length > 0 ? (
            <form onSubmit={handleCatalogAdd}>
              <div className="grid grid-cols-[1fr_auto_auto_auto] gap-3 items-end">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Supply Material")}</label>
                  <select
                    value={selectedMaterialId}
                    onChange={(e) => handleMaterialSelect(e.target.value)}
                    className="input"
                    required
                  >
                    <option value="">{t("Select a supply...")}</option>
                    {supplyMaterials.map((mat) => (
                      <option key={mat.id} value={mat.id}>
                        {mat.name} {mat.manufacturer ? `(${mat.manufacturer})` : ''} — {formatAmount(mat.cost_per_kg)}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Unit Cost (")}{symbol})</label>
                  <input
                    type="number"
                    value={newCost}
                    onChange={(e) => setNewCost(e.target.value)}
                    className="input w-28"
                    placeholder={t("0.00")}
                    min="0"
                    step="0.01"
                    readOnly
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Qty")}</label>
                  <input
                    type="number"
                    value={newQuantity}
                    onChange={(e) => setNewQuantity(e.target.value)}
                    className="input w-20"
                    min="1"
                  />
                </div>
                <div className="flex gap-2">
                  <button type="submit" className="btn btn-primary" disabled={!selectedMaterialId}>{t("Add")}</button>
                  <button type="button" onClick={() => setShowAddForm(false)} className="btn btn-ghost">
                    {t("Cancel")}</button>
                </div>
              </div>
            </form>
          ) : (
            <form onSubmit={handleManualAdd}>
              <div className="grid grid-cols-[1fr_auto_auto_auto] gap-3 items-end">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Name")}</label>
                  <input
                    type="text"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    className="input"
                    placeholder={t("e.g., Lamp cord, Lightbulb")}
                    autoFocus
                    required
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Unit Cost (")}{symbol})</label>
                  <input
                    type="number"
                    value={newCost}
                    onChange={(e) => setNewCost(e.target.value)}
                    className="input w-28"
                    placeholder={t("0.00")}
                    min="0"
                    step="0.01"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Qty")}</label>
                  <input
                    type="number"
                    value={newQuantity}
                    onChange={(e) => setNewQuantity(e.target.value)}
                    className="input w-20"
                    min="1"
                  />
                </div>
                <div className="flex gap-2">
                  <button type="submit" className="btn btn-primary">{t("Add")}</button>
                  <button type="button" onClick={() => setShowAddForm(false)} className="btn btn-ghost">
                    {t("Cancel")}</button>
                </div>
              </div>
            </form>
          )}
        </div>
      )}

      {supplies.length === 0 ? (
        <div className="card p-6 text-center text-surface-500 text-sm">
          {t("No supplies added yet. Add non-printed items like lamp cords, lightbulbs, etc.")}</div>
      ) : (
        <div className="card overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-surface-800">
                <th className="text-left text-xs font-medium text-surface-500 uppercase px-4 py-2">{t("Item")}</th>
                <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">{t("Unit Cost")}</th>
                <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">{t("Qty")}</th>
                <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">{t("Total")}</th>
                <th className="w-10 px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {supplies.map((supply) => (
                <tr key={supply.id} className="border-b border-surface-800/50">
                  <td className="px-4 py-3 text-surface-200">{supply.name}</td>
                  <td className="px-4 py-3 text-right text-surface-300">
                    {formatCents(supply.unit_cost_cents)}
                  </td>
                  <td className="px-4 py-3 text-right text-surface-300">{supply.quantity}</td>
                  <td className="px-4 py-3 text-right text-surface-100 font-medium">
                    {formatCents(supply.unit_cost_cents * supply.quantity)}
                  </td>
                  <td className="px-4 py-3">
                    {confirmDeleteSupplyId === supply.id ? (
                      <button
                        onClick={() => handleDelete(supply)}
                        onBlur={() => setConfirmDeleteSupplyId(null)}
                        className="text-xs px-2 py-1 rounded bg-red-500/20 text-red-400 border border-red-500/50 hover:bg-red-500/30 transition-colors"
                        autoFocus
                      >
                        {t("Delete?")}</button>
                    ) : (
                      <button
                        onClick={() => handleDelete(supply)}
                        className="p-1 rounded text-surface-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="border-t border-surface-700">
                <td colSpan={3} className="px-4 py-3 text-right text-sm font-medium text-surface-400">
                  {t("Total Supplies")}</td>
                <td className="px-4 py-3 text-right font-semibold text-surface-100">
                  {formatCents(totalCents)}
                </td>
                <td></td>
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </div>
  )
}

// Print History Tab Component
function PrintHistoryTab({
  projectId,
  parts,
  printers,
  onRecordOutcome,
  onHandleFailure,
}: {
  projectId: string
  parts: Part[]
  printers: { id: string; name: string }[]
  onRecordOutcome: (job: PrintJob) => void
  onHandleFailure: (job: PrintJob) => void
}) {
  const { t } = useTranslation('production')

  const { formatAmount } = useCurrency()
  const queryClient = useQueryClient()
  const [confirmDeleteJob, setConfirmDeleteJob] = useState<string | null>(null)
  const [confirmClearHistory, setConfirmClearHistory] = useState(false)
  const [historyError, setHistoryError] = useState('')

  const refreshHistory = () => {
    queryClient.invalidateQueries({ queryKey: ['project-job-stats', projectId] })
    queryClient.invalidateQueries({ queryKey: ['design-jobs'] })
  }

  const deleteJob = async (jobId: string) => {
    if (confirmDeleteJob !== jobId) {
      setConfirmDeleteJob(jobId)
      return
    }
    setHistoryError('')
    try {
      await printJobsApi.delete(jobId)
      setConfirmDeleteJob(null)
      refreshHistory()
    } catch (err) {
      setHistoryError(err instanceof Error ? err.message : t("Failed to delete print record"))
    }
  }

  const clearHistory = async () => {
    if (!confirmClearHistory) {
      setConfirmClearHistory(true)
      return
    }
    setHistoryError('')
    try {
      await projectsApi.clearJobs(projectId)
      setConfirmClearHistory(false)
      refreshHistory()
    } catch (err) {
      setHistoryError(err instanceof Error ? err.message : t("Failed to clear print history"))
    }
  }

  // Server-side job stats
  const { data: jobStats } = useQuery({
    queryKey: ['project-job-stats', projectId],
    queryFn: () => projectsApi.getJobStats(projectId),
    enabled: !!projectId,
  })

  // Get all designs from all parts
  const designQueries = parts.map((part) =>
    // eslint-disable-next-line react-hooks/rules-of-hooks
    useQuery({
      queryKey: ['designs', part.id],
      queryFn: () => designsApi.listByPart(part.id),
    })
  )

  const allDesigns = designQueries.flatMap((q) => q.data || [])
  const isLoadingDesigns = designQueries.some((q) => q.isLoading)

  // Get print jobs for all designs
  const jobQueries = allDesigns.map((design) =>
    // eslint-disable-next-line react-hooks/rules-of-hooks
    useQuery({
      queryKey: ['design-jobs', design.id],
      queryFn: () => designsApi.listPrintJobs(design.id),
      enabled: !!design.id,
    })
  )

  const allJobs = jobQueries.flatMap((q) => q.data || [])
  const isLoadingJobs = jobQueries.some((q) => q.isLoading)
  const { data: queueData, isLoading: isLoadingQueue } = useQuery({
    queryKey: ['queue', 'project-history', projectId],
    queryFn: () => queueApi.get(),
  })
  const projectQueueItems = (queueData?.items || [])
    .map(item => item.item)
    .filter(item => item.project_id === projectId && ['done', 'failed', 'cancelled', 'printing', 'paused', 'ready', 'queued'].includes(item.status))

  // Create lookup maps
  const designMap = Object.fromEntries(allDesigns.map((d) => [d.id, d]))
  const printerMap = Object.fromEntries(printers.map((p) => [p.id, p]))

  // Sort jobs by created_at descending
  const sortedJobs = [...allJobs].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  )

  const formatDuration = (startedAt?: string, completedAt?: string) => {
    if (!startedAt) return '-'
    const start = new Date(startedAt)
    const end = completedAt ? new Date(completedAt) : new Date()
    const diffMs = end.getTime() - start.getTime()
    const hours = Math.floor(diffMs / (1000 * 60 * 60))
    const mins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60))
    if (hours > 0) return t('{{hours}}h {{minutes}}m', { hours, minutes: mins })
    return t('{{minutes}}m', { minutes: mins })
  }

  if (isLoadingDesigns || isLoadingJobs || isLoadingQueue) {
    return <div className="text-surface-500">{t("Loading print history...")}</div>
  }

  if (allJobs.length === 0 && projectQueueItems.length === 0) {
    return (
      <div className="card p-8 text-center">
        <History className="h-12 w-12 mx-auto mb-3 text-surface-600" />
        <h3 className="text-lg font-medium text-surface-300 mb-2">
          {t("No print history yet")}</h3>
        <p className="text-surface-500">
          {t("Print jobs will appear here once you start printing")}</p>
      </div>
    )
  }

  // Use server-side stats when available, fallback to client-side
  const totalMaterialUsed = allJobs.reduce(
    (sum, job) => sum + (job.outcome?.material_used || 0),
    0
  ) + projectQueueItems.filter(item => item.status === 'done').reduce((sum, item) => sum + (item.filament_grams || 0), 0)
  const totalPrintSeconds = projectQueueItems.filter(item => item.status === 'done').reduce((sum, item) => sum + (item.estimated_seconds || 0), 0)
  const totalCost = allJobs.reduce(
    (sum, job) => sum + (job.outcome?.material_cost || 0),
    0
  )

  const projectQueueCompleted = projectQueueItems.filter(item => item.status === 'done').length
  const projectQueueFailed = projectQueueItems.filter(item => item.status === 'failed' || item.status === 'cancelled').length
  const projectQueuePrinting = projectQueueItems.filter(item => item.status === 'printing' || item.status === 'paused').length
  const projectQueueQueued = projectQueueItems.filter(item => item.status === 'queued' || item.status === 'ready').length
  const statsTotal = (jobStats?.total ?? allJobs.length) + projectQueueItems.length
  const statsCompleted = (jobStats?.completed ?? allJobs.filter((j) => j.outcome?.success === true).length) + projectQueueCompleted
  const statsFailed = (jobStats?.failed ?? allJobs.filter((j) => j.outcome?.success === false || j.status === 'failed').length) + projectQueueFailed
  const statsPrinting = (jobStats?.printing ?? allJobs.filter((j) => j.status === 'printing').length) + projectQueuePrinting
  const statsQueued = (jobStats?.queued ?? 0) + projectQueueQueued

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-sm text-surface-500">{t("Print History")}</div>
        <div className="flex gap-2">
          <button onClick={clearHistory} className="btn btn-secondary text-xs py-1 px-3 flex items-center gap-1">
            <Trash2 className="h-3.5 w-3.5" /> {confirmClearHistory ? t("Clear all print history? This cannot be undone.") : t("Clear history")}
          </button>
        </div>
      </div>
      {historyError && <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{historyError}</div>}
      {/* Stats Summary */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">{t("Total Prints")}</div>
          <div className="text-2xl font-semibold text-surface-100">
            {statsTotal}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">{t("Success Rate")}</div>
          <div className="text-2xl font-semibold text-emerald-400">
            {(statsCompleted + statsFailed) > 0
              ? Math.round((statsCompleted / (statsCompleted + statsFailed)) * 100)
              : 0}%
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">{t("Active")}</div>
          <div className="text-2xl font-semibold text-blue-400">
            {statsPrinting}{statsQueued > 0 && <span className="text-sm text-surface-400 ml-1">+{statsQueued} {t("queued")}</span>}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">{t("Material Used")}</div>
          <div className="text-2xl font-semibold text-surface-100">
            {t('{{value1}}g', { value1: totalMaterialUsed.toFixed(0) })}</div>
          <div className="mt-1 text-xs text-surface-500">{formatPrintTime(totalPrintSeconds, t)} {t("print time")}</div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">{t("Total Cost")}</div>
          <div className="text-2xl font-semibold text-emerald-400">
            {formatAmount(totalCost)}
          </div>
        </div>
      </div>

      {/* Job List */}
      <div className="space-y-3">
      {projectQueueItems.map((item) => (
        <div key={item.id} className="card p-4 border-blue-500/20">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div className={cn('w-10 h-10 rounded-lg flex items-center justify-center', item.status === 'done' ? 'bg-emerald-500/20 text-emerald-400' : item.status === 'failed' ? 'bg-red-500/20 text-red-400' : 'bg-blue-500/20 text-blue-400')}>
                {item.status === 'done' ? <CheckCircle className="h-5 w-5" /> : item.status === 'failed' ? <XCircle className="h-5 w-5" /> : <Clock className="h-5 w-5" />}
              </div>
              <div>
                <div className="font-medium text-surface-100">{item.display_name || item.file_name}</div>
                <div className="text-sm text-surface-500 flex items-center gap-2">
                  <span>{t("Queue from Projects")}</span>
                  <span>·</span>
                  <span>{formatRelativeTime(item.created_at)}</span>
                  {item.estimated_seconds && <><span>·</span><span>{formatPrintTime(item.estimated_seconds, t)}</span></>}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="text-right text-sm text-surface-300">
                {item.filament_grams ? `${Math.round(item.filament_grams)}g` : t("No grams")}
                {item.material_type && <span className="ml-2 text-surface-500">{item.material_type.toUpperCase()}</span>}
              </div>
              <span className={cn('badge', getStatusBadge(item.status))}>{t(({ pending: 'Pending', in_progress: 'In Progress', completed: 'Completed', cancelled: 'Cancelled', printing: 'Printing', paused: 'Paused', failed: 'Failed', queued: 'Queued', idle: 'Idle', offline: 'Offline', online: 'Online', error: 'Error', ready: 'Ready', sent: 'Sent', info: 'Info', success: 'Success', warning: 'Warning', critical: 'Critical', channels: 'Channels', templates: 'Templates', deliveries: 'Deliveries' } as Record<string, string>)[item.status] || item.status)}</span>
            </div>
          </div>
        </div>
      ))}
      {sortedJobs.map((job) => {
        const design = designMap[job.design_id]
        const printer = job.printer_id ? printerMap[job.printer_id] : undefined

        return (
          <div key={job.id} className="card p-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div
                  className={cn(
                    'w-10 h-10 rounded-lg flex items-center justify-center',
                    job.status === 'completed' && job.outcome?.success
                      ? 'bg-emerald-500/20 text-emerald-400'
                      : job.status === 'failed' || (job.outcome && !job.outcome.success)
                      ? 'bg-red-500/20 text-red-400'
                      : job.status === 'printing'
                      ? 'bg-blue-500/20 text-blue-400'
                      : 'bg-surface-700 text-surface-400'
                  )}
                >
                  {job.status === 'completed' && job.outcome?.success ? (
                    <CheckCircle className="h-5 w-5" />
                  ) : job.status === 'failed' ||
                    (job.outcome && !job.outcome.success) ? (
                    <XCircle className="h-5 w-5" />
                  ) : job.status === 'printing' ? (
                    <Printer className="h-5 w-5" />
                  ) : (
                    <Clock className="h-5 w-5" />
                  )}
                </div>

                <div>
                  <div className="font-medium text-surface-100">
                    {design?.file_name || t("Unknown design")}
                  </div>
                  <div className="text-sm text-surface-500 flex items-center gap-2">
                    <span>{printer?.name || t("Unknown printer")}</span>
                    <span>·</span>
                    <span>{formatRelativeTime(job.created_at)}</span>
                    {job.started_at && (
                      <>
                        <span>·</span>
                        <span>{formatDuration(job.started_at, job.completed_at)}</span>
                      </>
                    )}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-3">
                {job.outcome && (
                  <div className="text-right text-sm">
                    <div className="text-surface-300">
                      {job.outcome.material_used.toFixed(1)}{t("g used")}{job.outcome.material_cost > 0 && (
                        <span className="text-emerald-400 ml-2">
                          {formatAmount(job.outcome.material_cost)}
                        </span>
                      )}
                    </div>
                    {job.outcome.quality_rating && (
                      <div className="flex items-center gap-0.5 justify-end">
                        {[1, 2, 3, 4, 5].map((star) => (
                          <Star
                            key={star}
                            className={cn(
                              'h-3 w-3',
                              star <= job.outcome!.quality_rating!
                                ? 'fill-amber-400 text-amber-400'
                                : 'text-surface-600'
                            )}
                          />
                        ))}
                      </div>
                    )}
                  </div>
                )}

                <span
                  className={cn(
                    'badge',
                    getStatusBadge(
                      job.outcome?.success === false ? 'failed' : job.status
                    )
                  )}
                >
                  {job.outcome?.success === false
                    ? t("Failed")
                    : job.status === 'completed' && job.outcome?.success
                    ? t("Success")
                    : t(PRINT_JOB_STATUS_LABELS[job.status] || job.status)}
                </span>

                {/* Handle Failure button for failed jobs without outcome */}
                {job.status === 'failed' && !job.outcome && (
                  <button
                    onClick={() => onHandleFailure(job)}
                    className="btn btn-primary text-xs py-1 px-2 flex items-center gap-1"
                  >
                    <RefreshCw className="h-3 w-3" />
                    {t("Handle Failure")}</button>
                )}

                {/* Record Outcome button for completed jobs without outcome */}
                {job.status === 'completed' && !job.outcome && (
                  <button
                    onClick={() => onRecordOutcome(job)}
                    className="btn btn-secondary text-xs py-1 px-2"
                  >
                    {t("Record Outcome")}</button>
                )}

                <button
                  onClick={() => deleteJob(job.id)}
                  className={cn(
                    'rounded-lg border px-2 py-1 text-xs font-semibold transition-colors flex items-center gap-1',
                    confirmDeleteJob === job.id
                      ? 'border-red-500/70 bg-red-500/20 text-red-300 hover:bg-red-500/30'
                      : 'border-surface-700 bg-surface-800/50 text-surface-400 hover:text-red-300 hover:border-red-500/40'
                  )}
                >
                  <Trash2 className="h-3 w-3" />
                  {confirmDeleteJob === job.id ? t("Confirm") : t("Delete")}
                </button>
              </div>
            </div>

            {/* Expandable event timeline */}
            <div className="mt-3 pt-3 border-t border-surface-800">
              <ExpandableJobEvents jobId={job.id} />
            </div>
          </div>
        )
      })}
      </div>
    </div>
  )
}

// Quick Stats Bar (above tabs)
function ProjectQuickStats({ summary }: { summary: ProjectSummary }) {
  const { t } = useTranslation('production')

  const { formatCents } = useCurrency()

  const formatTime = (seconds: number) => {
    if (seconds <= 0) return '-'
    const hours = Math.floor(seconds / 3600)
    const mins = Math.floor((seconds % 3600) / 60)
    if (hours > 0) return t('{{hours}}h {{minutes}}m', { hours, minutes: mins })
    return t('{{minutes}}m', { minutes: mins })
  }

  const printSeconds = summary.total_print_seconds > 0
    ? summary.total_print_seconds
    : summary.estimated_print_seconds
  const materialGrams = summary.total_material_grams > 0
    ? summary.total_material_grams
    : summary.estimated_material_grams
  const avgProfit = summary.sales_count > 0
    ? Math.round(summary.gross_profit_cents / summary.sales_count)
    : 0

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <Timer className="h-5 w-5 text-blue-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">{t("Print Time")}</div>
          <div className="text-sm font-semibold text-surface-100 truncate">
            {formatTime(printSeconds)}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <DollarSign className="h-5 w-5 text-amber-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">{t("Unit Cost")}</div>
          <div className="text-sm font-semibold text-surface-100 truncate">
            {formatCents(summary.unit_cost_cents)}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <Scale className="h-5 w-5 text-purple-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">{t("Material")}</div>
          <div className="text-sm font-semibold text-surface-100 truncate">
            {materialGrams > 0 ? `${materialGrams.toFixed(0)}g` : '-'}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <TrendingUp className="h-5 w-5 text-emerald-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">{t("Avg Profit / Sale")}</div>
          <div className={cn(
            'text-sm font-semibold truncate',
            summary.sales_count > 0
              ? avgProfit >= 0 ? 'text-emerald-400' : 'text-red-400'
              : 'text-surface-500'
          )}>
            {summary.sales_count > 0 ? formatCents(avgProfit) : '-'}
          </div>
        </div>
      </div>
    </div>
  )
}

// Project Analytics Tab
function ProjectAnalyticsTab({ summary }: { summary?: ProjectSummary }) {
  const { t } = useTranslation('production')

  const { formatAmount, formatCents } = useCurrency()
  if (!summary) {
    return (
      <div className="card p-8 text-center">
        <BarChart3 className="h-12 w-12 mx-auto mb-3 text-surface-600" />
        <h3 className="text-lg font-medium text-surface-300 mb-2">
          {t("No analytics yet")}</h3>
        <p className="text-surface-500">
          {t("Complete some print jobs and record sales to see project analytics")}</p>
      </div>
    )
  }

  const formatSeconds = (seconds: number) => {
    if (seconds <= 0) return '-'
    const hours = Math.floor(seconds / 3600)
    const mins = Math.floor((seconds % 3600) / 60)
    if (hours > 0) return t('{{hours}}h {{minutes}}m', { hours, minutes: mins })
    return t('{{minutes}}m', { minutes: mins })
  }

  return (
    <div className="space-y-6">
      {/* Revenue Section */}
      <div>
        <h3 className="text-sm font-medium text-surface-400 uppercase tracking-wider mb-3 flex items-center gap-2">
          <DollarSign className="h-4 w-4" />
          {t("Revenue")}</h3>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Gross Revenue")}<Tooltip text={t("Total amount collected from all sales of this project before any deductions.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.total_revenue_cents)}
            </div>
            {summary.sales_count > 0 && (
              <div className="text-xs text-surface-500 mt-1">
                {t('{{count}} sale', { count: summary.sales_count })}
              </div>
            )}
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Fees")}<Tooltip text={t("Marketplace and payment processing fees deducted from sales (e.g. Etsy fees, PayPal fees).")} />
            </div>
            <div className="text-2xl font-semibold text-red-400">
              {formatCents(summary.total_fees_cents)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Net Revenue")}<Tooltip text={t("Gross revenue minus fees. The actual amount received after marketplace and payment deductions.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.net_revenue_cents)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Gross Profit")}<Tooltip text={t("Net revenue minus total cost of goods sold (COGS). This is the profit after accounting for all production costs and fees.")} />
            </div>
            <div className={cn(
              'text-2xl font-semibold',
              summary.gross_profit_cents >= 0 ? 'text-emerald-400' : 'text-red-400'
            )}>
              {formatCents(summary.gross_profit_cents)}
            </div>
            {summary.gross_margin_percent > 0 && (
              <div className="text-xs text-surface-500 mt-1">
                {summary.gross_margin_percent.toFixed(1)}{t("% margin")}</div>
            )}
          </div>
        </div>
      </div>

      {/* Cost Breakdown */}
      <div>
        <h3 className="text-sm font-medium text-surface-400 uppercase tracking-wider mb-3 flex items-center gap-2">
          <TrendingUp className="h-4 w-4" />
          {t("Cost Breakdown")}</h3>
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Unit Cost")}<Tooltip text={t("Total cost to produce one unit of this project, including printer time, material, and supplies.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.unit_cost_cents)}
            </div>
            <div className="text-xs text-surface-500 mt-1">{t("per unit produced")}</div>
          </div>
          {summary.sales_count > 1 && (
            <div className="card p-4">
              <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
                {t("Total COGS")}<Tooltip text={t("Cost of Goods Sold. Unit cost multiplied by the number of units sold. Represents total production cost for all sales.")} />
              </div>
              <div className="text-2xl font-semibold text-red-400">
                {formatCents(summary.total_cost_cents)}
              </div>
              <div className="text-xs text-surface-500 mt-1">
                {summary.sales_count} {t("units sold")}</div>
            </div>
          )}
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Printer Time")}<Tooltip text={t("Cost of printer usage based on each printer's hourly rate multiplied by actual print time from completed jobs.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.printer_time_cost_cents)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Material (Actual)")}<Tooltip text={t("Actual filament cost recorded when print jobs complete, calculated from material used and spool cost per kg.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.material_cost_cents)}
            </div>
            {summary.total_material_grams > 0 && (
              <div className="text-xs text-surface-500 mt-1">
                {summary.total_material_grams.toFixed(0)}{t("g used")}</div>
            )}
          </div>
          {summary.estimated_material_cost_cents > 0 && (
            <div className="card p-4">
              <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
                {t("Est. Material")}<Tooltip text={t("Estimated material cost calculated from slice profile weight data at a default rate of {{value1}}/kg. Used when actual costs aren't available.", { value1: formatAmount(19.99) })} />
              </div>
              <div className="text-2xl font-semibold text-amber-400">
                {formatCents(summary.estimated_material_cost_cents)}
              </div>
              <div className="text-xs text-surface-500 mt-1">{t("from slice profiles")}</div>
            </div>
          )}
          {summary.supply_cost_cents > 0 && (
            <div className="card p-4">
              <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
                {t("Supplies")}<Tooltip text={t("Total cost of non-printed items added to this project's bill of materials (e.g. hardware, wiring, packaging).")} />
              </div>
              <div className="text-2xl font-semibold text-surface-100">
                {formatCents(summary.supply_cost_cents)}
              </div>
              <div className="text-xs text-surface-500 mt-1">{t("non-printed items")}</div>
            </div>
          )}
        </div>
      </div>

      {/* Performance */}
      <div>
        <h3 className="text-sm font-medium text-surface-400 uppercase tracking-wider mb-3 flex items-center gap-2">
          <Timer className="h-4 w-4" />
          {t("Performance")}</h3>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Total Print Time")}<Tooltip text={t("Combined print time across all jobs. Uses actual time from completed jobs, or estimated time from slice profiles if no jobs have run.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatSeconds(summary.total_print_seconds || summary.estimated_print_seconds)}
            </div>
            {summary.total_print_seconds <= 0 && summary.estimated_print_seconds > 0 && (
              <div className="text-xs text-surface-500 mt-1">{t("estimated from slices")}</div>
            )}
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Avg Print Time")}<Tooltip text={t("Average print time per completed job. Calculated from actual completed print job durations.")} />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatSeconds(summary.avg_print_seconds)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Success Rate")}<Tooltip text={t("Percentage of print jobs that completed successfully out of all completed and failed jobs.")} />
            </div>
            <div className={cn(
              'text-2xl font-semibold',
              summary.success_rate >= 90 ? 'text-emerald-400' :
              summary.success_rate >= 70 ? 'text-amber-400' : 'text-red-400'
            )}>
              {summary.success_rate.toFixed(0)}%
            </div>
            <div className="text-xs text-surface-500 mt-1">
              {summary.completed_count}/{summary.job_count} {t("jobs")}</div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              {t("Profit / Hour")}<Tooltip text={t("Gross profit divided by total print hours. Measures how efficiently this project converts printer time into profit.")} />
            </div>
            <div className={cn(
              'text-2xl font-semibold',
              summary.profit_per_hour_cents >= 0 ? 'text-emerald-400' : 'text-red-400'
            )}>
              {formatCents(summary.profit_per_hour_cents)}
            </div>
            <div className="text-xs text-surface-500 mt-1">{t("per print hour")}</div>
          </div>
        </div>
      </div>
    </div>
  )
}

// Upload Design Modal
function UploadDesignModal({
  part,
  onClose,
  onSuccess,
}: {
  part: Part
  onClose: () => void
  onSuccess: () => void
}) {
  const { t } = useTranslation('production')

  const [selectedGCodeFile, setSelectedGCodeFile] = useState<RootProjectFile | null>(null)
  const [notes, setNotes] = useState('')
  const [uploading, setUploading] = useState(false)
  const [showPicker, setShowPicker] = useState(false)
  const [error, setError] = useState('')

  const handleUpload = async () => {
    if (!selectedGCodeFile) return

    setUploading(true)
    setError('')
    try {
      await designsApi.linkRootFile(part.id, { type: selectedGCodeFile.type, id: selectedGCodeFile.file.id }, notes)
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to add design"))
    } finally {
      setUploading(false)
    }
  }

  return (
    <Modal title={t("Add Design for {{value1}}", { value1: part.name })} onClose={onClose}>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            {t("Design File (G-code)")}</label>
          {selectedGCodeFile ? (
            <div className="rounded-lg border border-accent-500/40 bg-accent-500/5 p-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded bg-surface-800 overflow-hidden">
                    {selectedGCodeFile.file.thumbnail_file_id ? (
                      <img src={resourceURL(`/api/files/${selectedGCodeFile.file.thumbnail_file_id}`)} className="w-full h-full object-cover" />
                    ) : (
                      <FileCode className="h-5 w-5 m-2.5 text-surface-500" />
                    )}
                  </div>
                  <div>
                    <div className="font-medium text-surface-100">{selectedGCodeFile.file.display_name}</div>
                    <div className="text-xs text-surface-500 uppercase">{selectedGCodeFile.type}</div>
                  </div>
                </div>
                <button onClick={() => setSelectedGCodeFile(null)} className="text-surface-400 hover:text-surface-200"><X className="h-4 w-4" /></button>
              </div>
            </div>
          ) : (
            <button type="button" onClick={() => setShowPicker(true)} className="w-full rounded-lg border border-surface-700 bg-surface-900/40 px-4 py-4 text-sm text-surface-300 hover:border-accent-500 hover:text-accent-400">
              {t("Select a file from Files")}</button>
          )}
        </div>
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-1">
            {t("Notes")}</label>
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={2}
            className="input resize-none"
            placeholder={t("Optional notes about this version")}
          />
        </div>
        {error && <div className="text-sm text-red-400">{error}</div>}
      </div>
      {showPicker && (
        <GCodeFilePicker
          stlWithGCodeOnly
          onSelect={(item) => {
            setSelectedGCodeFile(item)
            setShowPicker(false)
          }}
          onClose={() => setShowPicker(false)}
        />
      )}
      <div className="flex justify-end gap-3 mt-6">
        <button onClick={onClose} className="btn btn-ghost">
          {t("Cancel")}</button>
        <button
          onClick={handleUpload}
          disabled={!selectedGCodeFile || uploading}
          className="btn btn-primary"
        >
          {uploading ? t("Adding...") : t("Add design")}
        </button>
      </div>
    </Modal>
  )
}

// Spool with material info type
interface SpoolWithMaterial {
  id: string
  material_id: string
  initial_weight: number
  remaining_weight: number
  status: string
  material?: Material
}

// Send to Printer Modal
function SendToPrinterModal({
  design,
  printers,
  printerStates,
  onClose,
}: {
  design: Design
  printers: { id: string; name: string; model: string }[]
  printerStates: Record<string, { status: string; progress: number }>
  onClose: () => void
}) {
  const { t } = useTranslation('production')

  const [selectedPrinter, setSelectedPrinter] = useState('')
  const [selectedSpool, setSelectedSpool] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState('')
  const queryClient = useQueryClient()

  const { data: spoolsWithMaterials = [] } = useSpoolsWithMaterials()

  // Filter spools to show only available ones (not empty or archived)
  const availableSpools = spoolsWithMaterials.filter(
    (spool: SpoolWithMaterial) =>
      spool.status !== 'empty' && spool.status !== 'archived'
  )

  // Show warning if spool has low remaining weight
  const LOW_WEIGHT_THRESHOLD = 100 // grams

  const handleSend = async () => {
    if (!selectedPrinter || !selectedSpool) return

    setSending(true)
    setSendError('')
    try {
      const job = await printJobsApi.create({
        design_id: design.id,
        printer_id: selectedPrinter,
        material_spool_id: selectedSpool,
      })

      await queueApi.fromPrintJob(job.id, {})

      queryClient.invalidateQueries({ queryKey: ['print-jobs'] })
      queryClient.invalidateQueries({ queryKey: ['queue'] })
      onClose()
    } catch (err) {
      setSendError(err instanceof Error ? err.message : t("Failed to add to queue"))
    } finally {
      setSending(false)
    }
  }

  const availablePrinters = printers.filter(
    (p) => printerStates[p.id]?.status === 'idle' || !printerStates[p.id]
  )

  return (
    <Modal title={t("Add to queue")} onClose={onClose}>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            {t("Design")}</label>
          <div className="p-3 rounded-lg bg-surface-800/50">
            <div className="font-medium text-surface-100">
              {t('v{{version}}', { version: design.version })} — {design.file_name}
            </div>
            <div className="text-sm text-surface-500">
              {formatBytes(design.file_size_bytes)}
            </div>
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            {t("Select Printer")}</label>
          {availablePrinters.length === 0 ? (
            <div className="text-surface-500 text-sm p-4 text-center bg-surface-800/50 rounded-lg">
              {t("No printers available. All printers are busy or offline.")}</div>
          ) : (
            <div className="space-y-2">
              {availablePrinters.map((printer) => (
                <label
                  key={printer.id}
                  className={cn(
                    'flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-colors',
                    selectedPrinter === printer.id
                      ? 'bg-accent-500/10 border border-accent-500'
                      : 'bg-surface-800/50 border border-transparent hover:bg-surface-800'
                  )}
                >
                  <input
                    type="radio"
                    name="printer"
                    value={printer.id}
                    checked={selectedPrinter === printer.id}
                    onChange={(e) => setSelectedPrinter(e.target.value)}
                    className="sr-only"
                  />
                  <div className="w-3 h-3 rounded-full border-2 border-current flex items-center justify-center">
                    {selectedPrinter === printer.id && (
                      <div className="w-1.5 h-1.5 rounded-full bg-current" />
                    )}
                  </div>
                  <div>
                    <div className="font-medium text-surface-100">
                      {printer.name}
                    </div>
                    <div className="text-xs text-surface-500">
                      {printer.model || t("Unknown model")}
                    </div>
                  </div>
                </label>
              ))}
            </div>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            {t("Select Material Spool")}</label>
          {availableSpools.length === 0 ? (
            <div className="text-surface-500 text-sm p-4 text-center bg-surface-800/50 rounded-lg">
              {t("No spools available. Add material spools in the Materials page.")}</div>
          ) : (
            <div className="space-y-2 max-h-48 overflow-y-auto">
              {availableSpools.map((spool: SpoolWithMaterial) => {
                const isLow = spool.remaining_weight < LOW_WEIGHT_THRESHOLD
                return (
                  <label
                    key={spool.id}
                    className={cn(
                      'flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-colors',
                      selectedSpool === spool.id
                        ? 'bg-accent-500/10 border border-accent-500'
                        : 'bg-surface-800/50 border border-transparent hover:bg-surface-800'
                    )}
                  >
                    <input
                      type="radio"
                      name="spool"
                      value={spool.id}
                      checked={selectedSpool === spool.id}
                      onChange={(e) => setSelectedSpool(e.target.value)}
                      className="sr-only"
                    />
                    <div className="w-3 h-3 rounded-full border-2 border-current flex items-center justify-center">
                      {selectedSpool === spool.id && (
                        <div className="w-1.5 h-1.5 rounded-full bg-current" />
                      )}
                    </div>
                    {spool.material?.color_hex && (
                      <div
                        className="w-4 h-4 rounded-full border border-surface-600"
                        style={{ backgroundColor: spool.material.color_hex }}
                      />
                    )}
                    <div className="flex-1 min-w-0">
                      <div className="font-medium text-surface-100 truncate">
                        {spool.material?.name || t("Unknown material")}
                      </div>
                      <div className="text-xs text-surface-500 flex items-center gap-2">
                        <span className="uppercase">
                          {spool.material?.type || '?'}
                        </span>
                        <span>•</span>
                        <span
                          className={cn(
                            isLow && 'text-amber-400 font-medium'
                          )}
                        >
                          {spool.remaining_weight.toFixed(0)}{t("g remaining")}</span>
                        {isLow && (
                          <AlertTriangle className="h-3 w-3 text-amber-400" />
                        )}
                      </div>
                    </div>
                  </label>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {sendError && <div className="mt-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{sendError}</div>}

      <div className="flex justify-end gap-3 mt-6">
        <button onClick={onClose} className="btn btn-ghost">
          {t("Cancel")}</button>
        <button
          onClick={handleSend}
          disabled={!selectedPrinter || !selectedSpool || sending}
          className="btn btn-primary"
        >
          <Play className="h-4 w-4 mr-2" />
          {sending ? t("Adding...") : t("Add to queue")}
        </button>
      </div>
    </Modal>
  )
}

// Generic Modal Component
function Modal({
  title,
  children,
  onClose,
}: {
  title: string
  children: React.ReactNode
  onClose: () => void
}) {
  useTranslation('production')

  return (
    <div
      className="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <div className="card w-full max-w-md p-6">
        <h2 className="text-xl font-semibold text-surface-100 mb-4">{title}</h2>
        {children}
      </div>
    </div>
  )
}

// Outcome Capture Modal
function OutcomeCaptureModal({
  job,
  onClose,
  onSuccess,
}: {
  job: PrintJob
  onClose: () => void
  onSuccess: () => void
}) {
  const { t } = useTranslation('production')

  const [success, setSuccess] = useState(true)
  const [qualityRating, setQualityRating] = useState(4)
  const [materialUsed, setMaterialUsed] = useState('')
  const [notes, setNotes] = useState('')
  const [failureReason, setFailureReason] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async () => {
    setSubmitting(true)
    try {
      const materialGrams = parseFloat(materialUsed) || 0

      await printJobsApi.recordOutcome(job.id, {
        success,
        quality_rating: success ? qualityRating : undefined,
        failure_reason: !success ? failureReason : undefined,
        notes: notes || undefined,
        material_used: materialGrams,
        material_cost: 0, // Will be calculated by backend
      })
      onSuccess()
    } catch (err) {
      console.error('Failed to record outcome:', err)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal title={t("Record Print Outcome")} onClose={onClose}>
      <div className="space-y-4">
        {/* Success/Failure Toggle */}
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            {t("Print Result")}</label>
          <div className="grid grid-cols-2 gap-2">
            <button
              type="button"
              onClick={() => setSuccess(true)}
              className={cn(
                'flex items-center justify-center gap-2 p-3 rounded-lg border transition-colors',
                success
                  ? 'bg-emerald-500/20 border-emerald-500 text-emerald-400'
                  : 'bg-surface-800/50 border-surface-700 text-surface-400 hover:bg-surface-800'
              )}
            >
              <CheckCircle className="h-5 w-5" />
              {t("Success")}</button>
            <button
              type="button"
              onClick={() => setSuccess(false)}
              className={cn(
                'flex items-center justify-center gap-2 p-3 rounded-lg border transition-colors',
                !success
                  ? 'bg-red-500/20 border-red-500 text-red-400'
                  : 'bg-surface-800/50 border-surface-700 text-surface-400 hover:bg-surface-800'
              )}
            >
              <XCircle className="h-5 w-5" />
              {t("Failed")}</button>
          </div>
        </div>

        {/* Quality Rating (only for success) */}
        {success && (
          <div>
            <label className="block text-sm font-medium text-surface-300 mb-2">
              {t("Quality Rating")}</label>
            <div className="flex gap-1">
              {[1, 2, 3, 4, 5].map((rating) => (
                <button
                  key={rating}
                  type="button"
                  onClick={() => setQualityRating(rating)}
                  className="p-1"
                >
                  <Star
                    className={cn(
                      'h-6 w-6 transition-colors',
                      rating <= qualityRating
                        ? 'fill-amber-400 text-amber-400'
                        : 'text-surface-600'
                    )}
                  />
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Failure Reason (only for failure) */}
        {!success && (
          <div>
            <label className="block text-sm font-medium text-surface-300 mb-1">
              {t("Failure Reason")}</label>
            <input
              type="text"
              value={failureReason}
              onChange={(e) => setFailureReason(e.target.value)}
              className="input"
              placeholder={t("e.g., Bed adhesion, spaghetti, layer shift")}
            />
          </div>
        )}

        {/* Material Used */}
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-1">
            {t("Material Used (grams)")}</label>
          <input
            type="number"
            value={materialUsed}
            onChange={(e) => setMaterialUsed(e.target.value)}
            className="input"
            placeholder={t("e.g., 25")}
            min="0"
            step="0.1"
          />
          <p className="text-xs text-surface-500 mt-1">
            {t("Enter the amount of material consumed during this print")}</p>
        </div>

        {/* Notes */}
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-1">
            {t("Notes")}</label>
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={2}
            className="input resize-none"
            placeholder={t("Optional notes about this print")}
          />
        </div>
      </div>

      <div className="flex justify-end gap-3 mt-6">
        <button onClick={onClose} className="btn btn-ghost">
          {t("Cancel")}</button>
        <button
          onClick={handleSubmit}
          disabled={submitting}
          className="btn btn-primary"
        >
          {submitting ? t("Saving...") : t("Save Outcome")}
        </button>
      </div>
    </Modal>
  )
}

