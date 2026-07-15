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
/* eslint-disable no-undef */
const adminService = require('../services/adminService');
const orgDao = require('../dao/organizationDao');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const util = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const { retrieveContentType } = require('../utils/util');

const getOrganization = async (req, res) => {
    try {
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
            const asset = await adminService.getOrgContent(req.orgId, req.params.viewId, req.query.fileType, req.query.fileName, req.query.filePath);
            if (asset) {
                const contentType = asset ? retrieveContentType(asset.file_name, asset.file_type) : "";
                res.set(constants.MIME_TYPES.CONYEMT_TYPE, contentType);
                return res.status(200).send(Buffer.isBuffer(asset.file_content) ? asset.file_content : constants.CHARSET_UTF8);
            } else {
                return util.sendError(res, 404, 'Not Found');
            }
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
