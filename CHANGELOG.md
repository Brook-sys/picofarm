# Changelog

All notable changes to PicoFarm will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The 1.1.x tags predate the fork detachment and were never published to this
repository; 1.2.0 is the first release built and published from
`Brook-sys/picofarm` via the GHCR workflow.

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
