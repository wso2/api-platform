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
const os = require('node:os');
const path = require('node:path');

const roleScopeMap = require('./roleScopeMap');

const SPEC_PATH = path.join(__dirname, '..', '..', 'docs', 'api-portal-openapi-spec-v0.9.yaml');
const SHIPPED_MAPPING_PATH = path.join(__dirname, '..', '..', 'resources', 'role-to-scope-mapping.yaml');

let tmpDir;
function writeFixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-role-scope-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

// ---------------------------------------------------------------------------
// Scope well-formedness (foreign namespaces)
// ---------------------------------------------------------------------------

test('isWellFormedScope accepts a trailing wildcard but not a non-trailing one', () => {
    assert.equal(roleScopeMap.isWellFormedScope('ap:gateway:*'), true);
    assert.equal(roleScopeMap.isWellFormedScope('ap:*'), true);
    // A wildcard that isn't the last segment would imply prefix/transitive matching,
    // which no component implements.
    assert.equal(roleScopeMap.isWellFormedScope('ap:*:read'), false);
    // ...and a wildcard glued into a segment is not a wildcard at all.
    assert.equal(roleScopeMap.isWellFormedScope('ap:gate*way:read'), false);
});

test('isWellFormedScope allows hyphens, since a foreign namespace picks its own convention', () => {
    assert.equal(roleScopeMap.isWellFormedScope('dp:api-key_read'), true);
    assert.equal(roleScopeMap.isWellFormedScope('ap:rest_api:deployment:manage'), true);
});

test('isWellFormedScope rejects a value that is not namespaced at all', () => {
    assert.equal(roleScopeMap.isWellFormedScope('notascope'), false);
    assert.equal(roleScopeMap.isWellFormedScope(''), false);
    assert.equal(roleScopeMap.isWellFormedScope('dp:'), false);
    assert.equal(roleScopeMap.isWellFormedScope(':read'), false);
});

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

test('parseRoleScopeMap reads roles into a map and collapses duplicate scopes within a role', () => {
    const map = roleScopeMap.parseRoleScopeMap(`
roles:
  - name: r1
    scopes:
      - dp:api:read
      - dp:api:read
      - dp:view:read
`, 'fixture');
    assert.deepEqual([...map.keys()], ['r1']);
    assert.deepEqual(map.get('r1'), ['dp:api:read', 'dp:view:read']);
});

test('parseRoleScopeMap rejects a duplicate role name rather than silently last-wins', () => {
    assert.throws(() => roleScopeMap.parseRoleScopeMap(`
roles:
  - name: dup
    scopes: [dp:api:read]
  - name: dup
    scopes: [dp:view:read]
`, 'fixture'), /declared more than once/);
});

test('parseRoleScopeMap rejects a file with no top-level roles list', () => {
    assert.throws(() => roleScopeMap.parseRoleScopeMap('something: else', 'fixture'),
        /must contain a top-level "roles" list/);
    assert.throws(() => roleScopeMap.parseRoleScopeMap('', 'fixture'),
        /must contain a top-level "roles" list/);
});

test('parseRoleScopeMap rejects an entry with no name or no scopes list', () => {
    assert.throws(() => roleScopeMap.parseRoleScopeMap('roles:\n  - scopes: [dp:api:read]\n', 'fixture'),
        /has no "name"/);
    assert.throws(() => roleScopeMap.parseRoleScopeMap('roles:\n  - name: r1\n', 'fixture'),
        /has no "scopes" list/);
});

// ---------------------------------------------------------------------------
// Namespace-scoped validation
// ---------------------------------------------------------------------------

test('validateRoleScopeMap rejects a dp: scope the OpenAPI spec does not declare', () => {
    const declared = new Set(['dp:api:read']);
    const map = new Map([['r1', ['dp:no_such_resource:manage']]]);
    assert.throws(() => roleScopeMap.validateRoleScopeMap(map, declared, 'fixture'),
        /does not declare/);
});

test('validateRoleScopeMap accepts a well-formed scope in another component namespace', () => {
    // This portal mints ap:* into nothing and enforces none, so it can neither confirm
    // nor deny their existence — shape is all it may check.
    const declared = new Set(['dp:api:read']);
    const map = new Map([['r1', ['dp:api:read', 'ap:rest_api:manage', 'ap:gateway:*']]]);
    assert.doesNotThrow(() => roleScopeMap.validateRoleScopeMap(map, declared, 'fixture'));
});

