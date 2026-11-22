import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
const source = readFileSync(new URL('../src/lib/pricingCalculator.ts', import.meta.url), 'utf8')
const context = { exports: {} }
vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, context)
const { syncPricingTargets, pricingCurrencyMatches, legacyPrinterDraft, formatPricingMoney, normalizePricingDecimal: decimal, pricingCount, catalogCentsToDecimal, catalogSecondsToHours, fullBatchRemainder } = context.exports

test('decimal editing is normalized only at submission without float conversion', () => {
  assert.equal(decimal(' 001,230000001 '), '1.230000001')
  assert.equal(decimal('0'), '0')
  assert.equal(decimal(''), '')
  assert.equal(decimal('1000000000'), '1000000000')
  for (const value of ['1,2,3', '1.2,3', '1e3', '-1', 'NaN', 'Infinity', '1.', '.5', '1 000', '1000000000.000000001', '1.1234567891']) assert.throws(() => decimal(value))
  assert.throws(() => decimal('', true))
})
test('signed targets are opt-in and response money never loses precision', () => {
  assert.equal(decimal('-1,25', true, true), '-1.25')
  assert.throws(() => decimal('-1,25', true))
  assert.equal(formatPricingMoney('900719925474099312345.67', 'USD', 'en-US'), '900,719,925,474,099,312,345.67 USD')
  assert.equal(formatPricingMoney('1234', 'JPY', 'en-US'), '1,234 JPY')
  assert.equal(formatPricingMoney('-0.50', 'BRL', 'pt-BR'), '−0,50 BRL')
})
test('counts and plate remainder are bounded integers', () => {
  assert.equal(pricingCount('1000000'), 1000000)
  for (const value of ['', '0', '1.5', '-2', '1000001', '1e2']) assert.throws(() => pricingCount(value))
  assert.equal(fullBatchRemainder('5', '2'), true)
  assert.equal(fullBatchRemainder('4', '2'), false)
  assert.equal(fullBatchRemainder('4', ''), false)
})
test('catalog conversion uses integer arithmetic and preserves missing data', () => {
  assert.equal(catalogCentsToDecimal(123), '1.23')
  assert.equal(catalogCentsToDecimal(0), '0.00')
  assert.equal(catalogCentsToDecimal(1.5), '')
  assert.equal(catalogSecondsToHours(3600), '1.000000000')
  assert.equal(catalogSecondsToHours(1), '0.000277778')
  assert.equal(catalogSecondsToHours(-1), '')
})
test('minimum-charge achieved gain never replaces requested active anchor', () => {
  const draft = { price: '', gain: '0', margin: '30', reserve: '3' }
  const achieved = { price: '20.00', gain: '17.00', margin_percent: '85.00' }
  const linked = syncPricingTargets(draft, 'gain', achieved)
  assert.equal(linked.gain, '0')
  assert.equal(linked.price, '20.00')
  assert.equal(linked.margin, '85.00')
  const edited = { ...linked, reserve: '4' }
  assert.equal(edited.gain, '0') // next request still targets zero, not achieved 17
  assert.equal(syncPricingTargets(draft, 'price', achieved).price, '')
  assert.equal(syncPricingTargets(draft, 'margin', achieved).margin, '30')
})
test('loaded printer currency cannot be relabeled by later currency edits', () => {
  assert.equal(pricingCurrencyMatches('USD', 'USD'), true)
  assert.equal(pricingCurrencyMatches('USD', 'EUR'), false)
  assert.equal(pricingCurrencyMatches('', 'EUR'), true) // separate legacy confirmation required
})
test('legacy printer replacement clears every ancillary printer cost', () => {
  const previous = { rate: '9', purchase: '1000', residual: '50', lifetime: '3000', maintenance: '2', watts: '150', tariff: '1', material: '80' }
  const next = { ...previous, ...legacyPrinterDraft(250) }
  assert.equal(next.rate, '2.50')
  assert.equal(next.material, '80')
  for (const key of ['purchase', 'residual', 'lifetime', 'maintenance', 'watts', 'tariff']) assert.equal(next[key], '')
})
test('screen keeps hidden costs, invalidates responses and does not store drafts', () => {
  const page = readFileSync(new URL('../src/pages/PricingCalculator.tsx', import.meta.url), 'utf8')
  assert.match(page, /hidden=\{!full\}/)
  assert.match(page, /generation\.current !== version/)
  assert.match(page, /controller\.current\?\.abort\(\)/)
  assert.match(page, /min_charge: d\('minimum'\)/)
  assert.match(page, /discount_percent: d\('discount'\)/)
  assert.doesNotMatch(page, /localStorage|sessionStorage|useBlocker|parseFloat/)
})
