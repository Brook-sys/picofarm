"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ArrowLeft, BookOpen, Menu, X } from "lucide-react";
import { LanguageSwitch, useLocale } from "../components/locale";
import { guideHref, guideOrder, guides } from "./content";

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const pathname = usePathname();
  const { locale } = useLocale();
  const pt = locale === "pt-BR";
  const language = pt ? "pt" : "en";

  return (
    <div className="min-h-screen bg-surface-950">
      <header className="fixed top-0 z-40 w-full border-b border-surface-800/50 bg-surface-950/95 backdrop-blur-xl">
        <div className="flex h-16 items-center justify-between gap-3 px-4 sm:px-6">
          <button type="button" onClick={() => setSidebarOpen(!sidebarOpen)} className="text-surface-300 lg:hidden" aria-label={pt ? "Alternar navegação da documentação" : "Toggle documentation navigation"} aria-expanded={sidebarOpen} aria-controls="docs-navigation">
            {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
          <Link href="/" className="flex items-center gap-2 text-sm text-surface-300 hover:text-white"><ArrowLeft className="h-4 w-4" /><span className="hidden sm:inline">{pt ? "Voltar ao início" : "Back to home"}</span></Link>
          <Link href="/docs" className="flex items-center gap-2 text-sm font-bold text-surface-50"><BookOpen className="h-4 w-4 text-accent-400" />PicoFarm Docs</Link>
          <LanguageSwitch />
        </div>
      </header>
      <div className="flex pt-16">
        {sidebarOpen && <button type="button" className="fixed inset-0 z-20 bg-black/50 lg:hidden" onClick={() => setSidebarOpen(false)} aria-label={pt ? "Fechar navegação" : "Close navigation"} />}
        <aside id="docs-navigation" className={`fixed bottom-0 top-16 z-30 w-64 overflow-y-auto border-r border-surface-800/50 bg-surface-950 px-3 py-4 transition-transform lg:translate-x-0 ${sidebarOpen ? "translate-x-0" : "-translate-x-full"}`}>
          <nav aria-label={pt ? "Documentação" : "Documentation"} className="space-y-1">
            {guideOrder.map((key) => {
              const guide = guides[key][language];
              const href = guideHref(key);
              const active = pathname === href;
              return <Link key={key} href={href} onClick={() => setSidebarOpen(false)} aria-current={active ? "page" : undefined} className={`block rounded-lg px-3 py-2 text-sm transition-colors ${active ? "bg-accent-500/10 font-medium text-accent-400" : "text-surface-300 hover:bg-surface-900 hover:text-white"}`}>
                {guide.title}
                {guide.experimental && <span className="mt-0.5 block text-xs text-surface-400">{pt ? "Experimental · em testes" : "Experimental · under testing"}</span>}
              </Link>;
            })}
          </nav>
        </aside>
        <main className="w-full lg:pl-64"><div className="mx-auto max-w-3xl px-6 py-12 lg:px-12">{children}</div></main>
      </div>
    </div>
  );
}
