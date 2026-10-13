---
name: ai-workspace-feature-flags
description: Add, gate, enable or retire an AI Workspace feature flag ([ai_workspace.feature_flags] in config.toml, isFeatureEnabled in portals/ai-workspace/src/config.env.ts). Use when a new AI Workspace feature must ship hidden while it rolls out, when a hidden feature still shows a nav entry, route, overview card or makes an API call, when a flag set in config.toml does not reach the SPA, or when a shipped feature's flag should be removed.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# AI Workspace feature flags

## The rule

A feature flag is **off unless it is explicitly turned on.** Only `true` shows a feature. `false` hides it, and so does a key that is not there at all. There is no default to set: a flag nobody names is already off.

| In the deployment's config | Feature |
|---|---|
| `<feature>_enabled = true` | shown |
| `<feature>_enabled = false` | hidden |
| key absent | hidden |

A flag is a presentation control. Hiding a feature removes it from the UI; it does not stop anyone calling the Platform API. Scope checks stay where they are.

## How a flag reaches the SPA

```
config.toml          [ai_workspace.feature_flags] guardrail_templates_enabled = true
  │  the BFF emits a key only if it is on its allowlist (browserSafeKeys)
  ▼
runtime-config.js    window.__RUNTIME_CONFIG__ = {"APIP_AIW_FEATURE_FLAGS_GUARDRAIL_TEMPLATES_ENABLED":"true"}
  │  getEnvOrDefault looks the name up, falling back to its default, false
  ▼
config.env.ts        isFeatureEnabled('guardrail-templates')  →  true
```

The browser-side name is derived from the key's path, never chosen: `APIP_AIW_` plus the path under `[ai_workspace]`, uppercased, with dots as underscores. `feature_flags.guardrail_templates_enabled` becomes `APIP_AIW_FEATURE_FLAGS_GUARDRAIL_TEMPLATES_ENABLED`. The same name works as an environment variable.

The BFF emits only keys that are present in the config, so an absent key never reaches the browser and the SPA's `false` applies.

## Adding a flag

The examples use a feature called `guardrail-templates`. Name the key `<feature>_enabled` so its direction reads at the point of use.

### 1. Allowlist it in the BFF

`portals/ai-workspace/bff/internal/config/runtime_config.go`, in `browserSafeKeys`:

```go
"feature_flags.agent_proxy_enabled",
"feature_flags.guardrail_templates_enabled",
```

Without this the flag never leaves the BFF, and setting it to `true` changes nothing.

### 2. Declare it in the SPA

`portals/ai-workspace/src/config.env.ts`, in `FEATURE_FLAGS`:

```ts
const FEATURE_FLAGS = {
  'agent-proxy': getEnvOrDefault('APIP_AIW_FEATURE_FLAGS_AGENT_PROXY_ENABLED', false),
  'guardrail-templates': getEnvOrDefault(
    'APIP_AIW_FEATURE_FLAGS_GUARDRAIL_TEMPLATES_ENABLED',
    false
  ),
};
```

The default is always `false`. `FeatureId` and `isFeatureEnabled` pick up the new entry by themselves.

### 3. Gate every surface

A hidden feature leaves no trace: no route, no nav entry, no overview entry, and no API call. Gate each place it appears.

| Surface | File | Shape |
|---|---|---|
| Routes | `src/App.tsx` | `{isFeatureEnabled('guardrail-templates') && (<Route ...>)}` in every route tree the feature appears in |
| Nav entry | `src/pages/appShell/AppSidebar.tsx` | `isFeatureEnabled('guardrail-templates') && hasPermission(...) && (...)` |
| Overview entry | `src/pages/appShell/appShellPages/overview/Overview.tsx` | filter its kind out of `kinds` |
| Provider | wherever it is mounted | mount it only while the flag is on |

On the nav entry, keep the permission check beside the flag. The flag decides whether the feature exists; the scope decides whether this user may use it.

**Gate the provider, not only what renders its data.** A provider fetches when it mounts, so filtering a card out of the overview still leaves its provider mounted and calling the API. Mount it conditionally:

```tsx
{isFeatureEnabled('guardrail-templates') ? (
  <GuardrailTemplatesProvider>
    <OverviewContent />
  </GuardrailTemplatesProvider>
) : (
  <OverviewContent />
)}
```

A hook used outside its provider returns the context's default value, so that default must be a safe empty state (an empty list, `isLoading: false`) rather than a throw.

### 4. Document it, but do not ship it on

Add the key to `portals/ai-workspace/configs/config-feature-flags.toml`. Nothing loads that file; it is where every flag is documented.

Do **not** add `[ai_workspace.feature_flags]` to `config.toml` or `config-template.toml`. A feature starts hidden everywhere, and leaving the table out of the shipped config is what keeps it hidden.

### 5. Test it

**BFF**, in `bff/internal/config/runtime_config_test.go`, mirroring the agent-proxy cases:

- a key that is set reaches the SPA unchanged, for both `true` and `false`
- a key that is absent is not emitted

**UI suite.** A block whose UI scenarios drive the feature must turn it on, or the screens they look for do not exist. A component takes one overlay per block, through `overlay:` in `tests/framework/suites/ui/ui-suite.yaml`. If the block already has one, add the key to that file's `[ai_workspace.feature_flags]` table. If not, create an overlay under `tests/framework/core/catalog/overlays/` and point the block's `ai-workspace` component at it:

```toml
[ai_workspace.feature_flags]
guardrail_templates_enabled = true
```

**Unit tests** of a component that checks the flag mock it rather than depending on config, using the path to `config.env` relative to the test file:

```ts
vi.mock('../../config.env', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  isFeatureEnabled: vi.fn(),
}));
```

Cover both states: the surface present with the flag on, and absent, with no API call, with it off.

## Turning a flag on

A deployment adds the table to its own config.toml:

```toml
[ai_workspace.feature_flags]
guardrail_templates_enabled = true
```

or sets `APIP_AIW_FEATURE_FLAGS_GUARDRAIL_TEMPLATES_ENABLED=true`. The SPA needs no rebuild. The BFF reads its config at startup, so restart it to pick up the change.

## Retiring a flag

A flag lives only as long as its rollout. Once a feature ships everywhere, remove the flag rather than defaulting it to `true`:

1. Remove every `isFeatureEnabled('<feature>')` call, leaving the gated code unconditional.
2. Remove the entry from `FEATURE_FLAGS`.
3. Remove the key from `browserSafeKeys`, its BFF tests, `config-feature-flags.toml`, and any UI suite overlay.

Removing the `FEATURE_FLAGS` entry removes its `FeatureId`, so TypeScript reports any call site that was missed.

## Common mistakes

| Mistake | Effect |
|---|---|
| Key missing from `browserSafeKeys` | Setting it to `true` does nothing |
| Overview entry gated but provider not | The hidden feature still calls the API on page load |
| Table added to the shipped `config.toml` | The feature is on for every deployment |
| Default `true` in `FEATURE_FLAGS` | The feature is on wherever the key is absent |
| Context default that throws | Conditionally mounting its provider crashes the page |
| Flag treated as access control | The endpoint stays reachable; scopes are the real gate |
| UI block given no overlay | Its scenarios cannot find the nav entry or route |
| Environment variable name written by hand | It must match the derived name exactly |
| Shipped feature's flag defaulted to `true` | A dead flag stays in the code; retire it instead |
