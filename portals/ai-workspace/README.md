# AI Workspace FrontEnd

The frontend implementation will cover the following key areas:
* **Client Logic**
* **Session Management**
* **User Interface (UI)**
* **User Experience (UX)**
---

## Product Context

The **AI Workspace** is a standalone web application designed to address AI-specific use cases.

In its initial phase, it is targeted to work with the **Bijira Cloud solution**, integrating with **Bijira's Asgardeo SP** as the service provider for authentication and identity management.

This application is intentionally scoped **only for AI-related workflows** and does **not** include functionality related to general API management or broader platform features.

---

## Technology Stack

The frontend will be built using the following technologies:

* **React**
* **Vite**
* **TypeScript**
* **Oxygen UI (new version)**

## Guide

This repository uses Rush (monorepo) and the `rushx` helper to run per-project npm scripts. The AI Workspace project is declared in the monorepo in a subspace as below:
```json
{
  "packageName": "@wso2-enterprise/ai-workspace",
  "projectFolder": "workspaces/apps/ai-workspace",
  "subspaceName": "ai-workspace"
}
```

---

### 1) Update dependencies for the AI Workspace (subspace/project)

- From the repository root:
```bash
cd workspaces/apps/ai-workspace
rush update --only @wso2-enterprise/ai-workspace
```
---

### 2) Install a new dependecy

- From the repository root:
```bash
cd workspaces/apps/ai-workspace
rush add -p <dependency>@<version>
rush update --only @wso2-enterprise/ai-workspace
```

Note: workspaces/apps/ai-workspace/package.json & common/config/subspaces/ai-workspace/pnpm-lock.yaml should be changed

### 3) Start the AI Workspace Dev Server with `rushx`

- From the repository root:
```bash
cd workspaces/apps/ai-workspace
rushx start
```

### PNPM Guide (To Fix Local Issues and Try)

```bash
rm -rf common/temp/ai-workspace/node_modules/.vite workspaces/apps/ai-workspace/node_modules/.vite && \
cd workspaces/apps/ai-workspace && pnpm install && pnpm run dev
```
---

## Project file tree 📁

Overview of the AI Workspace source layout:
- All file names use camelCase by convention
- All component names use PascalCase by convention

```
workspaces/apps/ai-workspace/
├── README.md (Web application overview)
├── index.html (Web application entry HTML)
├── package.json (Web application dependencies)
├── src/ (Source code)
│   ├── App.tsx (Root application component)
│   ├── apis/ (Context-based API definitions)
│   │   └── projectApis.ts (Choreo project–related APIs)
│   │   └── etc
│   ├── auth/ (Authentication-related logic)
│   │   ├── logout.ts (Logout + session clear logic)
│   │   ├── mockUsers.config.ts (Mock users for local dev)
│   │   └── permissions.ts (Roles, scopes, checkPermission)
│   ├── clients/ (API client implementations)
│   │   └── choreoApiClient.ts (Choreo API client)
│   ├── contexts/ (React context providers)
│   │   ├── AppAuthContext.tsx (Auth context + useAppAuth hook)
│   │   ├── OIDCAppAuthProvider.tsx (OIDC auth provider)
│   │   ├── MockAuthProvider.tsx (Mock auth provider)
│   │   └── AppShellContext.tsx (Global AppShell context)
│   ├── pages/ (Application pages)
│   │   ├── appShell/
│   │   │   ├── appShellMain.tsx (AppShell entry point)
│   │   │   └── appShellPages/
│   │   │       ├── page1/
│   │   │       │   ├── main.tsx
│   │   │       │   ├── component1.tsx
│   │   │       │   └── component2.tsx
│   │   │       └── page2/
│   │   │           ├── main.tsx
│   │   │           ├── component1.tsx
│   │   │           └── component2.tsx
│   │   └── login/
│   │       ├── login.tsx (Login page)
│   │       └── signinCallback.tsx (Sign-in redirect page)
│   ├── utils/ (Shared utilities)
│   │   ├── cookies.ts (Cookie utilities)
│   │   └── types.ts (Shared type definitions)
│   ├── components/ (Reusable UI components)
│   │   ├── sharedComponent1.tsx 
│   │   └── sharedComponent2.tsx 
│   ├── main.tsx (Vite application entry point)
│   ├── config.env.ts (Centralized environment configuration)
│   └── styles.css (Global styles)
├── tsconfig.json (TypeScript configuration)
└── vite.config.ts (Vite configuration)
```

---

## Authentication

The app supports two auth modes controlled by the `VITE_DISABLE_AUTH` environment variable.

### OIDC Mode (default — `VITE_DISABLE_AUTH=false`)

