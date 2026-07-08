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

// Generic poll-until helper for the async webhook pipeline: event write -> dispatcher
// poll (every 2s per src/services/webhooks/dispatcher.js) -> delivery.

const db = require('./db');

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

async function poll(fn, { timeoutMs = 15000, intervalMs = 500 } = {}) {
    const deadline = Date.now() + timeoutMs;
    let lastResult;
    do {
        lastResult = await fn();
        if (lastResult) return lastResult;
        await sleep(intervalMs);
    } while (Date.now() < deadline);
    throw new Error(`Timed out after ${timeoutMs}ms waiting for condition`);
}

// orgHandle is the REST-facing handle (e.g. org.id); it's resolved to the
// internal org_uuid once since dp_events isn't keyed by the public handle.
async function waitForEvent({ orgHandle, orgUuid, type, aggregateUuid, since }, options) {
    const resolvedOrgUuid = orgUuid || (orgHandle ? await db.findOrgUuidByHandle(orgHandle) : undefined);
    return poll(async () => {
        const rows = await db.findEvents({ orgUuid: resolvedOrgUuid, type, aggregateUuid, since });
        return rows[0];
    }, options);
}

async function waitForDelivery({ eventUuid, subscriberId }, options) {
    return poll(async () => {
        const rows = await db.findDeliveries({ eventUuid, subscriberId });
        const delivered = rows.find((r) => r.status === 'DELIVERED' || r.status === 'FAILED');
        return delivered;
    }, options);
}

module.exports = { sleep, poll, waitForEvent, waitForDelivery };
