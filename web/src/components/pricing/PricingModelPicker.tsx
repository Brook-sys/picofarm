import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Box, Clock, FileCode, Layers, RefreshCw, Scissors, Search, Star } from 'lucide-react'
import { fileLibraryApi, slicerApi } from '../../api/client'
import { catalogSecondsToDuration } from '../../lib/pricingCalculator'
import { resourceURL } from '../../lib/platform'
import type { FileLibraryResponse, GCodeLibraryFile, STLLibraryFile } from '../../types'
import SliceSTLModal from '../SliceSTLModal'
import SlicerJobsPanel from '../SlicerJobsPanel'
import { PricingNotice, PricingSegments, pricingInputClass } from './PricingFields'

const slicerJobsTransport = {
  list: () => slicerApi.listJobs(),
  cancel: (id: string) => slicerApi.cancelJob(id),
  delete: (id: string) => slicerApi.deleteJob(id),
}

interface Props {
  selectedGCodeId?: string
  onSelectGCode: (file: GCodeLibraryFile, stl?: STLLibraryFile) => void
}

function ModelThumbnail({ id, name, className = '' }: { id?: string | null; name: string; className?: string }) {
  const [failedId, setFailedId] = useState<string | null>(null)
  return (
    <div className={`flex shrink-0 items-center justify-center overflow-hidden rounded-xl border border-surface-700 bg-surface-950 ${className}`}>
      {id && failedId !== id ? (
        <img src={resourceURL(`/api/files/${id}`)} alt="" className="h-full w-full object-cover" loading="lazy" onError={() => setFailedId(id)} />
      ) : (
        <Box className="h-8 w-8 text-surface-600" aria-label={name} />
      )}
    </div>
  )
}

function GCodeCard({ file, stl, selected, onSelect }: {
  file: GCodeLibraryFile
  stl?: STLLibraryFile
  selected: boolean
  onSelect: (file: GCodeLibraryFile, stl?: STLLibraryFile) => void
}) {
  const { t, i18n } = useTranslation('pricing')
  const name = file.display_name || file.file_name
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={() => onSelect(file, stl)}
      className={`group flex min-w-0 gap-3 rounded-xl border p-3 text-left transition-colors min-h-28 ${selected ? 'border-accent-500 bg-accent-500/10' : 'border-surface-700 bg-surface-950/60 hover:border-surface-500 hover:bg-surface-800/60'}`}
    >
      <ModelThumbnail id={file.thumbnail_file_id} name={name} className="h-20 w-20" />
      <span className="min-w-0 flex-1">
        <span className="flex items-start gap-2">
          <span className="min-w-0 flex-1 break-words text-sm font-medium text-surface-100">{name}</span>
          {file.default_for_stl && <Star className="h-4 w-4 shrink-0 fill-accent-400 text-accent-400" aria-label={t('Default G-code')} />}
        </span>
        <span className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-surface-400">
          <span className="inline-flex items-center gap-1"><Clock className="h-3.5 w-3.5" />{file.estimated_seconds == null ? t('Time unavailable') : t('{{duration}} h', { duration: catalogSecondsToDuration(file.estimated_seconds) })}</span>
          <span>{file.filament_grams == null ? t('Mass unavailable') : t('{{grams}} g', { grams: new Intl.NumberFormat(i18n.language).format(file.filament_grams) })}</span>
        </span>
        {(file.metadata?.printer_model || file.metadata?.printer_settings_id) && <span className="mt-1 block break-words text-xs text-surface-500">{file.metadata.printer_model || file.metadata.printer_settings_id}</span>}
        <span className={`mt-2 inline-flex items-center gap-1 text-xs font-medium ${selected ? 'text-accent-300' : 'text-surface-500 group-hover:text-surface-300'}`}>
          <FileCode className="h-3.5 w-3.5" />{t(selected ? 'Selected for this estimate' : 'Use these estimates')}
        </span>
      </span>
    </button>
  )
}

