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

Sales-channel integrations add OAuth tokens, API keys, webhook secrets, signed callback/query parameters, and external API error payloads. Follow `docs/SALES_CHANNELS.md` for provider contracts and never expose raw credential material through generic `/api/sales-channels/*` responses, sync-run records, frontend state, logs, fixtures, or documentation.

## CORS and browser origins

`internal/api/router.go` restricts browser CORS origins through the `ALLOWED_ORIGINS` environment variable.

Default behavior remains development-friendly and only allows local browser origins:

- `http://localhost:*`
- `http://127.0.0.1:*`
- `http://[::1]:*`

For self-hosted access from another hostname, set explicit comma-separated origins. Examples:

```sh
ALLOWED_ORIGINS="https://picofarm.example.com"
ALLOWED_ORIGINS="https://picofarm.example.com,http://10.0.0.5:8084"
```

Do not use wildcard origins for deployments that expose printer controls, customer/order data, backups, credentials, or integrations to browsers outside the local machine. Router tests cover allowed and blocked origins.

## HTTP browser hardening headers

`internal/api/middleware.go` adds conservative response headers to every route:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Cross-Origin-Resource-Policy: same-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=()`

These headers reduce browser-side exposure for the local UI and API without introducing authentication or changing JSON contracts. They are not a substitute for an authentication/authorization model when exposing PicoFarm outside a trusted local environment.

## Sensitive endpoint classes

See [Sensitive endpoint inventory](SECURITY_ENDPOINTS.md) for the current route-level classification. Keep that inventory aligned with `internal/api/router.go` whenever routes are added, removed, or materially changed.

When changing these areas, add tests and update this document if behavior changes:

| Class | Examples | Risk |
| --- | --- | --- |
| Printer control | start, pause, resume, cancel, emergency stop, upload/send file | Physical device control and material waste |
| File operations | upload, download, delete, thumbnail generation | Arbitrary file handling, data leakage, disk usage |
| Backup/restore | create backup, restore database, retention cleanup | Data loss or rollback to stale state |
| Integrations | Etsy/Squarespace/Shopify sync, generic sales-channel providers, webhooks, OAuth callbacks | External account access and spoofed events |
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

- [x] CORS restricted by explicit allowed origins for self-hosted browser access.
- [x] Baseline browser hardening headers applied to all routes.
- [x] Authentication/authorization model documented and implemented or clearly delegated to a trusted reverse proxy.
- [x] Sensitive printer/file/backup endpoints inventoried and risk-classified.
- [x] Sensitive printer/file/backup endpoints protected by auth middleware or a documented trusted reverse proxy boundary.
- [x] Slicer port left unpublished unless an operator explicitly asks for LAN access.
- [ ] Webhook signature verification documented and tested for each provider.
- [ ] Secrets loaded from environment/config, not stored in source or logs.
- [ ] Backup/restore tested in an isolated temporary environment.
- [ ] Security-sensitive operations have regression tests.

## Optional Application Auth (API Token)

By default (local development), the API allows all requests. For simple self-hosted exposure where a reverse proxy with authentication is not possible, PicoFarm supports an optional shared API token.

To enable it, set `API_TOKEN` in the environment:

```sh
API_TOKEN="your-secure-token-here"
```

When set, all routes under `/api/*` (except `/api/public/*`) will require this token. Clients can provide it via:

- `Authorization: Bearer your-secure-token-here`
- `X-API-Token: your-secure-token-here`

This is a baseline mechanism. It is strongly recommended to use a robust reverse proxy (e.g. Tailscale, Cloudflare Access, Authelia, Nginx Basic Auth) for broad internet exposure.

### Named tokens (per-integration credentials)

Alongside the shared `API_TOKEN`, named tokens can be minted and revoked at runtime through
`/api/api-tokens` (see `docs/API_CONTRACTS.md` and the Settings → API Tokens page). The design
points that matter for security review:

- **Only a SHA-256 hash is stored.** The plaintext is returned once, by `POST /api/api-tokens`, and
  never again. A database leak does not yield replayable credentials.
- **Scopes are `/api` route prefixes.** `*` means full access; anything else (e.g. `/api/slicer`)
  allows that route and its sub-paths only. A scoped token hitting another route gets `403`, not
  `200`: the failure is visible rather than silent.
- **Revocation is immediate.** The in-memory validation cache is dropped on every create/revoke, so
  a retired token stops working on the next request — not after a cache TTL.
- **The routes live inside `/api`**, so managing credentials requires a credential whenever auth is
  enabled.

Known limitations of this mechanism, stated rather than implied away:

- **Per-process cache.** Validation results are cached for 30s per process so authentication does
  not queue behind SQLite writes on the single connection. With more than one backend replica, a
  revocation can take up to that TTL to reach the other processes. A single instance (the current
  deployment) is unaffected.
- **The WebSocket token travels in the query string.** Browsers cannot set headers on a WebSocket,
  so `?token=` is the only option that works for the bundled UI. That means the token can appear in
  proxy access logs and browser history. Use `X-API-Token`/`Authorization` from non-browser clients
  that connect directly.
- **Scopes are route prefixes, not actions.** `/api/settings` allows deleting settings, not just
  reading them. A scope narrows the surface; it is not a permission model.
- **No expiry, no rotation.** Tokens live until revoked. Rotation is revoke-and-create.
- **`last_used_at` lags by design.** It is written at most once per minute per token, so the page
  can show a timestamp up to a minute old.
- **Two tokens with the same plaintext would collide on their hash.** Irrelevant in practice (32
  random bytes), but it is why generation uses `crypto/rand` and never a name-derived value.

### Turning auth on without a shared token

`REQUIRE_API_TOKEN=1` (also `true`, `yes`, `on`, `enabled`) enables authentication even when
`API_TOKEN` is empty, so a deployment can operate on named tokens alone. If it is set while no
token service is available, the backend logs a warning and `/api` stays open — the fail mode is
open, not closed, and it is loud.

### `/ws` (realtime endpoint)

`/ws` sits outside the `/api` group, so it does its own checks, in this order:

1. **Origin allowlist, always on.** Three cases are accepted: no `Origin` header (a non-browser
   client), an origin listed in `ALLOWED_ORIGINS`, and **the server's own scheme+host** — the
   same-origin case. The last one matters because the defaults are loopback-only
   (`http://localhost:*`, `http://127.0.0.1:*`, `http://[::1]:*`): without it, an operator reaching
   the app by LAN IP or hostname would lose `/ws` the moment `ALLOWED_ORIGINS` is unset. A browser
   page on any other origin cannot open a socket.
2. **Token, only when auth is on.** With `API_TOKEN` set (or `REQUIRE_API_TOKEN`), the handshake
   must carry the token. Browsers cannot set headers on a WebSocket, so the frontend sends it as a
   `?token=` query parameter; non-browser clients may use `X-API-Token` or
   `Authorization: Bearer`. Named tokens connect when a scope covers `/ws`.

The order is deliberate: an unallowed origin fails at the handshake, before any token is read, so
probing cannot distinguish "bad origin" from "bad token". The check runs twice on purpose — once in
the hub and once in the WebSocket library — with the configured origins converted to the
library's host-only pattern form, so a change to either is caught by the other.

### What `API_TOKEN` does not cover

- The slicer neighbor service is a separate concern: `API_TOKEN` guards `/api/*` only (see
  "Neighbor service exposure (orca-slicer-api)").
- With `API_TOKEN` unset and `REQUIRE_API_TOKEN` off, `ApiTokenAuth` passes every request through,
  so the whole `/api` surface depends on the network boundary in front of it. This is the
  local-development default and the only state in which the API is open.
- Named tokens exist independently of enforcement: creating one while auth is off prepares a
  credential but protects nothing until `API_TOKEN`/`REQUIRE_API_TOKEN` is set. The API Tokens page
  states this instead of implying protection.
- The mechanism is a single shared bearer token or a short opaque token: no per-user identity, no
  expiry, no rotation. Revoke-and-recreate is the rotation path.

## Neighbor service exposure (orca-slicer-api)

The slicer is a separate container that PicoFarm only proxies. It carries **no authentication of
its own** and answers CORS with `Access-Control-Allow-Origin: *` for `GET, POST, DELETE,
OPTIONS`. Anything that can reach its port can list, overwrite and **delete** profiles and start
slices; a browser page served from any origin can drive it too.

Both containers normally live in the same compose project. In that layout:

- pin `SLICER_CONNECTION_URL` to the compose service name (e.g.
  `http://orca-slicer-api:3000`) so PicoFarm reaches the slicer over the internal container
  network — no published port and no hairpin NAT through the host;
- **do not publish the slicer port by default**. Publishing it puts an unauthenticated,
  `CORS: *` read/write/delete surface on the LAN for the whole time the stack is up;
- publishing the port is an explicit operator choice for direct access from another device
  (testing, other tooling). Treat every host that can reach it as a slicer operator: profile
  deletion and arbitrary slicing are both possible. Deleting the slicer service's `ports:`
  block closes the exposure again without affecting PicoFarm, because the internal network
  name keeps working.

The app's own `API_TOKEN` does **not** protect the slicer: the token guards `/api/*`, while
the slicer port speaks its own protocol with no token in it.

## Agent rules

Agents working on PicoFarm must:

- avoid printing secrets;
- prefer fake clients and temp directories over real printers/integrations;
- stop and ask before using real credentials or hardware;
- document new security assumptions in this file;
- include security-impact notes in the final report when touching sensitive endpoint classes.
