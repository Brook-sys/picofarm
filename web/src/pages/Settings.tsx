import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Store, ShoppingBag, ExternalLink, RefreshCw, Unplug, AlertCircle, CheckCircle2, Key, Eye, EyeOff, Save, Database, Download, Trash2, RotateCcw, Plus, Palette, Settings as SettingsIcon, Building, Globe } from 'lucide-react'
import { etsyApi, squarespaceApi, settingsApi, backupsApi } from '../api/client'
import { cn } from '../lib/utils'
import { CURRENCY_OPTIONS, formatMoney, normalizeCurrencyCode } from '../lib/currency'
import {
  CANONICAL_MATERIAL_TYPES,
  DEFAULT_MATERIAL_COLORS,
  FILAMENT_TYPE_COLORS_SETTING_KEY,
  HEX_COLOR,
  UNKNOWN_MATERIAL_COLOR,
  normalizeMaterialTypeKey,
  parseMaterialColors,
} from '../lib/materialColors'
import { MaterialBadge } from '../components/MaterialBadge'
import type { EtsyIntegration, SquarespaceIntegration, BackupInfo, BackupConfig } from '../types'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

type SettingsTab = 'integrations' | 'api-keys' | 'backups' | 'other'

const OLX_INTEGRATION_ENABLED = false

