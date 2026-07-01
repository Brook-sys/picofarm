import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, Plus, Send, Trash2, Webhook, MessageCircle, Hash, X, FileText, Eye, RotateCcw } from 'lucide-react'
import { notificationsApi } from '../api/client'
import { cn, formatRelativeTime } from '../lib/utils'
import type { NotificationChannel, NotificationTemplate, NotificationPreview } from '../types'

const EVENTS = ['print.started', 'print.completed', 'print.failed', 'print.cancelled', 'printer.offline', 'printer.online', 'printer.error', 'emergency.stop', 'queue.blocked', 'spool.low']
const SEVERITIES = ['info', 'success', 'warning', 'error', 'critical'] as const
const VARIABLES = ['{{event}}', '{{severity}}', '{{title}}', '{{message}}', '{{printer_name}}', '{{printer_model}}', '{{file_name}}', '{{status}}', '{{progress}}', '{{duration}}', '{{filament_grams}}', '{{wasted_grams}}', '{{notes}}', '{{timestamp}}']

const DEFAULT_FORM: Partial<NotificationChannel> = { name: '', type: 'discord', enabled: true, config: {}, events: ['print.completed', 'print.failed'], printer_ids: [], min_severity: 'info' }

function defaultTemplate(channelId: string, eventType = 'print.completed'): NotificationTemplate {
  return { channel_id: channelId, event_type: eventType, format: 'text', title_template: '{{title}}', body_template: '{{message}}', payload_template: '', enabled: true }
}

