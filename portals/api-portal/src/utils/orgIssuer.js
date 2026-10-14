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
 * Per-organization token issuers (multi-organization mode,
 * auth.idp.multi_organization.issuer_pattern).
 *
 * Some IDPs issue each organization's tokens under that organization's own issuer and
 * signing keys — WSO2 IS / Asgardeo sub-organizations sign as
 * ".../o/<orgId>/oauth2/token" with keys at ".../o/<orgId>/oauth2/jwks" — so a single
 * configured issuer and JWKS URL can only ever verify one organization's tokens.
 *
 * The issuer a token names is read before its signature is checked, so nothing from it
 * may steer where keys come from beyond one inert identifier: the configured pattern is
 * matched against the whole `iss`, only the {org} segment is taken from it, that segment
 * must be a plain identifier (no '/', '.' runs, '%' or other URL syntax), and it is
 * substituted into a configured JWKS URL template — so the scheme, host and path of the
 * key fetch always come from configuration. The verified token must then carry that
 * same organization in the claim named by issuer_org_claim, so a token signed by one
 * organization can't assert another.
 */

const { config } = require('../config/configLoader');

// An organization id as it may appear in an issuer path segment. Deliberately narrow:
// it is spliced into a URL path, and anything that could change the path's meaning
// ('/', '..', percent-escapes, query/fragment markers) must never get through.
const ORG_SEGMENT = /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/;

function escapeRegExp(s) {
    return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

let compiledFor = null;
let compiled = null;

function issuerRegExp(pattern) {
    if (compiledFor !== pattern) {
        const [before, after] = pattern.split('{org}');
        compiled = new RegExp(`^${escapeRegExp(before)}([^/]+)${escapeRegExp(after)}$`);
        compiledFor = pattern;
    }
    return compiled;
}

/**
 * If `iss` is a per-organization issuer (it fits
 * auth.idp.multi_organization.issuer_pattern), returns the organization it names and the
 * JWKS URL to verify it with; otherwise null. Callers only consult this in
 * multi-organization mode.
 *
 * @param {string} iss the token's (not yet verified) issuer claim
 * @returns {{ org: string, issuer: string, jwksUrl: string } | null}
 */
function matchOrgIssuer(iss) {
    const idp = config.auth?.idp || {};
    const pattern = idp.multiOrganization?.issuerPattern;
    if (!pattern || typeof iss !== 'string') return null;
    const match = issuerRegExp(pattern).exec(iss);
    if (!match || !ORG_SEGMENT.test(match[1])) return null;
    const org = match[1];
    const template = idp.multiOrganization?.jwksUrlTemplate;
    const jwksUrl = template ? template.replace('{org}', org) : idp.jwksUrl;
    if (!jwksUrl) return null;
    return { org, issuer: iss, jwksUrl };
}

module.exports = { matchOrgIssuer };
