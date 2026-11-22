import type { GCodeLibraryFile } from '../types'
import type { MaterialColorMap } from '../lib/materialColors'
import { MaterialBadge } from './MaterialBadge'

export default function GCodeFilamentSummary({ file, colors }: { file: GCodeLibraryFile; colors: MaterialColorMap }) {
  return <div className="mt-1"><MaterialBadge material={file.material_type} colors={colors} /></div>
}
