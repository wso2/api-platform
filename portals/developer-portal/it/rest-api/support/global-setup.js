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

// Runs once before the whole Jest run: waits for the devportal container's
// /health endpoint so specs don't race the server/dispatcher startup.

const http = require('http');
const https = require('https');

const BASE_URL = process.env.DEVPORTAL_BASE_URL || 'http://localhost:3000';
const TIMEOUT_MS = 60000;

module.exports = async function globalSetup() {
    const client = BASE_URL.startsWith('https') ? https : http;
    const deadline = Date.now() + TIMEOUT_MS;

    await new Promise((resolve, reject) => {
        const attempt = () => {
            const req = client
                .get(`${BASE_URL}/health`, (res) => {
                    if (res.statusCode === 200) return resolve();
                    res.resume();
                    retry();
                })
                .on('error', retry);
            // A connection that neither responds nor errors (server accepted the
            // socket but hangs) would otherwise never reach retry()/the deadline
            // check, hanging setup forever. Destroy it so 'error' fires and retry()
            // enforces TIMEOUT_MS.
            req.setTimeout(5000, () => req.destroy(new Error('health check request timed out')));
        };
        const retry = () => {
            if (Date.now() > deadline) {
                return reject(new Error(`devportal did not become healthy within ${TIMEOUT_MS}ms`));
            }
            setTimeout(attempt, 1000);
        };
        attempt();
    });
};
