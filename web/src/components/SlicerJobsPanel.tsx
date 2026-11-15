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
  queued: 'Na fila', running: 'Fatiando e importando', succeeded: 'Concluído',
  failed: 'Falhou', cancelled: 'Cancelado antes de iniciar', interrupted: 'Interrompido',
}

export default function SlicerJobsPanel({ api, stlFileId, onOpenResult }: Props) {
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
        if (!stopped) setError(e instanceof Error ? e.message : 'Falha ao consultar tarefas')
      } finally {
        if (!stopped) timer = setTimeout(() => void poll(), 3000)
      }
    }
    void poll()
    return () => { stopped = true; mounted.current = false; clearTimeout(timer) }
  }, [api, refresh])

  async function cancel(id: string) {
    setBusy(id)
    try {
      const updated = await api.cancel(id)
      if (mounted.current) setJobs(rows => rows.map(row => row.id === id ? updated : row))
    } catch (e) {
      if (mounted.current) setError(e instanceof Error ? e.message : 'Não foi possível cancelar')
    } finally {
      if (mounted.current) { setBusy(null); setRefresh(value => value + 1) }
    }
  }

  async function remove(job: SlicerJobView) {
    if (!window.confirm('Remover do histórico? O G-code permanece na biblioteca, mas a chave de deduplicação será esquecida. Uma nova submissão dos mesmos parâmetros poderá gerar outro G-code.')) return
    setBusy(job.id)
    try {
      await api.delete(job.id)
      forgetSlicerSubmission(job.idempotency_key)
      if (mounted.current) setJobs(rows => rows.filter(row => row.id !== job.id))
    } catch (e) {
      if (mounted.current) setError(e instanceof Error ? e.message : 'Falha ao remover histórico; atualize antes de tentar novamente')
    } finally {
      if (mounted.current) { setBusy(null); setRefresh(value => value + 1) }
    }
  }

  const visible = stlFileId ? jobs.filter(job => job.request.stl_file_id === stlFileId) : jobs
  return <section className="space-y-3" aria-label="Fatiamentos em segundo plano">
    <h3 className="font-semibold">Fatiamentos em segundo plano</h3>
    <p className="text-sm text-gray-500">Você pode navegar enquanto o PicoFarm estiver em execução. Reinícios interrompem tarefas; não há retomada automática. Somente tarefas na fila podem ser canceladas.</p>
    <p className="text-xs text-gray-500">Histórico: {jobs.length}/100. Ao atingir o limite, remova tarefas encerradas explicitamente para liberar espaço. Remover histórico não apaga o G-code e perde a deduplicação daquela tarefa.</p>
    {error && <div role="alert" className="text-sm text-red-600">{error} <button type="button" onClick={() => setRefresh(value => value + 1)}>Atualizar</button></div>}
    {visible.length === 0 && <p className="text-sm text-gray-500">Nenhum fatiamento registrado.</p>}
    <ul className="space-y-2">{visible.map(job => <li key={job.id} className="rounded border p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span>{job.request.display_name || job.request.stl_file_id}</span>
        <span>{labels[job.status]}</span>
      </div>
      <div className="text-xs text-gray-500">{new Date(job.created_at).toLocaleString()}</div>
      {job.error && <p>{job.error}</p>}
      {job.persistence_error && <p role="alert" className="text-red-600">Não foi possível salvar o estado. Confira a biblioteca antes de tentar novamente; novas tarefas estão bloqueadas.</p>}
      {job.status === 'queued' && <button type="button" disabled={busy !== null} onClick={() => void cancel(job.id)}>{busy === job.id ? 'Cancelando…' : 'Cancelar na fila'}</button>}
      {job.status !== 'queued' && job.status !== 'running' && <button type="button" disabled={busy !== null} onClick={() => void remove(job)}>Remover do histórico</button>}
      {job.result?.gcode && <div>G-code importado: {job.result.gcode.display_name || job.result.gcode.id}
        {onOpenResult && <button type="button" onClick={() => onOpenResult(job.result!.gcode.id)}>Abrir resultado</button>}
      </div>}
    </li>)}</ul>
  </section>
}
