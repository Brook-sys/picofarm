import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../src/lib/slicerToolpaths.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
const context = { exports: {} }
vm.runInNewContext(outputText, context)
const { sanitizeToolpaths, toolpathPaths, toolpathKind } = context.exports
const segment = { from: [-10, 0, 0.2], to: [10, 20, 0.2], extrusion: 1, feature: 'Outer wall', tool: 0 }
const payload = segments => ({ version: 1, positionUnit: 'mm', coordinateSystem: 'gcode', sourceComplete: true, truncated: false, layers: [{ index: 0, z: 0.2, segments }] })

test('legacy and unknown versions safely produce no geometry', () => {
  for (const value of [undefined, null, {}, { ...payload([segment]), version: 2 }]) {
    const result = sanitizeToolpaths(value)
    assert.equal(result.supported, false)
    assert.equal(result.layers.length, 0)
  }
})

test('bounds and fixed SVG paths derive only from finite retained geometry', () => {
  const result = sanitizeToolpaths({ ...payload([segment, { ...segment, to: [Infinity, 0, 0] }]), bounds: { min: [-1e300, 0, 0], max: [1e300, 0, 0] } })
  assert.equal(result.partial, true)
  assert.equal(result.layers[0].segments.length, 1)
  assert.equal(result.bounds.width, 20)
  const paths = toolpathPaths(result.layers[0], result.bounds)
  assert.match(paths.model, /^M[\d.,]+L[\d.,]+$/)
  assert.doesNotMatch(paths.model, /NaN|Infinity/)
})

test('segment and layer work are bounded, including invalid segment entries', () => {
  const result = sanitizeToolpaths(payload(Array(20_100).fill(segment)))
  assert.equal(result.layers[0].segments.length, 20_000)
  assert.equal(result.partial, true)
  const invalid = sanitizeToolpaths(payload([...Array(20_000).fill(null), segment]))
  assert.equal(invalid.layers[0].segments.length, 0)
  const manyLayers = sanitizeToolpaths({ ...payload([]), layers: Array(600).fill({ index: 0, z: 0, segments: [] }) })
  assert.equal(manyLayers.layers.length, 512)
  assert.equal(manyLayers.partial, true)
})

test('feature semantics classify non-extrusion first and exact backend support labels', () => {
  for (const feature of ['Support', 'Support interface', 'Support transition']) assert.equal(toolpathKind({ extrusion: 1, feature }), 'support')
  assert.equal(toolpathKind({ extrusion: 1, feature: 'Brim' }), 'brim')
  assert.equal(toolpathKind({ extrusion: 0, feature: 'Support' }), 'travel')
  assert.equal(toolpathKind({ extrusion: -1 }), 'travel')
  assert.equal(toolpathKind({ extrusion: 1, feature: 'Wipe' }), 'model')
})

test('partial flags and warning text are bounded without trusting remote strings', () => {
  for (const flags of [{ sourceComplete: false }, { truncated: true }, { omittedSegments: 1 }]) {
    assert.equal(sanitizeToolpaths({ ...payload([segment]), ...flags }).partial, true)
  }
  const result = sanitizeToolpaths({ ...payload([segment]), warnings: Array(100).fill('x'.repeat(1000)) })
  assert.equal(result.warnings.length, 20)
  assert.equal(result.warnings[0].length, 500)
})
