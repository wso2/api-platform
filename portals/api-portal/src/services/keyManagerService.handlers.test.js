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
 * End-to-end tests for the key manager handlers, against a real database.
 *
 * The sibling file covers the validators in isolation. This one drives the
 * request handlers, because the rules that matter most here are about state
 * rather than payload shape: a key manager's environment and driver type are
 * fixed once it exists, its handle cannot move while keys reference it, and it
 * cannot be deleted out from under them. None of that is reachable without a
 * database holding the prior state.
 *
 * Same harness as oauth2KeyService.handlers.test.js: a throwaway sqlite file
 * built from the shipped schema, driven through one child process so that
 * later calls see what earlier ones did. Each test below asserts one step of
 * that run, so a failure names the behaviour rather than the batch.
 *
 * No key manager here needs a driver that dials: every operation under test is
 * the portal's own bookkeeping.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const SERVICE_MODULE = path.join(__dirname, 'keyManagerService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');
const SQLITE = path.join(PROJECT_ROOT, 'node_modules', 'better-sqlite3');

const ORG = 'org-uuid-1';
const ADMIN = 'admin-1';

let tmpDir;
test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const CONFIG = (dbFile) => `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(dbFile)}

# A config-declared key manager, so the "declared in configuration" branches have
# something real to collide with: those entries are not editable through the API.
[[api_portal.key_manager]]
id   = "from-config"
name = "Config KM"
type = "provision"
token_endpoint        = "https://idp.example.com/oauth2/token"
registration_endpoint = "https://idp.example.com/oauth2/register"

  [api_portal.key_manager.auth]
  method        = "client_credentials"
  client_id     = "inert"
  client_secret = "inert"
`;