export default function Notifications() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<'channels' | 'templates' | 'deliveries'>('channels')
  const [editing, setEditing] = useState<Partial<NotificationChannel> | null>(null)
  const [error, setError] = useState('')
  const { data: channels = [], isLoading } = useQuery({ queryKey: ['notifications'], queryFn: notificationsApi.list })
  const { data: deliveries = [] } = useQuery({ queryKey: ['notification-deliveries'], queryFn: () => notificationsApi.deliveries() })
  const { data: templates = [] } = useQuery({ queryKey: ['notification-templates'], queryFn: () => notificationsApi.templates() })

  const saveMutation = useMutation({ mutationFn: (channel: Partial<NotificationChannel>) => channel.id ? notificationsApi.update(channel.id, channel) : notificationsApi.create(channel), onSuccess: () => { setEditing(null); setError(''); queryClient.invalidateQueries({ queryKey: ['notifications'] }) }, onError: err => setError(err instanceof Error ? err.message : 'Failed to save notification channel') })
  const deleteMutation = useMutation({ mutationFn: notificationsApi.delete, onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }) })
  const testMutation = useMutation({ mutationFn: notificationsApi.test, onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notification-deliveries'] }), onError: err => setError(err instanceof Error ? err.message : 'Failed to send test notification') })

  return (
    <div className="p-4 sm:p-6 lg:p-8 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-display font-bold text-surface-100 flex items-center gap-3"><Bell className="h-7 w-7 text-accent-400" />Notifications</h1>
          <p className="text-surface-500 mt-1">Configurable Telegram, Discord and webhook alerts with editable templates.</p>
        </div>
        {tab === 'channels' && <button onClick={() => setEditing(DEFAULT_FORM)} className="btn btn-primary"><Plus className="h-4 w-4 mr-2" /> Add Channel</button>}
      </div>

      <div className="mb-6 flex gap-2 border-b border-surface-800">
        {(['channels', 'templates', 'deliveries'] as const).map(item => <button key={item} onClick={() => setTab(item)} className={cn('px-4 py-2 text-sm font-semibold capitalize border-b-2', tab === item ? 'border-accent-500 text-accent-300' : 'border-transparent text-surface-500 hover:text-surface-200')}>{item}</button>)}
      </div>

      {error && <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      {tab === 'channels' && <ChannelsTab channels={channels} isLoading={isLoading} onEdit={setEditing} onDelete={id => deleteMutation.mutate(id)} onTest={id => testMutation.mutate(id)} testing={testMutation.isPending} />}
      {tab === 'templates' && <TemplatesTab channels={channels} templates={templates} onError={setError} />}
      {tab === 'deliveries' && <DeliveriesTab deliveries={deliveries} />}
      {editing && <ChannelModal channel={editing} onClose={() => setEditing(null)} onSave={channel => saveMutation.mutate(channel)} saving={saveMutation.isPending} />}
    </div>
  )
}

function ChannelsTab({ channels, isLoading, onEdit, onDelete, onTest, testing }: { channels: NotificationChannel[]; isLoading: boolean; onEdit: (channel: Partial<NotificationChannel>) => void; onDelete: (id: string) => void; onTest: (id: string) => void; testing: boolean }) {
  if (isLoading) return <div className="text-surface-500">Loading...</div>
  if (channels.length === 0) return <div className="card p-8 text-center text-surface-500">No notification channels yet.</div>
  return <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">{channels.map(channel => <ChannelCard key={channel.id} channel={channel} onEdit={() => onEdit(channel)} onDelete={() => onDelete(channel.id)} onTest={() => onTest(channel.id)} testing={testing} />)}</div>
}

function TemplatesTab({ channels, templates, onError }: { channels: NotificationChannel[]; templates: NotificationTemplate[]; onError: (error: string) => void }) {
  const queryClient = useQueryClient()
  const firstChannel = channels[0]?.id || ''
  const [channelId, setChannelId] = useState(firstChannel)
  const [eventType, setEventType] = useState('print.completed')
  const existing = templates.find(template => template.channel_id === channelId && template.event_type === eventType)
  const [draft, setDraft] = useState<NotificationTemplate>(existing || defaultTemplate(channelId, eventType))
  const [preview, setPreview] = useState<NotificationPreview | null>(null)

  const selectedChannel = channels.find(channel => channel.id === channelId)
  const saveMutation = useMutation({ mutationFn: notificationsApi.saveTemplate, onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['notification-templates'] }); onError('') }, onError: err => onError(err instanceof Error ? err.message : 'Failed to save template') })
  const previewMutation = useMutation({ mutationFn: notificationsApi.previewTemplate, onSuccess: setPreview, onError: err => onError(err instanceof Error ? err.message : 'Failed to preview template') })

  const reloadDraft = (nextChannelId: string, nextEventType: string) => {
    const found = templates.find(template => template.channel_id === nextChannelId && template.event_type === nextEventType)
    setDraft(found || defaultTemplate(nextChannelId, nextEventType))
    setPreview(null)
  }

  if (channels.length === 0) return <div className="card p-8 text-center text-surface-500">Create a channel first.</div>

  return (
    <div className="grid grid-cols-1 xl:grid-cols-[1fr_420px] gap-4">
      <div className="card p-5 space-y-4">
        <div className="flex items-center gap-2 mb-2"><FileText className="h-5 w-5 text-accent-400" /><h2 className="font-semibold text-surface-100">Message Template</h2></div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <label><span className="text-xs text-surface-500 mb-1 block">Channel</span><select value={channelId} onChange={e => { setChannelId(e.target.value); reloadDraft(e.target.value, eventType) }} className="input">{channels.map(channel => <option key={channel.id} value={channel.id}>{channel.name}</option>)}</select></label>
          <label><span className="text-xs text-surface-500 mb-1 block">Event</span><select value={eventType} onChange={e => { setEventType(e.target.value); reloadDraft(channelId, e.target.value) }} className="input">{EVENTS.map(event => <option key={event} value={event}>{event}</option>)}</select></label>
          <label><span className="text-xs text-surface-500 mb-1 block">Format</span><select value={draft.format} onChange={e => setDraft(prev => ({ ...prev, format: e.target.value as NotificationTemplate['format'] }))} className="input"><option value="text">Text</option><option value="telegram_html">Telegram HTML</option><option value="discord_embed">Discord Embed</option><option value="json">JSON</option></select></label>
        </div>
        <label className="block"><span className="text-xs text-surface-500 mb-1 block">Title template</span><input value={draft.title_template} onChange={e => setDraft(prev => ({ ...prev, title_template: e.target.value }))} className="input" /></label>
        <label className="block"><span className="text-xs text-surface-500 mb-1 block">Body template</span><textarea value={draft.body_template} onChange={e => setDraft(prev => ({ ...prev, body_template: e.target.value }))} className="input min-h-40 font-mono text-sm" /></label>
        {selectedChannel?.type === 'webhook' && <label className="block"><span className="text-xs text-surface-500 mb-1 block">Payload JSON template</span><textarea value={draft.payload_template} onChange={e => setDraft(prev => ({ ...prev, payload_template: e.target.value }))} className="input min-h-32 font-mono text-sm" placeholder='{"event":"{{event}}","file":"{{file_name}}"}' /></label>}
        <div><div className="text-xs text-surface-500 mb-2">Variables</div><div className="flex flex-wrap gap-1.5">{VARIABLES.map(variable => <button key={variable} onClick={() => navigator.clipboard?.writeText(variable)} className="badge bg-surface-800 text-surface-300 hover:text-surface-100">{variable}</button>)}</div></div>
        <div className="flex flex-wrap gap-2"><button onClick={() => previewMutation.mutate(draft)} className="btn btn-secondary"><Eye className="h-4 w-4 mr-2" />Preview</button><button onClick={() => saveMutation.mutate({ ...draft, channel_id: channelId, event_type: eventType })} disabled={saveMutation.isPending} className="btn btn-primary">Save template</button><button onClick={() => setDraft(defaultTemplate(channelId, eventType))} className="btn btn-secondary"><RotateCcw className="h-4 w-4 mr-2" />Reset draft</button></div>
      </div>
      <div className="card p-5">
        <h2 className="font-semibold text-surface-100 mb-3">Preview</h2>
        {preview ? <div className="space-y-4"><div><div className="text-xs text-surface-500 mb-1">Title</div><div className="rounded-lg bg-surface-900 p-3 text-surface-100">{preview.title}</div></div><div><div className="text-xs text-surface-500 mb-1">Body</div><pre className="rounded-lg bg-surface-900 p-3 text-sm text-surface-200 whitespace-pre-wrap">{preview.body}</pre></div>{preview.payload && <div><div className="text-xs text-surface-500 mb-1">Payload</div><pre className="rounded-lg bg-surface-900 p-3 text-xs text-surface-300 overflow-auto">{JSON.stringify(preview.payload, null, 2)}</pre></div>}</div> : <div className="text-sm text-surface-500">Click Preview to render sample data.</div>}
      </div>
    </div>
  )
}

