import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import * as THREE from 'three'

const source = readFileSync(new URL('../src/components/stlViewerUtils.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } })
const translate = (key, values = {}) => key.replace(/{{(\w+)}}/g, (match, name) => values[name] ?? match)
const i18n = { t: translate }
const context = { exports: {}, require: name => name === '../i18n' ? { default: i18n, t: translate } : THREE, Uint8Array, DataView }
vm.runInNewContext(outputText, context)
const { inspectSTLGeometry, stlViewPose, stlCameraDistance, readSTLResponse, MAX_STL_BYTES } = context.exports

function geometry(values) {
  return new THREE.BufferGeometry().setAttribute('position', new THREE.Float32BufferAttribute(values, 3))
}

test('dimensions and triangle count use unrotated STL axes, including offset coordinates', () => {
  const model = geometry([10, 20, 30, 14, 20, 30, 10, 26, 38])
  const info = inspectSTLGeometry(model)
  assert.deepEqual(info.dimensions.toArray(), [4, 6, 8])
  assert.deepEqual(info.center.toArray(), [12, 23, 34])
  assert.equal(info.triangles, 1)
  model.dispose()
})

test('empty, incomplete, non-finite, zero-sized and over-budget geometry rejected', () => {
  for (const values of [[], [0, 0, 0], [NaN, 0, 0, 1, 0, 0, 0, 1, 0], Array(9).fill(0)]) {
    const model = geometry(values)
    assert.throws(() => inspectSTLGeometry(model))
    model.dispose()
  }
  const model = geometry([0, 0, 0, 1, 0, 0, 0, 1, 0])
  model.setIndex({ count: 1_500_003 })
  assert.throws(() => inspectSTLGeometry(model), /triangle preview limit/)
})

test('canonical camera poses have perpendicular up vectors and fit portrait/landscape', () => {
  for (const view of ['top', 'front', 'side', 'reset']) {
    const { direction, up } = stlViewPose(view)
    assert.ok(direction.length() > 0)
    assert.ok(direction.clone().cross(up).length() > 0.5)
  }
  assert.deepEqual(stlViewPose('top').direction.toArray(), [0, 1, 0])
  assert.deepEqual(stlViewPose('front').direction.toArray(), [0, 0, 1])
  assert.deepEqual(stlViewPose('side').direction.toArray(), [1, 0, 0])
  assert.ok(stlCameraDistance(10, 0.5, 35) > stlCameraDistance(10, 2, 35))
})

test('download handles HTTP errors, header and streamed limits, cancellation and valid bytes', async () => {
  const signal = new AbortController().signal
  await assert.rejects(readSTLResponse(new Response('', { status: 404 }), signal), /HTTP 404/)
  await assert.rejects(readSTLResponse(new Response('', { headers: { 'content-length': String(MAX_STL_BYTES + 1) } }), signal), /32 MiB/)
  let cancelled = false
  const stream = new ReadableStream({
    start(controller) { controller.enqueue(new Uint8Array(MAX_STL_BYTES + 1)) },
    cancel() { cancelled = true },
  })
  await assert.rejects(readSTLResponse(new Response(stream), signal), /32 MiB/)
  assert.equal(cancelled, true)
  const abort = new AbortController()
  abort.abort()
  await assert.rejects(readSTLResponse(new Response('abc'), abort.signal), /abort/i)
  const bytes = await readSTLResponse(new Response(new Uint8Array([1, 2, 3])), signal)
  assert.deepEqual([...new Uint8Array(bytes)], [1, 2, 3])
})
