# Getting started with PicoFarm

This guide covers a source checkout and a locally built Docker image. It does not assume a particular host, private network, remote, or deployment service. For security boundaries, read the [security model](SECURITY_MODEL.md) before exposing an installation.

## 1. Choose how to run

| Mode | Browser URL | Use |
| --- | --- | --- |
| Development | `http://localhost:5174` | React hot reload; Vite proxies API and WebSocket traffic to port 8084 |
| Built application | `http://localhost:8084` | Go server serves `web/dist` and the API together |
| Docker example below | `http://localhost:8084` | Same application with persistent data under `/data` |

The marketing/documentation site in `site/` is separate; it is not required to run PicoFarm.

## 2. Install prerequisites and dependencies

For a source build, install:

- Git and Make.
- Go **1.26.11 or newer**, as required by `go.mod`. An older Go installation may download the required toolchain when `GOTOOLCHAIN=auto` is enabled and network access is available.
- Node.js compatible with Vite 7: **20.19+ on Node 20, or 22.12+**, with npm. Prefer a maintained Node.js release.

```sh
git clone https://github.com/Brook-sys/picofarm.git
cd picofarm
go version
node --version
npm --version
go mod download
(cd web && npm ci)
```

`npm ci` uses `web/package-lock.json` and fails if the lockfile disagrees with `package.json`. The existing `make deps` target is an alternative for dependency maintenance, but it runs `go mod tidy` and `npm install`, which may modify dependency files. It is not the reproducible-install path above.

## 3. Start development servers

From the repository root:

```sh
make dev
```

Or use two terminals, each starting in the repository root:

```sh
make backend
```

```sh
make frontend
```

Open `http://localhost:5174`. The configured Vite port is **5174**, not Vite's upstream default of 5173. If busy, Vite tries another port; use its printed URL. The development frontend binds to `0.0.0.0`; the backend also listens on all interfaces. Keep the environment isolated if other devices should not reach it.

Check backend liveness separately:

```sh
curl --fail http://localhost:8084/health
```

A successful health response is not a printer, integration, or data-recovery check. Stop the foreground development servers with `Ctrl+C`; verify both processes stopped before starting another copy.

## 4. Build and run the application

```sh
make build
./bin/server
```

This produces `bin/server` and `web/dist/`. Run from the repository root, or configure `STATIC_DIR` explicitly. Frontend assets are served from disk, not embedded in the binary. Open `http://localhost:8084` and use `Ctrl+C` for graceful shutdown.

For a portable installation, retain the binary and frontend assets together and configure explicit writable data paths. Start only one instance against a data directory unless you have separately validated multi-process behavior.

## 5. Configure paths and optional services

The server loads a `.env` file from its working directory when present. Exported environment values take precedence. Keep real values in ignored local configuration; never commit credentials or private infrastructure details.

