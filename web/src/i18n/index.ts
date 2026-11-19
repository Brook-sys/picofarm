import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { detectLanguage, normalizeLanguage, persistLanguage } from './locale'
export { LANGUAGES, LANGUAGE_STORAGE_KEY, normalizeLanguage } from './locale'

const catalogs = import.meta.glob<Record<string, string>>('./locales/*/*.json', { eager: true, import: 'default' })
const resources: Record<string, Record<string, Record<string, string>>> = {}
for (const [path, catalog] of Object.entries(catalogs)) {
  const [, language, namespace] = path.match(/\/locales\/([^/]+)\/([^/]+)\.json$/) || []
  if (language && namespace) (resources[language] ??= {})[namespace] = catalog
}

void i18n.use(initReactI18next).init({
  resources,
  lng: detectLanguage(),
  fallbackLng: 'en',
  supportedLngs: ['en', 'pt-BR'],
  defaultNS: 'common',
  keySeparator: false,
  nsSeparator: false,
  initAsync: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
})
i18n.on('languageChanged', persistLanguage)
// Initial detection updates the document, but is not an explicit saved preference.
if (typeof document !== 'undefined') document.documentElement.lang = normalizeLanguage(i18n.language)

export function getLocale(): string {
  return normalizeLanguage(i18n.resolvedLanguage || i18n.language)
}
export default i18n
