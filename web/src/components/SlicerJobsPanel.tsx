import { useTranslation } from 'react-i18next'
import { useEffect, useRef, useState } from 'react'
import { forgetSlicerSubmission } from '../lib/slicerJobSubmission'

export interface SlicerJobView {
  id: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted'
  request: { stl_file_id: string; display_name?: string }
  result?: { gcode: { id: string; display_name?: string } }
  error?: string
  idempotency_key: string
  persistence_error?: boolean
  created_at: string
  updated_at: string
}

// Inject the shared authenticated client. Do not duplicate token handling here.
export interface SlicerJobsTransport {
  list: () => Promise<SlicerJobView[]>
  cancel: (id: string) => Promise<SlicerJobView>
  delete: (id: string) => Promise<void>
}

interface Props {
  api: SlicerJobsTransport
  stlFileId?: string
  onOpenResult?: (gcodeId: string) => void
}

const labels: Record<SlicerJobView['status'], string> = {
  queued: "Queued", running: 'Slicing and importing', succeeded: "Completed",
  failed: 'Failed', cancelled: 'Cancelled before starting', interrupted: 'Interrupted',
}

function formatJobDate(dateString: string, dateLocale: string): string {
  const date = new Date(dateString)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleString(dateLocale)
}

export default function SlicerJobsPanel({ api, stlFileId, onOpenResult }: Props) {
  const { t, i18n } = useTranslation('components')
  const dateLocale = i18n.resolvedLanguage === 'pt-BR' ? 'pt-BR' : 'en-US'
  const [jobs, setJobs] = useState<SlicerJobView[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<string | null>(null)
  const [refresh, setRefresh] = useState(0)
  const mounted = useRef(false)

  useEffect(() => {
    mounted.current = true
    let stopped = false
    let timer: ReturnType<typeof setTimeout>
    // Schedule after completion: never overlap slow polls. Unmount stops polling,
    // not jobs; backend execution is independent of this component's lifetime.
    async function poll() {
      try {
        const rows = await api.list()
        if (!stopped) { setJobs(rows); setError('') }
      } catch (e) {
        if (!stopped) setError(e instanceof Error ? e.message : t("Failed to load slicing jobs"))
      } finally {
        if (!stopped) timer = setTimeout(() => void poll(), 3000)
      }
    }
    void poll()
    return () => { stopped = true; mounted.current = false; clearTimeout(timer) }
  }, [api, refresh, t])

  async function cancel(id: string) {
    setBusy(id)
    try {
      const updated = await api.cancel(id)
      if (mounted.current) setJobs(rows => rows.map(row => row.id === id ? updated : row))
    } catch (e) {
      if (mounted.current) setError(e instanceof Error ? e.message : t("Could not cancel"))
    } finally {
      if (mounted.current) { setBusy(null); setRefresh(value => value + 1) }
    }
  }

  async function remove(job: SlicerJobView) {
    if (!window.confirm(t('Remove from history? The G-code stays in the library, but its deduplication key will be forgotten. Submitting the same parameters again may generate another G-code.'))) return
    setBusy(job.id)
    try {
      await api.delete(job.id)
      forgetSlicerSubmission(job.idempotency_key)
      if (mounted.current) setJobs(rows => rows.filter(row => row.id !== job.id))
    } catch (e) {
      if (mounted.current) setError(e instanceof Error ? e.message : t("Failed to remove history; refresh before retrying"))
    } finally {
      if (mounted.current) { setBusy(null); setRefresh(value => value + 1) }
    }
  }

  const visible = stlFileId ? jobs.filter(job => job.request.stl_file_id === stlFileId) : jobs
  return <section className="space-y-3" aria-label={t("Background slicing jobs")}>
    <h3 className="font-semibold">{t("Background slicing jobs")}</h3>
    <p className="text-sm text-gray-500">{t("You can navigate while PicoFarm is running. Restarts interrupt jobs; there is no automatic recovery. Only queued jobs can be cancelled.")}</p>
    <p className="text-xs text-gray-500">{t("History: {{count}}/100. When the limit is reached, explicitly remove finished jobs to free space. Removing history keeps the G-code but loses that job's deduplication.", { count: jobs.length })}</p>
    {error && <div role="alert" className="text-sm text-red-600">{error} <button type="button" onClick={() => setRefresh(value => value + 1)}>{t("Refresh")}</button></div>}
    {visible.length === 0 && <p className="text-sm text-gray-500">{t("No slicing jobs recorded.")}</p>}
    <ul className="space-y-2">{visible.map(job => <li key={job.id} className="rounded border p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span>{job.request.display_name || job.request.stl_file_id}</span>
        <span>{t(labels[job.status] || job.status)}</span>
      </div>
      <div className="text-xs text-gray-500">{formatJobDate(job.created_at, dateLocale)}</div>
      {job.error && <p>{job.error}</p>}
      {job.persistence_error && <p role="alert" className="text-red-600">{t("Could not save state. Check the library before retrying; new jobs are blocked.")}</p>}
      {job.status === 'queued' && <button type="button" disabled={busy !== null} onClick={() => void cancel(job.id)}>{busy === job.id ? t("Cancelling…") : t("Cancel queued job")}</button>}
      {job.status !== 'queued' && job.status !== 'running' && <button type="button" disabled={busy !== null} onClick={() => void remove(job)}>{t("Remove from history")}</button>}
      {job.result?.gcode && <div>{t("Imported G-code:")}{' '}{job.result.gcode.display_name || job.result.gcode.id}
        {onOpenResult && <button type="button" onClick={() => onOpenResult(job.result!.gcode.id)}>{t("Open result")}</button>}
      </div>}
    </li>)}</ul>
  </section>
}
