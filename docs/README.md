# PicoFarm Documentation

This directory contains the operational documentation for PicoFarm. Prefer updating these docs whenever behavior, commands, architecture, validation, storage, integrations, or agent workflow changes.

## Start here

- [Camera wall](CAMERA_WALL.md) — multi-camera layout, local viewing controls and stream/performance limitations.

- [Install with Docker Compose](INSTALL_DOCKER_COMPOSE.md) — recommended BETA end-user setup, persistence, safe access, upgrades and non-Docker installation backlog.
- [Desktop installation](DESKTOP_INSTALL.md) — release downloads, dependencies, checksum verification and beta limitations.
- [Release notes and versioning](RELEASE_NOTES.md) — current release notes and shared SemVer convention.
- [Desktop app](DESKTOP.md) — optional Wails desktop product: modes, gateway contract, native features, build/test commands and validation limits.
- [Public website](WEBSITE.md) — bilingual content, screenshot placeholders, validation and Vercel configuration.

- [Getting started](GETTING_STARTED.md) — prerequisites, source/Docker setup, configuration, persistent data, validation, and troubleshooting.
- [Project baseline](PROJECT_BASELINE.md) — current stack, runtime defaults, validation commands, known warnings, and release/commit expectations.
- [Architecture](ARCHITECTURE.md) — backend, frontend, database, realtime, integrations, and where to make common changes.
- [API and data contracts](API_CONTRACTS.md) — HTTP/JSON conventions, route groups, shared enum values, and Go/TypeScript synchronization rules.
- [Sales channels architecture](SALES_CHANNELS.md) — modular provider/adapters plan for Etsy, Squarespace, Shopify, and future commerce channels.
- [Interface languages](I18N.md) — en/pt-BR catalogs, per-device preference, currency independence, adding languages and validation.
- [UX and accessibility review](UX_REVIEW.md) — heuristic findings, glossary/microcopy standard and the manual validation plan (proposals, not completed research).
- [Regression matrix](REGRESSION_MATRIX.md) — critical user flows to validate manually or automate as tests.
- [Improvements roadmap](IMPROVEMENTS_ROADMAP.md) — selected proposals and status of implemented local increments.
- [Slicer refinements](SLICER_REFINEMENTS.md) — editor contracts, snapshots, viewer budgets, test evidence and limitations.
- [Background slicer jobs](SLICER_JOBS.md) — bounded local queue, idempotency, recovery and cancellation boundaries.
- [Printer reconnection](PRINTER_RECONNECTION.md) — configurable attempt interval/concurrency, deadlines and power-cycle recovery limits.
- [Security model](SECURITY_MODEL.md) — current assumptions and hardening work for local/self-hosted operation.
- [Sensitive endpoint inventory](SECURITY_ENDPOINTS.md) — route-level risk classes for printer control, files, backups, settings, public routes, and integrations.

## Documentation policy for humans and agents

When changing PicoFarm, keep docs close to the code change:

| Change type | Docs to check |
| --- | --- |
| New or changed setup/build/test command | `README.md`, `docs/PROJECT_BASELINE.md`, `AGENTS.md` |
| Backend route, JSON response, or domain model | `docs/API_CONTRACTS.md`, `docs/ARCHITECTURE.md` |
| Sales-channel integration, sync, product linking, or marketplace/storefront provider | `docs/SALES_CHANNELS.md`, `docs/API_CONTRACTS.md`, `docs/SECURITY_ENDPOINTS.md`, `docs/REGRESSION_MATRIX.md` |
| Database migration or storage behavior | `docs/ARCHITECTURE.md`, `docs/PROJECT_BASELINE.md` |
| Critical workflow behavior | `docs/REGRESSION_MATRIX.md` |
| Self-hosted, CORS, auth, backups, uploads, printer control | `docs/SECURITY_MODEL.md`, `docs/SECURITY_ENDPOINTS.md` |
| Agent workflow or validation policy | `AGENTS.md`, `docs/PROJECT_BASELINE.md` |

Keep documentation practical and current. Do not describe planned behavior as already implemented. Use placeholders such as `[REDACTED]` instead of secrets, tokens, API keys, printer credentials, or connection strings.
