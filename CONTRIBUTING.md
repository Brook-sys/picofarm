# Contributing to PicoFarm

## Development setup

Follow [Getting started](docs/GETTING_STARTED.md) for prerequisites, dependency installation, development servers, Docker, and data safety. The backend defaults to **8084** and the development frontend to **5174** (or the next free port printed by Vite).

Create a branch from `main` in your fork or checkout. Keep changes focused; do not combine unrelated refactors with behavior changes.

## Making changes

1. Read the [architecture](docs/ARCHITECTURE.md) and the relevant domain documentation in the [docs index](docs/README.md).
2. Add or update tests for behavior changes. Use temporary databases/directories and fake integration clients; do not require real credentials or printers in CI.
3. Keep Go JSON models, TypeScript types, API helpers, and [API contracts](docs/API_CONTRACTS.md) aligned.
4. Update setup or operational docs when commands, configuration, persistence, or security behavior changes. Use generic examples only; keep private operator details in ignored local notes.
5. Run the validation gate below and record actual results, warnings, and untested workflows in the pull request.
6. Inspect the diff for secrets, generated files, lockfile churn, and unrelated changes before committing.
7. Open a pull request against `main` with a summary, motivation, validation evidence, and residual risks.

## Validation

Run from the repository root:

```sh
make test
(cd web && npm run lint)
make build
```

`make test` runs Go tests only. `make build` builds the backend and executes the frontend's `tsc -b && vite build`; this checks the referenced TypeScript projects. Running `npx tsc --noEmit` against the root `web/tsconfig.json` is not a substitute, because that file has no source files and delegates through project references.

For a frontend-only type check, use:

```sh
(cd web && npx tsc -b)
```

`make lint` also runs `golangci-lint`, which must be installed separately. Select domain-specific checks from the [regression matrix](docs/REGRESSION_MATRIX.md). Do not claim real-device, live integration, or restore validation unless you actually performed it with authorization.

**Data warning:** `make clean` deletes `uploads/` as well as build artifacts. Never use it to clean a checkout containing valuable uploads.

## Code style and architecture

```text
API handlers → Services → Repositories → SQLite
```

- **Go:** use `gofmt`, explicit error returns, and structured logging with `slog`. Handlers handle HTTP parsing/validation/responses; services hold business rules; repositories handle SQL and data mapping.
- **TypeScript/React:** strict TypeScript, functional components, and TanStack Query for server state. Keep API types synchronized with backend responses.
- **CSS:** Tailwind CSS 4 utility classes.
- **Tests:** place Go tests alongside source as `*_test.go`; prefer table-driven cases, isolated SQLite databases, and explicit failure-path coverage.
- **Database changes:** update the embedded schema and compatibility path, with fresh-database and upgrade tests.
- **Safety:** use manual/fake printers for tests. Review the [security model](docs/SECURITY_MODEL.md) before changing hardware controls, uploads, backups, credentials, or network exposure.

## Reporting issues

Use this repository's [issue tracker](https://github.com/Brook-sys/picofarm/issues). Include:

- Expected versus actual behavior.
- Minimal steps to reproduce.
- Application version/commit, operating system, browser, and run mode.
- Relevant sanitized logs and test output.

Do not include tokens, customer data, private hostnames, or network details. For vulnerabilities, follow [SECURITY.md](SECURITY.md) rather than opening a public issue with sensitive reproduction details.
