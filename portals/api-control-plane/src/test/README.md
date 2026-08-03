# Testing guide — api-control-plane

Stack: **Vitest** (jsdom) + **@testing-library/react** + **@testing-library/user-event**
+ **MSW** (network mocking) + **@vitest/coverage-v8**. Config lives in `vite.config.ts`
(`test` block); global setup in `src/test/setup.ts`.

## Layout (`src/test/`)
- `setup.ts` — jest-dom matchers, browser-API mocks, MSW server lifecycle.
- `browserMocks.ts` — `matchMedia` / `ResizeObserver` / `IntersectionObserver` / `scrollTo`
  stubs that jsdom lacks but Oxygen/MUI need.
- `server.ts` — the single MSW server. Extend by adding handlers here.
- `handlers/` — request handlers grouped by transport (`graphql.ts`, `platform.ts`).
- `utils.tsx` — `renderWithProviders` (the linchpin) + re-exports of RTL and `userEvent`.
- `mockAuthState.ts` — `makeAuthState()` + `authStatePresets`.
- `mockScope.ts` — `makeConsoleScope()` seeded from `src/api/mocks/data.ts`.

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
