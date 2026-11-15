"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

import { LOCALE_STORAGE_KEY, resolveLocale } from "./locale-preference.mjs";

export type Locale = "pt-BR" | "en";
const LocaleContext = createContext<{ locale: Locale; setLocale: (locale: Locale) => void } | null>(null);

export function LocaleProvider({ children }: { children: ReactNode }) {
  // A stable initial value keeps server rendering and hydration identical.
  const [locale, updateLocale] = useState<Locale>("en");

  useEffect(() => {
    let preferred: Locale = resolveLocale(navigator.language);
    try {
      preferred = resolveLocale(navigator.language, localStorage.getItem(LOCALE_STORAGE_KEY));
    } catch {
      // Storage can be unavailable in private/restricted browser contexts.
    }
    updateLocale(preferred);
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale === "pt-BR" ? "pt-BR" : "en";
  }, [locale]);

  function setLocale(next: Locale) {
    updateLocale(next);
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, next);
    } catch {
      // Switching language still works for this visit without persistence.
    }
  }

  return <LocaleContext.Provider value={{ locale, setLocale }}>{children}</LocaleContext.Provider>;
}

export function useLocale() {
  const context = useContext(LocaleContext);
  if (!context) throw new Error("useLocale must be used within LocaleProvider");
  return context;
}

export function LanguageSwitch({ className = "" }: { className?: string }) {
  const { locale, setLocale } = useLocale();
  return (
    <div role="group" aria-label={locale === "pt-BR" ? "Idioma" : "Language"} className={`inline-flex rounded-lg border border-surface-700 p-1 text-xs ${className}`}>
      {(["pt-BR", "en"] as const).map((option) => (
        <button key={option} type="button" lang={option === "pt-BR" ? "pt-BR" : "en"} aria-label={option === "pt-BR" ? "Português" : "English"} aria-pressed={locale === option} onClick={() => setLocale(option)} className={`rounded-md px-2.5 py-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent-400 ${locale === option ? "bg-accent-500/15 text-accent-400" : "text-surface-300 hover:text-white"}`}>
          {option === "pt-BR" ? "PT" : "EN"}
        </button>
      ))}
    </div>
  );
}
