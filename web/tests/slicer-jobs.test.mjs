import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../src/lib/slicerJobSubmission.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
let generated = 0
const context = { exports: {}, crypto: { randomUUID: () => `key-${++generated}` } }
vm.runInNewContext(outputText, context)
const { slicerSubmissionKey, forgetSlicerSubmission } = context.exports

test('ambiguous submission retries retain key; changes do not destroy previous dedup identity', async () => {
  const payload = { stl_file_id: 'stl', printer: 'printer', display_name: 'part.gcode' }
  const first = slicerSubmissionKey(payload)
  await assert.rejects(async () => { throw new Error('network response lost') })
  assert.equal(slicerSubmissionKey({ ...payload }), first)
  const changed = slicerSubmissionKey({ ...payload, printer: 'other' })
  assert.notEqual(changed, first)
  assert.equal(slicerSubmissionKey(payload), first, 'restoring parameters must not silently create a fresh task')
  forgetSlicerSubmission(first)
  assert.notEqual(slicerSubmissionKey(payload), first, 'only explicit history discard removes retained identity')
  assert.equal(slicerSubmissionKey({ ...payload, printer: 'other' }), changed)
})

test('UI keeps preview/comparison and uses background submission with stable transport', () => {
  const page = readFileSync(new URL('../src/pages/GCodeFiles.tsx', import.meta.url), 'utf8')
  assert.match(page, /slicerApi\.submitJob\(payload, slicerSubmissionKey\(payload\)\)/)
  assert.doesNotMatch(page, /await slicerApi\.sliceSTL/)
  assert.match(page, /<SlicerJobsPanel api=\{slicerJobsTransport\}/)
  assert.match(page, /slicerApi\.preview\(/)
  assert.match(page, /<GCodeVariantComparison/)
  assert.match(page, /previewIsStale/)
  const panel = readFileSync(new URL('../src/components/SlicerJobsPanel.tsx', import.meta.url), 'utf8')
  assert.match(panel, /window\.confirm/)
  assert.match(panel, /await api\.delete\(job\.id\)/)
  assert.match(panel, /forgetSlicerSubmission\(job\.idempotency_key\)/)
  assert.match(panel, /job\.status !== 'queued' && job\.status !== 'running'/)
})