test('validateRoleScopeMap reports every problem at once', () => {
    const declared = new Set(['dp:api:read']);
    const map = new Map([['r1', ['dp:nope:read', 'ap:*:read', 'notascope']]]);
    assert.throws(() => roleScopeMap.validateRoleScopeMap(map, declared, 'fixture'), (err) => {
        assert.match(err.message, /dp:nope:read/);
        assert.match(err.message, /ap:\*:read/);
        assert.match(err.message, /notascope/);
        return true;
    });
});

test('validateRoleScopeMap rejects a role that grants nothing', () => {
    assert.throws(() => roleScopeMap.validateRoleScopeMap(new Map([['r1', []]]), new Set(['dp:api:read']), 'fixture'),
        /grants no scopes/);
});

// ---------------------------------------------------------------------------
// File access
// ---------------------------------------------------------------------------

test('loadRoleScopeMap refuses a path containing a traversal sequence or a null byte', () => {
    assert.throws(() => roleScopeMap.loadRoleScopeMap('/etc/api-portal/../../etc/passwd', SPEC_PATH),
        /traversal/);
    assert.throws(() => roleScopeMap.loadRoleScopeMap('/etc/api-portal/x\0.yaml', SPEC_PATH),
        /not a usable file path/);
});

test('loadRoleScopeMap reports an unreadable path without leaking why', () => {
    const missing = path.join(os.tmpdir(), 'ap-role-scope-does-not-exist.yaml');
    assert.throws(() => roleScopeMap.loadRoleScopeMap(missing, SPEC_PATH), /could not be read/);
});

// ---------------------------------------------------------------------------
// Declared-scope extraction
// ---------------------------------------------------------------------------

test('readDeclaredPortalScopes pulls dp:* scopes out of the shipped spec', () => {
    const declared = roleScopeMap.readDeclaredPortalScopes(SPEC_PATH);
    assert.ok(declared.size > 50, `expected the spec to declare many scopes, got ${declared.size}`);
    assert.ok(declared.has('dp:api:read'));
    assert.ok(declared.has('dp:application:manage'));
    assert.ok(!declared.has('dp:not_a_real_scope:read'));
});

// ---------------------------------------------------------------------------
// The shipped sample — the counterpart of platform-api's
// TestShippedSampleRolesValidateAgainstShippedSpec, so a pack cannot ship a
// grant table that fails startup.
// ---------------------------------------------------------------------------

test('the shipped role-to-scope-mapping.yaml validates against the shipped OpenAPI spec', () => {
    const map = roleScopeMap.loadRoleScopeMap(SHIPPED_MAPPING_PATH, SPEC_PATH);
    // Two grants by design — the portal recognises an administrator and a consumer,
    // which is exactly what its page gate has tiers for — plus aliases for the role
    // names other components mint, and a service identity used by Platform API for
    // outbound publish calls. The publisher/operator/viewer personas belong to
    // platform-api's own grant table.
    assert.deepEqual(
        [...map.keys()],
        ['dp_admin', 'dp_subscriber', 'ap_admin', 'ap_subscriber', 'platform-api-system'],
    );
});

test('the shipped platform-api-system role grants exactly the five publishing scopes', () => {
    // Pinned scope list, not just presence: this role is granted to Platform API's
    // outbound publish caller, so silently widening it (accidentally adding
    // application/subscription scopes, say) would hand a service identity powers
    // meant for a human admin. Silently narrowing it would leave publishing
    // broken for whichever resource lost its scope, which the role-name-only
    // assertion above would miss.
    const map = roleScopeMap.loadRoleScopeMap(SHIPPED_MAPPING_PATH, SPEC_PATH);
    assert.deepEqual(map.get('platform-api-system'), [
        'dp:api:manage',
        'dp:api_content:manage',
        'dp:mcp_server:manage',
        'dp:mcp_server_content:manage',
        'dp:subscription_plan:manage',
    ]);
});

test('the shipped admin role covers every resource the shipped subscriber role touches', () => {
    // A narrower admin than consumer would be a packaging mistake, not a policy: an
    // administrator who cannot see what a subscriber can manage is never intended.
    const map = roleScopeMap.loadRoleScopeMap(SHIPPED_MAPPING_PATH, SPEC_PATH);
    const resourcesOf = (role) => new Set(map.get(role).map((s) => s.split(':')[1]));
    const adminResources = resourcesOf('dp_admin');
    for (const resource of resourcesOf('dp_subscriber')) {
        assert.ok(adminResources.has(resource), `dp_admin has no scope for dp:${resource}:*`);
    }
});

