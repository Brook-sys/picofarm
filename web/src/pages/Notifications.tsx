import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, Clock, Copy, Eye, FileText, Hash, MessageCircle, Plus, RotateCcw, Send, Sparkles, Trash2, Webhook, X } from 'lucide-react'
import { notificationsApi } from '../api/client'
import { cn, formatRelativeTime } from '../lib/utils'
import type { NotificationChannel, NotificationDelivery, NotificationPreview, NotificationTemplate } from '../types'

const EVENTS = [
  { value: 'print.started', label: 'Print Started', description: 'When a job begins printing', tone: 'text-blue-300', bg: 'bg-blue-500/10', border: 'border-blue-500/30' },
  { value: 'print.completed', label: 'Print Completed', description: 'When a job finishes successfully', tone: 'text-emerald-300', bg: 'bg-emerald-500/10', border: 'border-emerald-500/30' },
  { value: 'print.failed', label: 'Print Failed', description: 'When a job fails or is cancelled unexpectedly', tone: 'text-red-300', bg: 'bg-red-500/10', border: 'border-red-500/30' },
  { value: 'print.cancelled', label: 'Print Cancelled', description: 'When a print is cancelled', tone: 'text-orange-300', bg: 'bg-orange-500/10', border: 'border-orange-500/30' },
  { value: 'printer.offline', label: 'Printer Offline', description: 'When a printer disconnects', tone: 'text-red-300', bg: 'bg-red-500/10', border: 'border-red-500/30' },
  { value: 'printer.online', label: 'Printer Online', description: 'When a printer reconnects', tone: 'text-emerald-300', bg: 'bg-emerald-500/10', border: 'border-emerald-500/30' },
  { value: 'printer.error', label: 'Printer Error', description: 'When a printer reports an error', tone: 'text-red-300', bg: 'bg-red-500/10', border: 'border-red-500/30' },
  { value: 'emergency.stop', label: 'Emergency Stop', description: 'Critical safety event', tone: 'text-red-300', bg: 'bg-red-500/10', border: 'border-red-500/30' },
  { value: 'queue.blocked', label: 'Queue Blocked', description: 'When a queued item needs attention', tone: 'text-amber-300', bg: 'bg-amber-500/10', border: 'border-amber-500/30' },
  { value: 'spool.low', label: 'Spool Low', description: 'When material is running low', tone: 'text-amber-300', bg: 'bg-amber-500/10', border: 'border-amber-500/30' },
] as const

const SEVERITIES = ['info', 'success', 'warning', 'error', 'critical'] as const
const VARIABLES = ['{{title}}', '{{message}}', '{{printer_name}}', '{{file_name}}', '{{status}}', '{{progress}}', '{{duration}}', '{{filament_grams}}', '{{wasted_grams}}', '{{notes}}', '{{timestamp}}', '{{event}}', '{{severity}}', '{{printer_model}}']
const DEFAULT_FORM: Partial<NotificationChannel> = { name: '', type: 'discord', enabled: true, config: {}, events: ['print.completed', 'print.failed'], printer_ids: [], min_severity: 'info' }

type Tab = 'channels' | 'templates' | 'deliveries'
type TemplateStyle = 'clean' | 'detailed' | 'compact'

const templateStyles: Array<{ id: TemplateStyle; label: string; description: string }> = [
  { id: 'clean', label: 'Clean', description: 'Short, polished message for daily use.' },
  { id: 'detailed', label: 'Detailed', description: 'Includes printer, file, timing and material info.' },
  { id: 'compact', label: 'Compact', description: 'Minimal status update for noisy channels.' },
]

function templatePreset(channelId: string, eventType: string, format: NotificationTemplate['format'], style: TemplateStyle = 'clean'): NotificationTemplate {
  const content = templateContent(eventType, style)
  if (format === 'telegram_html') {
    return { channel_id: channelId, event_type: eventType, format, title_template: `<b>${content.title}</b>`, body_template: toTelegramBody(content.lines), payload_template: '', enabled: true }
  }
  if (format === 'json') {
    return { channel_id: channelId, event_type: eventType, format, title_template: content.title, body_template: content.summary, payload_template: jsonPayloadTemplate(eventType), enabled: true }
  }
  return { channel_id: channelId, event_type: eventType, format, title_template: content.title, body_template: content.lines.join('\n'), payload_template: '', enabled: true }
}

type TemplateContent = { title: string; summary: string; lines: string[] }

