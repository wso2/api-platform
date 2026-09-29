# Multi-Tenancy Mode

By default an API Portal instance serves exactly one organization — the one named by
`organization.handle` (see [Manage the Organization](manage-organizations.md)).
**Multi-tenancy mode** lets one portal serve every organization under its
`organization.portal_id` instead: each user works in the organization their identity provider (IDP) says they
belong to, and, if you allow it, an organization the portal hasn't seen before is
created the first time one of its users signs in.

Use it when one IDP (for example WSO2 Identity Server or Asgardeo with B2B
sub-organizations, or Keycloak with Organizations) holds many tenant organizations and
you want a single portal deployment for all of them.

> Multi-tenancy mode needs `auth.mode = "idp"`. Local auth (the Platform API
> login) has no per-user organization claim to route by, so it stays single-organization
> even with the setting on — the portal logs a warning at startup if you combine them.

## Quick start

```toml
[api_portal.multi_tenancy]
enabled = true

[api_portal.auth]
mode = "idp"
# false: provision organizations from token claims, and admit a token with no
# organization claim to organization.handle. Leave at true to refuse both.
enforce_org_validation = false

[api_portal.auth.claim_mappings]
organization = "org_id"       # claim carrying the organization's stable id
org_name     = "org_name"     # optional: names a provisioned organization
org_handle   = "org_handle"   # optional: its URL handle, taken as-is

[api_portal.auth.idp]
# Required in this mode: the audience bearer tokens must be issued for (usually the
# client id). Several may be given, comma-separated or as an array.
audience = "<client id>"
```

Everything else about IDP authentication is configured exactly as for a single
organization — see [Authentication](authentication.md).

## Deployment requirements

- **The deployment owns its `portal_id`.** Every row the portal stores is keyed by
  `organization.portal_id`, and in this mode any instance may serve — and deliver
  webhooks for — any organization under it. Several instances may share a `portal_id`
  only as replicas of the same deployment, with identical configuration. Don't run any
  other portal, single-organization or multi-tenancy, with the same `portal_id`.
- **Other portals may share the database under their own `portal_id`.** A
  single-organization portal, or a second multi-tenancy portal, sees none of this
  portal's organizations, sessions or events, and this portal sees none of theirs. An
  IDP organization that signs in to two multi-tenancy portals is provisioned separately
  in each.
- **Replicas share `security.encryption_key`.** Webhook subscriber secrets are
  encrypted with it, and any replica may be the one that signs a delivery or encrypts a
  generated API key for a subscriber.