function DeliveriesTab({ deliveries }: { deliveries: Array<{ id: string; event_type: string; severity: string; status: string; last_error: string; created_at: string }> }) {
  if (deliveries.length === 0) return <div className="card p-8 text-center text-surface-500">No deliveries yet.</div>
  return <div className="card p-5"><div className="space-y-2">{deliveries.map(delivery => <div key={delivery.id} className="flex items-center justify-between rounded-lg border border-surface-800 bg-surface-900/40 p-3 text-sm"><div><span className="text-surface-200">{delivery.event_type}</span><span className="ml-2 text-surface-500">{delivery.severity}</span>{delivery.last_error && <div className="text-xs text-red-400 mt-1">{delivery.last_error}</div>}</div><div className="text-right"><span className={cn('badge', delivery.status === 'sent' ? 'bg-emerald-500/20 text-emerald-300' : 'bg-red-500/20 text-red-300')}>{delivery.status}</span><div className="text-xs text-surface-500 mt-1">{formatRelativeTime(delivery.created_at)}</div></div></div>)}</div></div>
}

function ChannelCard({ channel, onEdit, onDelete, onTest, testing }: { channel: NotificationChannel; onEdit: () => void; onDelete: () => void; onTest: () => void; testing: boolean }) {
  const Icon = channel.type === 'telegram' ? MessageCircle : channel.type === 'discord' ? Hash : Webhook
  return <div className="card p-5"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><div className="flex items-center gap-2"><Icon className="h-5 w-5 text-accent-400" /><h2 className="font-semibold text-surface-100 truncate">{channel.name}</h2><span className={cn('badge', channel.enabled ? 'bg-emerald-500/20 text-emerald-300' : 'bg-surface-800 text-surface-500')}>{channel.enabled ? 'Enabled' : 'Disabled'}</span></div><div className="mt-2 text-sm text-surface-500 capitalize">{channel.type} · min {channel.min_severity}</div><div className="mt-3 flex flex-wrap gap-1.5">{(channel.events.length ? channel.events : ['all events']).map(event => <span key={event} className="badge bg-surface-800 text-surface-300">{event}</span>)}</div></div><div className="flex gap-2"><button onClick={onTest} disabled={testing} className="btn btn-secondary text-xs"><Send className="h-3.5 w-3.5 mr-1" />Test</button><button onClick={onEdit} className="btn btn-secondary text-xs">Edit</button><button onClick={onDelete} className="rounded-lg border border-red-500/40 bg-red-500/10 px-2.5 py-1 text-xs font-semibold text-red-300 hover:bg-red-500/20"><Trash2 className="h-3.5 w-3.5" /></button></div></div></div>
}

