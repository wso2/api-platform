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

/*
 * js-yaml v5 compatibility shim.
 *
 * js-yaml v5 changed `load()`/`loadAll()` to THROW on empty input, whereas v4
 * returned `undefined`. Much of this codebase relies on the v4 behavior — e.g.
 * `yaml.load(x) || {}`, `const doc = yaml.load(x); if (!doc) ...`, and ternaries
 * that treat an empty document as a falsy value. This shim restores the v4
 * empty-input => `undefined` contract so those call sites keep working, and
 * passes every other export (`dump`, schemas, etc.) straight through via a Proxy
 * so nothing else about v5 is altered.
 *
 * Import this instead of 'js-yaml' anywhere the empty-input case can occur.
 */
const yaml = require('js-yaml');

function isEmpty(input) {
    return input === null || input === undefined || String(input).trim() === '';
}

// v4-compatible: empty input yields `undefined` instead of throwing.
function load(input, options) {
    if (isEmpty(input)) return undefined;
    return yaml.load(input, options);
}

// v4-compatible: empty input yields no documents (undefined with an iterator,
// otherwise an empty array) instead of throwing.
function loadAll(input, iteratorOrOptions, options) {
    if (isEmpty(input)) {
        return typeof iteratorOrOptions === 'function' ? undefined : [];
    }
    return yaml.loadAll(input, iteratorOrOptions, options);
}

module.exports = new Proxy(yaml, {
    get(target, prop) {
        if (prop === 'load') return load;
        if (prop === 'loadAll') return loadAll;
        return target[prop];
    },
});
