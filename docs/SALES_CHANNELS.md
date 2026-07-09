# Sales Channels Architecture

This document is the canonical guide for PicoFarm sales-channel integrations. Use it when changing Etsy, Squarespace, Shopify, or any future commerce channel.

The goal is a modular, capability-driven architecture where each external marketplace or storefront is implemented as a provider behind a common contract. New contributors should be able to add a channel by following this document without reverse-engineering every existing integration.

> Current status: the generic `internal/saleschannel` contracts, provider registry, canonical storage repository, initial legacy-backed adapters for Etsy, Squarespace, and Shopify, generic descriptor/status/sync HTTP routes, and frontend descriptor/status discovery plus sync dispatch are implemented. Existing provider-specific routes (`/api/integrations/{provider}/*`) remain supported while connect/read-model/order-processing flows continue to migrate behind `/api/sales-channels/*`.

## Design goals

- Keep Etsy, Squarespace, Shopify, and future channels isolated behind provider adapters.
- Preserve existing integrations while introducing the common layer incrementally.
- Make the frontend provider-driven: render channels from descriptors and capabilities rather than hardcoded branches.
- Normalize orders, products/listings, sync results, product links, and webhook events enough for shared workflows.
- Preserve provider-specific data in raw JSON fields so future features are not blocked by an overly narrow abstraction.
- Keep external integrations testable with fake clients and fixtures; CI must not require real credentials.
- Document every provider well enough for humans and agents with no project history.

## Glossary

| Term | Meaning |
| --- | --- |
| Sales channel | A commerce source such as Etsy, Squarespace, Shopify, Amazon Handmade, eBay, WooCommerce, Mercado Livre, CSV/manual import, or a custom storefront. |
| Provider | Backend implementation for one channel. A provider exposes descriptors, capabilities, status, sync, order/product access, and optional webhooks. |
| Connection | One configured account/store for a channel. The first UI may support one connection per channel, but storage and contracts should not block multiple stores later. |
| Capability | A feature supported by a provider, such as OAuth, API-key auth, order sync, product sync, inventory write, or webhooks. |
| External order | Canonical copy of an order/receipt imported from a provider before or after it is converted into PicoFarm's internal `Order`. |
| External product | Canonical copy of a listing/product/variant imported from a provider and optionally linked to a PicoFarm project/SKU. |
| Product link | Mapping between an external product or variant and a PicoFarm project/SKU. |
| Sync run | Auditable record of one sync attempt, including channel, kind, counts, status, timestamps, and sanitized error. |
| Webhook event | Inbound event from a provider, stored with signature verification status, processing status, and replay metadata when supported. |

## Current implementation map

Existing provider-specific integration code lives in these areas:

| Channel | Backend model/repo/service/API | Provider client | Notes |
| --- | --- | --- | --- |
| Etsy | `internal/model/etsy.go`, `internal/repository/etsy.go`, `internal/service/etsy.go`, `internal/api/etsy_handler.go` | `internal/etsy/client.go` | OAuth, receipts/orders, listings, links, inventory/webhooks. |
| Squarespace | `internal/model/squarespace.go`, `internal/repository/squarespace.go`, `internal/service/squarespace.go`, `internal/api/squarespace_handler.go` | `internal/squarespace/client.go` | API-key style connection, orders/products sync and links. |
| Shopify | `internal/repository/shopify.go`, `internal/service/shopify.go`, `internal/api/shopify_handler.go`, Shopify model shapes in `internal/model/models.go` | service-level HTTP/OAuth code | Existing support is partial and should be exposed by capabilities, not assumptions. |
| Mercado Livre | `internal/service/sales_channel_adapters.go` provider shell, `internal/saleschannel/types.go` channel ID | `internal/mercadolivre/client.go` with injected HTTP client/fakes, `ListOrders` via `/orders/search`, and `ListItems` via `/users/{id}/items/search` + `/items/{id}` | Descriptor/capability contract and fakeable client are registered; `Sync(orders)` imports Mercado Livre orders idempotently via `UpsertExternalOrder`, and `Sync(products)` imports active listings idempotently via `UpsertExternalProduct` with SKU/stock variants for generic product linking. Live OAuth/write inventory/webhooks are follow-up ML cards. |
| Shopee | Planned provider only; no code should be added before the SHP discovery/MVP cards are complete. | Future injectable Open Platform client. | Official Open Platform docs expose signed API calls, shop authorization, order/product/stock endpoints, push notifications, and sandbox testing. Brazil/regional availability and partner access must be confirmed for the user's account before enabling capabilities. |

