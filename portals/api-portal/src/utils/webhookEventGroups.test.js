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

const test = require('node:test');
const assert = require('node:assert');

const { groupWebhookEventTypes } = require('./webhookEventGroups');

// The real set, kept literal here rather than imported: eventPublisher.js pulls in the
// DAO/db chain, and this also fails loudly if the real list changes shape.
const ALL = [
    'subscription.created',
    'subscription.updated',
    'subscription.deleted',
    'subscription.plan_changed',
    'subscription.token_regenerated',
    'apikey.generated',
    'apikey.regenerated',
    'apikey.revoked',
    'apikey.application_updated',
    'application.created',
    'application.updated',
    'application.deleted',
];

test('every event type lands in exactly one group', () => {
    // The property that matters: the rendered picker is a complete view of what can be
    // subscribed to, so no event type can silently go missing from the settings form.
    const groups = groupWebhookEventTypes(ALL);
    const flattened = groups.flatMap((g) => g.events.map((e) => e.type));

    assert.deepStrictEqual([...flattened].sort(), [...ALL].sort());
    assert.strictEqual(new Set(flattened).size, ALL.length, 'an event type appeared in more than one group');
});

test('groups the known prefixes with human labels, in a stable order', () => {
    const groups = groupWebhookEventTypes(ALL);
    assert.deepStrictEqual(
        groups.map((g) => [g.key, g.label]),
        [
            ['subscription', 'Subscriptions'],
            ['apikey', 'API keys'],
            ['application', 'Applications'],
        ]
    );
});

test('action is the part after the prefix, and type stays the full API value', () => {
    const groups = groupWebhookEventTypes(ALL);
    const apikey = groups.find((g) => g.key === 'apikey');

    assert.deepStrictEqual(
        apikey.events.map((e) => e.action),
        ['generated', 'regenerated', 'revoked', 'application_updated']
    );
    // The checkbox value must remain the full event type the API accepts.
    assert.ok(apikey.events.every((e) => e.type === `apikey.${e.action}`));
});

test('text humanises the action for display without touching type', () => {
    const groups = groupWebhookEventTypes(ALL);
    const byType = new Map(groups.flatMap((g) => g.events).map((e) => [e.type, e]));

    assert.strictEqual(byType.get('apikey.application_updated').text, 'Application updated');
    assert.strictEqual(byType.get('subscription.token_regenerated').text, 'Token regenerated');
    assert.strictEqual(byType.get('subscription.plan_changed').text, 'Plan changed');
    assert.strictEqual(byType.get('application.created').text, 'Created');

    // Display text must never leak into the value sent to the API.
    assert.strictEqual(byType.get('apikey.application_updated').type, 'apikey.application_updated');
    for (const e of byType.values()) {
        assert.ok(!e.text.includes('_'), `${e.type} label still has an underscore: ${e.text}`);
    }
});

test('an unlabelled prefix is surfaced with a derived label, never dropped', () => {
    // Guards the real regression risk: someone adds "billing.*" to VALID_EVENT_TYPES and
    // forgets GROUP_LABELS. It must still appear in the form.
    const groups = groupWebhookEventTypes([...ALL, 'billing.invoiced']);
    const billing = groups.find((g) => g.key === 'billing');

    assert.ok(billing, 'unlabelled prefix was dropped');
    assert.strictEqual(billing.label, 'Billing');
    assert.deepStrictEqual(billing.events, [
        { type: 'billing.invoiced', action: 'invoiced', text: 'Invoiced' },
    ]);
    // Unknown prefixes sort after the known ones.
    assert.strictEqual(groups[groups.length - 1].key, 'billing');
});

test('a type with no prefix becomes its own group rather than being skipped', () => {
    const groups = groupWebhookEventTypes(['ping']);
    assert.deepStrictEqual(groups, [
        { key: 'ping', label: 'Ping', events: [{ type: 'ping', action: 'ping', text: 'Ping' }] },
    ]);
});

test('empty input yields no groups', () => {
    assert.deepStrictEqual(groupWebhookEventTypes([]), []);
});
