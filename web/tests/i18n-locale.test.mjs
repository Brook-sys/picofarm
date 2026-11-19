import test from 'node:test'
import assert from 'node:assert/strict'
import { detectLanguage, normalizeLanguage, persistLanguage, LANGUAGE_STORAGE_KEY } from '../src/i18n/locale.ts'

test('language resolution supports regional variants and safe English fallback', () => {
  for (const value of ['pt', 'pt-BR', 'pt-PT', 'PT_br']) assert.equal(normalizeLanguage(value), 'pt-BR')
  for (const value of [undefined, null, '', 'en-US', 'fr', 'invalid']) assert.equal(normalizeLanguage(value), 'en')
})

test('browser preference overrides navigator and document reflects changes', () => {
  const descriptors = Object.fromEntries(['localStorage', 'navigator', 'document'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]))
  const store = new Map()
  try {
    Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: { getItem: key => store.get(key), setItem: (key, value) => store.set(key, value) } })
    Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { languages: ['pt-BR'], language: 'en' } })
    Object.defineProperty(globalThis, 'document', { configurable: true, value: { documentElement: { lang: '' } } })
    assert.equal(detectLanguage(), 'pt-BR')
    persistLanguage('en')
    assert.equal(store.get(LANGUAGE_STORAGE_KEY), 'en')
    assert.equal(detectLanguage(), 'en')
    assert.equal(document.documentElement.lang, 'en')
    Object.defineProperty(globalThis, 'localStorage', { configurable: true, get() { throw new Error('blocked') } })
    assert.equal(detectLanguage(), 'pt-BR')
    assert.doesNotThrow(() => persistLanguage('pt-BR'))
    assert.equal(document.documentElement.lang, 'pt-BR')
  } finally {
    for (const [key, descriptor] of Object.entries(descriptors)) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor)
      else delete globalThis[key]
    }
  }
})
