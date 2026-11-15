# Public PicoFarm website

The presentation/documentation website is the separate Next.js project in `site/`. It is not the Go/React printer-control application in `web/`. Hosting the website on Vercel does not host a PicoFarm instance or connect visitors' printers.

## Editorial boundaries

- Audience: one printer or a small farm, personal organization and print-flow control, without assuming sales.
- Main invitation: join the BETA by self-hosting with Docker Compose and reading the documentation.
- Prominent disclosure: originated as a [Daedalus](https://github.com/philjestin/Daedalus) fork; development of this fork is entirely AI-driven (vibe coding).
- Show queue, printer management, STL/G-code organization, optional on-demand slicing, printer files, notifications, cameras, filament inventory and consumption as primary capabilities. Protocol/service limitations still apply.
- Semi-automatic printing, commercial management and marketplace integrations belong to an explicitly experimental/testing-only section. Provider names are not claims of working synchronization.
- Screenshots are neutral gray placeholders until reviewed PicoFarm screenshots are available. Remove credentials, customer data and private infrastructure information from real captures before committing them.

## Languages

The website offers Brazilian Portuguese and English. On the first browser visit, the primary browser language is used: Portuguese (including pt-PT) selects pt-BR; other languages select English. A manual choice takes precedence on later visits when browser storage is available. Secondary browser preferences do not override the primary one.

Language selection is browser-side because the site is statically exported. The initial static HTML uses English before client initialization; it is not a separately indexed Portuguese URL. Language-specific URL/SEO work is a possible future improvement, not a current guarantee. No translation service or analytics is required. Google Fonts remains an external font dependency inherited from the visual design.

## Local validation

```sh
cd site
npm ci
npm test
npm run typecheck
npm run build
```

The build exports `site/out/`. Serve it with a static server and check landing/docs, mobile navigation, keyboard access, primary-language detection, persistent manual selection, blocked storage and reduced motion. Check both languages for untranslated inherited commercial claims.

## Vercel setup

Import the GitHub repository as a Vercel project and set **Root Directory** to `site` (not the repository root). Use the Next.js framework preset, install command `npm ci`, build command `npm run build`, and the static-export output `out`. No application secrets, printer credentials, database URL or private server addresses should be configured for the website.

Select a supported Node.js runtime matching the project's engine requirement. Start with a preview deployment and verify direct access to `/docs` and nested documentation routes, responsive layout and locale behavior. Select the public domain only after the owner approves the preview. Vercel account authorization, project linking and publication require operator consent/access; a local build does not prove a deployed domain exists.

## Issue reporting

The site points to GitHub Issues. `.github/ISSUE_TEMPLATE/` contains bilingual bug/suggestion forms, with required reproduction fields and privacy guidance. Security vulnerabilities use private reporting, described in `SECURITY.md`, not public bug attachments.
