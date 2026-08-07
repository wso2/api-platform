/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 
const adminService = require('../services/adminService');
const orgDao = require('../dao/organizationDao');
const orgContext = require('../utils/orgContext');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const util = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const { retrieveContentType } = require('../utils/util');
const fs = require('fs');
const path = require('path');

// Map a view asset `fileType` to its packaged src/defaultContent subdirectory.
// Only these static asset kinds have a safe on-disk fallback.
const DEFAULT_CONTENT_DIRS = { style: 'styles', image: 'images' };

/**
 * Serve the packaged src/defaultContent asset matching (fileType, fileName) when
 * a view has not uploaded its own copy. Themes commonly override only main.css
 * and inherit the rest; main.css's rewritten @imports (home.css, api-content.css)
 * resolve through this endpoint, so those must fall back rather than 404.
 * Returns true if it served a file. `fileName` is reduced to its basename so it
 * cannot escape the default-content directory.
 */
function serveDefaultContentAsset(res, fileType, fileName) {
    const dir = DEFAULT_CONTENT_DIRS[fileType];
    if (!dir) return false;
    const safeName = path.basename(String(fileName || ''));
    if (!safeName || safeName === '.' || safeName === '..') return false;
    const filePath = path.join(process.cwd(), 'src', 'defaultContent', dir, safeName);
    if (!fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) return false;
    res.set(constants.MIME_TYPES.CONYEMT_TYPE, retrieveContentType(safeName, fileType));
    res.status(200).send(fs.readFileSync(filePath));
    return true;
}

const getOrganization = async (req, res) => {
    try {
        // Only this instance's own organization is readable — the {orgId} parameter
        // selects nothing else, even though the shared database holds other orgs.
        await orgContext.requirePinnedOrg(req.params.orgId);
        const organization = await getOrganizationDetails(req.params.orgId);
        res.status(200).json(organization);
    } catch (error) {
        util.handleError(res, error);
    }
};

const getOrganizationDetails = async (orgId) => {
    const organization = await orgDao.get(orgId);
    const audit = await userIdpReferenceDao.buildSingleAuditFields(organization);
    return {
        id: organization.handle,
        displayName: organization.display_name,
        businessOwner: organization.business_owner,
        businessOwnerContact: organization.business_owner_contact,
        businessOwnerEmail: organization.business_owner_email,
        idpRefId: organization.idp_ref_id,
        cpRefId: organization.cp_ref_id,
        configuration: organization.configuration,
        ...audit,
    };
}

const getOrgContent = async (req, res) => {
    try {
        if (req.query.fileType && req.query.fileName) {
            // This endpoint is public — the login page fetches its CSS before a session
            // exists. The session org wins when there is one; otherwise style/image
            // assets fall back to this instance's own organization so the view's theme
            // still resolves.
            //
            // That fallback is the *configured* organization, never the caller-supplied
            // ?orgId. The database is shared across organizations, so on an endpoint with
            // no credential that query parameter would be an unauthenticated selector for
            // any tenant's branding. It is still accepted (util.js's style-URL rewrite
            // appends it, and the spec declares it) and ignored.
            //
            // Anything short of a resolved org plus a matching row — no org, a lookup
            // fault, no such asset — degrades to the packaged default content below
            // rather than a hard 404. The org is also guarded because passing an
            // undefined one into the DAO throws.
            let assetOrgId = req.orgId;
            if (!assetOrgId && DEFAULT_CONTENT_DIRS[req.query.fileType]) {
                assetOrgId = await orgContext.getOrgUuid().catch(() => null);
            }
            let asset = null;
            if (assetOrgId) {
                try {
                    asset = await adminService.getOrgContent(assetOrgId, req.params.viewId, req.query.fileType, req.query.fileName, req.query.filePath);
                } catch (lookupErr) {
                    logger.warn('View asset lookup failed; falling back to default content', {
                        error: lookupErr.message, viewId: req.params.viewId, fileName: req.query.fileName
                    });
                }
            }
            if (asset) {
                const contentType = retrieveContentType(asset.file_name, asset.file_type);
                res.set(constants.MIME_TYPES.CONYEMT_TYPE, contentType);
                return res.status(200).send(Buffer.isBuffer(asset.file_content) ? asset.file_content : constants.CHARSET_UTF8);
            }
            if (serveDefaultContentAsset(res, req.query.fileType, req.query.fileName)) {
                return;
            }
            return res.status(404).send('Not Found');
        } else if (req.params.fileType) {
            const assets = await adminService.getOrgContent(req.orgId, req.params.viewId, req.params.fileType);
            const results = [];
            for (const asset of assets) {
                const resp = {
                    id: asset.org_uuid,
                    fileName: asset.file_name,
                    fileContent: asset.file_content ? asset.file_content.toString(constants.CHARSET_UTF8) : null
                };
                results.push(resp);
            }
            return res.status(200).send(results);
        } else {
            util.sendError(res, 400, 'Invalid request');
        }
    } catch (error) {
        logger.error('Error while fetching organization content', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            viewId: req.params.viewId
        });
        return util.sendError(res, 500, 'Internal Server Error');
    }
};

module.exports = {
    getOrgContent,
    getOrganization,
    getOrganizationDetails
};
