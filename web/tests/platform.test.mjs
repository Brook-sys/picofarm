import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

function load(file, context = {}) {
  const source = readFileSync(new URL(`../src/lib/${file}.ts`, import.meta.url), 'utf8').replaceAll('import.meta.env', '({})')
  const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
  const scope = { exports: {}, URL, console, ...context }
  vm.runInNewContext(outputText, scope)
  return scope.exports
}
function browser(runtime, token = null) {
  return { location: { origin: 'https://web.example', assign() {} }, localStorage: { getItem: () => token }, __PICOFARM_DESKTOP__: runtime }
}

test('web defaults use same origin; desktop resources are rebased, third parties untouched', () => {
  const window = browser()
  const api = load('platform', { window })
  assert.equal(api.resourceURL('/api/files/one'), 'https://web.example/api/files/one')
  assert.equal(api.webSocketURL(), 'wss://web.example/ws')
  window.__PICOFARM_DESKTOP__ = { apiBaseURL: 'http://localhost:3456/' }
  assert.equal(api.resourceURL('/api/files/one'), 'http://localhost:3456/api/files/one')
  for (const path of ['https://other.example/api/files/one', '//other.example/a', 'blob:test', 'data:image/png;base64,a', '/projects/one', '/apiculture']) assert.equal(api.resourceURL(path), path)
})

test('both websocket clients use one late-token URL helper', () => {
  const window = browser({ apiBaseURL: 'https://api.example', wsBaseURL: 'https://socket.example/?other=1' }, 'a &b')
  const api = load('platform', { window })
  const url = new URL(api.webSocketURL())
  assert.equal(url.href.startsWith('wss://socket.example/ws?'), true)
  assert.equal(url.searchParams.get('token'), 'a &b')
  assert.equal(url.searchParams.get('other'), '1')
  for (const file of ['../src/api/client.ts', '../src/hooks/useWebSocket.ts']) assert.match(readFileSync(new URL(file, import.meta.url), 'utf8'), /webSocketURL\(\)/)
})

test('external URL validation and browser/desktop delegation', async () => {
  const calls = []
  const window = browser({ apiBaseURL: 'https://api.example', openExternal: url => calls.push(url) })
  const api = load('platform', { window })
  await api.openExternal('https://oauth.example/auth')
  assert.deepEqual(calls, ['https://oauth.example/auth'])
  await assert.rejects(api.openExternal('javascript:alert(1)'))
  await assert.rejects(api.openExternal('https://user:secret@example.com/'))
  window.__PICOFARM_DESKTOP__ = undefined
  window.location.assign = url => calls.push(url)
  await api.openExternal('https://web.example/auth')
  assert.equal(calls[1], 'https://web.example/auth')
})

test('notifications default narrowly, deduplicate, respect opt-outs and never forward details', () => {
  const calls = []
  let saved = '{}'
  const platform = { desktopRuntime: () => ({ notify: value => calls.push(value) }) }
  const api = load('desktopNotifications', { require: () => platform, localStorage: { getItem: () => saved, setItem: (_, value) => { saved = value } } })
  const event = { type: 'job_completed', data: { id: '1', customer: 'PRIVATE', name: 'PRIVATE' } }
  api.notifyDesktopEvent(event); api.notifyDesktopEvent(event)
  api.notifyDesktopEvent({ type: 'job_started', data: { id: '2' } })
  api.notifyDesktopEvent({ type: 'alerts_updated', data: { alerts: [{ id: 'alert', severity: 'critical', message: 'PRIVATE' }] } })
  assert.equal(calls.length, 2)
  assert.equal(JSON.stringify(calls).includes('PRIVATE'), false)
  api.saveNotificationPreferences({ finished: false, failed: false, critical: false })
  api.notifyDesktopEvent({ type: 'job_failed', data: { id: '3' } })
  assert.equal(calls.length, 2)
  saved = '{invalid'
  assert.equal(api.notificationPreferences().finished, true)
})

test('malformed desktop origins fail closed', () => {
  for (const apiBaseURL of ['file:///tmp/app', 'https://user:secret@example.com', 'not-a-url']) {
    const api = load('platform', { window: browser({ apiBaseURL }) })
    assert.throws(() => api.resourceURL('/api/files/one'))
  }
})
