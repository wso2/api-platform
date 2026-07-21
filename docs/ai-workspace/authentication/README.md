# Authentication

AI Workspace supports two authentication modes, configured via the `mode` key in `[ai_workspace.auth]` in `config.toml`.

## Modes

| Mode | Value | Best for |
|------|-------|----------|
| File-based | `basic` | Local development, quick demos — no external IDP required |
| OIDC | `oidc` | Production — delegates identity to an external OIDC-compliant IDP |

The mode is mutually exclusive: a running instance uses either file-based or OIDC auth, not both.

## Choosing a Mode

Use **`basic`** when:
- You are following the [Quick Start](../../QUICKSTART.md) and just want to try the product.
- You are running a demo or local development environment.
- You do not have an IDP available.

Use **`oidc`** when:
- You are deploying to a shared or production environment.
- You need multi-tenancy (multiple organizations).
- You need SSO with an existing identity system.

## Guides

- [File-Based Auth](file-based-auth.md) — configure local users and passwords
- [OIDC Auth](oidc-auth.md) — connect any OIDC-compliant IDP
- [Asgardeo Setup](asgardeo-setup.md) — end-to-end Asgardeo configuration walkthrough
