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
import { designsApi, printJobsApi, projectsApi, partsApi, suppliesApi, materialsApi, queueApi, gcodeLibraryApi, fileLibraryApi, enqueueProjectParts } from '../api/client'
import { cn, getStatusBadge, formatBytes, formatRelativeTime } from '../lib/utils'
import AppToast, { type AppToastState } from '../components/AppToast'
import { FailureModal } from '../components/FailureModal'
import { ExpandableJobEvents } from '../components/JobEventTimeline'
import { Tooltip } from '../components/Tooltip'
import GCodeFilePicker from '../components/GCodeFilePicker'
import type { Design, Part, Material, PrintJob, ProjectSummary, ProjectSupply, GCodeLibraryFile, STLLibraryFile } from '../types'

type RootProjectFile = { type: 'stl'; file: STLLibraryFile } | { type: 'gcode'; file: GCodeLibraryFile }

export default function ProjectDetail() {
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
  const [projectForm, setProjectForm] = useState({ name: '', description: '' })
  const [projectEditError, setProjectEditError] = useState('')
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
        setProjectToast({ title: 'Project queued', message: `${added} item(s) added to the print queue.`, tone: 'success' })
        queryClient.invalidateQueries({ queryKey: ['queue'] })
      } else if (missing > 0) {
        setProjectToast({ title: 'No G-code found', message: `${missing} part(s) do not have valid G-code.`, tone: 'info' })
      } else {
        setProjectToast({ title: 'Nothing queued', message: 'No items were added to the print queue.', tone: 'info' })
      }
    } catch (err) {
      setProjectToast({ title: 'Failed to queue project', message: err instanceof Error ? err.message : String(err), tone: 'error' })
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
        <div className="text-surface-500">Loading...</div>
      </div>
    )
  }

  if (!project) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="text-surface-500">Project not found</div>
      </div>
    )
  }

  const startProjectEdit = () => {
    setProjectForm({ name: project.name || '', description: project.description || '' })
    setProjectEditError('')
    setEditingProject(true)
  }

  const saveProjectEdit = async () => {
    setProjectEditError('')
    try {
      await updateProject.mutateAsync({ id: project.id, data: projectForm })
      setEditingProject(false)
    } catch (err) {
      setProjectEditError(err instanceof Error ? err.message : 'Failed to update project')
    }
  }

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
          Back to Projects
        </Link>
        
          <div className="card p-5">
          {editingProject ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-surface-500 mb-1">Nome do projeto</label>
                <input value={projectForm.name} onChange={e => setProjectForm(prev => ({ ...prev, name: e.target.value }))} className="input text-xl font-semibold" autoFocus />
              </div>
              <div>
                <label className="block text-xs text-surface-500 mb-1">Descrição</label>
                <textarea value={projectForm.description} onChange={e => setProjectForm(prev => ({ ...prev, description: e.target.value }))} rows={2} className="input resize-none" placeholder="Descrição opcional" />
              </div>
              {projectEditError && <div className="text-sm text-red-400">{projectEditError}</div>}
              <div className="flex gap-2">
                <button onClick={saveProjectEdit} disabled={updateProject.isPending} className="btn btn-primary text-sm">{updateProject.isPending ? 'Salvando...' : 'Salvar'}</button>
                <button onClick={() => setEditingProject(false)} className="btn btn-secondary text-sm">Cancelar</button>
              </div>
            </div>
          ) : (
            <div className="flex items-start justify-between gap-4">
              <div>
                <h1 className="text-3xl font-display font-bold text-surface-100">
                  {project.name}
                </h1>
                {project.description && (
                  <p className="text-surface-400 mt-1">{project.description}</p>
                )}
              </div>
              <div className="flex items-center gap-3">
                <button onClick={handlePrintProject} disabled={printingProject} className="btn btn-primary" title="Print whole project">
                  <Play className="h-4 w-4 mr-2" />
                  {printingProject ? 'Queueing...' : 'Print Project'}
                </button>
                <button onClick={startProjectEdit} className="btn btn-secondary text-sm">Editar</button>
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
            Printer Fleet
          </h2>
        </div>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          {printers.length === 0 ? (
            <div className="col-span-4 text-center py-4 text-surface-500">
              <Link to="/printers" className="text-accent-400 hover:text-accent-300">
                Add printers to get started
              </Link>
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
                      {state?.status || 'offline'}
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
          Parts
        </button>
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
          Print History
        </button>
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
          Analytics
        </button>
      </div>

      {/* Parts Tab */}
      {activeTab === 'parts' && (
        <>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-semibold text-surface-100">Parts</h2>
            <button
              onClick={() => setShowAddPart(true)}
              className="btn btn-secondary"
            >
              <Plus className="h-4 w-4 mr-2" />
              Add Part
            </button>
          </div>

          {partDeleteError && <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{partDeleteError}</div>}
          {partsLoading ? (
            <div className="text-surface-500">Loading parts...</div>
          ) : parts.length === 0 ? (
            <div className="card p-8 text-center">
              <Box className="h-12 w-12 mx-auto mb-3 text-surface-600" />
              <h3 className="text-lg font-medium text-surface-300 mb-2">
                No parts yet
              </h3>
              <p className="text-surface-500 mb-4">
                Add parts to start organizing your project
              </p>
              <button
                onClick={() => setShowAddPart(true)}
                className="btn btn-primary"
              >
                <Plus className="h-4 w-4 mr-2" />
                Add First Part
              </button>
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
                    if (!confirm(`Delete part "${part.name}"? This cannot be undone.`)) return
                    try {
                      setPartDeleteError('')
                      await partsApi.delete(part.id)
                      queryClient.invalidateQueries({ queryKey: ['parts', id] })
                    } catch (err) {
                      setPartDeleteError(err instanceof Error ? err.message : 'Failed to delete part')
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
        <Modal title="Add Part" onClose={() => setShowAddPart(false)}>
          <form onSubmit={handleAddPart}>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-surface-300 mb-2">Arquivos da aba Files</label>
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
                    Selecionar STL/G-code da aba Files
                  </button>
                )}
                {selectedRootFiles.length > 0 && <button type="button" onClick={() => setShowGCodePicker(true)} className="mt-2 text-xs text-accent-400 hover:text-accent-300">Alterar seleção</button>}
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                type="button"
                onClick={() => { setShowAddPart(false); setSelectedRootFiles([]) }}
                className="btn btn-ghost"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={createPart.isPending || selectedRootFiles.length === 0}
                className="btn btn-primary"
              >
                {createPart.isPending ? 'Adding...' : selectedRootFiles.length > 1 ? 'Add Parts' : 'Add Part'}
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

function formatPrintTime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function InfoRow({ label, value }: { label: string; value: string }) {
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
  const getGCodeProfile = (gcode: GCodeLibraryFile) => gcode.metadata?.print_settings_id || 'No profile'

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
      setToast({ title: 'Print added to queue', message: gcode.display_name || gcode.file_name, tone: 'success' })
      setTimeout(() => setToast(null), 3500)
    } catch (err) {
      setPartError(err instanceof Error ? err.message : 'Failed to add print to queue')
    } finally {
      setPartBusy('')
    }
  }

  const printDesign = async (design: Design) => {
    if (design.file_type === 'stl') {
      const stl = getDesignSTL(design)
      const gcode = stl?.gcodes?.find(file => file.default_for_stl) || stl?.gcodes?.[0]
      if (!gcode) {
        setPartError('STL has no linked G-code')
        return
      }
      await printGCode(gcode)
      return
    }
    if (design.file_type === 'gcode') {
      const gcode = getDesignGCode(design)
      if (!gcode) {
        setPartError('G-code not found in library')
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
          {gcode.default_for_stl && <span className="rounded-full border border-emerald-500/40 bg-emerald-500/15 px-2 py-0.5 text-[10px] font-bold uppercase text-emerald-300">Default</span>}
        </div>
        <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-surface-500">
          <span>Perfil: {getGCodeProfile(gcode)}</span>
          {gcode.material_type && <span>{gcode.material_type.toUpperCase()}</span>}
          {gcode.layer_height && <span>{gcode.layer_height}mm</span>}
          {gcode.estimated_seconds && <span>{formatPrintTime(gcode.estimated_seconds)}</span>}
          {gcode.filament_grams && <span>{Math.round(gcode.filament_grams)}g</span>}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <select value={selectedSpoolID} onChange={e => setSelectedGCodeMaterials(current => ({ ...current, [gcode.id]: e.target.value }))} className="input w-56 py-1 text-xs" disabled={matchingSpools.length === 0}>
          <option value="">{gcodeMaterialType ? `Spool ${gcodeMaterialType.toUpperCase()}` : 'Tipo não detectado'}</option>
          {matchingSpools.map(spool => <option key={spool.id} value={spool.id}>{spool.material ? `${spool.material.name} · ${spool.material.type.toUpperCase()}` : 'Unknown'} · {Math.round(spool.remaining_weight)}g{spool.default_for_material ? ' · Default' : ''}</option>)}
        </select>
        {!gcode.default_for_stl && gcode.parent_stl_id && <button onClick={() => void makeDefaultGCode(gcode)} className="btn btn-ghost text-xs py-1.5 px-2">Make default</button>}
        <button onClick={() => setViewingGCode(gcode)} className="btn btn-ghost text-xs py-1.5 px-2" title="G-code info"><Info className="h-3.5 w-3.5" /></button>
        <button onClick={() => void printGCode(gcode)} className="btn btn-primary text-xs py-1.5 px-3">
          <Play className="h-3.5 w-3.5 mr-1" />
          {partBusy === gcode.id ? 'Adding...' : 'Queue'}
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

        </div>
        <div className="flex items-center gap-2">
          <span className={cn('badge', getStatusBadge(part.status))}>
            {part.status}
          </span>
          <button
            onClick={onDelete}
            className="p-1.5 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
            title="Delete part"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Designs */}
      <div className="border-t border-surface-800 pt-4">
        <div className="flex items-center justify-between mb-3">
          <span className="text-sm font-medium text-surface-400">
            Designs ({designs.length} version{designs.length !== 1 ? 's' : ''})
          </span>
          <button onClick={onAddDesign} className="btn btn-secondary text-xs py-1.5 px-3"><Plus className="h-3.5 w-3.5 mr-1" />Add Design</button>
        </div>

        {partError && <div className="mb-3 rounded-lg border border-red-500/30 bg-red-500/10 p-2 text-sm text-red-300">{partError}</div>}
        {designs.length === 0 ? (
          <div className="text-center py-4 text-surface-500 text-sm">
            No designs uploaded yet
          </div>
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
                        {stl?.thumbnail_file_id ? <img src={`/api/files/${stl.thumbnail_file_id}`} className="h-full w-full object-contain" /> : <FileCode className="h-5 w-5 text-surface-500" />}
                      </div>
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-surface-200 truncate">
                          v{design.version} — {stl?.display_name || gcode?.display_name || design.file_name}
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
                            designsApi.openExternal(design.id, 'BambuStudio').catch((err) => {
                              alert('Failed to open Bambu Studio: ' + err.message)
                            })
                          }}
                          className="btn btn-ghost text-xs py-1.5 px-3"
                          title="Open in Bambu Studio"
                        >
                          <ExternalLink className="h-3.5 w-3.5 mr-1" />
                          Bambu Studio
                        </button>
                      )}
                      {stl && <button onClick={() => setViewingSTL(stl)} className="btn btn-ghost text-xs py-1.5 px-2" title="STL info"><Info className="h-3.5 w-3.5" /></button>}
                      {design.file_type !== 'stl' && (
                        <button onClick={() => void printDesign(design)} className="btn btn-primary text-xs py-1.5 px-3">
                          <Play className="h-3.5 w-3.5 mr-1" />
                          {partBusy === design.id || partBusy === gcode?.id ? 'Adding...' : 'Queue'}
                        </button>
                      )}
                      <button
                        onClick={async () => { if (!confirm(`Delete design ${design.file_name}?`)) return; await designsApi.delete(design.id); queryClient.invalidateQueries({ queryKey: ['designs', part.id] }) }}
                        className="btn btn-ghost text-xs py-1.5 px-2 text-red-400"
                        title="Delete design"
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
        <Modal title="STL info" onClose={() => setViewingSTL(null)}>
          <div className="space-y-3 text-sm">
            {viewingSTL.thumbnail_file_id && <img src={`/api/files/${viewingSTL.thumbnail_file_id}`} className="h-48 w-full rounded-lg bg-surface-900 object-contain" />}
            <InfoRow label="Title" value={viewingSTL.display_name || viewingSTL.file_name} />
            <InfoRow label="File" value={viewingSTL.file_name} />
            <InfoRow label="Size" value={formatBytes(viewingSTL.size_bytes)} />
            <InfoRow label="Linked G-codes" value={`${viewingSTL.gcodes?.length || 0}`} />
          </div>
        </Modal>
      )}
      {viewingGCode && (
        <Modal title="G-code info" onClose={() => setViewingGCode(null)}>
          <div className="space-y-3 text-sm">
            {viewingGCode.thumbnail_file_id && <img src={`/api/files/${viewingGCode.thumbnail_file_id}`} className="h-48 w-full rounded-lg bg-surface-900 object-contain" />}
            <InfoRow label="Title" value={viewingGCode.display_name || viewingGCode.file_name} />
            <InfoRow label="File" value={viewingGCode.file_name} />
            <InfoRow label="Perfil de Impressão" value={viewingGCode.metadata?.print_settings_id || '-'} />
            <InfoRow label="Perfil de Impressora" value={viewingGCode.metadata?.printer_settings_id || '-'} />
            <InfoRow label="Perfil de Filamento" value={viewingGCode.metadata?.filament_settings_id || viewingGCode.filament_name || '-'} />
            <InfoRow label="Impressora" value={viewingGCode.metadata?.printer_model || '-'} />
            <InfoRow label="Material" value={viewingGCode.material_type?.toUpperCase() || '-'} />
            <InfoRow label="Weight" value={viewingGCode.filament_grams ? `${Math.round(viewingGCode.filament_grams)}g` : '-'} />
            <InfoRow label="Time" value={viewingGCode.estimated_seconds ? formatPrintTime(viewingGCode.estimated_seconds) : '-'} />
            <InfoRow label="Layer" value={viewingGCode.layer_height ? `${viewingGCode.layer_height}mm` : '-'} />
            <InfoRow label="Nozzle" value={viewingGCode.nozzle_diameter ? `${viewingGCode.nozzle_diameter}mm` : '-'} />
          </div>
        </Modal>
      )}
    </div>
  )
}

// Supplies Section Component
function SuppliesSection({ projectId }: { projectId: string }) {
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
          Supplies
        </h2>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="btn btn-secondary"
        >
          <Plus className="h-4 w-4 mr-2" />
          Add Supply
        </button>
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
                From Catalog
              </button>
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
                Manual Entry
              </button>
            </div>
          )}

          {addMode === 'catalog' && supplyMaterials.length > 0 ? (
            <form onSubmit={handleCatalogAdd}>
              <div className="grid grid-cols-[1fr_auto_auto_auto] gap-3 items-end">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">Supply Material</label>
                  <select
                    value={selectedMaterialId}
                    onChange={(e) => handleMaterialSelect(e.target.value)}
                    className="input"
                    required
                  >
                    <option value="">Select a supply...</option>
                    {supplyMaterials.map((mat) => (
                      <option key={mat.id} value={mat.id}>
                        {mat.name} {mat.manufacturer ? `(${mat.manufacturer})` : ''} — ${mat.cost_per_kg.toFixed(2)}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">Unit Cost ($)</label>
                  <input
                    type="number"
                    value={newCost}
                    onChange={(e) => setNewCost(e.target.value)}
                    className="input w-28"
                    placeholder="0.00"
                    min="0"
                    step="0.01"
                    readOnly
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">Qty</label>
                  <input
                    type="number"
                    value={newQuantity}
                    onChange={(e) => setNewQuantity(e.target.value)}
                    className="input w-20"
                    min="1"
                  />
                </div>
                <div className="flex gap-2">
                  <button type="submit" className="btn btn-primary" disabled={!selectedMaterialId}>Add</button>
                  <button type="button" onClick={() => setShowAddForm(false)} className="btn btn-ghost">
                    Cancel
                  </button>
                </div>
              </div>
            </form>
          ) : (
            <form onSubmit={handleManualAdd}>
              <div className="grid grid-cols-[1fr_auto_auto_auto] gap-3 items-end">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">Name</label>
                  <input
                    type="text"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    className="input"
                    placeholder="e.g., Lamp cord, Lightbulb"
                    autoFocus
                    required
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">Unit Cost ($)</label>
                  <input
                    type="number"
                    value={newCost}
                    onChange={(e) => setNewCost(e.target.value)}
                    className="input w-28"
                    placeholder="0.00"
                    min="0"
                    step="0.01"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">Qty</label>
                  <input
                    type="number"
                    value={newQuantity}
                    onChange={(e) => setNewQuantity(e.target.value)}
                    className="input w-20"
                    min="1"
                  />
                </div>
                <div className="flex gap-2">
                  <button type="submit" className="btn btn-primary">Add</button>
                  <button type="button" onClick={() => setShowAddForm(false)} className="btn btn-ghost">
                    Cancel
                  </button>
                </div>
              </div>
            </form>
          )}
        </div>
      )}

      {supplies.length === 0 ? (
        <div className="card p-6 text-center text-surface-500 text-sm">
          No supplies added yet. Add non-printed items like lamp cords, lightbulbs, etc.
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-surface-800">
                <th className="text-left text-xs font-medium text-surface-500 uppercase px-4 py-2">Item</th>
                <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">Unit Cost</th>
                <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">Qty</th>
                <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">Total</th>
                <th className="w-10 px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {supplies.map((supply) => (
                <tr key={supply.id} className="border-b border-surface-800/50">
                  <td className="px-4 py-3 text-surface-200">{supply.name}</td>
                  <td className="px-4 py-3 text-right text-surface-300">
                    ${(supply.unit_cost_cents / 100).toFixed(2)}
                  </td>
                  <td className="px-4 py-3 text-right text-surface-300">{supply.quantity}</td>
                  <td className="px-4 py-3 text-right text-surface-100 font-medium">
                    ${((supply.unit_cost_cents * supply.quantity) / 100).toFixed(2)}
                  </td>
                  <td className="px-4 py-3">
                    {confirmDeleteSupplyId === supply.id ? (
                      <button
                        onClick={() => handleDelete(supply)}
                        onBlur={() => setConfirmDeleteSupplyId(null)}
                        className="text-xs px-2 py-1 rounded bg-red-500/20 text-red-400 border border-red-500/50 hover:bg-red-500/30 transition-colors"
                        autoFocus
                      >
                        Delete?
                      </button>
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
                  Total Supplies
                </td>
                <td className="px-4 py-3 text-right font-semibold text-surface-100">
                  ${(totalCents / 100).toFixed(2)}
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
      setHistoryError(err instanceof Error ? err.message : 'Failed to delete print record')
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
      setHistoryError(err instanceof Error ? err.message : 'Failed to clear print history')
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
    if (hours > 0) return `${hours}h ${mins}m`
    return `${mins}m`
  }

  if (isLoadingDesigns || isLoadingJobs || isLoadingQueue) {
    return <div className="text-surface-500">Loading print history...</div>
  }

  if (allJobs.length === 0 && projectQueueItems.length === 0) {
    return (
      <div className="card p-8 text-center">
        <History className="h-12 w-12 mx-auto mb-3 text-surface-600" />
        <h3 className="text-lg font-medium text-surface-300 mb-2">
          No print history yet
        </h3>
        <p className="text-surface-500">
          Print jobs will appear here once you start printing
        </p>
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
        <div className="text-sm text-surface-500">Print History</div>
        <div className="flex gap-2">
          <button onClick={clearHistory} className="btn btn-secondary text-xs py-1 px-3 flex items-center gap-1">
            <Trash2 className="h-3.5 w-3.5" /> {confirmClearHistory ? 'Confirmar limpar?' : 'Limpar histórico'}
          </button>
        </div>
      </div>
      {historyError && <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{historyError}</div>}
      {/* Stats Summary */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">Total Prints</div>
          <div className="text-2xl font-semibold text-surface-100">
            {statsTotal}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">Success Rate</div>
          <div className="text-2xl font-semibold text-emerald-400">
            {(statsCompleted + statsFailed) > 0
              ? Math.round((statsCompleted / (statsCompleted + statsFailed)) * 100)
              : 0}%
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">Active</div>
          <div className="text-2xl font-semibold text-blue-400">
            {statsPrinting}{statsQueued > 0 && <span className="text-sm text-surface-400 ml-1">+{statsQueued} queued</span>}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">Material Used</div>
          <div className="text-2xl font-semibold text-surface-100">
            {totalMaterialUsed.toFixed(0)}g
          </div>
          <div className="mt-1 text-xs text-surface-500">{formatPrintTime(totalPrintSeconds)} print time</div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-surface-500 mb-1">Total Cost</div>
          <div className="text-2xl font-semibold text-emerald-400">
            ${totalCost.toFixed(2)}
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
                  <span>Queue from Projects</span>
                  <span>·</span>
                  <span>{formatRelativeTime(item.created_at)}</span>
                  {item.estimated_seconds && <><span>·</span><span>{formatPrintTime(item.estimated_seconds)}</span></>}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="text-right text-sm text-surface-300">
                {item.filament_grams ? `${Math.round(item.filament_grams)}g` : 'No grams'}
                {item.material_type && <span className="ml-2 text-surface-500">{item.material_type.toUpperCase()}</span>}
              </div>
              <span className={cn('badge', getStatusBadge(item.status))}>{item.status}</span>
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
                    {design?.file_name || 'Unknown design'}
                  </div>
                  <div className="text-sm text-surface-500 flex items-center gap-2">
                    <span>{printer?.name || 'Unknown printer'}</span>
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
                      {job.outcome.material_used.toFixed(1)}g used
                      {job.outcome.material_cost > 0 && (
                        <span className="text-emerald-400 ml-2">
                          ${job.outcome.material_cost.toFixed(2)}
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
                    ? 'failed'
                    : job.status === 'completed' && job.outcome?.success
                    ? 'success'
                    : job.status}
                </span>

                {/* Handle Failure button for failed jobs without outcome */}
                {job.status === 'failed' && !job.outcome && (
                  <button
                    onClick={() => onHandleFailure(job)}
                    className="btn btn-primary text-xs py-1 px-2 flex items-center gap-1"
                  >
                    <RefreshCw className="h-3 w-3" />
                    Handle Failure
                  </button>
                )}

                {/* Record Outcome button for completed jobs without outcome */}
                {job.status === 'completed' && !job.outcome && (
                  <button
                    onClick={() => onRecordOutcome(job)}
                    className="btn btn-secondary text-xs py-1 px-2"
                  >
                    Record Outcome
                  </button>
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
                  {confirmDeleteJob === job.id ? 'Confirmar' : 'Apagar'}
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
  const formatCents = (cents: number) => {
    const negative = cents < 0
    const abs = Math.abs(cents)
    return `${negative ? '-' : ''}$${(abs / 100).toFixed(2)}`
  }

  const formatTime = (seconds: number) => {
    if (seconds <= 0) return '-'
    const hours = Math.floor(seconds / 3600)
    const mins = Math.floor((seconds % 3600) / 60)
    if (hours > 0) return `${hours}h ${mins}m`
    return `${mins}m`
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
          <div className="text-xs text-surface-500">Print Time</div>
          <div className="text-sm font-semibold text-surface-100 truncate">
            {formatTime(printSeconds)}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <DollarSign className="h-5 w-5 text-amber-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">Unit Cost</div>
          <div className="text-sm font-semibold text-surface-100 truncate">
            {formatCents(summary.unit_cost_cents)}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <Scale className="h-5 w-5 text-purple-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">Material</div>
          <div className="text-sm font-semibold text-surface-100 truncate">
            {materialGrams > 0 ? `${materialGrams.toFixed(0)}g` : '-'}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-surface-800/50 border border-surface-700">
        <TrendingUp className="h-5 w-5 text-emerald-400 shrink-0" />
        <div className="min-w-0">
          <div className="text-xs text-surface-500">Avg Profit / Sale</div>
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
  if (!summary) {
    return (
      <div className="card p-8 text-center">
        <BarChart3 className="h-12 w-12 mx-auto mb-3 text-surface-600" />
        <h3 className="text-lg font-medium text-surface-300 mb-2">
          No analytics yet
        </h3>
        <p className="text-surface-500">
          Complete some print jobs and record sales to see project analytics
        </p>
      </div>
    )
  }

  const formatCents = (cents: number) => {
    const negative = cents < 0
    const abs = Math.abs(cents)
    return `${negative ? '-' : ''}$${(abs / 100).toFixed(2)}`
  }

  const formatSeconds = (seconds: number) => {
    if (seconds <= 0) return '-'
    const hours = Math.floor(seconds / 3600)
    const mins = Math.floor((seconds % 3600) / 60)
    if (hours > 0) return `${hours}h ${mins}m`
    return `${mins}m`
  }

  return (
    <div className="space-y-6">
      {/* Revenue Section */}
      <div>
        <h3 className="text-sm font-medium text-surface-400 uppercase tracking-wider mb-3 flex items-center gap-2">
          <DollarSign className="h-4 w-4" />
          Revenue
        </h3>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Gross Revenue
              <Tooltip text="Total amount collected from all sales of this project before any deductions." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.total_revenue_cents)}
            </div>
            {summary.sales_count > 0 && (
              <div className="text-xs text-surface-500 mt-1">
                {summary.sales_count} sale{summary.sales_count !== 1 ? 's' : ''}
              </div>
            )}
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Fees
              <Tooltip text="Marketplace and payment processing fees deducted from sales (e.g. Etsy fees, PayPal fees)." />
            </div>
            <div className="text-2xl font-semibold text-red-400">
              {formatCents(summary.total_fees_cents)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Net Revenue
              <Tooltip text="Gross revenue minus fees. The actual amount received after marketplace and payment deductions." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.net_revenue_cents)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Gross Profit
              <Tooltip text="Net revenue minus total cost of goods sold (COGS). This is the profit after accounting for all production costs and fees." />
            </div>
            <div className={cn(
              'text-2xl font-semibold',
              summary.gross_profit_cents >= 0 ? 'text-emerald-400' : 'text-red-400'
            )}>
              {formatCents(summary.gross_profit_cents)}
            </div>
            {summary.gross_margin_percent > 0 && (
              <div className="text-xs text-surface-500 mt-1">
                {summary.gross_margin_percent.toFixed(1)}% margin
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Cost Breakdown */}
      <div>
        <h3 className="text-sm font-medium text-surface-400 uppercase tracking-wider mb-3 flex items-center gap-2">
          <TrendingUp className="h-4 w-4" />
          Cost Breakdown
        </h3>
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Unit Cost
              <Tooltip text="Total cost to produce one unit of this project, including printer time, material, and supplies." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.unit_cost_cents)}
            </div>
            <div className="text-xs text-surface-500 mt-1">per unit produced</div>
          </div>
          {summary.sales_count > 1 && (
            <div className="card p-4">
              <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
                Total COGS
                <Tooltip text="Cost of Goods Sold. Unit cost multiplied by the number of units sold. Represents total production cost for all sales." />
              </div>
              <div className="text-2xl font-semibold text-red-400">
                {formatCents(summary.total_cost_cents)}
              </div>
              <div className="text-xs text-surface-500 mt-1">
                {summary.sales_count} units sold
              </div>
            </div>
          )}
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Printer Time
              <Tooltip text="Cost of printer usage based on each printer's hourly rate multiplied by actual print time from completed jobs." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.printer_time_cost_cents)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Material (Actual)
              <Tooltip text="Actual filament cost recorded when print jobs complete, calculated from material used and spool cost per kg." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatCents(summary.material_cost_cents)}
            </div>
            {summary.total_material_grams > 0 && (
              <div className="text-xs text-surface-500 mt-1">
                {summary.total_material_grams.toFixed(0)}g used
              </div>
            )}
          </div>
          {summary.estimated_material_cost_cents > 0 && (
            <div className="card p-4">
              <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
                Est. Material
                <Tooltip text="Estimated material cost calculated from slice profile weight data at a default rate of $19.99/kg. Used when actual costs aren't available." />
              </div>
              <div className="text-2xl font-semibold text-amber-400">
                {formatCents(summary.estimated_material_cost_cents)}
              </div>
              <div className="text-xs text-surface-500 mt-1">from slice profiles</div>
            </div>
          )}
          {summary.supply_cost_cents > 0 && (
            <div className="card p-4">
              <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
                Supplies
                <Tooltip text="Total cost of non-printed items added to this project's bill of materials (e.g. hardware, wiring, packaging)." />
              </div>
              <div className="text-2xl font-semibold text-surface-100">
                {formatCents(summary.supply_cost_cents)}
              </div>
              <div className="text-xs text-surface-500 mt-1">non-printed items</div>
            </div>
          )}
        </div>
      </div>

      {/* Performance */}
      <div>
        <h3 className="text-sm font-medium text-surface-400 uppercase tracking-wider mb-3 flex items-center gap-2">
          <Timer className="h-4 w-4" />
          Performance
        </h3>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Total Print Time
              <Tooltip text="Combined print time across all jobs. Uses actual time from completed jobs, or estimated time from slice profiles if no jobs have run." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatSeconds(summary.total_print_seconds || summary.estimated_print_seconds)}
            </div>
            {summary.total_print_seconds <= 0 && summary.estimated_print_seconds > 0 && (
              <div className="text-xs text-surface-500 mt-1">estimated from slices</div>
            )}
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Avg Print Time
              <Tooltip text="Average print time per completed job. Calculated from actual completed print job durations." />
            </div>
            <div className="text-2xl font-semibold text-surface-100">
              {formatSeconds(summary.avg_print_seconds)}
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Success Rate
              <Tooltip text="Percentage of print jobs that completed successfully out of all completed and failed jobs." />
            </div>
            <div className={cn(
              'text-2xl font-semibold',
              summary.success_rate >= 90 ? 'text-emerald-400' :
              summary.success_rate >= 70 ? 'text-amber-400' : 'text-red-400'
            )}>
              {summary.success_rate.toFixed(0)}%
            </div>
            <div className="text-xs text-surface-500 mt-1">
              {summary.completed_count}/{summary.job_count} jobs
            </div>
          </div>
          <div className="card p-4">
            <div className="flex items-center gap-1.5 text-sm text-surface-500 mb-1">
              Profit / Hour
              <Tooltip text="Gross profit divided by total print hours. Measures how efficiently this project converts printer time into profit." />
            </div>
            <div className={cn(
              'text-2xl font-semibold',
              summary.profit_per_hour_cents >= 0 ? 'text-emerald-400' : 'text-red-400'
            )}>
              {formatCents(summary.profit_per_hour_cents)}
            </div>
            <div className="text-xs text-surface-500 mt-1">per print hour</div>
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
      setError(err instanceof Error ? err.message : 'Failed to add design')
    } finally {
      setUploading(false)
    }
  }

  return (
    <Modal title={`Add Design for ${part.name}`} onClose={onClose}>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            Design File (G-code)
          </label>
          {selectedGCodeFile ? (
            <div className="rounded-lg border border-accent-500/40 bg-accent-500/5 p-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded bg-surface-800 overflow-hidden">
                    {selectedGCodeFile.file.thumbnail_file_id ? (
                      <img src={`/api/files/${selectedGCodeFile.file.thumbnail_file_id}`} className="w-full h-full object-cover" />
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
              Selecionar arquivo da aba Files
            </button>
          )}
        </div>
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-1">
            Notes
          </label>
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={2}
            className="input resize-none"
            placeholder="Optional notes about this version"
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
          Cancel
        </button>
        <button
          onClick={handleUpload}
          disabled={!selectedGCodeFile || uploading}
          className="btn btn-primary"
        >
          {uploading ? 'Adicionando...' : 'Adicionar design'}
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
      setSendError(err instanceof Error ? err.message : 'Failed to add to queue')
    } finally {
      setSending(false)
    }
  }

  const availablePrinters = printers.filter(
    (p) => printerStates[p.id]?.status === 'idle' || !printerStates[p.id]
  )

  return (
    <Modal title="Adicionar à fila" onClose={onClose}>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            Design
          </label>
          <div className="p-3 rounded-lg bg-surface-800/50">
            <div className="font-medium text-surface-100">
              v{design.version} — {design.file_name}
            </div>
            <div className="text-sm text-surface-500">
              {formatBytes(design.file_size_bytes)}
            </div>
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            Select Printer
          </label>
          {availablePrinters.length === 0 ? (
            <div className="text-surface-500 text-sm p-4 text-center bg-surface-800/50 rounded-lg">
              No printers available. All printers are busy or offline.
            </div>
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
                      {printer.model || 'Unknown model'}
                    </div>
                  </div>
                </label>
              ))}
            </div>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            Select Material Spool
          </label>
          {availableSpools.length === 0 ? (
            <div className="text-surface-500 text-sm p-4 text-center bg-surface-800/50 rounded-lg">
              No spools available. Add material spools in the Materials page.
            </div>
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
                        {spool.material?.name || 'Unknown material'}
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
                          {spool.remaining_weight.toFixed(0)}g remaining
                        </span>
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
          Cancel
        </button>
        <button
          onClick={handleSend}
          disabled={!selectedPrinter || !selectedSpool || sending}
          className="btn btn-primary"
        >
          <Play className="h-4 w-4 mr-2" />
          {sending ? 'Adicionando...' : 'Adicionar à fila'}
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
    <Modal title="Record Print Outcome" onClose={onClose}>
      <div className="space-y-4">
        {/* Success/Failure Toggle */}
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-2">
            Print Result
          </label>
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
              Success
            </button>
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
              Failed
            </button>
          </div>
        </div>

        {/* Quality Rating (only for success) */}
        {success && (
          <div>
            <label className="block text-sm font-medium text-surface-300 mb-2">
              Quality Rating
            </label>
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
              Failure Reason
            </label>
            <input
              type="text"
              value={failureReason}
              onChange={(e) => setFailureReason(e.target.value)}
              className="input"
              placeholder="e.g., Bed adhesion, spaghetti, layer shift"
            />
          </div>
        )}

        {/* Material Used */}
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-1">
            Material Used (grams)
          </label>
          <input
            type="number"
            value={materialUsed}
            onChange={(e) => setMaterialUsed(e.target.value)}
            className="input"
            placeholder="e.g., 25"
            min="0"
            step="0.1"
          />
          <p className="text-xs text-surface-500 mt-1">
            Enter the amount of material consumed during this print
          </p>
        </div>

        {/* Notes */}
        <div>
          <label className="block text-sm font-medium text-surface-300 mb-1">
            Notes
          </label>
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={2}
            className="input resize-none"
            placeholder="Optional notes about this print"
          />
        </div>
      </div>

      <div className="flex justify-end gap-3 mt-6">
        <button onClick={onClose} className="btn btn-ghost">
          Cancel
        </button>
        <button
          onClick={handleSubmit}
          disabled={submitting}
          className="btn btn-primary"
        >
          {submitting ? 'Saving...' : 'Save Outcome'}
        </button>
      </div>
    </Modal>
  )
}