const PROBE = `
const svc = require(${JSON.stringify(SERVICE_MODULE)});
const out = {};

function call(fn, req) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200,
            status(c) { this.statusCode = c; return this; },
            json(b) { resolve({ status: this.statusCode, body: b }); return this; },
            send(b) { resolve({ status: this.statusCode, body: b === undefined ? null : b }); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.message) }));
    });
}

const as = (extra) => Object.assign(
    { orgId: ${JSON.stringify(ORG)}, user: { sub: ${JSON.stringify(ADMIN)} }, query: {}, params: {}, body: {} },
    extra || {}
);

const base = {
    displayName: 'Primary KM',
    tokenEndpoint: 'https://idp.example.com/oauth2/token',
};

(async () => {
    // --- create -----------------------------------------------------------
    out.created = await call(svc.createKeyManager, as({ body: { ...base, id: 'primary' } }));
    out.createdSandbox = await call(svc.createKeyManager, as({
        body: { ...base, id: 'sandbox-km', displayName: 'Sandbox KM', keyType: 'SANDBOX' },
    }));
    out.duplicate = await call(svc.createKeyManager, as({ body: { ...base, id: 'primary' } }));
    out.badKeyType = await call(svc.createKeyManager, as({
        body: { ...base, id: 'bad-type', keyType: 'STAGING' },
    }));
    out.shadowsConfig = await call(svc.createKeyManager, as({
        body: { ...base, id: 'from-config', displayName: 'Shadow' },
    }));

    // --- read -------------------------------------------------------------
    out.list = await call(svc.getKeyManagers, as());
    out.get = await call(svc.getKeyManager, as({ params: { kmId: 'primary' } }));
    out.getMissing = await call(svc.getKeyManager, as({ params: { kmId: 'no-such-km' } }));

    // --- update -----------------------------------------------------------
    out.renamed = await call(svc.updateKeyManager, as({
        params: { kmId: 'primary' }, body: { ...base, displayName: 'Renamed KM' },
    }));
    out.afterRename = await call(svc.getKeyManager, as({ params: { kmId: 'primary' } }));

    out.keyTypeChange = await call(svc.updateKeyManager, as({
        params: { kmId: 'primary' }, body: { ...base, displayName: 'Renamed KM', keyType: 'SANDBOX' },
    }));
    out.keyTypeUnchanged = await call(svc.updateKeyManager, as({
        params: { kmId: 'primary' }, body: { ...base, displayName: 'Renamed KM', keyType: 'PRODUCTION' },
    }));

    out.updateConfigDeclared = await call(svc.updateKeyManager, as({
        params: { kmId: 'from-config' }, body: { ...base, displayName: 'nope' },
    }));
    out.updateMissing = await call(svc.updateKeyManager, as({
        params: { kmId: 'no-such-km' }, body: { ...base, displayName: 'nope' },
    }));

    // --- delete, with and without keys referencing it ----------------------
    out.deleteConfigDeclared = await call(svc.deleteKeyManager, as({ params: { kmId: 'from-config' } }));
    out.deleteMissing = await call(svc.deleteKeyManager, as({ params: { kmId: 'no-such-km' } }));
    out.deleteSandbox = await call(svc.deleteKeyManager, as({ params: { kmId: 'sandbox-km' } }));
    out.getAfterDelete = await call(svc.getKeyManager, as({ params: { kmId: 'sandbox-km' } }));

    // A key recorded against "primary" blocks both its rename and its deletion.
    require(${JSON.stringify(SQLITE)})(${JSON.stringify('DB_FILE')}).prepare(
        \`INSERT INTO oauth2_consumer_keys
         (uuid, org_uuid, portal_id, key_manager_id, consumer_key, name, key_type, status, created_by, updated_by)
         VALUES (?,?,?,?,?,?,?,?,?,?)\`
    ).run('key-1', ${JSON.stringify(ORG)}, 'test-portal', 'primary', 'ck-1', 'a key',
          'PRODUCTION', 'ACTIVE', 'dev', 'dev');

    out.renameWithKeys = await call(svc.updateKeyManager, as({
        params: { kmId: 'primary' }, body: { ...base, id: 'primary-renamed', displayName: 'Renamed KM' },
    }));
    out.nameOnlyWithKeys = await call(svc.updateKeyManager, as({
        params: { kmId: 'primary' }, body: { ...base, displayName: 'Still Editable' },
    }));
    out.deleteWithKeys = await call(svc.deleteKeyManager, as({ params: { kmId: 'primary' } }));

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-kmh-'));
    const dbFile = path.join(tmpDir, 'portal.db');

    const seed = path.join(tmpDir, 'seed.js');
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require(${JSON.stringify(SQLITE)});
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        db.prepare(\`INSERT INTO organizations
            (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(ORG)}, 'test-portal', 'Default', 'default', 'idp-1', '{}', 'test', 'test');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE.replace(JSON.stringify('DB_FILE'), JSON.stringify(dbFile)));

    const run = spawnSync(process.execPath, [runner, '--config', cfgFile], {
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

test('createKeyManager stores one and answers 201', () => {
    const r = results().created;
    assert.equal(r.status, 201);
    assert.equal(r.body.id, 'primary');
    assert.equal(r.body.displayName, 'Primary KM');
});

test('createKeyManager records the environment it was created for', () => {
    assert.equal(results().created.body.keyType, 'PRODUCTION');
    assert.equal(results().createdSandbox.status, 201);
    assert.equal(results().createdSandbox.body.keyType, 'SANDBOX');
});

test('createKeyManager refuses a duplicate id with 409', () => {
    // The id is how a caller selects a key manager, so a second one under the
    // same id is a conflict rather than an update.
    assert.equal(results().duplicate.status, 409);
});

test('createKeyManager refuses an unknown environment with 400', () => {
    assert.equal(results().badKeyType.status, 400);
});

test('createKeyManager refuses an id already declared in configuration', () => {
    /*
     * A stored row on a config-declared handle would disappear behind the config
     * entry on every later read — created successfully and then invisible, which
     * is worse than being refused.
     */
    assert.equal(results().shadowsConfig.status, 409);
});

// ---------------------------------------------------------------------------
// read
// ---------------------------------------------------------------------------

test('getKeyManagers lists stored and config-declared key managers together', () => {
    const r = results().list;
    assert.equal(r.status, 200);
    const ids = (r.body.list || r.body).map((k) => k.id);
    assert.ok(ids.includes('primary'), 'the stored one is listed');
    assert.ok(ids.includes('from-config'), 'the config-declared one is listed beside it');
});

test('getKeyManager returns one by its handle', () => {
    assert.equal(results().get.status, 200);
    assert.equal(results().get.body.id, 'primary');
});

test('getKeyManager answers 404 for an unknown handle', () => {
    assert.equal(results().getMissing.status, 404);
});

// ---------------------------------------------------------------------------
// update
// ---------------------------------------------------------------------------

test('updateKeyManager renames a key manager that has no keys', () => {
    assert.equal(results().renamed.status, 200);
    assert.equal(results().afterRename.body.displayName, 'Renamed KM');
});

test('updateKeyManager refuses to change the environment, with 409', () => {
    /*
     * The keys already issued were recorded as one kind; relabelling the key
     * manager cannot change what they are. Refused as a conflict rather than a
     * 400 — the request is well formed, the current state is incompatible.
     */
    assert.equal(results().keyTypeChange.status, 409);
});

test('updateKeyManager accepts an unchanged environment in the payload', () => {
    // A read-modify-write round trip sends the whole object back. Only an actual
    // change is refused, or editing the display name would mean stripping fields.
    assert.equal(results().keyTypeUnchanged.status, 200);
});

test('updateKeyManager refuses to edit a config-declared key manager', () => {
    assert.equal(results().updateConfigDeclared.status, 409);
});

test('updateKeyManager answers 404 for an unknown handle', () => {
    assert.equal(results().updateMissing.status, 404);
});

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

test('deleteKeyManager removes one that nothing references', () => {
    assert.equal(results().deleteSandbox.status, 204);
    assert.equal(results().getAfterDelete.status, 404);
});

test('deleteKeyManager refuses a config-declared key manager', () => {
    assert.equal(results().deleteConfigDeclared.status, 409);
});

test('deleteKeyManager answers 404 for an unknown handle', () => {
    assert.equal(results().deleteMissing.status, 404);
});

// ---------------------------------------------------------------------------
// The guards that need keys to exist
// ---------------------------------------------------------------------------

test('a key manager with keys cannot be renamed', () => {
    /*
     * `oauth2_consumer_keys.key_manager_id` stores the handle as plain text with
     * no foreign key, so a rename does not cascade: every existing key would be
     * left pointing at an id that resolves to nothing.
     */
    const r = results().renameWithKeys;
    assert.equal(r.status, 409);
    assert.match(JSON.stringify(r.body), /key/i);
});

test('a key manager with keys can still be edited in every other way', () => {
    // Only the id is frozen. Refusing the whole update would make a key manager
    // uneditable the moment a developer created a key on it.
    assert.equal(results().nameOnlyWithKeys.status, 200);
});

test('a key manager with keys cannot be deleted', () => {
    // Deleting it would strand the keys and the clients they stand for at the
    // identity server, reachable by nothing.
    const r = results().deleteWithKeys;
    assert.equal(r.status, 409);
    assert.match(JSON.stringify(r.body), /still has 1 key/);
});
