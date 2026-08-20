import { setupServer } from 'msw/node';

import { handlers as graphqlMockHandlers } from '../api/mocks/handlers';

/**
 * Single MSW server shared by all tests (wired into `setup.ts`). Reuses the
 * runtime mock-mode GraphQL handlers and adds REST handlers for platform/BML
 * mode. Per-test overrides go through `server.use(...)`; unhandled requests
 * fail the test (configured in `setup.ts`).
 */
export const server = setupServer(...graphqlMockHandlers);
