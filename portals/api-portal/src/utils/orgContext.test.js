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
 * Organization resolution in both modes. The default mode must keep answering "only
 * the configured organization"; multi-tenancy mode must accept any existing organization
 * where that is safe (claims, page URLs, public branding) while still confining a
 * REST caller to its own organization.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const { CustomError, NotFoundError } = require('./errors/customErrors');

const r = (p) => path.resolve(__dirname, p);
const PATHS = {
    configLoader: r('../config/configLoader.js'),
    logger: r('../config/logger.js'),
    orgDao: r('../dao/organizationDao.js'),
    viewDao: r('../dao/viewDao.js'),
    seeder: r('../services/seederService.js'),
    orgContext: r('./orgContext.js'),
};

const ORGS = [
    { uuid: 'u-default', handle: 'default', display_name: 'Default', idp_ref_id: 'default' },
    { uuid: 'u-acme', handle: 'acme', display_name: 'Acme', idp_ref_id: 'acme-id' },
    { uuid: 'u-dup1', handle: 'dup1', display_name: 'Dup 1', idp_ref_id: 'dup' },
    { uuid: 'u-dup2', handle: 'dup2', display_name: 'Dup 2', idp_ref_id: 'dup' },
    // A display name equal to another organization's idp_ref_id-looking value.
    { uuid: 'u-lookalike', handle: 'lookalike', display_name: 'acme-id-lookalike', idp_ref_id: 'x' },
];

function loadOrgContext({ multiTenancy, mode = 'idp', enforce = true, idp = { tokenUrl: 'https://idp.example.com/token', jwksUrl: 'https://idp.example.com/jwks' }, orgs = ORGS, seeded = [], claimMappings = {} }) {
    const notFound = () => { throw new NotFoundError('Organization not found'); };
    const ORGS = orgs; // this load's own table, so provisioning in one test cannot leak into another
    const stubs = {
        configLoader: {
            config: { multiTenancy: { enabled: multiTenancy }, auth: { mode, enforceOrgValidation: enforce, idp, claimMappings }, organization: { handle: 'default' } },
            ORG_HANDLE_PATTERN: /^[a-z0-9][a-z0-9._-]*$/,
            RESERVED_ORG_HANDLES: new Set(['api', 'health', 'registry']),
        },
        seeder: {
            seedOrg: async (org) => {
                seeded.push(org);
                const row = { uuid: `u-new-${seeded.length}`, handle: org.handle, display_name: org.displayName, idp_ref_id: org.idpRefId };
                ORGS.push(row);
                return { org: row, existed: false };
            },
        },
        logger: { error() {}, warn() {}, info() {} },
        orgDao: {
            getByHandle: async (h) => ORGS.find((o) => o.handle === String(h).toLowerCase()) || notFound(),
            getByUuid: async (u) => ORGS.find((o) => o.uuid === u) || notFound(),
            listByIdpRefId: async (v) => ORGS.filter((o) => o.idp_ref_id === v),
            findByDisplayName: async (v) => ORGS.find((o) => o.display_name === v) || null,
            // The loose ladder: handle, then display_name, then idp_ref_id.
            getId: async (v) => (ORGS.find((o) => o.handle === String(v).toLowerCase())
                || ORGS.find((o) => o.display_name === v)
                || ORGS.find((o) => o.idp_ref_id === v) || notFound()).uuid,
        },
        viewDao: { getFallbackHandle: async (orgUuid) => `view-of-${orgUuid}` },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.orgContext];
    try {
        return require(PATHS.orgContext);
    } finally {
        // The seeder stub stays: orgContext requires it lazily, at provisioning time.
        for (const [k, p] of Object.entries(PATHS)) if (k !== 'seeder') delete require.cache[p];
    }
}

const is403 = (err) => err instanceof CustomError && err.statusCode === 403;