- **Provisioning needs verified TLS to the IDP.** See [Provisioning](#provisioning).
- **`auth.idp.audience` is required.** The portal refuses to start without it in this
  mode: otherwise a token the IDP issued to any other application would be accepted for
  every organization. Browser logins are unaffected — their tokens come from the
  portal's own code exchange.

At startup the portal logs that it is running in multi-tenancy mode and lists the
organizations it found under its `portal_id`, so you can check it is pointed at the
database and `portal_id` you meant. It
also logs an error for organizations that share an `idp_ref_id`: sign-ins to them are
refused as ambiguous until one is changed.

## How organizations are resolved

| Where | Single-organization mode | Multi-tenancy mode |
|---|---|---|
| Page URL `/api-portal/<handle>/...` | only `organization.handle`; anything else `404` | any existing organization with exactly that handle; unknown handles `404`. A URL never creates an organization. |
| Org claim of a session or bearer token | must name `organization.handle`'s organization | matched against organizations' `idp_ref_id`, **exactly** — never a handle or display name |
| `GET`/`PUT /organizations/{orgId}` | only the configured organization | only the **caller's own** organization |
| Public theme assets and API icons | the configured organization's | the organization whose page is being rendered |
| Webhook delivery | the configured organization's events | every organization's events |

A user's session belongs to one organization for its whole lifetime. Signed-in pages
(applications, subscriptions, API keys, settings) of any other organization return
`403`, and Settings is not shown there even to an administrator; public pages of every existing organization are open to anyone.

An org claim that is a list or a map (Keycloak Organizations can send either) is
accepted when it names exactly one organization and refused (`403`) when it names
several. An `idp_ref_id` shared by more than one organization is refused as ambiguous.

## `enforce_org_validation`

| Claim in the token | Single-org, `true` (default) | Single-org, `false` | Multi-tenancy, `true` | Multi-tenancy, `false` |
|---|---|---|---|---|
| The configured organization | allowed | allowed | allowed | allowed |
| Another existing organization | `403` | `403` | allowed, as that organization | allowed, as that organization |
| An organization that doesn't exist yet | `403` | `403` | `403` | **provisioned**, then allowed |
| No organization claim | `403` | the configured organization | `403` (login refused) | the configured organization |

With `false`, a login without an organization claim is recorded in the session as the
configured organization, so every later check treats it exactly like a login that
asserted it. A claim naming an organization is never redirected to the configured one.
The portal logs a warning at startup whenever this setting is `false`.

## Provisioning

With `multi_tenancy.enabled = true` and `enforce_org_validation = false`, a verified token
whose org claim names no known organization creates it, with the same defaults as the
configured organization gets on first start: a `default` view and label, and (with
`organization.auto_create_subscription_plans`) the default subscription plans.

- `idp_ref_id` is the org claim's value, verbatim.
- The URL handle is the `claim_mappings.org_handle` claim, lowercased, when
  the token carries it and it is a usable handle (only `a-z 0-9 . _ -`, starting with a
  letter or digit, at most 128 characters, not reserved). The IDP's handle is never
  rewritten to fit: an unusable one is logged and ignored.
- Otherwise the handle is derived from the `claim_mappings.org_name` claim,
  or failing that from the org claim: lowercased, accents removed, anything outside
  `a-z 0-9 . _ -` collapsed to `-`, at most 48 characters.
- The display name is the organization-name claim, else the handle claim, else the org
  claim.
- If another organization already has that handle or display name, a short suffix
  derived from the claim is added (`acme-3f2a1c`, `Acme (3f2a1c)`).
- The handle and display name are set once, at provisioning. Renaming the organization
  in the IDP later doesn't change them.
- Concurrent first logins for the same new organization create it exactly once.
- There is no limit on how many organizations can be provisioned.

Provisioning is refused — the login or request gets `403`, and the reason is logged —
unless the claim arrived over verified TLS:

- **Browser logins:** `auth.idp.token_url` must be `https` (plain `http` is allowed
  only to `localhost`/loopback). The ID token comes straight from the token endpoint
  over that connection and is trusted on its strength.
- **Bearer tokens:** a pinned `auth.idp.certificate`, or `auth.idp.jwks_url` over
  `https`.
- Either way, `NODE_TLS_REJECT_UNAUTHORIZED=0` disables provisioning. To trust a
  self-signed IDP certificate, add it with `NODE_EXTRA_CA_CERTS` instead.

Existing organizations keep working when provisioning is refused.

## Signing in

Every login returns through the single `auth.idp.callback_url`. In multi-tenancy
mode:

- **The configured organization's login page sends no organization hint** to the IDP,
  so the IDP runs its own login and organization selection. This is how a user whose
  organization the portal hasn't seen yet signs in (their organization's own pages don't
  exist yet).
- **Any other organization's login page** sends that organization's `idp_ref_id` as the
  `org` authorization parameter. Only an IDP that accepts `org=<that id>` scopes the
  login by it. WSO2 IS 7.x doesn't for the `org_id` UUIDs used here (it expects
  `orgId=<uuid>`, or `org=<handle>`), so its users choose their organization on the IS
  login page instead; the organization check after login is the same either way.
- **`?org=<id>` on a login URL** overrides the hint — useful as a direct sign-in link
  for one tenant: `/api-portal/default/views/default/login?org=<orgId>`.
- After login the user lands in **their own organization**: back on the page they came
  from if it belongs to it, otherwise on its default view.
- **Silent SSO** (`auth.idp.silent_sso`, on by default) sends the same hint as a login
  from the page being browsed. With an IDP that honours it a visitor is only signed in
  silently on *their own* organization's pages; with one that ignores it (WSO2 IS with
  `org_id` UUIDs, Keycloak, Auth0) they are signed in to their own organization wherever
  they browse — their session still only reaches their own organization's signed-in pages
  and data. Either way a silent sign-in leaves the visitor on the page they were
  reading, and with `enforce_org_validation = false` it can provision the visitor's
  organization exactly as an explicit login would.
- Roles and groups are read from the access token first, then the ID token; a user
  with no display name shows `preferred_username`, `username` or `sub`.

## Bearer tokens

