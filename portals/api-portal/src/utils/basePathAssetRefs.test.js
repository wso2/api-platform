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

// The portal mounts everything it serves under ROUTE.BASE_PATH, so a template's absolute
// asset URL has to be written `{{basePath}}/styles/…` (the handlebars helper in
// helpers/handlebarsHelpers.js) and a stylesheet's has to be relative to the stylesheet.
// A bare `/styles/…` resolves outside the mount and 404s.
//
// Asserted as a repo invariant rather than left to review: the prefix rollout has to hold
// across ~40 references in templates nobody re-checks, the failure is a silently missing
// stylesheet or icon rather than an error, and the theme-upload rewrites in util.js key
// off exactly these two shapes. A new reference in the wrong shape breaks all of that.
//
// Scoped to files a THEME can carry (src/defaultContent, and the sample themes under
// samples/layouts) plus the technical pages/assets under src/pages and src/styles that
// render alongside them.
const SRC_ROOT = path.resolve(__dirname, '..');
const REPO_ROOT = path.resolve(SRC_ROOT, '..');

const SCANNED_ROOTS = [
    path.join(SRC_ROOT, 'defaultContent'),
    path.join(SRC_ROOT, 'pages'),
    path.join(SRC_ROOT, 'styles'),
    path.join(REPO_ROOT, 'samples', 'layouts'),
];

// The mounts that live under BASE_PATH (app.js). `/mock` is design-mode only but is
// mounted the same way, so it belongs here too.
const PREFIXED_MOUNTS = ['styles', 'technical-styles', 'technical-scripts', 'images', 'mock'];
const MOUNT_ALTERNATION = PREFIXED_MOUNTS.join('|');
const BASE_PATH = require('./constants').ROUTE.BASE_PATH;

function collectFiles(dir, extensions, out = []) {
    if (!fs.existsSync(dir)) return out;
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) collectFiles(full, extensions, out);
        else if (extensions.some((ext) => entry.name.endsWith(ext))) out.push(full);
    }
    return out;
}

const rel = (file) => path.relative(REPO_ROOT, file);

// Everything that can precede `/<mount>/` in a reference, as a regex alternation. Only
// the {{basePath}} token is legal in a template; the literal prefix and the bare root are
// the two ways to get it wrong, so both are matched and then reported.
const BASE_PATH_TOKEN = '\\{\\{\\s*basePath\\s*\\}\\}';
const LITERAL_PREFIX = BASE_PATH.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

function scan(files, pattern) {
    const offenders = [];
    for (const file of files) {
        fs.readFileSync(file, 'utf8').split('\n').forEach((line, i) => {
            pattern.lastIndex = 0;
            if (pattern.test(line)) offenders.push(`${rel(file)}:${i + 1}: ${line.trim()}`);
        });
    }
    return offenders;
}

test('templates reference portal assets through {{basePath}}, never a bare or literal prefix', () => {
    // An opening quote (or url() followed by either a bare `/<mount>/` — no prefix at all —
    // or the hardcoded `/api-portal/<mount>/`. The latter renders correctly today but
    // silently defeats the single-source-of-truth the helper exists for, and would be
    // missed by util.js's theme rewrites, which match the token rather than the literal.
    const templateFiles = SCANNED_ROOTS.flatMap((r) => collectFiles(r, ['.hbs']));
    const wrongShape = new RegExp(
        `(?:["']|url\\(["']?)(?:${LITERAL_PREFIX})?/(?:${MOUNT_ALTERNATION})/`, 'g');
    const anyShape = new RegExp(
        `(?:["']|url\\(["']?)(?:${BASE_PATH_TOKEN}|${LITERAL_PREFIX})?/(?:${MOUNT_ALTERNATION})/`, 'g');

    const offenders = scan(templateFiles, wrongShape);
    assert.deepStrictEqual(offenders, [],
        'Absolute asset URLs in templates must be written "{{basePath}}/styles/main.css" — '
        + `neither bare ("/styles/...") nor hardcoded ("${BASE_PATH}/styles/..."):\n${offenders.join('\n')}`);

    // The scan is only meaningful if these references exist at all in the shape we accept.
    assert.ok(scan(templateFiles, anyShape).length > 10,
        'expected the shipped templates to carry {{basePath}}-prefixed asset references');
});

test('stylesheets reference sibling assets relatively, never rooted or prefixed', () => {
    // A stylesheet cannot interpolate {{basePath}}, so it must stay relative to itself:
    // that resolves correctly off the static mount AND lets util.rewriteViewStyleImports /
    // rewriteViewImages redirect it to the view asset endpoint when the sheet is served
    // from an uploaded theme. Both absolute forms break the second case — `/styles/x.css`
    // silently falls back to the built-in default, and `/api-portal/styles/x.css` isn't
    // matched by those rewrites at all.
    const styleFiles = SCANNED_ROOTS.flatMap((r) => collectFiles(r, ['.css']));
    const rooted = `(?:${LITERAL_PREFIX})?/(?:${MOUNT_ALTERNATION})/`;
    const badImport = new RegExp(`@import\\s*["']${rooted}`, 'g');
    const badUrl = new RegExp(`url\\(["']?${rooted}`, 'g');

    const offenders = [...scan(styleFiles, badImport), ...scan(styleFiles, badUrl)].sort();
    assert.deepStrictEqual(offenders, [],
        'Stylesheet references must be relative to the stylesheet (e.g. @import "home.css", '
        + `url("../images/icon.svg")) — not "/styles/..." and not "${BASE_PATH}/styles/...":`
        + `\n${offenders.join('\n')}`);
});

test('the scan actually covers the shipped templates and stylesheets', () => {
    // Guards the two tests above against silently passing because a path moved and the
    // walk now finds nothing.
    const templates = SCANNED_ROOTS.flatMap((r) => collectFiles(r, ['.hbs']));
    const stylesheets = SCANNED_ROOTS.flatMap((r) => collectFiles(r, ['.css']));
    assert.ok(templates.length > 30, `expected to scan the shipped templates, found ${templates.length}`);
    assert.ok(stylesheets.length > 5, `expected to scan the shipped stylesheets, found ${stylesheets.length}`);
});