function templateContent(eventType: string, style: TemplateStyle): TemplateContent {
  const compact = style === 'compact'
  const detailed = style === 'detailed'
  switch (eventType) {
    case 'print.started':
      return buildContent('Print started: {{file_name}}', 'Started {{file_name}} on {{printer_name}}.', compact ? ['{{printer_name}} started {{file_name}}.'] : ['Print started successfully.', '', 'Printer: {{printer_name}}', 'File: {{file_name}}', ...(detailed ? ['Estimated time: {{duration}}', 'Filament: {{filament_grams}}g', 'Started at: {{timestamp}}'] : ['Estimated time: {{duration}}'])])
    case 'print.completed':
      return buildContent('Print finished: {{file_name}}', '{{file_name}} finished successfully on {{printer_name}}.', compact ? ['{{file_name}} finished on {{printer_name}}.'] : ['Print completed successfully.', '', 'Printer: {{printer_name}}', 'File: {{file_name}}', 'Duration: {{duration}}', ...(detailed ? ['Filament used: {{filament_grams}}g', 'Completed at: {{timestamp}}'] : [])])
    case 'print.failed':
      return buildContent('Print failed: {{file_name}}', '{{file_name}} failed on {{printer_name}}.', compact ? ['{{file_name}} failed on {{printer_name}}.'] : ['Print failed and needs attention.', '', 'Printer: {{printer_name}}', 'File: {{file_name}}', 'Last progress: {{progress}}%', ...(detailed ? ['Estimated waste: {{wasted_grams}}g', 'Notes: {{notes}}', 'Failed at: {{timestamp}}'] : ['Notes: {{notes}}'])])
    case 'print.cancelled':
      return buildContent('Print cancelled: {{file_name}}', '{{file_name}} was cancelled on {{printer_name}}.', compact ? ['{{file_name}} was cancelled.'] : ['Print was cancelled before completion.', '', 'Printer: {{printer_name}}', 'File: {{file_name}}', 'Stopped at: {{progress}}%', ...(detailed ? ['Possible waste: {{wasted_grams}}g', 'Notes: {{notes}}'] : [])])
    case 'printer.offline':
      return buildContent('Printer offline: {{printer_name}}', '{{printer_name}} is offline.', compact ? ['{{printer_name}} is offline.'] : ['Printer connection was lost.', '', 'Printer: {{printer_name}}', 'Model: {{printer_model}}', ...(detailed ? ['Status: {{status}}', 'Detected at: {{timestamp}}', 'Notes: {{notes}}'] : ['Status: {{status}}'])])
    case 'printer.online':
      return buildContent('Printer online: {{printer_name}}', '{{printer_name}} is back online.', compact ? ['{{printer_name}} is online.'] : ['Printer is connected and ready.', '', 'Printer: {{printer_name}}', 'Model: {{printer_model}}', ...(detailed ? ['Status: {{status}}', 'Detected at: {{timestamp}}'] : [])])
    case 'printer.error':
      return buildContent('Printer error: {{printer_name}}', '{{printer_name}} reported an error.', compact ? ['{{printer_name}} reported an error.'] : ['Printer reported an error state.', '', 'Printer: {{printer_name}}', 'Status: {{status}}', ...(detailed ? ['Message: {{message}}', 'Notes: {{notes}}', 'Timestamp: {{timestamp}}'] : ['Message: {{message}}'])])
    case 'emergency.stop':
      return buildContent('Emergency stop: {{printer_name}}', 'Emergency stop triggered on {{printer_name}}.', compact ? ['Emergency stop triggered on {{printer_name}}.'] : ['Emergency stop was triggered.', '', 'Printer: {{printer_name}}', 'Status: {{status}}', 'Message: {{message}}', ...(detailed ? ['Timestamp: {{timestamp}}', 'Notes: {{notes}}'] : [])])
    case 'queue.blocked':
      return buildContent('Queue blocked: {{file_name}}', '{{file_name}} cannot be dispatched.', compact ? ['Queue blocked: {{file_name}}.'] : ['A queued item needs attention before printing.', '', 'File: {{file_name}}', 'Printer: {{printer_name}}', 'Reason: {{message}}', ...(detailed ? ['Notes: {{notes}}', 'Checked at: {{timestamp}}'] : [])])
    case 'spool.low':
      return buildContent('Spool low: {{printer_name}}', 'Material is running low for {{printer_name}}.', compact ? ['Spool is low on {{printer_name}}.'] : ['Material is running low.', '', 'Printer: {{printer_name}}', 'Remaining estimate: {{filament_grams}}g', 'Status: {{status}}', ...(detailed ? ['Notes: {{notes}}', 'Detected at: {{timestamp}}'] : [])])
    default:
      return buildContent('{{title}}', '{{message}}', compact ? ['{{message}}'] : ['{{message}}', '', 'Printer: {{printer_name}}', 'Status: {{status}}'])
  }
}

