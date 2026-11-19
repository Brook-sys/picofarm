import { useTranslation } from 'react-i18next'
import { useCallback, useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { ArrowLeft, Clock, Play, CheckCircle, XCircle, Package, Printer, AlertCircle, CalendarDays, Square, CheckSquare, DollarSign, Pencil, Trash2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { tasksApi, salesApi } from '../api/client'
import type { Task, TaskStatus, PrintJob, TaskChecklistItem, SalesChannel } from '../types'
import { cn } from '../lib/utils'
import { useCurrency } from '../hooks/useCurrency'

const SALES_CHANNELS: { value: SalesChannel; label: string }[] = [
  { value: 'direct', label: 'Direct' },
  { value: 'etsy', label: 'Etsy' },
  { value: 'marketplace', label: 'Marketplace' },
  { value: 'website', label: 'Website' },
  { value: 'other', label: 'Other' },
]

function CompleteTaskModal({
  task,
  onClose,
  onCompleted,
}: {
  task: Task
  onClose: () => void
  onCompleted: () => void
}) {
  const { t } = useTranslation('production')

  const { symbol } = useCurrency()
  const queryClient = useQueryClient()
  const [recordSale, setRecordSale] = useState(!!task.project?.price_cents)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const grossCents = (task.project?.price_cents ?? 0) * task.quantity

  const [saleForm, setSaleForm] = useState({
    item_description: task.project?.name ?? task.name,
    quantity: task.quantity,
    gross_cents: grossCents,
    channel: 'direct' as SalesChannel,
    fees_cents: 0,
    customer_name: '',
  })

  const handleSubmit = async () => {
    setSubmitting(true)
    setError('')
    try {
      await tasksApi.complete(task.id)

      if (recordSale) {
        await salesApi.create({
          item_description: saleForm.item_description,
          quantity: saleForm.quantity,
          gross_cents: saleForm.gross_cents,
          channel: saleForm.channel,
          fees_cents: saleForm.fees_cents,
          customer_name: saleForm.customer_name || undefined,
          project_id: task.project_id,
          occurred_at: new Date().toISOString(),
        })
        queryClient.invalidateQueries({ queryKey: ['sales'] })
        queryClient.invalidateQueries({ queryKey: ['sales', 'weekly-insights'] })
      }

      onCompleted()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to complete task"))
    } finally {
      setSubmitting(false)
    }
  }

  const formatDollars = (cents: number) => (cents / 100).toFixed(2)
  const parseDollars = (val: string) => Math.round(parseFloat(val || '0') * 100)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-surface-800 border border-surface-700 rounded-xl w-full max-w-md mx-4 shadow-2xl">
        <div className="p-6">
          <h2 className="text-lg font-semibold text-surface-100 mb-1">{t("Complete Task")}</h2>
          <p className="text-sm text-surface-400 mb-5">
            {task.name} ×{task.quantity}
          </p>

          {/* Record sale toggle */}
          <label className="flex items-center gap-3 p-3 bg-surface-900 rounded-lg cursor-pointer mb-4">
            <input
              type="checkbox"
              checked={recordSale}
              onChange={e => setRecordSale(e.target.checked)}
              className="w-4 h-4 rounded border-surface-600 text-accent-500 focus:ring-accent-500 bg-surface-700"
            />
            <DollarSign className="w-4 h-4 text-emerald-400" />
            <span className="text-sm font-medium text-surface-100">{t("Record a sale")}</span>
          </label>

          {/* Sale form */}
          {recordSale && (
            <div className="space-y-3 mb-4">
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Item Description")}</label>
                <input
                  type="text"
                  value={saleForm.item_description}
                  onChange={e => setSaleForm(f => ({ ...f, item_description: e.target.value }))}
                  className="input"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Quantity")}</label>
                  <input
                    type="number"
                    min={1}
                    value={saleForm.quantity}
                    onChange={e => setSaleForm(f => ({ ...f, quantity: parseInt(e.target.value) || 1 }))}
                    className="input"
                  />
                </div>
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Gross (")}{symbol})</label>
                  <input
                    type="number"
                    step="0.01"
                    min={0}
                    value={formatDollars(saleForm.gross_cents)}
                    onChange={e => setSaleForm(f => ({ ...f, gross_cents: parseDollars(e.target.value) }))}
                    className="input"
                  />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Channel")}</label>
                  <select
                    value={saleForm.channel}
                    onChange={e => setSaleForm(f => ({ ...f, channel: e.target.value as SalesChannel }))}
                    className="input"
                  >
                    {SALES_CHANNELS.map(ch => (
                      <option key={ch.value} value={ch.value}>{t(ch.label)}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-surface-500 mb-1">{t("Fees (")}{symbol})</label>
                  <input
                    type="number"
                    step="0.01"
                    min={0}
                    value={formatDollars(saleForm.fees_cents)}
                    onChange={e => setSaleForm(f => ({ ...f, fees_cents: parseDollars(e.target.value) }))}
                    className="input"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Customer Name")}</label>
                <input
                  type="text"
                  value={saleForm.customer_name}
                  onChange={e => setSaleForm(f => ({ ...f, customer_name: e.target.value }))}
                  placeholder={t("Optional")}
                  className="w-full px-3 py-2 bg-surface-900 border border-surface-700 rounded-lg text-sm text-surface-100 placeholder:text-surface-600 focus:outline-none focus:border-accent-500"
                />
              </div>
            </div>
          )}

          {error && (
            <p className="text-sm text-red-400 mb-4">{error}</p>
          )}

          <div className="flex gap-3 justify-end">
            <button
              onClick={onClose}
              disabled={submitting}
              className="px-4 py-2 text-sm text-surface-300 border border-surface-600 rounded-lg hover:bg-surface-700 transition-colors disabled:opacity-50"
            >
              {t("Cancel")}</button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="inline-flex items-center gap-2 px-4 py-2 text-sm bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 transition-colors disabled:opacity-50"
            >
              <CheckCircle className="w-4 h-4" />
              {submitting ? t("Completing...") : t("Complete")}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function EditTaskModal({
  task,
  onClose,
  onSaved,
}: {
  task: Task
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useTranslation('production')

  const [form, setForm] = useState({
    name: task.name,
    quantity: String(task.quantity),
    notes: task.notes ?? '',
    // The API expects YYYY-MM-DD; the task carries an RFC3339 timestamp
    pickup_date: task.pickup_date ? task.pickup_date.slice(0, 10) : '',
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      setError(t("Name is required"))
      return
    }
    const quantity = Number(form.quantity)
    if (!Number.isInteger(quantity) || quantity < 1) {
      setError(t("Quantity must be a whole number of at least 1"))
      return
    }
    setSubmitting(true)
    setError('')
    try {
      await tasksApi.update(task.id, {
        name: form.name.trim(),
        quantity,
        notes: form.notes,
        // Clearing the date means sending "", not null: the handler decodes
        // pickup_date into a *string, so a JSON null leaves the stored value
        // untouched while "" resets it to no date.
        pickup_date: form.pickup_date,
      })
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to update task"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-surface-800 border border-surface-700 rounded-xl w-full max-w-md mx-4 shadow-2xl">
        <div className="p-6">
          <h2 className="text-lg font-semibold text-surface-100 mb-1">{t("Edit Task")}</h2>
          <p className="text-sm text-surface-400 mb-5">
            {task.name} ×{task.quantity}
          </p>

          <div className="space-y-3">
            <div>
              <label className="block text-xs text-surface-500 mb-1">{t("Name")}</label>
              <input
                type="text"
                value={form.name}
                onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="input"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Quantity")}</label>
                <input
                  type="number"
                  min={1}
                  value={form.quantity}
                  onChange={e => setForm(f => ({ ...f, quantity: e.target.value }))}
                  className="input"
                />
              </div>
              <div>
                <label className="block text-xs text-surface-500 mb-1">{t("Pickup Date")}</label>
                <input
                  type="date"
                  value={form.pickup_date}
                  onChange={e => setForm(f => ({ ...f, pickup_date: e.target.value }))}
                  className="input"
                />
              </div>
            </div>
            <div>
              <label className="block text-xs text-surface-500 mb-1">{t("Notes")}</label>
              <textarea
                value={form.notes}
                onChange={e => setForm(f => ({ ...f, notes: e.target.value }))}
                rows={3}
                placeholder={t("Optional")}
                className="input resize-none"
              />
            </div>
          </div>

          {error && (
            <p className="text-sm text-red-400 mb-4">{error}</p>
          )}

          <div className="flex gap-3 justify-end">
            <button
              onClick={onClose}
              disabled={submitting}
              className="px-4 py-2 text-sm text-surface-300 border border-surface-600 rounded-lg hover:bg-surface-700 transition-colors disabled:opacity-50"
            >
              {t("Cancel")}</button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="inline-flex items-center gap-2 px-4 py-2 text-sm bg-accent-600 text-white rounded-lg hover:bg-accent-500 transition-colors disabled:opacity-50"
            >
              <CheckCircle className="w-4 h-4" />
              {submitting ? t("Saving...") : t("Save")}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

const statusConfig: Record<TaskStatus, { label: string; color: string; bgColor: string; icon: React.ReactNode }> = {
  pending: { label: 'Pending', color: 'text-surface-300', bgColor: 'bg-surface-700', icon: <Clock className="w-5 h-5" /> },
  in_progress: { label: 'In Progress', color: 'text-blue-400', bgColor: 'bg-blue-500/20', icon: <Play className="w-5 h-5" /> },
  completed: { label: 'Completed', color: 'text-emerald-400', bgColor: 'bg-emerald-500/20', icon: <CheckCircle className="w-5 h-5" /> },
  cancelled: { label: 'Cancelled', color: 'text-red-400', bgColor: 'bg-red-500/20', icon: <XCircle className="w-5 h-5" /> },
}

const jobStatusColors: Record<string, string> = {
  queued: 'bg-surface-700 text-surface-300',
  assigned: 'bg-amber-500/20 text-amber-400',
  uploaded: 'bg-blue-500/20 text-blue-400',
  sending: 'bg-blue-500/20 text-blue-400',
  printing: 'bg-indigo-500/20 text-indigo-400',
  paused: 'bg-amber-500/20 text-amber-400',
  completed: 'bg-emerald-500/20 text-emerald-400',
  failed: 'bg-red-500/20 text-red-400',
  cancelled: 'bg-surface-700 text-surface-400',
}

export function TaskDetail() {
  const { t, i18n } = useTranslation('production')
  const dateLocale = i18n.resolvedLanguage === 'pt-BR' ? 'pt-BR' : 'en-US'

  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [task, setTask] = useState<Task | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionLoading, setActionLoading] = useState(false)
  const [showCompleteModal, setShowCompleteModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)

  const loadTask = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const data = await tasksApi.get(id)
      setTask(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to load task"))
    } finally {
      setLoading(false)
    }
  }, [id, t])

  useEffect(() => {
    if (id) {
      loadTask()
    }
  }, [id, loadTask])

  const handleStart = async () => {
    if (!id) return
    setActionLoading(true)
    try {
      await tasksApi.start(id)
      loadTask()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to start task"))
    } finally {
      setActionLoading(false)
    }
  }

  const handleCancel = async () => {
    if (!id || !confirm(t("Are you sure you want to cancel this task?"))) return
    setActionLoading(true)
    try {
      await tasksApi.cancel(id)
      loadTask()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to cancel task"))
    } finally {
      setActionLoading(false)
    }
  }

  const handleToggleChecklist = async (item: TaskChecklistItem) => {
    if (!id) return
    try {
      await tasksApi.toggleChecklistItem(id, item.id, !item.completed)
      loadTask()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to update checklist"))
    }
  }

  const handlePrintFromChecklist = async (itemId: string) => {
    if (!id) return
    setActionLoading(true)
    try {
      await tasksApi.printFromChecklist(id, itemId)
      loadTask()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to create print job"))
    } finally {
      setActionLoading(false)
    }
  }

  const handleRegenerateChecklist = async () => {
    if (!id) return
    setActionLoading(true)
    try {
      await tasksApi.regenerateChecklist(id)
      loadTask()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to generate checklist"))
    } finally {
      setActionLoading(false)
    }
  }

  // Deleting is blocked by the database while the task still has print jobs:
  // print_jobs.task_id references tasks(id) with no ON DELETE rule and the
  // connection runs with foreign_keys=ON, so the DELETE fails with a raw
  // "FOREIGN KEY constraint failed" (handler maps it to 500). The checklist
  // does cascade, so only jobs need to be removed first.
  const handleDelete = async () => {
    if (!id) return
    if (!confirm(t("Delete this task? Its checklist goes with it. This cannot be undone."))) return
    try {
      await tasksApi.delete(id)
      navigate('/tasks')
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to delete task"))
    }
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString(dateLocale)
  }

  const getProgressColor = (progress: number) => {
    if (progress >= 100) return 'bg-emerald-500'
    if (progress >= 50) return 'bg-blue-500'
    if (progress > 0) return 'bg-amber-500'
    return 'bg-surface-600'
  }

  if (loading) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="text-surface-500">{t("Loading task...")}</div>
      </div>
    )
  }

  if (error || !task) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="text-center py-16">
          <AlertCircle className="w-16 h-16 mx-auto text-red-400 mb-4" />
          <h3 className="text-xl font-semibold text-surface-100 mb-2">{t("Error")}</h3>
          <p className="text-surface-500">{error || t("Task not found")}</p>
          <Link to="/tasks" className="mt-4 inline-block text-accent-400 hover:text-accent-300">
            {t("Back to Tasks")}</Link>
        </div>
      </div>
    )
  }

  const config = statusConfig[task.status]
  const progress = task.progress ?? 0
  const jobs = task.jobs ?? []
  const checklist = task.checklist_items ?? []
  const completedJobs = jobs.filter(j => j.status === 'completed').length
  const failedJobs = jobs.filter(j => j.status === 'failed').length
  const completedChecklist = checklist.filter(i => i.completed).length

  return (
    <div className="p-4 sm:p-6 lg:p-8">
      {/* Header */}
      <div className="mb-6">
        <Link to="/tasks" className="inline-flex items-center gap-2 text-surface-400 hover:text-surface-100 mb-4 transition-colors">
          <ArrowLeft className="w-4 h-4" />
          {t("Back to Tasks")}</Link>
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl font-bold text-surface-100">{task.name}</h1>
            <p className="text-surface-400">
              {t("Quantity:")}{' '}{task.quantity} {t("| Created:")}{' '}{formatDate(task.created_at)}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <div className={cn('inline-flex items-center gap-2 px-3 py-1.5 rounded-full', config.bgColor, config.color)}>
              {config.icon}
              <span className="font-medium">{t(config.label)}</span>
            </div>
            <button
              onClick={() => setShowEditModal(true)}
              className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-surface-300 border border-surface-600 rounded-lg hover:bg-surface-700 transition-colors"
            >
              <Pencil className="w-3.5 h-3.5" />
              {t("Edit")}</button>
            <button
              onClick={handleDelete}
              disabled={jobs.length > 0}
              title={jobs.length > 0
                ? t("This task still has {{count}} print job. The database blocks deleting a task with jobs — remove the jobs first.", { count: jobs.length })
                : t("Delete this task")}
              className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-red-400 border border-red-500/50 rounded-lg hover:bg-red-500/10 transition-colors disabled:opacity-50"
            >
              <Trash2 className="w-3.5 h-3.5" />
              {t("Delete")}</button>
          </div>
        </div>
      </div>

      {/* Progress */}
      <div className="card p-6 mb-6">
        <h2 className="text-lg font-semibold text-surface-100 mb-4">{t("Progress")}</h2>
        <div className="mb-4">
          <div className="flex justify-between text-sm mb-1">
            <span className="text-surface-400">{t("Overall Progress")}</span>
            <span className="font-medium text-surface-100">{Math.round(progress)}%</span>
          </div>
          <div className="h-3 bg-surface-700 rounded-full overflow-hidden">
            <div
              className={cn('h-full transition-all', getProgressColor(progress))}
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>
        <div className="grid grid-cols-3 gap-4 text-center">
          <div>
            <div className="text-2xl font-bold text-surface-100">{jobs.length}</div>
            <div className="text-sm text-surface-500">{t("Total Jobs")}</div>
          </div>
          <div>
            <div className="text-2xl font-bold text-emerald-400">{completedJobs}</div>
            <div className="text-sm text-surface-500">{t("Completed")}</div>
          </div>
          <div>
            <div className="text-2xl font-bold text-red-400">{failedJobs}</div>
            <div className="text-sm text-surface-500">{t("Failed")}</div>
          </div>
        </div>
      </div>

      {/* Checklist */}
      <div className="card p-6 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-surface-100">{t("Checklist")}</h2>
          {checklist.length > 0 ? (
            <span className="text-sm text-surface-400">
              {t("{{completed}} of {{count}} checklist item completed", { completed: completedChecklist, count: checklist.length })}</span>
          ) : (
            <button
              onClick={handleRegenerateChecklist}
              disabled={actionLoading}
              className="text-sm text-accent-400 hover:text-accent-300 transition-colors disabled:opacity-50"
            >
              {t("Generate from parts")}</button>
          )}
        </div>
        {checklist.length > 0 ? (
          <>
            <div className="mb-4">
              <div className="h-2 bg-surface-700 rounded-full overflow-hidden">
                <div
                  className={cn(
                    'h-full transition-all',
                    completedChecklist === checklist.length ? 'bg-emerald-500' : 'bg-accent-500'
                  )}
                  style={{ width: `${(completedChecklist / checklist.length) * 100}%` }}
                />
              </div>
            </div>
            <div className="space-y-2">
              {checklist.map((item) => (
                <div
                  key={item.id}
                  className="flex items-center gap-3 p-3 bg-surface-800 rounded-lg hover:bg-surface-700 transition-colors"
                >
                  <button
                    onClick={() => handleToggleChecklist(item)}
                    className="flex items-center gap-3 flex-1 text-left"
                  >
                    {item.completed ? (
                      <CheckSquare className="w-5 h-5 text-emerald-400 flex-shrink-0" />
                    ) : (
                      <Square className="w-5 h-5 text-surface-500 flex-shrink-0" />
                    )}
                    <span className={cn(
                      'font-medium',
                      item.completed ? 'text-surface-500 line-through' : 'text-surface-100'
                    )}>
                      {item.name}
                    </span>
                    {item.completed_at && (
                      <span className="ml-auto text-xs text-surface-600">
                        {formatDate(item.completed_at)}
                      </span>
                    )}
                  </button>
                  {item.part_id && !item.completed && task.status !== 'completed' && task.status !== 'cancelled' && (
                    <button
                      onClick={() => handlePrintFromChecklist(item.id)}
                      disabled={actionLoading}
                      className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-accent-400 border border-accent-500/50 rounded-lg hover:bg-accent-500/10 transition-colors disabled:opacity-50 flex-shrink-0"
                    >
                      <Printer className="w-3.5 h-3.5" />
                      {t("Print")}</button>
                  )}
                </div>
              ))}
            </div>
          </>
        ) : (
          <p className="text-sm text-surface-500">
            {t("No checklist items. Click \"Generate from parts\" to create from project parts.")}</p>
        )}
      </div>

      {/* Actions */}
      {task.status !== 'completed' && task.status !== 'cancelled' && (
        <div className="card p-6 mb-6">
          <h2 className="text-lg font-semibold text-surface-100 mb-4">{t("Actions")}</h2>
          <div className="flex gap-3">
            {task.status === 'pending' && (
              <button
                onClick={handleStart}
                disabled={actionLoading}
                className="btn btn-primary"
              >
                <Play className="w-4 h-4 mr-2" />
                {t("Start Task")}</button>
            )}
            {task.status === 'in_progress' && (
              <button
                onClick={() => setShowCompleteModal(true)}
                disabled={actionLoading}
                className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 transition-colors disabled:opacity-50"
              >
                <CheckCircle className="w-4 h-4" />
                {t("Mark Complete")}</button>
            )}
            <button
              onClick={handleCancel}
              disabled={actionLoading}
              className="inline-flex items-center gap-2 px-4 py-2 text-red-400 border border-red-500/50 rounded-lg hover:bg-red-500/10 transition-colors disabled:opacity-50"
            >
              <XCircle className="w-4 h-4" />
              {t("Cancel")}</button>
          </div>
        </div>
      )}

      {/* Project Info */}
      {task.project && (
        <div className="card p-6 mb-6">
          <h2 className="text-lg font-semibold text-surface-100 mb-4">{t("Project")}</h2>
          <Link
            to={`/projects/${task.project.id}`}
            className="flex items-center gap-3 p-3 bg-surface-800 rounded-lg hover:bg-surface-700 transition-colors"
          >
            <Package className="w-8 h-8 text-accent-400" />
            <div>
              <div className="font-medium text-surface-100">{task.project.name}</div>
              {task.project.sku && (
                <div className="text-sm text-surface-500">{t("SKU:")}{' '}{task.project.sku}</div>
              )}
            </div>
          </Link>
        </div>
      )}

      {/* Jobs */}
      <div className="card p-6 mb-6">
        <h2 className="text-lg font-semibold text-surface-100 mb-4">{t("Print Jobs")}</h2>
        {jobs.length === 0 ? (
          <div className="text-center py-8">
            <Printer className="w-10 h-10 mx-auto mb-3 text-surface-600" />
            <p className="text-surface-500">{t("No print jobs assigned to this task yet")}</p>
          </div>
        ) : (
          <div className="space-y-3">
            {jobs.map((job: PrintJob) => (
              <Link
                key={job.id}
                to={`/print-jobs/${job.id}`}
                className="flex items-center justify-between p-4 bg-surface-800 rounded-lg hover:bg-surface-700 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <Printer className="w-5 h-5 text-surface-500" />
                  <div>
                    <div className="font-medium text-surface-100">{t('Job {{id}}', { id: job.id.substring(0, 8) })}</div>
                    <div className="text-sm text-surface-500">
                      {t('Progress: {{progress}}%', { progress: Math.round(job.progress) })}
                      {job.started_at && t(' · {{label}}', { label: `${t("Started")}: ${formatDate(job.started_at)}` })}
                    </div>
                  </div>
                </div>
                <span className={cn('px-2.5 py-1 rounded-full text-xs font-medium', jobStatusColors[job.status] || 'bg-surface-700 text-surface-400')}>
                  {t(({ pending: 'Pending', in_progress: 'In Progress', completed: 'Completed', cancelled: 'Cancelled', printing: 'Printing', paused: 'Paused', failed: 'Failed', queued: 'Queued', idle: 'Idle', offline: 'Offline', online: 'Online', error: 'Error', ready: 'Ready', sent: 'Sent', info: 'Info', success: 'Success', warning: 'Warning', critical: 'Critical', channels: 'Channels', templates: 'Templates', deliveries: 'Deliveries' } as Record<string, string>)[job.status] || job.status)}
                </span>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Details */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-surface-100 mb-4">{t("Details")}</h2>
        <dl className="grid grid-cols-2 gap-4">
          <div>
            <dt className="text-sm text-surface-500">{t("Task ID")}</dt>
            <dd className="font-mono text-sm text-surface-300">{task.id}</dd>
          </div>
          <div>
            <dt className="text-sm text-surface-500">{t("Status")}</dt>
            <dd className="text-surface-100">{t(config.label)}</dd>
          </div>
          <div>
            <dt className="text-sm text-surface-500">{t("Created")}</dt>
            <dd className="text-surface-300">{formatDate(task.created_at)}</dd>
          </div>
          <div>
            <dt className="text-sm text-surface-500">{t("Updated")}</dt>
            <dd className="text-surface-300">{formatDate(task.updated_at)}</dd>
          </div>
          {task.pickup_date && (
            <div>
              <dt className="text-sm text-surface-500">{t("Pickup / Ship Date")}</dt>
              <dd className="flex items-center gap-2 text-surface-300">
                <CalendarDays className="w-4 h-4 text-accent-400" />
                {new Date(task.pickup_date).toLocaleDateString(dateLocale)}
              </dd>
            </div>
          )}
          {task.started_at && (
            <div>
              <dt className="text-sm text-surface-500">{t("Started")}</dt>
              <dd className="text-surface-300">{formatDate(task.started_at)}</dd>
            </div>
          )}
          {task.completed_at && (
            <div>
              <dt className="text-sm text-surface-500">{t("Completed")}</dt>
              <dd className="text-surface-300">{formatDate(task.completed_at)}</dd>
            </div>
          )}
          {task.order_id && (
            <div className="col-span-2">
              <dt className="text-sm text-surface-500">{t("Order")}</dt>
              <dd>
                <Link to={`/orders/${task.order_id}`} className="text-accent-400 hover:text-accent-300">
                  {t("View Order")}</Link>
              </dd>
            </div>
          )}
          {task.notes && (
            <div className="col-span-2">
              <dt className="text-sm text-surface-500">{t("Notes")}</dt>
              <dd className="whitespace-pre-wrap text-surface-300">{task.notes}</dd>
            </div>
          )}
        </dl>
      </div>

      {showCompleteModal && (
        <CompleteTaskModal
          task={task}
          onClose={() => setShowCompleteModal(false)}
          onCompleted={() => {
            setShowCompleteModal(false)
            loadTask()
          }}
        />
      )}

      {showEditModal && (
        <EditTaskModal
          task={task}
          onClose={() => setShowEditModal(false)}
          onSaved={() => {
            setShowEditModal(false)
            loadTask()
          }}
        />
      )}
    </div>
  )
}
