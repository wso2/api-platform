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
const { config } = require('../../config/configLoader');

/**
 * Returns all configured subscribers that should receive an event of the given
 * type and gateway_type.
 *
 * Matching rules:
 *   - subscriber.gatewayType === "*"  → matches any event gateway_type
 *   - subscriber.gatewayType === eventGatewayType  → matches
 *   - subscriber.events is absent / empty → receives all event types
 *   - subscriber.events is an array of glob-style patterns (only * wildcard at end)
 *
 * @param {string} eventType      — e.g. "apikey.generated"
 * @param {string|null} gatewayType — value from DP_EVENT.GATEWAY_TYPE
 * @returns {Array<{id,url,secret,publicKeyPath,publicKey,timeoutMs}>}
 */
function matchSubscribers(eventType, gatewayType) {
    const subscribers = config.webhooks && config.webhooks.subscribers;
    if (!Array.isArray(subscribers) || subscribers.length === 0) return [];

    return subscribers.filter(sub => {
        if (!sub.id || !sub.url || !sub.secret) return false;

        // gateway_type filter
        if (sub.gatewayType && sub.gatewayType !== '*') {
            if (sub.gatewayType !== gatewayType) return false;
        }

        // event type allowlist
        if (Array.isArray(sub.events) && sub.events.length > 0) {
            const matches = sub.events.some(pattern => {
                if (pattern.endsWith('.*')) {
                    const prefix = pattern.slice(0, -1); // "apikey."
                    return eventType.startsWith(prefix);
                }
                return pattern === eventType;
            });
            if (!matches) return false;
        }

        return true;
    });
}

/**
 * Returns subscriber by id, or null.
 */
function getSubscriber(id) {
    const subscribers = config.webhooks && config.webhooks.subscribers;
    if (!Array.isArray(subscribers)) return null;
    return subscribers.find(s => s.id === id) || null;
}

module.exports = { matchSubscribers, getSubscriber };
