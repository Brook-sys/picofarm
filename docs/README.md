# PicoFarm Documentation

This directory contains the operational documentation for PicoFarm. Prefer updating these docs whenever behavior, commands, architecture, validation, storage, integrations, or agent workflow changes.

## Start here

- [Project baseline](PROJECT_BASELINE.md) — current stack, runtime defaults, validation commands, known warnings, and release/commit expectations.
- [Architecture](ARCHITECTURE.md) — backend, frontend, database, realtime, integrations, and where to make common changes.
- [API and data contracts](API_CONTRACTS.md) — HTTP/JSON conventions, route groups, shared enum values, and Go/TypeScript synchronization rules.
- [Sales channels architecture](SALES_CHANNELS.md) — modular provider/adapters plan for Etsy, Squarespace, Shopify, and future commerce channels.
- [Regression matrix](REGRESSION_MATRIX.md) — critical user flows to validate manually or automate as tests.
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
