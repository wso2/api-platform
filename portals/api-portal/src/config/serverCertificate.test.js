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
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { readServerCertificate, readServerCertificateOrExit } = require('./serverCertificate');

/** Builds the slice of config readServerCertificate reads. */
function configWith({ enabled, designMode = false, certFile = 'missing-cert.pem', keyFile = 'missing-key.pem' }) {
    return { server: { https: { enabled, certFile, keyFile } }, designMode: { enabled: designMode } };
}

test('readServerCertificate returns null when HTTPS is off', () => {
    assert.equal(readServerCertificate(configWith({ enabled: false })), null);
});

test('readServerCertificate returns null in design mode, even with HTTPS on', () => {
    assert.equal(readServerCertificate(configWith({ enabled: true, designMode: true })), null);
});

test('readServerCertificate reads the configured certificate and key', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'server-cert-'));
    try {
        const certFile = path.join(dir, 'cert.pem');
        const keyFile = path.join(dir, 'key.pem');
        fs.writeFileSync(certFile, 'CERT');
        fs.writeFileSync(keyFile, 'KEY');
        const result = readServerCertificate(configWith({ enabled: true, certFile, keyFile }));
        assert.equal(result.cert.toString(), 'CERT');
        assert.equal(result.key.toString(), 'KEY');
    } finally {
        fs.rmSync(dir, { recursive: true, force: true });
    }
});

test('readServerCertificate throws a clear error naming both paths when a file is missing', () => {
    assert.throws(
        () => readServerCertificate(configWith({ enabled: true, certFile: '/nonexistent/cert.pem', keyFile: '/nonexistent/key.pem' })),
        (err) => {
            assert.match(err.message, /TLS certificate or key could not be read/);
            assert.equal(err.details.certFile, '/nonexistent/cert.pem');
            assert.equal(err.details.keyFile, '/nonexistent/key.pem');
            return true;
        }
    );
});

test('readServerCertificateOrExit logs the reason and exits with 1 when a file is missing', () => {
    const logged = [];
    const logger = { error: (message, meta) => logged.push({ message, meta }) };
    let exitCode;
    const result = readServerCertificateOrExit(
        configWith({ enabled: true, certFile: '/nonexistent/cert.pem', keyFile: '/nonexistent/key.pem' }),
        logger,
        (code) => { exitCode = code; return 'exited'; }
    );
    assert.equal(exitCode, 1);
    assert.equal(result, 'exited');
    assert.equal(logged.length, 1);
    assert.match(logged[0].message, /TLS certificate or key could not be read/);
    assert.equal(logged[0].meta.certFile, '/nonexistent/cert.pem');
    assert.equal(logged[0].meta.operation, 'httpsServerSetup');
});

test('readServerCertificateOrExit returns the result and does not exit when nothing is wrong', () => {
    let exited = false;
    const result = readServerCertificateOrExit(configWith({ enabled: false }), { error() {} }, () => { exited = true; });
    assert.equal(result, null);
    assert.equal(exited, false);
});
