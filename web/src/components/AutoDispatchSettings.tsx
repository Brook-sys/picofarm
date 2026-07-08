import { useState, type ReactNode } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Bell, Clock, Play, Save, Settings, Zap } from 'lucide-react'
import { dispatchApi } from '../api/client'
import { cn } from '../lib/utils'
import type { AutoDispatchSettings as AutoDispatchSettingsRecord } from '../types'

interface AutoDispatchSettingsProps {
  printerId: string
}

function Toggle({
  checked,
  disabled,
  onClick,
}: {
  checked: boolean
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors',
        disabled && 'cursor-not-allowed opacity-50',
        checked ? 'bg-accent-500' : 'bg-surface-600'
      )}
    >
      <span
        className={cn(
          'inline-block h-4 w-4 rounded-full bg-white transition-transform',
          checked ? 'translate-x-6' : 'translate-x-1'
        )}
      />
    </button>
  )
}

function SettingRow({
  icon,
  title,
  description,
  children,
}: {
  icon: ReactNode
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-lg bg-surface-800/50 p-3">
      <div className="flex min-w-0 items-center gap-3">
        <div className="text-surface-400">{icon}</div>
        <div className="min-w-0">
          <div className="text-surface-200">{title}</div>
          <div className="text-sm text-surface-500">{description}</div>
        </div>
      </div>
      {children}
    </div>
  )
}

