import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import type { STLLibraryFile } from '../types'
import { formatDuration } from '../lib/utils'


export function GCodeVariantComparison({ file }: { file: STLLibraryFile }) {
  const { t } = useTranslation('components')
  const measurement = (value: number | undefined, unit: string) => typeof value === 'number' && Number.isFinite(value) && value > 0 ? `${value} ${unit}` : t('Not provided')
  const variants = file.gcodes || []
  const [selected, setSelected] = useState<string[]>(() => variants.slice(0, 4).map(variant => variant.id))
  if (variants.length < 2) return null
  const compared = variants.filter(variant => selected.includes(variant.id))
  return (
    <section className="mt-5 border-t border-surface-800 pt-4" aria-label={t("Existing variant comparison")}>
      <h3 className="font-semibold text-surface-100">{t("Compare existing G-codes for this STL")}</h3>
      <p className="mt-1 text-xs text-surface-400">{t("Select 2 to 4 variants. File metadata and estimates do not indicate quality, compatibility, or print results. No new slicing is performed.")}</p>
      <div className="my-3 flex flex-wrap gap-3">
        {variants.map(variant => <label key={variant.id} className="flex items-center gap-2 text-xs text-surface-300"><input type="checkbox" checked={selected.includes(variant.id)} disabled={!selected.includes(variant.id) && selected.length >= 4} onChange={event => setSelected(current => event.target.checked ? [...current, variant.id] : current.filter(id => id !== variant.id))} />{variant.display_name}</label>)}
      </div>
      {compared.length < 2 ? <p className="text-sm text-surface-400">{t("Select at least two variants.")}</p> : <div className="overflow-x-auto"><table className="w-full text-left text-xs text-surface-300">
        <caption className="sr-only">{t("Metadata for G-codes linked to")}{file.display_name}</caption>
        <thead><tr><th className="p-2">{t("Parameter")}</th>{compared.map(variant => <th scope="col" key={variant.id} className="p-2 min-w-40">{variant.display_name}{variant.default_for_stl ? t("(default)") : ''}</th>)}</tr></thead>
        <tbody>{[
          { label: t("Estimated time"), values: compared.map(v => v.estimated_seconds && v.estimated_seconds > 0 ? formatDuration(v.estimated_seconds) : t("Not provided")) },
          { label: t("Material"), values: compared.map(v => v.material_type || t("Not provided")) },
          { label: t("Estimated filament"), values: compared.map(v => measurement(v.filament_grams, 'g')) },
          { label: t("Layer"), values: compared.map(v => measurement(v.layer_height, 'mm')) },
          { label: t("Nozzle"), values: compared.map(v => measurement(v.nozzle_diameter, 'mm')) },
          { label: t("Print profile"), values: compared.map(v => v.metadata?.print_settings_id || t("Not provided")) },
          { label: t("Filament profile"), values: compared.map(v => v.metadata?.filament_settings_id || v.filament_name || t("Not provided")) },
          { label: t("Printer profile"), values: compared.map(v => v.metadata?.printer_settings_id || t("Not provided")) },
        ].map(row => <tr key={row.label} className="border-t border-surface-800"><th scope="row" className="p-2 font-medium">{row.label}</th>{row.values.map((value, index) => <td key={compared[index].id} className="p-2 break-words">{value}</td>)}</tr>)}</tbody>
      </table></div>}
    </section>
  )
}
