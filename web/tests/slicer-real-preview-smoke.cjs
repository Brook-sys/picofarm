/* Isolated real-payload smoke against a built frontend; never contacts a backend.
 * REAL_PREVIEW_FILE=/path/to/private-preview.json is REQUIRED (do not commit it).
 * Optional: BASE_URL, PLAYWRIGHT_MODULE, CHROMIUM_EXECUTABLE, REAL_SCREENSHOT,
 * REAL_RESULTS_FILE (append-only JSON array), SMOKE_TIMEOUT_MS.
 * Screenshots/results must be outside the repository. Heap figures are Chromium
 * performance.memory observations, not a leak test or process/GPU memory metric.
 */
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { performance } = require('node:perf_hooks');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.BASE_URL || 'http://127.0.0.1:18084';
assert.ok(process.env.REAL_PREVIEW_FILE, 'REAL_PREVIEW_FILE is required');
const source = fs.readFileSync(process.env.REAL_PREVIEW_FILE);
const payload = JSON.parse(source);
const tp = payload.toolpaths;
assert.equal(tp.version, 2); assert.equal(tp.mode, 'full');
assert.equal(tp.geometryComplete, true); assert.equal(tp.sourceComplete, true);
assert.equal(tp.truncated, false); assert.equal(tp.omittedSegments, 0);
assert.equal(tp.returnedSegments, tp.totalSegments);
assert.equal(tp.layers.length, tp.totalLayers);
const all = tp.layers.flatMap(layer => layer.segments);
assert.equal(all.length, tp.totalSegments);
// Real complete fixtures contain extrusion only: fail rather than silently discount
// unexpected travel/degenerate geometry when asserting EVERY source segment.
assert.ok(all.every(s => s.extrusion > 0 && s.from.reduce((n, v, i) => n + (s.to[i] - v) ** 2, 0) > 1e-14));
const support = all.filter(s => ['support', 'support interface', 'support transition'].includes((s.feature || '').toLowerCase())).length;
assert.equal(support, tp.supportSegments);
const total = all.length, firstLayer = tp.layers[0].segments.length;
const stamp = '2026-01-01T00:00:00Z';
const stl = { id: 'stl', file_id: 'model', display_name: 'Real payload smoke', file_name: 'smoke.stl', size_bytes: 0, gcodes: [], created_at: stamp, updated_at: stamp };
const result = { fixture: path.basename(process.env.REAL_PREVIEW_FILE), quantity: payload.quantity, sourceSegments: total, supportSegments: support, firstLayerSegments: firstLayer, payloadBytes: source.length, checks: [] };
function externalOutput(file) {
  if (!file) return;
  // Resolve symlinks in the existing parent too, so output cannot hide in repo.
  const output = path.join(fs.realpathSync(path.dirname(path.resolve(file))), path.basename(file));
  const repo = fs.realpathSync(path.resolve(__dirname, '../..'));
  assert.ok(output !== repo && !output.startsWith(repo + path.sep), 'Output must be outside repository');
  return output;
}
const screenshot = externalOutput(process.env.REAL_SCREENSHOT);
const resultsFile = externalOutput(process.env.REAL_RESULTS_FILE);
(async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CHROMIUM_EXECUTABLE ? { executablePath: process.env.CHROMIUM_EXECUTABLE } : {}), args: ['--no-sandbox', '--enable-unsafe-swiftshader', '--use-gl=angle', '--use-angle=swiftshader', '--enable-precise-memory-info'] });
  const context = await browser.newContext({ locale: 'en-US', serviceWorkers: 'block', viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  page.setDefaultTimeout(Number(process.env.SMOKE_TIMEOUT_MS || 120000));
  const errors = [], unexpected = [], calls = [];
  page.on('pageerror', e => errors.push(e.message));
  await page.addInitScript(() => {
    window.__realSmokeContexts = [];
    const original = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (type, ...args) {
      const gl = original.call(this, type, ...args);
      if (gl && ['webgl', 'webgl2'].includes(type) && !window.__realSmokeContexts.includes(gl)) window.__realSmokeContexts.push(gl);
      return gl;
    };
  });
  await context.route('**/*', async route => {
    const req = route.request(), url = new URL(req.url()), p = url.pathname, method = req.method();
    if (url.origin !== new URL(base).origin) return route.abort();
    if (!p.startsWith('/api/')) {
      if (method === 'GET' && ['document', 'script', 'stylesheet', 'image', 'font', 'other'].includes(req.resourceType())) return route.continue();
      unexpected.push(`${method} ${p}`); return route.abort();
    }
    calls.push({ method, path: p });
    const json = data => route.fulfill({ json: data });
    if (p === '/api/slicer/preview' && method === 'POST') {
      assert.equal(req.postDataJSON().quantity, payload.quantity);
      return route.fulfill({ contentType: 'application/json', body: source });
    }
    if (method !== 'GET') { unexpected.push(`${method} ${p}`); return route.abort(); }
    if (p === '/api/file-library') return json({ stl_files: [stl], root_gcode_files: [] });
    if (p === '/api/slicer/config') return json({ connection_url: 'http://slicer.example', default_profiles: {} });
    if (/^\/api\/slicer\/(health|status)$/.test(p)) return json({ status: 'ok' });
    if (/^\/api\/slicer\/profiles\/(printers|presets|filaments)$/.test(p)) return json([{ name: 'smoke', default: true }]);
    return json(p.startsWith('/api/settings/') ? { value: '' } : []);
  });
  const canvas = page.locator('canvas[aria-label="3D sliced toolpath preview"]');
  const rendered = async expected => {
    await page.waitForFunction(value => document.querySelector('canvas[aria-label="3D sliced toolpath preview"]')?.getAttribute('data-rendered-segments') === String(value), expected);
    // The counter marks uploaded geometry; wait for the scheduled render frame too.
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  };
  const heap = () => page.evaluate(() => performance.memory ? { usedJSHeapSize: performance.memory.usedJSHeapSize, totalJSHeapSize: performance.memory.totalJSHeapSize, jsHeapSizeLimit: performance.memory.jsHeapSizeLimit } : null);
  const cleanup = () => page.evaluate(() => ({ canvases: document.querySelectorAll('canvas').length, contexts: window.__realSmokeContexts.map(gl => ({ attached: gl.canvas.isConnected, lost: gl.isContextLost() })) }));
  try {
    await page.goto(`${base}/files`);
    await page.getByRole('button', { name: 'Slice', exact: true }).first().click();
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('spinbutton', { name: 'Pieces on one plate' }).fill(String(payload.quantity));
    result.heapBefore = await heap();
    const started = performance.now();
    await dialog.getByRole('button', { name: 'Generate preview', exact: true }).click();
    await rendered(total);
    result.wallMsToRendered = Math.round(performance.now() - started);
    result.heapRendered = await heap();
    result.checks.push('all source segments rendered');
    if (screenshot) { await canvas.scrollIntoViewIfNeeded(); await page.screenshot({ path: screenshot }); result.screenshot = screenshot; }
    await dialog.getByRole('checkbox', { name: 'Support & interface', exact: true }).uncheck();
    await rendered(total - support);
    await dialog.getByRole('checkbox', { name: 'Support & interface', exact: true }).check();
    await rendered(total);
    result.checks.push('support toggle exact segment delta');
    const slider = dialog.getByRole('slider', { name: 'Show layers through' });
    await slider.focus(); await page.keyboard.press('Home'); await rendered(firstLayer);
    await page.keyboard.press('End'); await rendered(total);
    result.checks.push('first-layer cut and full restore');
    await dialog.getByRole('button', { name: 'Use 2D view', exact: true }).click();
    await dialog.locator('svg[role="img"]').waitFor({ state: 'visible' });
    assert.equal(await canvas.count(), 0);
    const switched = await cleanup();
    assert.ok(switched.contexts.length > 0 && switched.contexts.every(c => !c.attached && c.lost));
    await dialog.getByRole('button', { name: 'Use 3D view', exact: true }).click();
    await rendered(total);
    result.checks.push('2D fallback disposes context; 3D reentry renders all');
    await canvas.evaluate(element => {
      const gl = element.getContext('webgl2') || element.getContext('webgl');
      const extension = gl.getExtension('WEBGL_lose_context');
      if (!extension) throw new Error('WEBGL_lose_context required for context-loss smoke');
      extension.loseContext();
    });
    const alert = dialog.getByRole('alert').filter({ hasText: 'The browser lost its 3D context.' });
    await alert.waitFor({ state: 'visible' });
    await alert.getByRole('button', { name: 'Use 2D view', exact: true }).click();
    await dialog.locator('svg[role="img"]').waitFor({ state: 'visible' });
    await dialog.getByRole('button', { name: 'Use 3D view', exact: true }).click();
    await rendered(total);
    result.checks.push('actual WebGL context loss visible error and usable 2D fallback/reentry');
    await dialog.getByRole('button', { name: 'Close', exact: true }).click();
    await dialog.waitFor({ state: 'detached' });
    result.cleanup = await cleanup();
    assert.equal(result.cleanup.canvases, 0);
    assert.ok(result.cleanup.contexts.length >= 3 && result.cleanup.contexts.every(c => !c.attached && c.lost));
    result.heapAfterClose = await heap();
    result.checks.push('close removes canvases and loses all created WebGL contexts');
    assert.deepEqual(errors, []); assert.deepEqual(unexpected, []);
    result.apiCalls = calls; result.passed = true;
  } catch (error) {
    result.passed = false; result.error = error.message; result.pageErrors = errors; result.unexpected = unexpected;
    throw error;
  } finally {
    await browser.close();
    if (resultsFile) {
      const previous = fs.existsSync(resultsFile) ? JSON.parse(fs.readFileSync(resultsFile, 'utf8')) : [];
      assert.ok(Array.isArray(previous), 'REAL_RESULTS_FILE must contain a JSON array');
      fs.writeFileSync(resultsFile, JSON.stringify([...previous, result], null, 2) + '\n');
    }
    console.log(JSON.stringify(result, null, 2));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
