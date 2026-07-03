import { useEffect, useState } from 'react'
import { Plus, Globe, CheckCircle, XCircle, RefreshCw, Server, ExternalLink, DownloadCloud, Upload, Pencil } from 'lucide-react'
import { slicerApi } from '../api/client'

type ProfileCategory = 'printers' | 'presets' | 'filaments'
type AddModalState = { category: ProfileCategory; mode: 'url' | 'json'; name: string; url: string; json: string; overwrite: boolean; status: 'idle' | 'importing'; message: string } | null
type EditModalState = { category: ProfileCategory; name: string; json: string; originalJson: string; status: 'loading' | 'idle' | 'saving' | 'saved' | 'error'; message: string } | null

const CATEGORIES: Array<{ key: ProfileCategory; title: string; description: string }> = [
  { key: 'printers', title: 'Printers', description: 'Orca printer machine profiles' },
  { key: 'presets', title: 'Presets', description: 'Orca process/print profiles' },
  { key: 'filaments', title: 'Filaments', description: 'Orca filament profiles' },
]

export default function SlicerSettings() {
  const [connectionUrl, setConnectionUrl] = useState('')
  const [health, setHealth] = useState<Record<string, unknown> | null>(null)
  const [status, setStatus] = useState<Record<string, unknown> | null>(null)
  const [profiles, setProfiles] = useState<Record<ProfileCategory, Array<Record<string, unknown>>>>({ printers: [], presets: [], filaments: [] })
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [addModal, setAddModal] = useState<AddModalState>(null)
  const [editModal, setEditModal] = useState<EditModalState>(null)

  useEffect(() => {
    let cancelled = false

    const loadInitialState = async () => {
      setBusy('initial')
      setError('')
      try {
        const cfg = await slicerApi.getConfig()
        if (cancelled) return
        setConnectionUrl(cfg.connection_url || '')
        if (!cfg.connection_url) return

        const [healthRes, statusRes, printers, presets, filaments] = await Promise.all([
          slicerApi.health(),
          slicerApi.status().catch(() => null),
          slicerApi.profiles('printers'),
          slicerApi.profiles('presets'),
          slicerApi.profiles('filaments'),
        ])
        if (cancelled) return
        setHealth(healthRes)
        setStatus(statusRes)
        setProfiles({ printers, presets, filaments })
      } catch (err) {
        if (!cancelled) {
          setHealth(null)
          setStatus(null)
          setError(err instanceof Error ? err.message : 'Slicer offline')
        }
      } finally {
        if (!cancelled) setBusy('')
      }
    }

    loadInitialState()
    return () => { cancelled = true }
  }, [])

  const editCategory = editModal?.category
  const editName = editModal?.name
  const editJson = editModal?.json
  const editOriginalJson = editModal?.originalJson
  const editStatus = editModal?.status

  useEffect(() => {
    if (!editCategory || !editName || !editJson || editStatus === 'loading' || editJson === editOriginalJson) return
    const timer = window.setTimeout(async () => {
      try {
        JSON.parse(editJson)
        setEditModal(current => current ? { ...current, status: 'saving', message: 'Saving...' } : current)
        await slicerApi.uploadProfileJSON({ category: editCategory, name: editName, json: editJson })
        setEditModal(current => current ? { ...current, originalJson: current.json, status: 'saved', message: 'Saved automatically' } : current)
        await loadProfiles(false)
      } catch (err) {
        setEditModal(current => current ? { ...current, status: 'error', message: err instanceof Error ? err.message : 'Save failed' } : current)
      }
    }, 900)
    return () => window.clearTimeout(timer)
  }, [editCategory, editName, editJson, editOriginalJson, editStatus])

  const saveConnection = async () => {
    setBusy('save')
    setError('')
    setSuccess('')
    try {
      const cfg = await slicerApi.setConfig(connectionUrl)
      setConnectionUrl(cfg.connection_url || '')
      await checkConnection()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save slicer URL')
    } finally {
      setBusy('')
    }
  }

  const checkConnection = async () => {
    setBusy('health')
    setError('')
    setSuccess('')
    try {
      const [healthRes, statusRes] = await Promise.all([slicerApi.health(), slicerApi.status().catch(() => null)])
      setHealth(healthRes)
      setStatus(statusRes)
      setSuccess('Slicer connection checked successfully.')
    } catch (err) {
      setHealth(null)
      setStatus(null)
      setError(err instanceof Error ? err.message : 'Slicer offline')
    } finally {
      setBusy('')
    }
  }

  const loadProfiles = async (showFeedback = true) => {
    if (showFeedback) {
      setBusy('profiles')
      setError('')
      setSuccess('')
    }
    try {
      const [printers, presets, filaments] = await Promise.all([slicerApi.profiles('printers'), slicerApi.profiles('presets'), slicerApi.profiles('filaments')])
      setProfiles({ printers, presets, filaments })
      if (showFeedback) setSuccess(`Profiles loaded: ${printers.length} printers, ${presets.length} presets, ${filaments.length} filaments.`)
    } catch (err) {
      if (showFeedback) setError(err instanceof Error ? err.message : 'Failed to load profiles')
    } finally {
      if (showFeedback) setBusy('')
    }
  }

  const syncExternal = async () => {
    setBusy('sync')
    setError('')
    setSuccess('')
    try {
      let synced = 0
      let skipped = 0
      for (const category of CATEGORIES) {
        for (const profile of profiles[category.key]) {
          if (profile.sourceUrl && profile.name) {
            await slicerApi.updateProfile(category.key, String(profile.name))
            synced++
          } else {
            skipped++
          }
        }
      }
      const [printers, presets, filaments] = await Promise.all([slicerApi.profiles('printers'), slicerApi.profiles('presets'), slicerApi.profiles('filaments')])
      setProfiles({ printers, presets, filaments })
      setSuccess(synced > 0 ? `Sync completed: ${synced} profile(s) updated from source. ${skipped} local/raw profile(s) skipped.` : 'No external source profiles to sync. Profiles imported as Raw JSON do not have a source URL.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync profiles')
    } finally {
      setBusy('')
    }
  }

  const setDefaultProfile = async (category: ProfileCategory, name: string) => {
    setBusy(`default-${category}`)
    setError('')
    setSuccess('')
    try {
      await slicerApi.setDefaultProfile({ category, name })
      await loadProfiles(false)
      setSuccess(`${name} is now the default ${category} profile.`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to set default profile')
    } finally {
      setBusy('')
    }
  }

  const editProfile = async (category: ProfileCategory, name: string) => {
    setEditModal({ category, name, json: '', originalJson: '', status: 'loading', message: 'Loading profile...' })
    try {
      const profile = await slicerApi.profileJSON(category, name)
      const json = JSON.stringify(profile, null, 2)
      setEditModal({ category, name, json, originalJson: json, status: 'idle', message: '' })
    } catch (err) {
      setEditModal({ category, name, json: '', originalJson: '', status: 'error', message: err instanceof Error ? err.message : 'Failed to load profile' })
    }
  }

  const uploadProfileFiles = async (category: ProfileCategory, fileList?: FileList | null) => {
    if (!fileList || fileList.length === 0) return
    setBusy(`upload-${category}`)
    setError('')
    setSuccess('')
    try {
      let uploaded = 0
      const failed: string[] = []
      for (const file of Array.from(fileList)) {
        try {
          const json = await file.text()
          let profileName = file.name.replace(/\.json$/i, '')
          try {
            const parsed = JSON.parse(json)
            if (typeof parsed.name === 'string' && parsed.name.trim()) profileName = parsed.name.trim()
          } catch {
            throw new Error('Invalid JSON')
          }
          await slicerApi.uploadProfileJSON({ category, name: profileName, json })
          uploaded++
        } catch (err) {
          failed.push(`${file.name}: ${err instanceof Error ? err.message : 'failed'}`)
        }
      }
      await loadProfiles()
      setSuccess(`Uploaded ${uploaded} ${category} profile(s) to the Orca slicer server.${failed.length ? ` Failed: ${failed.join('; ')}` : ''}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to upload profiles')
    } finally {
      setBusy('')
    }
  }

  const importProfile = async () => {
    if (!addModal) return
    setAddModal({ ...addModal, status: 'importing', message: 'Importing...' })
    setError('')
    setSuccess('')
    try {
      if (addModal.mode === 'json') {
        await slicerApi.uploadProfileJSON({ category: addModal.category, name: addModal.name, json: addModal.json })
      } else {
        await slicerApi.importProfile({ category: addModal.category, name: addModal.name, url: addModal.url, overwrite: addModal.overwrite })
      }
      setAddModal(null)
      await loadProfiles()
      setSuccess(`Profile "${addModal.name}" saved on the Orca slicer server (${addModal.category}).`)
    } catch (err) {
      setAddModal(current => current ? { ...current, status: 'idle', message: err instanceof Error ? err.message : 'Import failed' } : current)
    }
  }

  const isHealthy = health?.status === 'healthy'

  return (
    <div className="p-4 sm:p-6 lg:p-8 max-w-6xl mx-auto">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-display font-bold text-surface-100">Slicer Settings</h1>
          <p className="text-surface-400 mt-1">Connect PicoFarm to an external OrcaSlicer API container.</p>
        </div>
        <div className="flex gap-2">
          <button onClick={() => loadProfiles()} disabled={!!busy} className="btn btn-secondary shrink-0"><RefreshCw className="h-4 w-4 mr-2" /> Load profiles</button>
          <button onClick={syncExternal} disabled={!!busy} className="btn btn-primary shrink-0"><DownloadCloud className="h-4 w-4 mr-2" /> Sync external</button>
        </div>
      </div>

      {error && <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      {success && <div className="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-300">{success}</div>}

      <div className="card p-5 mb-6">
        <h2 className="text-lg font-semibold text-surface-100 mb-4 flex items-center gap-2"><Server className="h-5 w-5 text-accent-400" /> Orca Slicer API Container</h2>
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_auto] gap-4 items-end">
          <label className="block">
            <span className="text-sm font-medium text-surface-300 mb-1 block">Connection URL</span>
            <input type="text" value={connectionUrl} onChange={e => setConnectionUrl(e.target.value)} placeholder="http://localhost:3000" className="input" />
          </label>
          <div className="flex gap-2">
            <button onClick={saveConnection} disabled={!connectionUrl || !!busy} className="btn btn-primary"><Globe className="h-4 w-4 mr-2" /> Save & Check</button>
            <button onClick={checkConnection} disabled={!connectionUrl || !!busy} className="btn btn-secondary">Check</button>
          </div>
        </div>
        <div className="mt-4 grid grid-cols-1 md:grid-cols-3 gap-3 text-sm">
          <HealthBadge label="API" ok={!!health} value={isHealthy ? 'Healthy' : health ? 'Unhealthy' : 'Not checked'} />
          <HealthBadge label="Orca configured" ok={health?.orcaSlicerConfigured === true} value={String(health?.orcaSlicerConfigured ?? '—')} />
          <HealthBadge label="Orca executable" ok={health?.orcaSlicerExecutable === true} value={String(health?.orcaSlicerExecutable ?? '—')} />
        </div>
        {status && <pre className="mt-4 rounded-lg bg-surface-950 border border-surface-800 p-3 text-xs text-surface-400 overflow-auto">{JSON.stringify(status, null, 2)}</pre>}
        <div className="mt-4 text-xs text-surface-500">
          Container: <span className="font-mono">Brook-sys/orca-slicer-api</span>. Expected endpoints: <span className="font-mono">/health</span>, <span className="font-mono">/profiles/*</span>, <span className="font-mono">/slice</span>.
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        {CATEGORIES.map(category => <ProfileSection key={category.key} category={category.key} title={category.title} description={category.description} items={profiles[category.key]} onAdd={() => setAddModal({ category: category.key, mode: 'url', name: '', url: '', json: '', overwrite: true, status: 'idle', message: '' })} onUpload={files => uploadProfileFiles(category.key, files)} onEdit={name => editProfile(category.key, name)} onSetDefault={name => setDefaultProfile(category.key, name)} uploading={busy === `upload-${category.key}` || busy === `default-${category.key}`} />)}
      </div>

      {editModal && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"><div className="card w-full max-w-4xl p-6"><div className="flex items-start justify-between gap-4 mb-4"><div><h2 className="text-xl font-semibold text-surface-100">Edit {editModal.name}</h2><p className="text-sm text-surface-500">{editModal.category} · JSON autosaves after you stop typing.</p></div><button onClick={() => setEditModal(null)} className="btn btn-secondary">Close</button></div>{editModal.message && <div className={editModal.status === 'error' ? 'mb-3 text-sm text-red-300' : 'mb-3 text-sm text-emerald-300'}>{editModal.message}</div>}{editModal.status === 'loading' ? <div className="py-12 text-center text-surface-500">Loading...</div> : <textarea value={editModal.json} onChange={e => setEditModal(current => current ? { ...current, json: e.target.value, status: 'idle', message: '' } : current)} className="input min-h-[60vh] font-mono text-xs" spellCheck={false} />}</div></div>}

      {addModal && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"><div className="card w-full max-w-lg p-6"><h2 className="text-xl font-semibold text-surface-100 mb-1">Import {addModal.category} profile</h2><p className="text-sm text-surface-500 mb-4">Import from a raw HTTPS URL or paste the Orca profile JSON directly.</p><div className="space-y-4"><div className="flex rounded-lg border border-surface-800 bg-surface-900/50 p-1"><button onClick={() => setAddModal({ ...addModal, mode: 'url' })} className={addModal.mode === 'url' ? 'btn btn-primary flex-1 text-xs' : 'btn btn-ghost flex-1 text-xs'}>URL</button><button onClick={() => setAddModal({ ...addModal, mode: 'json' })} className={addModal.mode === 'json' ? 'btn btn-primary flex-1 text-xs' : 'btn btn-ghost flex-1 text-xs'}>Raw JSON</button></div><label className="block"><span className="text-xs text-surface-500 mb-1 block">Profile name {addModal.mode === 'json' ? '(optional, uses JSON name)' : ''}</span><input value={addModal.name} onChange={e => setAddModal({ ...addModal, name: e.target.value })} className="input" placeholder="standard-020" autoFocus /></label>{addModal.mode === 'url' ? <><label className="block"><span className="text-xs text-surface-500 mb-1 block">Raw JSON URL</span><input value={addModal.url} onChange={e => setAddModal({ ...addModal, url: e.target.value })} className="input" placeholder="https://raw.githubusercontent.com/.../profile.json" /></label><label className="flex items-center gap-2 text-sm text-surface-300"><input type="checkbox" checked={addModal.overwrite} onChange={e => setAddModal({ ...addModal, overwrite: e.target.checked })} />Overwrite existing profile</label></> : <label className="block"><span className="text-xs text-surface-500 mb-1 block">Profile JSON</span><textarea value={addModal.json} onChange={e => setAddModal({ ...addModal, json: e.target.value })} className="input min-h-56 font-mono text-xs" placeholder='{ "layer_height": "0.20" }' /></label>}{addModal.message && <div className="text-xs text-red-400">{addModal.message}</div>}</div><div className="mt-6 flex justify-end gap-2"><button onClick={() => setAddModal(null)} className="btn btn-secondary">Cancel</button><button onClick={importProfile} disabled={(addModal.mode === 'url' && (!addModal.name.trim() || !addModal.url.trim())) || (addModal.mode === 'json' && !addModal.json.trim()) || addModal.status === 'importing'} className="btn btn-primary">Import</button></div></div></div>}
    </div>
  )
}

function HealthBadge({ label, ok, value }: { label: string; ok: boolean; value: string }) {
  return <div className="rounded-lg border border-surface-800 bg-surface-900/50 p-3"><div className="text-xs text-surface-500 mb-1">{label}</div><div className={ok ? 'text-emerald-400 flex items-center gap-1.5' : 'text-red-400 flex items-center gap-1.5'}>{ok ? <CheckCircle className="h-4 w-4" /> : <XCircle className="h-4 w-4" />}{value}</div></div>
}

function ProfileSection({ category, title, description, items, onAdd, onUpload, onEdit, onSetDefault, uploading }: { category: ProfileCategory; title: string; description: string; items: Array<Record<string, unknown>>; onAdd: () => void; onUpload: (files?: FileList | null) => void; onEdit: (name: string) => void; onSetDefault: (name: string) => void; uploading: boolean }) {
  return <div className="card p-5"><div className="flex items-start justify-between gap-3 mb-4"><div><h3 className="font-semibold text-surface-100">{title}</h3><p className="text-xs text-surface-500 mt-1">{description}</p></div><div className="flex gap-2"><label className="btn btn-secondary text-xs py-1.5 px-3 cursor-pointer"><Upload className="h-3.5 w-3.5 mr-1" /> {uploading ? 'Uploading...' : 'Upload JSON'}<input type="file" accept=".json,application/json" multiple className="hidden" onChange={e => { onUpload(e.target.files); e.currentTarget.value = '' }} /></label><button onClick={onAdd} className="btn btn-secondary text-xs py-1.5 px-3"><Plus className="h-3.5 w-3.5 mr-1" /> Import</button></div></div><div className="space-y-3">{items.map((item, index) => <div key={`${category}-${String(item.name)}-${index}`} className="rounded-xl border border-surface-800 bg-surface-900/60 p-3"><div className="flex items-start justify-between gap-2"><div className="min-w-0"><div className="flex items-center gap-2"><div className="font-medium text-surface-100 truncate">{String(item.name || 'Unnamed')}</div>{item.default ? <span className="rounded-full border border-accent-500/40 bg-accent-500/10 px-2 py-0.5 text-[10px] text-accent-300">Default</span> : null}</div><div className="mt-1 text-xs text-surface-500">{item.size ? `${String(item.size)} bytes` : 'profile'}{item.updatedAt ? ` · ${String(item.updatedAt)}` : ''}</div></div><div className="flex gap-2">{item.name && !item.default ? <button onClick={() => onSetDefault(String(item.name))} className="btn btn-secondary text-xs py-1.5 px-2">Default</button> : null}{item.name ? <button onClick={() => onEdit(String(item.name))} className="btn btn-secondary text-xs py-1.5 px-2"><Pencil className="h-3.5 w-3.5 mr-1" /> Edit</button> : null}</div></div>{typeof item.sourceUrl === 'string' && item.sourceUrl.length > 0 ? <a href={String(item.sourceUrl)} target="_blank" rel="noreferrer" className="mt-2 inline-flex items-center text-xs text-blue-300 hover:text-blue-200"><ExternalLink className="h-3 w-3 mr-1" /> Source</a> : null}</div>)}{items.length === 0 && <div className="text-center py-6 text-surface-500 text-sm">No {title.toLowerCase()} loaded</div>}</div></div>
}