test('multi-tenancy mode is off by default and never on outside IDP mode', () => {
    assert.strictEqual(loadOrgContext({ multiTenancy: false }).isMultiTenancyEnabled(), false);
    assert.strictEqual(loadOrgContext({ multiTenancy: true, mode: 'local' }).isMultiTenancyEnabled(), false);
    assert.strictEqual(loadOrgContext({ multiTenancy: true }).isMultiTenancyEnabled(), true);
});

test('default mode: a claim must name the configured organization, by any spelling', async () => {
    const ctx = loadOrgContext({ multiTenancy: false });
    assert.strictEqual(await ctx.resolveClaimOrg('default', 't'), 'u-default');
    assert.strictEqual(await ctx.resolveClaimOrg('Default', 't'), 'u-default'); // display name
    await assert.rejects(ctx.resolveClaimOrg('acme-id', 't'), is403);
    await assert.rejects(ctx.resolveClaimOrg('nope', 't'), is403);
});

test('multi-tenancy mode: a claim resolves by exact idp_ref_id to any organization', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(await ctx.resolveClaimOrg('acme-id', 't'), 'u-acme');
    assert.strictEqual(await ctx.resolveClaimOrg('default', 't'), 'u-default');
});

test('multi-tenancy mode: a claim never resolves through a handle or display name', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    await assert.rejects(ctx.resolveClaimOrg('acme', 't'), is403); // acme's handle, not its idp_ref_id
    await assert.rejects(ctx.resolveClaimOrg('Acme', 't'), is403); // display name
    await assert.rejects(ctx.resolveClaimOrg('unknown', 't'), is403);
});

test('multi-tenancy mode: an idp_ref_id shared by two organizations is refused as ambiguous', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    await assert.rejects(ctx.resolveClaimOrg('dup', 't'), is403);
});

test('multi-tenancy mode: a page URL resolves by exact handle only', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(await ctx.requireKnownOrgHandle('acme'), 'u-acme');
    assert.strictEqual(await ctx.requireKnownOrgHandle('ACME'), 'u-acme');
    await assert.rejects(ctx.requireKnownOrgHandle('acme-id'), NotFoundError); // idp_ref_id
    await assert.rejects(ctx.requireKnownOrgHandle('Acme Corp'), NotFoundError);
});

test('default mode: an {orgId} parameter must be the configured organization', async () => {
    const ctx = loadOrgContext({ multiTenancy: false });
    assert.strictEqual(await ctx.requireCallerOrg('default', 'u-default'), 'u-default');
    await assert.rejects(ctx.requireCallerOrg('acme', 'u-acme'), is403);
});

test('multi-tenancy mode: an {orgId} parameter must be the caller\'s own organization', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(await ctx.requireCallerOrg('acme', 'u-acme'), 'u-acme');
    // An existing organization, but not the caller's — the cross-tenant case.
    await assert.rejects(ctx.requireCallerOrg('default', 'u-acme'), is403);
    await assert.rejects(ctx.requireCallerOrg('acme', undefined), is403);
    await assert.rejects(ctx.requireCallerOrg('nope', 'u-acme'), is403);
});

test('default mode: public content ignores ?orgId', async () => {
    const ctx = loadOrgContext({ multiTenancy: false });
    assert.strictEqual(await ctx.resolvePublicContentOrg('u-acme', undefined), 'u-default');
    assert.strictEqual(await ctx.resolvePublicContentOrg('u-acme', 'u-default'), 'u-default');
});

test('multi-tenancy mode: public content follows ?orgId when it names an organization', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(await ctx.resolvePublicContentOrg('u-acme', 'u-default'), 'u-acme');
    assert.strictEqual(await ctx.resolvePublicContentOrg('u-missing', 'u-acme'), 'u-acme');
    assert.strictEqual(await ctx.resolvePublicContentOrg(undefined, undefined), 'u-default');
});

test('the fallback view is chosen from the given organization', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(await ctx.getFallbackViewHandle('u-acme'), 'view-of-u-acme');
    assert.strictEqual(await ctx.getFallbackViewHandle(), 'view-of-u-default');
});

