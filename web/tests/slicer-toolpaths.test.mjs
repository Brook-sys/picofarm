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

test('missing and unknown wire versions safely produce no geometry', () => {
  for (const value of [undefined, null, {}, { ...payload([segment]), version: 2 }, { ...payload([segment]), version: 3 }]) {
    const result = sanitizeToolpaths(value)
    assert.equal(result.supported, false)
    assert.equal(result.layers.length, 0)
  }
})

test('full v2 requires explicit completeness and coherent source counters', () => {
  const full = {
    ...payload([{ ...segment, to: [10, 20, 0.4], feedrateMmSec: 25, object: 'benchy', tool: 2, widthMm: 0.42, heightMm: null }]),
    version: 2, mode: 'full', geometryComplete: true,
    totalLayers: 1, returnedLayers: 1, totalSegments: 1, returnedSegments: 1, omittedSegments: 0,
  }
  const result = sanitizeToolpaths(full, true)
  assert.equal(result.supported, true)
  assert.equal(result.completeGeometry, true)
  assert.equal(result.partial, false)
  assert.equal(result.layers[0].segments[0].feedrateMmSec, 25)
  assert.equal(result.layers[0].segments[0].object, 'benchy')
  assert.equal(result.layers[0].segments[0].tool, 2)
  assert.equal(result.layers[0].segments[0].widthMm, 0.42)
  assert.equal(result.layers[0].segments[0].heightMm, undefined)
  assert.equal(result.bounds.minZ, 0.2)
  assert.ok(Math.abs(result.bounds.depth - 0.2) < 1e-12)
  for (const invalid of [
    { geometryComplete: false },
    { totalSegments: 2 },
    { totalLayers: 2 },
    { returnedSegments: 0 },
    { returnedLayers: 0 },
    { mode: 'legacy' },
    { omittedSegments: -1 },
    { omittedSegments: undefined },
    { layers: [{ index: 1, z: 0.2, segments: [segment] }] },
  ]) {
    const candidate = sanitizeToolpaths({ ...full, ...invalid }, true)
    assert.equal(candidate.completeGeometry, false)
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

test('full preview above the shared 250k budget never claims completeness', () => {
  const count = 250_001
  const result = sanitizeToolpaths({ ...payload(Array(count).fill(segment)), version: 2, mode: 'full', geometryComplete: true,
    totalLayers: 1, returnedLayers: 1, totalSegments: count, returnedSegments: count, omittedSegments: 0 }, true)
  assert.equal(result.layers[0].segments.length, 250_000)
  assert.equal(result.partial, true)
  assert.equal(result.completeGeometry, false)
})

test('legacy counters cannot upgrade XY-only geometry into a full XYZ scene', () => {
  const result = sanitizeToolpaths({ ...payload([segment]), totalLayers: 1, returnedLayers: 1, totalSegments: 1, returnedSegments: 1, omittedSegments: 0 }, true)
  assert.equal(result.completeGeometry, false)
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
