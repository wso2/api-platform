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
 * Unit tests for the shared chip input.
 *
 * Two fields now depend on this one file — an API's tags and a key manager's
 * scopes — so a regression here is a regression in both, which is exactly why
 * the behaviour was lifted out of settings-apis.js in the first place. The
 * Cypress suites drive the fields, not the helper, and neither exercises the
 * paste or dedupe paths at all.
 *
 * The DOM here is a hand-rolled stub rather than jsdom: the module touches a
 * dozen DOM members in total, and adding a dependency to reach them would be a
 * larger change than the thing under test. The stub parses exactly one HTML
 * shape — the chip fragment this module itself writes — and nothing else.
 */

const test = require('node:test');
const assert = require('node:assert');

// ---------------------------------------------------------------------------
// Minimal DOM
// ---------------------------------------------------------------------------

/* The inverse of the module's own esc(). Kept deliberately separate from it so
   an escaping assertion below reads the raw written HTML, never a round-trip
   through a decoder that shares the bug. */
function decodeEntities(str) {
    return String(str)
        .replace(/&lt;/g, '<').replace(/&gt;/g, '>')
        .replace(/&quot;/g, '"').replace(/&#39;/g, "'")
        .replace(/&amp;/g, '&');
}

class FakeEl {
    constructor(tag) {
        this.tagName = tag;
        this.className = '';
        this.value = '';
        this.dataset = {};
        this.children = [];
        this.listeners = {};
        this.focusCount = 0;
        this.selectionStart = null;
        this.selectionEnd = null;
        this._html = '';
    }

    get innerHTML() { return this._html; }

    /* The module assigns innerHTML in exactly two places: '' to empty the chip
       container, and one chip's label + remove button. Recognising that single
       fragment is all the parsing needed for querySelector to find the button. */
    set innerHTML(html) {
        this._html = html;
        this.children = [];
        const m = /class="cfg-chip-remove" data-value="([^"]*)"/.exec(html);
        if (m) {
            const btn = new FakeEl('button');
            btn.className = 'cfg-chip-remove';
            btn.dataset.value = decodeEntities(m[1]);
            this.children.push(btn);
        }
    }

    querySelector(sel) {
        return this.children.find((c) => '.' + c.className === sel) || null;
    }

    appendChild(child) { this.children.push(child); return child; }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    dispatch(type, ev) { (this.listeners[type] || []).forEach((fn) => fn(ev)); }
    focus() { this.focusCount += 1; }
}

const registry = new Map();
global.window = {};
global.document = {
    getElementById: (id) => registry.get(id) || null,
    createElement: (tag) => new FakeEl(tag),
};

require('./cfg-chip-input');
const { create } = global.window.cfgChipInput;

/** Installs the three elements create() looks for and returns them. */
function mount(opts) {
    registry.clear();
    const wrap = new FakeEl('div');
    const chips = new FakeEl('div');
    const input = new FakeEl('input');
    if (!(opts && opts.noWrap)) registry.set('w', wrap);
    if (!(opts && opts.noChips)) registry.set('c', chips);
    if (!(opts && opts.noInput)) registry.set('i', input);
    return { wrap, chips, input };
}

function build(createOpts, mountOpts) {
    const els = mount(mountOpts);
    const api = create(Object.assign({ wrapId: 'w', chipsId: 'c', inputId: 'i' }, createOpts));
    return Object.assign({ api }, els);
}

/** The label text of each rendered chip, in order. */
function rendered(chips) {
    return chips.children.map((c) => decodeEntities(c.innerHTML.split('<button')[0]));
}

function key(name) {
    const ev = { key: name, prevented: false, preventDefault() { this.prevented = true; } };
    return ev;
}

// ---------------------------------------------------------------------------
// Absent markup — the reason every call site guards the return
// ---------------------------------------------------------------------------

