# Testing guide — api-control-plane

Stack: **Vitest** (jsdom) + **@testing-library/react** + **@testing-library/user-event**
+ **MSW** (network mocking) + **@vitest/coverage-v8**. Config lives in `vite.config.ts`
(`test` block); global setup in `src/test/setup.ts`.

## Layout (`src/test/`)
- `setup.ts` — jest-dom matchers, browser-API mocks, MSW server lifecycle.
- `browserMocks.ts` — `matchMedia` / `ResizeObserver` / `IntersectionObserver` / `scrollTo`
  stubs that jsdom lacks but Oxygen/MUI need.
- `server.ts` — the single MSW server, with **no default handlers** (see below).
- `msw/` — the toolkit for platform-api tests:
  - `apiBase.ts` — `API_BASE` / `apiUrl()`, derived from `runtimeConfig`.
  - `fixtures.ts` — spec-typed entity builders (`aRestApi`, `aProject`, `manyRestApis`, …).
  - `handlers.ts` — handler builders (`collection`, `resource`, `accepts`, `noContent`,
    `failure`, `malformedFailure`, `networkError`, `neverResponds`) + `recorder()`.
- `utils.tsx` — `renderWithProviders` (the linchpin) + re-exports of RTL and `userEvent`.
- `mockAuthState.ts` — `makeAuthState()` + `authStatePresets`.
- `mockScope.ts` — `makeConsoleScope()` seeded from `src/api/mocks/data.ts`.

## MSW: no defaults, typed builders

The server ships **no default handlers**. With `onUnhandledRequest: 'error'`, a test must
declare the endpoints it exercises, so what a test depends on is visible in the test, and
no shared fake backend can quietly decide what an assertion is really checking. The builders
make that a line each:

```ts
import { server } from '../../test/server';
import { collection, recorder, aRestApi, manyRestApis, type Recorder } from '../../test/msw';

let requests: Recorder;
beforeEach(() => { requests = recorder(); });

// A paginated collection that honours limit/offset/query like the real endpoint,
// so a caller that forgets to send them fails here.
server.use(collection('/rest-apis', manyRestApis(30), { record: requests }));

expect(requests.last()?.params.get('limit')).toBe('12');
expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
```

Rules that keep this useful:

- **Fixtures return generated spec types.** `aRestApi()` is typed `Schema<'RESTAPI'>`, so a
  fixture that doesn't match the contract fails to compile. This has already caught two real
  bugs (`upstream.production`, the field is `main`; and `CreateRESTAPIRequest` resolving to an uninhabitable type).
- **Never hard-code the base URL.** Use `apiUrl('/rest-apis')`; it follows `runtimeConfig`,
  so a version bump doesn't leave handlers matching a URL nothing requests.
- **Reset the transport** in `beforeEach` with `resetHttpClient()`, the axios instance is
  memoised on first use.
- **Assert absence with a settle, not `waitFor`.** `waitFor` passes immediately and proves
  nothing; to show no request fired, await a short timeout then assert `requests.count()` is 0.
- **Legacy GraphQL handlers are opt-in**, not global:
  `beforeEach(() => server.use(...legacyGraphqlHandlers))`. No test currently needs them; they
  are deleted with the legacy layer.

### Which layer to test how
| Testing | Needs | Example |
|---|---|---|
| `*.endpoints.ts` | MSW only — no React | `restApis.endpoints.test.ts` |
| `*.hooks.ts` | MSW + `QueryClientProvider` + `ApiScopeProvider` | — |
| A page | MSW + `renderWithProviders` + `ApiScopeProvider` | `ApiListPage.test.tsx` |
| Transport / cache policy | MSW, or nothing at all | `core/http.test.ts`, `core/queryClient.test.ts` |

## Conventions
- **Colocate** tests next to source as `*.test.ts` (logic) / `*.test.tsx` (rendering).
- **Always render via `renderWithProviders`** — never wrap providers by hand.
  ```tsx
  import { renderWithProviders, screen } from '../../test/utils';
  const { user } = renderWithProviders(<ApiListPage />, {
    route: '/organizations/api-platform-demo/projects/retail-apis/apis',
    scope: makeConsoleScope(),
    authState: authStatePresets.authenticated(),
  });
  ```
- **Never hit the real network.** Unhandled requests fail the test (`onUnhandledRequest:'error'`).
  Override per-test with `server.use(http.get(...))` / `graphql...`; handlers reset after each test.
- **Switching client mode** (GraphQL vs platform REST): the client reads `runtimeConfig` at
  module load, so set the env *before* importing the client:
  ```ts
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', 'http://platform.test');
  vi.resetModules();
  const { listApis } = await import('./apiClient');
  ```
  Restore with `vi.unstubAllEnvs()` in `afterEach`.
- **Auth is injected** via `AuthStateContext` — component tests do not mock the Asgardeo SDK.
  Only `platformClient` tests mock `@asgardeo/auth-react` (for the 401-refresh-retry path).
- **Prefer `userEvent`** over `fireEvent`; **query by role/label**, fall back to test-id only
  when necessary; assert async UI with `await screen.findBy...` / `waitFor`.

## Commands
- `npm run test` — run once. `npm run test:watch` — watch. `npm run test:coverage` — coverage report.
- Coverage thresholds start low and **ratchet up** as tiers land (see `vite.config.ts`); a drop
  below the current floor fails the build.