Current provider-specific route groups remain supported during migration:

- `/api/integrations/etsy/*`
- `/api/integrations/squarespace/*`
- `/api/integrations/shopify/*`

The generic route group starts at `/api/sales-channels/*`.

## Target backend shape

Provider-neutral contracts live in:

```text
internal/saleschannel/
  types.go
  provider.go
  registry.go
```

Initial adapters currently live in `internal/service/sales_channel_adapters.go` so they can wrap existing Etsy, Squarespace, and Shopify services without moving legacy business logic yet. Mercado Livre is registered there as a provider shell so descriptor/capability discovery can drive the UI before the live client is added. They expose descriptors, capabilities, connection status, and generic sync entry points while read-model conversion for external orders/products remains a follow-up.

Generic API handlers live in `internal/api/sales_channel_handler.go`. They expose registered provider descriptors, current connection status, and capability-checked sync dispatch without returning credentials, OAuth codes, API keys, or raw provider payloads.

Frontend sales-channel API types live in `web/src/types/index.ts`, the provider-neutral client lives in `web/src/api/client.ts`, and `web/src/pages/Channels.tsx` now discovers channel display names/status from `GET /api/sales-channels` and dispatches sync through `POST /api/sales-channels/{channel}/sync`. It still keeps legacy Etsy/Squarespace data, process, and link calls until those generic flows exist.

Handlers stay thin in `internal/api`. Business orchestration belongs in `internal/saleschannel` or `internal/service`; persistence belongs in `internal/repository`.

### Provider descriptor and capabilities

Providers should describe themselves rather than requiring the UI to know each channel ahead of time.

Canonical capabilities should include at least:

- `oauth`
- `api_key`
- `orders_read`
- `products_read`
- `inventory_write`
- `webhooks`

A provider must return a descriptor with ID, display name, description, auth type, capabilities, and optional documentation URL.

### Provider interface

The first interface should stay pragmatic. Do not add methods until the application needs them.

Required concepts:

- `Descriptor()`
- `Status(ctx)`
- `Sync(ctx, kind)`
- `ListOrders(ctx, filter)`
- `GetOrder(ctx, externalID)`
- `ProcessOrder(ctx, externalID)`
- `ListProducts(ctx, filter)`
- `LinkProduct(ctx, externalProductID, projectID, sku)`
- `UnlinkProduct(ctx, externalProductID, projectID)`

Providers may return a clear unsupported-capability error when a method does not apply. The handler/service should translate unsupported operations into a stable API error without panics.

### Provider-neutral connect/auth plan

Generic connection endpoints should be planned before legacy credentials move behind `/api/sales-channels/*`. Keep the first implementation as a thin orchestration layer over provider-specific services rather than a credentials rewrite.

Contract additions should introduce small typed request/response structs in `internal/saleschannel`:

