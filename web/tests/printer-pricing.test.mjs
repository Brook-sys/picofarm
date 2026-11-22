import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import ts from 'typescript'

const source = readFileSync(new URL('../src/components/PrinterPricingSettings.tsx', import.meta.url), 'utf8')

test('printer profile is optional, starts without currency, and is independent of legacy cents', () => {
  assert.match(source, /<details/)
  assert.match(source, /<summary/)
  assert.match(source, /currency: ''/)
  assert.match(source, /select required value=\{config.currency\}/)
  assert.doesNotMatch(source, /useCurrency|cost_per_hour_cents|purchase_price_cents/)
  assert.match(source, /pricing_config: pricingPayload\(config\)/)
  assert.match(source, /pricing_config: null/)
  assert.match(source, /No profile means unknown costs/)
  assert.match(source, /<details className="mt-3/)
})

test('profile form uses labeled inputs, explicit includes flags, and localized feedback', () => {
  assert.match(source, /<label/)
  assert.match(source, /includes_energy: false, includes_maintenance: false/)
  assert.match(source, /disabled=\{config.rate_mode === 'derived'\}/)
  assert.match(source, /role="alert"/)
  assert.match(source, /role="status"/)
  const keys = ['Pricing (optional)', 'Pricing currency', 'Save pricing', 'Clear pricing', 'Direct hourly rate', 'Pricing purchase value', 'Residual value', 'Productive lifetime (hours)', 'Maintenance per hour', 'Average power (W)', 'Energy tariff (per kWh)']
  for (const locale of ['en', 'pt-BR']) {
    const catalog = JSON.parse(readFileSync(new URL(`../src/i18n/locales/${locale}/fleet.json`, import.meta.url), 'utf8'))
    for (const key of keys) assert.ok(catalog[key], `${locale}: ${key}`)
  }
})

// Execute the component's pure payload/visibility helpers, rather than duplicate
// their logic in the test. React rendering/browser coverage is a separate gate.
const helpers = source.slice(source.indexOf('const decimalFields'), source.indexOf('export default function'))
const compiled = ts.transpile(helpers, { target: ts.ScriptTarget.ES2022 })
const { pricingPayload, visiblePricingField } = new Function(`${compiled}; return { pricingPayload, visiblePricingField }`)()
const draft = {
  currency: 'BRL', rate_mode: 'direct', direct_rate_per_hour: '1,25',
  purchase_price: '2000', residual_value: '0', productive_lifetime_hours: '10000',
  maintenance_per_hour: '0,50', average_power_watts: '150,25', energy_tariff: '0,90',
  includes_energy: false, includes_maintenance: false,
}

test('payload normalizes comma decimals and preserves inactive mode values without mutating draft', () => {
  const result = pricingPayload(draft)
  assert.equal(result.direct_rate_per_hour, '1.25')
  assert.equal(result.energy_tariff, '0.90')
  assert.equal(result.maintenance_per_hour, '0.50')
  assert.equal(result.purchase_price, '2000')
  assert.equal(result.residual_value, '0')
  assert.equal(result.productive_lifetime_hours, '10000')
  assert.equal(draft.purchase_price, '2000')
  assert.equal(draft.direct_rate_per_hour, '1,25')
  assert.equal(visiblePricingField(draft, 'purchase_price'), false)
  const derived = { ...draft, rate_mode: 'derived' }
  assert.equal(visiblePricingField(derived, 'direct_rate_per_hour'), false)
  assert.equal(visiblePricingField(derived, 'residual_value'), true)
  assert.equal(pricingPayload(derived).direct_rate_per_hour, '1.25')
  assert.equal(pricingPayload(derived).residual_value, '0')
})

test('included energy and maintenance are hidden but preserved in stored profile and draft', () => {
  const included = { ...draft, includes_energy: true, includes_maintenance: true }
  for (const field of ['maintenance_per_hour', 'average_power_watts', 'energy_tariff']) {
    assert.equal(visiblePricingField(included, field), false)
    assert.equal(pricingPayload(included)[field], draft[field].replace(',', '.'))
    assert.equal(included[field], draft[field])
    assert.equal(visiblePricingField(draft, field), true)
  }
})

test('edits invalidate saved feedback and register a dirty unload warning', () => {
  assert.match(source, /const edit = [\s\S]*?setSaved\(false\)[\s\S]*?setDirty\(true\)/)
  assert.match(source, /addEventListener\('beforeunload', warn\)/)
  assert.match(source, /removeEventListener\('beforeunload', warn\)/)
  assert.match(source, /pattern="[^"\n]*\[\.,\]/)
  assert.match(source, /decimalFields.filter\(\(\[field\]\) => visiblePricingField\(config, field\)\)/)
})
