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
 * [api_portal.monetization] disabled, as the Settings page shows it (showMonetization): the
 * Monetization section of the menu, its panels, scripts and dialogs, and the plan dialog's Paid
 * plan option. The page is rendered with the menu and dialogs as they are and the other partials
 * replaced by markers.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const configLoader = path.resolve(__dirname, '../config/configLoader.js');
require.cache[configLoader] = { id: configLoader, filename: configLoader, loaded: true,
    exports: { config: { designMode: { enabled: false } } } };

const Handlebars = require('handlebars');
const { registerHelpers } = require('../helpers/handlebarsHelpers');

registerHelpers();

const SETTINGS = path.resolve(__dirname, '../pages/settings');
const read = (relative) => fs.readFileSync(path.join(SETTINGS, relative), 'utf8');
const page = read('page.hbs');
const modals = read('partials/cfg-modals.hbs');
for (const source of [page, modals]) {
    for (const [, name] of source.matchAll(/{{>\s*([\w-]+)/g)) {
        Handlebars.registerPartial(name, `[partial ${name}]`);
    }
}
Handlebars.registerPartial('cfg-nav', read('partials/cfg-nav.hbs'));
Handlebars.registerPartial('cfg-modals', modals);
const renderPage = Handlebars.compile(page);

function render(showMonetization) {
    const context = { basePath: '/portal', slots: {}, showMonetization };
    const html = renderPage(context);
    return { html, scripts: context.slots.scripts || '' };
}

const MONETIZATION_PANELS = ['cfg-billing-panel', 'cfg-meters-panel', 'cfg-revenue-panel'];
const MONETIZATION_SCRIPTS = ['settings-meters.js', 'settings-billing.js', 'revenue-totals.js', 'settings-revenue.js'];
const MONETIZATION_DIALOGS = ['[partial cfg-billing-form]', 'id="cfg-billing-remove-modal"', 'id="cfg-meter-modal"',
    'id="cfg-delete-meter-modal"'];

function pricingFieldset(html) {
    const tag = html.match(/<fieldset[^>]*id="pol-pricing-fields"[^>]*>/);
    assert.ok(tag, 'the plan dialog has its pricing fieldset');
    return tag[0];
}

test('Settings shows the Monetization section, its panels, scripts and dialogs, and the Paid plan option', () => {
    const { html, scripts } = render(true);
    assert.match(html, /MONETIZATION/);
    for (const panel of ['cfg-billing', 'cfg-meters', 'cfg-revenue']) {
        assert.ok(html.includes(`data-panel="${panel}"`), `menu link ${panel}`);
    }
    for (const panel of MONETIZATION_PANELS) assert.ok(html.includes(`[partial ${panel}]`), panel);
    for (const script of MONETIZATION_SCRIPTS) assert.ok(scripts.includes(`/technical-scripts/${script}`), script);
    for (const dialog of MONETIZATION_DIALOGS) assert.ok(html.includes(dialog), dialog);
    assert.doesNotMatch(pricingFieldset(html), /\shidden\b/);
});

test('with monetization disabled, Settings leaves out the Monetization section and hides the Paid plan option', () => {
    const { html, scripts } = render(false);
    assert.doesNotMatch(html, /MONETIZATION/);
    for (const panel of ['cfg-billing', 'cfg-meters', 'cfg-revenue']) {
        assert.ok(!html.includes(`data-panel="${panel}"`), `menu link ${panel}`);
    }
    for (const panel of MONETIZATION_PANELS) assert.ok(!html.includes(`[partial ${panel}]`), panel);
    for (const script of MONETIZATION_SCRIPTS) assert.ok(!scripts.includes(`/technical-scripts/${script}`), script);
    for (const dialog of MONETIZATION_DIALOGS) assert.ok(!html.includes(dialog), dialog);
    // Hidden rather than left out: settings-plans.js binds to these fields at load.
    assert.match(pricingFieldset(html), /\shidden\b/);
    assert.ok(html.includes('id="pol-paid"'));
});

test('with monetization disabled, the rest of Settings is unchanged', () => {
    const shown = render(true);
    const hidden = render(false);
    for (const panel of ['cfg-organizations', 'cfg-plans', 'cfg-keymanagers', 'cfg-apis', 'cfg-webhooks', 'cfg-theming']) {
        assert.ok(hidden.html.includes(`data-panel="${panel}"`), `menu link ${panel}`);
    }
    for (const panel of ['cfg-plans-panel', 'cfg-webhooks-panel', 'cfg-keymanagers-panel']) {
        assert.ok(hidden.html.includes(`[partial ${panel}]`), panel);
    }
    for (const script of ['settings-nav.js', 'settings-plans.js', 'settings-webhooks.js']) {
        assert.ok(hidden.scripts.includes(`/technical-scripts/${script}`), script);
    }
    const others = (scripts) => scripts.split('\n')
        .filter((line) => !MONETIZATION_SCRIPTS.some((script) => line.includes(script)));
    assert.deepEqual(others(hidden.scripts), others(shown.scripts));
    assert.ok(hidden.html.includes('id="cfg-plan-modal"'));
});