- `ConnectRequest`: provider `channel`, optional `connection_id`, auth method, and provider-specific settings in a sanitized `config` map. Secrets such as API keys are accepted from the request body but are never echoed back.
- `ConnectResult`: channel, connection state, display/account metadata, and a redacted `requires_action`/`message` when additional OAuth or setup work is needed.
- `AuthURLRequest`: channel, optional `redirect_uri`, optional provider settings such as Shopify `shop`, and a server-generated `state`.
- `AuthURLResult`: `auth_url`, `state`, and `expires_at`; never include client secrets, code verifier, access tokens, refresh tokens, or signed headers.
- `CallbackRequest`: channel, provider callback query, and verified `state`. OAuth `code` is accepted only at the handler/service boundary and is not logged or returned.
- `DisconnectRequest`: channel and optional `connection_id`, with a future `revoke_remote` flag when provider APIs support token revocation.

Provider behavior by auth family:

| Auth family | Providers | Generic flow | Notes |
| --- | --- | --- | --- |
| OAuth | Etsy, Shopify, Mercado Livre | `GET /auth-url` creates/validates state and returns URL; `/callback` validates state then delegates token exchange. | State must be unguessable, single-use where supported, and tied to redirect/provider metadata. Callback errors must be sanitized before redirect/query strings are built. Mercado Livre uses OAuth 2.0 with `offline_access` for refresh tokens, provides test users instead of a dedicated sandbox, and exposes application rate limits (for example 18,000 requests/hour in application metadata). |
| API key | Squarespace, future API-key providers | `POST /connect` accepts `{ "api_key": "[REDACTED]" }`, validates with a fakeable provider service, persists only through the existing credential path, and returns redacted connection metadata. | Never return the key, store it in `Connection.ConfigJSON`, or include it in sync-run errors. |
| Signed partner API | Shopee | `POST /connect` should accept partner/shop identifiers and secret material as `[REDACTED]`; provider client owns HMAC signing. | Tests should assert signing inputs without real credentials. |
| Manual/import-only | OLX fallback, CSV/manual | `POST /connect` creates local metadata only and reports limited capabilities. | Do not imply live API support until provider availability is validated. |

Implementation sequence:

1. Add connection/auth interfaces only after this plan is accepted: `Connector`, `OAuthConnector`, and/or optional methods on provider adapters. Keep unsupported methods explicit.
2. Add generic handler tests first for unknown channel, unsupported capability, missing required body fields, OAuth state errors, and redaction of `api_key`, `client_secret`, `code`, access/refresh tokens, and signed headers.
3. Wire legacy-backed adapters one provider at a time: Squarespace API-key connect/disconnect first, Etsy OAuth auth-url/callback next, Shopify OAuth after state validation is consistent.
4. Keep legacy `/api/integrations/{provider}/*` routes as compatibility wrappers until the generic flow has equivalent tests and UI support.
5. Update `web/src/pages/Channels.tsx` only after backend contracts are stable; the UI should render connect actions from `AuthType` and capabilities.

### Registry

The registry is responsible for:

- registering providers by unique channel ID;
- returning descriptors/status for all channels;
- looking up one provider by channel ID;
- returning clear errors for unknown providers;
- preventing accidental duplicate registration.

Registry tests must cover duplicate IDs, missing providers, descriptor listing, and stable ordering if the UI depends on order.

## Canonical storage model

Introduce generic storage without deleting provider-specific tables in the first migration. Prefer idempotent upserts and unique provider keys.

Recommended tables:

| Table | Purpose |
| --- | --- |
| `sales_channel_connections` | Configured channel/account metadata, status, capabilities, last sync timestamps, and sanitized last error. |
| `sales_channel_sync_runs` | One row per sync attempt with kind, status, counts, timestamps, and sanitized error. |
| `external_orders` | Provider-neutral imported orders/receipts linked optionally to internal `orders`. |
| `external_order_items` | Provider-neutral imported line items linked to an external order. |
| `external_products` | Provider-neutral imported products/listings. |
| `external_product_variants` | Provider-neutral product variants/SKUs. |
| `sales_channel_product_links` | Mapping from external product/variant/SKU to PicoFarm project. |
| `sales_channel_webhook_events` | Provider-neutral inbound event inbox when webhooks are supported. |

