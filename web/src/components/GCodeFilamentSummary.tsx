import { useTranslation } from 'react-i18next'
import type { GCodeLibraryFile } from '../types'
import type { MaterialColorMap } from '../lib/materialColors'
import { MaterialBadge } from './MaterialBadge'

/** Slicer-declared filament, not an inventory/spool assignment. */
export default function GCodeFilamentSummary({ file, colors }: { file: GCodeLibraryFile; colors: MaterialColorMap }) {
  const { t } = useTranslation('library')
  const profile = file.metadata?.filament_settings_id?.trim() || file.filament_name?.trim()
  return (
    <div className="mt-2 min-w-0 rounded-lg border border-surface-700/70 bg-surface-950/50 px-2 py-1.5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[10px] font-medium text-surface-500">{t('Filament Profile')}</span>
        <MaterialBadge material={file.material_type} colors={colors} />
      </div>
      <p className="mt-1 break-words text-xs text-surface-300 [overflow-wrap:anywhere]">{profile || t('Not found')}</p>
    </div>
  )
}