export default function Settings() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [activeTab, setActiveTab] = useState<SettingsTab>('integrations')
  const [etsyStatus, setEtsyStatus] = useState<EtsyIntegration | null>(null)
  const [squarespaceStatus, setSquarespaceStatus] = useState<SquarespaceIntegration | null>(null)
  const [loading, setLoading] = useState(true)
  const [connecting, setConnecting] = useState(false)
  const [disconnecting, setDisconnecting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  // API Key settings
  const [anthropicKey, setAnthropicKey] = useState('')
  const [anthropicKeyMasked, setAnthropicKeyMasked] = useState('')
  const [showAnthropicKey, setShowAnthropicKey] = useState(false)
  const [savingKey, setSavingKey] = useState(false)
  const [keyLoaded, setKeyLoaded] = useState(false)

  // Squarespace API key
  const [squarespaceKey, setSquarespaceKey] = useState('')
  const [showSquarespaceKey, setShowSquarespaceKey] = useState(false)
  const [connectingSquarespace, setConnectingSquarespace] = useState(false)
  const [disconnectingSquarespace, setDisconnectingSquarespace] = useState(false)

  // OLX Brasil API key
  const [olxKey, setOlxKey] = useState('')
  const [olxKeyMasked, setOlxKeyMasked] = useState('')
  const [showOlxKey, setShowOlxKey] = useState(false)
  const [savingOlxKey, setSavingOlxKey] = useState(false)
  const [deletingOlxKey, setDeletingOlxKey] = useState(false)

  // Etsy Client ID configuration
  const [etsyClientId, setEtsyClientId] = useState('')
  const [configuringEtsy, setConfiguringEtsy] = useState(false)
  // Editable on an already-configured integration: before this existed, a wrong
  // keystring or redirect URI could only be fixed by hand-editing the database,
  // because the form was only rendered when !configured.
  const [etsyRedirectUri, setEtsyRedirectUri] = useState('')
  const [showEtsyCredentials, setShowEtsyCredentials] = useState(false)

  // Thingiverse API token
  const [thingiverseToken, setThingiverseToken] = useState('')
  const [thingiverseTokenMasked, setThingiverseTokenMasked] = useState('')
  const [showThingiverseToken, setShowThingiverseToken] = useState(false)
  const [savingThingiverseToken, setSavingThingiverseToken] = useState(false)

  // Backup settings
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [backupsLoading, setBackupsLoading] = useState(true)
  const [creatingBackup, setCreatingBackup] = useState(false)
  const [deletingBackup, setDeletingBackup] = useState<string | null>(null)
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null)

  // Check for OAuth callback results
  useEffect(() => {
    const etsyParam = searchParams.get('etsy')
    if (etsyParam === 'connected') {
      setSuccessMessage('Successfully connected to Etsy!')
      searchParams.delete('etsy')
      setSearchParams(searchParams, { replace: true })
    } else if (etsyParam === 'error') {
      const message = searchParams.get('message') || 'Connection failed'
      const description = searchParams.get('description')
      // Forwarded by the backend since the Etsy fix: Etsy answers a rejected
      // redirect URI with error_description, and internal failures carry a
      // bounded reason. Both used to be dropped, leaving only a bare code.
      const reason = searchParams.get('reason')
      let detail = message
      if (description) {
        detail = `${message} — ${description}`
      }
      if (reason) {
        detail = `${detail} (${reason})`
      }
      setError(`Etsy connection error: ${detail}`)
      searchParams.delete('etsy')
      searchParams.delete('message')
      searchParams.delete('description')
      searchParams.delete('reason')
      setSearchParams(searchParams, { replace: true })
    }
  }, [searchParams, setSearchParams])

  // Load Etsy status, Squarespace status, and API key settings
  useEffect(() => {
    loadEtsyStatus()
    loadSquarespaceStatus()
    loadApiKeys()
    loadBackups()
    loadThingiverseToken()
    loadOlxKey()
  }, [])

  const loadEtsyStatus = async () => {
    try {
      setLoading(true)
      const status = await etsyApi.getStatus()
      setEtsyStatus(status)
      // Keep the form in sync with what the backend actually sends to Etsy, so
      // an edit starts from the real value instead of an empty field.
      if (status.redirect_uri) {
        setEtsyRedirectUri(status.redirect_uri)
      }
    } catch (err) {
      console.error('Failed to load Etsy status:', err)
      setError('Failed to load Etsy integration status')
    } finally {
      setLoading(false)
    }
  }

  const loadSquarespaceStatus = async () => {
    try {
      const status = await squarespaceApi.getStatus()
      setSquarespaceStatus(status)
    } catch (err) {
      console.error('Failed to load Squarespace status:', err)
    }
  }

  const handleConnectSquarespace = async () => {
    if (!squarespaceKey.trim()) {
      setError('Please enter a Squarespace API key')
      return
    }

    try {
      setConnectingSquarespace(true)
      setError(null)
      const result = await squarespaceApi.connect(squarespaceKey.trim())
      setSquarespaceStatus(result)
      setSquarespaceKey('')
      setShowSquarespaceKey(false)
      setSuccessMessage('Successfully connected to Squarespace!')
    } catch (err) {
      console.error('Failed to connect Squarespace:', err)
      setError(err instanceof Error ? err.message : 'Failed to connect to Squarespace')
    } finally {
      setConnectingSquarespace(false)
    }
  }

  const handleDisconnectSquarespace = async () => {
    if (!confirm('Are you sure you want to disconnect your Squarespace store?')) {
      return
    }

    try {
      setDisconnectingSquarespace(true)
      setError(null)
      await squarespaceApi.disconnect()
      setSquarespaceStatus({ connected: false })
      setSuccessMessage('Squarespace store disconnected')
    } catch (err) {
      console.error('Failed to disconnect Squarespace:', err)
      setError(err instanceof Error ? err.message : 'Failed to disconnect Squarespace')
    } finally {
      setDisconnectingSquarespace(false)
    }
  }

  const loadApiKeys = async () => {
    try {
      const setting = await settingsApi.get('anthropic_api_key')
      setAnthropicKeyMasked(setting.value)
      setKeyLoaded(true)
    } catch {
      // Key not set yet — that's fine
      setKeyLoaded(true)
    }
  }

  const loadBackups = async () => {
    try {
      setBackupsLoading(true)
      const list = await backupsApi.list()
      setBackups(list || [])
    } catch (err) {
      console.error('Failed to load backups:', err)
    } finally {
      setBackupsLoading(false)
    }
  }

  const loadThingiverseToken = async () => {
    try {
      const res = await settingsApi.get('thingiverse_api_token')
      if (res.value) {
        setThingiverseTokenMasked('********')
      }
    } catch {
      // not set yet
    }
  }

  const loadOlxKey = async () => {
    try {
      const res = await settingsApi.get('olx_api_key')
      if (res.value) {
        setOlxKeyMasked(res.value)
      }
    } catch {
      // OLX key not configured yet
    }
  }

  const handleSaveAnthropicKey = async () => {
    if (!anthropicKey.trim()) return

    try {
      setSavingKey(true)
      setError(null)
      await settingsApi.set('anthropic_api_key', anthropicKey.trim())
      setSuccessMessage('Anthropic API key saved')
      setAnthropicKeyMasked(
        anthropicKey.trim().length > 8
          ? anthropicKey.trim().slice(0, 4) + '...' + anthropicKey.trim().slice(-4)
          : '****'
      )
      setAnthropicKey('')
      setShowAnthropicKey(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save API key')
    } finally {
      setSavingKey(false)
    }
  }

  const handleSaveThingiverseToken = async () => {
    if (!thingiverseToken.trim()) return

    try {
      setSavingThingiverseToken(true)
      setError(null)
      await settingsApi.set('thingiverse_api_token', thingiverseToken.trim())
      setSuccessMessage('Thingiverse API token saved')
      setThingiverseTokenMasked('********')
      setThingiverseToken('')
      setShowThingiverseToken(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save Thingiverse token')
    } finally {
      setSavingThingiverseToken(false)
    }
  }

  const handleSaveOlxKey = async () => {
    if (!olxKey.trim()) return

    try {
      setSavingOlxKey(true)
      setError(null)
      const trimmed = olxKey.trim()
      await settingsApi.set('olx_api_key', trimmed)
      setSuccessMessage('OLX Brasil API key saved')
      setOlxKeyMasked(trimmed.length > 8 ? `${trimmed.slice(0, 4)}...${trimmed.slice(-4)}` : '****')
      setOlxKey('')
      setShowOlxKey(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save OLX API key')
    } finally {
      setSavingOlxKey(false)
    }
  }

  const handleDeleteOlxKey = async () => {
    if (!confirm('Remove the OLX Brasil API key?')) return

    try {
      setDeletingOlxKey(true)
      setError(null)
      await settingsApi.delete('olx_api_key')
      setOlxKey('')
      setOlxKeyMasked('')
      setSuccessMessage('OLX Brasil API key removed')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove OLX API key')
    } finally {
      setDeletingOlxKey(false)
    }
  }

  const handleConnectEtsy = async () => {
    try {
      setConnecting(true)
      setError(null)
      const { url } = await etsyApi.getAuthUrl()
      // Redirect to Etsy OAuth page
      window.location.href = url
    } catch (err) {
      console.error('Failed to start Etsy OAuth:', err)
      setError(err instanceof Error ? err.message : 'Failed to start Etsy connection')
      setConnecting(false)
    }
  }

  const handleDisconnectEtsy = async () => {
    if (!confirm('Are you sure you want to disconnect your Etsy shop?')) {
      return
    }

    try {
      setDisconnecting(true)
      setError(null)
      await etsyApi.disconnect()
      setEtsyStatus({ connected: false, configured: etsyStatus?.configured || false })
      setSuccessMessage('Etsy shop disconnected')
    } catch (err) {
      console.error('Failed to disconnect Etsy:', err)
      setError(err instanceof Error ? err.message : 'Failed to disconnect Etsy')
    } finally {
      setDisconnecting(false)
    }
  }

  const handleConfigureEtsy = async () => {
    setConfiguringEtsy(true)
    setError(null)
    try {
      const wasConfigured = etsyStatus?.configured === true
      await etsyApi.configure({
        client_id: etsyClientId,
        // Omitted when blank so the backend applies its own default (port 8084).
        ...(etsyRedirectUri.trim() ? { redirect_uri: etsyRedirectUri.trim() } : {}),
      })
      const status = await etsyApi.getStatus()
      setEtsyStatus(status)
      if (status.redirect_uri) {
        setEtsyRedirectUri(status.redirect_uri)
      }
      setShowEtsyCredentials(false)
      if (status.configured && !wasConfigured) {
        // First-time setup: continue straight into the OAuth flow.
        handleConnectEtsy()
      } else if (status.configured) {
        // Reconfigured: an existing token stays valid, and the operator can now
        // retry the connection with the corrected redirect URI.
        setSuccessMessage('Etsy credentials updated')
      }
    } catch (err) {
      console.error('Failed to configure Etsy:', err)
      setError(err instanceof Error ? err.message : 'Failed to configure Etsy')
    } finally {
      setConfiguringEtsy(false)
    }
  }

  const handleCreateBackup = async () => {
    try {
      setCreatingBackup(true)
      setError(null)
      const backup = await backupsApi.create()
      setBackups(prev => [backup, ...prev])
      setSuccessMessage('Backup created successfully')
    } catch (err) {
      console.error('Failed to create backup:', err)
      setError(err instanceof Error ? err.message : 'Failed to create backup')
    } finally {
      setCreatingBackup(false)
    }
  }

  const handleDeleteBackup = async (name: string) => {
    if (!confirm(`Are you sure you want to delete backup "${name}"?`)) {
      return
    }

    try {
      setDeletingBackup(name)
      setError(null)
      await backupsApi.delete(name)
      setBackups(prev => prev.filter(b => b.name !== name))
      setSuccessMessage('Backup deleted')
    } catch (err) {
      console.error('Failed to delete backup:', err)
      setError(err instanceof Error ? err.message : 'Failed to delete backup')
    } finally {
      setDeletingBackup(null)
    }
  }

  const handleRestoreBackup = async (name: string) => {
    if (!confirm(`Are you sure you want to restore from "${name}"?\n\nThis will replace your current database. The application will need to be restarted.`)) {
      return
    }

    try {
      setRestoringBackup(name)
      setError(null)
      const result = await backupsApi.restore(name)
      setSuccessMessage(result.message + ' Please restart the application.')
    } catch (err) {
      console.error('Failed to restore backup:', err)
      setError(err instanceof Error ? err.message : 'Failed to restore backup')
    } finally {
      setRestoringBackup(null)
    }
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'Never'
    return new Date(dateStr).toLocaleString()
  }

  const isTokenExpiringSoon = () => {
    if (!etsyStatus?.token_expires_at) return false
    const expiresAt = new Date(etsyStatus.token_expires_at)
    const hoursUntilExpiry = (expiresAt.getTime() - Date.now()) / (1000 * 60 * 60)
    return hoursUntilExpiry < 24
  }

  const tabs: { id: SettingsTab; label: string; icon: React.ReactNode }[] = [
    { id: 'integrations', label: 'Integrations', icon: <Store className="h-4 w-4" /> },
    { id: 'api-keys', label: 'API Keys', icon: <Key className="h-4 w-4" /> },
    { id: 'backups', label: 'Backups', icon: <Database className="h-4 w-4" /> },
    { id: 'other', label: 'Other', icon: <SettingsIcon className="h-4 w-4" /> },
  ]

  return (
    <div className="p-6">
      <div className="max-w-4xl">
        <h1 className="text-2xl font-display font-semibold text-surface-100 mb-6">
          Settings
        </h1>

        {/* Tab Navigation */}
        <div className="flex gap-1 mb-6 border-b border-surface-800">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                'flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-t-lg transition-colors',
                activeTab === tab.id
                  ? 'bg-surface-900/50 text-surface-100 border-b-2 border-accent-500'
                  : 'text-surface-400 hover:text-surface-200 hover:bg-surface-900/30'
              )}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </div>

        {/* Success Message */}
        {successMessage && (
          <div className="mb-6 p-4 bg-green-500/10 border border-green-500/30 rounded-lg flex items-center gap-3">
            <CheckCircle2 className="h-5 w-5 text-green-400 flex-shrink-0" />
            <p className="text-green-300">{successMessage}</p>
            <button
              onClick={() => setSuccessMessage(null)}
              className="ml-auto text-green-400 hover:text-green-300"
            >
              &times;
            </button>
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div className="mb-6 p-4 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-3">
            <AlertCircle className="h-5 w-5 text-red-400 flex-shrink-0" />
            <p className="text-red-300">{error}</p>
            <button
              onClick={() => setError(null)}
              className="ml-auto text-red-400 hover:text-red-300"
            >
              &times;
            </button>
          </div>
        )}

        {/* Database Backups Card */}
        {activeTab === 'backups' && (
        <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6 mb-6">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-500/10 rounded-lg">
                <Database className="h-6 w-6 text-blue-400" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-surface-100">Database Backups</h2>
                <p className="text-sm text-surface-400">
                  Create and restore database backups
                </p>
              </div>
            </div>
            <button
              onClick={handleCreateBackup}
              disabled={creatingBackup}
              className="flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 text-sm"
            >
              {creatingBackup ? (
                <RefreshCw className="h-4 w-4 animate-spin" />
              ) : (
                <Plus className="h-4 w-4" />
              )}
              Create Backup
            </button>
          </div>

          {backupsLoading ? (
            <div className="flex items-center justify-center py-8">
              <RefreshCw className="h-6 w-6 text-surface-400 animate-spin" />
            </div>
          ) : backups.length === 0 ? (
            <div className="bg-surface-800/50 rounded-lg p-4 text-center">
              <p className="text-surface-400">No backups yet. Create your first backup to protect your data.</p>
            </div>
          ) : (
            <div className="space-y-2">
              {backups.map((backup) => (
                <div
                  key={backup.name}
                  className="flex items-center justify-between bg-surface-800/50 rounded-lg p-3"
                >
                  <div className="flex items-center gap-3">
                    <Download className="h-4 w-4 text-surface-400" />
                    <div>
                      <p className="text-sm font-medium text-surface-200">{backup.name}</p>
                      <p className="text-xs text-surface-500">
                        {formatDate(backup.created_at)} · {formatBytes(backup.size)}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleRestoreBackup(backup.name)}
                      disabled={restoringBackup === backup.name}
                      className="flex items-center gap-1 px-2 py-1 text-xs font-medium text-amber-400 hover:text-amber-300 hover:bg-amber-500/10 rounded transition-colors disabled:opacity-50"
                      title="Restore this backup"
                    >
                      {restoringBackup === backup.name ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <RotateCcw className="h-3 w-3" />
                      )}
                      Restore
                    </button>
                    <button
                      onClick={() => handleDeleteBackup(backup.name)}
                      disabled={deletingBackup === backup.name}
                      className="flex items-center gap-1 px-2 py-1 text-xs font-medium text-red-400 hover:text-red-300 hover:bg-red-500/10 rounded transition-colors disabled:opacity-50"
                      title="Delete this backup"
                    >
                      {deletingBackup === backup.name ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <Trash2 className="h-3 w-3" />
                      )}
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Auto-Backup Settings */}
          <BackupSettings />

          <p className="mt-4 text-xs text-surface-500">
            Backups are stored locally in your data directory. Consider copying important backups to external storage.
          </p>
        </div>
        )}

        {/* API Keys Card */}
        {activeTab === 'api-keys' && (
        <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6 mb-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-purple-500/10 rounded-lg">
              <Key className="h-6 w-6 text-purple-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-surface-100">API Keys</h2>
              <p className="text-sm text-surface-400">
                Configure API keys for AI-powered features
              </p>
            </div>
          </div>

          <div className="space-y-4">
            {/* Anthropic API Key */}
            <div>
              <label className="block text-sm font-medium text-surface-300 mb-2">
                Anthropic API Key
              </label>
              <p className="text-xs text-surface-500 mb-2">
                Required for AI receipt parsing. Get your key at{' '}
                <a
                  href="https://console.anthropic.com/settings/keys"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-accent-400 hover:underline"
                >
                  console.anthropic.com
                </a>
              </p>

              {keyLoaded && anthropicKeyMasked && !anthropicKey && (
                <div className="flex items-center gap-2 mb-2">
                  <CheckCircle2 className="h-4 w-4 text-green-400" />
                  <span className="text-sm text-green-300">Key configured: {anthropicKeyMasked}</span>
                </div>
              )}

              <div className="flex gap-2">
                <div className="relative flex-1">
                  <input
                    type={showAnthropicKey ? 'text' : 'password'}
                    value={anthropicKey}
                    onChange={(e) => setAnthropicKey(e.target.value)}
                    placeholder={anthropicKeyMasked ? 'Enter new key to update...' : 'sk-ant-...'}
                    className="w-full bg-surface-800 border border-surface-700 rounded-lg px-3 py-2 text-sm text-surface-100 placeholder-surface-500 focus:outline-none focus:ring-2 focus:ring-accent-500 focus:border-transparent pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowAnthropicKey(!showAnthropicKey)}
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-400 hover:text-surface-300"
                  >
                    {showAnthropicKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <button
                  onClick={handleSaveAnthropicKey}
                  disabled={!anthropicKey.trim() || savingKey}
                  className="flex items-center gap-2 px-4 py-2 bg-accent-600 hover:bg-accent-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                >
                  {savingKey ? (
                    <RefreshCw className="h-4 w-4 animate-spin" />
                  ) : (
                    <Save className="h-4 w-4" />
                  )}
                  Save
                </button>
               </div>
             </div>
           </div>
         </div>
         )}

         {/* Thingiverse API Token */}
         {activeTab === 'api-keys' && (
         <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6">
           <div className="flex items-center gap-3 mb-4">
             <div className="p-2 bg-blue-500/10 rounded-lg">
               <Globe className="h-6 w-6 text-blue-400" />
             </div>
             <div>
               <h2 className="text-lg font-semibold text-surface-100">Thingiverse Integration</h2>
               <p className="text-sm text-surface-400">
                 Import models from Thingiverse using your Personal Access Token
               </p>
             </div>
           </div>

           <div className="space-y-4">
             <div>
               <label className="block text-sm font-medium text-surface-300 mb-2">
                 Thingiverse API Token
               </label>
               <div className="flex gap-2">
                 <div className="relative flex-1">
                   <input
                     type={showThingiverseToken ? 'text' : 'password'}
                     value={thingiverseToken}
                     onChange={(e) => setThingiverseToken(e.target.value)}
                     placeholder={thingiverseTokenMasked ? 'Enter new token to update...' : 'Paste your Thingiverse token'}
                     className="input pr-10 w-full"
                   />
                   <button
                     type="button"
                     onClick={() => setShowThingiverseToken(!showThingiverseToken)}
                     className="absolute right-3 top-1/2 -translate-y-1/2 text-surface-400 hover:text-surface-200"
                   >
                     {showThingiverseToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                   </button>
                 </div>
                 <button
                   onClick={handleSaveThingiverseToken}
                   disabled={!thingiverseToken.trim() || savingThingiverseToken}
                   className="flex items-center gap-2 px-4 py-2 bg-accent-600 hover:bg-accent-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                 >
                   {savingThingiverseToken ? (
                     <RefreshCw className="h-4 w-4 animate-spin" />
                   ) : (
                     <Save className="h-4 w-4" />
                   )}
                   Save
                 </button>
               </div>
               {thingiverseTokenMasked && !thingiverseToken && (
                 <div className="mt-2">
                   <span className="text-sm text-green-300">Token configured: {thingiverseTokenMasked}</span>
                 </div>
               )}
               <p className="mt-2 text-xs text-surface-500">
                 Get your token at <a href="https://www.thingiverse.com/settings/personal-access-tokens" target="_blank" rel="noreferrer" className="text-accent-400 hover:underline">Thingiverse Settings → Personal Access Tokens</a>
               </p>
              </div>
            </div>
          </div>
          )}
 
           {/* Etsy Integration Card */}
         {activeTab === 'integrations' && (
         <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-orange-500/10 rounded-lg">
              <Store className="h-6 w-6 text-orange-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-surface-100">Etsy Integration</h2>
              <p className="text-sm text-surface-400">
                Connect your Etsy shop to sync orders automatically
              </p>
            </div>
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <RefreshCw className="h-6 w-6 text-surface-400 animate-spin" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* The credentials form is no longer reachable only on a fresh
                  install: a keystring or redirect URI that is wrong must be
                  replaceable, otherwise Etsy stays stuck at "configured but not
                  connected" until someone edits the database. */}
              {showEtsyCredentials || !etsyStatus?.configured ? (
                <div className="bg-surface-800/50 rounded-lg p-4">
                  <p className="text-surface-300 mb-3">
                    Connect your Etsy shop to sync orders and listings.
                  </p>
                  <div className="space-y-3">
                    <div>
                      <label className="block text-sm text-surface-400 mb-1">Etsy Client ID</label>
                      <input
                        type="text"
                        value={etsyClientId}
                        onChange={e => setEtsyClientId(e.target.value)}
                        placeholder="Paste your Etsy app's Client ID (Keystring)"
                        className="input w-full"
                      />
                      <p className="text-xs text-surface-500 mt-1">
                        Get this from the <a href="https://www.etsy.com/developers/your-apps" target="_blank" rel="noopener noreferrer" className="text-accent-400 hover:underline">Etsy Developer Portal</a>
                      </p>
                    </div>
                    <div>
                      <label className="block text-sm text-surface-400 mb-1">Redirect URI</label>
                      <input
                        type="text"
                        value={etsyRedirectUri}
                        onChange={e => setEtsyRedirectUri(e.target.value)}
                        placeholder="http://localhost:8084/api/integrations/etsy/callback"
                        className="input w-full font-mono text-sm"
                      />
                      <p className="text-xs text-surface-500 mt-1">
                        Must be registered as a Callback URL on your app in the <a href="https://www.etsy.com/developers/your-apps" target="_blank" rel="noopener noreferrer" className="text-accent-400 hover:underline">Etsy Developer Portal</a>.
                        Etsy rejects an unregistered value with "The requested redirect URL is not permitted.".
                        It must also be reachable in your browser when Etsy sends it back. Leave it empty to use the default.
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={handleConfigureEtsy}
                        disabled={!etsyClientId.trim() || configuringEtsy}
                        className="btn btn-primary"
                      >
                        {configuringEtsy ? 'Saving...' : 'Save & Connect'}
                      </button>
                      {etsyStatus?.configured && (
                        <button
                          onClick={() => setShowEtsyCredentials(false)}
                          className="btn btn-ghost"
                        >
                          Cancel
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ) : etsyStatus.connected ? (
                <div className="space-y-4">
                  {/* Connected Shop Info */}
                  <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <CheckCircle2 className="h-4 w-4 text-green-400" />
                      <span className="text-green-300 font-medium">Connected</span>
                    </div>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <p className="text-surface-400">Shop Name</p>
                        <p className="text-surface-100 font-medium">{etsyStatus.shop_name}</p>
                      </div>
                      <div>
                        <p className="text-surface-400">Shop ID</p>
                        <p className="text-surface-100 font-medium">{etsyStatus.shop_id}</p>
                      </div>
                      <div>
                        <p className="text-surface-400">Last Synced</p>
                        <p className="text-surface-100">{formatDate(etsyStatus.last_sync_at)}</p>
                      </div>
                      <div>
                        <p className="text-surface-400">Connected Since</p>
                        <p className="text-surface-100">{formatDate(etsyStatus.created_at)}</p>
                      </div>
                    </div>
                  </div>

                  {/* Registered Redirect URI */}
                  <div className="bg-surface-800/50 rounded-lg p-3">
                    <p className="text-sm text-surface-400 mb-1">Redirect URI</p>
                    <p className="text-xs text-surface-200 font-mono break-all">
                      {etsyStatus.redirect_uri || 'not set'}
                    </p>
                    <p className="text-xs text-surface-500 mt-1">
                      Etsy only accepts addresses registered on your app. A different address means every reconnect fails.
                    </p>
                  </div>

                  {/* Token Expiry Warning */}
                  {isTokenExpiringSoon() && (
                    <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-3 flex items-center gap-2">
                      <AlertCircle className="h-4 w-4 text-yellow-400" />
                      <span className="text-sm text-yellow-300">
                        Token expires soon. It will be automatically refreshed on next API call.
                      </span>
                    </div>
                  )}

                  {/* Scopes */}
                  <div>
                    <p className="text-sm text-surface-400 mb-2">Permissions</p>
                    <div className="flex flex-wrap gap-2">
                      {etsyStatus.scopes?.map((scope) => (
                        <span
                          key={scope}
                          className="text-xs bg-surface-700 text-surface-300 px-2 py-1 rounded"
                        >
                          {scope}
                        </span>
                      ))}
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => {
                        setEtsyClientId('')
                        setShowEtsyCredentials(true)
                      }}
                      className="px-4 py-2 text-sm font-medium text-surface-300 hover:text-surface-100 hover:bg-surface-700 rounded-lg transition-colors"
                    >
                      Edit credentials
                    </button>
                    <button
                      onClick={handleDisconnectEtsy}
                      disabled={disconnecting}
                      className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-400 hover:text-red-300 hover:bg-red-500/10 rounded-lg transition-colors disabled:opacity-50"
                    >
                      {disconnecting ? (
                        <RefreshCw className="h-4 w-4 animate-spin" />
                      ) : (
                        <Unplug className="h-4 w-4" />
                      )}
                      Disconnect Shop
                    </button>
                  </div>
                </div>
              ) : (
                <div className="space-y-4">
                  <p className="text-surface-300">
                    Connect your Etsy shop to automatically import orders and sync inventory.
                  </p>
                  <div className="bg-surface-800/50 rounded-lg p-3">
                    <p className="text-sm text-surface-400 mb-1">Redirect URI</p>
                    <p className="text-xs text-surface-200 font-mono break-all">
                      {etsyStatus.redirect_uri || 'not set'}
                    </p>
                    <p className="text-xs text-surface-500 mt-1">
                      {etsyStatus.redirect_uri
                        ? 'This is the address Etsy is told to return to. It must be registered as a Callback URL on your app.'
                        : 'Using the built-in default. It must be registered as a Callback URL on your app in the Etsy Developer Portal.'}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={handleConnectEtsy}
                      disabled={connecting}
                      className="flex items-center gap-2 px-4 py-2 bg-orange-500 hover:bg-orange-600 text-white font-medium rounded-lg transition-colors disabled:opacity-50"
                    >
                      {connecting ? (
                        <RefreshCw className="h-4 w-4 animate-spin" />
                      ) : (
                        <ExternalLink className="h-4 w-4" />
                      )}
                      Connect to Etsy
                    </button>
                    <button
                      onClick={() => {
                        setEtsyClientId('')
                        setShowEtsyCredentials(true)
                      }}
                      className="px-4 py-2 text-sm font-medium text-surface-300 hover:text-surface-100 hover:bg-surface-700 rounded-lg transition-colors"
                    >
                      Edit credentials
                    </button>
                  </div>
                </div>
               )}
              </div>
          )}
         </div>
         )}

         {/* Squarespace Integration Card */}
         {activeTab === 'integrations' && (
         <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-purple-500/10 rounded-lg">
              <ShoppingBag className="h-6 w-6 text-purple-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-surface-100">Squarespace Integration</h2>
              <p className="text-sm text-surface-400">
                Connect your Squarespace store to sync orders automatically
              </p>
            </div>
          </div>

          {squarespaceStatus?.connected ? (
            <div className="space-y-4">
              {/* Connected Store Info */}
              <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-4">
                <div className="flex items-center gap-2 mb-2">
                  <CheckCircle2 className="h-4 w-4 text-green-400" />
                  <span className="text-green-300 font-medium">Connected</span>
                </div>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-surface-400">Site Title</p>
                    <p className="text-surface-100 font-medium">{squarespaceStatus.site_title}</p>
                  </div>
                  <div>
                    <p className="text-surface-400">Site ID</p>
                    <p className="text-surface-100 font-medium">{squarespaceStatus.site_id}</p>
                  </div>
                  <div>
                    <p className="text-surface-400">Last Order Sync</p>
                    <p className="text-surface-100">{formatDate(squarespaceStatus.last_order_sync_at)}</p>
                  </div>
                  <div>
                    <p className="text-surface-400">Connected Since</p>
                    <p className="text-surface-100">{formatDate(squarespaceStatus.created_at)}</p>
                  </div>
                </div>
              </div>

              {/* Disconnect Button */}
              <button
                onClick={handleDisconnectSquarespace}
                disabled={disconnectingSquarespace}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-400 hover:text-red-300 hover:bg-red-500/10 rounded-lg transition-colors disabled:opacity-50"
              >
                {disconnectingSquarespace ? (
                  <RefreshCw className="h-4 w-4 animate-spin" />
                ) : (
                  <Unplug className="h-4 w-4" />
                )}
                Disconnect Store
                </button>
                </div>
                ) : (
                <div className="space-y-4">
                <p className="text-surface-300">
                Connect your Squarespace store to automatically import orders.
                </p>
                <p className="text-xs text-surface-500">
                Get your API key from{' '}
                <a
                  href="https://support.squarespace.com/hc/en-us/articles/12880553888141-Commerce-APIs"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-accent-400 hover:underline"
                >
                  Squarespace Developer Settings
                </a>
                . You'll need the <code className="bg-surface-700 px-1 rounded">Orders Read</code> and{' '}
                <code className="bg-surface-700 px-1 rounded">Products Read</code> permissions.
                </p>

                <div className="flex gap-2">
                <div className="relative flex-1">
                  <input
                    type={showSquarespaceKey ? 'text' : 'password'}
                    value={squarespaceKey}
                    onChange={(e) => setSquarespaceKey(e.target.value)}
                    placeholder="Enter your Squarespace API key..."
                    className="w-full bg-surface-800 border border-surface-700 rounded-lg px-3 py-2 text-sm text-surface-100 placeholder-surface-500 focus:outline-none focus:ring-2 focus:ring-accent-500 focus:border-transparent pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowSquarespaceKey(!showSquarespaceKey)}
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-400 hover:text-surface-300"
                  >
                    {showSquarespaceKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <button
                  onClick={handleConnectSquarespace}
                  disabled={!squarespaceKey.trim() || connectingSquarespace}
                  className="flex items-center gap-2 px-4 py-2 bg-purple-500 hover:bg-purple-600 text-white font-medium rounded-lg transition-colors disabled:opacity-50"
                >
                  {connectingSquarespace ? (
                    <RefreshCw className="h-4 w-4 animate-spin" />
                  ) : (
                    <ExternalLink className="h-4 w-4" />
                  )}
                  Connect
                </button>
                </div>
                </div>
                )}
                </div>
                )}

                {/* OLX Brasil Integration Card intentionally hidden until official partner/API access is practical. */}
                {OLX_INTEGRATION_ENABLED && activeTab === 'integrations' && (
                <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6">
                <div className="flex items-center gap-3 mb-4">
                <div className="p-2 bg-emerald-500/10 rounded-lg">
                <Globe className="h-6 w-6 text-emerald-400" />
                </div>
                <div>
                <h2 className="text-lg font-semibold text-surface-100">OLX Brasil Integration</h2>
                <p className="text-sm text-surface-400">
                Configure your OLX API key for classifieds and lead sync
                </p>
                </div>
                </div>

                <div className="space-y-4">
                <p className="text-xs text-surface-500">
                OLX API access requires integrator registration. Store your API key here; status will reflect validation via the OLX client.
                </p>

                <div>
                <label className="block text-sm font-medium text-surface-300 mb-2">OLX API Key</label>
                {olxKeyMasked && !olxKey && (
                <div className="flex items-center gap-2 mb-2">
                  <CheckCircle2 className="h-4 w-4 text-green-400" />
                  <span className="text-sm text-green-300">Key configured: {olxKeyMasked}</span>
                </div>
                )}
                <div className="flex gap-2">
                <div className="relative flex-1">
                  <input
                    type={showOlxKey ? 'text' : 'password'}
                    value={olxKey}
                    onChange={(e) => setOlxKey(e.target.value)}
                    placeholder={olxKeyMasked ? 'Enter new key to update...' : 'Paste your OLX API key'}
                    className="w-full bg-surface-800 border border-surface-700 rounded-lg px-3 py-2 text-sm text-surface-100 placeholder-surface-500 focus:outline-none focus:ring-2 focus:ring-accent-500 focus:border-transparent pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowOlxKey(!showOlxKey)}
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-400 hover:text-surface-300"
                  >
                    {showOlxKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <button
                  onClick={handleSaveOlxKey}
                  disabled={!olxKey.trim() || savingOlxKey}
                  className="flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                >
                  {savingOlxKey ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                  Save
                </button>
                {olxKeyMasked && (
                  <button
                    onClick={handleDeleteOlxKey}
                    disabled={deletingOlxKey}
                    className="flex items-center gap-2 px-3 py-2 text-sm text-red-400 hover:text-red-300 hover:bg-red-500/10 rounded-lg transition-colors disabled:opacity-50"
                  >
                    {deletingOlxKey ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                    Remove
                  </button>
                )}
                </div>
                </div>
                </div>
                </div>
                )}

                {/* General display and business information */}
                {activeTab === 'other' && (
                  <>
                    <CurrencySettings />
                    <FilamentTypeColorSettings />
                    <BusinessInfoSettings />
                  </>
                )}

         <CreditsFooter />
      </div>
    </div>
  )
}

function CreditsFooter() {
  return (
    <div className="mt-8 pt-6 border-t border-surface-800">
      <h2 className="text-sm font-semibold text-surface-300">Credits</h2>
      <p className="text-xs text-surface-500 mt-2">
        PicoFarm is a fork evolved from{' '}
        <a href="https://github.com/philjestin/Daedalus" target="_blank" rel="noopener noreferrer" className="text-accent-400 hover:underline">
          Daedalus
        </a>
        , created by the original Daedalus project contributors.
      </p>
      <p className="text-xs text-surface-600 mt-2">
        We keep this credit here to acknowledge the foundation inherited from Daedalus while PicoFarm moves toward a server/container-first print farm platform.
      </p>
    </div>
  )
}

function CurrencySettings() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['settings', 'currency_code'],
    queryFn: () => settingsApi.get('currency_code'),
    retry: false,
  })
  const [currencyDraft, setCurrencyDraft] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const currency = currencyDraft ?? normalizeCurrencyCode(data?.value)

  const saveCurrency = useMutation({
    mutationFn: (value: string) => settingsApi.set('currency_code', value),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['settings', 'currency_code'] })
      setCurrencyDraft(null)
      setSaved(true)
      window.setTimeout(() => setSaved(false), 2500)
    },
  })

  return (
    <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6 mb-6">
      <div className="flex items-start gap-3 mb-5">
        <div className="p-2 bg-emerald-500/10 rounded-lg">
          <Globe className="h-6 w-6 text-emerald-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-surface-100">Currency</h2>
          <p className="text-sm text-surface-400">
            Choose how PicoFarm displays costs, revenue, profit, quotes, and other monetary values. Existing amounts are not converted.
          </p>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
        <label className="block">
          <span className="block text-sm font-medium text-surface-300 mb-2">Display currency</span>
          <select
            value={currency}
            onChange={event => setCurrencyDraft(event.target.value)}
            disabled={isLoading || saveCurrency.isPending}
            className="input w-full"
          >
            {CURRENCY_OPTIONS.map(option => (
              <option key={option.code} value={option.code}>{option.code} — {option.label}</option>
            ))}
          </select>
          <span className="mt-2 block text-xs text-surface-500">
            Preview: {formatMoney(1234.56, currency)}
          </span>
        </label>
        <button
          type="button"
          onClick={() => saveCurrency.mutate(currency)}
          disabled={isLoading || saveCurrency.isPending}
          className="btn btn-primary min-w-28"
        >
          {saveCurrency.isPending ? 'Saving...' : saved ? 'Saved' : 'Save'}
        </button>
      </div>
      {saveCurrency.isError && (
        <p className="mt-3 text-sm text-red-400">
          {saveCurrency.error instanceof Error ? saveCurrency.error.message : 'Failed to save currency'}
        </p>
      )}
    </div>
  )
}