function buildContent(title: string, summary: string, lines: string[]): TemplateContent {
  return { title, summary, lines }
}

function toTelegramBody(lines: string[]) {
  return lines.map(line => {
    const match = line.match(/^([^:]+):\s(.+)$/)
    return match ? `<b>${match[1]}:</b> ${match[2]}` : line
  }).join('\n')
}

function jsonPayloadTemplate(eventType: string) {
  const base = ['  "event": "{{event}}"', '  "severity": "{{severity}}"', '  "title": "{{title}}"', '  "message": "{{message}}"', '  "timestamp": "{{timestamp}}"']
  const fieldsByEvent: Record<string, string[]> = {
    'print.started': ['  "printer": "{{printer_name}}"', '  "file": "{{file_name}}"', '  "estimated_duration": "{{duration}}"', '  "filament_grams": "{{filament_grams}}"'],
    'print.completed': ['  "printer": "{{printer_name}}"', '  "file": "{{file_name}}"', '  "duration": "{{duration}}"', '  "filament_grams": "{{filament_grams}}"'],
    'print.failed': ['  "printer": "{{printer_name}}"', '  "file": "{{file_name}}"', '  "progress": "{{progress}}"', '  "wasted_grams": "{{wasted_grams}}"', '  "notes": "{{notes}}"'],
    'print.cancelled': ['  "printer": "{{printer_name}}"', '  "file": "{{file_name}}"', '  "progress": "{{progress}}"', '  "wasted_grams": "{{wasted_grams}}"'],
    'printer.offline': ['  "printer": "{{printer_name}}"', '  "printer_model": "{{printer_model}}"', '  "status": "{{status}}"'],
    'printer.online': ['  "printer": "{{printer_name}}"', '  "printer_model": "{{printer_model}}"', '  "status": "{{status}}"'],
    'printer.error': ['  "printer": "{{printer_name}}"', '  "status": "{{status}}"', '  "notes": "{{notes}}"'],
    'emergency.stop': ['  "printer": "{{printer_name}}"', '  "status": "{{status}}"', '  "notes": "{{notes}}"'],
    'queue.blocked': ['  "file": "{{file_name}}"', '  "printer": "{{printer_name}}"', '  "notes": "{{notes}}"'],
    'spool.low': ['  "printer": "{{printer_name}}"', '  "remaining_grams": "{{filament_grams}}"', '  "status": "{{status}}"'],
  }
  return `{
${[...base, ...(fieldsByEvent[eventType] || [])].join(',\n')}
}`
}

function eventMeta(eventType: string) {
  return EVENTS.find(event => event.value === eventType) || EVENTS[0]
}

