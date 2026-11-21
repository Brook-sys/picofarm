import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../src/lib/gcodeLibraryView.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } })
const context = { exports: {} }
vm.runInNewContext(outputText, context)
const { sortLibraryItems, paginateLibrary, createRequestGate, slicePreviewKey } = context.exports
const gcode = (id, values = {}) => ({ type: 'gcode', file: { id, display_name: id, created_at: '2026-01-01', ...values } })
const ids = items => Array.from(items, item => item.file.id)

test('metric sorting is global across root G-code and STL default variants, missing last', () => {
  const items = [gcode('unknown'), gcode('slow', { estimated_seconds: 200 }), { type: 'stl', file: { id: 'group', gcodes: [{ default_for_stl: false, estimated_seconds: 1 }, { default_for_stl: true, estimated_seconds: 100 }] } }]
  assert.deepEqual(ids(sortLibraryItems(items, 'time_asc')), ['group', 'slow', 'unknown'])
  assert.deepEqual(ids(items), ['unknown', 'slow', 'group'], 'source order is unchanged')
})

test('descending metrics keep missing, invalid and unconfigured STL groups last', () => {
  const items = [gcode('missing'), gcode('zero', { filament_grams: 0 }), gcode('heavy', { filament_grams: 20 }), gcode('invalid', { filament_grams: NaN }), { type: 'stl', file: { id: 'no-default', gcodes: [{ filament_grams: 99 }] } }]
  assert.deepEqual(ids(sortLibraryItems(items, 'grams_desc')).slice(0, 2), ['heavy', 'zero'])
  assert.deepEqual(ids(sortLibraryItems([gcode('one', { print_count: 1 }), gcode('two', { print_count: 2 })], 'prints_desc')), ['two', 'one'])
})

test('name/date sorting and tie-breakers are deterministic', () => {
  const items = [gcode('b'), gcode('a'), gcode('c', { created_at: '2026-02-01' })]
  assert.deepEqual(ids(sortLibraryItems(items, 'created_desc')), ['c', 'a', 'b'])
  assert.deepEqual(ids(sortLibraryItems(items, 'name_desc')), ['c', 'b', 'a'])
})

test('pagination slices groups, clamps after deletes/filter changes, handles empty results', () => {
  const items = Array.from({ length: 103 }, (_, index) => index)
  assert.deepEqual(Array.from(paginateLibrary(items, 2, 50).items), items.slice(50, 100))
  const last = paginateLibrary(items, 9, 50)
  assert.equal(last.current, 3)
  assert.deepEqual(Array.from(last.items), [100, 101, 102])
  assert.equal(paginateLibrary([], 9, 50).current, 1)
  assert.equal(paginateLibrary(items.slice(0, 4), 3, 50).current, 1)
})

test('preview fingerprint covers every slicing parameter but ignores output name', () => {
  const form = { printer: 'printer', preset: 'preset', filament: 'pla', arrange: true, orient: true, enable_support: false, brim_type: false, print_sequence_by_object: false, display_name: 'first' }
  const key = slicePreviewKey('stl', form)
  assert.equal(key, slicePreviewKey('stl', { ...form, display_name: 'renamed' }))
  for (const field of ['printer', 'preset', 'filament', 'arrange', 'orient', 'enable_support', 'brim_type', 'print_sequence_by_object']) {
    assert.notEqual(key, slicePreviewKey('stl', { ...form, [field]: typeof form[field] === 'boolean' ? !form[field] : 'other' }), field)
  }
  assert.notEqual(key, slicePreviewKey('other-stl', form))
  assert.notEqual(key, slicePreviewKey('stl', { ...form, quantity: 2 }))
  for (const axis of ['rotate', 'rotate_x', 'rotate_y']) assert.notEqual(key, slicePreviewKey('stl', { ...form, [axis]: '0' }))
})

test('request gate rejects replaced responses and every pending response after close/unmount', async () => {
  const gate = createRequestGate()
  const first = gate.begin()
  const second = gate.begin()
  assert.equal(gate.isCurrent(first), false)
  assert.equal(gate.isCurrent(second), true)
  gate.invalidate()
  assert.equal(gate.isCurrent(second), false)
  const next = gate.begin()
  assert.equal(gate.isCurrent(next), true)
})