/**
 * Filament label colors. The Files screen shows one colored chip per G-code
 * material type; the colors live in the generic `filament_type_colors` setting
 * so they survive backups like any other configuration.
 */
function FilamentTypeColorSettings() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['settings', FILAMENT_TYPE_COLORS_SETTING_KEY],
    queryFn: () => settingsApi.get(FILAMENT_TYPE_COLORS_SETTING_KEY),
    retry: false,
  })
  const stored = useMemo(() => parseMaterialColors(data?.value), [data])
  const [draft, setDraft] = useState<Record<string, string> | null>(null)
  const [newType, setNewType] = useState('')
  const [saved, setSaved] = useState(false)

  const colors = draft ?? stored

  // Canonical types always come first (in the order people know them), then
  // whatever else exists in the setting.
  const rows = useMemo(() => {
    const extra = Object.keys(colors).filter(k => !CANONICAL_MATERIAL_TYPES.includes(k as (typeof CANONICAL_MATERIAL_TYPES)[number])).sort()
    return [...CANONICAL_MATERIAL_TYPES, ...extra]
  }, [colors])

  const updateColor = (type: string, color: string) => {
    setDraft(prev => ({ ...(prev ?? stored), [type]: color }))
  }

  const removeType = (type: string) => {
    setDraft(prev => {
      const next = { ...(prev ?? stored) }
      delete next[type]
      return next
    })
  }

  const addType = () => {
    const type = normalizeMaterialTypeKey(newType)
    if (!type || rows.includes(type as (typeof rows)[number])) return
    setDraft(prev => ({ ...(prev ?? stored), [type]: DEFAULT_MATERIAL_COLORS[type] || UNKNOWN_MATERIAL_COLOR }))
    setNewType('')
  }

  const saveColors = useMutation({
    mutationFn: (value: Record<string, string>) => settingsApi.set(FILAMENT_TYPE_COLORS_SETTING_KEY, JSON.stringify(value)),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['settings', FILAMENT_TYPE_COLORS_SETTING_KEY] })
      setDraft(null)
      setSaved(true)
      window.setTimeout(() => setSaved(false), 2500)
    },
  })

  return (
    <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6 mb-6">
      <div className="flex items-start gap-3 mb-5">
        <div className="p-2 bg-sky-500/10 rounded-lg">
          <Palette className="h-6 w-6 text-sky-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-surface-100">Filament label colors</h2>
          <p className="text-sm text-surface-400">
            Each G-code on the Files screen shows a chip with its material type. Pick the color for each type; types you have not
            configured use the built-in palette, and files with no detected material show a neutral label.
          </p>
        </div>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center py-6">
          <RefreshCw className="h-5 w-5 text-surface-400 animate-spin" />
        </div>
      ) : (
        <div className="space-y-2">
          {rows.map(type => {
            const color = colors[type] || DEFAULT_MATERIAL_COLORS[type] || UNKNOWN_MATERIAL_COLOR
            const isCanonical = CANONICAL_MATERIAL_TYPES.includes(type as (typeof CANONICAL_MATERIAL_TYPES)[number])
            return (
              <div key={type} className="flex items-center gap-3 rounded-lg border border-surface-800 bg-surface-900/40 px-3 py-2">
                <input
                  type="color"
                  aria-label={`Color for ${type}`}
                  value={HEX_COLOR.test(color) ? color.toLowerCase() : '#ffffff'}
                  onChange={event => updateColor(type, event.target.value)}
                  className="h-8 w-10 cursor-pointer rounded border border-surface-700 bg-surface-900 p-0.5"
                />
                <div className="w-28">
                  <MaterialBadge material={type} colors={colors} />
                </div>
                <div className="flex-1">
                  <span className="text-sm text-surface-300 font-mono uppercase">{type}</span>
                  {!isCanonical && <span className="ml-2 text-[10px] text-surface-500">custom</span>}
                </div>
                {!isCanonical && (
                  <button
                    type="button"
                    onClick={() => removeType(type)}
                    className="btn btn-ghost text-xs text-red-400"
                    title="Remove this type"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                )}
              </div>
            )
          })}

          <div className="flex items-center gap-2 pt-2">
            <input
              value={newType}
              onChange={event => setNewType(event.target.value)}
              onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); addType() } }}
              placeholder="Add another type (e.g. pc)"
              className="input h-9 text-sm flex-1"
            />
            <button type="button" onClick={addType} disabled={!newType.trim()} className="btn btn-secondary text-sm">
              <Plus className="h-4 w-4 mr-1" />Add type
            </button>
          </div>

          <p className="text-xs text-surface-500">
            Names are normalized the same way the G-code parser normalizes material_type: lowercase, without spaces or separators.
            That is why "PLA-CF" becomes PLA — it is what the file actually reports.
          </p>

          <div className="flex items-center gap-3 pt-2">
            <button
              type="button"
              onClick={() => saveColors.mutate(colors)}
              disabled={saveColors.isPending}
              className="btn btn-primary min-w-28"
            >
              {saveColors.isPending ? 'Saving...' : saved ? 'Saved' : 'Save'}
            </button>
            <button
              type="button"
              onClick={() => setDraft(DEFAULT_MATERIAL_COLORS)}
              disabled={saveColors.isPending}
              className="px-4 py-2 text-sm font-medium text-surface-300 hover:text-surface-100 hover:bg-surface-700 rounded-lg transition-colors"
            >
              <RotateCcw className="h-4 w-4 mr-1 inline" />Restore built-in palette
            </button>
          </div>
          {saveColors.isError && (
            <p className="text-sm text-red-400">
              {saveColors.error instanceof Error ? saveColors.error.message : 'Failed to save colors'}
            </p>
          )}
        </div>
      )}
    </div>
  )
}

