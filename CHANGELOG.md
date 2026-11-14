# Changelog

All notable changes to PicoFarm will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The 1.1.x tags predate the fork detachment and were never published to this
repository; 1.2.0 is the first release built and published from
`Brook-sys/picofarm` via the GHCR workflow.

## [Unreleased]

### Added

- Named API tokens (`/api/api-tokens`): mint, list and revoke per-integration
  credentials at runtime. Only a SHA-256 hash is stored, so a database leak does
  not yield replayable credentials; the plaintext is returned once by
  `POST /api/api-tokens` and never again.
- Token scopes as `/api` route prefixes: `*` grants everything, anything else
  (e.g. `/api/slicer`) allows that route and its sub-paths, and a token reaching
  outside its scope answers `403` instead of silently succeeding.
- Immediate revocation: the in-memory validation cache is dropped on every
  create/revoke, so a retired token stops working on the next request rather
  than after a cache TTL. Revoked rows are kept as a record and their names can
  be reused.
- `REQUIRE_API_TOKEN` (accepts `1`/`true`/`yes`/`on`/`enabled`) enables
  authentication without a shared `API_TOKEN`, for deployments that run on named
  tokens alone.
- Settings → API Tokens page: authentication status (with an explicit warning
  that an unconfigured `API_TOKEN` leaves the API open), token creation with
  scope selection, one-time plaintext display with copy, and the active/revoked
  list with per-token last-used timestamps.

### Changed

- `/ws` now enforces an origin allowlist on every handshake, and requires the API token when
  auth is enabled. The allowlist accepts the configured `ALLOWED_ORIGINS`, the same-origin
  scheme+host of the request, and clients that send no `Origin` at all (non-browser tools);
  anything else is refused with `403` before the upgrade. Previously the endpoint accepted any
  origin with no token at all.
- The frontend sends the token as a `?token=` query parameter, because browsers cannot set headers
  on a WebSocket.
- The shared-token comparison is now constant-time (`crypto/subtle`) instead of
  `!=`.
- Usage accounting (`last_used_at`) is throttled to one write per minute per
  token and runs on a detached context, because SQLite serializes writes on a
  single connection.

### Security

- Token material is no longer present in the API surface at all: `token_hash` is
  excluded from JSON, and no endpoint returns a token value after creation.
- An origin that is not allowed fails the WebSocket handshake before any token is
  read, so probing cannot distinguish "bad origin" from "bad token".

## [1.2.0] - 2026-11-11

First release published from this repository. Covers 133 commits of work that
accumulated after the v1.1.3 tag, mostly around sales channels, printer
fleet robustness and queue/dispatch reliability.

### Added

- Configurable display/aggregation currency (`currency_code` setting, 13 supported codes)
  with formatting across dashboard, sales, expenses, materials, printers, projects
  and quotes; no exchange-rate conversion is performed
- Squarespace product variants persist the currency reported by the provider
- Printer preparation analytics: setup overhead per printer job, broken down by phase
- Public business-info endpoint exposes the configured currency

### Changed

- Sales, expenses and financial aggregates are currency-scoped to the configured
  `currency_code` instead of implicitly USD
- Release tooling targets the `picofarm` remote

### Fixed

- Print queue and active-job handling hardening (manual restart, terminal states)
- Printer remote directory traversal validation
- Camera/webcam discovery for Moonraker printers
- Dispatch and notification edge cases

### Security

- Harden printer remote directory traversal validation and extend tests

## [1.0.0] - 2026-02-27

### Added

- Product catalog with parts, designs, and recipe/template system
- Multi-channel order tracking (Etsy, Squarespace, Shopify, manual)
- Printer fleet management with Bambu Lab (MQTT), OctoPrint, and Moonraker integrations
- Auto-dispatch system for print job queue management
- Print job lifecycle tracking with retry chains and failure detection
- Material and spool inventory with low-stock alerts
- Expense tracking with receipt OCR
- Sales recording with weekly insights
- Financial analytics dashboard (revenue, costs, margins, time series)
- Tag system for organizing parts and designs
- Timeline/Gantt view for orders and projects
- Automated backup system with scheduled backups
- Real-time WebSocket updates for printer status and job state
- Desktop builds for macOS, Windows, and Linux via Wails v2
- Versioning and release system with CI/CD pipeline
