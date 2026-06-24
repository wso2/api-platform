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
const eventDao = require('../../dao/eventDao');
const DPEvent = require('../../models/event');
const { getSubscriber } = require('./subscriberRegistry');
const { sign } = require('./signer');
const { nextAttemptAt, getMaxAttempts } = require('./backoff');
const logger = require('../../config/logger');

let running = false;
let intervalHandle = null;

/** Determine if an HTTP status warrants immediate dead-lettering (not retryable). */
function isNotRetryable(status) {
    // 4xx except 408 (Request Timeout) and 429 (Too Many Requests)
    return status >= 400 && status < 500 && status !== 408 && status !== 429;
}

/**
 * POST a single delivery. Returns { ok, status, error }.
 */
async function post(delivery, event) {
    const sub = await getSubscriber(delivery.SUBSCRIBER_ID);
    if (!sub) {
        return { ok: false, error: `Subscriber '${delivery.SUBSCRIBER_ID}' not found` };
    }

    const deliveryId = delivery.DELIVERY_ID;
    const timeoutMs = (sub && sub.timeoutMs) || 5000;

    // Build the outgoing payload: base event payload + per-subscriber encrypted fields.
    const outgoing = {
        event_id: event.EVENT_ID,
        event_type: event.EVENT_TYPE,
        occurred_at: event.OCCURRED_AT,
        org_id: event.ORG_ID,
        gateway_type: event.GATEWAY_TYPE,
        data: { ...(event.PAYLOAD || {}) }
    };
    if (delivery.ENCRYPTED_FIELDS) {
        outgoing.data.encrypted_key = delivery.ENCRYPTED_FIELDS;
    }

    const body = JSON.stringify(outgoing);
    const { header: sigHeader } = sign(sub.secret, body);

    const headers = {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(body),
        'X-Devportal-Event': event.EVENT_TYPE,
        'X-Devportal-Event-Id': event.EVENT_ID,
        'X-Devportal-Delivery-Id': deliveryId,
        'X-Devportal-Signature': sigHeader
    };

    return new Promise((resolve) => {
        const parsedUrl = new URL(delivery.TARGET_URL);
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

    const eventIds = [...new Set(deliveries.map(d => d.EVENT_ID))];
    const events = await DPEvent.findAll({ where: { EVENT_ID: eventIds } });
    const eventMap = Object.fromEntries(events.map(e => [e.EVENT_ID, e]));

    for (const delivery of deliveries) {
        const event = eventMap[delivery.EVENT_ID];
        if (!event) {
            logger.warn('[deliveryWorker] event not found for delivery', { deliveryId: delivery.DELIVERY_ID });
            continue;
        }

        let result;
        try {
            result = await post(delivery, event);
        } catch (postErr) {
            const newAttemptCount = delivery.ATTEMPT_COUNT + 1;
            const deadLetter = newAttemptCount >= getMaxAttempts();
            const nextAt = deadLetter ? new Date() : nextAttemptAt(newAttemptCount - 1);
            await eventDao.markFailed(delivery.DELIVERY_ID, {
                httpStatus: 0,
                error: postErr.message,
                attemptCount: newAttemptCount,
                nextAttemptAt: nextAt,
                deadLetter
            });
            logger.error('[deliveryWorker] post threw unexpectedly', {
                deliveryId: delivery.DELIVERY_ID, error: postErr.message
            });
            continue;
        }

        const newAttemptCount = delivery.ATTEMPT_COUNT + 1;

        if (result.ok) {
            await eventDao.markDelivered(delivery.DELIVERY_ID, result.status);
            logger.info('[deliveryWorker] delivered', {
                deliveryId: delivery.DELIVERY_ID, subscriberId: delivery.SUBSCRIBER_ID,
                eventType: event.EVENT_TYPE, status: result.status
            });
        } else {
            const deadLetter = isNotRetryable(result.status) || newAttemptCount >= getMaxAttempts();
            const nextAt = deadLetter ? new Date() : nextAttemptAt(newAttemptCount - 1);

            await eventDao.markFailed(delivery.DELIVERY_ID, {
                httpStatus: result.status,
                error: result.error,
                attemptCount: newAttemptCount,
                nextAttemptAt: nextAt,
                deadLetter
            });

            if (deadLetter) {
                logger.error('[deliveryWorker] dead-lettered', {
                    deliveryId: delivery.DELIVERY_ID, subscriberId: delivery.SUBSCRIBER_ID,
                    eventType: event.EVENT_TYPE, attempts: newAttemptCount,
                    status: result.status, error: result.error
                });
            } else {
                logger.warn('[deliveryWorker] will retry', {
                    deliveryId: delivery.DELIVERY_ID, subscriberId: delivery.SUBSCRIBER_ID,
                    eventType: event.EVENT_TYPE, attempts: newAttemptCount,
                    nextAttemptAt: nextAt, error: result.error || result.status
                });
            }
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
            logger.error('[deliveryWorker] batch error', { error: err.message || String(err) });
        }
    }

    intervalHandle = setInterval(tick, pollMs);
    logger.info('[deliveryWorker] started', { pollIntervalMs: pollMs });
}

function stop() {
    running = false;
    if (intervalHandle) {
        clearInterval(intervalHandle);
        intervalHandle = null;
    }
}

module.exports = { start, stop };