export default function Notifications() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>('channels')
  const [editing, setEditing] = useState<Partial<NotificationChannel> | null>(null)
  const [error, setError] = useState('')
  const { data: channels = [], isLoading } = useQuery({ queryKey: ['notifications'], queryFn: notificationsApi.list })
  const { data: deliveries = [] } = useQuery({ queryKey: ['notification-deliveries'], queryFn: () => notificationsApi.deliveries(), refetchInterval: 10000 })
  const { data: templates = [] } = useQuery({ queryKey: ['notification-templates'], queryFn: () => notificationsApi.templates() })
  const enabledChannels = channels.filter(channel => channel.enabled).length
  const failedDeliveries = deliveries.filter(delivery => delivery.status !== 'sent').length

  const saveMutation = useMutation({ mutationFn: (channel: Partial<NotificationChannel>) => channel.id ? notificationsApi.update(channel.id, channel) : notificationsApi.create(channel), onSuccess: () => { setEditing(null); setError(''); queryClient.invalidateQueries({ queryKey: ['notifications'] }) }, onError: err => setError(err instanceof Error ? err.message : 'Failed to save notification channel') })
  const deleteMutation = useMutation({ mutationFn: notificationsApi.delete, onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }) })
  const testMutation = useMutation({ mutationFn: notificationsApi.test, onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notification-deliveries'] }), onError: err => setError(err instanceof Error ? err.message : 'Failed to send test notification') })

  return (
    <div className="p-4 sm:p-6 lg:p-8 max-w-7xl mx-auto">
      <div className="mb-6 overflow-hidden rounded-2xl border border-surface-800 bg-gradient-to-br from-surface-900 via-surface-900 to-accent-950/30 p-6 shadow-xl shadow-black/20">
        <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-accent-500/30 bg-accent-500/10 px-3 py-1 text-xs font-semibold text-accent-200">
              <Sparkles className="h-3.5 w-3.5" /> Smart alerts
            </div>
            <h1 className="text-3xl font-display font-bold text-surface-100 flex items-center gap-3"><Bell className="h-8 w-8 text-accent-400" />Notifications</h1>
            <p className="mt-2 max-w-2xl text-sm text-surface-400">Configure beautiful Telegram, Discord and webhook alerts for prints, printers, queue issues and material warnings.</p>
          </div>
          <div className="grid grid-cols-3 gap-3 text-center sm:min-w-[420px]">
            <HeroMetric label="Channels" value={channels.length} />
            <HeroMetric label="Enabled" value={enabledChannels} tone="text-emerald-300" />
            <HeroMetric label="Failures" value={failedDeliveries} tone={failedDeliveries ? 'text-red-300' : 'text-surface-100'} />
          </div>
        </div>
      </div>

      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="inline-flex rounded-xl border border-surface-800 bg-surface-900 p-1">
          {(['channels', 'templates', 'deliveries'] as Tab[]).map(item => (
            <button key={item} onClick={() => setTab(item)} className={cn('rounded-lg px-4 py-2 text-sm font-semibold capitalize transition-colors', tab === item ? 'bg-accent-500 text-white shadow-lg shadow-accent-500/20' : 'text-surface-400 hover:bg-surface-800 hover:text-surface-100')}>{item}</button>
          ))}
        </div>
        {tab === 'channels' && <button onClick={() => setEditing(DEFAULT_FORM)} className="btn btn-primary"><Plus className="h-4 w-4 mr-2" />Add Channel</button>}
      </div>

      {error && <div className="mb-4 rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      {tab === 'channels' && <ChannelsTab channels={channels} isLoading={isLoading} onEdit={setEditing} onDelete={id => deleteMutation.mutate(id)} onTest={id => testMutation.mutate(id)} testing={testMutation.isPending} />}
      {tab === 'templates' && <TemplatesTab channels={channels} templates={templates} onError={setError} />}
      {tab === 'deliveries' && <DeliveriesTab deliveries={deliveries} channels={channels} />}
      {editing && <ChannelModal channel={editing} onClose={() => setEditing(null)} onSave={channel => saveMutation.mutate(channel)} saving={saveMutation.isPending} />}
    </div>
  )
}

function HeroMetric({ label, value, tone = 'text-surface-100' }: { label: string; value: number; tone?: string }) {
  return <div className="rounded-xl border border-surface-800 bg-surface-950/50 p-3"><div className={cn('text-2xl font-bold', tone)}>{value}</div><div className="text-xs text-surface-500">{label}</div></div>
}

function ChannelsTab({ channels, isLoading, onEdit, onDelete, onTest, testing }: { channels: NotificationChannel[]; isLoading: boolean; onEdit: (channel: Partial<NotificationChannel>) => void; onDelete: (id: string) => void; onTest: (id: string) => void; testing: boolean }) {
  if (isLoading) return <div className="card p-8 text-center text-surface-500">Loading channels...</div>
  if (channels.length === 0) return <EmptyState icon={Bell} title="No notification channels yet" description="Add Discord, Telegram or a webhook to start receiving alerts." />
  return <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">{channels.map(channel => <ChannelCard key={channel.id} channel={channel} onEdit={() => onEdit(channel)} onDelete={() => onDelete(channel.id)} onTest={() => onTest(channel.id)} testing={testing} />)}</div>
}

function TemplatesTab({ channels, templates, onError }: { channels: NotificationChannel[]; templates: NotificationTemplate[]; onError: (error: string) => void }) {
  const queryClient = useQueryClient()
  const firstChannel = channels[0]?.id || ''
  const [channelId, setChannelId] = useState(firstChannel)
  const [eventType, setEventType] = useState('print.completed')
  const [style, setStyle] = useState<TemplateStyle>('clean')
  const existing = templates.find(template => template.channel_id === channelId && template.event_type === eventType)
  const [draft, setDraft] = useState<NotificationTemplate>(existing || templatePreset(channelId, eventType, 'discord_embed'))
  const [preview, setPreview] = useState<NotificationPreview | null>(null)
  const selectedChannel = channels.find(channel => channel.id === channelId)
  const selectedEvent = eventMeta(eventType)
  const saveMutation = useMutation({ mutationFn: notificationsApi.saveTemplate, onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['notification-templates'] }); onError('') }, onError: err => onError(err instanceof Error ? err.message : 'Failed to save template') })
  const previewMutation = useMutation({ mutationFn: notificationsApi.previewTemplate, onSuccess: setPreview, onError: err => onError(err instanceof Error ? err.message : 'Failed to preview template') })

  useEffect(() => {
    if (!channelId && firstChannel) setChannelId(firstChannel)
  }, [channelId, firstChannel])

  const reloadDraft = (nextChannelId: string, nextEventType: string) => {
    const found = templates.find(template => template.channel_id === nextChannelId && template.event_type === nextEventType)
    setDraft(found || templatePreset(nextChannelId, nextEventType, selectedChannel?.type === 'telegram' ? 'telegram_html' : selectedChannel?.type === 'webhook' ? 'json' : 'discord_embed', style))
    setPreview(null)
  }
  const applyPreset = (nextStyle = style) => {
    setStyle(nextStyle)
    setDraft(templatePreset(channelId, eventType, draft.format, nextStyle))
    setPreview(null)
  }
  const copyVariable = (variable: string) => navigator.clipboard?.writeText(variable)

  if (channels.length === 0) return <EmptyState icon={MessageCircle} title="Create a channel first" description="Templates are attached to a notification channel." />

  return (
    <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1fr)_440px] gap-4">
      <div className="space-y-4">
        <div className="card overflow-hidden">
          <div className="border-b border-surface-800 bg-surface-900/80 p-5">
            <div className="flex items-start gap-3">
              <div className={cn('rounded-xl border p-2', selectedEvent.bg, selectedEvent.border)}><FileText className={cn('h-5 w-5', selectedEvent.tone)} /></div>
              <div>
                <h2 className="font-semibold text-surface-100">Message Template</h2>
                <p className="text-sm text-surface-500">Pick an event, choose a style, preview it, then save.</p>
              </div>
            </div>
          </div>
          <div className="space-y-5 p-5">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <Field label="Channel"><select value={channelId} onChange={e => { setChannelId(e.target.value); reloadDraft(e.target.value, eventType) }} className="input">{channels.map(channel => <option key={channel.id} value={channel.id}>{channel.name}</option>)}</select></Field>
              <Field label="Event"><select value={eventType} onChange={e => { setEventType(e.target.value); reloadDraft(channelId, e.target.value) }} className="input">{EVENTS.map(event => <option key={event.value} value={event.value}>{event.label}</option>)}</select></Field>
              <Field label="Format"><select value={draft.format} onChange={e => setDraft(prev => ({ ...prev, format: e.target.value as NotificationTemplate['format'] }))} className="input"><option value="text">Plain text</option><option value="telegram_html">Telegram HTML</option><option value="discord_embed">Discord embed</option><option value="json">Webhook JSON</option></select></Field>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {templateStyles.map(item => (
                <button key={item.id} onClick={() => applyPreset(item.id)} className={cn('rounded-xl border p-3 text-left transition-colors', style === item.id ? 'border-accent-500/60 bg-accent-500/15' : 'border-surface-800 bg-surface-900/50 hover:border-surface-700')}>
                  <div className="text-sm font-semibold text-surface-100">{item.label}</div>
                  <div className="mt-1 text-xs text-surface-500">{item.description}</div>
                </button>
              ))}
            </div>

            <Field label="Title template"><input value={draft.title_template} onChange={e => setDraft(prev => ({ ...prev, title_template: e.target.value }))} className="input" /></Field>
            <Field label="Body template"><textarea value={draft.body_template} onChange={e => setDraft(prev => ({ ...prev, body_template: e.target.value }))} className="input min-h-44 font-mono text-sm leading-6" /></Field>
            {(selectedChannel?.type === 'webhook' || draft.format === 'json') && <Field label="Payload JSON template"><textarea value={draft.payload_template} onChange={e => setDraft(prev => ({ ...prev, payload_template: e.target.value }))} className="input min-h-36 font-mono text-sm" /></Field>}

            <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-3">
              <div className="mb-2 flex items-center justify-between"><div className="text-xs font-semibold uppercase tracking-wide text-surface-500">Variables</div><div className="text-xs text-surface-600">Click to copy</div></div>
              <div className="flex flex-wrap gap-1.5">{VARIABLES.map(variable => <button key={variable} onClick={() => copyVariable(variable)} className="inline-flex items-center gap-1 rounded-lg border border-surface-700 bg-surface-800 px-2 py-1 text-xs text-surface-300 hover:border-accent-500/50 hover:text-accent-200"><Copy className="h-3 w-3" />{variable}</button>)}</div>
            </div>

            <div className="flex flex-wrap gap-2">
              <button onClick={() => previewMutation.mutate({ ...draft, channel_id: channelId, event_type: eventType })} className="btn btn-secondary" disabled={previewMutation.isPending}><Eye className="h-4 w-4 mr-2" />Preview</button>
              <button onClick={() => saveMutation.mutate({ ...draft, channel_id: channelId, event_type: eventType })} disabled={saveMutation.isPending} className="btn btn-primary">Save template</button>
              <button onClick={() => applyPreset()} className="btn btn-secondary"><RotateCcw className="h-4 w-4 mr-2" />Reset to preset</button>
            </div>
          </div>
        </div>
      </div>
      <PreviewPanel preview={preview} eventType={eventType} format={draft.format} />
    </div>
  )
}

