import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

// Formatting must follow the interface locale without converting amounts or
// rewriting the configured currency code. Uses the real module with a stub i18n.
function load(locale = 'en') {
  const source = readFileSync(new URL('../src/lib/currency.ts', import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
  const scope = { exports: {}, console, Intl, require: () => ({ getLocale: () => locale }) }
  vm.runInNewContext(outputText, scope)
  return scope.exports
}

// ICU versions differ on whether the currency/number separator is a no-break
// space, so compare normalized text rather than raw whitespace.
const norm = value => value.replace(/[\u00a0\u202f]/g, ' ')

test('same amount and currency format differently per locale, without conversion', () => {
  const en = load('en'), pt = load('pt-BR')
  assert.equal(norm(en.formatMoney(1234.56, 'BRL')), 'R$1,234.56')
  assert.equal(norm(pt.formatMoney(1234.56, 'BRL')), 'R$ 1.234,56')
  assert.equal(norm(en.formatMoney(1234.56, 'USD')), '$1,234.56')
  assert.equal(norm(pt.formatMoney(1234.56, 'USD')), 'US$ 1.234,56')
  // Brazilian Real must not become US dollars because the UI is in English.
  assert.match(en.formatMoneyCents(123456, 'BRL'), /^R\$/)
  assert.equal(en.currencySymbol('BRL'), 'R$')
  assert.equal(pt.currencySymbol('BRL'), 'R$')
})

test('invalid currency codes and non-finite amounts fall back instead of throwing', () => {
  for (const locale of ['en', 'pt-BR']) {
    const api = load(locale)
    assert.match(api.formatMoney(10, 'not-a-currency'), /\$/)
    assert.match(api.formatMoney(10, ''), /\$/)
    assert.match(api.formatMoney(Number.NaN, 'USD'), /0\.00|0,00/)
    assert.equal(api.normalizeCurrencyCode('brl'), 'BRL')
    assert.equal(api.normalizeCurrencyCode('nope'), 'USD')
    assert.equal(api.formatDateTime('not-a-date'), '—')
  }
})

test('an explicit locale argument overrides the current interface language', () => {
  const api = load('en')
  assert.equal(norm(api.formatMoney(1234.56, 'BRL', 'pt-BR')), 'R$ 1.234,56')
  assert.equal(norm(api.formatMoney(1234.56, 'BRL', 'en')), 'R$1,234.56')
})
