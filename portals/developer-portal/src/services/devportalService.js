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
const adminDao = require('../dao/admin');
const util = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const { validationResult } = require('express-validator');
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
    const organization = await adminDao.getOrganization(orgId);
    return {
        orgId: organization.ORG_ID,
        orgName: organization.ORG_NAME,
        businessOwner: organization.BUSINESS_OWNER,
        businessOwnerContact: organization.BUSINESS_OWNER_CONTACT,
        businessOwnerEmail: organization.BUSINESS_OWNER_EMAIL,
        orgHandle: organization.ORG_HANDLE,
        roleClaimName: organization.ROLE_CLAIM_NAME,
        groupsClaimName: organization.GROUPS_CLAIM_NAME,
        organizationClaimName: organization.ORGANIZATION_CLAIM_NAME,
        organizationIdentifier: organization.ORGANIZATION_IDENTIFIER,
        adminRole: organization.ADMIN_ROLE,
        superAdminRole: organization.SUPER_ADMIN_ROLE,
        subscriberRole: organization.SUBSCRIBER_ROLE,
        groupClaimName: organization.GROUP_CLAIM_NAME,
        orgConfiguration: organization.ORG_CONFIG,
    };
}

const getOrgContent = async (req, res) => {
    try {
        const rules = util.validateRequestParameters();
        for (let validation of rules) {
            await validation.run(req);
        }
        const errors = validationResult(req);
        if (!errors.isEmpty()) {
            return res.status(400).json(util.getErrors(errors));
        }
        if (req.query.fileType && req.query.fileName) {
            const asset = await adminService.getOrgContent(req.params.orgId, req.params.name, req.query.fileType, req.query.fileName, req.query.filePath);
            if (asset) {
                const contentType = asset ? retrieveContentType(asset.FILE_NAME, asset.FILE_TYPE) : "";
                res.set(constants.MIME_TYPES.CONYEMT_TYPE, contentType);
                return res.status(200).send(Buffer.isBuffer(asset.FILE_CONTENT) ? asset.FILE_CONTENT : constants.CHARSET_UTF8);
            }
        } else if (req.params.fileType) {
            const assets = await adminService.getOrgContent(req.params.orgId, req.params.name, req.params.fileType);
            const results = [];
            for (const asset of assets) {
                const resp = {
                    orgId: asset.ORG_ID,
                    fileName: asset.FILE_NAME,
                    fileContent: asset.FILE_CONTENT ? asset.FILE_CONTENT.toString(constants.CHARSET_UTF8) : null
                };
                results.push(resp);
            }
            return res.status(200).send(results);
        } else {
            res.status(400).send('Invalid request');
        }
    } catch (error) {
        logger.error('Error while fetching organization content', {
            error: error.message,
            stack: error.stack,
            orgId: req.params.orgId,
            viewName: req.params.name
        });
        res.status(404).send(error.message);
    }
};

module.exports = {
    getOrgContent,
    getOrganization,
    getOrganizationDetails
};
