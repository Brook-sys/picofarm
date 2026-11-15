import { useState } from 'react'
import type { STLLibraryFile } from '../types'
import { formatDuration } from '../lib/utils'

const measurement = (value: number | undefined, unit: string) => typeof value === 'number' && Number.isFinite(value) && value > 0 ? `${value} ${unit}` : 'Não informado'

export function GCodeVariantComparison({ file }: { file: STLLibraryFile }) {
  const variants = file.gcodes || []
  const [selected, setSelected] = useState<string[]>(() => variants.slice(0, 4).map(variant => variant.id))
  if (variants.length < 2) return null
  const compared = variants.filter(variant => selected.includes(variant.id))
  return (
    <section className="mt-5 border-t border-surface-800 pt-4" aria-label="Comparação de variantes existentes">
      <h3 className="font-semibold text-surface-100">Comparar G-codes existentes deste STL</h3>
      <p className="mt-1 text-xs text-surface-400">Selecione de 2 a 4 variantes. Metadados e estimativas do arquivo; não indicam qualidade, compatibilidade ou resultado de impressão. Nenhum novo fatiamento é executado.</p>
      <div className="my-3 flex flex-wrap gap-3">
        {variants.map(variant => <label key={variant.id} className="flex items-center gap-2 text-xs text-surface-300"><input type="checkbox" checked={selected.includes(variant.id)} disabled={!selected.includes(variant.id) && selected.length >= 4} onChange={event => setSelected(current => event.target.checked ? [...current, variant.id] : current.filter(id => id !== variant.id))} />{variant.display_name}</label>)}
      </div>
      {compared.length < 2 ? <p className="text-sm text-surface-400">Selecione ao menos duas variantes.</p> : <div className="overflow-x-auto"><table className="w-full text-left text-xs text-surface-300">
        <caption className="sr-only">Metadados dos G-codes vinculados a {file.display_name}</caption>
        <thead><tr><th className="p-2">Parâmetro</th>{compared.map(variant => <th scope="col" key={variant.id} className="p-2 min-w-40">{variant.display_name}{variant.default_for_stl ? ' (padrão)' : ''}</th>)}</tr></thead>
        <tbody>{[
          { label: 'Tempo estimado', values: compared.map(v => v.estimated_seconds && v.estimated_seconds > 0 ? formatDuration(v.estimated_seconds) : 'Não informado') },
          { label: 'Material', values: compared.map(v => v.material_type || 'Não informado') },
          { label: 'Filamento estimado', values: compared.map(v => measurement(v.filament_grams, 'g')) },
          { label: 'Camada', values: compared.map(v => measurement(v.layer_height, 'mm')) },
          { label: 'Nozzle', values: compared.map(v => measurement(v.nozzle_diameter, 'mm')) },
          { label: 'Perfil de impressão', values: compared.map(v => v.metadata?.print_settings_id || 'Não informado') },
          { label: 'Perfil de filamento', values: compared.map(v => v.metadata?.filament_settings_id || v.filament_name || 'Não informado') },
          { label: 'Perfil de impressora', values: compared.map(v => v.metadata?.printer_settings_id || 'Não informado') },
        ].map(row => <tr key={row.label} className="border-t border-surface-800"><th scope="row" className="p-2 font-medium">{row.label}</th>{row.values.map((value, index) => <td key={compared[index].id} className="p-2 break-words">{value}</td>)}</tr>)}</tbody>
      </table></div>}
    </section>
  )
}
