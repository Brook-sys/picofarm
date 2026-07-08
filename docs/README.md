# PicoFarm Documentation

This directory contains the operational documentation for PicoFarm. Prefer updating these docs whenever behavior, commands, architecture, validation, storage, integrations, or agent workflow changes.

## Start here

- [Project baseline](PROJECT_BASELINE.md) — current stack, runtime defaults, validation commands, known warnings, and release/commit expectations.
- [Architecture](ARCHITECTURE.md) — backend, frontend, database, realtime, integrations, and where to make common changes.
- [API and data contracts](API_CONTRACTS.md) — HTTP/JSON conventions, route groups, shared enum values, and Go/TypeScript synchronization rules.
- [Regression matrix](REGRESSION_MATRIX.md) — critical user flows to validate manually or automate as tests.
- [Security model](SECURITY_MODEL.md) — current assumptions and hardening work for local/self-hosted operation.

## Documentation policy for humans and agents

When changing PicoFarm, keep docs close to the code change:

| Change type | Docs to check |
| --- | --- |
| New or changed setup/build/test command | `README.md`, `docs/PROJECT_BASELINE.md`, `AGENTS.md` |
| Backend route, JSON response, or domain model | `docs/API_CONTRACTS.md`, `docs/ARCHITECTURE.md` |
| Database migration or storage behavior | `docs/ARCHITECTURE.md`, `docs/PROJECT_BASELINE.md` |
| Critical workflow behavior | `docs/REGRESSION_MATRIX.md` |
| Self-hosted, CORS, auth, backups, uploads, printer control | `docs/SECURITY_MODEL.md` |
| Agent workflow or validation policy | `AGENTS.md`, `docs/PROJECT_BASELINE.md` |

Keep documentation practical and current. Do not describe planned behavior as already implemented. Use placeholders such as `[REDACTED]` instead of secrets, tokens, API keys, printer credentials, or connection strings.
