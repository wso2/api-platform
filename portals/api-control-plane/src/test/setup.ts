import '@testing-library/jest-dom/vitest';
import { afterAll, afterEach, beforeAll } from 'vitest';

import './browserMocks';
import { server } from './server';

// Start one MSW server for the whole test run. Unhandled requests fail the
// test so we never silently hit the network; per-test overrides via server.use.
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