Important constraints:

- external orders should be unique by connection/channel and provider order ID;
- external products should be unique by connection/channel and provider product ID;
- use `raw_json` text/blob fields for provider-specific payloads;
- store secrets only in existing secure config mechanisms or dedicated credential tables with redaction rules;
- never return raw credential material from API responses.

If multiple stores per channel are required now or likely soon, include `connection_id` on external orders/products/links/webhook events from the beginning. If the UI initially supports one store per channel, the backend can still be future-compatible.

## Generic API contract

The target API group is `/api/sales-channels`. Keep legacy `/api/integrations/{provider}` routes during migration.

Representative routes:

| Route | Purpose |
| --- | --- |
| `GET /api/sales-channels` | List registered provider descriptors, capabilities, and connection status. |
| `GET /api/sales-channels/{channel}` | Descriptor, capabilities, and connection status for one provider. |
| `POST /api/sales-channels/{channel}/connect` | Planned generic connect/configure endpoint. Body is provider-specific but must be validated by the adapter and must never be echoed with secrets. |
| `POST /api/sales-channels/{channel}/disconnect` | Planned generic disconnect endpoint for one provider or connection; should revoke remote credentials only when explicitly supported. |
| `GET /api/sales-channels/{channel}/auth-url` | Planned OAuth start endpoint. Generates/validates state and returns a URL without secrets. |
| `GET /api/sales-channels/{channel}/callback` | Planned OAuth callback endpoint. Validates state, sanitizes provider errors, and redirects without leaking codes/tokens. |
| `POST /api/sales-channels/{channel}/sync` | Trigger sync for `orders`, `products`, or `all`. |
| `GET /api/sales-channels/{channel}/sync-runs` | List sync history. |
| `GET /api/sales-channels/orders` | Implemented read-model endpoint. Lists canonical external orders, filterable by `channel`, `processed`, `status`, `limit`, and `offset`. Responses include line items and omit provider `raw_json`. |
| `GET /api/sales-channels/orders/{id}` | Get one canonical external order. |
| `POST /api/sales-channels/orders/{id}/process` | Implemented action endpoint. Converts one canonical external order into a PicoFarm order, marks the external order processed, and rejects repeat processing with `409`. |
| `GET /api/sales-channels/products` | Implemented read-model endpoint. Lists canonical external products/listings, filterable by `channel`, `linked`, `status`, `limit`, and `offset`. Responses include variants and omit provider `raw_json`. |
| `GET /api/sales-channels/products/{id}` | Planned get-one canonical external product route. |
| `POST /api/sales-channels/products/{id}/link` | Implemented action endpoint. Links product/variant/SKU to a PicoFarm project via canonical `sales_channel_product_links`. |
| `DELETE /api/sales-channels/products/{id}/link` | Implemented action endpoint. Removes a product/variant/project link via canonical storage. |
| `POST /api/sales-channels/{channel}/webhook` | Inbound webhook endpoint where supported. |
| `GET /api/sales-channels/{channel}/webhook-events` | Inspect webhook events where supported. |
| `POST /api/sales-channels/{channel}/webhook-events/{id}/reprocess` | Replay a stored webhook event. |

API rules:

- Use snake_case JSON field names.
- Return capability errors clearly when a channel cannot perform an action.
- Do not leak tokens, API keys, OAuth codes, webhook secrets, or signed headers.
- Keep request/response types synchronized with `web/src/types/index.ts` or feature-local TypeScript types.
- Update `docs/API_CONTRACTS.md` and `docs/SECURITY_ENDPOINTS.md` whenever routes or payloads change.

## Frontend target shape

Create a feature area for sales channels rather than expanding `web/src/pages/Channels.tsx` directly:

```text
web/src/features/sales-channels/
  types.ts
  api.ts
  registry.ts
  components/
    ChannelStatusCard.tsx
    ChannelConnectForm.tsx
    ChannelSyncPanel.tsx
    ExternalOrdersTable.tsx
    ExternalProductsTable.tsx
```