test('create returns null when the field is not on the page', () => {
    /*
     * The settings bundle loads on pages that have neither field. create() is
     * called at script load, so returning null rather than throwing is what keeps
     * an unrelated page working — and is why both call sites test the result
     * before using it.
     */
    assert.equal(build({}, { noInput: true }).api, null, 'no input element');
    assert.equal(build({}, { noChips: true }).api, null, 'no chip container');
});

test('a missing wrapper is not fatal, since it only forwards clicks', () => {
    const { api } = build({}, { noWrap: true });
    assert.ok(api, 'the field still works without its clickable wrapper');
    api.set(['a']);
    assert.deepEqual(api.get(), ['a']);
});

test('clicking the wrapper focuses the input', () => {
    const { wrap, input } = build();
    wrap.dispatch('click', {});
    assert.equal(input.focusCount, 1, 'the whole box behaves as one field');
});

// ---------------------------------------------------------------------------
// set / get / clear
// ---------------------------------------------------------------------------

test('set accepts an array and get returns it', () => {
    const { api, chips } = build();
    api.set(['alpha', 'beta']);
    assert.deepEqual(api.get(), ['alpha', 'beta']);
    assert.deepEqual(rendered(chips), ['alpha', 'beta']);
});

test('set accepts a separator-delimited string, which is how stored values arrive', () => {
    // Scopes are persisted space-separated, tags comma-separated; both reach the
    // field as one string and must land as separate chips.
    const scopes = build({ splitOn: /[\s,]+/, commitKey: ' ' }).api;
    scopes.set('openid profile email');
    assert.deepEqual(scopes.get(), ['openid', 'profile', 'email']);

    const tags = build().api;
    tags.set('one,two');
    assert.deepEqual(tags.get(), ['one', 'two']);
});

test('set replaces rather than appends, and clears a half-typed value', () => {
    const { api, input } = build();
    api.set(['first']);
    input.value = 'leftover';
    api.set(['second']);
    assert.deepEqual(api.get(), ['second'], 'the previous contents are gone');
    assert.equal(input.value, '', 'and so is the uncommitted text');
});

test('set treats empty, null and undefined as "no chips"', () => {
    const { api } = build();
    api.set(['x']);
    for (const empty of [undefined, null, '', []]) {
        api.set(empty);
        assert.deepEqual(api.get(), [], `${JSON.stringify(empty)} clears the field`);
    }
});

test('get returns a copy, so a caller cannot mutate the chips behind the field', () => {
    const { api } = build();
    api.set(['a']);
    api.get().push('injected');
    assert.deepEqual(api.get(), ['a']);
});

test('clear empties the chips and the input together', () => {
    const { api, input, chips } = build();
    api.set(['a', 'b']);
    input.value = 'typing';
    api.clear();
    assert.deepEqual(api.get(), []);
    assert.equal(input.value, '');
    assert.deepEqual(rendered(chips), []);
});

// ---------------------------------------------------------------------------
// Committing
// ---------------------------------------------------------------------------

test('Enter commits the typed value instead of submitting the form', () => {
    const { api, input } = build();
    input.value = 'tag-one';
    const ev = key('Enter');
    input.dispatch('keydown', ev);
    assert.ok(ev.prevented, 'Enter must not reach the surrounding form');
    assert.deepEqual(api.get(), ['tag-one']);
    assert.equal(input.value, '');
});

test('the separator key commits instead of being typed as a character', () => {
    const { api, input } = build();
    input.value = 'tag-one';
    const ev = key(',');
    input.dispatch('keydown', ev);
    assert.ok(ev.prevented);
    assert.deepEqual(api.get(), ['tag-one'], 'no stray comma in the value');
});

test('an extra commitKey commits too — space, for scopes', () => {
    // Space is the separator OAuth2 itself uses, and the habit the old
    // space-separated text field taught.
    const { api, input } = build({ splitOn: /[\s,]+/, commitKey: ' ' });
    input.value = 'openid';
    const ev = key(' ');
    input.dispatch('keydown', ev);
    assert.ok(ev.prevented);
    assert.deepEqual(api.get(), ['openid']);
});

