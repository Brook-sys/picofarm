/* Isolated pricing browser smoke; never point this at a deployed instance.
 * Start a built frontend and a TEMPORARY backend with an empty database first.
 * BASE_URL=http://127.0.0.1:18084 API_BASE=http://127.0.0.1:18085 \
 * PLAYWRIGHT_MODULE=/tmp/opencode/pricing-browser/node_modules/playwright \
 * CHROMIUM_EXECUTABLE=/usr/bin/chromium node web/tests/pricing-browser-smoke.cjs
 * Only POST /api/pricing/calculate reaches API_BASE. All catalog GETs are fixtures.
 * Requires explicit ISOLATED_PRICING_BACKEND=1 acknowledgement. No quote writes.
 * Screenshots/report go outside the repository (ARTIFACT_DIR overrides the default).
 */
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

function localOrigin(value, name) {
  assert.ok(value, `${name} is required; use an isolated local server`);
  const url = new URL(value);
  assert.equal(url.protocol, 'http:', `${name} must use local HTTP`);
  assert.ok(['127.0.0.1', '[::1]'].includes(url.hostname), `${name} must use a loopback IP, not a production hostname`);
  assert.equal(url.username + url.password + url.search + url.hash, '');
  assert.equal(url.pathname, '/');
  return url.origin;
}
const base = localOrigin(process.env.BASE_URL, 'BASE_URL');
const api = localOrigin(process.env.API_BASE, 'API_BASE');
assert.equal(process.env.ISOLATED_PRICING_BACKEND, '1', 'Set ISOLATED_PRICING_BACKEND=1 only for a temporary backend without hardware');
const artifacts = path.resolve(process.env.ARTIFACT_DIR || '/tmp/opencode/pricing-browser-smoke');
assert.ok(!artifacts.startsWith(path.resolve(__dirname, '../..') + path.sep), 'Artifacts must be outside the repository');
fs.mkdirSync(artifacts, { recursive: true });
const stamp = '2026-01-01T00:00:00Z';
const printers = [{ id: 'legacy', name: 'Legacy fixture', cost_per_hour_cents: 250, status: 'offline', created_at: stamp, updated_at: stamp }];
printers.push({ ...printers[0], id: 'configured', name: 'Configured fixture', pricing_config: { currency: 'BRL', rate_mode: 'direct', direct_rate_per_hour: '7.50', purchase_price: '1000', residual_value: '100', productive_lifetime_hours: '300', maintenance_per_hour: '0.50', average_power_watts: '200', energy_tariff: '0.30', includes_energy: true, includes_maintenance: true } });
const materials = [{ id: 'pla', name: 'PLA fixture', type: 'filament', material_type: 'PLA', cost_per_kg: 10, color: '#ffffff' }];
const gcode = { id: 'plate', file_id: 'plate-file', display_name: 'Two-piece fixture', file_name: 'fixture.gcode', filament_grams: 100, estimated_seconds: 3600, parent_stl_id: 'model', default_for_stl: true, thumbnail_file_id: 'gcode-thumbnail', created_at: stamp, updated_at: stamp };
const missingGcode = { ...gcode, id: 'missing', file_id: 'missing-file', display_name: 'Missing metadata fixture', filament_grams: null, estimated_seconds: null, default_for_stl: false };
const rootGcode = { ...gcode, id: 'root', file_id: 'root-file', display_name: 'Root fixture', parent_stl_id: undefined, default_for_stl: false };
const stl = { id: 'model', file_id: 'model-file', display_name: 'Fixture model', file_name: 'fixture.stl', size_bytes: 1024, thumbnail_file_id: 'stl-thumbnail', gcodes: [gcode], created_at: stamp, updated_at: stamp };
const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64');
const report = { passed: [], calls: [], unexpected: [], errors: [], externalBlocked: [], viewports: [] };
let browser;
(async () => {
  browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || '/usr/bin/chromium', args: ['--no-sandbox'] });
  for (const language of ['en', 'pt-BR']) {
    const catalogFile = path.resolve(__dirname, `../src/i18n/locales/${language}/pricing.json`);
    assert.ok(fs.existsSync(catalogFile), `BLOCKED: pricing translation catalog is not available: ${language}`);
    const catalog = JSON.parse(fs.readFileSync(catalogFile, 'utf8'));
    const t = key => {
      assert.ok(catalog[key], `Missing ${language} pricing translation: ${key}`);
      return catalog[key];
    };
    const context = await browser.newContext({ locale: language === 'en' ? 'en-US' : language, serviceWorkers: 'block', viewport: { width: 1440, height: 1000 } });
    // HTTP routing does not intercept WebSocket handshakes. Never connect them.
    assert.equal(typeof context.routeWebSocket, 'function', 'Playwright >= 1.48 is required for WebSocket isolation');
    await context.routeWebSocket('**/*', socket => socket.close());
    await context.addInitScript(lang => localStorage.setItem('picofarm.language', lang), language);
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    page.on('pageerror', error => report.errors.push(`${language}: ${error.message}`));
    let dismissNextDialog = false;
    const dialogs = [];
    page.on('dialog', async dialog => {
      dialogs.push(dialog.message());
      if (dismissNextDialog) { dismissNextDialog = false; await dialog.dismiss(); }
      else await dialog.accept();
    });
    let hold = null;
    let libraryHold = null;
    let sliced = false;
    const slicedGcode = { ...gcode, id: 'new-plate', file_id: 'new-plate-file', display_name: 'New sliced fixture', filament_grams: 80, estimated_seconds: 1800 };
    const slicingJob = () => ({ id: 'job-1', status: sliced ? 'succeeded' : 'queued', request: { stl_file_id: stl.id, display_name: slicedGcode.display_name }, result: sliced ? { gcode: { id: slicedGcode.id, display_name: slicedGcode.display_name } } : undefined, idempotency_key: 'fixture-key', created_at: stamp, updated_at: stamp });
    await context.route('**/*', async route => {
      const request = route.request(), url = new URL(request.url()), p = url.pathname, method = request.method();
      if (url.origin !== base) { report.externalBlocked.push(`${method} ${url.origin}${p}`); return route.abort(); }
      if (!p.startsWith('/api/')) {
        if (method === 'GET' && !['xhr', 'fetch', 'eventsource'].includes(request.resourceType())) return route.continue();
        report.unexpected.push(`${method} ${p}`); return route.abort();
      }
      if (method === 'POST' && p === '/api/pricing/calculate') {
        const entry = { language, body: request.postDataJSON() };
        report.calls.push(entry);
        const gate = hold;
        try {
          const response = await context.request.post(`${api}/api/pricing/calculate`, { data: entry.body, maxRedirects: 0, timeout: 10000 });
          assert.equal(response.status(), 200, await response.text());
          entry.result = await response.json();
          if (gate) { gate.started(); await gate.release; }
          // Editing deliberately aborts the in-flight fetch in the stale-result case.
          await route.fulfill({ response }).catch(error => {
            if (!gate) throw error;
          });
        } catch (error) {
          report.errors.push(`calculate proxy: ${error.message}`);
          await route.abort().catch(() => {});
        }
        return;
      }
      if (method === 'POST' && p === '/api/slicer/jobs') {
        const body = request.postDataJSON();
        assert.equal(body.stl_file_id, stl.id);
        sliced = true;
        report.slicingCalls = (report.slicingCalls || 0) + 1;
        return route.fulfill({ status: 202, json: { ...slicingJob(), status: 'queued', result: undefined } });
      }
      if (method !== 'GET') { report.unexpected.push(`${method} ${p}`); return route.abort(); }
      if (p === '/api/printers') return route.fulfill({ json: printers });
      if (p === '/api/materials') return route.fulfill({ json: materials });
      if (p === '/api/file-library') {
         const gate = libraryHold;
         if (gate) { gate.started(); await gate.release; }
         return route.fulfill({ json: { stl_files: [{ ...stl, gcodes: sliced ? [gcode, missingGcode, slicedGcode] : [gcode, missingGcode] }], root_gcode_files: [rootGcode] } }).catch(error => { if (!gate) throw error; });
       }
      if (p === '/api/slicer/jobs') return route.fulfill({ json: sliced ? [slicingJob()] : [] });
      if (p === '/api/slicer/config') return route.fulfill({ json: { connection_url: '', default_profiles: { printers: 'Printer fixture', presets: 'Process fixture', filaments: 'Filament fixture' } } });
      if (p === '/api/slicer/profiles/printers') return route.fulfill({ json: [{ name: 'Printer fixture', default: true }] });
      if (p === '/api/slicer/profiles/presets') return route.fulfill({ json: [{ name: 'Process fixture', default: true }] });
      if (p === '/api/slicer/profiles/filaments') return route.fulfill({ json: [{ name: 'Filament fixture', default: true }] });
      if (p === '/api/gcode-library') return route.fulfill({ json: { items: [gcode], total: 1, page: 1, page_size: 25 } });
      if (p === '/api/stl-library') return route.fulfill({ json: { items: [{ ...stl, gcodes: undefined }], total: 1, page: 1, page_size: 25 } });
      if (p === '/api/files/stl-thumbnail' || p === '/api/files/gcode-thumbnail') return route.fulfill({ contentType: 'image/png', body: pixel });
      if (p.startsWith('/api/settings/')) return route.fulfill({ json: { value: p.endsWith('/currency_code') ? 'BRL' : '' } });
      // Layout reads are deliberately inert, never forwarded to backend/hardware.
      return route.fulfill({ json: [] });
    });
    const selectLabels = new Set(['Fee base', 'Fixed fee occurrence', 'Load printer rates', 'Load material rate', 'Production model', 'Machine rate model', 'Cost occurrence']);
    const label = key => selectLabels.has(key)
      ? page.getByLabel(new RegExp(`^${t(key).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`))
      : page.getByLabel(t(key), { exact: true });
    const fill = (key, value) => label(key).fill(value);
    const target = async (key, value) => {
      await button(key === 'Goods price target' ? 'Price' : key === 'Gain target' ? 'Gain' : 'Margin').click();
      assert.equal(await page.locator('#pricing-price, #pricing-gain, #pricing-margin').count(), 1, 'Exactly one target input');
      await fill(key, value);
    };
    const confirmCatalog = () => page.locator('label').filter({ hasText: t('I confirm loaded amounts use {{currency}}. Legacy currency is unknown; no conversion.').replace('{{currency}}', 'BRL') }).getByRole('checkbox').check();
    const button = key => page.getByRole('button', { name: t(key), exact: true });
    const summary = key => page.locator('summary').filter({ hasText: t(key) }).click();
    const check = async (name, action) => { await action(); report.passed.push(`${language}: ${name}`); console.log(`PASS ${language}: ${name}`); };
    const calculate = async () => {
      await button('Calculate estimate').click();
      await page.getByRole('heading', { name: t('Estimate results'), exact: true }).waitFor();
      const last = report.calls.at(-1);
      assert.ok(last?.result, 'A real isolated backend response is required');
      return last;
    };
    const scenario = key => page.locator('section').filter({ has: page.getByRole('heading', { name: t(key), exact: true }) }).last();
    const amount = async (section, key, expected) => {
      const dt = section.locator('dt').filter({ hasText: new RegExp(`^${t(key).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`) });
      // Independent fixture oracle: no floating point or shared production formatter.
      const [whole, fraction] = expected.replace(/^-/, '').split('.');
      const group = language === 'pt-BR' ? '.' : ',';
      const decimal = language === 'pt-BR' ? ',' : '.';
      const localized = `${expected.startsWith('-') ? '−' : ''}${whole.replace(/\B(?=(\d{3})+(?!\d))/g, group)}${fraction === undefined ? '' : decimal + fraction} BRL`;
      assert.equal(await dt.locator('xpath=following-sibling::dd[1]').innerText(), localized);
    };
    try {
      await page.goto(`${base}/pricing`);
      await page.getByRole('heading', { name: t('Pricing calculator'), exact: true }).waitFor();
      await page.waitForFunction(() => document.querySelector('#pricing-currency')?.value === 'BRL');
      assert.equal(await label('Currency').inputValue(), 'BRL');
      const snapshots = async phase => {
        for (const width of [1440, 390]) {
          await page.setViewportSize({ width, height: 1000 });
          if (width === 390) {
            const collapse = page.getByRole('button', { name: language === 'pt-BR' ? 'Recolher menu lateral' : 'Collapse sidebar', exact: true });
            if (await collapse.isVisible()) await collapse.click();
            await page.waitForFunction(() => document.querySelector('main')?.getBoundingClientRect().width >= 320);
          }
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `Overflow: ${phase} ${width}`);
          const duplicates = await page.locator('main [id]').evaluateAll(elements => {
            const seen = new Set();
            return elements.map(el => el.id).filter(id => seen.has(id) || !seen.add(id));
          });
          assert.deepEqual(duplicates, []);
          const clipped = await page.locator('main label, main summary, main button').evaluateAll(elements => elements.filter(el => {
            if (!el.getClientRects().length) return false;
            const style = getComputedStyle(el);
            return style.textOverflow === 'ellipsis' || (el.clientWidth > 0 && el.scrollWidth > el.clientWidth + 1 && ['hidden', 'clip'].includes(style.overflowX));
          }).map(el => el.textContent.trim()));
          assert.deepEqual(clipped, [], `Clipped labels: ${phase} ${width}`);
          report.viewports.push({ language, width, phase, mainWidth: await page.locator('main').evaluate(el => el.getBoundingClientRect().width) });
          // Layout scrolls <main>, so ordinary fullPage screenshots clip results.
          // Expand only for capture, then restore the exact original inline styles.
          const captureStyles = await page.locator('main').evaluate(main => {
            const saved = [];
            for (let el = main; el; el = el.parentElement) {
              saved.push({ tag: el.tagName, id: el.id, style: el.getAttribute('style') });
              el.style.height = 'auto'; el.style.maxHeight = 'none'; el.style.overflow = 'visible';
            }
            return saved;
          });
          await page.screenshot({ path: path.join(artifacts, `${language}-${width}-${phase}.png`), fullPage: true });
          await page.locator('main').evaluate((main, saved) => {
            let el = main;
            for (const entry of saved) {
              if (entry.style === null) el.removeAttribute('style'); else el.setAttribute('style', entry.style);
              el = el.parentElement;
            }
          }, captureStyles);
        }
        await page.setViewportSize({ width: 1440, height: 1000 });
      };
      const card = name => page.getByRole('button').filter({ hasText: name });
      const showEstimates = async () => {
        const item = page.locator('summary').filter({ hasText: t('Edit file estimates') });
        if (!(await item.locator('..').evaluate(el => el.open))) await item.click();
      };
      const rejectCalculate = async () => {
        const before = report.calls.length;
        if (await button('Calculate estimate').isEnabled()) {
          await button('Calculate estimate').click(); await page.getByRole('alert').waitFor();
        }
        assert.equal(report.calls.length, before, 'Invalid input must not reach backend');
      };
      await check('default ready list includes associated and root G-codes; visible editable margin 30%', async () => {
        for (const name of ['Two-piece fixture', 'Missing metadata fixture', 'Root fixture']) await card(name).waitFor();
        assert.equal(await label('Margin target percent').inputValue(), '30');
        assert.ok(await label('Margin target percent').isVisible());
        assert.equal(await page.locator('#pricing-quantity:visible, #pricing-units:visible').count(), 0);
        assert.equal(await page.locator('main input[type=checkbox]:visible').count(), 0);
        await snapshots('before');
      });
      await check('basic whole-file journey requires only G-code and price/kg; exact real financial result', async () => {
        await card('Two-piece fixture').click(); await fill('Material cost per kg', '10');
        const last = await calculate();
        assert.equal(last.body.quantity, 1);
        assert.deepEqual(last.body.production, { mode: 'batches', units_per_batch: 1, grams: '100', hours: '1.000000000' });
        assert.deepEqual(last.body.target, { anchor: 'margin', value: '30' });
        assert.equal(last.result.total_grams, '100.000000'); assert.equal(last.result.effective_plates, '1');
        assert.equal(last.result.base.price, '1.43'); assert.equal(last.result.base.cost_total, '1.00'); assert.equal(last.result.base.gain, '0.43');
        await amount(scenario('Base scenario'), 'Suggested price for this print', '1.43');
        await amount(scenario('Base scenario'), 'Included costs and fees', '1.00');
        await amount(scenario('Base scenario'), 'Surplus over included costs', '0.43');
        assert.ok(await page.getByText(t('For the whole print file; no piece count assumed.'), { exact: true }).isVisible());
        assert.equal(await page.getByText(t('Machine hourly rate is missing.'), { exact: true }).isVisible(), false);
        await snapshots('basic');
      });
      await check('missing metadata allows manual fallback without piece-count acknowledgment', async () => {
        await button('Change file').click(); await card('Missing metadata fixture').click();
        assert.equal(await label('Grams for the whole print').inputValue(), '');
        await rejectCalculate(); await fill('Grams for the whole print', '100');
        assert.equal((await calculate()).result.base.price, '1.43');
        await button('Change file').click(); await card('Two-piece fixture').click();
      });
      await check('opening full/disclosures never changes payload and collapsing never drops costs', async () => {
        const before = await calculate(); await button('Full mode').click();
        assert.deepEqual((await calculate()).body, before.body);
        for (const [key, value] of Object.entries({ 'Machine rate per hour': '2', 'Maintenance per hour': '0.5', 'Average power watts': '100', 'Energy tariff per kWh': '0.5', 'Labor minutes for the whole print': '6', 'Labor rate per hour': '10', 'Fixed order reserve': '1', 'Shipping cost': '5', 'Shipping charged': '4' })) await fill(key, value);
        await button('Add fee').click(); await fill('Fee percent', '10'); await fill('Fixed fee', '1');
        await label('Fee base').selectOption('total'); await target('Goods price target', '20,004');
        const fullResult = await calculate();
        assert.equal(fullResult.body.target.value, '20.004'); assert.equal(fullResult.result.base.price, '20.00');
        assert.equal(fullResult.result.base.fees, '3.40'); assert.equal(fullResult.result.base.cost_total, '13.95'); assert.equal(fullResult.result.base.gain, '10.05');
        await amount(scenario('Base scenario'), 'Surplus over included costs', '10.05');
        await snapshots('optional'); await button('Quick mode').click();
        const collapsed = await calculate(); assert.deepEqual(collapsed.body, fullResult.body);
        assert.equal(collapsed.result.base.gain, '10.05');
        assert.equal(await label('Goods price target').inputValue(), '20,004');
        await button('Full mode').click(); assert.equal(await label('Shipping cost').inputValue(), '5'); await button('Quick mode').click();
      });
      await check('piece pricing requires explicit counts and verification; whole-file draft restores costs', async () => {
        const whole = await calculate(); await button('Price individual pieces or batches').click();
        assert.equal(await label('Requested units').inputValue(), ''); assert.equal(await label('Units per full plate').inputValue(), '');
        await rejectCalculate(); await fill('Requested units', '5'); await fill('Units per full plate', '2'); await rejectCalculate();
        await label('I verified the mass, time and units per plate, or changed them to per-unit values.').check();
        const piece = await calculate(); assert.equal(piece.body.quantity, 5); assert.equal(piece.result.effective_plates, '3'); assert.equal(piece.result.total_grams, '300.000000');
        assert.ok(piece.result.warnings.includes('full_batch_remainder'));
        await label('Production model').selectOption('proportional'); await rejectCalculate(); await button('Keep and reinterpret values').click();
        assert.equal(await label('Grams per unit').inputValue(), '100');
        await label('Production model').selectOption('batches'); await button('Clear mass and time').click(); assert.equal(await label('Grams per full plate').inputValue(), '');
        await button('Price one whole print run instead').click(); assert.deepEqual((await calculate()).body, whole.body);
      });
      await check('slicing is explicit; opening/closing never submits; result stays whole-file', async () => {
        const library = JSON.parse(fs.readFileSync(path.resolve(__dirname, `../src/i18n/locales/${language}/library.json`), 'utf8'));
        await button('Change file').click(); await button('STL models').click(); await card('Fixture model').click();
        const before = report.slicingCalls || 0, calculationsBefore = report.calls.length;
        await button('Slice a new G-code').click(); const dialog = page.getByRole('dialog'); await dialog.waitFor(); assert.equal(report.slicingCalls || 0, before);
        await dialog.getByRole('button', { name: library.Close, exact: true }).click(); await dialog.waitFor({ state: 'hidden' });
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.equal(report.calls.length, calculationsBefore); assert.equal(report.slicingCalls || 0, before);
        await button('Slice a new G-code').click(); await dialog.waitFor();
        await dialog.getByRole('button', { name: library['Generate in background'], exact: true }).click(); await dialog.waitFor({ state: 'hidden' }); assert.equal(report.slicingCalls, before + 1);
        await button('Use in estimate').click(); await showEstimates();
        assert.equal(await label('Grams for the whole print').inputValue(), '80'); assert.equal(await label('Duration for the whole print').inputValue(), '0:30');
        const last = await calculate(); assert.equal(last.body.quantity, 1); assert.equal(last.body.production.units_per_batch, 1);
      });
      await check('editing invalidates an already in-flight real response', async () => {
        let started, release; const began = new Promise(resolve => { started = resolve; });
        hold = { started, release: new Promise(resolve => { release = resolve; }) };
        await button('Calculate estimate').click();
        await Promise.race([began, new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('Deferred calculate never reached backend')), 10000); timer.unref(); })]);
        await fill('Grams for the whole print', '90'); release(); hold = null;
        await page.getByRole('status').filter({ hasText: t('Your estimate will appear here.') }).waitFor();
        assert.equal(await page.getByRole('heading', { name: t('Estimate results'), exact: true }).count(), 0);
        assert.equal((await calculate()).body.production.grams, '90');
      });
      await check('scope reinterpretation warns about labor/extras/fixed fees and cancel preserves payload', async () => {
        await button('Full mode').click();
        await fill('Labor minutes for the whole print', '60');
        await button('Add extra cost').click(); await fill('Cost label', 'Scope audit'); await fill('Amount', '10');
        await page.locator('select[id^="extra-"][id$="-scope"]').selectOption('unit');
        await label('Fixed fee occurrence').selectOption('unit');
        const whole = await calculate();
        const beforeDialogs = dialogs.length; dismissNextDialog = true;
        await button('Price individual pieces or batches').click();
        assert.equal(dialogs.length, beforeDialogs + 1);
        assert.ok(dialogs.at(-1).includes(t('Labor minutes, per-unit extras and per-unit fixed fees will now apply to EACH requested piece, not the whole print. Their values are kept and may multiply your costs. Continue and review them?')));
        assert.equal(await page.locator('#pricing-quantity').count(), 0);
        assert.deepEqual((await calculate()).body, whole.body);
        await button('Price individual pieces or batches').click();
        await fill('Requested units', '2'); await fill('Units per full plate', '1');
        await label('I verified the mass, time and units per plate, or changed them to per-unit values.').check();
        const pieces = await calculate();
        assert.equal(pieces.body.labor.minutes, '60'); assert.equal(pieces.body.extras[0].scope, 'unit');
        assert.equal(pieces.result.breakdown.labor, '20.00'); assert.equal(pieces.result.breakdown.extras, '20.00');
        await button('Price one whole print run instead').click();
        assert.deepEqual((await calculate()).body, whole.body);
      });
      await check('included maintenance preserves hidden value without requiring missing duration', async () => {
        await button('Change file').click(); await button('Ready G-codes').click(); await card('Missing metadata fixture').click();
        await fill('Grams for the whole print', '100');
        await fill('Machine rate per hour', '0'); await fill('Maintenance per hour', '5');
        await fill('Average power watts', ''); await fill('Energy tariff per kWh', '');
        assert.equal(await button('Calculate estimate').isEnabled(), false, 'Active maintenance requires duration');
        await label('Hourly rate already includes maintenance').check();
        assert.ok(await button('Calculate estimate').isEnabled(), 'Excluded maintenance must not require time');
        const last = await calculate(); assert.equal(last.result.breakdown.maintenance, '0.00');
        assert.equal(last.body.machine.includes_maintenance, true);
        assert.equal(last.body.machine.maintenance_hourly, undefined);
        await label('Hourly rate already includes maintenance').uncheck();
        assert.equal(await label('Maintenance per hour').inputValue(), '5');
        await label('Hourly rate already includes maintenance').check();
        await calculate();
      });
      await check('deferred slicing library selection cannot cross a financial scope change', async () => {
        await button('Change file').click(); await button('STL models').click(); await card('Fixture model').click();
        await button('Show slicing jobs').click();
        let started, release;
        const began = new Promise(resolve => { started = resolve; });
        libraryHold = { started, release: new Promise(resolve => { release = resolve; }) };
        await button('Use in estimate').click();
        await Promise.race([began, new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('Deferred library fetch never started')), 10000); timer.unref(); })]);
        await button('Price individual pieces or batches').click();
        await fill('Requested units', '7'); await fill('Units per full plate', '2');
        // Subscribe before releasing to avoid racing a fast loopback response.
        const refreshed = page.waitForResponse(response => new URL(response.url()).pathname === '/api/file-library');
        release(); libraryHold = null;
        await refreshed;
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.equal(await label('Requested units').inputValue(), '7');
        assert.equal(await label('Units per full plate').inputValue(), '2');
        assert.equal(await label('Grams per full plate').inputValue(), '100', 'Old sliced 80g result must not load after scope change');
        await button('Price one whole print run instead').click(); await calculate();
      });
      await check('localized details contain no raw keys or interpolations', async () => {
        await button('Full mode').click();
        for (const key of ['Cost breakdown and assumptions', 'Estimate assumptions and warnings']) {
          const item = page.locator('summary').filter({ hasText: t(key) });
          if (await item.count() && !(await item.locator('..').evaluate(el => el.open))) await item.click();
        }
        const text = await page.locator('main').innerText(); assert.ok(!/Warning: [a-z_]+|\{\{|pricing\.[a-z_]+/.test(text));
        if (language === 'pt-BR') assert.ok(!/Pricing calculator|Suggested price for this print|Surplus over included costs|Optional costs/.test(text), 'Portuguese must not fall back to English');
      });
    } catch (error) {
      await page.screenshot({ path: path.join(artifacts, `${language}-failure.png`), fullPage: true }).catch(() => {});
      fs.writeFileSync(path.join(artifacts, `${language}-failure.txt`), await page.locator('body').innerText().catch(() => 'No document'));
      throw error;
    } finally { await context.close(); }
  }
  assert.deepEqual(report.unexpected, [], 'Unexpected mutation/network request');
  assert.deepEqual(report.errors, [], 'Browser/proxy errors');
})().catch(error => { report.failure = error.stack; console.error(error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
  fs.writeFileSync(path.join(artifacts, 'report.json'), JSON.stringify(report, null, 2));
  console.log(`Artifacts: ${artifacts}`);
});
