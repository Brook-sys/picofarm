"use client";

import { ArrowRight, Bell, Camera, CheckCircle2, Container, FileText, FlaskConical, FolderOpen, Github, Layers, ListOrdered, Menu, Package, Printer, ScanLine, Settings2, TriangleAlert, X } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { LanguageSwitch, useLocale } from "./components/locale";
import { landingContent } from "./components/landing-content";

const REPO = "https://github.com/Brook-sys/picofarm";
const DAEDALUS = "https://github.com/philjestin/Daedalus";
const primary = "inline-flex items-center justify-center gap-2 rounded-xl bg-accent-500 px-5 py-3 text-sm font-semibold text-white hover:bg-accent-600 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent-400";
const secondary = "inline-flex items-center justify-center gap-2 rounded-xl border border-surface-700 px-5 py-3 text-sm font-medium text-surface-200 hover:border-accent-400 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent-400";

function AnimatedSection({ children, className = "" }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const element = ref.current;
    if (!element || !window.matchMedia || !window.IntersectionObserver) return;
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    let animation: Animation | undefined;
    const stop = () => { if (motion.matches) animation?.cancel(); };
    const observer = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) return;
      if (!motion.matches && element.animate) {
        animation = element.animate([{ opacity: 0, transform: "translateY(24px)" }, { opacity: 1, transform: "translateY(0)" }], { duration: 650, easing: "ease-out" });
      }
      observer.unobserve(element);
    }, { threshold: 0.08 });
    observer.observe(element);
    motion.addEventListener("change", stop);
    return () => { observer.disconnect(); animation?.cancel(); motion.removeEventListener("change", stop); };
  }, []);
  // Content stays visible if JavaScript, IntersectionObserver or animation fails.
  return <div ref={ref} className={className}>{children}</div>;
}

function Brand() {
  return <a href="/" aria-label="PicoFarm" className="flex shrink-0 items-center gap-2.5 rounded-md focus-visible:outline-2 focus-visible:outline-accent-400"><img src="/picofarm_logo.png" alt="" width={36} height={36} className="h-9 w-9 object-contain" /><span className="text-lg font-bold tracking-tight text-surface-50">PicoFarm</span></a>;
}

function Nav() {
  const { locale } = useLocale();
  const t = landingContent[locale];
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const header = useRef<HTMLElement>(null);
  const links = ["#features", "#experimental", "#deploy", "/docs"];
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setOpen(false); button.current?.focus(); }
    };
    const onPointer = (event: PointerEvent) => {
      if (!header.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPointer);
    return () => { document.removeEventListener("keydown", onKey); document.removeEventListener("pointerdown", onPointer); };
  }, [open]);
  return <header ref={header} className="fixed top-0 z-50 w-full border-b border-surface-800/70 bg-surface-950/95 backdrop-blur-xl">
    <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-surface-950 focus:p-4 focus:text-accent-400">{t.skip}</a>
    <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-3 sm:px-6">
      <Brand />
      <nav aria-label={locale === "pt-BR" ? "Navegação principal" : "Main navigation"} className="hidden items-center gap-5 lg:flex">{links.map((href, i) => <a key={href} href={href} className="rounded text-sm text-surface-300 hover:text-white focus-visible:outline-2 focus-visible:outline-accent-400">{t.nav[i]}</a>)}</nav>
      <div className="flex items-center gap-2"><LanguageSwitch /><button ref={button} type="button" aria-expanded={open} aria-controls="mobile-navigation" aria-label={open ? t.close : t.menu} onClick={() => setOpen(!open)} className="rounded-lg p-3 text-surface-200 focus-visible:outline-2 focus-visible:outline-accent-400 lg:hidden">{open ? <X aria-hidden="true" className="h-5 w-5" /> : <Menu aria-hidden="true" className="h-5 w-5" />}</button></div>
    </div>
    {open && <nav id="mobile-navigation" aria-label={locale === "pt-BR" ? "Navegação móvel" : "Mobile navigation"} className="border-t border-surface-800 px-6 py-4 lg:hidden">{links.map((href, i) => <a key={href} href={href} onClick={() => setOpen(false)} className="block rounded py-3 text-surface-200 focus-visible:outline-2 focus-visible:outline-accent-400">{t.nav[i]}</a>)}</nav>}
  </header>;
}

function Heading({ badge, title, description }: { badge: string; title: string; description: string }) {
  return <AnimatedSection className="mx-auto mb-12 max-w-2xl text-center"><p className="mb-4 text-xs font-semibold uppercase tracking-widest text-accent-400">{badge}</p><h2 className="mb-4 text-3xl font-bold tracking-tight text-surface-50 sm:text-4xl">{title}</h2><p className="leading-relaxed text-surface-400 sm:text-lg">{description}</p></AnimatedSection>;
}

