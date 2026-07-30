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
 *
 */

/*
 * OpenAPI operation handlers for the Organizations tag.
 *
 * This portal serves a single organization, fixed by config and created on startup
 * by the seeder (src/utils/orgContext.js, src/services/seederService.js), so the
 * organization lifecycle is not something a client drives:
 *
 *   createOrganization  405 — the seeder owns creation
 *   getOrganizations    405 — listing is inherently cross-organization
 *   deleteOrganization  405 — nothing here may remove this instance's own org
 *
 * The operations stay in the spec, and the adminService functions behind them stay
 * intact, so re-enabling any of them is a one-line change here. The 405 lives at
 * this routing boundary rather than inside the service because that is where an
 * HTTP status belongs — and because the isolation guarantee comes from the org pin
 * in authMiddleware/orgGuard, not from these three shims.
 */
const adminService = require('../../../services/adminService');
const apiPortalService = require('../../../services/apiPortalService');
const util = require('../../../utils/util');
const logger = require('../../../config/logger');

/**
 * Rejects an organization-lifecycle operation this deployment does not offer.
 * Declared as a "405" response on each of these operations in the spec, so the
 * router's response validation treats it as expected rather than drift.
 */
function singleOrgNotSupported(req, res) {
    logger.info('Rejected organization lifecycle operation on a single-organization portal', {
        method: req.method,
        operation: 'singleOrgNotSupported',
    });
    return util.sendError(
        res,
        405,
        'This API Portal serves a single organization, which is configured and ' +
        'provisioned at startup. Organizations cannot be created, listed, or deleted ' +
        'through the API.'
    );
}

module.exports = {
    createOrganization: singleOrgNotSupported,
    getOrganizations: singleOrgNotSupported,
    deleteOrganization: singleOrgNotSupported,
    updateOrganization: adminService.updateOrganization,
    getOrganization: apiPortalService.getOrganization,
};