Bearer tokens on the REST API are verified exactly as in single-organization mode:
signed by the keys at `auth.idp.jwks_url` (or the pinned `auth.idp.certificate`),
issued by `auth.idp.issuer`, for `auth.idp.audience`. The organization is then the
one the token's organization claim names.

A token issued under an organization's *own* issuer — for example one obtained directly
from a WSO2 IS or Asgardeo sub-organization's token endpoint (`.../o/<orgId>/oauth2/token`)
— is rejected, as it is by the Platform API. Browser sign-ins to every organization are
unaffected: they go through the configured issuer.

## Webhooks

In multi-tenancy mode the webhook dispatcher and delivery worker handle every
organization's events. Each organization's subscribers receive only that organization's
events (subscribers are registered per organization, through the caller's own token).
Events of other portals in the same database, under a different `portal_id`, are left to
those portals. Nothing extra to configure — see [Webhook Integration](webhook-integration.md).

## IDP recipes

Check each value against your IDP — claim names and issuer formats vary by version and
configuration. Decode a real token (for example with `jq -R 'split(".")[1] | @base64d
| fromjson'`) to confirm `iss` and the organization claims before relying on them.

| IDP | `claim_mappings.organization` | `claim_mappings.org_name` | `claim_mappings.org_handle` | Notes |
|---|---|---|---|---|
| WSO2 IS 7.x | `org_id` | `org_name` | `org_handle` | Shared root app, one sub-organization per tenant. Set `auth.idp_org_id` to the root organization's `org_id` so root users map to the configured organization — see [WSO2 Identity Server Setup](wso2-is-setup.md#the-root-organization). |
| Asgardeo | `org_id` | `org_name` | check your tokens | Same model as WSO2 IS — see [Asgardeo Setup](asgardeo-setup.md). |
| Keycloak (Organizations) | `organization` | — | — | The claim is a list or map of organization aliases; one per user. The portal's `org` login hint is not a Keycloak parameter, so users pick their organization in Keycloak. |
| Auth0 (Organizations) | `org_id` | `org_name` | — | Auth0 prompts for the organization itself; the portal's `org` hint is not Auth0's `organization` parameter. |
| Okta | your custom org claim | — | — | Add the claim to your authorization server's access and ID tokens. |

Keycloak's realm-per-tenant model (a different login endpoint and client per
organization) is not supported: the portal has one IDP login configuration.

## Onboarding a tenant's catalog

APIs published from the Platform API (its shared-key calls to the portal) always land in
the configured organization, `organization.handle`: the portal has one shared key for
all of them, so it doesn't let those calls pick an organization, and one whose
`organization` header names any other organization is refused with `403`. A provisioned
organization gets its catalog through the portal's REST API, with a token issued for it.

`scripts/seed-samples.sh` can seed one organization's APIs and MCP servers with a token
issued for it:

```bash
ACCESS_TOKEN="<a token for the tenant's admin>" \
SAMPLES_DIR="<that tenant's samples directory>" \
PLAN_OVERRIDE="Gold|Silver" \
API_PORTAL_URL="https://portal.example.com" \
  ./scripts/seed-samples.sh
```

`PLAN_OVERRIDE` rewrites each sample's subscription plans to that list, so the catalog
only advertises plans the organization has.

## Turning it off

With `multi_tenancy.enabled = false` the portal serves only `organization.handle` again.
Organizations created while it was on stay in the database but are no longer reachable
— their pages `404`, their users' tokens are refused, and their pending webhook
deliveries stay pending until the mode is turned back on.

## Troubleshooting

| Log line | Meaning |
|---|---|
| `Rejected org claim naming an unknown organization` | The claim matches no organization's `idp_ref_id`, and provisioning is off (`enforce_org_validation = true`). |
| `Refused to provision an organization from an org claim: …` | Provisioning is on, but the IDP channel isn't verified TLS — the reason follows. |
| `Org claim matches more than one organization's idp_ref_id` | Two organizations share an `idp_ref_id`; fix the data. |
| `Multi-tenancy: organizations share an idp_ref_id …` (at startup) | The same, found before anyone signs in; it names the organizations. Common after pointing `auth.idp_org_id` at an id an already-provisioned organization holds — for WSO2 IS's root organization see [The root organization](wso2-is-setup.md#the-root-organization). |
| `Rejected org claim naming more than one organization` | A list/map org claim names several organizations. |
| `Rejected login: token carries no organization claim` | Multi-tenancy mode with enforcement on; check `claim_mappings.organization`. |
