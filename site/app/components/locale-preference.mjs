export const LOCALE_STORAGE_KEY = "picofarm-site-locale";

/** Browser language is only the default; a valid manual preference takes priority. */
export function resolveLocale(language, saved) {
  if (saved === "pt-BR" || saved === "en") return saved;
  return typeof language === "string" && /^pt(?:-|$)/i.test(language) ? "pt-BR" : "en";
}
