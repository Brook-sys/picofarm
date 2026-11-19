export const LANGUAGE_STORAGE_KEY = 'picofarm.language'
export const LANGUAGES = [
  { code: 'en', label: 'English' },
  { code: 'pt-BR', label: 'Português (Brasil)' },
] as const
export type Language = typeof LANGUAGES[number]['code']

export function normalizeLanguage(value?: string | null): Language {
  const normalized = value?.trim().replaceAll('_', '-').toLowerCase()
  return normalized === 'pt' || normalized?.startsWith('pt-') ? 'pt-BR' : 'en'
}

export function detectLanguage(): Language {
  try {
    const stored = localStorage.getItem(LANGUAGE_STORAGE_KEY)
    if (stored) return normalizeLanguage(stored)
  } catch { /* Storage may be blocked; the session still works. */ }
  return normalizeLanguage(typeof navigator === 'undefined' ? undefined : navigator.languages?.[0] || navigator.language)
}

export function persistLanguage(language: string): void {
  const normalized = normalizeLanguage(language)
  try { localStorage.setItem(LANGUAGE_STORAGE_KEY, normalized) } catch { /* Per-session fallback. */ }
  if (typeof document !== 'undefined') document.documentElement.lang = normalized
}