test('space is an ordinary character for a field that did not ask for it', () => {
    const { api, input } = build();
    input.value = 'two words';
    input.dispatch('keydown', key(' '));
    assert.deepEqual(api.get(), [], 'nothing committed');
    assert.equal(input.value, 'two words', 'and the text is untouched');
});

test('blur commits, so a typed value survives clicking Save', () => {
    /*
     * The failure this prevents: type a scope, click Save without pressing Enter,
     * and the scope is silently dropped from the request.
     */
    const { api, input } = build();
    input.value = 'nearly-lost';
    input.dispatch('blur', {});
    assert.deepEqual(api.get(), ['nearly-lost']);
});

test('committing an empty or whitespace-only input adds nothing', () => {
    const { api, input } = build();
    for (const blank of ['', '   ', ',', ' , ']) {
        input.value = blank;
        input.dispatch('blur', {});
        assert.deepEqual(api.get(), [], `${JSON.stringify(blank)} is not a chip`);
    }
});

test('a committed value is trimmed', () => {
    const { api, input } = build();
    input.value = '  padded  ';
    input.dispatch('blur', {});
    assert.deepEqual(api.get(), ['padded']);
});

test('one commit can produce several chips', () => {
    const { api, input } = build();
    input.value = 'a, b ,c';
    input.dispatch('blur', {});
    assert.deepEqual(api.get(), ['a', 'b', 'c']);
});

// ---------------------------------------------------------------------------
// Dedupe
// ---------------------------------------------------------------------------

test('a duplicate is ignored case-insensitively, keeping the casing first entered', () => {
    // Unlike a free-text field the duplicate would be plainly visible, so the same
    // value in two casings is a typo every time.
    const { api } = build();
    api.set(['OpenID', 'openid', 'OPENID', 'profile']);
    assert.deepEqual(api.get(), ['OpenID', 'profile']);
});

test('a duplicate typed later does not displace the existing chip', () => {
    const { api, input } = build();
    api.set(['Alpha', 'beta']);
    input.value = 'ALPHA';
    input.dispatch('blur', {});
    assert.deepEqual(api.get(), ['Alpha', 'beta'], 'order and casing both preserved');
});

// ---------------------------------------------------------------------------
// Removal
// ---------------------------------------------------------------------------

test('Backspace on an empty input removes the last chip', () => {
    const { api, input } = build();
    api.set(['a', 'b']);
    const ev = key('Backspace');
    input.dispatch('keydown', ev);
    assert.ok(ev.prevented);
    assert.deepEqual(api.get(), ['a']);
});

test('Backspace while text is typed edits the text, not the chips', () => {
    const { api, input } = build();
    api.set(['a']);
    input.value = 'par';
    const ev = key('Backspace');
    input.dispatch('keydown', ev);
    assert.ok(!ev.prevented, 'the browser handles the deletion');
    assert.deepEqual(api.get(), ['a']);
});

test('Backspace with nothing to remove is harmless', () => {
    const { api, input } = build();
    input.dispatch('keydown', key('Backspace'));
    assert.deepEqual(api.get(), []);
});

test('the remove button deletes its own chip and returns focus to the input', () => {
    const { api, chips, input } = build();
    api.set(['a', 'b', 'c']);
    let stopped = false;
    chips.children[1].querySelector('.cfg-chip-remove')
        .dispatch('click', {
            currentTarget: chips.children[1].querySelector('.cfg-chip-remove'),
            stopPropagation() { stopped = true; },
        });
    assert.deepEqual(api.get(), ['a', 'c'], 'the middle chip, not the last');
    assert.ok(stopped, 'the click must not also reach the wrapper and refocus twice');
    assert.equal(input.focusCount, 1);
});

// ---------------------------------------------------------------------------
// Paste
// ---------------------------------------------------------------------------