Frontend rules:

- `Channels.tsx` should render descriptors/status/capabilities from the generic API.
- Avoid provider branches such as `if (channel === 'etsy')` except in small adapter boundaries or provider-specific connection forms.
- Render actions from capabilities: OAuth connect, API-key connect, sync orders, sync products, inventory sync, webhooks.
- Keep provider-specific copy/help text in a registry or descriptor, not scattered through the page.
- Show operational health: connected/disconnected, last sync, last error, sync history, and “needs attention” states.
- Keep API types in sync with Go JSON and document any UI-only types clearly.

## Migration strategy

Do not perform a big-bang rewrite. Use these phases:

1. Document the target architecture and update docs references.
2. Add provider contracts and registry with unit tests.
3. Add generic storage and repository tests.
4. Add adapters that wrap existing Etsy/Squarespace/Shopify services.
5. Add generic API routes while keeping legacy routes.
6. Refactor the frontend to consume the generic API.
7. Add sync-run observability and secret-redaction tests.
8. Mark legacy routes as compatibility wrappers only after the generic path is stable.
9. Remove or simplify legacy internals only in a later explicit cleanup cycle.

## Mercado Livre discovery matrix

Use this matrix as the source of truth for the first Mercado Livre implementation cards. It is based on the official Mercado Livre/Mercado Libre Developers documentation inspected during ML-01 and should be re-checked before any live integration because endpoint behavior, scopes, and limits can change.

| Area | Official API shape | PicoFarm capability mapping | Implementation notes |
| --- | --- | --- | --- |
| Authentication | OAuth 2.0 authorization flow with bearer tokens and refresh tokens via `offline_access`. Apps expose callback URL, scopes, active status, and request limits in application metadata. | `oauth`, connection status, future generic `/api/sales-channels/mercado_livre/auth-url` and `/callback`. | Store tokens only through the existing secret/credential path. Never place access/refresh tokens in `Connection.ConfigJSON`, sync-run errors, logs, or API responses. |
| Testing | Mercado Livre does not provide a separate sandbox. Official testing uses test users created via `POST https://api.mercadolibre.com/users/test_user` with a developer token; one seller and one buyer test user are recommended. | Fake-client CI plus optional manual QA with test users. | Do not use personal/production accounts for tests. Test users and test listings can expire or be removed after inactivity; CI must not depend on them. |
| Orders | Order lookup uses `GET https://api.mercadolibre.com/orders/{order_id}` and order search such as `/orders/search?seller={seller_id}`. Order payloads include line items, variation IDs/attributes, seller SKU, amounts, status, pack/shipping IDs, and fraud/shipping context. | `orders_read`, canonical `external_orders`, idempotent order sync, `POST /api/sales-channels/orders/{id}/process`. | ML-04 should map order IDs, pack IDs, order items, variation IDs, SKU, buyer/shipping summary, status, currency, totals, and raw provider payload into canonical storage while omitting `raw_json` from API responses. |
| Products/listings | Seller listing discovery uses `/users/{USER_ID}/items/search` and item multiget via `/items?ids=...`. Public search endpoints expose approximate `available_quantity`; authenticated item resources are needed for precise seller operations. | `products_read`, canonical `external_products`, variants, product links. | ML-05 should sync item ID, title, status, category, listing type, price/currency, available quantity where precise, variation IDs/attributes/SKUs, and permalink/image metadata when available. |
| Inventory/listing updates | Item update uses `PUT https://api.mercadolibre.com/items/{ITEM_ID}`. Active items can update price, pictures, description, shipping, and `available_quantity`; items with sales restrict title/condition/buying mode changes. Variations require updating the item `variations` collection with the relevant variation IDs/stock. | `inventory_write` after read-only sync is stable. | Treat inventory writes as a later capability-gated path. Tests must cover item-level and variation-level stock payload generation with fake clients before any UI action is enabled. |
| Shipping/fulfillment | Orders can contain shipping IDs; official docs direct clients to shipment resources such as `/shipments/{shipping_id}` for logistics mode/status/details. | Future fulfillment/shipping status read capability. | Capture shipping IDs on order sync, but defer fulfillment mutations until a dedicated card because shipping modes differ by site/logistics configuration. |
| Notifications/webhooks | Notifications send a resource path, topic, user ID, application ID, attempts, sent/received timestamps. Topics include `items` and `orders`; clients fetch the referenced resource with bearer auth. | `webhooks`, canonical `sales_channel_webhook_events`, `POST /api/sales-channels/mercado_livre/webhook`, `GET /api/sales-channels/mercado_livre/webhook-events`. | ML-06 stores inbound notification payloads idempotently by channel/event ID (or a deterministic payload hash), keeps raw payload/signature only for replay, and exposes metadata-only event listings. Mercado Livre notification signature support is not assumed in CI; validate provider/account metadata and rely on idempotent replay until a documented signature source is configured. Fetching referenced resources for automatic reprocessing remains a follow-up. |
| Rate limits | Application metadata exposes `max_requests_per_hour` (official examples show 18,000/hour). | Provider throttling/backoff policy. | Add client-level retry/backoff and sync-run counters/errors before live polling. Respect `429`/rate-limit responses and keep errors sanitized. |

