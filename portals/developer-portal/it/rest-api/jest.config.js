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

module.exports = {
    testEnvironment: 'node',
    testMatch: ['**/*.spec.js'],
    globalSetup: '<rootDir>/support/global-setup.js',
    globalTeardown: '<rootDir>/support/global-teardown.js',
    // Registers a per-suite afterAll that deletes resources tracked via
    // support/cleanup.js, so specs don't accumulate objects in the shared org.
    setupFilesAfterEnv: ['<rootDir>/support/autoCleanup.js'],
    testTimeout: 20000,
    // The devportal-under-test runs SQLite through a single shared connection
    // (see src/db/adapters/sqliteAdapter.js — correct for a single-writer SQLite
    // file) used by every DAO AND the session store. Because better-sqlite3 is
    // synchronous, that one connection processes every DB op —
    // logins' session writes, fixture CRUD from every spec file, the webhook
    // dispatcher/delivery-worker polling — strictly one at a time regardless of
    // Jest's worker count, so parallel workers add queuing/coordination risk
    // without any real throughput gain. Under load, queued ops can exceed the
    // test timeout and surface as spurious "Login failed" redirects
    // (authController.js's session.regenerate/logIn failure branches), not real
    // auth bugs — this was flaky even at maxWorkers: 2 depending on host load.
    // Fully serial removes the contention risk entirely.
    maxWorkers: 1,
    reporters: [
        'default',
        ['jest-junit', { outputDirectory: 'reports', outputName: 'rest-api-results.xml' }],
    ],
};