function BusinessInfoSettings() {
  const [businessName, setBusinessName] = useState('')
  const [phone, setPhone] = useState('')
  const [email, setEmail] = useState('')
  const [website, setWebsite] = useState('')
  const [address, setAddress] = useState({ line1: '', line2: '', city: '', state: '', zip: '', country: '' })
  const [loaded, setLoaded] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    const load = async () => {
      try {
        const keys = ['business_name', 'business_phone', 'business_email', 'business_website', 'business_address_json']
        const results = await Promise.allSettled(keys.map(k => settingsApi.get(k)))
        const vals: Record<string, string> = {}
        results.forEach((r, i) => {
          if (r.status === 'fulfilled') vals[keys[i]] = r.value.value
        })
        setBusinessName(vals.business_name || '')
        setPhone(vals.business_phone || '')
        setEmail(vals.business_email || '')
        setWebsite(vals.business_website || '')
        if (vals.business_address_json) {
          try { setAddress(JSON.parse(vals.business_address_json)) } catch { /* ignore malformed JSON */ }
        }
      } catch { /* ignore load errors */ }
      setLoaded(true)
    }
    load()
  }, [])

  const handleSave = async () => {
    setSaving(true)
    try {
      await Promise.all([
        settingsApi.set('business_name', businessName),
        settingsApi.set('business_phone', phone),
        settingsApi.set('business_email', email),
        settingsApi.set('business_website', website),
        settingsApi.set('business_address_json', JSON.stringify(address)),
      ])
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err) {
      console.error('Failed to save business info:', err)
    } finally {
      setSaving(false)
    }
  }

  if (!loaded) return null

  return (
    <div className="bg-surface-900/50 border border-surface-800 rounded-xl p-6 mb-6">
      <div className="flex items-center gap-3 mb-4">
        <div className="p-2 bg-accent-500/10 rounded-lg">
          <Building className="h-6 w-6 text-accent-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-surface-100">Business Information</h2>
          <p className="text-sm text-surface-400">Shown on customer-facing quotes</p>
        </div>
      </div>

      <div className="space-y-3">
        <div>
          <label className="block text-xs font-medium text-surface-400 mb-1">Business Name</label>
          <input value={businessName} onChange={e => setBusinessName(e.target.value)} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1">Phone</label>
            <input value={phone} onChange={e => setPhone(e.target.value)} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1">Email</label>
            <input value={email} onChange={e => setEmail(e.target.value)} type="email" className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
          </div>
        </div>
        <div>
          <label className="block text-xs font-medium text-surface-400 mb-1">Website</label>
          <input value={website} onChange={e => setWebsite(e.target.value)} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
        </div>
        <div>
          <label className="block text-xs font-medium text-surface-400 mb-1">Address</label>
          <div className="space-y-1.5">
            <input value={address.line1} onChange={e => setAddress(a => ({ ...a, line1: e.target.value }))} placeholder="Address line 1" className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
            <input value={address.line2} onChange={e => setAddress(a => ({ ...a, line2: e.target.value }))} placeholder="Address line 2" className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
            <div className="grid grid-cols-3 gap-1.5">
              <input value={address.city} onChange={e => setAddress(a => ({ ...a, city: e.target.value }))} placeholder="City" className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
              <input value={address.state} onChange={e => setAddress(a => ({ ...a, state: e.target.value }))} placeholder="State" className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
              <input value={address.zip} onChange={e => setAddress(a => ({ ...a, zip: e.target.value }))} placeholder="Zip" className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
            </div>
            <input value={address.country} onChange={e => setAddress(a => ({ ...a, country: e.target.value }))} placeholder="Country" className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
          </div>
        </div>
        <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 bg-accent-500 text-white rounded-lg hover:bg-accent-600 text-sm font-medium disabled:opacity-50">
          {saving ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          {saved ? 'Saved!' : 'Save Business Info'}
        </button>
      </div>
    </div>
  )
}


