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

- **Management endpoint `POST /api/v0.9/api-portals/{apiPortalId}/webhook-secret`.**
  Scope `ap:api_portal:manage`. Generates (or rotates) the per-portal secret
  and returns `{ secret, receiverPath }` **once** — the plaintext is not
  retrievable from Platform API after this response. The caller registers the
  returned secret and receiver URL (Platform API base URL + `receiverPath`)
  on the portal via its admin REST API, as any other webhook subscriber.
  No portal restart required.

- **PDK methods `APIPortals.EnsureWebhookSecret` / `RotateWebhookSecret`.**
  Used by the apim-saas managed-portals plugin to borrow a per-portal signing
  secret and the per-portal receiver URL at seed time, so a leaked secret in
  cloud only affects the one portal's organisation.

### Deprecated

- **Legacy webhook receiver route `POST /api/internal/v0.9/webhook/events`.**
  Formally deprecated in this release and unmounted by default. The new config
  key `platform_api.webhook.legacy_shared_secret` defaults to `false`; an
  operator whose portals still sign with the global secret sets it to `true`
  to keep the route mounted while they migrate. The route code is retained as
  a reversible fallback. Outright removal is deferred to a later release.

- **`platform_api.webhook.secret` (the single global signing secret).** Used
  only by the legacy route and only while `legacy_shared_secret=true`. When
  `legacy_shared_secret=true` is set together with an empty `secret`,
  Platform API refuses to start — the combination is unsafe (a flag saying
  "accept legacy deliveries" without a secret to verify against).

### Changed

- **Platform API config `platform_api.webhook`.** New keys: `legacy_shared_secret`
  (bool, default `false`) and `rotation_grace` (duration, default
  `10m` — operator-configurable; negative values refuse to start).
- **Per-portal receiver's in-memory key cache now expires after 60 seconds**
  (previously held until an explicit rotation on the same replica). This
  bounds how long a rotation performed on one replica takes to be honoured
  on every other replica, so a rotated-out secret cannot keep being accepted
  on a stale replica past the grace deadline. Each portal's row is also
  lazy-cleared of the previous slot once the grace window has passed, so
  `previous_secret_enc` does not carry ciphertext for a rotated-out secret
  indefinitely.

### Upgrade notes

- **Action required for portals signing with the global secret.** The legacy
  route is unmounted by default, so deliveries to
  `POST /api/internal/v0.9/webhook/events` return `404` after upgrade. Either
  migrate every portal before upgrading (below), or set
  `platform_api.webhook.legacy_shared_secret=true` (with
  `platform_api.webhook.secret` still configured) to keep the route mounted
  while migrating.
- Operators migrate portals to the per-portal receiver by (a) calling
  `POST /api/v0.9/api-portals/{id}/webhook-secret` for each portal on Platform
  API, then (b) registering the returned secret and receiver URL on the
  portal via its admin REST API (any other subscriber flow, UI or API). The
  portal then signs with the per-portal secret and Platform API verifies at
  the matching per-portal URL. No portal restart required.
- Once every portal is migrated, leave `legacy_shared_secret` unset (or
  `false`) and drop `platform_api.webhook.secret`.
