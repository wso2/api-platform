# WSO2 Identity Server Setup

This guide configures a self-hosted **WSO2 Identity Server** (7.1+) as the identity
provider for an API Portal in [multi-tenancy mode](multi-tenancy.md), and then
onboards a tenant organization. Every step uses the IS **Console**
(`https://<is-host>:9443/console`) and the portal's own UI; screen and field names are
as in WSO2 IS 7.3.

For the cloud equivalent, see [Asgardeo Setup](asgardeo-setup.md) — the organization
model is the same.

## Overview

1. One application, **API Portal**, is registered in the IS **root** organization and
   shared with every organization. Each organization gets its own copy of it (shown as a
   *Shared app* in that organization).
2. A portal organization maps to one IS organization through the `org_id` claim in the
   tokens IS issues for it (stored as the portal organization's `idp_ref_id`). With
   `enforce_org_validation = false` the portal creates that organization itself the first
   time one of its users signs in — there is no portal-side registration step.
3. Two roles, `dp_admin` and `dp_subscriber`, are created once on the root application and
   appear in every organization automatically. Assigning a user one of them is what gives
   them the portal's administrator or subscriber tier
   (`[api_portal.auth.authorization.portal_roles]`).
4. Onboarding a tenant is then: create the IS organization, add its users and assign
   roles; its first sign-in to the portal provisions it; its administrator sets it up in
   the portal (Section 4).

## Prerequisites

- WSO2 IS 7.1+ running, and a Console login for the root organization's administrator.
- The API Portal's browser-facing URL decided (this guide uses `http://localhost:9543`).
  The redirect URLs below must match it exactly.

---

## 1. Create the API Portal application

In the root organization's Console:

1. Go to **Applications** → **New Application** → **Traditional Web Application**.
2. Fill in:
   - **Name:** `API Portal`
   - **Protocol:** **OpenID Connect**
   - **Authorized redirect URLs:** `http://localhost:9543/api-portal/default/callback`
     (the portal's `auth.idp.callback_url`)
   - Tick **Allow sharing with organizations**.
3. Click **Create**.

### 1a. Protocol

Open the application's **Protocol** tab:

- **Allowed grant types:** **Code** and **Refresh Token**.
- **Authorized redirect URLs:** add the portal's `auth.idp.logout_redirect_uri` too (for
  example `http://localhost:9543/api-portal/logout`), next to the callback URL.
- **Allowed origins:** the portal's origin, `http://localhost:9543`.
- **Access Token** → **Token type:** **JWT**, and under **Access Token Attributes** select
  `roles` (and `groups` if you map groups).
- Click **Update**.

The **Client ID** and **Client secret** at the top of this tab go into the portal's
`auth.idp.client_id` / `client_secret` (Section 3).

### 1b. User attributes

Without these, users sign in without a name and role-based authorization fails, because
no `roles` claim reaches the portal.

Open the **User Attributes** tab, tick these scopes and click **Update**:

- **Email**
- **Profile** — expand it and select at least **First Name**, **Last Name** and
  **Username**
- **Roles**

The organization claims (`org_id`, `org_name`, `org_handle`) need no attribute: IS adds
them to tokens issued for an organization.

### 1c. Shared access

Open the **Shared Access** tab:

1. Under **Sharing Policy**, select **Share the application with all organizations** and
   click **Save**. This covers organizations created later too.
2. Turn on **Enable enhanced organization login**. Without it, a sign-in to an
   organization succeeds but carries no `roles`, so the portal grants nothing.

---

## 2. Create the `dp_admin` / `dp_subscriber` roles

These names must match `[api_portal.auth.authorization.portal_roles]` in the portal's
configuration (`admin` / `subscriber`, see [Section 3](#3-configure-the-api-portal)).

On the application's **Roles** tab:

1. Leave **Role Audience** at **Application**.
2. Click **+ New Role**, create `dp_admin`; repeat for `dp_subscriber`.
3. Check both are listed under **Assigned Roles**, and click **Update**.

Because the application is shared, both roles appear in every organization on their own —
under **User Management** → **Roles** they are marked *Shared role* with the audience
*application | API Portal*. There is nothing to share per role.

To give a root-organization user (for example the IS administrator, while testing) a
role: **User Management** → **Roles** → open the role → **Users** → **Assign User**.

---

## 3. Configure the API Portal

Set `[api_portal.auth.idp]` to the application's credentials (Section 1a), and the portal
roles to the names from Section 2:

```toml
[api_portal.multi_tenancy]
enabled = true

[api_portal.auth]
mode = "idp"
# The root organization's org_id — see "The root organization" below.
idp_org_id = "10084a8d-113f-4211-a0d5-efe36b082211"
# false: an organization's first sign-in provisions its portal organization.
enforce_org_validation = false

[api_portal.auth.claim_mappings]
organization = "org_id"
org_name     = "org_name"
org_handle   = "org_handle"   # the IS organization handle becomes its portal URL handle
roles        = "roles"

[api_portal.auth.idp]
client_id = "<API Portal client ID>"
client_secret = "<API Portal client secret>"
# Required in multi-tenancy mode.
audience = "<API Portal client ID>"
# The root organization's issuer, which bearer tokens are verified against. Unset, the
# issuer isn't checked at all.
issuer = "https://localhost:9443/oauth2/token"
authorization_url = "https://localhost:9443/oauth2/authorize"
token_url = "https://host.docker.internal:9443/oauth2/token"
callback_url = "http://localhost:9543/api-portal/default/callback"
jwks_url = "https://host.docker.internal:9443/oauth2/jwks"
logout_url = "https://localhost:9443/oidc/logout"
logout_redirect_uri = "http://localhost:9543/api-portal/logout"

[api_portal.auth.authorization]
enabled = true
mode    = "role"
role_to_scope_mapping = "./resources/role-to-scope-mapping.yaml"
page_role_validation = true

[api_portal.auth.authorization.portal_roles]
admin      = "dp_admin"
subscriber = "dp_subscriber"
```

**`authorization_url` / `logout_url` and `token_url` / `jwks_url` use different hosts on
purpose.** The first two are browser redirects: the browser runs on the host, where
`host.docker.internal` doesn't resolve, so use `localhost`. The other two are called by
the portal container itself, which reaches IS through `host.docker.internal`. Running the
portal outside Docker, use `localhost` for all of them.

**Trust IS's certificate** instead of disabling TLS checks: point `NODE_EXTRA_CA_CERTS`
at IS's certificate (in Docker, mount it into the container first). Don't use
`NODE_TLS_REJECT_UNAUTHORIZED=0` — besides disabling verification everywhere, it stops
the portal provisioning organizations (see
[Provisioning](multi-tenancy.md#provisioning)).

**Bearer tokens for the REST API** must come from the root organization's issuer
(`auth.idp.issuer`). A token obtained directly from an organization's own token endpoint
(`.../o/<organization id>/oauth2/token`) is rejected. Browser sign-ins, including every
organization's users, go through the root organization's endpoints, so their tokens carry
that issuer too.

To confirm the issuer, open the root organization's OIDC discovery document in a browser,
`https://localhost:9443/oauth2/token/.well-known/openid-configuration`, and copy its
`issuer` value exactly. The portal compares it character for character, so a different
host name or a trailing slash rejects every token.

### The root organization

Users of the root organization — including the IS super admin — sign in with an `org_id`
claim too: the root organization's own id, which on WSO2 IS 7.x (checked on 7.3) is
`10084a8d-113f-4211-a0d5-efe36b082211` (its `org_name` is `Super`, its `org_handle`
`carbon.super`). Set `auth.idp_org_id` to that value so the portal's configured
organization (`organization.handle`, usually `default`) *is* the root organization.

Leave it unset and `idp_ref_id` of the configured organization defaults to its handle,
which no token carries — so with `enforce_org_validation = false` the first root-org
sign-in provisions a separate organization named `Super` (handle `super`, or
`carbon.super` with `claim_mappings.org_handle` set), and root users land there instead of
in the configured organization.

Confirm the id on your IS by decoding a root user's token (`jq -R 'split(".")[1] |
@base64d | fromjson | .org_id'`). The portal writes `idp_org_id` into the configured
organization at startup, so an existing deployment picks it up on restart; root users
signed in before the change must sign in again.

**If a `Super` organization was already provisioned**, retire it *before* that restart.
Otherwise two organizations carry the root organization's id, and root sign-ins are
refused as ambiguous (the portal also logs this at startup). Organizations can't be
deleted through the portal, so give it an id no token carries — it keeps its data, and
nobody can sign in to it:

```sql
UPDATE organizations SET idp_ref_id = 'retired-super'
 WHERE handle = 'super' AND idp_ref_id = '10084a8d-113f-4211-a0d5-efe36b082211'
   AND portal_id = 'portal_id';
```

Use the handle it was provisioned with (`super`, or `carbon.super` with
`claim_mappings.org_handle` set), and your deployment's `organization.portal_id`, so the
statement can't touch another portal's organization in a shared database.

---

## 4. Onboard a tenant organization

The example tenant is `acme`.

### 4a. Create the organization in IS

In the root organization's Console, go to **Organizations** → **New Organization**:

- **Organization Name:** `acme`
- **Organization Handle:** `acme` — set it explicitly. With `claim_mappings.org_handle` set
  it becomes the tenant's portal URL (`/api-portal/acme/...`), and it can't be changed
  after the organization is created.
- **Description:** optional.

Click **Create**.

### 4b. Add the organization's users

Switch into the organization: on the **Organizations** page, click the switch icon (⇄)
on its row. The header then shows **Super / acme**, and the organization's id is the
`/o/<id>/` segment of the Console URL — you need it in 4d.

On the organization's **Applications** page, **API Portal** is listed as a *Shared app*.
Then:

1. **User Management** → **Users** → **Add User**: create the tenant's administrator (for
   example `acmeadmin`), and any subscriber users.
2. **User Management** → **Roles** → **dp_admin** → **Users** → **Assign User**: assign the
   administrator. Assign subscriber users to **dp_subscriber** the same way.

### 4c. First sign-in

The tenant's administrator signs in to the portal from the configured organization's page
(`http://localhost:9543/api-portal/default`) and picks their organization at the IS login.
The portal creates the tenant's organization — handle and name from the IS organization —
and lands them in it (`/api-portal/acme/...`). Its public pages are browsable from then on.

### 4d. Key manager (optional)

A key manager lets the tenant's applications get tokens from the tenant's own IS
organization. See [Key Manager Integration](key-manager-integration.md).

1. In IS, switched into the organization: **Applications** → **New Application** →
   **M2M Application**, named for example `acme-key-manager`. Its **Protocol** tab shows
   the **Client ID** and **Client secret** used below.
2. In the portal, signed in as the tenant's administrator: **Settings** → **Key Managers**
   → **Add key manager**, with **Token endpoint**
   `https://host.docker.internal:9443/o/<organization id>/oauth2/token`.

The token endpoint is called by the portal container, so use the IS host the container
reaches (as for `token_url` in Section 3). If the IS organization is ever recreated, its id
changes — update the key manager's token endpoint to match.

### 4e. Plans, applications and subscriptions

All in the portal:

- **Subscription plans** — the organization starts with the default plans. Change them
  under **Settings** → **Subscription Plans** (administrator).
- **Applications** — a subscriber user goes to **Applications** → **Create application**,
  then in the application **Manage Keys** → **Associate existing key**, entering the M2M
  application's client ID from 4d as **Consumer Key**. **Generate Access Token** there
  issues a token with its client secret.
- **Subscriptions** — on an API's page, **Subscribe** and pick a plan.

APIs and MCP servers are published into the organization as usual. To load the sample
catalog for a demo, `scripts/seed-samples.sh` takes a token issued for the organization's
administrator (`ACCESS_TOKEN`); see
[Onboarding a tenant's catalog](multi-tenancy.md#onboarding-a-tenants-catalog).

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| Users sign in with no name, or get no access | The application's **User Attributes** (1b) or **Access Token Attributes** (1a) are missing `roles` / profile attributes. |
| A tenant user signs in but gets no admin or subscriber access | **Enable enhanced organization login** (1c) is off, or the user isn't assigned `dp_admin` / `dp_subscriber` in *their* organization (4b). |
| The IS administrator lands in an organization called `Super` | `auth.idp_org_id` isn't the root organization's id — see [The root organization](#the-root-organization). |
| `Org claim matches more than one organization's idp_ref_id` | Two portal organizations share an IS organization id — typically a leftover `Super` organization; see [The root organization](#the-root-organization). |
| The portal can't reach IS (`token_url`, JWKS) from Docker, or TLS errors | Section 3: container-side URLs use `host.docker.internal`, and IS's certificate must be trusted with `NODE_EXTRA_CA_CERTS`. |