Recommended implementation sequence:

1. Add a `mercado_livre` descriptor with OAuth, orders read, products read, inventory write, and webhooks marked according to implemented support, not aspirational support.
2. Build a fakeable client with fixtures for OAuth token refresh, orders, item search/multiget, item update, shipments read, and notifications.
3. Implement read-only order sync first, then products/listings, then inventory write, then notifications/webhooks.
4. Keep every live endpoint behind capabilities and fake-client tests; do not require Mercado Livre credentials for CI.

## Shopee discovery matrix

Use this matrix as the source of truth for the first Shopee implementation cards. It is based on the Shopee Open Platform pages discovered during SHP-01, including the developer guides for shop authorization, API calls, push mechanism, sandbox testing, and V2 order/product/stock documents. Re-check the official docs before implementation because partner access, regional support, and endpoint contracts can change.

| Area | Official API shape | PicoFarm capability mapping | Implementation notes |
| --- | --- | --- | --- |
| Platform/access | Shopee Open Platform is a partner-app portal. Production usage requires a registered partner app and seller/shop authorization; Brazilian availability was found through Shopee Open API Brazil material, but partner/regional eligibility must be confirmed against the user's account. | Planned channel ID `shopee`; do not register a live descriptor until access and MVP scope are confirmed. | Treat SHP-02 as the approval gate for whether Shopee becomes a full sales channel or remains blocked by partner access. Do not use scraping or unofficial endpoints as the integration path. |
| Authentication | Shop authorization redirects the seller to authorize the app and returns an authorization code. Token exchange uses public/auth endpoints to obtain access and refresh tokens tied to `shop_id`/merchant context. | `oauth` plus future generic `/api/sales-channels/shopee/auth-url` and `/callback`. | OAuth `code`, access tokens, refresh tokens, partner keys, and signed query strings must never be logged, returned, stored in `Connection.ConfigJSON`, or embedded in docs except as `[REDACTED]`. State validation and replay protection are required before enabling the callback. |
| Request signing | V2 API calls are signed with HMAC-SHA256 over a base string that includes partner identity, request path, timestamp, and, for shop-scoped calls, token/shop context. Requests include `partner_id`, `timestamp`, and `sign`; shop-scoped calls also use `shop_id` and access token. | Future signed OAuth provider/client, not `api_key`. | Isolate canonical base-string construction and HMAC signing in a small package with table-driven tests using fake IDs/keys. Do not scatter signing logic in services or UI. |
| Orders | Official V2 order docs include order list and order detail endpoints. Order detail returns order numbers, statuses, timestamps, buyer/recipient/shipping fields, currency/totals, and item/model details needed for fulfillment mapping. | `orders_read`, canonical `external_orders`, future `POST /api/sales-channels/orders/{id}/process`. | SHP-04 should map order SN, item IDs, model IDs, seller SKU/model SKU, quantity, totals, currency, order status, recipient/shipping summary, and raw provider payload into canonical storage while omitting `raw_json` from API responses. |
| Products/items/models | Official V2 product docs include item list/base-info and model/variation detail endpoints. Shopee separates item and model/variation concepts; stock/SKU may live at model level for variation listings. | `products_read`, canonical `external_products` and variants. | Product sync must preserve item ID, model ID, item/model SKU, title, status, price/currency, stock, images/permalink when available, and variation metadata for generic product linking. |
| Inventory writes | Official product docs include stock update endpoints for item/model stock. | `inventory_write` only after read-only product sync and link mapping are stable. | Keep inventory writes capability-gated and disabled until fake-client tests prove item-level and model-level stock payloads, validation, and sanitized errors. |
| Webhooks/push | Shopee Open Platform documents a push mechanism for event notifications. | `webhooks`, canonical `sales_channel_webhook_events`, future `POST /api/sales-channels/shopee/webhook`. | Store push events idempotently and expose metadata-only listings. Signature/verification requirements must be confirmed in the official push docs before accepting live callbacks; raw payload/signature must not be echoed. |
| Sandbox/testing | Shopee documents Sandbox Testing V2. | Fake-client CI plus optional sandbox manual QA. | CI must not require real seller credentials. Use local fixtures/fake clients for auth, signing, order list/detail, item/model reads, stock updates, and push events; reserve Shopee sandbox accounts for manual validation. |
| Rate limits/errors | Official docs and partner policies can impose per-app/per-shop limits and signed request expiry behavior. | Provider throttling/backoff and sanitized sync-run diagnostics. | Add retry/backoff for `429`/transient failures before live polling. Persist only sanitized errors through `sales_channel_sync_runs`; never persist full signed URLs, bearer tokens, or partner secrets. |

