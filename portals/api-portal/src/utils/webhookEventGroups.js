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

// Human labels for the known event-type prefixes. A prefix missing from here is NOT
// dropped — groupWebhookEventTypes derives a label for it — so adding a new event type
// to VALID_EVENT_TYPES surfaces it in the settings UI without touching this file.
const GROUP_LABELS = {
    subscription: 'Subscriptions',
    apikey: 'API keys',
    application: 'Applications',
};

// Display order for known prefixes. Unknown prefixes sort alphabetically after these.
const GROUP_ORDER = ['subscription', 'apikey', 'application'];

function labelFor(prefix) {
    if (GROUP_LABELS[prefix]) return GROUP_LABELS[prefix];
    // Fallback for a prefix nobody has labelled yet: "billing" -> "Billing".
    return prefix.charAt(0).toUpperCase() + prefix.slice(1);
}

/**
 * Turn an event action into display text: "token_regenerated" -> "Token regenerated".
 * snake_case reads badly as a UI label, and the group heading already supplies the
 * prefix, so only the action needs humanising. The full event type is still shown on
 * hover, so nothing about the exact API value is hidden.
 */
function humanizeAction(action) {
    const spaced = action.replace(/_/g, ' ');
    return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

/**
 * Group flat event types ("apikey.generated") by their prefix, for rendering the
 * webhook event picker as categorised checkboxes.
 *
 * Every input type appears in exactly one group's `events`, so the rendered form is
 * always a complete view of what can be subscribed to. `type` is the value the API
 * expects, `action` the raw short part, and `text` the humanised label rendered on the
 * chip (the group heading already carries the prefix).
 *
 * @param {Iterable<string>} eventTypes e.g. VALID_EVENT_TYPES
 * @returns {Array<{key: string, label: string, events: Array<{type: string, action: string, text: string}>}>}
 */
function groupWebhookEventTypes(eventTypes) {
    const byPrefix = new Map();

    for (const type of eventTypes) {
        const dot = String(type).indexOf('.');
        // A type with no dot becomes its own group rather than being skipped.
        const prefix = dot === -1 ? String(type) : String(type).slice(0, dot);
        const action = dot === -1 ? String(type) : String(type).slice(dot + 1);

        if (!byPrefix.has(prefix)) {
            byPrefix.set(prefix, []);
        }
        byPrefix.get(prefix).push({ type: String(type), action, text: humanizeAction(action) });
    }

    const known = GROUP_ORDER.filter((p) => byPrefix.has(p));
    const unknown = [...byPrefix.keys()].filter((p) => !GROUP_ORDER.includes(p)).sort();

    return [...known, ...unknown].map((prefix) => ({
        key: prefix,
        label: labelFor(prefix),
        events: byPrefix.get(prefix),
    }));
}

module.exports = { groupWebhookEventTypes, GROUP_LABELS, GROUP_ORDER };
