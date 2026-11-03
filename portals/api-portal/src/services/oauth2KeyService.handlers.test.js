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

/*
 * End-to-end tests for the OAuth2 key handlers, against a real database.
 *
 * The companion file (oauth2KeyService.test.js) covers the two pure functions.
 * This one drives the request handlers themselves — create, list, get, update,
 * delete, associate — because that is where the ownership scoping, the key-type
 * inheritance and the error mapping live, and none of it is reachable without a
 * database and a driver behind it.
 *
 * Nothing is mocked. A throwaway sqlite file is built from the shipped schema,
 * and the key manager is a config-declared `provision` entry: that driver
 * registers nothing and dials nothing — it records a client id the developer
 * already has — so every path below runs the real code with no network and no
 * identity server. A stubbed driver would have proved only that the stub was
 * called; this proves the handlers agree with a driver the product ships.
 *
 * One child process runs the whole scenario list in order and emits the results,
 * because the handlers share one database and the interesting assertions are
 * about what a later call sees after an earlier one — a key that is gone after a
 * delete, a key another user cannot read. Each test below asserts one step of
 * that run, so a failure still names the behaviour rather than the batch.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const SERVICE_MODULE = path.join(__dirname, 'oauth2KeyService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');
const SQLITE = path.join(PROJECT_ROOT, 'node_modules', 'better-sqlite3');

const ORG = 'org-uuid-1';
const OWNER = 'dev-1';
const OTHER = 'dev-2';
const APP = 'app-uuid-1';
// The association endpoint resolves the application by HANDLE, not uuid -- it
// calls applicationDao.getId(orgId, actor, applicationId) -- so this is what the
// request body carries.
const APP_HANDLE = 'my-app';

let tmpDir;
function tmp() {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-okh-'));
    return tmpDir;
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

/*
 * Two key managers, both `provision`. The second is declared SANDBOX so the
 * key-type inheritance has something to inherit that is not the default — a
 * value equal to the default would pass whether it was copied or not.
 */
function configFor(dbFile) {
    return `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(dbFile)}

[[api_portal.key_manager]]
id   = "offline-km"
name = "Offline KM"
type = "provision"
token_endpoint        = "https://idp.example.com/oauth2/token"
registration_endpoint = "https://idp.example.com/oauth2/register"

  [api_portal.key_manager.auth]
  method        = "client_credentials"
  client_id     = "inert-client"
  client_secret = "inert-secret"

[[api_portal.key_manager]]
id   = "sandbox-km"
name = "Sandbox KM"
type = "provision"
key_type = "SANDBOX"
token_endpoint        = "https://idp.example.com/oauth2/token"
registration_endpoint = "https://idp.example.com/oauth2/register"

  [api_portal.key_manager.auth]
  method        = "client_credentials"
  client_id     = "inert-client"
  client_secret = "inert-secret"
`;
}

