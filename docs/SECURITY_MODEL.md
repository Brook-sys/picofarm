# PicoFarm Security Model

This document records PicoFarm's current security assumptions and the hardening work needed before broader self-hosted exposure.

## Current operating assumption

PicoFarm currently appears optimized for local or trusted-network operation. It manages sensitive operational capabilities:

- printer start/pause/resume/cancel/emergency actions;
- uploads and file deletion;
- backups and restore paths;
- customer/order data;
- commerce integrations;
- printer credentials and access codes;
- notification channels.

Treat remote exposure as a separate hardening project. Do not assume that a LAN or reverse proxy is safe by default.

## Secrets and credentials

Never commit or print real credentials. Examples include:

- Etsy client secrets/tokens;
- Squarespace API keys;
- Bambu access codes or auth material;
- OctoPrint API keys;
- Moonraker tokens or URLs containing credentials;
- webhook secrets;
- JWT/signing secrets;
- SMTP/webhook/notification provider tokens.

Use `[REDACTED]` in docs, issues, plans, and logs shared with agents.

## CORS and browser origins

`internal/api/router.go` currently uses permissive CORS behavior. This is convenient during local development but risky for self-hosted browser access.

Recommended hardening path:

1. Introduce an explicit `ALLOWED_ORIGINS` environment variable or equivalent configuration.
2. Keep local development convenient for `localhost` origins.
3. Require explicit origins for self-hosted/production mode.
4. Add router tests for allowed and blocked origins.
5. Document deployment examples after the behavior is implemented.

## Sensitive endpoint classes

When changing these areas, add tests and update this document if behavior changes:

| Class | Examples | Risk |
| --- | --- | --- |
| Printer control | start, pause, resume, cancel, emergency stop, upload/send file | Physical device control and material waste |
| File operations | upload, download, delete, thumbnail generation | Arbitrary file handling, data leakage, disk usage |
| Backup/restore | create backup, restore database, retention cleanup | Data loss or rollback to stale state |
| Integrations | Etsy/Squarespace sync, webhooks, OAuth callbacks | External account access and spoofed events |
| Notifications | template rendering, test delivery, channel config | Data leakage to external channels |
| Admin/settings | credentials, runtime config, dispatch settings | Persistent unsafe configuration |

## Upload and file storage considerations

File-related changes should verify:

- file names are sanitized or content-addressed where appropriate;
- storage paths cannot escape the intended root;
- invalid 3MF/GCode/STL inputs fail safely;
- large files do not exhaust memory unexpectedly;
- delete operations cannot remove unrelated files;
- temp files are cleaned up.

## Backups and restore considerations

Backup/restore work should verify:

- backups are written to the intended directory;
- retention deletes only backup files it owns;
- restore refuses unsafe paths;
- restore behavior is documented and validated against a temp DB;
- runtime writes are stopped or coordinated during restore.

## Minimum hardening checklist before remote/self-hosted exposure

- [ ] CORS restricted by explicit allowed origins.
- [ ] Authentication/authorization model documented and implemented or clearly delegated to a trusted reverse proxy.
- [ ] Sensitive printer/file/backup endpoints reviewed.
- [ ] Webhook signature verification documented and tested for each provider.
- [ ] Secrets loaded from environment/config, not stored in source or logs.
- [ ] Backup/restore tested in an isolated temporary environment.
- [ ] Security-sensitive operations have regression tests.

## Agent rules

Agents working on PicoFarm must:

- avoid printing secrets;
- prefer fake clients and temp directories over real printers/integrations;
- stop and ask before using real credentials or hardware;
- document new security assumptions in this file;
- include security-impact notes in the final report when touching sensitive endpoint classes.
