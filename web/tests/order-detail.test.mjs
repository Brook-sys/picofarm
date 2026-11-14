import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

// Exercise the real component handlers without a DOM or an added test framework.
const source = readFileSync(new URL('../src/pages/OrderDetail.tsx', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
})

function harness({ tracking = null, api = {}, order = { id: 'test-order', status: 'completed', source: 'manual', customer_name: 'Test' } } = {}) {
  const values = [order, null, false]
  const setters = []
  const callbacks = []
  let index = 0
  const jsx = (type, props) => ({ type, props })
  const modules = {
    react: {
      useState: initial => {
        const slot = index++
        if (!(slot in values)) values[slot] = initial
        setters[slot] = value => { values[slot] = value }
        return [values[slot], setters[slot]]
      },
      useCallback: fn => { callbacks.push(fn); return fn },
      useEffect: () => {},
    },
    'react/jsx-runtime': { jsx, jsxs: jsx },
    'react-router-dom': { useParams: () => ({ id: 'test-order' }), Link: 'a' },
    'lucide-react': new Proxy({}, { get: (_, name) => name }),
    '../api/client': { ordersApi: api, projectsApi: {} },
  }
  const context = { exports: {}, require: name => modules[name], prompt: () => tracking, confirm: () => true, console: { error() {} } }
  vm.runInNewContext(outputText, context)
  const render = () => {
    index = 0
    return context.exports.OrderDetail()
  }
  return { tree: render(), values, callbacks, render }
}

function find(node, predicate) {
  if (!node || typeof node !== 'object') return undefined
  if (predicate(node)) return node
  const children = node.props?.children
  for (const child of [children].flat(Infinity)) {
    const match = find(child, predicate)
    if (match) return match
  }
}

for (const tracking of [null, '', 'TRACK-123']) {
  test(`shipping prompt ${JSON.stringify(tracking)}`, async () => {
    const calls = []
    const app = harness({ tracking, api: { markShipped: async (...args) => { calls.push(args); return { status: 'shipped' } } } })
    await find(app.tree, node => node.props?.children === 'Mark Shipped').props.onClick()
    assert.equal(calls.length, tracking === null ? 0 : 1)
    if (tracking !== null) assert.deepEqual(calls[0], ['test-order', tracking || undefined])
    assert.equal(app.values[6], false, 'status controls are released')
  })
}

test('progress failure preserves order and presents retry instead of not-found', async () => {
  const order = { id: 'test-order', status: 'completed', source: 'manual' }
  const app = harness({ order: null, api: { get: async () => order, getProgress: async () => { throw Error('500') } } })
  await app.callbacks[0]()
  assert.equal(app.values[0], order)
  assert.equal(app.values[1], null)
  const alert = find(app.render(), node => node.props?.role === 'alert')
  assert.ok(alert)
  assert.match(app.values[7], /Unable to load progress/)
})

test('order load failure offers retry and does not claim not-found', async () => {
  const app = harness({ order: null, api: { get: async () => { throw Error('offline') }, getProgress: async () => ({}) } })
  await app.callbacks[0]()
  const tree = app.render()
  assert.match(find(tree, node => node.props?.role === 'alert').props.children, /Unable to load order/)
  assert.ok(find(tree, node => node.props?.children === 'Retry loading order'))
})

test('processing failure shows an accessible error and releases the button', async () => {
  const app = harness({ order: { id: 'test-order', status: 'pending', source: 'manual', items: [{ id: 'item', template_id: 'template' }] }, api: { processItem: async () => { throw Error('500') } } })
  const button = find(app.tree, node => node.type === 'button' && node.props.children?.includes?.('Process'))
  await button.props.onClick()
  assert.equal(app.values[4], null)
  assert.match(app.values[8], /Unable to process item/)
  assert.ok(find(app.render(), node => node.props?.role === 'alert'))
})
