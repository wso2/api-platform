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

const { sanitizeMarkdownHtml, readSanitizeConfig, safeJsonForScript } = require('./sanitizeDocs');

test('readSanitizeConfig: null org config → enabled with empty additions', () => {
    const cfg = readSanitizeConfig(null);
    assert.deepStrictEqual(cfg, { enabled: true, additionalAllowedTags: [], additionalAllowedAttributes: [] });
});

test('readSanitizeConfig: missing sanitizeHtmlDocs key → enabled with empty additions', () => {
    const cfg = readSanitizeConfig({ someOtherKey: 'value' });
    assert.deepStrictEqual(cfg, { enabled: true, additionalAllowedTags: [], additionalAllowedAttributes: [] });
});

test('readSanitizeConfig: enabled: false is honoured', () => {
    const cfg = readSanitizeConfig({ sanitizeHtmlDocs: { enabled: false } });
    assert.strictEqual(cfg.enabled, false);
});

test('readSanitizeConfig: non-array additions are coerced to []', () => {
    const cfg = readSanitizeConfig({ sanitizeHtmlDocs: { additionalAllowedTags: 'not-an-array' } });
    assert.deepStrictEqual(cfg.additionalAllowedTags, []);
});

test('sanitizeMarkdownHtml: strips <script> tag by default', () => {
    const result = sanitizeMarkdownHtml('<p>hi</p><script>alert(1)</script>', null);
    assert.ok(!result.includes('<script>'), 'script tag must be stripped');
    assert.ok(result.includes('<p>hi</p>'), 'legitimate paragraph must survive');
});

test('sanitizeMarkdownHtml: strips onerror event handler on <img>', () => {
    const result = sanitizeMarkdownHtml('<img src="x" onerror="alert(1)">', null);
    assert.ok(!result.includes('onerror'), 'onerror attribute must be stripped');
});

test('sanitizeMarkdownHtml: strips javascript: URI from <a href>', () => {
    const result = sanitizeMarkdownHtml('<a href="javascript:alert(1)">click</a>', null);
    assert.ok(!result.includes('javascript:'), 'javascript: URI must be stripped');
});

test('sanitizeMarkdownHtml: <img> tag is allowed with http(s)/data src by default', () => {
    const httpImg = sanitizeMarkdownHtml('<img src="https://example.com/a.png" alt="x">', null);
    assert.ok(httpImg.includes('<img'), 'img tag must survive');
    assert.ok(httpImg.includes('src="https://example.com/a.png"'), 'https src must survive');

    const dataImg = sanitizeMarkdownHtml('<img src="data:image/png;base64,AAA" alt="y">', null);
    assert.ok(dataImg.includes('src="data:image/png;base64,AAA"'), 'data: URI on img must survive');
});

test('sanitizeMarkdownHtml: null/empty input returned unchanged', () => {
    assert.strictEqual(sanitizeMarkdownHtml(null, null), null);
    assert.strictEqual(sanitizeMarkdownHtml('', null), '');
});

test('sanitizeMarkdownHtml: enabled: false returns HTML unchanged', () => {
    const dirty = '<script>alert(1)</script>';
    const result = sanitizeMarkdownHtml(dirty, { sanitizeHtmlDocs: { enabled: false } });
    assert.strictEqual(result, dirty, 'org opt-out must bypass sanitization');
});

test('sanitizeMarkdownHtml: additionalAllowedTags lets through tags that are normally stripped', () => {
    const dirty = '<iframe src="https://example.com"></iframe>';
    const defaultResult = sanitizeMarkdownHtml(dirty, null);
    assert.ok(!defaultResult.includes('<iframe'), 'iframe stripped by default');

    const extended = sanitizeMarkdownHtml(dirty, {
        sanitizeHtmlDocs: { additionalAllowedTags: ['iframe'] },
    });
    assert.ok(extended.includes('<iframe'), 'iframe allowed when added to allowlist');
});

test('sanitizeMarkdownHtml: additionalAllowedAttributes lets through attrs that are normally stripped', () => {
    const dirty = '<p data-custom="x">hi</p>';
    const defaultResult = sanitizeMarkdownHtml(dirty, null);
    assert.ok(!defaultResult.includes('data-custom'), 'data-custom stripped by default');

    const extended = sanitizeMarkdownHtml(dirty, {
        sanitizeHtmlDocs: { additionalAllowedAttributes: ['data-custom'] },
    });
    assert.ok(extended.includes('data-custom="x"'), 'data-custom allowed when added to allowlist');
});

test('sanitizeMarkdownHtml: markdown-essential tags and per-tag attributes survive the base allowlist', () => {
    const html = [
        '<pre><code class="language-javascript">const x = 1;</code></pre>',
        '<h2 id="section-anchor">Section</h2>',
        '<table><thead><tr><th align="center" colspan="2" scope="col">H</th></tr></thead>',
        '<tbody><tr><td align="right" rowspan="2">c</td></tr></tbody></table>',
        '<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>',
        '<p><del>deleted</del> <ins>inserted</ins></p>',
        '<details><summary>Toggle</summary><p>body</p></details>',
    ].join('\n');
    const result = sanitizeMarkdownHtml(html, null);

    assert.ok(result.includes('class="language-javascript"'), 'code class survives (syntax highlighting)');
    assert.ok(result.includes('id="section-anchor"'), 'heading id survives (anchors)');
    assert.ok(result.includes('align="center"') && result.includes('align="right"'), 'th/td align survives');
    assert.ok(result.includes('colspan="2"') && result.includes('rowspan="2"'), 'th/td colspan/rowspan survive');
    assert.ok(result.includes('scope="col"'), 'th scope survives');
    assert.ok(result.includes('rel="noopener noreferrer"'), 'a rel survives (noopener hardening)');
    assert.ok(result.includes('<del>') && result.includes('<ins>'), 'del/ins tags survive');
    assert.ok(result.includes('<details>') && result.includes('<summary>'), 'details/summary tags survive');
});

test('safeJsonForScript: escapes < and U+2028/U+2029, round-trips via JSON.parse', () => {
    const payload = {
        desc: '</script><script>alert(1)</script>',
        note: 'line\u2028break\u2029again',
        safe: 'ordinary text with "quotes" and \\ backslashes',
    };
    const encoded = safeJsonForScript(payload);

    assert.ok(!encoded.includes('</script>'), '</script> sequence stripped (prevents HTML parser breakout)');
    assert.ok(!encoded.includes('\u2028'), 'raw U+2028 absent (prevents JS line-terminator breakout)');
    assert.ok(!encoded.includes('\u2029'), 'raw U+2029 absent');
    assert.ok(encoded.includes('\\u003c'), '< encoded as \\u003c');

    assert.deepStrictEqual(JSON.parse(encoded), payload, 'round-trips to the original object via JSON.parse');

    assert.strictEqual(safeJsonForScript(undefined), 'null', 'undefined → "null" literal (keeps the host <script> syntactically valid)');
});
