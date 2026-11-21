/* Isolated built-frontend smoke. Install Playwright separately; no backend is used.
 * PLAYWRIGHT_MODULE=playwright CHROMIUM_EXECUTABLE=/usr/bin/chromium
 * BASE_URL=http://127.0.0.1:18084 node web/tests/slicer-browser-smoke.cjs
 * Strict by default: missing WebGL fails. ALLOW_NO_WEBGL=1 explicitly skips
 * dimensions/camera assertions and checks the visible initialization fallback.
 */
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.BASE_URL || 'http://127.0.0.1:18084';
const stamp = '2026-01-01T00:00:00Z';
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1sAAAAASUVORK5CYII=';
const gcodes = [0, 1].map(i => ({ id: `g${i}`, file_id: `gf${i}`, parent_stl_id: 'stl', display_name: `Variant ${i}`, file_name: `variant-${i}.gcode`, default_for_stl: i === 0, material_type: i ? 'PETG' : 'PLA', filament_grams: 12 + i, estimated_seconds: 3600 + i * 600, layer_height: i ? 0.3 : 0.2, nozzle_diameter: 0.4, print_count: 0, metadata: { print_settings_id: `process-${i}` }, created_at: stamp, updated_at: stamp }));
const stl = { id: 'stl', file_id: 'model', display_name: 'Synthetic STL', file_name: 'synthetic.stl', size_bytes: 184, gcodes, created_at: stamp, updated_at: stamp };
// Two synthetic triangles span original axes 10 x 20 x 30 mm; not a closed solid.
const model = `solid smoke\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 10 0 0\nvertex 0 20 0\nendloop\nendfacet\nfacet normal 1 0 0\nouter loop\nvertex 0 0 0\nvertex 0 20 0\nvertex 0 0 30\nendloop\nendfacet\nendsolid smoke`;
(async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CHROMIUM_EXECUTABLE ? { executablePath: process.env.CHROMIUM_EXECUTABLE } : {}), args: ['--no-sandbox', '--enable-unsafe-swiftshader', '--use-gl=angle', '--use-angle=swiftshader'] });
  const context = await browser.newContext({ locale: 'en-US', serviceWorkers: 'block', viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  const errors = [], calls = [], unexpected = [];
  page.on('pageerror', e => errors.push(e.message));
  page.on('dialog', d => d.accept());
  let current = JSON.stringify({ name: 'smoke', layer_height: '0.2' }), hash = 'hash-1', conflict = false;
  let revisions = [{ id: 1, hash: 'snapshot-hash', json: current, created_at: stamp }];
  let jobs = ['queued', 'running', 'succeeded', 'failed', 'interrupted'].map((status, i) => ({ id: `job-${i}`, status, request: { stl_file_id: 'stl', display_name: `Smoke ${status}` }, idempotency_key: `key-${i}`, created_at: stamp, updated_at: stamp, ...(status === 'succeeded' ? { result: { gcode: { id: 'g0', display_name: 'Variant 0' } } } : {}) }));
  await page.route('**/*', async route => {
    const req = route.request(), url = new URL(req.url()), p = url.pathname, method = req.method();
    if (!p.startsWith('/api/')) {
      if (url.origin === new URL(base).origin && !['fetch', 'xhr', 'eventsource'].includes(req.resourceType())) return route.continue();
      // External fonts/logos are irrelevant to this isolated smoke; never fetch them.
      if (url.origin !== new URL(base).origin) return route.abort();
      unexpected.push(`${method} ${p}`); return route.abort();
    }
    calls.push({ method, path: p, body: req.postData() });
    const json = data => route.fulfill({ json: data });
    if (p === '/api/files/model') return route.fulfill({ contentType: 'model/stl', body: model });
    if (p === '/api/file-library') return json({ stl_files: [stl], root_gcode_files: [] });
    if (p === '/api/slicer/config') return json({ connection_url: 'http://slicer.example', default_profiles: {} });
    if (/^\/api\/slicer\/(health|status)$/.test(p)) return json({ status: 'ok' });
    if (/^\/api\/slicer\/profiles\/(printers|presets|filaments)$/.test(p)) return json([{ name: 'smoke', default: true }]);
    if (p.endsWith('/history')) return json({ current_json: current, current_hash: hash, revisions });
    if (p === '/api/slicer/profiles/validate') return json({ valid: true, errors: [], warnings: ['Mock preflight only'] });
    if (p === '/api/slicer/profiles/revisions') {
      const data = req.postDataJSON(); assert.equal(data.expected_hash, hash);
      if (conflict) return route.fulfill({ status: 409, json: { error: 'Profile changed externally; reload before saving' } });
      current = data.json; hash = 'hash-2'; revisions = [...revisions, { id: 2, hash, json: current, created_at: stamp }];
      return json({ current_json: current, current_hash: hash, revisions });
    }
    if (p === '/api/slicer/preview') return json({ usesSupport: true, printTime: 3660, filamentUsedG: 12.5, filamentUsedMm: 4200, thumbnail: png, quantity: req.postDataJSON().quantity, toolpaths: { version: 1, positionUnit: 'mm', coordinateSystem: 'gcode', sourceComplete: true, truncated: true, omittedSegments: 10, warnings: ['Synthetic parser warning'], layers: [0, 1].map(index => ({ index, z: 0.2 + index * 0.2, segments: [{ from: [10, 10, 0.2], to: [20, 20, 0.2], extrusion: 1, feature: index ? 'Support' : 'Brim', tool: 0 }] })) } });
    if (p === '/api/slicer/jobs' && method === 'GET') return json(jobs);
    if (p.endsWith('/cancel') && method === 'POST') { jobs[0].status = 'cancelled'; return json(jobs[0]); }
    if (p === '/api/slicer/jobs/job-3' && method === 'DELETE') { jobs = jobs.filter(j => j.id !== 'job-3'); return json({}); }
    if (method !== 'GET') unexpected.push(`${method} ${p}`);
    return json(p.startsWith('/api/settings/') ? { value: '' } : []);
  });
  const visible = async (locator) => { await locator.waitFor({ state: 'visible' }); };
  const text = async (locator, value) => { await visible(locator); assert.ok((await locator.innerText()).includes(value), value); };
  try {
    await page.goto(`${base}/files`);
    await page.getByRole('button', { name: 'Details', exact: true }).first().click();
    const comparison = page.getByRole('region', { name: 'Existing variant comparison' });
    await text(comparison, 'Variant 0(default)'); await text(comparison, 'PETG'); await text(comparison, '0.2 mm'); await text(comparison, '0.3 mm');
    await comparison.getByRole('checkbox', { name: 'Variant 1' }).uncheck();
    await text(comparison, 'Select at least two variants.');
    await comparison.getByRole('checkbox', { name: 'Variant 1' }).check();
    await visible(comparison.getByRole('table'));
    console.log('PASS comparison selection and metadata for two linked variants');
    const webgl = await page.evaluate(() => !!document.createElement('canvas').getContext('webgl2'));
    if (!webgl && process.env.ALLOW_NO_WEBGL === '1') {
      await visible(page.getByText('Could not initialize 3D preview. Check that WebGL is available in your browser.', { exact: true }));
      console.log('SKIP STL dimensions/cameras: Chromium cannot create WebGL2; verified visible error fallback only');
    } else {
      await visible(page.getByText('X: 10 mm · Y: 20 mm · Z: 30 mm · Triangles: 2', { exact: true }));
      assert.equal(await page.locator('canvas').count(), 1);
      for (const name of ['Top', 'Front', 'Side', 'Reset']) await page.getByRole('button', { name, exact: true }).click();
      console.log('PASS real WebGL STL dimensions and camera controls');
    }
    await page.getByRole('button', { name: 'Close', exact: true }).click();
    await page.getByRole('button', { name: 'Slice', exact: true }).first().click();
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('spinbutton', { name: 'Pieces on one plate' }).fill('513');
    assert.equal(await dialog.getByRole('button', { name: 'Generate preview', exact: true }).isDisabled(), true);
    await dialog.getByRole('spinbutton', { name: 'Pieces on one plate' }).fill('3');
    assert.equal(await dialog.getByRole('checkbox', { name: 'Auto-arrange', exact: true }).isDisabled(), true);
    await dialog.getByRole('spinbutton', { name: 'Pieces on one plate' }).fill('1');
    await dialog.getByText('Advanced positioning', { exact: true }).click();
    await dialog.getByRole('spinbutton', { name: 'Z (°)', exact: true }).fill('0');
    assert.equal(await dialog.getByRole('checkbox', { name: 'Auto-orient', exact: true }).isChecked(), false);
    assert.equal(await dialog.getByRole('checkbox', { name: 'Auto-orient', exact: true }).isDisabled(), true);
    await page.getByRole('button', { name: 'Generate preview', exact: true }).click();
    await visible(page.getByText('Preview generated for the submitted parameters.', { exact: true }));
    await visible(page.getByText('Filament: 12.5 g /4200 mm', { exact: true }));
    await visible(dialog.getByText('Verified pieces: 1 · Plate 1', { exact: true }));
    await visible(dialog.getByText(/Partial preview:/));
    assert.equal(await dialog.locator('svg[role="img"]').count(), 1);
    const slider = dialog.getByRole('slider');
    await slider.focus(); await page.keyboard.press('ArrowRight');
    assert.equal(await slider.inputValue(), '1');
    assert.equal(await dialog.locator('svg path').count(), 1);
    const sent = JSON.parse(calls.find(c => c.path === '/api/slicer/preview').body);
    assert.equal(sent.quantity, 1); assert.equal(sent.rotate, 0); assert.equal(sent.orient, false);
    console.log('PASS bounded layer preview, partial warning, quantity constraints and explicit zero rotation');
    await page.getByText('Auto-arrange', { exact: true }).click();
    await visible(page.getByRole('status').filter({ hasText: 'Preview out of date' }));
    assert.equal(calls.filter(c => c.path === '/api/slicer/preview').length, 1);
    await page.getByRole('button', { name: 'Generate preview', exact: true }).click();
    await visible(page.getByText('Preview generated for the submitted parameters.', { exact: true }));
    assert.equal(await page.getByRole('status').filter({ hasText: 'Preview out of date' }).count(), 0);
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth + 1), true, 'mobile dialog has no horizontal overflow');
    await dialog.getByRole('button', { name: 'Close', exact: true }).focus();
    await page.keyboard.press('Shift+Tab');
    assert.equal(await dialog.evaluate(element => element.contains(document.activeElement)), true, 'focus remains in dialog');
    await page.keyboard.press('Escape');
    await dialog.waitFor({ state: 'detached' });
    await page.setViewportSize({ width: 1440, height: 1000 });
    console.log('PASS preview staleness, mobile dialog width, keyboard focus and Escape close');
    const panel = page.getByRole('region', { name: 'Background slicing jobs' });
    for (const label of ['Queued', 'Slicing and importing', 'Completed', 'Failed', 'Interrupted']) await text(panel, label);
    await panel.getByRole('button', { name: 'Cancel queued job', exact: true }).click();
    await visible(panel.getByText('Cancelled before starting', { exact: true }));
    await panel.locator('li').filter({ hasText: 'Smoke failed' }).getByRole('button', { name: 'Remove from history' }).click();
    await panel.locator('li').filter({ hasText: 'Smoke failed' }).waitFor({ state: 'detached' });
    console.log('PASS jobs status labels, queued cancellation and explicit terminal-history removal');
    await page.goto(`${base}/slicer`);
    await page.getByRole('button', { name: 'Edit', exact: true }).first().click();
    const editor = page.getByRole('dialog');
    await visible(editor.getByText('Recovery snapshots (1, latest 50, up to 4 MiB)', { exact: true }));
    const save = editor.getByRole('button', { name: 'Save', exact: true });
    assert.equal(await save.isDisabled(), true);
    await editor.locator('textarea').fill(JSON.stringify({ name: 'smoke', layer_height: '0.3' }));
    assert.equal(await save.isDisabled(), true);
    await editor.getByRole('button', { name: 'Validate', exact: true }).click();
    await visible(editor.getByText('Preflight checks passed. Review warnings before saving.', { exact: true }));
    await save.click(); await visible(editor.getByText('Saved and read back from slicer.', { exact: true }));
    await text(editor, 'Recovery snapshots (2'); assert.equal(await save.isDisabled(), true);
    conflict = true;
    const draft = JSON.stringify({ name: 'smoke', layer_height: '0.4' });
    await editor.locator('textarea').fill(draft); assert.equal(await save.isDisabled(), true);
    await editor.getByRole('button', { name: 'Validate', exact: true }).click();
    await visible(editor.getByText('Preflight checks passed. Review warnings before saving.', { exact: true }));
    await save.click(); await visible(editor.getByText('Profile changed externally; reload before saving', { exact: true }));
    assert.equal(await editor.locator('textarea').inputValue(), draft);
    assert.equal(JSON.parse(current).layer_height, '0.3');
    await editor.getByRole('button', { name: 'Reload', exact: true }).click();
    await visible(editor.getByText('No unsaved changes', { exact: true }));
    assert.equal(JSON.parse(await editor.locator('textarea').inputValue()).layer_height, '0.3');
    console.log('PASS editor Validate→Save gating, expected hash, history update, 409 preserves draft, explicit reload');
    assert.deepEqual(errors, []); assert.deepEqual(unexpected, []);
    console.log(`PASS no page errors or unmocked API writes (${calls.length} intercepted API requests)`);
  } catch (error) { console.error(await page.locator('body').innerText()); console.error({ errors, unexpected }); throw error; }
  finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