test('a pasted list is split into chips', () => {
    const { api, input } = build();
    const ev = {
        clipboardData: { getData: () => 'a, b, c' },
        prevented: false,
        preventDefault() { this.prevented = true; },
    };
    input.dispatch('paste', ev);
    assert.ok(ev.prevented);
    assert.deepEqual(api.get(), ['a', 'b', 'c']);
});

test('pasted text joins a half-typed value instead of orphaning it', () => {
    /*
     * "foo" already typed, "bar, baz" pasted at the end, commits as "foobar" and
     * "baz" — exactly what typing those same characters would have done.
     */
    const { api, input } = build();
    input.value = 'foo';
    input.selectionStart = 3;
    input.selectionEnd = 3;
    input.dispatch('paste', {
        clipboardData: { getData: () => 'bar, baz' },
        preventDefault() {},
    });
    assert.deepEqual(api.get(), ['foobar', 'baz']);
});

test('a paste over a selection replaces it', () => {
    const { api, input } = build();
    input.value = 'keepDROPme';
    input.selectionStart = 4;
    input.selectionEnd = 8;
    input.dispatch('paste', {
        clipboardData: { getData: () => 'x, y' },
        preventDefault() {},
    });
    assert.deepEqual(api.get(), ['keepx', 'yme']);
});

test('a paste with nothing to split on is left to the browser', () => {
    // Preventing the default here would mean re-implementing ordinary paste.
    const { api, input } = build();
    const ev = {
        clipboardData: { getData: () => 'single' },
        prevented: false,
        preventDefault() { this.prevented = true; },
    };
    input.dispatch('paste', ev);
    assert.ok(!ev.prevented, 'the browser pastes it normally');
    assert.deepEqual(api.get(), [], 'and nothing is committed yet');
});

test('a paste event carrying no clipboard data is ignored', () => {
    const { api, input } = build();
    input.dispatch('paste', { clipboardData: null, preventDefault() {} });
    assert.deepEqual(api.get(), []);
});

// ---------------------------------------------------------------------------
// Escaping
// ---------------------------------------------------------------------------

test('a chip value is HTML-escaped on the way into the DOM', () => {
    /*
     * Chips are built with innerHTML, so this is the output encoding for the
     * field. Both callers render values that came back from the server, and a
     * scope or tag is free text — js-output-encoding-xss.md directive 1.
     */
    const { api, chips } = build();
    api.set(['<img src=x onerror=alert(1)>']);
    const html = chips.children[0].innerHTML;
    assert.ok(!html.includes('<img'), 'no live tag reaches the DOM');
    assert.ok(html.includes('&lt;img'), 'it is escaped, not stripped');
    assert.deepEqual(api.get(), ['<img src=x onerror=alert(1)>'], 'the value itself is unchanged');
});

test('a quote in a value cannot break out of the remove button attribute', () => {
    const { api, chips } = build();
    api.set(['a" onclick="alert(1)']);
    const html = chips.children[0].innerHTML;
    assert.ok(html.includes('&quot;'), 'the quote is encoded');
    assert.ok(!/data-value="a" /.test(html), 'the attribute is not terminated early');
});

test('an escaped value still round-trips through its remove button', () => {
    // The chip is removed by the data-value the browser decodes, so escaping must
    // not break the match.
    const { api, chips } = build();
    api.set(['plain', '<b>&amp;</b>']);
    const btn = chips.children[1].querySelector('.cfg-chip-remove');
    btn.dispatch('click', { currentTarget: btn, stopPropagation() {} });
    assert.deepEqual(api.get(), ['plain']);
});

test('ampersands and apostrophes survive a set/get round trip unchanged', () => {
    const { api } = build();
    const awkward = ["O'Brien & Sons", 'a&amp;b'];
    api.set(awkward);
    assert.deepEqual(api.get(), awkward, 'escaping is for display only');
});
