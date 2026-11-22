import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import ts from 'typescript'
import * as THREE from 'three'

const source = readFileSync(new URL('../src/components/toolpath3D.ts', import.meta.url), 'utf8')
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }).outputText
const imported = js.replace("from 'three'", `from ${JSON.stringify(import.meta.resolve('three'))}`)
const { extrusionCategory, buildExtrusionMeshes, extrusionInstances } = await import(`data:text/javascript;base64,${Buffer.from(imported).toString('base64')}`)
const all = Object.fromEntries(['walls', 'infill', 'bridge', 'support', 'brim', 'other'].map(key => [key, true]))
const segment = (feature, overrides = {}) => ({ from: [0, 0, 0.2], to: [10, 0, 0.2], extrusion: 1, feature, widthMm: 0.4, heightMm: 0.2, tool: 0, ...overrides })
const layers = segments => [{ index: 0, z: 0.2, segments }, { index: 1, z: 0.4, segments: segments.map(item => ({ ...item, from: [0, 0, 0.4], to: [10, 0, 0.4] })) }]

test('Orca roles distinguish supports, bridges, walls and infill; unknown is not guessed', () => {
  assert.equal(extrusionCategory('Support interface'), 'support')
  assert.equal(extrusionCategory('Support transition'), 'support')
  assert.equal(extrusionCategory('Bridge'), 'bridge')
  assert.equal(extrusionCategory('Internal Bridge'), 'bridge')
  assert.equal(extrusionCategory('Overhang wall'), 'walls')
  assert.equal(extrusionCategory('Internal solid infill'), 'infill')
  assert.equal(extrusionCategory('Gap infill'), 'infill')
  assert.equal(extrusionCategory('Brim'), 'brim')
  assert.equal(extrusionCategory('Custom'), 'other')
  assert.equal(extrusionCategory('Mystery bridge-ish'), 'other')
  assert.equal(extrusionCategory('Custom infill category'), 'other')
  assert.equal(extrusionCategory('Unknown wall-like'), 'other')
  assert.equal(extrusionCategory('Top surface'), 'infill')
})

test('physical bead instances have exact bounds and outward triangle winding', () => {
  const result = buildExtrusionMeshes(layers([segment('Outer wall')]), 0, all)
  assert.equal(result.length, 1)
  assert.equal(result[0].segments, 1)
  assert.equal(result[0].estimatedSegments, false)
  assert.equal(result[0].matrices.length, 16)
  assert.equal(result[0].matrices.byteLength, 64)
  const material = new THREE.MeshStandardMaterial()
  const mesh = extrusionInstances(result[0], material)
  assert.equal(mesh.count, 1)
  assert.equal(mesh.geometry.attributes.position.count, 24)
  assert.equal(mesh.geometry.index.count, 36)
  const matrix = new THREE.Matrix4().fromArray(result[0].matrices)
  const positions = mesh.geometry.attributes.position
  for (let axis = 0; axis < 3; axis++) {
    const values = Array.from({ length: positions.count }, (_, index) => new THREE.Vector3().fromBufferAttribute(positions, index).applyMatrix4(matrix).getComponent(axis))
    const actual = [Math.min(...values), Math.max(...values)]
    const expected = [[0, 10], [-0.2, 0.2], [0.1, 0.3]][axis]
    actual.forEach((value, index) => assert.ok(Math.abs(value - expected[index]) < 1e-6))
  }
  const index = mesh.geometry.index
  for (let face = 0; face < index.count; face += 3) {
    const points = [0, 1, 2].map(offset => new THREE.Vector3().fromBufferAttribute(positions, index.getX(face + offset)).applyMatrix4(matrix))
    const normal = new THREE.Vector3().subVectors(points[1], points[0]).cross(new THREE.Vector3().subVectors(points[2], points[0])).normalize()
    const center = points[0].clone().add(points[1]).add(points[2]).multiplyScalar(1 / 3).sub(new THREE.Vector3(5, 0, 0.2))
    assert.ok(center.dot(normal) > 0, `triangle ${face / 3} normal must point outwards`)
  }
  mesh.dispose(); mesh.geometry.dispose(); material.dispose()
})

test('layer cut, filters, non-extrusion and zero-length moves are honored', () => {
  const segments = [segment('Outer wall'), segment('Support'), segment('Bridge'), segment('Brim', { extrusion: 0 }), segment('Custom', { to: [0, 0, 0.2] })]
  const onlySupport = buildExtrusionMeshes(layers(segments), 0, { ...all, walls: false, bridge: false })
  assert.equal(onlySupport.length, 1)
  assert.equal(onlySupport[0].category, 'support')
  const both = buildExtrusionMeshes(layers([segment('Outer wall')]), 1, all)
  assert.equal(both[0].segments, 2)
  assert.equal(both[0].matrices.byteLength, 2 * 64)
  assert.equal(buildExtrusionMeshes(layers([segment('Outer wall')]), 0, { ...all, walls: false }).length, 0)
})

test('missing dimensions are explicitly marked estimated, including Z lift', () => {
  for (const overrides of [{ widthMm: undefined }, { heightMm: undefined }, { widthMm: undefined, heightMm: undefined }]) {
    const [result] = buildExtrusionMeshes(layers([segment('Bridge', { ...overrides, to: [1, 1, 0.4] })]), 0, all)
    assert.equal(result.category, 'bridge')
    assert.equal(result.estimatedSegments, true)
    assert.ok([...result.matrices].every(Number.isFinite))
    assert.ok(new THREE.Matrix4().fromArray(result.matrices).determinant() > 0)
  }
  const [vertical] = buildExtrusionMeshes(layers([segment('Custom', { to: [0, 0, 1.2] })]), 0, all)
  assert.ok([...vertical.matrices].every(Number.isFinite))
  assert.ok(new THREE.Matrix4().fromArray(vertical.matrices).determinant() > 0)
})
