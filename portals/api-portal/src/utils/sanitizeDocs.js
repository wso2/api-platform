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

const sanitizeHtml = require('sanitize-html');

// Extend sanitize-html's defaults with the tags and attributes markdown docs
// universally need and that carry no execution/clobbering risk on their own.
// once a tag/attr lives in the base allowlist, no org can turn it back off.
const BASE_ALLOWED_TAGS = sanitizeHtml.defaults.allowedTags.concat([
    'img',                 // inline markdown images
    'del', 'ins',          // GFM strikethrough / insertion (marked emits <del> for ~~x~~)
    'details', 'summary',  // GitHub-style collapsible sections
]);

const BASE_ALLOWED_ATTRIBUTES = {
    ...sanitizeHtml.defaults.allowedAttributes,
    // Fenced code blocks: marked emits <code class="language-<lang>"> and every
    // syntax highlighter (Prism/highlight.js/Shiki) keys on it.
    code: ['class'],
    pre: ['class'],
    // Heading anchors (TOC navigation, URL fragments). Scoped to h1-h6 to narrow
    // the DOM-clobbering surface vs. allowing `id` on every tag.
    h1: ['id'], h2: ['id'], h3: ['id'],
    h4: ['id'], h5: ['id'], h6: ['id'],
    // GFM table semantics: alignment, row/column spans, and header scope.
    th: ['align', 'colspan', 'rowspan', 'scope'],
    td: ['align', 'colspan', 'rowspan'],
    // `rel` on <a> is a *security* enabler, not a feature: authors can harden
    // external links with rel="noopener noreferrer" alongside target="_blank".
    a: [...sanitizeHtml.defaults.allowedAttributes.a, 'rel'],
};

const ALLOWED_SCHEMES_BY_TAG = { img: ['http', 'https', 'data'] };

/**
 * Reads [sanitizeHtmlDocs] from the org's configuration JSON. An absent key is
 * treated as sanitization-enabled with no allowlist additions — the secure
 * default. An org opts out by explicitly setting `enabled: false`.
 */
const readSanitizeConfig = (orgConfiguration) => {
    const cfg = orgConfiguration?.sanitizeHtmlDocs;
    if (!cfg || typeof cfg !== 'object') {
        return { enabled: true, additionalAllowedTags: [], additionalAllowedAttributes: [] };
    }
    return {
        enabled: cfg.enabled !== false,
        additionalAllowedTags: Array.isArray(cfg.additionalAllowedTags) ? cfg.additionalAllowedTags : [],
        additionalAllowedAttributes: Array.isArray(cfg.additionalAllowedAttributes) ? cfg.additionalAllowedAttributes : [],
    };
};

/**
 * Sanitize markdown-rendered HTML. Pass `orgConfiguration` from
 * organizations.configuration (already parsed JSON) to apply any org-level
 * allowlist extensions; pass null for filesystem/design-mode sources with no
 * org context — sanitization still runs with the base allowlist.
 *
 * Returns `html` unchanged when the org has explicitly disabled sanitization
 * (`sanitizeHtmlDocs.enabled === false`).
 */
const sanitizeMarkdownHtml = (html, orgConfiguration) => {
    if (!html) return html;
    const cfg = readSanitizeConfig(orgConfiguration);
    if (!cfg.enabled) return html;

    const allowedTags = [...BASE_ALLOWED_TAGS, ...cfg.additionalAllowedTags];
    const allowedAttributes = { ...BASE_ALLOWED_ATTRIBUTES };
    if (cfg.additionalAllowedAttributes.length > 0) {
        allowedAttributes['*'] = [
            ...(allowedAttributes['*'] || []),
            ...cfg.additionalAllowedAttributes,
        ];
    }

    return sanitizeHtml(html, {
        allowedTags,
        allowedAttributes,
        allowedSchemesByTag: ALLOWED_SCHEMES_BY_TAG,
    });
};

/**
 * Serialize a value as JSON that is safe to inline inside executable tags.
 */
const safeJsonForScript = (value) => {
    const serialized = JSON.stringify(value);
    if (typeof serialized !== 'string') return 'null';
    return serialized
        .replace(/</g, '\\u003c')
        .replace(/\u2028/g, '\\u2028')
        .replace(/\u2029/g, '\\u2029');
};

module.exports = { sanitizeMarkdownHtml, readSanitizeConfig, safeJsonForScript };
