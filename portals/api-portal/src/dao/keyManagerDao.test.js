/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

'use strict';

if (!process.argv.includes('--config')) {
    process.argv.push('--config', 'it/test-config.toml');
}
process.env.APIP_AP_SECURITY_ENCRYPTION_KEY = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
process.env.APIP_AP_SECURITY_SESSION_SECRET = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';

const test = require('node:test');
const assert = require('node:assert/strict');
const kmDao = require('./keyManagerDao');

test('deleteKm deletes linked app_key_mappings before deleting key_managers row', async (t) => {
    const executedQueries = [];
    const mockDbExec = {
        execute: async (sql, params) => {
            executedQueries.push({ sql, params });
            if (sql.includes('DELETE FROM key_managers')) {
                return { rowCount: 1 };
            }
            return { rowCount: 0 };
        },
    };

    const deletedCount = await kmDao.delete('km-123', mockDbExec);
    assert.equal(deletedCount, 1);
    assert.equal(executedQueries.length, 2);
    assert.equal(executedQueries[0].sql, 'DELETE FROM app_key_mappings WHERE km_uuid = ?');
    assert.deepEqual(executedQueries[0].params, ['km-123']);
    assert.equal(executedQueries[1].sql, 'DELETE FROM key_managers WHERE uuid = ?');
    assert.deepEqual(executedQueries[1].params, ['km-123']);
});
