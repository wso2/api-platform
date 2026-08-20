// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// The app calls app.disable('x-powered-by'), so no response should carry the
// X-Powered-By header — public or API route, success or error.

const client = require('../support/client');

describe('response headers — vendor fingerprint abstraction', () => {
    it('does not send X-Powered-By on a public route', async () => {
        const res = await client.raw().get('/health');
        expect(res.headers['x-powered-by']).toBeUndefined();
    });

    it('does not send X-Powered-By on an API route (even unauthenticated)', async () => {
        // No session/API key — returns an auth error, but the header must be absent regardless.
        const res = await client.raw().get(`${client.API_PREFIX}/organizations/${client.ORG_HANDLE}`);
        expect(res.headers['x-powered-by']).toBeUndefined();
    });
});