// ---------------------------------------------------------------------------
// Expansion
// ---------------------------------------------------------------------------

test('expandRoles grants nothing before a grant table is loaded', () => {
    // Fresh module instance: an unloaded map must never read as "grant everything".
    const isolated = require.cache[require.resolve('./roleScopeMap')];
    delete require.cache[require.resolve('./roleScopeMap')];
    const fresh = require('./roleScopeMap');
    assert.equal(fresh.isLoaded(), false);
    assert.deepEqual(fresh.expandRoles(['dp_admin']), []);
    require.cache[require.resolve('./roleScopeMap')] = isolated;
});

test('expandRoles expands a single role, and unions across several without duplicates', () => {
    const file = writeFixture('union.yaml', `
roles:
  - name: reader
    scopes:
      - dp:api:read
      - dp:view:read
  - name: writer
    scopes:
      - dp:api:manage
      - dp:view:read
`);
    roleScopeMap.init(file, SPEC_PATH);

    assert.deepEqual(roleScopeMap.expandRoles(['reader']), ['dp:api:read', 'dp:view:read']);

    const both = roleScopeMap.expandRoles(['reader', 'writer']);
    // Union, most-permissive wins; dp:view:read is granted by both and appears once.
    assert.deepEqual(both.slice().sort(), ['dp:api:manage', 'dp:api:read', 'dp:view:read']);
    assert.equal(both.length, new Set(both).size);
});

test('expandRoles accepts a string claim as well as an array', () => {
    const file = writeFixture('string-claim.yaml', `
roles:
  - name: reader
    scopes: [dp:api:read]
  - name: writer
    scopes: [dp:api:manage]
`);
    roleScopeMap.init(file, SPEC_PATH);

    assert.deepEqual(roleScopeMap.expandRoles('reader writer').sort(), ['dp:api:manage', 'dp:api:read']);
    assert.deepEqual(roleScopeMap.expandRoles('reader,writer').sort(), ['dp:api:manage', 'dp:api:read']);
});

test('expandRoles grants nothing for an unknown role, an empty claim, or a blank entry', () => {
    const file = writeFixture('nothing.yaml', 'roles:\n  - name: reader\n    scopes: [dp:api:read]\n');
    roleScopeMap.init(file, SPEC_PATH);

    // An unknown role must contribute nothing rather than being treated as a scope
    // value of its own — the failure mode is a denied request, never a grant.
    assert.deepEqual(roleScopeMap.expandRoles(['no_such_role']), []);
    assert.deepEqual(roleScopeMap.expandRoles([]), []);
    assert.deepEqual(roleScopeMap.expandRoles(undefined), []);
    assert.deepEqual(roleScopeMap.expandRoles(''), []);
    assert.deepEqual(roleScopeMap.expandRoles(['', '  ', null]), []);
    // A known role alongside an unknown one still grants its own scopes.
    assert.deepEqual(roleScopeMap.expandRoles(['no_such_role', 'reader']), ['dp:api:read']);
});

test('the shipped aliases resolve to the same grant as the role they mirror', () => {
    // They are YAML anchors, so a grant is defined once; this pins that they stay
    // wired up rather than drifting into two hand-maintained lists.
    const map = roleScopeMap.loadRoleScopeMap(SHIPPED_MAPPING_PATH, SPEC_PATH);
    assert.deepEqual(map.get('ap_admin'), map.get('dp_admin'));
    assert.deepEqual(map.get('ap_subscriber'), map.get('dp_subscriber'));
});

test('role mode is usable by the shipped local-auth quickstart out of the box', () => {
    // platform-api's shipped config grants its admin roles = ["ap_admin"] and mints
    // that name into the roles claim. With role mode now the default, a missing alias
    // would mean login succeeds and every REST request is denied.
    roleScopeMap.init(SHIPPED_MAPPING_PATH, SPEC_PATH);
    const scopes = roleScopeMap.expandRoles(['ap_admin']);
    assert.ok(scopes.includes('dp:organization:manage'), 'ap_admin must reach admin scopes');
    assert.ok(scopes.includes('dp:api:manage'));
});
