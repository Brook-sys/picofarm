export type SlicerForm = {
  printer: string; preset: string; filament: string; display_name: string
  quantity: number; arrange: boolean; orient: boolean
  rotate: string; rotate_x: string; rotate_y: string
  enable_support: boolean; brim_type: boolean; print_sequence_by_object: boolean
}

export function slicerFormError(form: SlicerForm): 'quantity' | 'rotation' | 'arrange' | 'orientation' | null {
  if (!Number.isInteger(form.quantity) || form.quantity < 1 || form.quantity > 512) return 'quantity'
  if ([form.rotate, form.rotate_x, form.rotate_y].some(value => value.trim() !== '' && !Number.isFinite(Number(value)))) return 'rotation'
  if (form.quantity > 1 && !form.arrange) return 'arrange'
  if (form.orient && [form.rotate, form.rotate_x, form.rotate_y].some(value => value.trim() !== '')) return 'orientation'
  return null
}

export function slicerFormPayload(stlId: string, form: SlicerForm) {
  const { rotate, rotate_x, rotate_y, ...rest } = form
  return {
    stl_file_id: stlId, ...rest, export_type: 'gcode',
    ...(rotate.trim() !== '' ? { rotate: Number(rotate) } : {}),
    ...(rotate_x.trim() !== '' ? { rotate_x: Number(rotate_x) } : {}),
    ...(rotate_y.trim() !== '' ? { rotate_y: Number(rotate_y) } : {}),
  }
}
