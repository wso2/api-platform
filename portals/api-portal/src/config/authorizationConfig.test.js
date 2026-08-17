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
 * Startup validation for [api_portal.auth.authorization] (configLoader.js).
 *
 * Driven through a child process rather than by calling the validator directly: these
 * checks are fail-closed via process.exit, and configLoader runs them as a side effect
 * of module load. Spawning is what lets the test assert the thing that actually
 * matters — that the portal REFUSES TO START — rather than that a function returned an
 * error object.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const SHIPPED_MAPPING_PATH = path.join(PROJECT_ROOT, 'resources', 'role-to-scope-mapping.yaml');

// A config carrying only what the unrelated startup checks demand, so anything this
// suite observes comes from the authorization validation and nothing else.
const BASE_CONFIG = `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
`;

let tmpDir;
function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-authz-config-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

function loadConfig(overlayToml) {
    const base = fixture('base.toml', BASE_CONFIG);
    const args = ['--config', base];
    if (overlayToml !== undefined) {
        args.push('--config', fixture(`overlay-${Math.abs(hash(overlayToml))}.toml`, overlayToml));
    }
    const runner = fixture('runner.js', `
        const { config } = require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        // Marker-prefixed: dotenv writes a banner to stdout, so the JSON cannot be
        // the only thing there.
        process.stdout.write('\\nAUTHZ_JSON:' + JSON.stringify(config.auth.authorization) + '\\n');
    `);
    const result = spawnSync(process.execPath, [runner, ...args], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        // Keep the parent's environment out of it: config.toml-free fixtures must not
        // pick up an APIP_AP_* value that happens to be set in the developer's shell.
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    return { status: result.status, stdout: result.stdout, stderr: result.stderr };
}

/** Pulls the marked JSON line out of a child's stdout, ignoring any banners around it. */
function parseAuthz(stdout) {
    const line = String(stdout).split('\n').find(l => l.startsWith('AUTHZ_JSON:'));
    assert.ok(line, `no AUTHZ_JSON line in child stdout: ${stdout}`);
    return JSON.parse(line.slice('AUTHZ_JSON:'.length));
}

/**
 * Runs `body` inside a child that has loaded the given config, with `authz` bound to
 * middlewares/authorization.js, and returns whatever it hands to `emit`.
 *
 * That module reads config at call time and configLoader resolves it at module load, so
 * a spawned child is the only way to exercise it under a chosen config. It deliberately
 * pulls in only authorization.js — not authMiddleware.js, whose DAO chain would drag the
 * native database driver into a test that has nothing to do with the database.
 */
function inChild(overlayToml, body) {
    const base = fixture('base.toml', BASE_CONFIG);
    const overlay = fixture(`ov-${Math.abs(hash(overlayToml + body))}.toml`, overlayToml);
    const runner = fixture(`probe-${Math.abs(hash(body))}.js`, `
        const authz = require(${JSON.stringify(path.join(__dirname, '..', 'middlewares', 'authorization.js'))});
        const emit = (v) => process.stdout.write('\\nAUTHZ_JSON:' + JSON.stringify(v) + '\\n');
        ${body}
    `);
    const result = spawnSync(process.execPath, [runner, '--config', base, '--config', overlay], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(result.status, 0, result.stderr);
    return parseAuthz(result.stdout);
}

const ROLE_MODE_NESTED_CLAIM = `
[api_portal.auth.claim_mappings]
roles = "realm_access.roles"

[api_portal.auth.authorization]
mode = "role"
role_to_scope_mapping = ${JSON.stringify(SHIPPED_MAPPING_PATH)}
`;

// Stable per-content fixture name, so concurrent tests don't clobber each other's overlay.
function hash(s) {
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
    return h;
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

test('defaults resolve to enabled role-mode authorization backed by the shipped table', () => {
    const { status, stdout } = loadConfig();
    assert.equal(status, 0);
    const authz = parseAuthz(stdout);
    assert.equal(authz.enabled, true);
    // Role mode is the default, so the default mapping path must point at a file that
    // actually exists — the copy baked into the image / present in the project root.
    assert.equal(authz.mode, 'role');
    assert.equal(authz.roleToScopeMapping, './resources/role-to-scope-mapping.yaml');
    // Page gating stays off by default — the behaviour auth.role_validation had.
    assert.equal(authz.pageRoleValidation, false);
    assert.deepEqual(authz.portalRoles, { admin: 'admin', subscriber: 'Internal/subscriber' });
});

test('an unknown mode refuses to start', () => {
    const { status, stderr } = loadConfig('[api_portal.auth.authorization]\nmode = "scopes"\n');
    assert.equal(status, 1);
    assert.match(stderr, /auth\.authorization\.mode must be one of/);
});

test('an unknown mode refuses to start even while authorization is disabled', () => {
    // Rejected regardless of `enabled`, so a typo surfaces when it is written rather
    // than months later when enforcement is switched back on.
    const { status, stderr } = loadConfig(
        '[api_portal.auth.authorization]\nenabled = false\nmode = "rolls"\n');
    assert.equal(status, 1);
    assert.match(stderr, /auth\.authorization\.mode must be one of/);
});

test('role mode without a mapping file refuses to start', () => {
    // The default supplies a path, so this is the case where an operator blanks it out.
    const { status, stderr } = loadConfig(
        '[api_portal.auth.authorization]\nmode = "role"\nrole_to_scope_mapping = ""\n');
    assert.equal(status, 1);
    assert.match(stderr, /requires auth\.authorization\.role_to_scope_mapping/);
});

test('role mode without a roles claim mapping refuses to start', () => {
    const { status, stderr } = loadConfig(`
[api_portal.auth.claim_mappings]
roles = ""

[api_portal.auth.authorization]
mode = "role"
role_to_scope_mapping = ${JSON.stringify(SHIPPED_MAPPING_PATH)}
`);
    assert.equal(status, 1);
    assert.match(stderr, /requires auth\.claim_mappings\.roles/);
});

test('role mode with the shipped mapping starts and loads its five roles', () => {
    const { status, stderr } = loadConfig(`
[api_portal.auth.authorization]
mode = "role"
role_to_scope_mapping = ${JSON.stringify(SHIPPED_MAPPING_PATH)}
`);
    assert.equal(status, 0, stderr);
    assert.match(stderr, /loaded 5 role\(s\)/);
});

test('a mapping file is loaded and validated even in scope mode', () => {
    // Loaded whenever a path is set, so an operator finds out at the next restart that
    // their grant table is valid — not the first time a request is authorized with it.
    const bad = fixture('bad.yaml', 'roles:\n  - name: r1\n    scopes: [dp:not_a_real_scope:read]\n');
    const { status, stderr } = loadConfig(`
[api_portal.auth.authorization]
mode = "scope"
role_to_scope_mapping = ${JSON.stringify(bad)}
`);
    assert.equal(status, 1);
    assert.match(stderr, /does not declare/);
});

test('disabling authorization starts, but warns', () => {
    const { status, stderr } = loadConfig('[api_portal.auth.authorization]\nenabled = false\n');
    assert.equal(status, 0, stderr);
    assert.match(stderr, /\[WARN\].*enabled = false/);
});

// ---------------------------------------------------------------------------
// Retired keys (hard break)
// ---------------------------------------------------------------------------

test('the retired auth.role_validation key refuses to start, naming its replacement', () => {
    // A retired key is ignored, so the effective setting would be the DEFAULT rather
    // than what the file says — "role_validation = true" would read as "page gating on"
    // to the operator and mean "off" to the portal.
    const { status, stderr } = loadConfig('[api_portal.auth]\nrole_validation = true\n');
    assert.equal(status, 1);
    assert.match(stderr, /auth\.role_validation is retired/);
    assert.match(stderr, /auth\.authorization\.page_role_validation/);
    // The migration note matters: role_validation is NOT auth.authorization.enabled.
    assert.match(stderr, /auth\.authorization\.enabled/);
});

test('the retired auth.idp.roles key refuses to start, naming its replacement', () => {
    const { status, stderr } = loadConfig('[api_portal.auth.idp.roles]\nadmin = "admin"\n');
    assert.equal(status, 1);
    assert.match(stderr, /auth\.idp\.roles is retired/);
    assert.match(stderr, /auth\.authorization\.portal_roles/);
});

test('both retired keys are reported together, not one per run', () => {
    const { status, stderr } = loadConfig(`
[api_portal.auth]
role_validation = false

[api_portal.auth.idp.roles]
admin = "admin"
`);
    assert.equal(status, 1);
    assert.match(stderr, /auth\.role_validation is retired/);
    assert.match(stderr, /auth\.idp\.roles is retired/);
});

test('a retired key is rejected even when the new section is also present', () => {
    // Otherwise a half-migrated config would start and quietly enforce the new
    // section while the operator believes the old key still applies.
    const { status, stderr } = loadConfig(`
[api_portal.auth]
role_validation = true

[api_portal.auth.authorization]
page_role_validation = true
`);
    assert.equal(status, 1);
    assert.match(stderr, /auth\.role_validation is retired/);
});

test('the shipped config.toml resolves the authorization section from plain literals', () => {
    // The whole section is written as literals, matching how
    // [platform_api.auth.authorization] is written — deployment policy an operator edits
    // in place, not a per-environment value or a secret. The env map below is
    // deliberately bare: it proves nothing here depends on an APIP_AP_* variable, so a
    // reader of config.toml sees the effective settings without tracing indirection.
    //
    // Secrets in the shipped file come from container-only {{ file }} paths, so the base
    // fixture is layered on top to supply them; everything else is exercised as shipped.
    const base = fixture('base.toml', BASE_CONFIG);
    const runner = fixture('runner-shipped.js', `
        const { config } = require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        process.stdout.write('\\nAUTHZ_JSON:' + JSON.stringify(config.auth.authorization) + '\\n');
    `);
    const result = spawnSync(process.execPath,
        [runner, '--config', path.join(PROJECT_ROOT, 'configs', 'config.toml'), '--config', base],
        {
            cwd: PROJECT_ROOT,
            encoding: 'utf8',
            env: { PATH: process.env.PATH, HOME: process.env.HOME },
        });
    assert.equal(result.status, 0, result.stderr);
    const authz = parseAuthz(result.stdout);
    assert.equal(authz.enabled, true);
    assert.equal(authz.mode, 'role');
    // Relative on purpose: the one literal resolves against WORKDIR /app in the
    // container (where compose mounts the host copy over the baked one) and against the
    // project root for `npm run start:local`.
    assert.equal(authz.roleToScopeMapping, './resources/role-to-scope-mapping.yaml');
    assert.equal(authz.pageRoleValidation, true);
    assert.deepEqual(authz.portalRoles, { admin: 'ap_admin', subscriber: 'ap_subscriber' });
});

test('the shipped config.toml adds no APIP_AP_AUTH_AUTHORIZATION_* env indirection', () => {
    // Guards the convention itself: platform-api's config.toml reserves {{ env }} for
    // secrets and the log level, and this section follows suit. A token creeping back in
    // would mean the file no longer states its own effective policy.
    const shipped = fs.readFileSync(path.join(PROJECT_ROOT, 'configs', 'config.toml'), 'utf8');
    const section = shipped.slice(shipped.indexOf('[api_portal.auth.authorization]'));
    const upToNextTable = section.slice(0, section.indexOf('\n[api_portal.organization]'));
    assert.ok(!/\{\{\s*env/.test(upToNextTable),
        `the authorization section should use plain literals, found:\n${upToNextTable}`);
});

// ---------------------------------------------------------------------------
// effectiveScopes — the decision every credential path shares
// ---------------------------------------------------------------------------

test('scope mode passes the token scope claim through, as an array or a string', () => {
    const out = inChild('[api_portal.auth.authorization]\nmode = "scope"\n', `
        emit({
            fromString: authz.effectiveScopes('dp:api:read dp:view:read', {}),
            fromArray: authz.effectiveScopes(['dp:api:read'], {}),
            // A roles claim is irrelevant in scope mode — it must not leak in.
            rolesIgnored: authz.effectiveScopes('dp:api:read', { roles: ['dp_admin'] }),
        });
    `);
    assert.deepEqual(out.fromString, ['dp:api:read', 'dp:view:read']);
    assert.deepEqual(out.fromArray, ['dp:api:read']);
    assert.deepEqual(out.rolesIgnored, ['dp:api:read']);
});

test('role mode expands a nested roles claim and ignores the scope claim entirely', () => {
    const out = inChild(ROLE_MODE_NESTED_CLAIM, `
        // Keycloak's shape, reached via auth.claim_mappings.roles = "realm_access.roles".
        const token = { sub: 'alice', realm_access: { roles: ['dp_subscriber'] }, scope: 'openid profile' };
        emit(authz.effectiveScopes('openid profile', token));
    `);
    assert.ok(out.includes('dp:application:manage'), 'expected the role grant to be applied');
    assert.ok(out.includes('dp:api:read'));
    // Ignoring rather than merging the scope claim is what stops a caller widening a
    // role's grant by asking their IDP for extra scope values.
    assert.ok(!out.includes('openid'), 'the raw scope claim must not be merged in');
    assert.ok(!out.includes('dp:organization:manage'), 'dp_subscriber must not reach admin scopes');
});

test('role mode falls back to the flat roles key for a session profile', () => {
    // passportConfig stores a session's roles at the flat `roles` key even when the
    // configured claim path is nested, so the fallback is what keeps an IDP browser
    // session authorized rather than silently granted nothing.
    const out = inChild(ROLE_MODE_NESTED_CLAIM, `
        emit(authz.effectiveScopes('openid', { roles: ['dp_subscriber'], grantedScopes: 'openid' }));
    `);
    assert.ok(out.includes('dp:api:read'), 'the flat roles key was not read');
    assert.ok(out.includes('dp:application:manage'));
    assert.ok(!out.includes('dp:organization:manage'), 'dp_subscriber is not an administrator');
});

test('role mode grants nothing for a role absent from the grant table', () => {
    const out = inChild(ROLE_MODE_NESTED_CLAIM, `
        emit(authz.effectiveScopes('dp:organization:manage', { realm_access: { roles: ['some_other_idp_group'] } }));
    `);
    // Not even the scope claim survives — an unmapped role is a denied request.
    assert.deepEqual(out, []);
});

test('portalRoles and the two switches read from the authorization section', () => {
    const out = inChild(`
[api_portal.auth.authorization]
enabled = false
page_role_validation = true

[api_portal.auth.authorization.portal_roles]
admin = "dp_admin"
`, `
        emit({
            enabled: authz.isAuthorizationEnabled(),
            pageRoleValidation: authz.isPageRoleValidationEnabled(),
            roles: authz.portalRoles(),
        });
    `);
    assert.equal(out.enabled, false);
    assert.equal(out.pageRoleValidation, true);
    assert.equal(out.roles.admin, 'dp_admin');
    // The unset tier keeps its default rather than becoming undefined.
    assert.equal(out.roles.subscriber, 'Internal/subscriber');
    // There is no superAdmin tier any more.
    assert.equal(out.roles.superAdmin, undefined);
});

// ---------------------------------------------------------------------------
// Page tiers (ensurePermission) — two tiers, admin and subscriber
// ---------------------------------------------------------------------------

/**
 * Exercises ensureAuthenticated.js's page-tier decision in a child, the same way
 * inChild() does for the scope decision. req.user carries the tier role NAMES (that is
 * what ensureAuthenticated assigns onto it before calling ensurePermission), and
 * `roles` is the caller's own roles claim.
 */
function checkPage(overlayToml, page, callerRoles) {
    return inChild(overlayToml, `
        const { ensurePermission } = require(${JSON.stringify(path.join(__dirname, '..', 'middlewares', 'ensureAuthenticated.js'))});
        const { portalRoles } = authz;
        const { admin, subscriber } = portalRoles();
        const req = { user: { admin, subscriber } };
        emit(ensurePermission(${JSON.stringify(page)}, ${JSON.stringify(callerRoles)}, req));
    `);
}

const DEFAULT_TIERS = '[api_portal.auth.authorization]\npage_role_validation = true\n';

test('the settings page requires the admin tier', () => {
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/settings', ['admin']), true);
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/settings', ['Internal/subscriber']), false);
});

test('applications, api-keys and subscriptions accept either tier', () => {
    for (const page of ['/acme/applications', '/acme/api-keys', '/acme/subscriptions']) {
        assert.equal(checkPage(DEFAULT_TIERS, page, ['Internal/subscriber']), true, `${page} / subscriber`);
        assert.equal(checkPage(DEFAULT_TIERS, page, ['admin']), true, `${page} / admin`);
    }
});

test('a page outside both tier lists is denied, not defaulted open', () => {
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/something-else', ['admin']), false);
});

test('a caller with no matching role is denied', () => {
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/applications', ['some_other_group']), false);
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/applications', []), false);
});

test('the retired superAdmin tier no longer grants anything on its own', () => {
    // Deliberate behaviour change: "superAdmin" used to grant the settings page and the
    // /portal pages. Those routes are not served here, and the tier is gone — a
    // deployment whose IDP still emits it must now map it via portal_roles.admin.
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/settings', ['superAdmin']), false);
    assert.equal(checkPage(DEFAULT_TIERS, '/acme/applications', ['superAdmin']), false);
    // ...which is exactly how an operator restores it.
    const mapped = '[api_portal.auth.authorization.portal_roles]\nadmin = "superAdmin"\n';
    assert.equal(checkPage(mapped, '/acme/settings', ['superAdmin']), true);
});

test('the old /portal pages are no longer a tier of their own', () => {
    // API_PORTAL_ROOT is gone, so these fall through to the deny at the end rather
    // than to a superAdmin-only branch.
    for (const page of ['/portal', '/portal/x/edit', '/devportal']) {
        assert.equal(checkPage(DEFAULT_TIERS, page, ['admin']), false, page);
    }
});

test('the shipped config pins page tiers to roles the shipped grant table defines', () => {
    // A coherence check on the shipped pack, not a general rule: page tiers and REST
    // authorization are separate mechanisms, and portal_roles names roles as they
    // appear in the TOKEN. But with role mode the default, a shipped page tier naming
    // a role the shipped table doesn't define would mean the quickstart admin can
    // reach the REST API and still be refused the settings page.
    const roleScopeMap = require('./roleScopeMap');
    const table = roleScopeMap.loadRoleScopeMap(SHIPPED_MAPPING_PATH,
        path.join(PROJECT_ROOT, 'docs', 'api-portal-openapi-spec-v0.9.yaml'));
    const shipped = fs.readFileSync(path.join(PROJECT_ROOT, 'configs', 'config.toml'), 'utf8');
    // Scope the scan to the portal_roles table so an `admin = ...` key in some other
    // table can never be mistaken for a page tier.
    const start = shipped.indexOf('[api_portal.auth.authorization.portal_roles]');
    assert.notEqual(start, -1, 'shipped config.toml has no portal_roles table');
    const rest = shipped.slice(start + 1);
    const nextTable = rest.indexOf('\n[');
    const portalRolesToml = nextTable === -1 ? rest : rest.slice(0, nextTable);
    const tiers = [...portalRolesToml.matchAll(/^(admin|subscriber)\s*=\s*"([a-z0-9_]+)"/gm)]
        .map((m) => ({ tier: m[1], role: m[2] }));
    assert.equal(tiers.length, 2, `expected two portal_roles entries, found ${tiers.length}`);
    for (const { tier, role } of tiers) {
        assert.ok(table.has(role),
            `portal_roles.${tier} defaults to "${role}", which the shipped grant table does not define`);
    }
});
