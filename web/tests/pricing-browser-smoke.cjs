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
const gcode = { id: 'plate', file_id: 'plate-file', display_name: 'Two-piece fixture', file_name: 'fixture.gcode', filament_grams: 100, estimated_seconds: 3600, created_at: stamp, updated_at: stamp };
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
    page.on('dialog', dialog => dialog.accept());
    let hold = null;
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
      if (method !== 'GET') { report.unexpected.push(`${method} ${p}`); return route.abort(); }
      if (p === '/api/printers') return route.fulfill({ json: printers });
      if (p === '/api/materials') return route.fulfill({ json: materials });
      if (p === '/api/gcode-library') return route.fulfill({ json: { items: [gcode], total: 1, page: 1, page_size: 25 } });
      if (p.startsWith('/api/settings/')) return route.fulfill({ json: { value: p.endsWith('/currency_code') ? 'BRL' : '' } });
      // Layout reads are deliberately inert, never forwarded to backend/hardware.
      return route.fulfill({ json: [] });
    });
    const selectLabels = new Set(['Fee base', 'Fixed fee occurrence', 'Load printer rates', 'Load material rate', 'Production model', 'Machine rate model']);
    const label = key => selectLabels.has(key)
      ? page.getByLabel(new RegExp(`^${t(key).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`))
      : page.getByLabel(t(key), { exact: true });
    const fill = (key, value) => label(key).fill(value);
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
      assert.equal(await dt.locator('..').locator('dd').innerText(), localized);
    };
    try {
      await page.goto(`${base}/pricing`);
      await page.getByRole('heading', { name: t('Pricing calculator'), exact: true }).waitFor();
      await page.waitForFunction(name => [...document.querySelectorAll('select')].find(select => select.getAttribute('aria-label') === name)?.value === 'BRL', t('Currency'));
      assert.equal(await label('Currency').inputValue(), 'BRL');
      await label('Full mode').check();
      for (const [key, value] of Object.entries({ 'Requested units': '3', 'Grams per unit': '100', 'Hours per unit': '1', 'Material cost per kg': '10', 'Machine rate per hour': '0', 'Labor minutes per requested unit': '0', 'Labor rate per hour': '0', 'Fixed order reserve': '0', 'Shipping cost': '5', 'Shipping charged': '4' })) await fill(key, value);
      await label('Hourly rate already includes energy').check();
      await label('Hourly rate already includes maintenance').check();
      await summary('Sales fees');
      await button('Add fee').click();
      await fill('Fee percent', '10'); await fill('Fixed fee', '1');
      await label('Fee base').selectOption('total'); await label('Fixed fee occurrence').selectOption('unit');
      await check('exact totals, shipping fee base and all three last-edited anchors', async () => {
        for (const [key, value, anchor] of [['Goods price target', '20', 'price'], ['Gain target', '10,6', 'gain'], ['Margin target percent', '53', 'margin']]) {
          // Active-anchor text is nested inside its label; use prefix matching.
          await page.getByLabel(new RegExp(`^${t(key)}`)).fill(value);
          const last = await calculate();
          assert.equal(last.body.target.anchor, anchor);
          assert.equal(last.result.base.price, '20.00'); assert.equal(last.result.base.gain, '10.60');
          assert.equal(last.result.base.fees, '5.40'); assert.equal(last.result.minimum_sale_price, '8.23');
          await amount(scenario('Base scenario'), 'Goods total', '20.00');
          await amount(scenario('Base scenario'), 'Estimated gain', '10.60');
        }
      });
      await check('signed gain target is accepted without making costs negative', async () => {
        await page.getByLabel(new RegExp(`^${t('Gain target')}`)).fill('-1');
        const last = await calculate();
        assert.equal(last.body.target.anchor, 'gain'); assert.equal(last.body.target.value, '-1');
        assert.equal(last.result.base.price, '7.11'); assert.equal(last.result.base.gain, '-1.00');
        await amount(scenario('Base scenario'), 'Estimated gain', '-1.00');
        await page.getByLabel(new RegExp(`^${t('Margin target percent')}`)).fill('53');
        await calculate();
      });
      await check('quick/full retains hidden costs and fees', async () => {
        await label('Full mode').uncheck();
        const last = await calculate();
        assert.equal(last.body.shipping.cost, '5'); assert.equal(last.body.fees.length, 1);
        assert.equal(last.result.base.gain, '10.60');
        await label('Full mode').check(); assert.equal(await label('Shipping cost').inputValue(), '5');
      });
      await check('active anchor retains original precision across recalculations', async () => {
        const anchor = page.getByLabel(new RegExp(`^${t('Goods price target')}`));
        await anchor.fill('20.004'); await calculate();
        assert.equal(await anchor.inputValue(), '20.004');
        const last = await calculate(); assert.equal(last.body.target.value, '20.004');
        assert.equal(last.result.base.price, '20.00');
        await page.getByLabel(new RegExp(`^${t('Margin target percent')}`)).fill('53'); await calculate();
      });
      await check('configured printer currency cannot silently change after loading', async () => {
        await label('Load printer rates').selectOption('configured');
        assert.equal(await label('Machine rate per hour').inputValue(), '7.50');
        await label('Currency').selectOption('USD');
        const count = report.calls.length;
        await button('Calculate estimate').click(); await page.getByRole('alert').waitFor();
        assert.equal(report.calls.length, count, 'Mismatched configured currency must not reach backend');
        await label('Currency').selectOption('BRL');
      });
      await check('configured to legacy clears ancillary fields and included-cost flags', async () => {
        await label('Load printer rates').selectOption('legacy');
        assert.equal(await label('Hourly rate already includes energy').isChecked(), false);
        assert.equal(await label('Hourly rate already includes maintenance').isChecked(), false);
        for (const key of ['Maintenance per hour', 'Average power watts', 'Energy tariff per kWh']) assert.equal(await label(key).inputValue(), '', key);
        await page.locator('label').filter({ hasText: t('I confirm loaded amounts use {{currency}}. Legacy currency is unknown; no conversion.').replace('{{currency}}', 'BRL') }).getByRole('checkbox').check();
        await fill('Machine rate per hour', '0');
        await label('Hourly rate already includes energy').check();
        await label('Hourly rate already includes maintenance').check();
      });
      await check('conditional discount makes a visible loss', async () => {
        await summary('Conditional discount, for example Pix'); await fill('Conditional discount percent', '90');
        const last = await calculate();
        assert.equal(last.result.discounted.price, '2.00'); assert.equal(last.result.discounted.gain, '-5.60');
        await amount(scenario('Conditional discount scenario'), 'Estimated gain', '-5.60');
      });
      await check('legacy source requires currency acknowledgment and remains editable', async () => {
        const count = report.calls.length;
        await label('Load printer rates').selectOption('legacy');
        assert.equal(await label('Machine rate per hour').inputValue(), '2.50');
        await button('Calculate estimate').click(); await page.getByRole('alert').waitFor();
        assert.equal(report.calls.length, count, 'Unconfirmed legacy currency must not calculate');
        await page.locator('label').filter({ hasText: t('I confirm loaded amounts use {{currency}}. Legacy currency is unknown; no conversion.').replace('{{currency}}', 'BRL') }).getByRole('checkbox').check();
        await fill('Machine rate per hour', '0');
        await calculate();
      });
      await check('G-code copy count guard, conservative plate remainder and manual edits', async () => {
        await summary('Load G-code estimates'); await label('Choose G-code').selectOption('plate');
        assert.equal(await label('Grams per full plate').inputValue(), '100');
        assert.equal(Number(await label('Hours per full plate').inputValue()), 1);
        const count = report.calls.length;
        await button('Calculate estimate').click(); await page.getByRole('alert').waitFor();
        assert.equal(report.calls.length, count);
        await fill('Units per full plate', '2'); await fill('Requested units', '5');
        await label('I verified the mass, time and units per plate, or changed them to per-unit values.').check();
        await fill('Grams per full plate', '200');
        const last = await calculate(); assert.equal(last.result.effective_plates, '3'); assert.equal(last.result.total_grams, '600.000000');
        assert.ok(last.result.warnings.includes('full_batch_remainder'));
      });
      await check('editing invalidates a response already in flight', async () => {
        let started, release;
        const began = new Promise(resolve => { started = resolve; });
        hold = { started, release: new Promise(resolve => { release = resolve; }) };
        await button('Calculate estimate').click();
        await Promise.race([began, new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('Deferred calculate never reached backend')), 10000); timer.unref(); })]);
        await fill('Requested units', '6'); release(); hold = null;
        await page.getByRole('status').filter({ hasText: t('No current result. Calculate after editing to refresh the estimate.') }).waitFor();
        assert.equal(await page.getByRole('heading', { name: t('Estimate results'), exact: true }).count(), 0);
        const last = await calculate(); assert.equal(last.body.quantity, 6);
      });
      await check('desktop/mobile overflow and localized results', async () => {
        for (const width of [1440, 390]) {
          await page.setViewportSize({ width, height: 1000 });
          if (width === 390) {
            const collapseName = language === 'pt-BR' ? 'Recolher menu lateral' : 'Collapse sidebar';
            const collapse = page.getByRole('button', { name: collapseName, exact: true });
            if (await collapse.count()) await collapse.click();
            // No-overflow alone can hide an unusably narrow page behind navigation.
            // Wait only for the existing sidebar's bounded width transition.
            await page.waitForFunction(() => document.querySelector('main')?.getBoundingClientRect().width >= 320 && document.querySelector('aside')?.getBoundingClientRect().width <= 64.1, null, { timeout: 10000 });
            assert.ok(await page.locator('main').evaluate(el => el.getBoundingClientRect().width >= 320), 'Mobile calculator content must have at least 320px after collapsing navigation');
          }
          const dimensions = await page.locator('main').evaluate(el => ({ mainWidth: el.getBoundingClientRect().width, calculatorWidth: el.querySelector('h1')?.closest('header')?.parentElement?.getBoundingClientRect().width }));
          report.viewports.push({ language, viewportWidth: width, ...dimensions });
          console.log(`LAYOUT ${language} ${width}px: main=${dimensions.mainWidth}px calculator=${dimensions.calculatorWidth}px`);
          assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), true, `Horizontal overflow at ${width}px`);
          await page.screenshot({ path: path.join(artifacts, `${language}-${width}.png`), fullPage: true });
        }
        // Reveal translated warnings/breakdown too; collapsed details hide raw keys.
        await summary('Cost breakdown and assumptions');
        const warnings = page.locator('summary').filter({ hasText: t('Missing costs and estimate warnings') });
        if (await warnings.count()) {
          // The page reserves bottom clearance for the floating feedback action.
          await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
          await warnings.click();
        }
        const text = await page.locator('main').innerText();
        // English catalog intentionally uses labels such as "Cost: material".
        const rawKeys = language === 'pt-BR' ? /Warning: [a-z_]+|Cost: [a-z_]+|\{\{|pricing\.[a-z_]+/ : /Warning: [a-z_]+|\{\{|pricing\.[a-z_]+/;
        assert.ok(!rawKeys.test(text), 'Raw translation key or interpolation leaked');
        if (language === 'pt-BR') assert.ok(!text.includes('Pricing calculator'), 'Portuguese page fell back to English');
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