function ScreenshotPlaceholder({ title }: { title: string }) {
  const { locale } = useLocale();
  const t = landingContent[locale];
  return <figure className="overflow-hidden rounded-2xl border border-surface-700 bg-surface-900 shadow-2xl">
    <div aria-hidden="true" className="flex items-center gap-2 border-b border-surface-700 bg-surface-800/50 px-4 py-3">{[0, 1, 2].map((n) => <span key={n} className="h-2 w-2 rounded-full bg-surface-600" />)}<span className="ml-3 truncate font-mono text-xs text-surface-400">PicoFarm / {title}</span></div>
    <div className="flex aspect-video min-h-44 flex-col items-center justify-center gap-3 bg-surface-800/40 px-5 text-center"><ScanLine aria-hidden="true" className="h-10 w-10 text-surface-500" /><p className="text-sm font-medium text-surface-300 sm:text-lg">{title}</p><p className="font-mono text-xs text-surface-400">{t.placeholder}</p></div>
    <figcaption className="border-t border-surface-700 px-4 py-3 text-center text-xs text-surface-400">{t.notScreenshot}</figcaption>
  </figure>;
}

export default function Home() {
  const { locale } = useLocale();
  const t = landingContent[locale];
  const icons = [ListOrdered, Printer, FolderOpen, Layers, FileText, Bell, Camera, Settings2, Package];
  return <>
    <Nav />
    <main id="main" tabIndex={-1}>
      <section className="relative overflow-hidden pb-20 pt-28 sm:pb-28 sm:pt-32">
        <div aria-hidden="true" className="pointer-events-none absolute inset-0 grid-bg" />
        <div aria-hidden="true" className="pointer-events-none absolute left-1/2 top-1/3 h-[500px] w-[500px] -translate-x-1/2 rounded-full bg-accent-500/5 blur-[100px]" />
        <div className="relative mx-auto max-w-6xl px-6">
          <aside aria-labelledby="beta-warning" className="mb-10 rounded-2xl border-2 border-accent-500/70 bg-surface-900 p-5 sm:p-7">
            <div className="flex items-start gap-3"><TriangleAlert aria-hidden="true" className="mt-1 h-7 w-7 shrink-0 text-accent-400" /><div><h2 id="beta-warning" className="mb-2 text-lg font-bold text-accent-400 sm:text-xl">{t.warningTitle}</h2><p className="leading-relaxed text-surface-200">{t.warningBefore}<a className="font-semibold underline underline-offset-4 hover:text-accent-400" href={DAEDALUS}>Daedalus</a>{t.warningAfter}</p><p className="mt-3 text-sm leading-relaxed text-surface-300">{t.report} <a href={`${REPO}/issues/new/choose`} className="font-semibold text-accent-400 underline underline-offset-4">{t.reportLink} <span aria-hidden="true">↗</span></a></p></div></div>
          </aside>
          <AnimatedSection className="mx-auto max-w-4xl text-center">
            <p className="mb-5 inline-flex rounded-full border border-accent-500/25 bg-accent-500/5 px-4 py-2 text-xs font-medium text-accent-400">{t.badge}</p>
            <h1 className="mb-6 text-4xl font-bold tracking-tight text-surface-50 sm:text-6xl lg:text-7xl">{t.title}<br /><span className="gradient-text">{t.highlight}</span></h1>
            <p className="mx-auto mb-8 max-w-2xl text-lg leading-relaxed text-surface-400">{t.intro}</p>
            <div className="flex flex-col justify-center gap-3 sm:flex-row"><a href="/docs#installation" className={primary}><Container aria-hidden="true" className="h-4 w-4" />{t.install}</a><a href="/docs" className={secondary}>{t.docs}<ArrowRight aria-hidden="true" className="h-4 w-4" /></a></div>
            <a href={REPO} className="mt-5 inline-flex items-center gap-2 rounded py-2 text-sm text-surface-300 underline-offset-4 hover:underline"><Github aria-hidden="true" className="h-4 w-4" />{t.github}</a>
          </AnimatedSection>
          <AnimatedSection className="glow-accent mt-12 rounded-2xl"><ScreenshotPlaceholder title={t.preview} /></AnimatedSection>
        </div>
      </section>

      <section id="features" className="scroll-mt-24 py-20 sm:py-28"><div className="mx-auto max-w-6xl px-6">
        <Heading badge={t.featuresBadge} title={t.featuresTitle} description={t.featuresIntro} />
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{t.features.map(([title, description], i) => { const Icon = icons[i]; return <AnimatedSection key={i}><article className="glow-card h-full rounded-xl border border-surface-800 bg-surface-900/50 p-6"><span className="mb-4 flex h-11 w-11 items-center justify-center rounded-lg bg-accent-500/10 text-accent-500"><Icon aria-hidden="true" className="h-5 w-5" /></span><h3 className="mb-2 text-lg font-semibold text-surface-50">{title}</h3><p className="text-sm leading-relaxed text-surface-400">{description}</p></article></AnimatedSection>; })}</div>
      </div></section>

      <section className="border-t border-surface-800/50 py-20 sm:py-28"><div className="mx-auto grid max-w-6xl items-center gap-12 px-6 lg:grid-cols-2">
        <AnimatedSection><h2 className="mb-4 text-3xl font-bold text-surface-50">{t.workflowTitle}</h2><p className="mb-6 leading-relaxed text-surface-400">{t.workflowIntro}</p><ul className="space-y-4">{t.workflowBullets.map((item) => <li key={item} className="flex items-start gap-3 text-sm text-surface-300"><CheckCircle2 aria-hidden="true" className="h-5 w-5 shrink-0 text-accent-500" />{item}</li>)}</ul></AnimatedSection>
        <AnimatedSection><ScreenshotPlaceholder title={t.libraryPreview} /></AnimatedSection>
      </div></section>

      <section id="experimental" className="scroll-mt-24 border-y border-surface-800 bg-surface-900/30 py-20 sm:py-28"><div className="mx-auto max-w-6xl px-6">
        <Heading badge={t.experimentalBadge} title={t.experimentalTitle} description={t.experimentalIntro} />
        <div className="grid gap-5 lg:grid-cols-3">{t.experiments.map(([title, description]) => <AnimatedSection key={title}><article className="h-full rounded-xl border border-dashed border-accent-500/35 p-6"><FlaskConical aria-hidden="true" className="mb-4 h-6 w-6 text-accent-400" /><p className="mb-3 text-xs font-medium uppercase tracking-wide text-accent-400">{t.experimentalBadge}</p><h3 className="mb-3 text-lg font-semibold text-surface-100">{title}</h3><p className="text-sm leading-relaxed text-surface-400">{description}</p></article></AnimatedSection>)}</div>
      </div></section>

      <section id="deploy" className="scroll-mt-24 py-20 sm:py-28"><div className="mx-auto max-w-6xl px-6">
        <Heading badge={t.deployBadge} title={t.deployTitle} description={t.deployIntro} />
        <AnimatedSection className="glow-card mx-auto max-w-3xl rounded-2xl border border-surface-800 bg-surface-900/50 p-6 sm:p-8">
          <Container aria-hidden="true" className="mb-6 h-9 w-9 text-accent-500" />
          <ol className="space-y-5">{t.steps.map((step, i) => <li key={step} className="flex items-start gap-4 text-surface-200"><span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-accent-500/10 font-mono text-sm text-accent-400">{i + 1}</span><span>{step}</span></li>)}</ol>
          <p className="my-7 border-t border-surface-800 pt-6 text-sm leading-relaxed text-surface-400">{t.deployNote}</p>
          <a href="/docs#installation" className={primary}>{t.install}<ArrowRight aria-hidden="true" className="h-4 w-4" /></a>
        </AnimatedSection>
      </div></section>

      <section className="relative overflow-hidden border-t border-surface-800/50 py-20 sm:py-28"><div aria-hidden="true" className="pointer-events-none absolute inset-0 grid-bg" /><AnimatedSection className="relative mx-auto max-w-2xl px-6 text-center"><h2 className="mb-4 text-3xl font-bold text-surface-50 sm:text-4xl">{t.betaTitle}</h2><p className="mb-8 leading-relaxed text-surface-400 sm:text-lg">{t.betaIntro}</p><a href={REPO} className={primary}><Github aria-hidden="true" className="h-4 w-4" />{t.github}</a></AnimatedSection></section>
    </main>
    <footer className="border-t border-surface-800/50 py-10"><div className="mx-auto flex max-w-6xl flex-col justify-between gap-6 px-6 sm:flex-row"><div><Brand /><p className="mt-3 text-sm text-surface-400">{t.footer}</p></div><div className="flex flex-wrap items-center gap-5 text-xs text-surface-400"><a className="hover:text-white" href="/docs">{t.nav[3]}</a><a className="hover:text-white" href={REPO}>GitHub</a><a className="hover:text-white" href={DAEDALUS}>{t.origin}</a><span className="text-accent-400">{t.beta}</span></div></div></footer>
  </>;
}