function PreviewPanel({ preview, eventType, format }: { preview: NotificationPreview | null; eventType: string; format: NotificationTemplate['format'] }) {
  const event = eventMeta(eventType)
  return (
    <div className="card sticky top-4 self-start overflow-hidden">
      <div className="border-b border-surface-800 bg-surface-900/80 p-5">
        <h2 className="font-semibold text-surface-100">Live Preview</h2>
        <p className="text-sm text-surface-500">Rendered with sample event data.</p>
      </div>
      <div className="p-5">
        {preview ? (
          <div className={cn('rounded-2xl border p-4', event.bg, event.border)}>
            <div className="mb-3 flex items-center gap-2"><span className={cn('h-2.5 w-2.5 rounded-full', event.value.includes('failed') || event.value.includes('offline') || event.value.includes('error') ? 'bg-red-400' : event.value.includes('completed') || event.value.includes('online') ? 'bg-emerald-400' : 'bg-blue-400')} /><span className="text-xs font-semibold uppercase tracking-wide text-surface-400">{formatLabel(format)}</span></div>
            <div className="rounded-xl border border-surface-700 bg-surface-950/70 p-4 shadow-inner">
              <div className="text-base font-semibold text-surface-100">{preview.title}</div>
              <pre className="mt-3 whitespace-pre-wrap text-sm leading-6 text-surface-300">{preview.body}</pre>
            </div>
            {preview.payload && <pre className="mt-3 max-h-64 overflow-auto rounded-xl border border-surface-800 bg-black/30 p-3 text-xs text-surface-300">{JSON.stringify(preview.payload, null, 2)}</pre>}
          </div>
        ) : (
          <div className="flex min-h-72 flex-col items-center justify-center rounded-2xl border border-dashed border-surface-800 bg-surface-900/40 p-6 text-center text-sm text-surface-500">
            <Eye className="mb-3 h-8 w-8 text-surface-600" />
            Click Preview to render this template.
          </div>
        )}
      </div>
    </div>
  )
}

