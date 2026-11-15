import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../src/components/cameras/cameraWallPreferences.ts', import.meta.url), 'utf8')
function load(storage) {
  const context = { exports: {}, localStorage: storage }
  const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
  vm.runInNewContext(outputText, context)
  return context.exports
}
const api = load()
const plain = value => JSON.parse(JSON.stringify(value))

test('invalid storage values normalize to safe defaults', () => {
  for (const value of [null, false, 'bad', 42, [], { size: 'huge', layout: 'unknown', selected: false }]) {
    assert.deepEqual(plain(api.normalizeCameraWallPreferences(value)), { size: 'auto', layout: 'grid', selected: null, order: [], favorites: [] })
  }
})
test('selection distinguishes all including future cameras from explicitly none', () => {
  assert.equal(api.normalizeCameraWallPreferences({}).selected, null)
  assert.deepEqual(plain(api.normalizeCameraWallPreferences({ selected: [] }).selected), [])
})
test('preferences discard unknown fields and deduplicate/validate IDs', () => {
  const value = api.normalizeCameraWallPreferences({ size: 'small', layout: 'focus', selected: ['c2', 'c2', '', 1, null, 'x'.repeat(513), 'c1'], order: ['c2', 'c2'], favorites: ['c1'], url: 'https://camera.example/private', token: 'fake-secret' })
  assert.deepEqual(plain(value), { size: 'small', layout: 'focus', selected: ['c2', 'c1'], order: ['c2'], favorites: ['c1'] })
  assert.equal(JSON.stringify(value).includes('private'), false)
})
test('saved ordering ignores removed IDs and appends discoveries in API order', () => {
  const available = ['c1', 'c2', 'c3', 'c4'], saved = ['removed', 'c3', 'c1', 'c3']
  assert.deepEqual(Array.from(api.orderCameraIds(available, saved)), ['c3', 'c1', 'c2', 'c4'])
  assert.deepEqual(available, ['c1', 'c2', 'c3', 'c4'])
  assert.deepEqual(saved, ['removed', 'c3', 'c1', 'c3'])
})
test('blocked, missing and malformed localStorage are nonfatal', () => {
  for (const storage of [undefined, { getItem() { throw Error('blocked') } }, { getItem() { return '{broken' } }, { getItem() { return null } }]) {
    assert.equal(load(storage).readCameraWallPreferences().size, 'auto')
  }
  assert.equal(load({ getItem() { return '{"size":"large"}' } }).readCameraWallPreferences().size, 'large')
})
