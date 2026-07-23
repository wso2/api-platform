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
const https = require('https');
const http = require('http');
const { URL } = require('url');
const { config } = require('../../config/configLoader');
const db = require('../../db/driver');
const { parseJsonColumn } = require('../../db/rows');
const eventDao = require('../../dao/eventDao');
const { getSubscriber } = require('./subscriberRegistry');
const { sign } = require('./signer');
const logger = require('../../config/logger');

let running = false;
let intervalHandle = null;

/**
 * POST a single delivery to the subscriber's target URL.
 *
 * Payload shape:
 * {
 *   event_id, event_type, occurred_at,
 *   org: { ref_id },          — CP_REF_ID, falls back to internal ORG_UUID
 *   encrypted_fields: [],     — names of fields in `data` that carry an encrypted envelope
 *   data: {
 *     ...event payload,
 *     [fieldName]: { wrappedKey, iv, tag, ciphertext }  — per encrypted field
 *   }
 * }
 *
 * event.CP_REF_ID must be set by the caller (runBatch) before calling this function.
 *
 * Returns { ok, status, error }.
 */
async function post(delivery, event) {
    const sub = await getSubscriber(delivery.subscriber_id);
    if (!sub) {
        return { ok: false, error: `Subscriber '${delivery.subscriber_id}' not found` };
    }

    const deliveryId = delivery.uuid;
    const timeoutMs = (sub && sub.timeoutMs) || 5000;

    const encryptedFields = delivery.encrypted_fields || {};
    const outgoing = {
        event_id: event.uuid,
        event_type: event.type,
        occurred_at: event.occurred_at,
        org: { ref_id: event.cp_ref_id || event.org_uuid },
        encrypted_fields: Object.keys(encryptedFields),
        data: { ...(event.payload || {}), ...encryptedFields }
    };

    const body = JSON.stringify(outgoing);

    const headers = {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(body),
        'X-Devportal-Event': event.type,
        'X-Devportal-Event-Id': event.uuid,
        'X-Devportal-Delivery-Id': deliveryId,
    };
    if (sub.secret) {
        const { header: sigHeader } = sign(sub.secret, body);
        headers['X-Devportal-Signature'] = sigHeader;
    }

    return new Promise((resolve) => {
        const parsedUrl = new URL(delivery.target_url);
        const transport = parsedUrl.protocol === 'https:' ? https : http;
        const options = {
            hostname: parsedUrl.hostname,
            port: parsedUrl.port || (parsedUrl.protocol === 'https:' ? 443 : 80),
            path: parsedUrl.pathname + parsedUrl.search,
            method: 'POST',
            headers,
            timeout: timeoutMs
        };

        const req = transport.request(options, (res) => {
            res.resume(); // drain body
            resolve({ ok: res.statusCode >= 200 && res.statusCode < 300, status: res.statusCode });
        });

        req.on('timeout', () => { req.destroy(); resolve({ ok: false, error: 'timeout' }); });
        req.on('error', (err) => resolve({ ok: false, error: err.message }));
        req.write(body);
        req.end();
    });
}

async function runBatch() {
    const delivery = config.webhooks && config.webhooks.delivery;
    const batchSize = (delivery && delivery.batchSize) || 50;
    const deliveries = await eventDao.claimDueDeliveries(batchSize);
    if (deliveries.length === 0) return;

    const eventIds = [...new Set(deliveries.map(d => d.event_uuid))];
    const eventPlaceholders = eventIds.map(() => '?').join(', ');
    const events = await db.query(`SELECT * FROM dp_events WHERE uuid IN (${eventPlaceholders})`, eventIds);
    // payload is JSONB on postgres (auto-parsed by `pg`) but TEXT on sqlite/mssql —
    // parse it back into an object here, matching eventDao.js's own parseEventRow.
    // Without this, `{ ...event.payload }` below silently spreads a JSON STRING
    // character-by-character instead of spreading its actual fields.
    for (const event of events) {
        event.payload = parseJsonColumn(event.payload);
    }
    const eventMap = Object.fromEntries(events.map(e => [e.uuid, e]));

    const orgIds = [...new Set(events.map(e => e.org_uuid))];
    let orgCpRefIdMap = {};
    if (orgIds.length > 0) {
        const orgPlaceholders = orgIds.map(() => '?').join(', ');
        const orgs = await db.query(
            `SELECT uuid, cp_ref_id FROM dp_organizations WHERE uuid IN (${orgPlaceholders})`,
            orgIds
        );
        orgCpRefIdMap = Object.fromEntries(orgs.map(o => [o.uuid, o.cp_ref_id]));
    }

    for (const delivery of deliveries) {
        const event = eventMap[delivery.event_uuid];
        if (!event) {
            logger.warn('Event not found for delivery', { deliveryId: delivery.uuid });
            continue;
        }
        event.cp_ref_id = orgCpRefIdMap[event.org_uuid] ?? null;

        let result;
        try {
            result = await post(delivery, event);
        } catch (postErr) {
            await eventDao.markFailed(delivery.uuid, { httpStatus: 0, error: postErr.message });
            logger.error('Post threw unexpectedly', {
                deliveryId: delivery.uuid, error: postErr.message
            });
            continue;
        }

        if (result.ok) {
            await eventDao.markDelivered(delivery.uuid, result.status);
            logger.info('Delivered', {
                deliveryId: delivery.uuid, subscriberId: delivery.subscriber_id,
                eventType: event.type, status: result.status
            });
        } else {
            await eventDao.markFailed(delivery.uuid, { httpStatus: result.status, error: result.error });
            logger.warn('[deliveryWorker] failed', {
                deliveryId: delivery.uuid, subscriberId: delivery.subscriber_id,
                eventType: event.type, status: result.status, error: result.error
            });
        }
    }
}

function start() {
    if (running) return;
    running = true;

    const wdelivery = config.webhooks && config.webhooks.delivery;
    const pollMs = (wdelivery && wdelivery.pollIntervalMs) || 2000;

    async function tick() {
        try {
            await runBatch();
        } catch (err) {
            logger.error('Batch error', { error: err.message || String(err) });
        }
    }

    intervalHandle = setInterval(tick, pollMs);
    logger.info('Delivery worker started', { pollIntervalMs: pollMs });
}

function stop() {
    running = false;
    if (intervalHandle) {
        clearInterval(intervalHandle);
        intervalHandle = null;
    }
}

module.exports = { start, stop };
