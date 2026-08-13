/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { setupServer } from 'msw/node';

import { handlers as legacyGraphqlHandlers } from '../api/mocks/handlers';

/**
 * The single MSW server shared by every test, started once in `setup.ts`.
 *
 * It intentionally ships **no default handlers**. Combined with
 * `onUnhandledRequest: 'error'`, that means a test must declare the endpoints it
 * exercises — so what a test depends on is visible in the test, and no shared
 * fake backend can quietly decide what an assertion is really checking.
 *
 * Declaring handlers is a line each thanks to the builders in `./msw`:
 *
 *     import { collection, aRestApi, recorder } from '../../test/msw';
 *
 *     const requests = recorder();
 *     server.use(collection('/rest-apis', [aRestApi()], { record: requests }));
 *
 * Handlers registered with `server.use(...)` are reset after every test.
 */
export const server = setupServer();

/**
 * Handlers for the legacy GraphQL project-api, opt-in rather than global.
 *
 * The old data layer is being replaced resource by resource, and no test
 * currently relies on these — verified by removing them and running the suite.
 * They stay available for anything that still needs the GraphQL transport, and
 * are deleted along with the legacy layer.
 *
 *     beforeEach(() => server.use(...legacyGraphqlHandlers));
 */
export { legacyGraphqlHandlers };
