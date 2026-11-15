/* Isolated UI smoke: no backend, printers, credentials or external requests.
 * PLAYWRIGHT_MODULE=/path/to/playwright CHROMIUM_EXECUTABLE=/usr/bin/chromium
 * BASE_URL=http://127.0.0.1:3197 node web/tests/camera-wall-browser-smoke.cjs
 */
const assert = require('node:assert/strict')
const http = require('node:http')
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const base = process.env.BASE_URL || 'http://127.0.0.1:3197'
const stamp = '2026-01-01T00:00:00Z'
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1sAAAAASUVORK5CYII=', 'base64')
const printers = Array.from({ length: 9 }, (_, i) => ({ id: `p${i}`, name: `Printer ${i + 1}`, model: 'Synthetic', status: i === 0 ? 'printing' : 'idle', connection_type: 'moonraker', enabled: true }))
const cameras = printers.map((p, i) => ({ id: `c${i}`, printer_id: p.id, name: `Camera ${i + 1}`, type: i === 1 ? 'snapshot' : 'mjpeg', url: `${base}/synthetic-camera/${i}`, enabled: i !== 8, created_at: stamp, updated_at: stamp }))
;(async () => {
  let activeStreams = 0
  const streamServer = http.createServer((request, response) => {
    activeStreams++
    response.writeHead(200, { 'Content-Type': 'multipart/x-mixed-replace; boundary=frame', 'Cache-Control': 'no-store' })
    const frame = () => { response.write(`--frame\r\nContent-Type: image/png\r\nContent-Length: ${png.length}\r\n\r\n`); response.write(png); response.write('\r\n') }
    frame()
    const timer = setInterval(frame, 100)
    request.on('close', () => { clearInterval(timer); activeStreams-- })
  })
  await new Promise(resolve => streamServer.listen(0, '127.0.0.1', resolve))
  const streamOrigin = `http://127.0.0.1:${streamServer.address().port}`
  cameras[0].url = `${streamOrigin}/mjpeg`
  const waitStreams = async expected => {
    for (let i = 0; i < 100; i++) { if (activeStreams === expected) return; await new Promise(resolve => setTimeout(resolve, 50)) }
    assert.equal(activeStreams, expected, 'actual synthetic MJPEG connections')
  }
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || '/usr/bin/chromium', args: ['--no-sandbox'] })
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' })
  const page = await context.newPage()
  const errors = [], mutations = [], images = []
  page.on('pageerror', error => errors.push(error.message))
  await page.route('**/*', async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname
    if (url.origin === streamOrigin) return route.continue()
    if (url.origin !== new URL(base).origin) return route.abort()
    if (path.startsWith('/synthetic-camera/')) {
      images.push(path)
      if (path.endsWith('/7')) return route.fulfill({ status: 503, body: 'synthetic failure' })
      return route.fulfill({ contentType: 'image/png', body: png })
    }
    if (!path.startsWith('/api/')) return route.continue()
    if (request.method() !== 'GET') mutations.push(`${request.method()} ${path}`)
    const data = path === '/api/cameras' ? cameras : path === '/api/printers' ? printers : path === '/api/printers/states' ? Object.fromEntries(printers.map(p => [p.id, { printer_id: p.id, status: p.status, progress: 45, updated_at: stamp }])) : path.startsWith('/api/settings') ? { value: '' } : []
    return route.fulfill({ json: data })
  })
  try {
    await page.goto(`${base}/cameras`)
    await page.getByRole('heading', { name: 'Camera wall', exact: true }).waitFor()
    // Assertions below exercise only synthetic image responses, not real MJPEG capacity.
    await page.getByText('Camera 1', { exact: true }).first().waitFor()
    assert.equal(images.includes('/synthetic-camera/8'), false, 'disabled camera must not load')
    const card = name => page.getByRole('article', { name: `${name} camera`, exact: true })
    await card('Camera 1').locator('img').waitFor()
    await waitStreams(1)
    assert.equal(await page.locator('article').count(), 9)
    await page.getByRole('button', { name: 'Pause Camera 1', exact: true }).click()
    assert.equal(await card('Camera 1').locator('img').count(), 0)
    await waitStreams(0)
    await page.getByRole('button', { name: 'Pause all', exact: true }).click()
    assert.equal(await page.locator('article img').count(), 0)
    await page.getByRole('button', { name: 'Resume all', exact: true }).click()
    assert.equal(await card('Camera 1').locator('img').count(), 0, 'individual pause survives resume all')
    await page.getByRole('button', { name: 'Resume Camera 1', exact: true }).click()
    await card('Camera 1').locator('img').waitFor()
    await page.getByRole('button', { name: 'Favorite Camera 1', exact: true }).click()
    await page.getByRole('combobox', { name: /^Show/ }).selectOption('favorites')
    assert.equal(await page.locator('article').count(), 1)
    await page.getByRole('combobox', { name: /^Show/ }).selectOption('printing')
    assert.equal(await page.locator('article').count(), 1)
    await page.getByRole('combobox', { name: /^Show/ }).selectOption('all')
    await page.getByRole('combobox', { name: /^Card size/ }).selectOption('small')
    await page.getByRole('button', { name: 'Move Camera 2 earlier', exact: true }).click()
    assert.equal(await page.locator('article').first().getAttribute('aria-label'), 'Camera 2 camera')
    await page.getByRole('button', { name: 'Enlarge Camera 1', exact: true }).click()
    await page.getByRole('dialog').waitFor()
    assert.equal(await page.locator('img[alt="Camera 1 camera preview"]').count(), 1, 'enlargement must not duplicate stream')
    await waitStreams(1)
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Enlarge Camera 1', exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Enlarge Camera 1', exact: true }).evaluate(node => node === document.activeElement), true, 'focus returns to enlargement trigger')
    await card('Camera 1').getByRole('button', { name: 'Focus', exact: true }).click()
    assert.equal(await page.getByRole('combobox', { name: /^Layout/ }).inputValue(), 'focus')
    await page.getByRole('button', { name: 'Wall mode', exact: true }).click()
    await page.getByRole('dialog', { name: 'Camera wall', exact: true }).waitFor()
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Wall mode', exact: true }).waitFor()
    await page.reload()
    await card('Camera 1').waitFor()
    assert.equal(await page.getByRole('combobox', { name: /^Card size/ }).inputValue(), 'small')
    assert.equal(await page.locator('article').first().getAttribute('aria-label'), 'Camera 2 camera')
    assert.equal(await page.getByRole('button', { name: 'Unfavorite Camera 1', exact: true }).count(), 1)
    await page.getByText('Choose cameras', { exact: false }).click()
    await page.getByRole('checkbox', { name: 'Camera 1 Printer 1', exact: true }).uncheck()
    assert.equal(await card('Camera 1').count(), 0)
    await waitStreams(0)
    await page.reload()
    await card('Camera 2').waitFor()
    assert.equal(await card('Camera 1').count(), 0)
    await page.setViewportSize({ width: 390, height: 844 })
    await page.getByRole('button', { name: 'Wall mode', exact: true }).click()
    const wall = page.getByRole('dialog', { name: 'Camera wall', exact: true })
    await wall.waitFor()
    assert.ok(await wall.evaluate(node => node.scrollWidth <= node.clientWidth + 1), 'mobile wall must not overflow')
    await page.keyboard.press('Escape')
    const failedBefore = images.filter(path => path === '/synthetic-camera/7').length
    await card('Camera 8').getByRole('button', { name: 'Retry preview', exact: true }).click()
    await card('Camera 8').getByRole('button', { name: 'Retry preview', exact: true }).waitFor()
    assert.equal(images.filter(path => path === '/synthetic-camera/7').length, failedBefore + 1)
    await page.getByRole('button', { name: 'Reset layout', exact: true }).click()
    await card('Camera 1').waitFor()
    assert.equal(await page.getByRole('combobox', { name: /^Card size/ }).inputValue(), 'auto')
    await page.getByRole('button', { name: 'Wall mode', exact: true }).click()
    await page.getByRole('button', { name: 'Enlarge Camera 1', exact: true }).click()
    assert.equal(await page.locator('dialog[open]').count(), 2)
    await page.keyboard.press('Escape')
    assert.equal(await page.locator('dialog[open]').count(), 1)
    await page.keyboard.press('Escape')
    assert.equal(await page.locator('dialog[open]').count(), 0)
    const restricted = await browser.newContext({ viewport: { width: 1280, height: 800 } })
    await restricted.addInitScript(() => { const get = Storage.prototype.getItem; const set = Storage.prototype.setItem; Storage.prototype.getItem = function (key) { if (key === 'picofarm.camera-wall.v1') throw new Error('blocked'); return get.call(this, key) }; Storage.prototype.setItem = function (key, value) { if (key === 'picofarm.camera-wall.v1') throw new Error('blocked'); return set.call(this, key, value) } })
    const restrictedPage = await restricted.newPage()
    // Reuse synthetic routing without contacting a backend.
    await restrictedPage.route('**/*', async route => {
      const url = new URL(route.request().url())
      if (url.origin === streamOrigin) return route.continue()
      if (url.origin !== new URL(base).origin) return route.abort()
      if (url.pathname.startsWith('/synthetic-camera/')) return route.fulfill({ contentType: 'image/png', body: png })
      if (url.pathname.startsWith('/api/')) return route.fulfill({ json: url.pathname === '/api/cameras' ? cameras : url.pathname === '/api/printers' ? printers : url.pathname === '/api/printers/states' ? {} : url.pathname.startsWith('/api/settings') ? { value: '' } : [] })
      return route.continue()
    })
    restrictedPage.on('pageerror', error => errors.push(error.message))
    await restrictedPage.goto(`${base}/cameras`)
    await restrictedPage.getByRole('article', { name: 'Camera 1 camera', exact: true }).waitFor({ timeout: 8000 }).catch(async error => { console.error(await restrictedPage.locator('body').innerText(), errors); throw error })
    await restrictedPage.getByRole('combobox', { name: /^Card size/ }).selectOption('large')
    await restricted.close()
    assert.deepEqual(mutations, [])
    assert.deepEqual(errors, [])
    console.log('PASS camera wall synthetic smoke')
  } finally { await browser.close(); streamServer.closeAllConnections(); await new Promise(resolve => streamServer.close(resolve)) }
})().catch(error => { console.error(error); process.exitCode = 1 })
