# Data access (`src/api`)

All backend access flows through **TanStack Query hooks**, which resolve the
transport from a **React context** (DI) and default their scope from the active
console scope. Components never call API clients directly.

## Layers

```
component → hook (src/api/hooks/useMvpQueries.ts, policyHub/usePolicyHub.ts)
              ├─ resolves client fns via useApiClient()   (ApiClientProvider)
              └─ resolves org/project via ConsoleScopeContext.activeScope
                    → client fn (apis/apiClient.ts, gateways/gatewayClient.ts, …)
                        → platformClient (REST/BML) | postGraphql (legacy) | mock
```

- **`ApiClientProvider`** (`src/api/ApiClientProvider.tsx`) — the DI seam. Exposes
  the client function surface (`ApiClient`) via context; `useApiClient()` returns
  it. Defaults to `realApiClient`; tests inject stubs (see below).
- **Hooks** (`useMvpQueries.ts`, `usePolicyHub.ts`) — the only thing components
  import. They own query keys, `enabled` gating, and cache invalidation.
- **Clients** (`*/​*Client.ts`, `mvpApi.ts`) — transport. Imported only by the
  provider/hooks, never by UI.

## Rules

- **Use a hook.** Add a query/mutation hook for new resources; don't fetch in
  components. `import { ... } from '@api/...Client'` outside `src/api/**` is
  blocked by ESLint (`no-restricted-imports`; type-only imports are allowed).
- **Context-aware scope.** Call hooks with no args — `useApis()`,
  `useApiDetail()`, `useCreateApi()` — and they read the **token-ready**
  `activeScope` (org/project/api) from `ConsoleScopeContext`. Pass explicit args
  only when you intentionally diverge from the active route (the
  `ConsoleScopeProvider` does this while building the scope). `orgHandle` in
  `activeScope` is only set after the org-token exchange, so context-aware
  queries never fire before their bearer token is ready.
- **No hidden request state.** Org/project reach the GraphQL/axios transport as
  explicit per-call args (`postGraphql(query, vars, { orgHandle, projectHandler })`),
  not a localStorage/interceptor side-channel. The REST/platform path sends
  `X-Org-Id` explicitly via `platformClient`.

## Testing

`renderWithProviders(ui, { scope, apiClient })` (`src/test/utils.tsx`):

- `scope` — inject a `ConsoleScope` (use `makeConsoleScope()`); context-aware
  hooks read its `activeScope`.
- `apiClient` — a `Partial<ApiClient>` merged over the real client, so a test can
  stub specific calls **without `vi.mock`**. Example:
  `renderWithProviders(<Page/>, { scope, apiClient: { listApis: vi.fn()... } })`.

See `src/api/hooks/useContextAwareHooks.test.tsx` for a worked example.