const PROBE = `
const svc = require(${JSON.stringify(SERVICE_MODULE)});
const out = {};

function call(fn, req) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200, headers: {},
            status(c) { this.statusCode = c; return this; },
            setHeader(k, v) { this.headers[k] = v; },
            json(b) { resolve({ status: this.statusCode, body: b, headers: this.headers }); return this; },
            send(b) { resolve({ status: this.statusCode, body: b === undefined ? null : b, headers: this.headers }); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.message) }));
    });
}

const as = (user, extra) => Object.assign(
    { orgId: ${JSON.stringify(ORG)}, user: { sub: user }, baseUrl: '/api', path: '/oauth2-keys', query: {}, params: {}, body: {} },
    extra || {}
);

(async () => {
    // --- create -----------------------------------------------------------
    out.created = await call(svc.createOAuth2Key, as(${JSON.stringify(OWNER)}, {
        body: { keyManagerId: 'offline-km', properties: { client_name: 'primary', consumerKey: 'ck-primary' } },
    }));
    const keyId = out.created.body && out.created.body.keyId;

    out.createdSandbox = await call(svc.createOAuth2Key, as(${JSON.stringify(OWNER)}, {
        body: { keyManagerId: 'sandbox-km', properties: { client_name: 'sandbox', consumerKey: 'ck-sandbox' } },
    }));

    out.unknownKm = await call(svc.createOAuth2Key, as(${JSON.stringify(OWNER)}, {
        body: { keyManagerId: 'no-such-km', properties: { client_name: 'x', consumerKey: 'y' } },
    }));

    out.missingRequired = await call(svc.createOAuth2Key, as(${JSON.stringify(OWNER)}, {
        body: { keyManagerId: 'offline-km', properties: { client_name: 'no consumer key' } },
    }));

    out.undeclaredProp = await call(svc.createOAuth2Key, as(${JSON.stringify(OWNER)}, {
        body: { keyManagerId: 'offline-km', properties: { client_name: 'x', consumerKey: 'y', bogus: 1 } },
    }));

    // --- read -------------------------------------------------------------
    out.list = await call(svc.listOAuth2Keys, as(${JSON.stringify(OWNER)}));
    out.listOther = await call(svc.listOAuth2Keys, as(${JSON.stringify(OTHER)}));
    out.listFiltered = await call(svc.listOAuth2Keys, as(${JSON.stringify(OWNER)}, { query: { keyManagerId: 'sandbox-km' } }));

    out.get = await call(svc.getOAuth2Key, as(${JSON.stringify(OWNER)}, { params: { keyId } }));
    out.getOther = await call(svc.getOAuth2Key, as(${JSON.stringify(OTHER)}, { params: { keyId } }));
    out.getMissing = await call(svc.getOAuth2Key, as(${JSON.stringify(OWNER)}, { params: { keyId: 'no-such-key' } }));

    // --- update: this driver implements no update --------------------------
    out.update = await call(svc.updateOAuth2Key, as(${JSON.stringify(OWNER)}, {
        params: { keyId }, body: { properties: { client_name: 'renamed', consumerKey: 'ck-primary' } },
    }));

    // --- association -------------------------------------------------------
    out.associate = await call(svc.associateOAuth2KeyApplication, as(${JSON.stringify(OWNER)}, {
        params: { keyId }, body: { applicationId: ${JSON.stringify(APP_HANDLE)} },
    }));
    out.listAfterAssociate = await call(svc.listOAuth2Keys, as(${JSON.stringify(OWNER)}));
    out.dissociate = await call(svc.dissociateOAuth2KeyApplication, as(${JSON.stringify(OWNER)}, {
        params: { keyId },
    }));
    out.listAfterDissociate = await call(svc.listOAuth2Keys, as(${JSON.stringify(OWNER)}));

    // --- delete ------------------------------------------------------------
    out.deleteOther = await call(svc.deleteOAuth2Key, as(${JSON.stringify(OTHER)}, { params: { keyId } }));
    out.delete = await call(svc.deleteOAuth2Key, as(${JSON.stringify(OWNER)}, { params: { keyId } }));
    out.getAfterDelete = await call(svc.getOAuth2Key, as(${JSON.stringify(OWNER)}, { params: { keyId } }));
    out.deleteAgain = await call(svc.deleteOAuth2Key, as(${JSON.stringify(OWNER)}, { params: { keyId } }));

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

/** Builds the database and runs every scenario once; memoised for the file. */
let _results;
function results() {
    if (_results) return _results;
    const dir = tmp();
    const dbFile = path.join(dir, 'portal.db');

    const seed = path.join(dir, 'seed.js');
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require(${JSON.stringify(SQLITE)});
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        db.prepare(\`INSERT INTO organizations
            (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(ORG)}, 'test-portal', 'Default', 'default', 'idp-1', '{}', 'test', 'test');
        db.prepare(\`INSERT INTO applications
            (uuid, org_uuid, portal_id, created_by, display_name, handle, description, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(APP)}, ${JSON.stringify(ORG)}, 'test-portal', ${JSON.stringify(OWNER)},
                 'My App', 'my-app', 'for the association test', ${JSON.stringify(OWNER)});
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding the database failed: ${seeded.stderr}`);

    const cfg = path.join(dir, 'config.toml');
    fs.writeFileSync(cfg, configFor(dbFile));
    const runner = path.join(dir, 'probe.js');
    fs.writeFileSync(runner, PROBE);

    const run = spawnSync(process.execPath, [runner, '--config', cfg], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(run.status, 0, `probe failed: ${run.stderr}`);
    const line = run.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${run.stdout}\n${run.stderr}`);
    _results = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results;
}

// ---------------------------------------------------------------------------
// create
// ---------------------------------------------------------------------------

test('createOAuth2Key records the client and answers 201 with its identity', () => {
    const r = results().created;
    assert.equal(r.status, 201);
    assert.ok(r.body.keyId, 'a key id is assigned by the portal');
    assert.equal(r.body.consumerKey, 'ck-primary');
    assert.equal(r.body.name, 'primary');
    assert.equal(r.body.keyManagerId, 'offline-km');
});

test('createOAuth2Key returns a Location header pointing at the new key', () => {
    const r = results().created;
    assert.ok(r.headers.Location, 'Location header present');
    assert.ok(r.headers.Location.endsWith(encodeURIComponent(r.body.keyId)));
});

test('createOAuth2Key issues no secret for a client the portal did not register', () => {
    // The provision driver records a client id that already exists elsewhere, so
    // there is no secret to hand back — and none to store. An empty string here
    // rather than a placeholder is what tells the UI there is nothing to reveal.
    assert.equal(results().created.body.consumerSecret, '');
});

test('createOAuth2Key stamps the key manager key type onto the key', () => {
    // The environment is a fact about the credential, copied at creation rather
    // than read back through the key manager, so a key manager's own flag
    // changing later cannot relabel keys already issued.
    assert.equal(results().created.body.keyType, 'PRODUCTION');
    assert.equal(results().createdSandbox.status, 201);
    assert.equal(results().createdSandbox.body.keyType, 'SANDBOX');
});

test('createOAuth2Key answers 404 for an unknown key manager without echoing the id', () => {
    const r = results().unknownKm;
    assert.equal(r.status, 404);
    // The submitted id is never reflected: a rejected id must not become a way to
    // probe which key managers exist.
    assert.ok(!JSON.stringify(r.body).includes('no-such-km'), 'the submitted id is not echoed back');
});

test('createOAuth2Key rejects a submission missing a required property', () => {
    const r = results().missingRequired;
    assert.equal(r.status, 400);
    assert.match(JSON.stringify(r.body), /consumerKey/);
});

test('createOAuth2Key rejects a property the key manager does not declare', () => {
    const r = results().undeclaredProp;
    assert.equal(r.status, 400);
    assert.match(JSON.stringify(r.body), /bogus/);
});

// ---------------------------------------------------------------------------
// list and get — ownership
// ---------------------------------------------------------------------------

test('listOAuth2Keys returns the caller own keys', () => {
    const r = results().list;
    assert.equal(r.status, 200);
    assert.equal(r.body.count, 2, 'both keys created by this developer');
    assert.deepEqual(r.body.list.map((k) => k.consumerKey).sort(), ['ck-primary', 'ck-sandbox']);
});

test('listOAuth2Keys shows another developer none of them', () => {
    // Ownership is applied in SQL rather than filtered afterwards, so another
    // user's row is never loaded into memory in the first place.
    const r = results().listOther;
    assert.equal(r.status, 200);
    assert.equal(r.body.count, 0);
});

test('listOAuth2Keys filters by key manager when asked', () => {
    const r = results().listFiltered;
    assert.equal(r.status, 200);
    assert.equal(r.body.count, 1);
    assert.equal(r.body.list[0].keyManagerId, 'sandbox-km');
});

test('getOAuth2Key returns one key to its creator', () => {
    const r = results().get;
    assert.equal(r.status, 200);
    assert.equal(r.body.consumerKey, 'ck-primary');
});

test('getOAuth2Key answers 404 for another developer key, not 403', () => {
    /*
     * 404 rather than 403 on purpose: 403 would confirm the key exists, which
     * turns a guessed id into an existence oracle. The caller cannot tell "not
     * yours" from "no such key", which is the same answer they get for both.
     */
    assert.equal(results().getOther.status, 404);
});

test('getOAuth2Key answers 404 for an unknown key id', () => {
    assert.equal(results().getMissing.status, 404);
});

// ---------------------------------------------------------------------------
// update
// ---------------------------------------------------------------------------

test('updateOAuth2Key answers 409 where the driver implements no update', () => {
    // The provision driver manages nothing at the key manager, so there is
    // nothing to update there. 409 rather than 404 or 501: the key exists and
    // the caller may see it; this key manager simply does not offer the operation.
    assert.equal(results().update.status, 409);
});

// ---------------------------------------------------------------------------
// association
// ---------------------------------------------------------------------------

test('associateOAuth2KeyApplication attaches a key to an application', () => {
    const r = results().associate;
    assert.equal(r.status, 200);
    // The response carries the application and not the key: the caller already
    // holds the key, and re-sending it would invite treating this as a key read.
    assert.ok(r.body.application, 'the association response names the application');
    assert.ok(!('consumerKey' in r.body), 'and does not re-send the key');
});

test('an associated key reports its application in the listing', () => {
    const row = results().listAfterAssociate.body.list.find((k) => k.consumerKey === 'ck-primary');
    assert.ok(row, 'the key is still listed');
    assert.match(JSON.stringify(row), /My App/, 'the listing carries the association');
});

test('dissociateOAuth2KeyApplication detaches it again', () => {
    const r = results().dissociate;
    assert.ok([200, 204].includes(r.status), `expected 200/204, got ${r.status}`);
    const row = results().listAfterDissociate.body.list.find((k) => k.consumerKey === 'ck-primary');
    assert.ok(row, 'the key itself survives the dissociation');
    assert.doesNotMatch(JSON.stringify(row), /My App/, 'the association is gone');
});

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

test('deleteOAuth2Key refuses another developer key', () => {
    assert.equal(results().deleteOther.status, 404);
});

test('deleteOAuth2Key removes the key and answers 204', () => {
    assert.equal(results().delete.status, 204);
});

test('a deleted key is gone from the caller view', () => {
    assert.equal(results().getAfterDelete.status, 404);
});

test('deleting an already-deleted key answers 404 rather than throwing', () => {
    assert.equal(results().deleteAgain.status, 404);
});
