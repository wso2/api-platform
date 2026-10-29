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
const { parseAllowedClients } = require('./allowedClients');

const SECTION = 'api_portal.health';

test('no entries allows loopback only', () => {
    for (const entries of [undefined, []]) {
        const set = parseAllowedClients(SECTION, entries);
        assert.equal(set.allowsAll, false);
        assert.equal(set.allows('127.0.0.1'), true);
        assert.equal(set.allows('::1'), true);
        assert.equal(set.allows('127.0.0.2'), false);
        assert.equal(set.allows('10.0.0.1'), false);
    }
});

test('an IPv4-mapped peer address matches its IPv4 entry', () => {
    const set = parseAllowedClients(SECTION, []);
    assert.equal(set.allows('::ffff:127.0.0.1'), true);
    assert.equal(set.allows('::ffff:10.0.0.1'), false);
});

test('"*" alone allows every client', () => {
    const set = parseAllowedClients(SECTION, ['*']);
    assert.equal(set.allowsAll, true);
    assert.equal(set.allows('203.0.113.9'), true);
    assert.equal(set.allows(undefined), true);
});

test('"*" with other entries is refused', () => {
    assert.throws(
        () => parseAllowedClients(SECTION, ['127.0.0.1', '*']),
        { message: 'api_portal.health.allowed_clients: "*" cannot be combined with other entries' },
    );
});

test('an invalid entry is refused, naming its index', () => {
    for (const bad of ['not-an-ip', '10.0.0.0/33', '10.1', '0x7f.0.0.1', 'fe80::1%eth0', '10.0.0.0/', '::ffff:10.0.0.0/90']) {
        assert.throws(
            () => parseAllowedClients(SECTION, ['127.0.0.1', bad]),
            { message: 'api_portal.health.allowed_clients[1] is not a valid IP address or CIDR range' },
            bad,
        );
    }
});

test('CIDR ranges and single addresses match only what they cover', () => {
    const set = parseAllowedClients(SECTION, [' 10.0.0.0/8 ', '192.168.1.5', 'fc00::/7']);
    assert.equal(set.allows('10.200.3.4'), true);
    assert.equal(set.allows('::ffff:10.200.3.4'), true);
    assert.equal(set.allows('11.0.0.1'), false);
    assert.equal(set.allows('192.168.1.5'), true);
    assert.equal(set.allows('192.168.1.6'), false);
    assert.equal(set.allows('fd12::1'), true);
    assert.equal(set.allows('fe80::1%eth0'), false);
    assert.equal(set.allows('127.0.0.1'), false);
});

test('an IPv4-mapped entry is reduced to IPv4', () => {
    const set = parseAllowedClients(SECTION, ['::ffff:10.0.0.0/104', '::ffff:192.0.2.1']);
    assert.equal(set.allows('10.1.2.3'), true);
    assert.equal(set.allows('192.0.2.1'), true);
    assert.equal(set.allows('192.0.2.2'), false);
});

test('an IPv6 zone on the peer address is ignored', () => {
    const set = parseAllowedClients(SECTION, ['fe80::/10']);
    assert.equal(set.allows('fe80::1%eth0'), true);
});

test('an unparseable or missing peer address is refused', () => {
    const set = parseAllowedClients(SECTION, ['10.0.0.0/8']);
    for (const peer of [undefined, null, '', 'garbage', 42]) {
        assert.equal(set.allows(peer), false, String(peer));
    }
});
