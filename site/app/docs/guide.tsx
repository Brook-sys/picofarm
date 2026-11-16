"use client";

import Link from "next/link";
import { useLocale } from "../components/locale";
import { guides, guideHref, guideOrder } from "./content";

const repository = "https://github.com/Brook-sys/picofarm";

export default function DocsGuide({ id }: { id: string }) {
  const { locale } = useLocale();
  const pt = locale === "pt-BR";
  const language = pt ? "pt" : "en";
  const guide = guides[id][language];

  return (
    <article className="docs-prose" lang={pt ? "pt-BR" : "en"}>
      <p className="!text-sm !text-accent-400">PicoFarm · {pt ? "Documentação beta" : "Beta documentation"}</p>
      <h1>{guide.title}</h1>
      <p className="text-lg !text-surface-300">{guide.intro}</p>
      {guide.experimental && (
        <div className="docs-callout" role="note">
          <strong>{pt ? "Experimental — em testes." : "Experimental — under testing."}</strong>{" "}
          {pt ? "Área comercial herdada; não apresentada como pronta para produção." : "Inherited commercial area; not presented as production-ready."}
        </div>
      )}
      {guide.sections.map((section, index) => (
        <section key={section.title} id={id === "start" && index === 0 ? "installation" : undefined} className="scroll-mt-24">
          <h2>{section.title}</h2>
          {section.body.map((paragraph) => <p key={paragraph}>{paragraph}</p>)}
          {id === "start" && index === 0 && (
            <p><Link href="/docs/deployment">{pt ? "Instalar PicoFarm com Docker Compose →" : "Install PicoFarm with Docker Compose →"}</Link></p>
          )}
        </section>
      ))}
      {id === "deployment" && (
        <>
          <h2>{pt ? "Exemplo de configuração de rede" : "Network configuration example"}</h2>
          <p>{pt ? "Trecho ilustrativo do serviço; use o compose.yaml completo do repositório, não este trecho isolado." : "Illustrative service excerpt; use the complete repository compose.yaml, not this excerpt alone."}</p>
          <pre><code>{`services:
  picofarm:
    image: ghcr.io/brook-sys/picofarm:latest
    ports:
      - "127.0.0.1:8084:8084"
    environment:
      ENCRYPTION_KEY: \${ENCRYPTION_KEY:?Set ENCRYPTION_KEY in .env}
    volumes:
      - picofarm-data:/data
volumes:
  picofarm-data:`}</code></pre>
          <p><a href={`${repository}/blob/main/docs/INSTALL_DOCKER_COMPOSE.md`}>{pt ? "Guia central Docker Compose (inglês): instalação, configuração e verificação" : "Central Docker Compose guide: installation, configuration and verification"}</a></p>
          <p><a href={`${repository}/blob/main/compose.yaml`}>compose.yaml</a>{" · "}<a href={`${repository}/blob/main/.env.compose.example`}>.env.compose.example</a>{" · "}<a href={`${repository}/blob/main/docs/GETTING_STARTED.md`}>{pt ? "Desenvolvimento a partir do código" : "Source development"}</a></p>
        </>
      )}
      {(id === "dashboard" || id === "materials") && (
        <figure className="not-prose my-8 overflow-hidden rounded-xl border border-surface-700">
          <img src={`/screenshots/${id === "dashboard" ? "tela_geral_dashboardprincipal" : "tela_filamentos"}.webp`} alt={guide.title} width={1600} height={id === "dashboard" ? 897 : 899} loading="lazy" decoding="async" className="h-auto w-full" />
          <figcaption className="p-3 text-center text-xs text-surface-400">{pt ? "Captura real do beta com dados de demonstração." : "Real beta screenshot with demo data."}</figcaption>
        </figure>
      )}
      {id === "start" && (
        <>
          <h2>{pt ? "Guias" : "Guides"}</h2>
          <ul>{guideOrder.filter((key) => key !== "start").map((key) => <li key={key}><Link href={guideHref(key)}>{guides[key][language].title}</Link>{guides[key][language].experimental ? (pt ? " — experimental" : " — experimental") : ""}</li>)}</ul>
          <p><Link href="/docs/deployment">{pt ? "Instalar com Docker Compose" : "Install with Docker Compose"}</Link>{" · "}<a href={`${repository}/issues`}>{pt ? "Relatar problemas no GitHub" : "Report bugs on GitHub"}</a></p>
          <h2>{pt ? "Origem e transparência" : "Origin and transparency"}</h2>
          <p>{pt ? "PicoFarm é um fork de " : "PicoFarm is a fork of "}<a href="https://github.com/philjestin/Daedalus">Daedalus</a>{pt ? ", adaptado com auxílio de IA. Recursos herdados não são prova de testes ou maturidade nesta versão. Consulte o código e relate falhas reproduzíveis." : ", adapted with AI assistance. Inherited features are not evidence of testing or maturity in this version. Inspect the code and report reproducible failures."}</p>
        </>
      )}
      <hr />
      <p className="!text-sm">{pt ? "Beta desenvolvido com auxílio de IA. Faça backup, teste com supervisão e confira os limites da integração antes de usar." : "Beta developed with AI assistance. Back up, test under supervision and check integration limits before use."}{" "}<a href={`${repository}/issues`}>{pt ? "Relatar problema" : "Report a bug"}</a></p>
    </article>
  );
}