function ChannelModal({ channel, onClose, onSave, saving }: { channel: Partial<NotificationChannel>; onClose: () => void; onSave: (channel: Partial<NotificationChannel>) => void; saving: boolean }) {
  const [form, setForm] = useState<Partial<NotificationChannel>>(channel)
  const config = form.config || {}
  const setConfig = (key: string, value: string) => setForm(prev => ({ ...prev, config: { ...(prev.config || {}), [key]: value } }))
  const toggleEvent = (event: string) => setForm(prev => ({ ...prev, events: prev.events?.includes(event) ? prev.events.filter(item => item !== event) : [...(prev.events || []), event] }))
  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"><div className="w-full max-w-2xl rounded-2xl border border-surface-800 bg-surface-950 shadow-2xl max-h-[90vh] overflow-y-auto"><div className="flex items-center justify-between border-b border-surface-800 p-5"><h2 className="text-lg font-semibold text-surface-100">{form.id ? 'Edit Channel' : 'Add Channel'}</h2><button onClick={onClose} className="rounded-lg p-2 text-surface-400 hover:bg-surface-800 hover:text-surface-100"><X className="h-5 w-5" /></button></div><div className="p-5 space-y-4"><div className="grid grid-cols-1 md:grid-cols-2 gap-3"><label className="block"><span className="text-xs text-surface-500 mb-1 block">Name</span><input value={form.name || ''} onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))} className="input" /></label><label className="block"><span className="text-xs text-surface-500 mb-1 block">Type</span><select value={form.type || 'discord'} onChange={e => setForm(prev => ({ ...prev, type: e.target.value as NotificationChannel['type'], config: {} }))} className="input"><option value="discord">Discord</option><option value="telegram">Telegram</option><option value="webhook">Webhook</option></select></label><label className="block"><span className="text-xs text-surface-500 mb-1 block">Minimum severity</span><select value={form.min_severity || 'info'} onChange={e => setForm(prev => ({ ...prev, min_severity: e.target.value as NotificationChannel['min_severity'] }))} className="input">{SEVERITIES.map(severity => <option key={severity} value={severity}>{severity}</option>)}</select></label><label className="flex items-center gap-2 pt-6"><input type="checkbox" checked={form.enabled ?? true} onChange={e => setForm(prev => ({ ...prev, enabled: e.target.checked }))} /> <span className="text-sm text-surface-300">Enabled</span></label></div>{form.type === 'telegram' && <div className="grid grid-cols-1 md:grid-cols-2 gap-3"><input value={String(config.bot_token || '')} onChange={e => setConfig('bot_token', e.target.value)} className="input" placeholder="Bot Token" /><input value={String(config.chat_id || '')} onChange={e => setConfig('chat_id', e.target.value)} className="input" placeholder="Chat ID" /></div>}{form.type === 'discord' && <input value={String(config.webhook_url || '')} onChange={e => setConfig('webhook_url', e.target.value)} className="input" placeholder="Discord Webhook URL" />}{form.type === 'webhook' && <div className="grid grid-cols-1 gap-3"><input value={String(config.url || '')} onChange={e => setConfig('url', e.target.value)} className="input" placeholder="Webhook URL" /><input value={String(config.secret || '')} onChange={e => setConfig('secret', e.target.value)} className="input" placeholder="Optional HMAC secret" /></div>}<div><div className="text-xs text-surface-500 mb-2">Events</div><div className="grid grid-cols-1 sm:grid-cols-2 gap-2">{EVENTS.map(event => <label key={event} className="flex items-center gap-2 rounded-lg border border-surface-800 bg-surface-900/40 p-2 text-sm text-surface-300"><input type="checkbox" checked={form.events?.includes(event) || false} onChange={() => toggleEvent(event)} /> {event}</label>)}</div></div></div><div className="flex justify-end gap-2 border-t border-surface-800 p-5"><button onClick={onClose} className="btn btn-secondary">Cancel</button><button onClick={() => onSave(form)} disabled={saving} className="btn btn-primary">Save</button></div></div></div>
}
