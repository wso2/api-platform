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
│   │   ├── login.ts (Login logic)
│   │   ├── logout.ts (Logout logic)
│   │   ├── stsExchange.ts (Token exchange logic)
│   │   └── useSignInSilent.ts (Silent sign-in logic)
│   ├── clients/ (API client implementations)
│   │   └── choreoApiClient.ts (Choreo API client)
│   ├── contexts/ (React context providers)
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

## Other Documentations
[Mock Backend Documentation](workspaces/apps/ai-workspace/mock-service/README.md)
[Production Documentation](workspaces/apps/ai-workspace/production/README.md)