function DeliveriesTab({ deliveries, channels }: { deliveries: NotificationDelivery[]; channels: NotificationChannel[] }) {
  if (deliveries.length === 0) return <EmptyState icon={Clock} title="No deliveries yet" description="Sent notification attempts will appear here." />
  return (
    <div className="card overflow-hidden">
      <div className="border-b border-surface-800 bg-surface-900/80 p-5"><h2 className="font-semibold text-surface-100">Delivery History</h2><p className="text-sm text-surface-500">Recent notification attempts and failures.</p></div>
      <div className="divide-y divide-surface-800">
        {deliveries.map(delivery => {
          const channel = channels.find(item => item.id === delivery.channel_id)
          const ok = delivery.status === 'sent'
          return (
            <div key={delivery.id} className="grid gap-3 p-4 md:grid-cols-[1fr_auto] md:items-center">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2"><span className="font-medium text-surface-100">{eventMeta(delivery.event_type).label}</span><span className="badge bg-surface-800 text-surface-300">{channel?.name || 'Unknown channel'}</span><span className={cn('badge', severityClass(delivery.severity))}>{delivery.severity}</span></div>
                {delivery.last_error && <div className="mt-2 rounded-lg border border-red-500/30 bg-red-500/10 p-2 text-xs text-red-300">{delivery.last_error}</div>}
              </div>
              <div className="text-left md:text-right"><span className={cn('badge', ok ? 'bg-emerald-500/20 text-emerald-300' : 'bg-red-500/20 text-red-300')}>{ok ? 'sent' : delivery.status}</span><div className="mt-1 text-xs text-surface-500">{formatRelativeTime(delivery.sent_at || delivery.created_at)}</div></div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function ChannelCard({ channel, onEdit, onDelete, onTest, testing }: { channel: NotificationChannel; onEdit: () => void; onDelete: () => void; onTest: () => void; testing: boolean }) {
  const Icon = channel.type === 'telegram' ? MessageCircle : channel.type === 'discord' ? Hash : Webhook
  const activeEvents = channel.events.length ? channel.events : EVENTS.map(event => event.value)
  return (
    <div className="card overflow-hidden transition-colors hover:border-accent-500/30">
      <div className="p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-3"><div className="rounded-xl border border-accent-500/30 bg-accent-500/10 p-2"><Icon className="h-5 w-5 text-accent-300" /></div><div><h2 className="font-semibold text-surface-100 truncate">{channel.name}</h2><div className="mt-1 text-xs capitalize text-surface-500">{channel.type} · minimum {channel.min_severity}</div></div></div>
          </div>
          <span className={cn('badge', channel.enabled ? 'bg-emerald-500/20 text-emerald-300' : 'bg-surface-800 text-surface-500')}>{channel.enabled ? 'Enabled' : 'Disabled'}</span>
        </div>
        <div className="mt-4 flex flex-wrap gap-1.5">{activeEvents.slice(0, 6).map(event => <span key={event} className="badge bg-surface-800 text-surface-300">{eventMeta(event).label}</span>)}{activeEvents.length > 6 && <span className="badge bg-surface-800 text-surface-500">+{activeEvents.length - 6}</span>}</div>
      </div>
      <div className="flex items-center justify-end gap-2 border-t border-surface-800 bg-surface-900/50 p-3"><button onClick={onTest} disabled={testing || !channel.enabled} className="btn btn-secondary text-xs"><Send className="h-3.5 w-3.5 mr-1" />Test</button><button onClick={onEdit} className="btn btn-secondary text-xs">Edit</button><button onClick={onDelete} className="rounded-lg border border-red-500/40 bg-red-500/10 px-2.5 py-1 text-xs font-semibold text-red-300 hover:bg-red-500/20"><Trash2 className="h-3.5 w-3.5" /></button></div>
    </div>
  )
}

function ChannelModal({ channel, onClose, onSave, saving }: { channel: Partial<NotificationChannel>; onClose: () => void; onSave: (channel: Partial<NotificationChannel>) => void; saving: boolean }) {
  const [form, setForm] = useState<Partial<NotificationChannel>>(channel)
  const config = form.config || {}
  const setConfig = (key: string, value: string) => setForm(prev => ({ ...prev, config: { ...(prev.config || {}), [key]: value } }))
  const toggleEvent = (event: string) => setForm(prev => ({ ...prev, events: prev.events?.includes(event) ? prev.events.filter(item => item !== event) : [...(prev.events || []), event] }))
  const canSave = Boolean(form.name?.trim())

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="w-full max-w-3xl overflow-hidden rounded-2xl border border-surface-800 bg-surface-950 shadow-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between border-b border-surface-800 bg-surface-900/80 p-5"><div><h2 className="text-lg font-semibold text-surface-100">{form.id ? 'Edit Channel' : 'Add Channel'}</h2><p className="text-sm text-surface-500">Choose where alerts should be delivered.</p></div><button onClick={onClose} className="rounded-lg p-2 text-surface-400 hover:bg-surface-800 hover:text-surface-100"><X className="h-5 w-5" /></button></div>
        <div className="space-y-5 p-5">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3"><Field label="Name"><input value={form.name || ''} onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))} className="input" placeholder="Production Discord" /></Field><Field label="Type"><select value={form.type || 'discord'} onChange={e => setForm(prev => ({ ...prev, type: e.target.value as NotificationChannel['type'], config: {} }))} className="input"><option value="discord">Discord</option><option value="telegram">Telegram</option><option value="webhook">Webhook</option></select></Field><Field label="Minimum severity"><select value={form.min_severity || 'info'} onChange={e => setForm(prev => ({ ...prev, min_severity: e.target.value as NotificationChannel['min_severity'] }))} className="input">{SEVERITIES.map(severity => <option key={severity} value={severity}>{severity}</option>)}</select></Field><label className="flex items-center gap-2 pt-6"><input type="checkbox" checked={form.enabled ?? true} onChange={e => setForm(prev => ({ ...prev, enabled: e.target.checked }))} /> <span className="text-sm text-surface-300">Enabled</span></label></div>
          {form.type === 'telegram' && <div className="grid grid-cols-1 md:grid-cols-2 gap-3"><Field label="Bot token"><input value={String(config.bot_token || '')} onChange={e => setConfig('bot_token', e.target.value)} className="input" type="password" /></Field><Field label="Chat ID"><input value={String(config.chat_id || '')} onChange={e => setConfig('chat_id', e.target.value)} className="input" /></Field></div>}
          {form.type === 'discord' && <Field label="Webhook URL"><input value={String(config.webhook_url || '')} onChange={e => setConfig('webhook_url', e.target.value)} className="input" type="password" placeholder="https://discord.com/api/webhooks/..." /></Field>}
          {form.type === 'webhook' && <div className="grid grid-cols-1 md:grid-cols-2 gap-3"><Field label="URL"><input value={String(config.url || '')} onChange={e => setConfig('url', e.target.value)} className="input" /></Field><Field label="Secret header value"><input value={String(config.secret || '')} onChange={e => setConfig('secret', e.target.value)} className="input" type="password" /></Field></div>}
          <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-4"><div className="mb-3 flex items-center justify-between"><div><div className="font-semibold text-surface-100">Events</div><div className="text-sm text-surface-500">Select what this channel should receive.</div></div><button onClick={() => setForm(prev => ({ ...prev, events: EVENTS.map(event => event.value) }))} className="text-xs text-accent-300 hover:text-accent-200">Select all</button></div><div className="grid grid-cols-1 md:grid-cols-2 gap-2">{EVENTS.map(event => <label key={event.value} className={cn('flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors', form.events?.includes(event.value) ? `${event.bg} ${event.border}` : 'border-surface-800 bg-surface-950/40 hover:border-surface-700')}><input type="checkbox" checked={form.events?.includes(event.value) || false} onChange={() => toggleEvent(event.value)} className="mt-1" /><span><span className="block text-sm font-medium text-surface-100">{event.label}</span><span className="text-xs text-surface-500">{event.description}</span></span></label>)}</div></div>
        </div>
        <div className="flex justify-end gap-2 border-t border-surface-800 bg-surface-900/60 p-5"><button onClick={onClose} className="btn btn-secondary">Cancel</button><button disabled={!canSave || saving} onClick={() => onSave(form)} className="btn btn-primary">{saving ? 'Saving...' : 'Save channel'}</button></div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block"><span className="mb-1 block text-xs font-medium text-surface-500">{label}</span>{children}</label>
}

function EmptyState({ icon: Icon, title, description }: { icon: React.ComponentType<{ className?: string }>; title: string; description: string }) {
  return <div className="card flex min-h-72 flex-col items-center justify-center p-8 text-center"><Icon className="mb-4 h-12 w-12 text-surface-600" /><h3 className="text-lg font-semibold text-surface-200">{title}</h3><p className="mt-2 max-w-md text-sm text-surface-500">{description}</p></div>
}

function formatLabel(format: NotificationTemplate['format']) {
  switch (format) {
    case 'telegram_html': return 'Telegram HTML'
    case 'discord_embed': return 'Discord Embed'
    case 'json': return 'Webhook JSON'
    default: return 'Text'
  }
}

function severityClass(severity: string) {
  switch (severity) {
    case 'success': return 'bg-emerald-500/20 text-emerald-300'
    case 'warning': return 'bg-amber-500/20 text-amber-300'
    case 'error': return 'bg-red-500/20 text-red-300'
    case 'critical': return 'bg-red-600/30 text-red-200'
    default: return 'bg-blue-500/20 text-blue-300'
  }
}
