import { useTranslation } from 'react-i18next'
import { externalLinkClick } from '../lib/platform'
import { useEffect, useRef, useState } from 'react'
import { Plus, Globe, CheckCircle, XCircle, RefreshCw, Server, ExternalLink, DownloadCloud, Upload, Pencil, Save } from 'lucide-react'
import { slicerApi, type SlicerProfileBundle } from '../api/client'

type ProfileCategory = 'printers' | 'presets' | 'filaments'
type AddModalState = { category: ProfileCategory; mode: 'url' | 'json'; name: string; url: string; json: string; overwrite: boolean; status: 'idle' | 'importing'; message: string } | null
type ProfileRevision = { id: number; hash: string; json: string; created_at: string }
type EditModalState = { category: ProfileCategory; name: string; json: string; originalJson: string; currentHash: string; revisions: ProfileRevision[]; validation: { valid: boolean; errors: string[]; warnings: string[] } | null; status: 'loading' | 'validating' | 'idle' | 'saving' | 'saved' | 'error'; message: string } | null
type RestoreState = { bundle: SlicerProfileBundle; overwrite: boolean; status: 'idle' | 'restoring'; message: string } | null

const CATEGORIES: Array<{ key: ProfileCategory; title: string; description: string }> = [
  { key: 'printers', title: 'Printers', description: 'Orca printer machine profiles' },
  { key: 'presets', title: 'Presets', description: 'Orca process/print profiles' },
  { key: 'filaments', title: 'Filaments', description: 'Orca filament profiles' },
]