test('deriveHandle makes a URL-safe slug from a name or id claim', () => {
    const { deriveHandle } = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(deriveHandle('Acme Corp'), 'acme-corp');
    assert.strictEqual(deriveHandle('Société Générale'), 'societe-generale');
    assert.strictEqual(deriveHandle('  --Weird__Name!!  '), 'weird__name');
    assert.strictEqual(deriveHandle('3f6c2b1e-9d1a-4c1e-8a57-0e2f1b9d7c44'), '3f6c2b1e-9d1a-4c1e-8a57-0e2f1b9d7c44');
    assert.strictEqual(deriveHandle('組織'), '');
    assert.ok(deriveHandle('x'.repeat(200)).length <= 48);
});

test('enforcement on: an unknown org is refused even in multi-tenancy mode', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: true, orgs: [...ORGS], seeded });
    await assert.rejects(ctx.resolveClaimOrg('new-org', 't', { provision: 'login' }), is403);
    assert.strictEqual(seeded.length, 0);
});

test('multi-tenancy off: enforcement off never provisions', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: false, enforce: false, orgs: [...ORGS], seeded });
    await assert.rejects(ctx.resolveClaimOrg('new-org', 't', { provision: 'login' }), is403);
    assert.strictEqual(seeded.length, 0);
});

test('provisioning: an unknown org is created with the claim as its idp_ref_id and a name-derived handle', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    const uuid = await ctx.resolveClaimOrg('9d1a-org-id', 't', { provision: 'login', orgNames: { name: 'Globex Corp' } });
    assert.strictEqual(uuid, 'u-new-1');
    assert.deepStrictEqual(seeded, [{ handle: 'globex-corp', displayName: 'Globex Corp', idpRefId: '9d1a-org-id' }]);
    // A second login with the same claim reuses it rather than provisioning again.
    assert.strictEqual(await ctx.resolveClaimOrg('9d1a-org-id', 't', { provision: 'login', orgNames: { name: 'Globex Corp' } }), 'u-new-1');
    assert.strictEqual(seeded.length, 1);
});

test('provisioning without a name claim derives the handle from the org claim', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    await ctx.resolveClaimOrg('Initech', 't', { provision: 'bearer' });
    assert.deepStrictEqual(seeded, [{ handle: 'initech', displayName: 'Initech', idpRefId: 'Initech' }]);
});

test('provisioning moves to a suffixed handle and display name when another org holds them', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    // "acme" (handle) and "Acme" (display name) belong to the existing acme organization.
    await ctx.resolveClaimOrg('other-acme-id', 't', { provision: 'login', orgNames: { name: 'Acme' } });
    assert.strictEqual(seeded.length, 1);
    assert.match(seeded[0].handle, /^acme-[0-9a-f]{6}$/);
    assert.match(seeded[0].displayName, /^Acme \([0-9a-f]{6}\)$/);
    assert.strictEqual(seeded[0].idpRefId, 'other-acme-id');
});

test('provisioning never uses a reserved or unusable handle', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    await ctx.resolveClaimOrg('org-x', 't', { provision: 'login', orgNames: { name: 'API' } });
    assert.match(seeded[0].handle, /^api-[0-9a-f]{6}$/);
    await ctx.resolveClaimOrg('org-y', 't', { provision: 'login', orgNames: { name: '組織' } });
    assert.strictEqual(seeded[1].handle, 'org-y');
    // "login" would be shadowed by the portal's own /login route.
    await ctx.resolveClaimOrg('org-z', 't', { provision: 'login', orgNames: { name: 'Login' } });
    assert.match(seeded[2].handle, /^login-[0-9a-f]{6}$/);
});

test('provisioning takes the handle claim as-is, and names the org from the name claim', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    await ctx.resolveClaimOrg('7f0c-uuid', 't', { provision: 'login', orgNames: { name: 'TEST 1', handle: 'test1' } });
    assert.deepStrictEqual(seeded, [{ handle: 'test1', displayName: 'TEST 1', idpRefId: '7f0c-uuid' }]);
    // Handles are stored lowercase; without a name claim the handle claim names the org.
    await ctx.resolveClaimOrg('8e1d-uuid', 't', { provision: 'login', orgNames: { handle: 'Test2' } });
    assert.deepStrictEqual(seeded[1], { handle: 'test2', displayName: 'Test2', idpRefId: '8e1d-uuid' });
});