function BackupSettings() {
  const queryClient = useQueryClient()

  const { data: config, isLoading } = useQuery({
    queryKey: ['backup-config'],
    queryFn: () => backupsApi.getConfig(),
  })

  const updateMutation = useMutation({
    mutationFn: (newConfig: BackupConfig) => backupsApi.updateConfig(newConfig),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backup-config'] })
    },
  })

  const updateField = <K extends keyof BackupConfig>(key: K, value: BackupConfig[K]) => {
    if (!config) return
    updateMutation.mutate({ ...config, [key]: value })
  }

  if (isLoading || !config) {
    return (
      <div className="mt-4 pt-4 border-t border-surface-800">
        <div className="flex items-center gap-2 text-surface-400 text-sm">
          <RefreshCw className="h-4 w-4 animate-spin" />
          Loading backup settings...
        </div>
      </div>
    )
  }

  return (
    <div className="mt-4 pt-4 border-t border-surface-800">
      <div className="flex items-center gap-2 mb-3">
        <SettingsIcon className="h-4 w-4 text-surface-400" />
        <h3 className="text-sm font-medium text-surface-200">Automatic Backups</h3>
      </div>

      <div className="space-y-3">
        {/* Backup on startup */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-surface-300">Backup on startup</p>
            <p className="text-xs text-surface-500">Create a backup before migrations run on each launch</p>
          </div>
          <button
            onClick={() => updateField('auto_on_startup', !config.auto_on_startup)}
            disabled={updateMutation.isPending}
            className={cn(
              'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
              config.auto_on_startup ? 'bg-blue-500' : 'bg-surface-600'
            )}
          >
            <span
              className={cn(
                'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                config.auto_on_startup ? 'translate-x-6' : 'translate-x-1'
              )}
            />
          </button>
        </div>

        {/* Scheduled backups */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-surface-300">Scheduled backups</p>
            <p className="text-xs text-surface-500">Automatically create backups on a schedule</p>
          </div>
          <button
            onClick={() => updateField('schedule_enabled', !config.schedule_enabled)}
            disabled={updateMutation.isPending}
            className={cn(
              'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
              config.schedule_enabled ? 'bg-blue-500' : 'bg-surface-600'
            )}
          >
            <span
              className={cn(
                'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                config.schedule_enabled ? 'translate-x-6' : 'translate-x-1'
              )}
            />
          </button>
        </div>

        {/* Schedule interval (only shown when schedule is enabled) */}
        {config.schedule_enabled && (
          <div className="flex items-center justify-between pl-4">
            <p className="text-sm text-surface-300">Interval</p>
            <select
              value={config.schedule_interval}
              onChange={(e) => updateField('schedule_interval', e.target.value as 'daily' | 'weekly')}
              disabled={updateMutation.isPending}
              className="input h-auto py-1.5 w-auto"
            >
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
            </select>
          </div>
        )}

        {/* Retention count */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-surface-300">Retention count</p>
            <p className="text-xs text-surface-500">Auto-backups to keep per type (0 = unlimited)</p>
          </div>
          <input
            type="number"
            min={0}
            value={config.retention_count}
            onChange={(e) => {
              const val = parseInt(e.target.value, 10)
              if (!isNaN(val) && val >= 0) {
                updateField('retention_count', val)
              }
            }}
            disabled={updateMutation.isPending}
            className="input h-auto py-1.5 w-20 text-center"
          />
        </div>

      </div>
    </div>
  )
}