| Variable | Default / role |
| --- | --- |
| `PORT` | `8084`, backend and built frontend HTTP port |
| `DATABASE_PATH` | `~/.picofarm/picofarm.db`; initialized and migrated on startup |
| `UPLOAD_DIR` | `./uploads`, relative to the server's working directory |
| `STATIC_DIR` | `./web/dist`, production frontend assets |
| `ALLOWED_ORIGINS` | Unset allows loopback development origins; use explicit comma-separated origins for another browser origin |
| `ENCRYPTION_KEY` | Optional explicit key for encrypted integration settings; preserve it securely across restarts and migrations |
| `SLICER_CONNECTION_URL` | Optional external slicer URL; overrides the saved slicer connection setting |
| `PICOFARM_SCAN_NETWORK` | Optional printer-discovery CIDR; see [runtime defaults](PROJECT_BASELINE.md#runtime-defaults) for container networking and scan limits |

This is not an exhaustive variable list. Authentication settings and their limitations are documented in the [security model](SECURITY_MODEL.md); Etsy settings and additional runtime defaults are in the [baseline](PROJECT_BASELINE.md). No commerce credentials or slicer service are required for the basic catalog/manual-printer workflow.

Without an explicit `ENCRYPTION_KEY`, the encryption implementation derives a fallback from machine identity. Moving the database to a different machine/container identity can therefore make encrypted settings unreadable. Choose and securely retain an explicit key before configuring integrations; do not assume changing it later re-encrypts existing settings.

Frontend settings are **build-time** Vite variables:

- `VITE_API_URL` overrides the API origin; otherwise requests use the browser's origin. Do not append `/api`, because the client adds it.
- `VITE_WS_URL` overrides the full WebSocket URL, including `/ws`.

Normally leave both unset: Vite proxies `/api` and `/ws` in development, and the built app uses the backend's origin. If changing the backend `PORT`, also adjust the development proxy targets in `web/vite.config.ts` or configure both frontend URLs and the backend origin policy. Changing `PORT` alone does not update the proxy. Never put secrets in `VITE_*` variables: they become browser-visible assets.

## 6. Run with Docker instead

Docker builds the frontend and backend inside the image, so local Go/Node installations are not needed for this option. Run from the checkout root:

```sh
docker build -t picofarm:local .
docker run --name picofarm --detach \
  --publish 127.0.0.1:8084:8084 \
  --mount source=picofarm-data,target=/data \
  picofarm:local
```

The loopback-only host binding intentionally avoids exposing the application to the LAN. The named volume stores `/data/picofarm.db`, uploads under `/data/uploads`, and database backups under `/data/backups`. The image runs as a non-root user; bind-mounted directories must be writable by that user.

```sh
docker logs picofarm
curl --fail http://localhost:8084/health
docker stop picofarm
docker rm picofarm
```

Removing the container does not remove the named volume. Do **not** delete the volume unless discarding its data is intentional. Configure a stable encryption key securely before using integrations in a replaceable container.

Printer discovery inside a bridge container sees the container network by default, not necessarily the printer LAN. Configure `PICOFARM_SCAN_NETWORK` and verify routing/firewall access deliberately. If adding the optional slicer container, keep its port unpublished and connect over the internal container network; PicoFarm does not provide authentication for a directly exposed slicer port.

## 7. Try a safe first workflow

1. Start with a fresh database and no real integration credentials.
2. Create a project in the product catalog and inspect its parts/design area.
3. Register a **manual** printer for record-keeping rather than connecting real hardware.
4. Create a manual order, add an item linked to a project, and process it into a task.
5. Inspect the task, checklist, and production views before attempting real printer actions.
6. Review Settings for currency and costing assumptions. Changing display currency does not convert stored amounts.

Capabilities vary by printer protocol and sales channel. Do not interpret a provider descriptor in the UI as proof of live account connectivity; Shopify is partial, and Mercado Livre/Shopee currently lack production credential wiring. See [API contracts](API_CONTRACTS.md#integrations).

## 8. Validate a checkout

From the repository root:

```sh
make test
(cd web && npm run lint)
make build
```

- `make test` runs `go test -v ./...`; it does not run frontend tests.
- Frontend lint runs ESLint.
- `make build` compiles Go and runs `tsc -b && vite build` in `web/`, checking the referenced TypeScript projects before bundling.
- `make lint` additionally requires a separately installed `golangci-lint`.

Use [Contributing](../CONTRIBUTING.md) and the [regression matrix](REGRESSION_MATRIX.md) to select additional tests. These gates do not establish production readiness or validate real hardware, external credentials, or disaster recovery.

## 9. Preserve data and troubleshoot

### Data protection

Keep the SQLite database, uploaded files, and any encryption key together in a recovery plan. Database backups are stored in a `backups` directory beside the database; they do not replace a backup of `UPLOAD_DIR`. Back up before upgrading, because startup applies migrations automatically. Test recovery in an isolated directory before relying on it. After an application database restore, restart the server.

**Avoid `make clean` on an installation with valuable uploads:** it deletes `bin/`, `web/dist/`, and `uploads/` relative to the checkout. Keep persistent data outside build-cleanup paths.

### Common failures

| Symptom | Check |
| --- | --- |
| Browser cannot load the development UI | Use Vite's printed port, normally 5174; verify the frontend process is running |
| API proxy fails | Check `/health` on 8084 and the proxy targets in `web/vite.config.ts` |
| Built server does not show the UI | Confirm `web/dist/index.html` exists and `STATIC_DIR` resolves from the current working directory |
| SQLite or upload permission error | Check configured paths, writable parent directories, and container mount ownership |
| Node engine/build error | Check `node --version` against the Vite requirement; reinstall with `npm ci` after selecting Node |
| Go toolchain cannot download | Install the required Go version locally or allow the toolchain download through the network policy |
| Printer discovery finds nothing in Docker | Check scan CIDR, container routing, and firewall rules; do not assume discovery can traverse networks |
| Slicer unavailable | Check the configured URL and service health; the slicer is a separate optional service |

Consult [Architecture](ARCHITECTURE.md) for code locations, [API contracts](API_CONTRACTS.md) for endpoint behavior, and the [documentation index](README.md) for the full reference set.
