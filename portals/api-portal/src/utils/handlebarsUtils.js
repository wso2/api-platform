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

/** Keys getValue does not look up. */
const NON_DATA_KEYS = new Set(['__proto__', 'constructor', 'prototype']);

/**
 * The getValue helper: the value of an object's own property, or undefined.
 *
 * @param {*} obj The value to read from.
 * @param {string} key The property name.
 * @returns {*} The own property's value, or undefined.
 */
function ownValue(obj, key) {
    if (obj === null || obj === undefined || typeof key !== 'string' || NON_DATA_KEYS.has(key)) {
        return undefined;
    }
    return Object.prototype.hasOwnProperty.call(obj, key) ? obj[key] : undefined;
}

/**
 * A partial that returns HTML that has already been rendered.
 *
 * The partial table is shared by every request, so a concurrent request can read this entry
 * before its own registration replaces it. The template it was rendered from is kept on the
 * function, so that request still renders from the template (templateOf).
 *
 * @param {string} html The rendered HTML.
 * @param {string} sourceTemplate The template the HTML was rendered from.
 * @returns {Function} A partial that returns the HTML.
 */
function renderedPartial(html, sourceTemplate) {
    const partial = () => html;
    partial.sourceTemplate = sourceTemplate;
    return partial;
}

/**
 * The template source of a partial: the string itself, or what a rendered partial came from.
 *
 * @param {string|Function|undefined} partial A registered partial.
 * @returns {string|undefined} The partial's template source.
 */
function templateOf(partial) {
    return typeof partial === 'function' ? partial.sourceTemplate : partial;
}

module.exports = { ownValue, renderedPartial, templateOf };
