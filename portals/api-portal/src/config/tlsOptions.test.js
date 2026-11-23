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
const { buildTLSOptions } = require('./tlsOptions');

const VALID = {
    minimumProtocolVersion: 'TLS1_2',
    maximumProtocolVersion: 'TLS1_3',
    ecdhCurves: 'X25519,P-256',
    ciphers: '',
};

// Each invalid config with the field its error message must name.
const INVALID_CASES = [
    { name: 'unknown minimum version', cfg: { minimumProtocolVersion: 'TLS9' }, field: 'minimum_protocol_version' },
    { name: 'unknown maximum version', cfg: { maximumProtocolVersion: 'TLS9' }, field: 'maximum_protocol_version' },
    { name: 'inverted version range', cfg: { minimumProtocolVersion: 'TLS1_3', maximumProtocolVersion: 'TLS1_2' }, field: 'minimum_protocol_version' },
    { name: 'empty ecdh curves', cfg: { ecdhCurves: '' }, field: 'ecdh_curves' },
    { name: 'unsupported ecdh curve', cfg: { ecdhCurves: 'X448' }, field: 'ecdh_curves' },
    { name: 'unsupported cipher', cfg: { ciphers: 'NOT-A-CIPHER' }, field: 'ciphers' },
];

test('buildTLSOptions accepts a valid config', () => {
    const options = buildTLSOptions(VALID, 'http_client.tls');
    assert.equal(options.minVersion, 'TLSv1.2');
    assert.equal(options.maxVersion, 'TLSv1.3');
    assert.equal(options.ecdhCurve, 'X25519:prime256v1');
});

for (const { name, cfg, field } of INVALID_CASES) {
    test(`buildTLSOptions names the given section on ${name}`, () => {
        assert.throws(
            () => buildTLSOptions({ ...VALID, ...cfg }, 'http_client.tls'),
            (err) => {
                assert.ok(err.message.startsWith(`http_client.tls.${field}`), err.message);
                assert.ok(!err.message.includes('server.https'), err.message);
                return true;
            },
        );
    });

    test(`buildTLSOptions defaults to server.https on ${name}`, () => {
        assert.throws(
            () => buildTLSOptions({ ...VALID, ...cfg }),
            (err) => err.message.startsWith(`server.https.${field}`),
        );
    });
}