export default function PricingModelPicker({ selectedGCodeId = '', onSelectGCode }: Props) {
  const { t } = useTranslation('pricing')
  const [tab, setTab] = useState<'stl' | 'gcode'>('gcode')
  const [query, setQuery] = useState('')
  const [selectedSTLId, setSelectedSTLId] = useState('')
  const [slicing, setSlicing] = useState<STLLibraryFile | null>(null)
  const [showJobs, setShowJobs] = useState(false)
  const [actionError, setActionError] = useState('')
  const selectionGeneration = useRef(0)
  const mounted = useRef(false)
  useEffect(() => {
    const generation = selectionGeneration
    mounted.current = true
    return () => { mounted.current = false; generation.current++ }
  }, [])
  function selectFile(file: GCodeLibraryFile, stl?: STLLibraryFile) {
    selectionGeneration.current++
    onSelectGCode(file, stl)
  }
  const library = useQuery<FileLibraryResponse>({
    queryKey: ['pricing-file-library'],
    queryFn: () => fileLibraryApi.get(),
    staleTime: 30_000,
  })

  const normalizedQuery = query.trim().toLocaleLowerCase()
  const stls = useMemo(() => (library.data?.stl_files ?? []).filter(file =>
    !normalizedQuery || `${file.display_name} ${file.file_name}`.toLocaleLowerCase().includes(normalizedQuery),
  ), [library.data, normalizedQuery])
  const availableGCodes = useMemo(() => {
    const entries = [
      ...(library.data?.root_gcode_files ?? []).map(file => ({ file, stl: undefined as STLLibraryFile | undefined })),
      ...(library.data?.stl_files ?? []).flatMap(stl => (stl.gcodes ?? []).map(file => ({ file, stl }))),
    ]
    const seen = new Set<string>()
    return entries.filter(({ file, stl }) => {
      if (seen.has(file.id)) return false
      seen.add(file.id)
      return !normalizedQuery || `${file.display_name} ${file.file_name} ${stl?.display_name ?? ''}`.toLocaleLowerCase().includes(normalizedQuery)
    })
  }, [library.data, normalizedQuery])
  const [visibleLimit, setVisibleLimit] = useState(12)
  const selectedSTL = (library.data?.stl_files ?? []).find(file => file.id === selectedSTLId)

  async function selectJobResult(gcodeId: string) {
    const version = ++selectionGeneration.current
    setActionError('')
    const refreshed = await library.refetch()
    if (!mounted.current || selectionGeneration.current !== version) return
    const files = refreshed.data
    if (!files) {
      setActionError(t('The library could not be refreshed. Try again.'))
      return
    }
    for (const stl of files.stl_files) {
      const file = stl.gcodes?.find(gcode => gcode.id === gcodeId)
      if (file) {
        setSelectedSTLId(stl.id)
        onSelectGCode(file, stl)
        return
      }
    }
    const root = files.root_gcode_files.find(file => file.id === gcodeId)
    if (root) {
      onSelectGCode(root)
      return
    }
    setActionError(t('The generated G-code is no longer in the library. Refresh and inspect the slicing job before retrying.'))
  }

  async function finishSlice(createdFile?: GCodeLibraryFile) {
    setSlicing(null)
    setShowJobs(true)
    setActionError('')
    if (createdFile) await selectJobResult(createdFile.id)
    else await library.refetch()
  }

  return (
    <div className="space-y-4">
      <PricingSegments
        label={t('Estimate source')}
        value={tab}
        onChange={value => { selectionGeneration.current++; setTab(value); setVisibleLimit(12) }}
        options={[
          { value: 'gcode', label: t('Ready G-codes'), icon: <FileCode className="h-4 w-4 shrink-0" /> },
          { value: 'stl', label: t('STL models'), icon: <Box className="h-4 w-4 shrink-0" /> },
        ]}
      />
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-3.5 h-4 w-4 text-surface-500" />
        <input
          className={`${pricingInputClass} pl-9`}
          value={query}
          onChange={event => { setQuery(event.target.value); setVisibleLimit(12) }}
          placeholder={t(tab === 'stl' ? 'Search STL models' : 'Search all G-codes')}
          aria-label={t(tab === 'stl' ? 'Search STL models' : 'Search all G-codes')}
        />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <button type="button" className="btn btn-ghost min-h-11 gap-2 text-xs" disabled={library.isFetching} onClick={() => void library.refetch()}><RefreshCw className={`h-3.5 w-3.5 ${library.isFetching ? 'animate-spin' : ''}`} />{t('Refresh library')}</button>
        {selectedSTL && <button type="button" className="btn btn-ghost min-h-11 text-xs" aria-expanded={showJobs} onClick={() => setShowJobs(value => !value)}>{t('Show slicing jobs')}</button>}
      </div>
      {(library.data?.stl_files.length ?? 0) >= 1000 && <PricingNotice tone="amber">{t('Library snapshot may be incomplete. Models beyond the first 1,000 are not available here.')}</PricingNotice>}
      {library.isPending && <p role="status" className="flex items-center gap-2 text-sm text-surface-400"><RefreshCw className="h-4 w-4 animate-spin" />{t('Loading your model library…')}</p>}
      {library.isError && (
        <PricingNotice tone="red" role="alert">
          <p>{t('The model library could not be loaded. Manual entry remains available.')}</p>
          <button type="button" className="mt-2 underline underline-offset-2" onClick={() => void library.refetch()}>{t('Try again')}</button>
        </PricingNotice>
      )}
      {actionError && <PricingNotice tone="red" role="alert">{actionError}</PricingNotice>}

      {library.isSuccess && tab === 'stl' && !selectedSTL && (
        stls.length ? (
          <div className="grid gap-3 sm:grid-cols-2 min-[1400px]:grid-cols-3">
            {stls.map(file => {
              const name = file.display_name || file.file_name
              return (
                <button key={file.id} type="button" onClick={() => { selectionGeneration.current++; setSelectedSTLId(file.id); setShowJobs(false) }} className="group min-w-0 rounded-xl border border-surface-700 bg-surface-950/60 p-3 text-left hover:border-surface-500 hover:bg-surface-800/60">
                  <ModelThumbnail id={file.thumbnail_file_id} name={name} className="mb-3 aspect-[4/3] w-full" />
                  <span className="block break-words text-sm font-medium text-surface-100">{name}</span>
                  <span className="mt-1.5 flex items-center gap-1.5 text-xs text-surface-400"><Layers className="h-3.5 w-3.5" />{t('{{count}} associated G-codes', { count: file.gcodes?.length ?? 0 })}</span>
                  <span className="mt-2 inline-block text-xs font-medium text-accent-300">{t('Choose a G-code')}</span>
                </button>
              )
            })}
          </div>
        ) : <p className="rounded-xl border border-dashed border-surface-700 p-5 text-center text-sm text-surface-400">{t(normalizedQuery ? 'No STL models match this search.' : 'No STL models are available. Enter estimates manually or upload a model in Files.')}</p>
      )}

      {library.isSuccess && tab === 'stl' && selectedSTL && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-surface-700 bg-surface-950/60 p-3">
            <div className="flex min-w-0 items-center gap-3">
              <ModelThumbnail id={selectedSTL.thumbnail_file_id} name={selectedSTL.display_name} className="h-14 w-14" />
              <div className="min-w-0">
                <p className="break-words text-sm font-semibold text-surface-100">{selectedSTL.display_name || selectedSTL.file_name}</p>
                <button type="button" className="mt-1 inline-flex min-h-8 items-center gap-1 text-xs text-surface-400 hover:text-surface-100" onClick={() => { selectionGeneration.current++; setSelectedSTLId(''); setShowJobs(false) }}><ArrowLeft className="h-3.5 w-3.5" />{t('All STL models')}</button>
              </div>
            </div>
            <button type="button" className="btn btn-secondary min-h-11" onClick={() => setSlicing(selectedSTL)}><Scissors className="mr-2 h-4 w-4" />{t('Slice a new G-code')}</button>
          </div>

          <div>
            <h3 className="mb-2 text-sm font-medium text-surface-200">{t('Choose an existing G-code')}</h3>
            {selectedSTL.gcodes?.length ? (
              <div className="grid gap-3 md:grid-cols-2">
                {selectedSTL.gcodes.map(file => <GCodeCard key={file.id} file={file} stl={selectedSTL} selected={selectedGCodeId === file.id} onSelect={selectFile} />)}
              </div>
            ) : (
              <div className="rounded-xl border border-dashed border-surface-700 p-4 text-sm text-surface-400">
                <p>{t('This STL has no associated G-code yet.')}</p>
                <button type="button" className="mt-2 inline-flex min-h-10 items-center gap-2 text-accent-300 underline underline-offset-2" onClick={() => setSlicing(selectedSTL)}><Scissors className="h-4 w-4" />{t('Slice this STL online')}</button>
              </div>
            )}
          </div>
          {showJobs && (
            <div className="rounded-xl border border-surface-700 bg-surface-900/60 p-3">
              <SlicerJobsPanel api={slicerJobsTransport} stlFileId={selectedSTL.id} compact resultLabel={t('Use in estimate')} onOpenResult={id => void selectJobResult(id)} />
            </div>
          )}
        </div>
      )}

      {library.isSuccess && tab === 'gcode' && (
        availableGCodes.length ? (
          <div className="space-y-3">
            <div className="grid gap-3 md:grid-cols-2">
              {availableGCodes.slice(0, visibleLimit).map(({ file, stl }) => <GCodeCard key={file.id} file={file} stl={stl} selected={selectedGCodeId === file.id} onSelect={selectFile} />)}
            </div>
            {availableGCodes.length > visibleLimit && <button type="button" className="btn btn-secondary min-h-11 w-full" onClick={() => setVisibleLimit(limit => limit + 12)}>{t('Show more G-codes')}</button>}
          </div>
        ) : <p className="rounded-xl border border-dashed border-surface-700 p-5 text-center text-sm text-surface-400">{t(normalizedQuery ? 'No G-codes match this search.' : 'No ready G-codes yet. Choose an STL to slice, or enter the weight manually.')}</p>
      )}

      {slicing && <SliceSTLModal key={slicing.id} file={slicing} onClose={() => setSlicing(null)} onDone={file => void finishSlice(file)} />}
    </div>
  )
}