test('an unusable or taken handle claim falls back like a derived handle would', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    // Taken by the existing acme organization: suffixed, never another org's handle.
    await ctx.resolveClaimOrg('u-1', 't', { provision: 'login', orgNames: { name: 'Acme Two', handle: 'acme' } });
    assert.match(seeded[0].handle, /^acme-[0-9a-f]{6}$/);
    // Outside the handle alphabet, reserved, or too long: derived from the name instead.
    await ctx.resolveClaimOrg('u-2', 't', { provision: 'login', orgNames: { name: 'Beta Co', handle: 'beta co' } });
    assert.strictEqual(seeded[1].handle, 'beta-co');
    await ctx.resolveClaimOrg('u-3', 't', { provision: 'login', orgNames: { name: 'Gamma', handle: 'login' } });
    assert.strictEqual(seeded[2].handle, 'gamma');
    await ctx.resolveClaimOrg('u-4', 't', { provision: 'login', orgNames: { name: 'Delta', handle: 'd'.repeat(129) } });
    assert.strictEqual(seeded[3].handle, 'delta');
});

test('orgNameClaims reads only the mapped name and handle claims', () => {
    const ctx = loadOrgContext({ multiTenancy: true, claimMappings: { orgName: 'org_name', orgHandle: 'org.handle' } });
    assert.deepStrictEqual(ctx.orgNameClaims({ org_name: ' TEST 1 ', org: { handle: 'test1' } }), { name: 'TEST 1', handle: 'test1' });
    assert.deepStrictEqual(ctx.orgNameClaims({ org_name: '', org: { handle: 42 } }), { name: undefined, handle: undefined });
    assert.deepStrictEqual(loadOrgContext({ multiTenancy: true }).orgNameClaims({ org_name: 'x', org_handle: 'y' }), { name: undefined, handle: undefined });
});

test('provisioning is refused when the claim\'s channel is not verified TLS', async () => {
    const plainHttp = { tokenUrl: 'http://idp.example.com/token', jwksUrl: 'http://idp.example.com/jwks' };
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, idp: plainHttp, orgs: [...ORGS], seeded });
    await assert.rejects(ctx.resolveClaimOrg('new-org', 't', { provision: 'login' }), is403);
    await assert.rejects(ctx.resolveClaimOrg('new-org', 't', { provision: 'bearer' }), is403);
    // Existing organizations are unaffected by the guard.
    assert.strictEqual(await ctx.resolveClaimOrg('acme-id', 't', { provision: 'login' }), 'u-acme');
    assert.strictEqual(seeded.length, 0);
});

test('provisioning allows plain http to a loopback IDP, and a pinned certificate for bearer tokens', async () => {
    const seeded = [];
    const loopback = loadOrgContext({ multiTenancy: true, enforce: false, idp: { tokenUrl: 'http://localhost:9900/token' }, orgs: [...ORGS], seeded });
    await loopback.resolveClaimOrg('dev-org', 't', { provision: 'login' });
    const pinned = loadOrgContext({ multiTenancy: true, enforce: false, idp: { certificate: 'PEM', jwksUrl: 'http://idp.example.com/jwks' }, orgs: [...ORGS], seeded });
    await pinned.resolveClaimOrg('cert-org', 't', { provision: 'bearer' });
    assert.strictEqual(seeded.length, 2);
});

test('provisioning is refused when TLS verification is disabled process-wide', async () => {
    const seeded = [];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs: [...ORGS], seeded });
    process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0';
    try {
        await assert.rejects(ctx.resolveClaimOrg('new-org', 't', { provision: 'login' }), is403);
    } finally {
        delete process.env.NODE_TLS_REJECT_UNAUTHORIZED;
    }
    assert.strictEqual(seeded.length, 0);
});