export default function SlicerSettings() {
  const { t, i18n } = useTranslation('fleet')

  const [connectionUrl, setConnectionUrl] = useState('')
  // Where the effective URL came from: "env" means SLICER_CONNECTION_URL is set
  // on the server and the field below cannot change what the backend talks to.
  const [connectionUrlSource, setConnectionUrlSource] = useState('')
  const [health, setHealth] = useState<Record<string, unknown> | null>(null)
  const [status, setStatus] = useState<Record<string, unknown> | null>(null)
  const [profiles, setProfiles] = useState<Record<ProfileCategory, Array<Record<string, unknown>>>>({ printers: [], presets: [], filaments: [] })
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [addModal, setAddModal] = useState<AddModalState>(null)
  const [editModal, setEditModal] = useState<EditModalState>(null)
  const editorRequest = useRef(0)
  const editorBusy = useRef(false)
  const [restoreModal, setRestoreModal] = useState<RestoreState>(null)

  useEffect(() => {
    let cancelled = false

    const loadInitialState = async () => {
      setBusy('initial')
      setError('')
      try {
        const cfg = await slicerApi.getConfig()
        if (cancelled) return
        setConnectionUrl(cfg.connection_url || '')
        setConnectionUrlSource(cfg.connection_url_source || '')
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
          setError(err instanceof Error ? err.message : t("Slicer offline"))
        }
      } finally {
        if (!cancelled) setBusy('')
      }
    }

    loadInitialState()
    return () => { cancelled = true }
  }, [t])

  const editorDirty = !!editModal && editModal.json !== editModal.originalJson
  useEffect(() => {
    if (!editorDirty) return
    const warnBeforeUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    window.addEventListener('beforeunload', warnBeforeUnload)
    return () => window.removeEventListener('beforeunload', warnBeforeUnload)
  }, [editorDirty])

  const closeEditor = () => {
    if (editorBusy.current) return
    if (editModal && editModal.json !== editModal.originalJson && !window.confirm(t("Discard unsaved profile changes?"))) return
    editorRequest.current++
    setEditModal(null)
  }

  const validateEditor = async () => {
    if (!editModal || editorBusy.current) return
    const draft = editModal
    editorBusy.current = true
    setEditModal({ ...draft, status: 'validating', message: '' })
    try {
      const validation = await slicerApi.validateProfile({ category: draft.category, name: draft.name, json: draft.json })
      setEditModal(current => current ? { ...current, status: 'idle', validation, message: validation.valid ? t("Preflight checks passed. Review warnings before saving.") : t("Fix validation errors before saving.") } : current)
    } catch (err) {
      setEditModal(current => current ? { ...current, status: 'error', message: err instanceof Error ? err.message : t("Validation failed") } : current)
    } finally { editorBusy.current = false }
  }

  const saveEditor = async () => {
    if (!editModal || editorBusy.current || !editModal.validation?.valid || !editModal.currentHash) return
    const draft = editModal
    editorBusy.current = true
    setEditModal({ ...draft, status: 'saving', message: t("Saving with recovery snapshots...") })
    try {
      const result = await slicerApi.saveProfileRevision({ category: draft.category, name: draft.name, json: draft.json, expected_hash: draft.currentHash })
      const json = JSON.stringify(JSON.parse(result.current_json), null, 2)
      setEditModal(current => current ? { ...current, json, originalJson: json, currentHash: result.current_hash, revisions: result.revisions, validation: null, status: 'saved', message: t("Saved and read back from slicer.") } : current)
      await loadProfiles(false)
    } catch (err) {
      setEditModal(current => current ? { ...current, status: 'error', message: err instanceof Error ? err.message : t("Save failed; reload before retrying if outcome is uncertain.") } : current)
    } finally { editorBusy.current = false }
  }

  const saveConnection = async () => {
    setBusy('save')
    setError('')
    setSuccess('')
    try {
      const cfg = await slicerApi.setConfig(connectionUrl)
      setConnectionUrl(cfg.connection_url || '')
      setConnectionUrlSource(cfg.connection_url_source || '')
      await checkConnection()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to save slicer URL"))
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
      setSuccess(t("Slicer connection checked successfully."))
    } catch (err) {
      setHealth(null)
      setStatus(null)
      setError(err instanceof Error ? err.message : t("Slicer offline"))
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
      if (showFeedback) setSuccess(t("Profiles loaded: {{printers}} printers, {{presets}} presets, {{filaments}} filaments.", { printers: printers.length, presets: presets.length, filaments: filaments.length }))
    } catch (err) {
      if (showFeedback) setError(err instanceof Error ? err.message : t("Failed to load profiles"))
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
      setSuccess(synced > 0 ? t("Sync completed: {{synced}} profiles updated from source. {{skipped}} local/raw profiles skipped.", { synced, skipped }) : t("No external source profiles to sync. Profiles imported as Raw JSON do not have a source URL."))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to sync profiles"))
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
      setSuccess(t("{{name}} is now the default profile ({{category}}).", { name, category: t(CATEGORIES.find(item => item.key === category)!.title) }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to set default profile"))
    } finally {
      setBusy('')
    }
  }

  const editProfile = async (category: ProfileCategory, name: string) => {
    if (editorBusy.current) return
    const request = ++editorRequest.current
    const initial: NonNullable<EditModalState> = { category, name, json: '', originalJson: '', currentHash: '', revisions: [], validation: null, status: 'loading', message: t("Loading profile and recovery history...") }
    setEditModal(initial)
    try {
      const result = await slicerApi.profileHistory(category, name)
      if (request !== editorRequest.current) return
      const json = JSON.stringify(JSON.parse(result.current_json), null, 2)
      setEditModal({ ...initial, json, originalJson: json, currentHash: result.current_hash, revisions: result.revisions, status: 'idle', message: '' })
    } catch (err) {
      if (request !== editorRequest.current) return
      setEditModal({ ...initial, status: 'error', message: err instanceof Error ? err.message : t("Failed to load profile") })
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
            throw new Error(t("Invalid JSON"))
          }
          await slicerApi.uploadProfileJSON({ category, name: profileName, json })
          uploaded++
        } catch (err) {
          failed.push(`${file.name}: ${err instanceof Error ? err.message : t("failed")}`)
        }
      }
      await loadProfiles()
      setSuccess(t("Uploaded {{count}} profiles to the Orca slicer server. {{failures}}", { count: uploaded, failures: failed.length ? t('Failures: {{failures}}', { failures: failed.join('; ') }) : '' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to upload profiles"))
    } finally {
      setBusy('')
    }
  }

  const importProfile = async () => {
    if (!addModal) return
    setAddModal({ ...addModal, status: 'importing', message: t("Importing...") })
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
      setSuccess(t("Profile \"{{name}}\" saved on the Orca slicer server.", { name: addModal.name }))
    } catch (err) {
      setAddModal(current => current ? { ...current, status: 'idle', message: err instanceof Error ? err.message : t("Import failed") } : current)
    }
  }

  const isHealthy = health?.status === 'healthy'

  // The slicer exposes no export endpoint of its own, so the backend composes
  // the bundle from the profile list plus one read per profile. The file is the
  // only thing that can rebuild the slicer profiles if its volume is lost.
  const exportProfiles = async () => {
    setBusy('export')
    setError('')
    setSuccess('')
    try {
      const bundle = await slicerApi.exportProfiles()
      const stamp = new Date().toISOString().slice(0, 16).replace(/[-:T]/g, '')
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `picofarm-slicer-profiles-${stamp}.json`
      link.click()
      URL.revokeObjectURL(url)
      setSuccess(t("Exported {{count}} profiles (Orca {{version}}). Keep this file safe: it is the only backup of your slicer profiles.", { count: bundle.profiles.length, version: bundle.slicer_version || '—' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to export profiles"))
    } finally {
      setBusy('')
    }
  }

  // Reading and checking the file here means a broken bundle never costs a round
  // trip, and the operator sees what will happen before anything is uploaded.
  const selectRestoreFile = async (fileList?: FileList | null) => {
    const file = fileList?.[0]
    if (!file) return
    setBusy('restore')
    setError('')
    setSuccess('')
    try {
      const parsed = JSON.parse(await file.text()) as Partial<SlicerProfileBundle>
      if (!parsed || !Array.isArray(parsed.profiles) || parsed.profiles.length === 0) {
        throw new Error(t("This file is not a PicoFarm slicer profile bundle"))
      }
      for (const profile of parsed.profiles) {
        if (!profile || typeof profile.category !== 'string' || typeof profile.name !== 'string' || typeof profile.json !== 'string') {
          throw new Error(t("The bundle contains an invalid profile entry"))
        }
        JSON.parse(profile.json)
      }
      setRestoreModal({ bundle: parsed as SlicerProfileBundle, overwrite: false, status: 'idle', message: '' })
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to read the bundle file"))
    } finally {
      setBusy('')
    }
  }

  const runRestore = async () => {
    if (!restoreModal) return
    setRestoreModal({ ...restoreModal, status: 'restoring', message: t("Restoring...") })
    setError('')
    setSuccess('')
    try {
      const result = await slicerApi.importProfiles({
        profiles: restoreModal.bundle.profiles.map(profile => ({ category: profile.category, name: profile.name, json: profile.json })),
        overwrite: restoreModal.overwrite,
      })
      await loadProfiles(false)
      setRestoreModal(null)
      const summary = t("Restored {{restored}} profiles, skipped {{skipped}}, failed {{failed}}.", { restored: result.restored, skipped: result.skipped, failed: result.failed })
      const failures = result.profiles.filter(profile => profile.status === 'failed')
      if (result.failed > 0) {
        setError(`${summary} ${failures.map(profile => `${profile.category}/${profile.name}: ${profile.message || t("failed")}`).join('; ')}`)
      } else {
        setSuccess(summary)
      }
    } catch (err) {
      setRestoreModal(current => current ? { ...current, status: 'idle', message: err instanceof Error ? err.message : t("Restore failed") } : current)
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8 max-w-6xl mx-auto">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-display font-bold text-surface-100">{t("Slicer Settings")}</h1>
          <p className="text-surface-400 mt-1">{t("Connect PicoFarm to an external OrcaSlicer API container.")}</p>
        </div>
        <div className="flex gap-2">
          <button onClick={() => loadProfiles()} disabled={!!busy} className="btn btn-secondary shrink-0"><RefreshCw className="h-4 w-4 mr-2" /> {t("Load profiles")}</button>
          <button onClick={syncExternal} disabled={!!busy} className="btn btn-primary shrink-0"><DownloadCloud className="h-4 w-4 mr-2" /> {t("Sync external")}</button>
        </div>
      </div>

      {error && <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      {success && <div className="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-300">{success}</div>}

      <div className="card p-5 mb-6">
        <h2 className="text-lg font-semibold text-surface-100 mb-4 flex items-center gap-2"><Server className="h-5 w-5 text-accent-400" /> {t("Orca Slicer API Container")}</h2>
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_auto] gap-4 items-end">
          <label className="block">
            <span className="text-sm font-medium text-surface-300 mb-1 block">{t("Connection URL")}</span>
            <input type="text" value={connectionUrl} onChange={e => setConnectionUrl(e.target.value)} placeholder="http://localhost:3000" className="input" />
          </label>
          <div className="flex gap-2">
            <button onClick={saveConnection} disabled={!connectionUrl || !!busy} className="btn btn-primary"><Globe className="h-4 w-4 mr-2" /> {t("Save & Check")}</button>
            <button onClick={checkConnection} disabled={!connectionUrl || !!busy} className="btn btn-secondary">{t("Check")}</button>
          </div>
        </div>
        {connectionUrlSource === 'env' && (
          <div className="mt-4 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">
            {t('The server is using {{variable}}, so it overrides anything saved here. Saving still stores this URL for when that variable is removed, but it will not change which slicer is used until then.', { variable: 'SLICER_CONNECTION_URL' })}
          </div>
        )}
        <div className="mt-4 grid grid-cols-1 md:grid-cols-3 gap-3 text-sm">
          <HealthBadge label={t("API")} ok={!!health} value={isHealthy ? t("Healthy") : health ? t("Unhealthy") : t("Not checked")} />
          <HealthBadge label={t("Orca configured")} ok={health?.orcaSlicerConfigured === true} value={health?.orcaSlicerConfigured === true ? t("Yes") : health?.orcaSlicerConfigured === false ? t("No") : '—'} />
          <HealthBadge label={t("Orca executable")} ok={health?.orcaSlicerExecutable === true} value={health?.orcaSlicerExecutable === true ? t("Yes") : health?.orcaSlicerExecutable === false ? t("No") : '—'} />
        </div>
        {status && <pre className="mt-4 rounded-lg bg-surface-950 border border-surface-800 p-3 text-xs text-surface-400 overflow-auto">{JSON.stringify(status, null, 2)}</pre>}
        <div className="mt-4 text-xs text-surface-500">
          {t('Container: {{image}}. Expected endpoints: /health, /profiles/*, /slice.', { image: 'Brook-sys/orca-slicer-api' })}
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        {CATEGORIES.map(category => <ProfileSection key={category.key} category={category.key} title={t(category.title)} description={t(category.description)} items={profiles[category.key]} onAdd={() => setAddModal({ category: category.key, mode: 'url', name: '', url: '', json: '', overwrite: true, status: 'idle', message: '' })} onUpload={files => uploadProfileFiles(category.key, files)} onEdit={name => editProfile(category.key, name)} onSetDefault={name => setDefaultProfile(category.key, name)} uploading={busy === `upload-${category.key}` || busy === `default-${category.key}`} />)}
      </div>

      <div className="card p-5 mt-4">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div>
            <h3 className="font-semibold text-surface-100">{t("Profile backup")}</h3>
            <p className="text-xs text-surface-500 mt-1">
              {t("Downloads every profile of every category as one JSON file, and restores it after the slicer volume is lost or the container is recreated. The bundle keeps the raw profile JSON, so restoring writes the same bytes back. Export before changing profiles in bulk.")}
            </p>
          </div>
          <div className="flex gap-2 shrink-0">
            <button onClick={exportProfiles} disabled={!!busy || !health} className="btn btn-primary text-xs py-1.5 px-3"><Save className="h-3.5 w-3.5 mr-1" /> {t("Export backup")}</button>
            <label className={busy === 'restore' ? 'btn btn-secondary text-xs py-1.5 px-3 opacity-60 pointer-events-none' : 'btn btn-secondary text-xs py-1.5 px-3 cursor-pointer'}>
              <Upload className="h-3.5 w-3.5 mr-1" /> {t("Restore backup")}
              <input type="file" accept=".json,application/json" className="hidden" onChange={e => { selectRestoreFile(e.target.files); e.currentTarget.value = '' }} />
            </label>
          </div>
        </div>
        <p className="text-xs text-surface-500">
          {t("Restoring never overwrites a profile that already exists unless you ask it to. A skipped profile keeps working while a replaced one is gone. Default profile selections are not part of the bundle — set those again after a restore.")}
        </p>
      </div>

      {editModal && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"><div role="dialog" aria-modal="true" aria-labelledby="profile-editor-title" className="card w-full max-w-4xl p-6 max-h-[95vh] overflow-y-auto">
        <div className="flex items-start justify-between gap-4 mb-4"><div><h2 id="profile-editor-title" className="text-xl font-semibold text-surface-100">{t("Edit")} {editModal.name}</h2><p className="text-sm text-surface-500">{t("Changes are only sent when you click Save.")}</p></div><button onClick={closeEditor} disabled={editModal.status === 'saving' || editModal.status === 'validating'} className="btn btn-secondary">{t("Close")}</button></div>
        {editModal.message && <div role="status" className={editModal.status === 'error' ? 'mb-3 text-sm text-red-300' : 'mb-3 text-sm text-surface-300'}>{editModal.message}</div>}
        {editModal.status === 'loading' ? <div className="py-12 text-center text-surface-500">{t("Loading...")}</div> : <>
          <textarea aria-label={t("Profile JSON")} value={editModal.json} disabled={!editModal.currentHash || editModal.status === 'saving' || editModal.status === 'validating'} onChange={e => setEditModal(current => current ? { ...current, json: e.target.value, validation: null, status: 'idle', message: '' } : current)} className="input min-h-[40vh] font-mono text-xs" spellCheck={false} />
          {editModal.validation && <div className="my-3 text-sm"><ul className="text-red-300">{editModal.validation.errors.map((message, index) => <li key={index}>{message}</li>)}</ul><ul className="text-amber-200">{editModal.validation.warnings.map((message, index) => <li key={index}>{message}</li>)}</ul></div>}
          <div className="flex justify-end gap-2 my-3"><span className="text-xs text-surface-400 mr-auto">{editModal.json !== editModal.originalJson ? t("Unsaved changes") : t("No unsaved changes")}</span><button onClick={() => { if (editModal.json === editModal.originalJson || window.confirm(t("Discard draft and reload current slicer profile?"))) editProfile(editModal.category, editModal.name) }} disabled={editModal.status === 'saving' || editModal.status === 'validating'} className="btn btn-secondary">{t("Reload")}</button><button onClick={validateEditor} disabled={!editModal.currentHash || editModal.status === 'saving' || editModal.status === 'validating'} className="btn btn-secondary">{t("Validate")}</button><button onClick={saveEditor} disabled={!editModal.validation?.valid || !editModal.currentHash || editModal.json === editModal.originalJson || editModal.status === 'saving' || editModal.status === 'validating'} className="btn btn-primary"><Save className="h-4 w-4 mr-2" />{t("Save")}</button></div>
          <details className="border-t border-surface-800 pt-3"><summary className="text-sm text-surface-300 cursor-pointer">{t("Recovery snapshots ({{count}}, latest 50, up to 4 MiB)", { count: editModal.revisions.length })}</summary><p className="text-xs text-surface-500 my-2">{t("Snapshots include attempted saves, not only successful ones. Load into draft, validate and Save to create a new revision. Recovery covers this editor, not direct slicer edits or import/sync paths. Storage is capped at 64 MiB globally; when full, saves are refused without deleting snapshots. Export a full backup separately.")}</p><div className="space-y-2">{editModal.revisions.map(revision => <div key={revision.id} className="flex items-center justify-between gap-2 text-xs text-surface-400"><span>#{revision.id} · {new Date(revision.created_at).toLocaleString(i18n.resolvedLanguage)} · {revision.hash.slice(0, 12)}</span><button disabled={editModal.status === 'saving' || editModal.status === 'validating'} onClick={() => { if (editModal.json !== editModal.originalJson && !window.confirm(t("Replace unsaved draft with this snapshot?"))) return; setEditModal(current => current ? { ...current, json: JSON.stringify(JSON.parse(revision.json), null, 2), validation: null, status: 'idle', message: t("Snapshot loaded into draft; nothing sent to slicer.") } : current) }} className="btn btn-secondary text-xs">{t("Load into draft")}</button></div>)}</div></details>
        </>}
      </div></div>}

      {addModal && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"><div className="card w-full max-w-lg p-6"><h2 className="text-xl font-semibold text-surface-100 mb-1">{t("Import profile ({{category}})", { category: t(CATEGORIES.find(item => item.key === addModal.category)!.title) })}</h2><p className="text-sm text-surface-500 mb-4">{t("Import from a raw HTTPS URL or paste the Orca profile JSON directly.")}</p><div className="space-y-4"><div className="flex rounded-lg border border-surface-800 bg-surface-900/50 p-1"><button onClick={() => setAddModal({ ...addModal, mode: 'url' })} className={addModal.mode === 'url' ? 'btn btn-primary flex-1 text-xs' : 'btn btn-ghost flex-1 text-xs'}>{t("URL")}</button><button onClick={() => setAddModal({ ...addModal, mode: 'json' })} className={addModal.mode === 'json' ? 'btn btn-primary flex-1 text-xs' : 'btn btn-ghost flex-1 text-xs'}>{t("Raw JSON")}</button></div><label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("Profile name")} {addModal.mode === 'json' ? t("(optional, uses JSON name)") : ''}</span><input value={addModal.name} onChange={e => setAddModal({ ...addModal, name: e.target.value })} className="input" placeholder="standard-020" autoFocus /></label>{addModal.mode === 'url' ? <><label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("Raw JSON URL")}</span><input value={addModal.url} onChange={e => setAddModal({ ...addModal, url: e.target.value })} className="input" placeholder="https://raw.githubusercontent.com/.../profile.json" /></label><label className="flex items-center gap-2 text-sm text-surface-300"><input type="checkbox" checked={addModal.overwrite} onChange={e => setAddModal({ ...addModal, overwrite: e.target.checked })} />{t("Overwrite existing profile")}</label></> : <label className="block"><span className="text-xs text-surface-500 mb-1 block">{t("Profile JSON")}</span><textarea value={addModal.json} onChange={e => setAddModal({ ...addModal, json: e.target.value })} className="input min-h-56 font-mono text-xs" placeholder='{ "layer_height": "0.20" }' /></label>}{addModal.message && <div className="text-xs text-red-400">{addModal.message}</div>}</div><div className="mt-6 flex justify-end gap-2"><button onClick={() => setAddModal(null)} className="btn btn-secondary">{t("Cancel")}</button><button onClick={importProfile} disabled={(addModal.mode === 'url' && (!addModal.name.trim() || !addModal.url.trim())) || (addModal.mode === 'json' && !addModal.json.trim()) || addModal.status === 'importing'} className="btn btn-primary">{t("Import")}</button></div></div></div>}

      {restoreModal && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"><div className="card w-full max-w-lg p-6"><h2 className="text-xl font-semibold text-surface-100 mb-1">{t("Restore slicer profiles")}</h2><p className="text-sm text-surface-500 mb-4">{t("This uploads every profile in the bundle to the slicer. Nothing here can delete a profile, so the risk is replacing one you changed since the backup.")}</p><div className="space-y-2 text-sm text-surface-300"><div>{t("Profiles in the bundle:")} <span className="text-surface-100 font-medium">{restoreModal.bundle.profiles.length}</span>{restoreModal.bundle.exported_at ? ' · ' + t("Exported {{date}}", { date: new Date(restoreModal.bundle.exported_at).toLocaleString(i18n.resolvedLanguage) }) : ''}{restoreModal.bundle.slicer_version ? ` · Orca ${restoreModal.bundle.slicer_version}` : ''}</div><div className="text-xs text-surface-500">{CATEGORIES.map(category => `${restoreModal.bundle.profiles.filter(profile => profile.category === category.key).length} ${t(category.title).toLocaleLowerCase(i18n.resolvedLanguage)}`).join(' · ')}</div>{restoreModal.bundle.source ? <div className="text-xs text-surface-500 break-all">{t("Original slicer:")} <span className="font-mono">{restoreModal.bundle.source}</span></div> : null}<label className="flex items-center gap-2 text-sm text-surface-300 pt-2"><input type="checkbox" checked={restoreModal.overwrite} onChange={e => setRestoreModal(current => current ? { ...current, overwrite: e.target.checked } : current)} />{t("Overwrite profiles that already exist on this slicer")}</label></div>{restoreModal.message && <div className="mt-3 text-xs text-red-400">{restoreModal.message}</div>}<div className="mt-6 flex justify-end gap-2"><button onClick={() => setRestoreModal(null)} className="btn btn-secondary">{t("Cancel")}</button><button onClick={runRestore} disabled={restoreModal.status === 'restoring'} className="btn btn-primary">{restoreModal.status === 'restoring' ? t("Restoring...") : t("Restore")}</button></div></div></div>}
    </div>
  )
}

function HealthBadge({ label, ok, value }: { label: string; ok: boolean; value: string }) {

  return <div className="rounded-lg border border-surface-800 bg-surface-900/50 p-3"><div className="text-xs text-surface-500 mb-1">{label}</div><div className={ok ? 'text-emerald-400 flex items-center gap-1.5' : 'text-red-400 flex items-center gap-1.5'}>{ok ? <CheckCircle className="h-4 w-4" /> : <XCircle className="h-4 w-4" />}{value}</div></div>
}

function ProfileSection({ category, title, description, items, onAdd, onUpload, onEdit, onSetDefault, uploading }: { category: ProfileCategory; title: string; description: string; items: Array<Record<string, unknown>>; onAdd: () => void; onUpload: (files?: FileList | null) => void; onEdit: (name: string) => void; onSetDefault: (name: string) => void; uploading: boolean }) {
  const { t, i18n } = useTranslation('fleet')

  return <div className="card p-5"><div className="flex items-start justify-between gap-3 mb-4"><div><h3 className="font-semibold text-surface-100">{title}</h3><p className="text-xs text-surface-500 mt-1">{description}</p></div><div className="flex gap-2"><label className="btn btn-secondary text-xs py-1.5 px-3 cursor-pointer"><Upload className="h-3.5 w-3.5 mr-1" /> {uploading ? t("Uploading...") : t("Upload JSON")}<input type="file" accept=".json,application/json" multiple className="hidden" onChange={e => { onUpload(e.target.files); e.currentTarget.value = '' }} /></label><button onClick={onAdd} className="btn btn-secondary text-xs py-1.5 px-3"><Plus className="h-3.5 w-3.5 mr-1" /> {t("Import")}</button></div></div><div className="space-y-3">{items.map((item, index) => <div key={`${category}-${String(item.name)}-${index}`} className="rounded-xl border border-surface-800 bg-surface-900/60 p-3"><div className="flex items-start justify-between gap-2"><div className="min-w-0"><div className="flex items-center gap-2"><div className="font-medium text-surface-100 truncate">{String(item.name || t("Unnamed"))}</div>{item.default ? <span className="rounded-full border border-accent-500/40 bg-accent-500/10 px-2 py-0.5 text-[10px] text-accent-300">{t("Default")}</span> : null}</div><div className="mt-1 text-xs text-surface-500">{item.size ? t("{{size}} bytes", { size: String(item.size) }) : t("profile")}{item.updatedAt ? ` · ${new Date(String(item.updatedAt)).toLocaleString(i18n.resolvedLanguage)}` : ''}</div></div><div className="flex gap-2">{item.name && !item.default ? <button onClick={() => onSetDefault(String(item.name))} className="btn btn-secondary text-xs py-1.5 px-2">{t("Default")}</button> : null}{item.name ? <button onClick={() => onEdit(String(item.name))} className="btn btn-secondary text-xs py-1.5 px-2"><Pencil className="h-3.5 w-3.5 mr-1" /> {t("Edit")}</button> : null}</div></div>{typeof item.sourceUrl === 'string' && item.sourceUrl.length > 0 ? <a href={String(item.sourceUrl)} onClick={externalLinkClick} target="_blank" rel="noreferrer" className="mt-2 inline-flex items-center text-xs text-blue-300 hover:text-blue-200"><ExternalLink className="h-3 w-3 mr-1" /> {t("Source")}</a> : null}</div>)}{items.length === 0 && <div className="text-center py-6 text-surface-500 text-sm">{t("No profiles loaded. Import a profile or upload JSON.")}</div>}</div></div>
}