export default function AutoDispatchSettings({ printerId }: AutoDispatchSettingsProps) {
  const queryClient = useQueryClient()
  const [macroGcodeDraft, setMacroGcodeDraft] = useState<{ source: string; value: string }>({ source: '', value: '' })

  const { data: globalSettings, isLoading: globalLoading } = useQuery({
    queryKey: ['dispatch-global-settings'],
    queryFn: () => dispatchApi.getGlobalSettings(),
  })

  const { data: settings, isLoading: printerLoading } = useQuery({
    queryKey: ['printer-dispatch-settings', printerId],
    queryFn: () => dispatchApi.getPrinterSettings(printerId),
  })

  const updateGlobalMutation = useMutation({
    mutationFn: (enabled: boolean) => dispatchApi.updateGlobalSettings(enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dispatch-global-settings'] })
    },
  })

  const updatePrinterMutation = useMutation({
    mutationFn: (updates: Partial<AutoDispatchSettingsRecord>) =>
      dispatchApi.updatePrinterSettings(printerId, updates),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['printer-dispatch-settings', printerId] })
    },
  })

  const isLoading = globalLoading || printerLoading

  if (isLoading) {
    return (
      <div className="card p-6">
        <div className="mb-4 flex items-center gap-2">
          <Zap className="h-5 w-5 text-surface-400" />
          <h2 className="text-lg font-semibold text-surface-100">Semi-automatic printing</h2>
        </div>
        <div className="py-4 text-center text-surface-500">Loading dispatch settings...</div>
      </div>
    )
  }

  if (!settings) return null

  const globalEnabled = !!globalSettings?.enabled
  const updating = updateGlobalMutation.isPending || updatePrinterMutation.isPending
  const savedMacroGcode = settings.macro_empty_queue_gcode || ''
  const macroGcode = macroGcodeDraft.source === savedMacroGcode ? macroGcodeDraft.value : savedMacroGcode
  const macroDirty = macroGcode !== savedMacroGcode

  const updatePrinter = (updates: Partial<AutoDispatchSettingsRecord>) => {
    updatePrinterMutation.mutate(updates)
  }

  return (
    <div className="card p-6">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-2">
            <Zap className="h-5 w-5 text-accent-400" />
            <h2 className="text-lg font-semibold text-surface-100">Semi-automatic printing</h2>
          </div>
          <p className="max-w-2xl text-sm text-surface-500">
            One dispatch flow for queued G-code: a printer can ask for the next job through the
            hardware macro, or create the same confirmation card when it becomes idle.
          </p>
        </div>
        <span
          className={cn(
            'badge self-start',
            globalEnabled ? 'bg-emerald-500/15 text-emerald-300' : 'bg-surface-800 text-surface-400'
          )}
        >
          {globalEnabled ? 'Fleet enabled' : 'Fleet paused'}
        </span>
      </div>

      <div className="space-y-4">
        <SettingRow
          icon={<Settings className="h-4 w-4" />}
          title="Fleet dispatch switch"
          description="Master switch shared by macro and idle-triggered semi-automatic dispatch."
        >
          <Toggle
            checked={globalEnabled}
            disabled={updating}
            onClick={() => updateGlobalMutation.mutate(!globalEnabled)}
          />
        </SettingRow>

        <div className="rounded-xl border border-accent-500/20 bg-accent-500/5 p-4">
          <div className="mb-3 flex items-center gap-2">
            <Play className="h-4 w-4 text-accent-300" />
            <h3 className="font-medium text-surface-100">Hardware macro trigger</h3>
          </div>
          <p className="mb-4 text-sm text-surface-500">
            This is the preferred semi-automatic flow: the printer/operator explicitly signals that
            the bed is clear, then PicoFarm starts the next queued job for this printer.
          </p>

          <div className="space-y-3">
            <SettingRow
              icon={<Zap className="h-4 w-4" />}
              title="Enable macro dispatch for this printer"
              description="Allows Klipper/Moonraker macro events to pull from the print queue."
            >
              <Toggle
                checked={settings.macro_auto_dispatch_enabled}
                disabled={updating || !globalEnabled}
                onClick={() => updatePrinter({ macro_auto_dispatch_enabled: !settings.macro_auto_dispatch_enabled })}
              />
            </SettingRow>

            <div className="rounded-lg bg-surface-800/50 p-3">
              <div className="mb-3 flex items-center gap-3">
                <Settings className="h-4 w-4 text-surface-400" />
                <div>
                  <div className="text-surface-200">Empty queue G-code</div>
                  <div className="text-sm text-surface-500">
                    Optional response when the macro runs but there is no compatible job.
                  </div>
                </div>
              </div>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={macroGcode}
                  onChange={(event) => setMacroGcodeDraft({ source: savedMacroGcode, value: event.target.value })}
                  disabled={updating || !settings.macro_auto_dispatch_enabled || !globalEnabled}
                  placeholder="e.g. M117 Queue Empty"
                  className="input w-full"
                />
                <button
                  type="button"
                  onClick={() => updatePrinter({ macro_empty_queue_gcode: macroGcode })}
                  disabled={updating || !macroDirty}
                  className="btn btn-secondary shrink-0"
                >
                  <Save className="mr-2 h-4 w-4" />
                  Save
                </button>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-4">
          <div className="mb-3 flex items-center gap-2">
            <Bell className="h-4 w-4 text-surface-400" />
            <h3 className="font-medium text-surface-200">Idle confirmation fallback</h3>
          </div>
          <p className="mb-4 text-sm text-surface-500">
            Kept as a secondary path from the older idle-triggered flow: when the printer transitions
            from printing to idle, PicoFarm creates the same confirmation notification instead of
            adding another workflow.
          </p>

          <div className="space-y-3">
            <SettingRow
              icon={<Bell className="h-4 w-4" />}
              title="Create confirmation when printer becomes idle"
              description="Does not start by itself; it opens the shared bed-clear confirmation card."
            >
              <Toggle
                checked={settings.enabled}
                disabled={updating || !globalEnabled}
                onClick={() => updatePrinter({ enabled: !settings.enabled })}
              />
            </SettingRow>

            <SettingRow
              icon={<Play className="h-4 w-4" />}
              title="Start immediately after confirmation"
              description="After the operator confirms bed clear, start the assigned G-code automatically."
            >
              <Toggle
                checked={settings.auto_start}
                disabled={updating || !globalEnabled || (!settings.enabled && !settings.macro_auto_dispatch_enabled)}
                onClick={() => updatePrinter({ auto_start: !settings.auto_start })}
              />
            </SettingRow>

            <div className="rounded-lg bg-surface-800/50 p-3">
              <div className="mb-3 flex items-center gap-3">
                <Clock className="h-4 w-4 text-surface-400" />
                <div>
                  <div className="text-surface-200">Confirmation timeout</div>
                  <div className="text-sm text-surface-500">How long the shared confirmation card stays valid.</div>
                </div>
              </div>
              <select
                value={settings.timeout_minutes}
                onChange={(event) => updatePrinter({ timeout_minutes: parseInt(event.target.value, 10) })}
                disabled={updating || !globalEnabled}
                className="input"
              >
                <option value={5}>5 minutes</option>
                <option value={10}>10 minutes</option>
                <option value={15}>15 minutes</option>
                <option value={30}>30 minutes</option>
                <option value={60}>1 hour</option>
                <option value={120}>2 hours</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {!globalEnabled && (
        <div className="mt-4 rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-sm text-amber-200">
          Semi-automatic printing is paused globally. Enable the fleet switch above before this printer can dispatch jobs.
        </div>
      )}
    </div>
  )
}
