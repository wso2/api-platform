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
 * What the startup seeder should do with the organization's stored IdP org
 * identifier — auth.idp_org_id in config, the idp_ref_id column on the row.
 *
 * Kept as a standalone, dependency-free module — not a helper inside
 * seederService.js — so the policy can be unit-tested without loading the config
 * layer and database driver that requiring the seeder would pull in.
 */

/**
 * Decides what the startup reconcile should do with the stored value, given the
 * explicitly configured auth.idp_org_id (orgContext.getConfiguredIdpOrgId), what is
 * stored on the organization row, and any other organization already answering to it.
 *
 *   - 'skip'     — nothing configured (never rewrite a stored value back to the
 *                  handle just because the setting is absent), or already in sync.
 *   - 'conflict' — another organization in a shared database already resolves this
 *                  value via its handle/display_name/idp_ref_id; taking it would
 *                  shadow that organization, so the stored value is left as-is.
 *   - 'update'   — write the configured value.
 *
 * @param {{configured: string, stored: string, conflictingOrgHandle?: string}} input
 * @returns {{action: 'skip'|'update'|'conflict'}}
 */
function planIdpOrgIdReconcile({ configured, stored, conflictingOrgHandle }) {
    if (!configured) return { action: 'skip' };
    // Compared verbatim: the stored value is matched case-sensitively against the token
    // claim, so a case-only difference is a real difference worth writing.
    if (configured === stored) return { action: 'skip' };
    if (conflictingOrgHandle) return { action: 'conflict' };
    return { action: 'update' };
}

module.exports = { planIdpOrgIdReconcile };