Recommended implementation sequence:

1. Complete SHP-02 with a Brazil/account-specific MVP decision and capability matrix.
2. Add only a descriptor/provider skeleton once the chosen capabilities are approved; keep unsupported capabilities absent, not aspirational.
3. Build the signed fakeable client first: authorization URL/token exchange, signature helper, order list/detail, item/model reads, stock update, and push-event parsing.
4. Implement read-only order sync, then product/model sync, then product links and inventory writes, then webhook replay.
5. Keep every Shopee path covered by fake-client tests and do not require real Shopee credentials in CI.

## Checklist for adding a new channel

Before coding:

- [ ] Read this document, `docs/API_CONTRACTS.md`, `docs/SECURITY_ENDPOINTS.md`, and `docs/REGRESSION_MATRIX.md`.
- [ ] Decide auth type: OAuth, API key, manual/import-only, or none.
- [ ] Decide capabilities: orders, products, inventory, webhooks, fulfillment updates, etc.
- [ ] Confirm whether multiple accounts/stores must be supported.
- [ ] Identify provider sandbox/testing options and API rate limits.

Backend implementation:

- [ ] Add provider client with injectable HTTP client or fakeable interface.
- [ ] Add conversion tests from provider payloads to canonical external orders/products.
- [ ] Register descriptor and capabilities.
- [ ] Implement status/connect/disconnect/auth callback as applicable.
- [ ] Implement order sync with idempotent upsert.
- [ ] Implement product/listing sync with idempotent upsert if supported.
- [ ] Implement product-to-project/SKU linking if supported.
- [ ] Implement webhook signature verification and event storage if supported.
- [ ] Add handler/service/repository tests using fake clients and fixtures.