test('the configured organization\'s claim value comes from its stored idp_ref_id', async () => {
    const orgs = [{ uuid: 'u-default', handle: 'default', display_name: 'Default', idp_ref_id: 'SEEDED-ID' }];
    const ctx = loadOrgContext({ multiTenancy: true, enforce: false, orgs });
    assert.strictEqual(await ctx.getConfiguredOrgIdpRefId(), 'SEEDED-ID');
});

test('normalizeOrgClaim: a string is kept verbatim, a single-entry list or map is its entry', () => {
    const { normalizeOrgClaim } = loadOrgContext({ multiTenancy: true });
    assert.strictEqual(normalizeOrgClaim('Acme-ID'), 'Acme-ID');
    assert.strictEqual(normalizeOrgClaim(['acme']), 'acme');
    assert.strictEqual(normalizeOrgClaim({ acme: { id: '1f2e' } }), 'acme');
    assert.strictEqual(normalizeOrgClaim(undefined), '');
    assert.strictEqual(normalizeOrgClaim([]), '');
    assert.strictEqual(normalizeOrgClaim(['', 'acme']), 'acme');
});

test('normalizeOrgClaim: a claim naming more than one organization is refused', () => {
    const { normalizeOrgClaim } = loadOrgContext({ multiTenancy: true });
    assert.throws(() => normalizeOrgClaim(['acme', 'globex']), is403);
    assert.throws(() => normalizeOrgClaim({ acme: {}, globex: {} }), is403);
});

test('claimBelongsToOrg: exact idp_ref_id or an authorizedOrgs entry, nothing looser', () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    const acme = ORGS[1];
    assert.strictEqual(ctx.claimBelongsToOrg({ orgClaimName: 'acme-id' }, acme), true);
    assert.strictEqual(ctx.claimBelongsToOrg({ orgClaimName: 'other', authorizedOrgs: ['acme-id'] }, acme), true);
    assert.strictEqual(ctx.claimBelongsToOrg({ orgClaimName: 'ACME-ID' }, acme), false);
    assert.strictEqual(ctx.claimBelongsToOrg({ orgClaimName: 'acme' }, acme), false); // handle
    assert.strictEqual(ctx.claimBelongsToOrg({}, acme), false);
    assert.strictEqual(ctx.claimBelongsToOrg({ orgClaimName: 'acme-id' }, {}), false);
});

test('isForeignOrgSession: only a signed-in user of another organization', async () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    const acme = await ctx.requireKnownOrg('acme');
    assert.strictEqual(acme.uuid, 'u-acme');
    assert.strictEqual(ctx.isForeignOrgSession(null, acme), false); // anonymous
    assert.strictEqual(ctx.isForeignOrgSession({ orgClaimName: 'acme-id' }, acme), false);
    assert.strictEqual(ctx.isForeignOrgSession({ orgClaimName: 'default' }, acme), true);
    assert.strictEqual(ctx.isForeignOrgSession({}, acme), true); // no claim proves nothing
});

test('findSharedIdpRefIds reports each idp_ref_id carried by more than one organization', () => {
    const ctx = loadOrgContext({ multiTenancy: true });
    assert.deepStrictEqual(ctx.findSharedIdpRefIds(ORGS), [{ idpRefId: 'dup', handles: ['dup1', 'dup2'] }]);
    assert.deepStrictEqual(ctx.findSharedIdpRefIds([
        { handle: 'default', idp_ref_id: 'root-uuid' }, { handle: 'super', idp_ref_id: 'root-uuid' },
        { handle: 'a', idp_ref_id: 'Root-UUID' }, { handle: 'b' }, { handle: 'c', idp_ref_id: '' },
    ]), [{ idpRefId: 'root-uuid', handles: ['default', 'super'] }]);
    assert.deepStrictEqual(ctx.findSharedIdpRefIds([]), []);
});
