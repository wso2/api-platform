// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// Keeps the two IT grant tables in step.
//
// The whole suite runs in both authorization modes against ONE set of expectations,
// which only works because each IT account is granted the same thing either way:
//
//   scope mode — platform-api expands the account's roles into the token's scope claim
//                (configs/roles-platform-api-it.yaml)
//   role  mode — the portal expands the same roles through its own table
//                (configs/portal-roles-it.yaml)
//
// Two files, so they can drift. The failure that drift causes is nasty: add a scope to
// platform-api's table for a new endpoint's tests, and the scope-mode run goes green
// while the role-mode run fails somewhere unrelated-looking, with a 403 that points at
// the endpoint rather than at the table. This spec turns that into one obvious failure
// naming the role and the missing scopes.
//
// No HTTP — it reads the two fixture files directly, so it costs nothing and reports
// the same way in both modes.

const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

// Mounted at /it-configs in the test container (the repo's it/configs is outside
// the /rest-api mount); the relative path is the fallback for a host-side run.
const CONFIGS = process.env.IT_CONFIGS_DIR
    || (fs.existsSync('/it-configs') ? '/it-configs' : path.join(__dirname, '..', '..', 'configs'));

// Deliberately divergent — see the comments in both files. This is the one role whose
// portal-side grant is narrower than its scope claim, which is what makes the
// mode difference observable in authorization-mode.spec.js.
const INTENTIONALLY_DIVERGENT = new Set(['dp_narrow_it']);

function loadRoles(file) {
    const doc = yaml.load(fs.readFileSync(path.join(CONFIGS, file), 'utf8'));
    return new Map((doc.roles || []).map((r) => [r.name, [...r.scopes].sort()]));
}

describe('IT grant tables', () => {
    const platformApi = loadRoles('roles-platform-api-it.yaml');
    const portal = loadRoles('portal-roles-it.yaml');
    const shared = [...platformApi.keys()].filter((r) => !INTENTIONALLY_DIVERGENT.has(r));

    it('define the same roles on both sides', () => {
        expect(shared.length).toBeGreaterThan(0); // guards against an empty-file false pass
        expect([...portal.keys()].sort()).toEqual([...platformApi.keys()].sort());
    });

    it.each(shared)('grant %s identical scopes on both sides', (role) => {
        // Fails as a readable scope diff naming the role, rather than as a 403 in
        // whichever unrelated spec happened to need the missing scope first.
        expect(portal.get(role)).toEqual(platformApi.get(role));
    });

    it('keeps dp_narrow_it deliberately narrower on the portal side', () => {
        // The divergence is load-bearing, so assert it rather than merely excluding
        // it: if someone "fixes" the mirror by syncing this role too, the scope-claim-
        // is-ignored proof in authorization-mode.spec.js silently stops proving it.
        const paScopes = platformApi.get('dp_narrow_it');
        const portalScopes = portal.get('dp_narrow_it');
        expect(paScopes).toContain('dp:application:create');
        expect(portalScopes).not.toContain('dp:application:create');
        expect(portalScopes.every((s) => s.endsWith(':read'))).toBe(true);
    });
});
