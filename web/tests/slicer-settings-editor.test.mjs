import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

// Like order-detail.test.mjs, exercise actual handlers and rendered props without
// a DOM. Browser focus, native confirmation and navigation remain smoke checks.
const source = readFileSync(new URL('../src/pages/SlicerSettings.tsx', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
})
const original = '{\n  "name": "q1",\n  "layer_height": "0.2"\n}'
const draft = '{"name":"q1","layer_height":"0.3"}'

function harness(api = {}) {
  const values = []
  const refs = []
  const effects = []
  let stateIndex = 0
  let refIndex = 0
  let timerCalls = 0
  const jsx = (type, props) => ({ type, props })
  const modules = {
    react: {
      useState: initial => {
        const slot = stateIndex++
        if (!(slot in values)) values[slot] = initial
        return [values[slot], next => { values[slot] = typeof next === 'function' ? next(values[slot]) : next }]
      },
      useRef: initial => {
        const slot = refIndex++
        refs[slot] ??= { current: initial }
        return refs[slot]
      },
      useEffect: (fn, deps) => { effects.push({ fn, deps }) },
    },
    'react/jsx-runtime': { jsx, jsxs: jsx },
    'lucide-react': new Proxy({}, { get: (_, name) => name }),
    '../api/client': { slicerApi: api },
  }
  const context = {
    exports: {}, require: name => modules[name], Error,
    window: {
      confirm: () => true,
      addEventListener() {}, removeEventListener() {},
      setTimeout: () => { timerCalls++; return 1 }, clearTimeout() {},
    },
  }
  vm.runInNewContext(outputText, context)
  const render = () => {
    stateIndex = 0
    refIndex = 0
    effects.length = 0
    return context.exports.default()
  }
  render()
  // Locate the editor state slot from the actual setter use in source, without
  // exporting component internals solely for a test.
  const declarations = [...source.matchAll(/const \[(\w+),\s*\w+\] = useState/g)].map(match => match[1])
  const editorSlot = declarations.indexOf('editModal')
  assert.ok(editorSlot >= 0)
  values[editorSlot] = {
    category: 'presets', name: 'q1', json: original, originalJson: original,
    currentHash: 'original-hash', revisions: [], validation: null,
    status: 'idle', message: '',
  }
  return { render, effects, editor: () => values[editorSlot], timers: () => timerCalls }
}

function find(node, predicate) {
  if (!node || typeof node !== 'object') return undefined
  if (predicate(node)) return node
  for (const child of [node.props?.children].flat(Infinity)) {
    const result = find(child, predicate)
    if (result) return result
  }
}
function text(node) {
  if (typeof node === 'string') return node
  if (!node || typeof node !== 'object') return ''
  return [node.props?.children].flat(Infinity).map(text).join('')
}
function button(app, label) {
  const found = find(app.render(), node => node.type === 'button' && text(node) === label)
  assert.ok(found, `missing ${label} button`)
  return found.props
}
function edit(app, json) {
  find(app.render(), node => node.type === 'textarea' && node.props['aria-label'] === 'Profile JSON').props.onChange({ target: { value: json } })
}
const valid = { valid: true, errors: [], warnings: ['Not a certification'] }

test('editor has no autosave; validate then edit invalidates approval; Save sends only explicit draft', async () => {
  const validations = []
  const saves = []
  const app = harness({
    validateProfile: async request => { validations.push(request); return valid },
    saveProfileRevision: async request => {
      saves.push(request)
      return { current_json: request.json, current_hash: 'new-hash', revisions: [] }
    },
    profiles: async () => [],
  })
  edit(app, draft)
  assert.equal(saves.length, 0)
  assert.equal(button(app, 'Save').disabled, true)
  // No effect is allowed to depend on editor text for persistence. The only
  // editor effect is the boolean unsaved-draft beforeunload warning.
  assert.ok(app.effects.every(effect => !effect.deps?.includes(draft)))
  assert.equal(app.timers(), 0)
  await button(app, 'Validate').onClick()
  assert.equal(validations.length, 1)
  assert.equal(button(app, 'Save').disabled, false)
  const updated = '{"name":"q1","layer_height":"0.4"}'
  edit(app, updated)
  assert.equal(app.editor().validation, null)
  assert.equal(button(app, 'Save').disabled, true)
  await button(app, 'Save').onClick()
  assert.equal(saves.length, 0, 'handler also refuses unvalidated drafts')
  await button(app, 'Validate').onClick()
  assert.equal(saves.length, 0, 'validation never saves')
  await button(app, 'Save').onClick()
  assert.equal(saves.length, 1)
  assert.equal(saves[0].json, updated)
  assert.equal(saves[0].expected_hash, 'original-hash')
  assert.equal(app.editor().json, app.editor().originalJson)
  assert.equal(app.editor().currentHash, 'new-hash')
})

for (const message of ['profile changed since it was loaded; reload before saving', 'save not confirmed; recovery snapshots retained']) {
  test(`failed save preserves draft and original: ${message}`, async () => {
    const app = harness({ validateProfile: async () => valid, saveProfileRevision: async () => { throw Error(message) } })
    edit(app, draft)
    await button(app, 'Validate').onClick()
    await button(app, 'Save').onClick()
    assert.equal(app.editor().json, draft)
    assert.equal(app.editor().originalJson, original)
    assert.equal(app.editor().currentHash, 'original-hash')
    assert.equal(app.editor().status, 'error')
    assert.equal(app.editor().message, message)
    assert.equal(button(app, 'Close').disabled, false)
  })
}

test('invalid preflight keeps draft and blocks save', async () => {
  let saves = 0
  const app = harness({
    validateProfile: async () => ({ valid: false, errors: ['JSON must be an object'], warnings: [] }),
    saveProfileRevision: async () => { saves++ },
  })
  edit(app, 'null')
  await button(app, 'Validate').onClick()
  assert.equal(button(app, 'Save').disabled, true)
  await button(app, 'Save').onClick()
  assert.equal(saves, 0)
  assert.equal(app.editor().json, 'null')
})
