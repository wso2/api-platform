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

/*
 * The application picker on the OAuth2 keys page.
 *
 * An application holds one key per key type, so associating a key with an
 * application that already has one of that type is refused by the service with a
 * 409. This picker is what keeps the developer from meeting that refusal: a
 * conflicting application is listed but disabled, and when nothing in the list
 * can take the key the modal says so instead of opening.
 *
 * Disabled rather than hidden, deliberately — the application still exists, and
 * dropping it from the list looks like the list is wrong. The one application
 * never disabled is the one this key already sits on: re-picking it is a no-op.
 *
 * The DOM is a hand-rolled stub, as in cfg-chip-input.test.js. The page script
 * is large, but the picker reaches only getElementById, createElement and the
 * delegated click listener, so the stub stays small.
 */

const test = require('node:test');
const assert = require('node:assert/strict');

// ---------------------------------------------------------------------------
// Minimal DOM
// ---------------------------------------------------------------------------

class FakeEl {
    constructor(tag) {
        this.tagName = tag;
        this.value = '';
        this.textContent = '';
        this.className = '';
        this.disabled = false;
        this.hidden = false;
        this.style = {};
        this.children = [];
        this.attrs = {};
        this.listeners = {};
    }

    getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; }

    setAttribute(name, value) { this.attrs[name] = String(value); }

    appendChild(child) { this.children.push(child); return child; }

    addEventListener(type, handler) {
        (this.listeners[type] = this.listeners[type] || []).push(handler);
    }

    querySelectorAll() { return []; }

    closest() { return null; }
}

/*
 * Open the picker for one key row against one set of applications.
 *
 * `apps` is the model the controller rendered into `ok-applications-data`, each
 * entry carrying the key types that application already holds. `row` describes
 * the key whose button was clicked.
 */
function openPicker({ apps, row }) {
    const elements = new Map();
    const el = (id) => {
        if (!elements.has(id)) elements.set(id, new FakeEl('div'));
        return elements.get(id);
    };

    el('oauth2-keys-root').attrs['data-generation-available'] = 'true';
    el('ok-applications-data').textContent = JSON.stringify(apps);

    const alerts = [];
    const documentListeners = {};
    global.showAlert = (message, type) => { alerts.push({ message, type }); };
    global.window = {
        apiPortalApi: { csrfToken: () => 'csrf' },
        location: { reload: () => {} },
        addEventListener: () => {},
    };
    global.document = {
        getElementById: (id) => el(id),
        createElement: (tag) => new FakeEl(tag),
        querySelectorAll: () => [],
        addEventListener: (type, handler) => {
            (documentListeners[type] = documentListeners[type] || []).push(handler);
        },
    };

    /*
     * The module is an IIFE that wires itself on load, so each case needs a fresh
     * evaluation against the fresh document above.
     *
     * It also starts a one-minute interval to re-render relative timestamps. In a
     * browser that is harmless; under the test runner an unreferenced interval
     * keeps the event loop alive and the process never exits, so it is neutered
     * for the duration of the load and restored immediately after.
     */
    const realSetInterval = global.setInterval;
    global.setInterval = () => 0;
    try {
        delete require.cache[require.resolve('./oauth2-keys-page')];
        require('./oauth2-keys-page');
    } finally {
        global.setInterval = realSetInterval;
    }

    // The row the button lives in, carrying what the key is and where it sits.
    const tr = new FakeEl('tr');
    tr.attrs['data-key-id'] = 'key-1';
    tr.attrs['data-key-type'] = row.keyType;
    tr.attrs['data-application-id'] = row.applicationId || '';

    // The association button, reached through the page's delegated click handler.
    const button = new FakeEl('button');
    button.closest = (selector) => (selector === 'tr' ? tr : null);
    const target = {
        closest: (selector) => (selector === '.ok-row-app' ? button : null),
    };
    (documentListeners.click || []).forEach((handler) => handler({ target }));

    const select = el('ok-app-select');
    return {
        alerts,
        modalShown: el('ok-app-modal').style.display === 'flex',
        options: select.children.map((o) => ({
            value: o.value, label: o.textContent, disabled: o.disabled,
        })),
        selected: select.value,
    };
}

const FREE = { id: 'billing-app', displayName: 'Billing', takenTypes: [] };
const HAS_PROD = { id: 'checkout-app', displayName: 'Checkout', takenTypes: ['PRODUCTION'] };
const HAS_BOTH = { id: 'portal-app', displayName: 'Portal', takenTypes: ['PRODUCTION', 'SANDBOX'] };

test('an application already holding this key type is listed but disabled', async () => {
    const picker = openPicker({ apps: [FREE, HAS_PROD], row: { keyType: 'PRODUCTION' } });

    const byValue = new Map(picker.options.map((o) => [o.value, o]));
    assert.equal(byValue.get('billing-app').disabled, false);
    assert.equal(byValue.get('checkout-app').disabled, true);
    assert.match(byValue.get('checkout-app').label, /already has a production key/);
    assert.equal(picker.modalShown, true);
});

test('the slot checked is this key own type, not a fixed one', async () => {
    // The same application is selectable for a sandbox key, because only its
    // production slot is taken.
    const picker = openPicker({ apps: [HAS_PROD], row: { keyType: 'SANDBOX' } });

    const checkout = picker.options.find((o) => o.value === 'checkout-app');
    assert.equal(checkout.disabled, false);
    assert.equal(picker.modalShown, true);
});

test('the application this key already sits on is never disabled', async () => {
    // Re-picking it is a no-op, not a conflict — the service excludes the key
    // being associated from its own conflict lookup for the same reason.
    const picker = openPicker({
        apps: [HAS_PROD],
        row: { keyType: 'PRODUCTION', applicationId: 'checkout-app' },
    });

    const checkout = picker.options.find((o) => o.value === 'checkout-app');
    assert.equal(checkout.disabled, false);
    assert.equal(picker.selected, 'checkout-app');
    assert.equal(picker.modalShown, true);
});

test('None is always offered first, since picking it dissociates', async () => {
    const picker = openPicker({ apps: [FREE], row: { keyType: 'PRODUCTION' } });

    assert.equal(picker.options[0].value, '');
    assert.match(picker.options[0].label, /None/);
});

test('with every application taken the modal does not open', async () => {
    const picker = openPicker({ apps: [HAS_PROD, HAS_BOTH], row: { keyType: 'PRODUCTION' } });

    assert.equal(picker.modalShown, false);
    assert.equal(picker.alerts.length, 1);
    assert.match(picker.alerts[0].message, /Every application already has an OAuth2 key/);
    assert.equal(picker.alerts[0].type, 'error');
});

test('a key already on an application still opens, so it can be dissociated', async () => {
    // Every application is full, but this key is on one of them. Refusing to open
    // here would leave no way to detach it — which is the only way to free a slot.
    const picker = openPicker({
        apps: [HAS_PROD, HAS_BOTH],
        row: { keyType: 'PRODUCTION', applicationId: 'checkout-app' },
    });

    assert.equal(picker.modalShown, true);
    assert.equal(picker.alerts.length, 0);
});

test('with no applications at all the message says to create one', async () => {
    const picker = openPicker({ apps: [], row: { keyType: 'PRODUCTION' } });

    assert.equal(picker.modalShown, false);
    assert.match(picker.alerts[0].message, /no applications yet/);
});
