# Changelog

All notable changes to Platform API, API Portal, and sibling components in this
repository. Entries are grouped per release; the top-most heading is the
next-shipping release. The format is based on [Keep a Changelog][kac]; this
file is created in service of a specific feature and is intentionally light —
earlier releases predate it and are tracked only in commit history and GitHub
release notes.

[kac]: https://keepachangelog.com/en/1.1.0/

## Unreleased

### Added

- **Per-portal webhook signing secrets.** Platform API now stores one HMAC
  signing secret per API Portal, keyed on `api_portals.uuid`. A new per-portal
  receiver route `POST /api/internal/v0.9/webhook/portals/{portalUUID}/events`
  verifies deliveries with that portal's secret, falls back to the previous
  secret during a bounded grace window after a rotation, and rejects any
  delivery whose payload organisation does not resolve to the sending portal's
  own organisation. A compromised portal pod can now forge events only for the
  one organisation that owns that portal.

- **OSS management endpoint `POST /api/v0.9/api-portals/{apiPortalId}/webhook-secret`.**
  Scope `ap:api_portal:manage`. Generates (or rotates) the per-portal secret
  and returns `{ secret, receiverPath }` **once** — the plaintext is not
  retrievable from Platform API after this response. The operator pastes the
  secret, and the receiver URL (Platform API base URL + `receiverPath`), into
  the API Portal's `config.toml` under
  `[api_portal.webhooks.platform_api]` and restarts the portal; the portal
  self-seeds its `platform-api` webhook subscriber row from that config. The
  OSS README's "Webhook signing secret for Platform API deliveries" section
  states the ordering explicitly.

- **`ApiPortalResponse.webhookSecretConfigured`.** Single-portal reads
  (`GET /api-portals/{apiPortalId}`, `POST /api-portals`, `PUT
  /api-portals/{apiPortalId}`) now carry a boolean reporting whether a
  per-portal signing secret exists for that portal. The plaintext is never
  returned on reads.

- **PDK methods `APIPortals.EnsureWebhookSecret` / `RotateWebhookSecret`.**
  Used by the apim-saas managed-portals plugin to borrow a per-portal signing
  secret and the per-portal receiver URL at seed time, so a leaked secret in
  cloud only affects the one portal's organisation.

- **OSS admin-API refusal of config-owned subscriber row.** When
  `config.webhooks.platformApi.url` and `.secret` are both set, the API
  Portal's admin API returns `403` on `PUT /webhook-subscribers/platform-api`
  and `DELETE /webhook-subscribers/platform-api`. All other subscriber ids
  behave as before. The subscriber DTO carries `configOwned: true` for the
  config-driven row so admin UIs can grey out the edit affordance.

### Deprecated

- **Legacy webhook receiver route `POST /api/internal/v0.9/webhook/events`.**
  Formally deprecated in this release. The route stays mounted in this release
  so existing deployments continue to accept deliveries from portals still
  signing with the global secret — the new config key
  `platform_api.webhook.legacy_shared_secret` defaults to `true` here. In the
  next release the default flips to `false` and the route is unmounted unless
  an operator explicitly opts in. The route code is retained one further
  release as a reversible emergency fallback. Outright removal is deferred to
  a later release.

- **`platform_api.webhook.secret` (the single global signing secret).** Used
  only by the legacy route and only while `legacy_shared_secret=true`. When
  `legacy_shared_secret=true` is set together with an empty `secret`,
  Platform API refuses to start — the combination is unsafe (a flag saying
  "accept legacy deliveries" without a secret to verify against).

### Changed

- **Platform API config `platform_api.webhook`.** New keys: `legacy_shared_secret`
  (bool, default `true` in this release) and `rotation_grace` (duration, default
  `10m` — operator-configurable; negative values refuse to start).

### Upgrade notes

- Existing deployments continue to work on upgrade without any operator step.
  Every delivery that was accepted before upgrade is still accepted after
  upgrade through the legacy route.
- Operators migrate portals to the per-portal receiver by (a) calling
  `POST /api/v0.9/api-portals/{id}/webhook-secret` for each portal on Platform
  API, (b) pasting the returned secret and URL into the portal's
  `config.toml`, and (c) restarting the portal. The portal then signs with the
  per-portal secret and Platform API verifies at the matching per-portal
  URL.
- In the next release, `legacy_shared_secret` defaults to `false`. Operators
  who have not migrated every portal by then should either migrate before
  upgrading or flip the flag back on in configuration to buy more time; the
  flag stays available one further release as an emergency fallback.