Frontend implementation:

- [ ] Add descriptor/help text only where the generic backend descriptor is insufficient.
- [ ] Use generic API helpers and canonical types.
- [ ] Render only actions supported by provider capabilities.
- [ ] Add clear loading, error, empty, disconnected, and needs-attention states.
- [ ] Avoid adding provider-specific branches to `Channels.tsx` unless isolated and documented.

Docs and validation:

- [ ] Update this file if the extension process changes.
- [ ] Update `docs/API_CONTRACTS.md` for new/changed JSON or routes.
- [ ] Update `docs/SECURITY_ENDPOINTS.md` for sensitive routes.
- [ ] Update `docs/REGRESSION_MATRIX.md` for manual and automated validation.
- [ ] Run relevant backend tests, frontend lint/build if UI changed, then full project gate before commit.

## Security rules

- Never commit, print, log, or return raw credentials. Examples must use `[REDACTED]`.
- Treat OAuth callbacks, API-key connection routes, sync routes, product links, inventory updates, and webhooks as security-sensitive integration endpoints.
- Validate OAuth state and redirect configuration.
- Verify webhook signatures where the provider supports them; document providers that cannot sign webhooks.
- Store inbound webhook events idempotently to tolerate duplicate delivery and replay.
- Sanitize external API errors before saving them in `sales_channel_sync_runs` or returning them to the browser.
- `GET /api/sales-channels/sync-runs` is the provider-neutral diagnostic read model for sync attempts. It supports `channel`, `kind`, `connection_id`, `limit`, and `offset` filters and must only return sanitized `last_error` values.
- Tests must use fake clients, fixtures, or local test servers. Do not require real marketplace accounts in CI.

## Regression matrix additions

When changing sales channels, select validation based on impact:

| Change | Suggested validation |
| --- | --- |
| Descriptor/registry only | `go test ./internal/saleschannel -count=1`, full Go tests if package wiring changes. |
| Storage/migration | repository tests, fresh DB test, upgrade/partial-upgrade test if compatibility logic changes. |
| Provider adapter | fake-client conversion tests and sync tests for that provider. |
| Generic API route | handler tests for success, unknown channel, unsupported capability, duplicate processing/linking, auth/secret redaction. |
| Frontend page/API | `cd web && npm run lint && npm run build`, manual channel page smoke check. |
| Webhooks | signature validation tests, duplicate/replay tests, sanitized logging checks. |

## Troubleshooting guide

| Symptom | First checks |
| --- | --- |
| Channel does not appear | Confirm provider is registered, descriptor is returned by `GET /api/sales-channels`, and frontend filters do not hide disconnected providers. |
| Sync fails immediately | Check connection status, sanitized last error, provider credentials, and fake-client test coverage. Do not print secrets. |
| Sync creates duplicates | Check unique keys and upsert by connection/channel plus external ID. |
| Order cannot process | Verify external order items have SKU/project links or supported fallback behavior. |
| Product cannot link | Verify project ID/SKU validation, provider capability, and variant/external product ID mapping. |
| Webhook ignored | Check route, signature verification, event uniqueness, and replay status. |
| Shopify behaves differently from Etsy/Squarespace | Check Shopify capabilities and whether its current implementation is marked partial. |

## Done criteria for the modularization effort

The architecture is considered successful when:

- A new provider can be added by following this document and writing isolated provider/adapter code.
- `Channels.tsx` is mostly provider-agnostic and capability-driven.
- Etsy, Squarespace, and Shopify are represented through descriptors/capabilities.
- Sync runs are auditable and errors are sanitized.
- External orders/products can be listed and processed through generic endpoints.
- Legacy routes continue to work until an explicit deprecation/removal cycle.
- Documentation, API contracts, security inventory, regression matrix, Go tests, TypeScript types, and frontend API helpers remain synchronized.