Uses [`react-oidc-context`](https://github.com/authelia/react-oidc-context) with auto-discovery via `{VITE_OIDC_AUTHORITY}/.well-known/openid-configuration`. The app is configured to target **Bijira's Asgardeo SP** in production, but works with any OIDC-compliant provider.

| Env var | Purpose | Default |
|---|---|---|
| `VITE_OIDC_AUTHORITY` | IdP issuer URL | `https://localhost:8090` |
| `VITE_OIDC_CLIENT_ID` | OIDC client ID | _(empty)_ |
| `VITE_OIDC_REDIRECT_URI` | Post-login redirect | `https://{DOMAIN}/signin` |
| `VITE_OIDC_POST_LOGOUT_REDIRECT_URI` | Post-logout redirect | `https://{DOMAIN}/login` |
| `VITE_OIDC_END_SESSION_ENDPOINT` | Override end-session URL | _(auto-discovered)_ |
| `VITE_OIDC_SCOPE` | Space-separated OAuth2 scopes to request | _(all platform scopes)_ |
| `VITE_OIDC_IDP_HINT_PARAM` | Query param for federated IdP hint | `fidp` |
| `VITE_OIDC_ORG_CLAIM` | JWT claim carrying the org UUID | `organization` |
| `VITE_OIDC_USERNAME_CLAIM` | JWT claim for display name | `given_name` |
| `VITE_OIDC_EMAIL_CLAIM` | JWT claim for email | `email` |

Social/federated IdP hint values (passed as `{VITE_OIDC_IDP_HINT_PARAM}=<value>`):

| Env var | Default value |
|---|---|
| `VITE_SOCIAL_IDP_GOOGLE` | `google` |
| `VITE_SOCIAL_IDP_GITHUB` | `github` |
| `VITE_SOCIAL_IDP_MICROSOFT` | `microsoft` |
| `VITE_SOCIAL_IDP_ENTERPRISE` | `EnterpriseIDP` |

**Provider:** `OIDCAppAuthProvider` wraps `react-oidc-context`'s `useAuth` hook and exposes the unified `AppAuthContext`.

### Mock Auth Mode (`VITE_DISABLE_AUTH=true`)

Replaces OIDC with a local username/password login form. No IdP is needed — credentials are checked against `src/auth/mockUsers.config.ts` in memory.

**Default users:**

| Username | Password | Role |
|---|---|---|
| `admin` | `admin` | `admin` |
| `developer` | `developer` | `developer` |
| `viewer` | `viewer` | `viewer` |

Users are stored in `sessionStorage` and a mock JWT (alg: none) is minted locally. To add or change users, edit `MOCK_USERS` in `src/auth/mockUsers.config.ts`.

Set `VITE_DEV_ORG_ID` to the UUID of your seeded local org (must match the platform-api database).

**Provider:** `MockAuthProvider` exposes the same `AppAuthContext` shape as OIDC mode — components are auth-mode agnostic.

### Roles and Permissions

Five platform roles are defined in `src/auth/permissions.ts`:

| Role | Description |
|---|---|
| `admin` | Full manage access to all resources |
| `developer` | Manage projects, apps, proxies; no gateway management |
| `publisher` | Publish APIs, manage dev portals; read-only on most resources |
| `operator` | Manage gateways and deployments; read-only on others |
| `viewer` | Read-only across all resources |

Permission resolution is controlled by `VITE_PERMISSION_MODE`:
- **`scope`** (default) — the raw OAuth2 scopes from the JWT access token are used directly.
- **`role`** — the `platform_role` (or `role`) JWT claim is used to expand to a predefined scope set from `ROLE_SCOPES`.

Use the `hasPermission(scope)` function from `useAppAuth()` to guard UI elements. It handles `:manage` parent-scope inheritance automatically (e.g. `api-platform:gateway:manage` grants all `api-platform:gateway:*` scopes).

### Auth Context API

```ts
const { isAuthenticated, isLoading, user, accessToken, hasPermission, login, logout } = useAppAuth();
```

| Field | Type | Description |
|---|---|---|
| `isAuthenticated` | `boolean` | Whether a user session is active |
| `isLoading` | `boolean` | Auth state is still being resolved |
| `user` | `AppUser \| null` | `{ name, email, role, scopes }` |
| `accessToken` | `string \| null` | Bearer token for API calls |
| `hasPermission(scope)` | `(string) => boolean` | Scope-level permission check |
| `login(credentials?)` | `async () => void` | Redirects to IdP (OIDC) or logs in locally (mock) |
| `logout()` | `async () => void` | Clears session and redirects to `/login` |

### Key Source Files

```
src/
├── auth/
│   ├── permissions.ts         — PlatformRole, SCOPES, ROLE_SCOPES, checkPermission
│   ├── mockUsers.config.ts    — Mock user list for local dev
│   └── logout.ts              — handleLogout, clearAuthData (clears known sessionStorage keys)
└── contexts/
    ├── AppAuthContext.tsx      — Shared context definition and useAppAuth() hook
    ├── OIDCAppAuthProvider.tsx — OIDC auth provider (react-oidc-context)
    └── MockAuthProvider.tsx   — Mock auth provider (local credentials)
```

---

## Other Documentations
[Mock Backend Documentation](workspaces/apps/ai-workspace/mock-service/README.md)
[Production Documentation](workspaces/apps/ai-workspace/production/README.md)
