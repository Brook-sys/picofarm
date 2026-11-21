import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
const source = readFileSync(new URL('../src/lib/slicerForm.ts', import.meta.url), 'utf8')
const context = { exports: {} }
vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, context)
const { slicerFormError, slicerFormPayload } = context.exports
const base = { printer: 'p', preset: 's', filament: 'f', display_name: 'x', quantity: 1, arrange: true, orient: true, rotate: '', rotate_x: '', rotate_y: '', enable_support: false, brim_type: false, print_sequence_by_object: false }
test('quantity is exact, integral and bounded', () => {
  for (const quantity of [0, -1, 513, 1.5, NaN]) assert.equal(slicerFormError({ ...base, quantity }), 'quantity')
  for (const quantity of [1, 512]) assert.equal(slicerFormError({ ...base, quantity }), null)
  assert.equal(slicerFormError({ ...base, quantity: 2, arrange: false }), 'arrange')
})
test('absent angles differ from explicit zero and conflict with orientation', () => {
  assert.equal('rotate' in slicerFormPayload('id', base), false)
  assert.equal(slicerFormError({ ...base, rotate: '0' }), 'orientation')
  const explicit = { ...base, orient: false, rotate: '0', rotate_x: '90', rotate_y: '-45' }
  assert.equal(slicerFormError(explicit), null)
  const payload = slicerFormPayload('id', explicit)
  assert.equal(payload.rotate, 0); assert.equal(payload.rotate_x, 90); assert.equal(payload.rotate_y, -45)
  assert.equal(payload.quantity, 1); assert.equal(payload.export_type, 'gcode')
  assert.equal(slicerFormError({ ...base, rotate: 'Infinity' }), 'rotation')
})
