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
 * The associate picker on the application page.
 *
 * The controller now hands this script only the keys the application could
 * actually take — a key of a type it already holds is filtered out server-side —
 * so the list can arrive empty for a caller who owns several keys. The modal must
 * then say so instead of opening on a select with nothing in it.
 *
 * The DOM here is a hand-rolled stub, as in cfg-chip-input.test.js: the script
 * touches getElementById, createElement and addEventListener and nothing else,
 * and pulling in jsdom to reach three members would be a larger change than the
 * thing under test.
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
        this.disabled = false;
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

    click() { (this.listeners.click || []).forEach((h) => h({ target: this })); }
}

/*
 * One page load. `keys` is what the controller rendered into the config
 * element's data attribute — already filtered to what this application can take.
 */
function loadPage(keys, { hasAssociatedKeys = false, keysElsewhere = 0 } = {}) {
    const elements = new Map();
    const el = (id) => {
        if (!elements.has(id)) elements.set(id, new FakeEl('div'));
        return elements.get(id);
    };

    const config = el('application-oauth2-keys-config');
    config.attrs['data-app-id'] = 'checkout-app';
    config.attrs['data-available-oauth2-keys'] = JSON.stringify(keys);
    config.attrs['data-has-associated-keys'] = hasAssociatedKeys ? 'true' : 'false';
    config.attrs['data-keys-elsewhere'] = String(keysElsewhere);

    const alerts = [];
    global.showAlert = (message, type) => { alerts.push({ message, type }); };
    global.window = {
        apiPortalApi: { csrfToken: () => 'csrf' },
        location: { reload: () => {} },
    };
    global.document = {
        getElementById: (id) => el(id),
        createElement: (tag) => new FakeEl(tag),
        addEventListener: () => {},
    };

    // The module is an IIFE that wires itself on load, so each page load needs a
    // fresh evaluation against the fresh document above.
    delete require.cache[require.resolve('./application-oauth2-keys')];
    require('./application-oauth2-keys');

    el('btn-open-associate-oauth2-key').click();

    const select = el('associate-oauth2-key-select');
    return {
        alerts,
        modalShown: el('associate-oauth2-key-modal').style.display === 'flex',
        options: select.children.map((o) => o.textContent),
        optionValues: select.children.map((o) => o.value),
    };
}

test('with no keys at all the modal does not open', async () => {
    const page = loadPage([], { hasAssociatedKeys: false });

    assert.equal(page.modalShown, false);
    assert.equal(page.alerts.length, 1);
    assert.equal(page.alerts[0].message, 'No OAuth2 key is available to associate.');
    assert.equal(page.alerts[0].type, 'error');
});

test('the empty message does not tell the caller to go and create a key', async () => {
    // Even with nothing associated, the caller may own keys that are all on other
    // applications. Sending them off to create another would be wrong.
    const page = loadPage([], { hasAssociatedKeys: false });

    assert.doesNotMatch(page.alerts[0].message, /Generate one|create one/i);
});

test('an application that already holds a key says so, not "none available"', async () => {
    // The picker is empty because this application's slot is taken, not because
    // the caller owns nothing. Reporting the latter sends someone who knows they
    // own other keys looking for a bug that is not there.
    const page = loadPage([], { hasAssociatedKeys: true });

    assert.equal(page.modalShown, false);
    assert.equal(page.alerts.length, 1);
    assert.equal(
        page.alerts[0].message,
        'A key is already associated with this application. Remove it to associate another.'
    );
});

test('keys parked on other applications are explained, not reported as none', async () => {
    // They are not offered — taking one would detach it from an application this
    // page is not about — but the caller does own them, so saying "none available"
    // would be false from where they are sitting.
    const page = loadPage([], { hasAssociatedKeys: false, keysElsewhere: 2 });

    assert.equal(page.modalShown, false);
    assert.match(page.alerts[0].message, /associated with other applications/);
    assert.match(page.alerts[0].message, /OAuth2 Keys page/);
});

test('the three empty-picker reasons each get their own message', async () => {
    const nothingOwned = loadPage([], { hasAssociatedKeys: false, keysElsewhere: 0 });
    const slotTaken = loadPage([], { hasAssociatedKeys: true, keysElsewhere: 0 });
    const parkedElsewhere = loadPage([], { hasAssociatedKeys: false, keysElsewhere: 1 });

    const messages = [nothingOwned, slotTaken, parkedElsewhere].map((p) => p.alerts[0].message);
    assert.equal(new Set(messages).size, 3);
});

test('a key already on this application outranks keys parked elsewhere', async () => {
    // Both are true at once when the slot is taken here and other keys sit on other
    // applications. The one the caller can act on from this page comes first.
    const page = loadPage([], { hasAssociatedKeys: true, keysElsewhere: 3 });

    assert.match(page.alerts[0].message, /already associated with this application/);
});

test('an application holding a key can still associate one of another type', async () => {
    // A production key here does not block a sandbox key: the controller leaves
    // the sandbox key in the list, so the modal opens as normal.
    const page = loadPage(
        [{ keyId: 'k9', name: 'Spare', consumerKey: 'xyz', keyType: 'SANDBOX' }],
        { hasAssociatedKeys: true }
    );

    assert.equal(page.modalShown, true);
    assert.equal(page.alerts.length, 0);
});

test('an offerable key opens the modal and is listed', async () => {
    const page = loadPage([
        { keyId: 'k1', name: 'Checkout prod', consumerKey: 'abc123', keyType: 'PRODUCTION' },
    ]);

    assert.equal(page.modalShown, true);
    assert.equal(page.alerts.length, 0);
    assert.deepEqual(page.optionValues, ['', 'k1']);
    assert.equal(page.options[1], 'Checkout prod · abc123');
});

test('a sandbox key is marked, so the choice is legible before it is made', async () => {
    const page = loadPage([
        { keyId: 'k2', name: 'Spare', consumerKey: 'def456', keyType: 'SANDBOX' },
    ]);

    assert.match(page.options[1], /· Sandbox$/);
});

test('options carry no "(on X)" suffix, since nothing offered is attached', async () => {
    // The controller no longer sends keys that sit on another application, so an
    // option can never need to say where its key currently is. A stray
    // applicationName must not reintroduce that label.
    const page = loadPage([
        {
            keyId: 'k3', name: 'Shared', consumerKey: 'ghi789',
            keyType: 'PRODUCTION', applicationName: 'Billing',
        },
    ]);

    assert.equal(page.options[1], 'Shared · ghi789');
});
