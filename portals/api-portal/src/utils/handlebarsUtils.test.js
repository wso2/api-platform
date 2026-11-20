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
const fs = require('node:fs');
const path = require('node:path');
const Handlebars = require('handlebars');
const { marked } = require('marked');
const { ownValue, renderedPartial, templateOf } = require('./handlebarsUtils');

const API_DOC_TEMPLATE = fs.readFileSync(
    path.join(__dirname, '..', 'defaultContent', 'pages', 'docs', 'partials', 'api-doc.hbs'), 'utf8');

// Renders a document page the way registerDocsPageContent and the docs page do.
function renderDocPage(markdown) {
    const hb = Handlebars.create();
    const html = hb.compile(API_DOC_TEMPLATE)({ baseUrl: '/b', apiMD: marked.parse(markdown) });
    hb.registerPartial('api-doc', renderedPartial(html, API_DOC_TEMPLATE));
    return hb.compile('<main>{{> api-doc}}</main>')({ baseUrl: '/b' });
}

test('getValue returns own properties', () => {
    assert.strictEqual(ownValue({ a: 1 }, 'a'), 1);
    assert.strictEqual(ownValue([10, 20], '1'), 20);
    assert.strictEqual(ownValue('text', 'length'), 4);
});

test('getValue returns undefined for properties the object does not have', () => {
    assert.strictEqual(ownValue({ a: 1 }, 'b'), undefined);
    assert.strictEqual(ownValue({}, 'toString'), undefined);
    assert.strictEqual(ownValue(null, 'a'), undefined);
    assert.strictEqual(ownValue(undefined, 'a'), undefined);
});

test('a document page shows the rendered Markdown', () => {
    const out = renderDocPage('# Title\n\nSome **bold** text.');
    assert.match(out, /<h1[^>]*>Title<\/h1>/);
    assert.match(out, /<strong>bold<\/strong>/);
    assert.match(out, /^<main>[\s\S]*api-markdown-content[\s\S]*<\/main>$/);
});

test('templateOf returns the template of a rendered partial', () => {
    assert.strictEqual(templateOf('<p>{{title}}</p>'), '<p>{{title}}</p>');
    assert.strictEqual(templateOf(renderedPartial('<p>done</p>', '<p>{{title}}</p>')), '<p>{{title}}</p>');
    assert.strictEqual(templateOf(undefined), undefined);
});
