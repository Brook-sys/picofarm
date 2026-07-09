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

Initial adapters currently live in `internal/service/sales_channel_adapters.go` so they can wrap existing Etsy, Squarespace, and Shopify services without moving legacy business logic yet. They expose descriptors, capabilities, connection status, and generic sync entry points while read-model conversion for external orders/products remains a follow-up.

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
| `POST /api/sales-channels/{channel}/connect` | Connect/configure a provider, with provider-specific body validated by adapter. |
| `POST /api/sales-channels/{channel}/disconnect` | Disconnect provider or connection. |
| `GET /api/sales-channels/{channel}/auth-url` | Start OAuth where supported. |
| `GET /api/sales-channels/{channel}/callback` | OAuth callback where supported. |
| `POST /api/sales-channels/{channel}/sync` | Trigger sync for `orders`, `products`, or `all`. |
| `GET /api/sales-channels/{channel}/sync-runs` | List sync history. |
| `GET /api/sales-channels/orders` | List external orders, filterable by channel/connection/processed/status. |
| `GET /api/sales-channels/orders/{id}` | Get one canonical external order. |
| `POST /api/sales-channels/orders/{id}/process` | Convert external order into internal PicoFarm order/workflow. |
| `GET /api/sales-channels/products` | List external products/listings, filterable by channel/linked/status. |
| `GET /api/sales-channels/products/{id}` | Get one canonical external product. |
| `POST /api/sales-channels/products/{id}/link` | Link product/variant/SKU to a PicoFarm project. |
| `DELETE /api/sales-channels/products/{id}/link` | Remove a product link. |
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
- Tests must use fake clients, fixtures, or local test servers. Do not require real marketplace accounts in CI.

## Regression matrix additions

When changing sales channels, select validation based on impact:

| Change | Suggested validation |
| --- | --- |
| Descriptor/registry only | `go test ./internal/saleschannel -count=1`, full Go tests if package wiring changes. |
| Storage/migration | repository tests, fresh DB test, upgrade/partial-upgrade test if compatibility logic changes. |
| Provider adapter | fake-client conversion tests and sync tests for that provider. |
| Generic API route | handler tests for success, unknown channel, unsupported capability, auth/secret redaction. |
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
