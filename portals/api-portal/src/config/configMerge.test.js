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
const assert = require('node:assert/strict');

const { snakeToCamelDeep, mergeOver, parseConfigPaths } = require('./configMerge');

test('parseConfigPaths collects repeatable --config flags in order', () => {
    assert.deepEqual(
        parseConfigPaths(['--config', 'base.toml', '--config', 'overlay.toml']),
        ['base.toml', 'overlay.toml'],
    );
});

test('parseConfigPaths supports the --config=<path> form', () => {
    assert.deepEqual(parseConfigPaths(['--config=base.toml']), ['base.toml']);
});

test('parseConfigPaths ignores unrelated arguments', () => {
    assert.deepEqual(
        parseConfigPaths(['--inspect', '--config', 'base.toml', 'extra']),
        ['base.toml'],
    );
});

test('parseConfigPaths returns an empty list when no --config is present', () => {
    assert.deepEqual(parseConfigPaths(['--inspect']), []);
});

test('parseConfigPaths throws when --config has no value', () => {
    assert.throws(() => parseConfigPaths(['--config']), /requires a file path/);
    assert.throws(() => parseConfigPaths(['--config', '--other']), /requires a file path/);
    assert.throws(() => parseConfigPaths(['--config=']), /non-empty file path/);
});

test('mergeOver deep-merges nested tables key by key', () => {
    const dst = { server: { port: 9543, host: 'a' } };
    mergeOver(dst, { server: { host: 'b' } });
    assert.deepEqual(dst, { server: { port: 9543, host: 'b' } });
});

test('mergeOver replaces arrays wholesale rather than merging index-wise', () => {
    const dst = { scopes: ['read', 'write', 'admin'] };
    mergeOver(dst, { scopes: ['read'] });
    assert.deepEqual(dst.scopes, ['read']);
});

test('mergeOver applied sequentially gives last-wins layering', () => {
    const merged = [
        { a: 1, nested: { x: 1, y: 1 } },
        { a: 2, nested: { y: 2 } },
        { nested: { z: 3 } },
    ].reduce((acc, file) => mergeOver(acc, file), {});
    assert.deepEqual(merged, { a: 2, nested: { x: 1, y: 2, z: 3 } });
});

test('mergeOver ignores prototype-polluting keys', () => {
    const dst = {};
    mergeOver(dst, JSON.parse('{"__proto__": {"polluted": true}}'));
    assert.equal({}.polluted, undefined);
});

test('snakeToCamelDeep converts nested keys and preserves arrays', () => {
    assert.deepEqual(
        snakeToCamelDeep({ base_url: 'x', nested_table: { max_open_conns: 5 }, list_key: [{ a_b: 1 }] }),
        { baseUrl: 'x', nestedTable: { maxOpenConns: 5 }, listKey: [{ aB: 1 }] },
    );
});
