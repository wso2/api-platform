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
const { CustomError } = require('../utils/errors/customErrors');
const adminDao = require('../dao/admin');
const apiDao = require('../dao/apiMetadata');
const util = require('../utils/util');
const fs = require('fs');
const path = require('path');
const logger = require('../config/logger');
const IdentityProviderDTO = require("../dto/identityProvider");
const constants = require('../utils/constants');
const { validationResult } = require('express-validator');
const sequelize = require("../db/sequelize");
const { ApplicationDTO, SubscriptionDTO } = require('../dto/application');
const APIDTO = require('../dto/apiDTO');
const { config } = require('../config/configLoader');
const controlPlaneUrl = config.controlPlane.url;
const controlPlaneGwUrl = config.controlPlane.gwUrl;
const { invokeApiRequest } = require('../utils/util');
const yaml = require('js-yaml');
const { Sequelize } = require("sequelize");
const { trackGenerateCredentials, trackSubscribeApi, trackUnsubscribeApi } = require('../utils/telemetry');
const kmDao = require('../dao/keyManager');
const { getKeyManagerAdapter } = require('../adapters/keyManager');

function mapYamlToOrganization(parsed) {
    const { metadata = {}, spec = {} } = parsed;
    return {
        orgHandle: metadata.name,
        orgName: spec.displayName,
        organizationIdentifier: spec.organizationIdentifier,
        businessOwner: spec.businessOwner,
        businessOwnerContact: spec.businessOwnerContact,
        businessOwnerEmail: spec.businessOwnerEmail,
        roleClaimName: spec.roleClaimName,
        organizationClaimName: spec.organizationClaimName,
        groupsClaimName: spec.groupsClaimName,
        adminRole: spec.adminRole,
        subscriberRole: spec.subscriberRole,
        superAdminRole: spec.superAdminRole,
        identityProvider: spec.identityProvider || null,
        labels: spec.labels || null,
        views: spec.views || null,
    };
}

function parseOrganizationFromYamlFile(fileBuffer) {
    let parsed;
    try {
        parsed = yaml.load(fileBuffer.toString(constants.CHARSET_UTF8));
    } catch (e) {
        throw new Sequelize.ValidationError(`Invalid organization YAML file: ${e.message}`);
    }
    if (!parsed || typeof parsed !== 'object') {
        throw new Sequelize.ValidationError('Organization YAML file is empty or invalid');
    }
    if (parsed.kind !== 'Organization') {
        throw new Sequelize.ValidationError(
            `Unknown organization YAML kind '${parsed.kind}'. Expected 'Organization'`
        );
    }
    const { spec = {} } = parsed;
    if (spec.labels !== undefined && spec.labels !== null) {
        if (!Array.isArray(spec.labels) || spec.labels.some(l => typeof l !== 'object' || !l.name)) {
            throw new Sequelize.ValidationError("Invalid organization YAML: 'spec.labels' must be an array of objects with a 'name' field");
        }
    }
    if (spec.views !== undefined && spec.views !== null) {
        if (!Array.isArray(spec.views) || spec.views.some(v => typeof v !== 'object' || !v.name)) {
            throw new Sequelize.ValidationError("Invalid organization YAML: 'spec.views' must be an array of objects with a 'name' field");
        }
    }
    if (spec.identityProvider !== undefined && spec.identityProvider !== null) {
        if (typeof spec.identityProvider !== 'object' || Array.isArray(spec.identityProvider)) {
            throw new Sequelize.ValidationError("Invalid organization YAML: 'spec.identityProvider' must be an object");
        }
    }
    return mapYamlToOrganization(parsed);
}

function mapYamlToIdentityProvider(parsed) {
    const { metadata = {}, spec = {} } = parsed;
    return {
        name: metadata.name,
        issuer: spec.issuer,
        authorizationURL: spec.authorizationURL,
        tokenURL: spec.tokenURL,
        userInfoURL: spec.userInfoURL,
        clientId: spec.clientId,
        callbackURL: spec.callbackURL,
        scope: spec.scope,
        signUpURL: spec.signUpURL,
        logoutURL: spec.logoutURL,
        logoutRedirectURI: spec.logoutRedirectURI,
        jwksURL: spec.jwksURL,
        certificate: spec.certificate,
    };
}

function parseIdentityProviderFromYamlFile(fileBuffer) {
    let parsed;
    try {
        parsed = yaml.load(fileBuffer.toString(constants.CHARSET_UTF8));
    } catch (e) {
        throw new Sequelize.ValidationError(`Invalid identity provider YAML file: ${e.message}`);
    }
    if (!parsed || typeof parsed !== 'object') {
        throw new Sequelize.ValidationError('Identity provider YAML file is empty or invalid');
    }
    if (parsed.kind !== 'IdentityProvider') {
        throw new Sequelize.ValidationError(
            `Unknown YAML kind '${parsed.kind}'. Expected 'IdentityProvider'`
        );
    }
    return mapYamlToIdentityProvider(parsed);
}

const createOrganization = async (req, res) => {
    if (req.files?.organization?.[0]) {
        try {
            req.body = parseOrganizationFromYamlFile(req.files.organization[0].buffer);
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    logger.info('Initiate organization creation...', req.body);

    const rules = util.validateOrganization();
    for (let validation of rules) {
        await validation.run(req);
    }

    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        const errObj = util.getErrors(errors);
        logger.error('Organization creation request validation failed', {
            errors: errObj
        });
        return res.status(400).json(errObj);
    }
    logger.info('Organization creation request validation successful');

    const payload = req.body;
    payload.orgConfig = {
        devportalMode: constants.DEVPORTAL_MODE.DEFAULT,
    };

    let organization = "";
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            organization = await adminDao.createOrganization(payload, t);
            const orgId = organization.ORG_ID;
            logger.info('Organization created successfully', {
                orgId,
                orgName: organization.ORG_NAME
            });

            // Labels: use YAML-defined if provided, else fall back to default
            const labelDefs = payload.labels?.length
                ? payload.labels
                : [{ name: 'default', displayName: 'default' }];

            const createdLabels = await apiDao.createLabels(orgId, labelDefs, t);
            logger.info('Labels created successfully', { orgId });

            // Build name→ID map for view→label linking
            const labelMap = {};
            createdLabels.forEach(l => { labelMap[l.dataValues.NAME] = l.dataValues.LABEL_ID; });

            // Views: use YAML-defined if provided, else fall back to default
            const viewDefs = payload.views?.length
                ? payload.views
                : [{ name: 'default', displayName: 'default', labels: [labelDefs[0].name] }];

            for (const viewDef of viewDefs) {
                const viewResponse = await apiDao.addView(orgId, viewDef, t);
                const viewID = viewResponse.dataValues.VIEW_ID;
                for (const lName of (viewDef.labels || [])) {
                    const labelId = labelMap[lName];
                    if (labelId) {
                        await apiDao.addLabel(orgId, labelId, viewID, t);
                    }
                }
            }
            logger.info('Views created successfully', { orgId });
            //create default provider
            await adminDao.createProvider(organization.ORG_ID, { name: 'WSO2', providerURL: config.controlPlane.url }, t);
            logger.info('Default provider created successfully', {
                orgId
            });

            //store default subscription policies
            if (config.generateDefaultSubPolicies) {
                await apiDao.bulkCreateSubscriptionPolicies(orgId, constants.DEFAULT_SUBSCRIPTION_PLANS, t);
            }
            logger.info('Default subscription policies created successfully', {
                orgId
            });

            if (payload.identityProvider) {
                await adminDao.createIdentityProvider(orgId, payload.identityProvider, t);
                logger.info('Identity provider created successfully', { orgId });
            }
        });

        const orgCreationResponse = {
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
            subscriberRole: organization.SUBSCRIBER_ROLE,
            groupClaimName: organization.GROUP_CLAIM_NAME,
            orgConfiguration: organization.dataValues.ORG_CONFIG
        };
        logger.info('Organization creation flow completed successfully', {
            orgId: orgCreationResponse.orgId,
            orgName: orgCreationResponse.orgName,
        });
        res.status(201).send(orgCreationResponse);
    } catch (error) {
        logger.error('Organization creation failed', {
            error: error.message,
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

const getOrganizations = async (req, res) => {
    try {
        const orgList = await getAllOrganizations();
        res.status(200).send(orgList);
    } catch (error) {
        util.handleError(res, error);
    }
};

const getAllOrganizations = async () => {
    const organizations = await adminDao.getOrganizations();
    const orgList = [];
    if (organizations.length > 0) {
        for (const organization of organizations) {
            orgList.push({
                orgName: organization.dataValues.ORG_NAME,
                orgID: organization.dataValues.ORG_ID,
                businessOwner: organization.dataValues.BUSINESS_OWNER,
                businessOwnerContact: organization.dataValues.BUSINESS_OWNER_CONTACT,
                businessOwnerEmail: organization.dataValues.BUSINESS_OWNER_EMAIL,
                orgHandle: organization.ORG_HANDLE,
                roleClaimName: organization.ROLE_CLAIM_NAME,
                groupsClaimName: organization.GROUPS_CLAIM_NAME,
                organizationClaimName: organization.ORGANIZATION_CLAIM_NAME,
                organizationIdentifier: organization.ORGANIZATION_IDENTIFIER,
                adminRole: organization.ADMIN_ROLE,
                subscriberRole: organization.SUBSCRIBER_ROLE,
                superAdminRole: organization.SUPER_ADMIN_ROLE,
                orgConfiguration: organization.dataValues.ORG_CONFIG
            });
        }
    }
    return orgList;
}

const updateOrganization = async (req, res) => {
    const orgId = req.params.orgId;
    if (req.files?.organization?.[0]) {
        try {
            req.body = parseOrganizationFromYamlFile(req.files.organization[0].buffer);
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    logger.info('Initiate update organization...', {
        orgId,
        ...req.body
    });
    try {
        if (!orgId) {
            logger.warn('Missing required parameter: orgId');
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const rules = util.validateOrganization();
        for (let validation of rules) {
            await validation.run(req);
        }
        const errors = validationResult(req);
        if (!errors.isEmpty()) {
            return res.status(400).json(util.getErrors(errors));
        }
        const payload = req.body;
        payload.orgId = orgId;

        let updatedOrg;
        await sequelize.transaction({ timeout: 60000 }, async (t) => {
            [, updatedOrg] = await adminDao.updateOrganization(payload, t);
            logger.info('Organization update successful', { orgId });

            // IDP upsert — only if present in payload
            if (payload.identityProvider) {
                const existing = await adminDao.getIdentityProvider(orgId);
                if (existing.length > 0) {
                    await adminDao.updateIdentityProvider(orgId, payload.identityProvider, t);
                } else {
                    await adminDao.createIdentityProvider(orgId, payload.identityProvider, t);
                }
                logger.info('Identity provider upserted successfully', { orgId });
            }

            // Labels upsert — only if present in payload
            if (payload.labels?.length) {
                for (const label of payload.labels) {
                    await apiDao.updateLabel(orgId, label, t);
                }
                logger.info('Labels upserted successfully', { orgId });
            }

            // Views upsert — only if present in payload
            if (payload.views?.length) {
                for (const viewDef of payload.views) {
                    const view = await apiDao.updateView(orgId, viewDef.name, viewDef.displayName, t);
                    if (viewDef.labels?.length) {
                        await apiDao.replaceViewLabels(orgId, view.dataValues.VIEW_ID, viewDef.labels, t);
                    }
                }
                logger.info('Views upserted successfully', { orgId });
            }
        });

        res.status(200).json({
            orgId: updatedOrg[0].dataValues.ORG_ID,
            orgName: updatedOrg[0].dataValues.ORG_NAME,
            businessOwner: updatedOrg[0].dataValues.BUSINESS_OWNER,
            businessOwnerContact: updatedOrg[0].dataValues.BUSINESS_OWNER_CONTACT,
            businessOwnerEmail: updatedOrg[0].dataValues.BUSINESS_OWNER_EMAIL,
            orgHandle: updatedOrg[0].dataValues.ORG_HANDLE,
            roleClaimName: updatedOrg[0].dataValues.ROLE_CLAIM_NAME,
            groupsClaimName: updatedOrg[0].dataValues.GROUPS_CLAIM_NAME,
            organizationClaimName: updatedOrg[0].dataValues.ORGANIZATION_CLAIM_NAME,
            organizationIdentifier: updatedOrg[0].dataValues.ORGANIZATION_IDENTIFIER,
            adminRole: updatedOrg[0].dataValues.ADMIN_ROLE,
            subscriberRole: updatedOrg[0].dataValues.SUBSCRIBER_ROLE,
            superAdminRole: updatedOrg[0].dataValues.SUPER_ADMIN_ROLE,
            orgConfiguration: updatedOrg[0].dataValues.ORG_CONFIG
        });
    } catch (error) {
        logger.error('Organization update failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const deleteOrganization = async (req, res) => {
    const orgId = req.params.orgId;
    logger.info('Initiate delete organization...', {
        orgId
    });
    try {
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const deletedRowsCount = await adminDao.deleteOrganization(orgId);
        if (deletedRowsCount > 0) {
            logger.info('Organization deletion successful', {
                orgId
            });
            res.status(204).send();
        } else {
            throw new CustomError(404, "Records Not Found", 'Organization not found');
        }
    } catch (error) {
        logger.error('Organization deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const createIdentityProvider = async (req, res) => {
    const orgId = req.params.orgId;
    if (req.files?.identityProvider?.[0]) {
        try {
            req.body = parseIdentityProviderFromYamlFile(req.files.identityProvider[0].buffer);
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    logger.info('Initiate create identity provider...', {
        orgId,
        ...req.body
    });
    try {
        const idpData = req.body;
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const rules = util.validateIDP();
        for (let validation of rules) {
            await validation.run(req);
        }
        const errors = validationResult(req);
        if (!errors.isEmpty()) {
            return res.status(400).json(util.getErrors(errors));
        }
        const idpResponse = await adminDao.createIdentityProvider(orgId, idpData);
        logger.info('Identity provider created successfully', {
            orgId
        });
        res.status(201).send(new IdentityProviderDTO(idpResponse.dataValues));
    } catch (error) {
        logger.error('Identity provider creation failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const updateIdentityProvider = async (req, res) => {
    const orgId = req.params.orgId;
    if (req.files?.identityProvider?.[0]) {
        try {
            req.body = parseIdentityProviderFromYamlFile(req.files.identityProvider[0].buffer);
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    const idpData = req.body;
    logger.info('Initiate update identity provider...', {
        orgId,
        ...idpData
    });
    try {
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const rules = util.validateIDP();
        for (let validation of rules) {
            await validation.run(req);
        }
        const errors = validationResult(req);
        if (!errors.isEmpty()) {
            return res.status(400).json(util.getErrors(errors));
        }
        const [updatedRows, updatedIDP] = await adminDao.updateIdentityProvider(orgId, idpData);
        if (!updatedRows) {
            throw new Sequelize.EmptyResultError("No record found to update");
        }
        logger.info('Identity provider updated successfully', {
            orgId
        });
        res.status(200).send(new IdentityProviderDTO(updatedIDP[0].dataValues));
    } catch (error) {
        logger.error('Identity provider update failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const getIdentityProvider = async (req, res) => {

    const orgID = req.params.orgId;
    if (!orgID) {
        throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
    }
    try {
        const retrievedIDP = await adminDao.getIdentityProvider(orgID);
        // Create response object
        if (retrievedIDP.length > 0) {
            res.status(200).send(new IdentityProviderDTO(retrievedIDP[0]));
        } else {
            res.status(404).send();
        }
    } catch (error) {
        logger.error('Identity provider retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID
        });
        util.handleError(res, error);
    }
}

const deleteIdentityProvider = async (req, res) => {
    const orgId = req.params.orgId;
    logger.info('Initiate delete identity provider...', {
        orgId: orgId
    });
    if (!orgId) {
        throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
    }
    try {
        const idpDeleteResponse = await adminDao.deleteIdentityProvider(orgId);
        if (idpDeleteResponse === 0) {
            throw new Sequelize.EmptyResultError("Resource not found to delete");
        } else {
            logger.info('Identity provider deleted successfully', {
                orgId: orgId
            });
            res.status(200).send("Resouce Deleted Successfully");
        }
    } catch (error) {
        logger.error('Identity provider deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const createOrgContent = async (req, res) => {
    const orgId = req.params.orgId;
    const viewName = req.params.name;
    logger.info('Initiate create organization content...', {
        orgId,
        viewName
    });

    const extractPath = path.join(process.cwd(), '..', '.tmp', orgId);

    try {
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const zipPath = req.file?.path;
        if (!zipPath) {
            throw new CustomError(400, "Bad Request", "Missing required zip file");
        }
        if (req.file.size > 50 * 1024 * 1024) {
            throw new CustomError(400, "Bad Request", "File size exceeds the 50MB limit");
        }
        await util.unzipDirectory(zipPath, extractPath);
        const files = await util.readFilesInDirectory(extractPath, orgId, req.protocol, req.get('host'), viewName);
        for (const { filePath, fileName, fileContent, fileType } of files) {
            await createContent(filePath, fileName, fileContent, fileType, orgId, viewName);
        }
        logger.info('Organization content created successfully', {
            orgId,
            viewName
        });
        res.status(201).send({ "orgId": orgId, "fileName": req.file.originalname });
        fs.rmSync(extractPath, { recursive: true, force: true });

    } catch (error) {
        logger.error('Organization content creation failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            viewName,
            fileName: req.file?.originalname
        });
        fs.rmSync(extractPath, { recursive: true, force: true });
        return util.handleError(res, error);
    }
};

const createContent = async (filePath, fileName, fileContent, fileType, orgId, viewName) => {
    let content;
    // eslint-disable-next-line no-useless-catch
    try {
        if (fileName != null && !fileName.startsWith('.')) {
            content = await adminDao.createOrgContent({
                fileType: fileType,
                fileName: fileName,
                fileContent: fileContent,
                filePath: filePath,
                orgId: orgId,
                viewName: viewName
            });
        }
    } catch (error) {
        throw error;
    }
    return content;
};

const updateOrgContent = async (req, res) => {
    const orgId = req.params.orgId;
    const viewName = req.params.name;
    logger.info('Initiate update organization content...', {
        orgId,
        viewName
    });
    const extractPath = path.join(process.cwd(), '..', '.tmp', orgId);
    try {
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const zipPath = req.file?.path;
        if (!zipPath) {
            throw new CustomError(400, "Bad Request", "Missing required zip file");
        }
        if (req.file.size > 50 * 1024 * 1024) {
            throw new CustomError(400, "Bad Request", "File size exceeds the 50MB limit");
        }
        await util.unzipDirectory(zipPath, extractPath);
        const files = await util.readFilesInDirectory(extractPath, orgId, req.protocol, req.get('host'), viewName);
        for (const { filePath, fileName, fileContent, fileType } of files) {
            if (fileName != null && !fileName.startsWith('.')) {
                const organizationContent = await getOrgContent(orgId, viewName, fileType, fileName, filePath);
                if (organizationContent) {
                    await adminDao.updateOrgContent({
                        fileType: fileType,
                        fileName: fileName,
                        fileContent: fileContent,
                        filePath: filePath,
                        orgId: orgId,
                        viewName: viewName
                    });
                } else {
                    logger.info('Content not found during update, creating new content', {
                        orgId,
                        viewName,
                        fileType,
                        fileName,
                        filePath
                    });
                    await createContent(filePath, fileName, fileContent, fileType, orgId, viewName);
                }
            }
        }
        fs.rmSync(extractPath, { recursive: true, force: true });
        logger.info('Organization content updated successfully', {
            orgId,
            viewName
        });
        res.status(201).send({ "orgId": orgId, "fileName": req.file.originalname });
    } catch (error) {
        logger.error('Organization content update failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            viewName,
            fileName: req.file?.originalname
        });
        fs.rmSync(extractPath, { recursive: true, force: true });
        util.handleError(res, error);
    }
};

const getOrgContent = async (orgId, viewName, fileType, fileName, filePath) => {

    return await adminDao.getOrgContent({
        orgId: orgId,
        viewName: viewName,
        fileType: fileType,
        fileName: fileName,
        filePath: filePath
    });
};

const deleteOrgContent = async (req, res) => {
    const orgId = req.params.orgId;
    logger.info('Initiate delete organization content...', {
        orgId,
        viewName: req.params.name
    });
    try {
        const fileName = req.query.fileName;
        let deletedRowsCount;
        if (!req.query.fileName) {
            deletedRowsCount = await adminDao.deleteAllOrgContent(orgId, req.params.name);
        } else {
            deletedRowsCount = await adminDao.deleteOrgContent(orgId, req.params.name, fileName);
        }
        if (deletedRowsCount > 0) {
            logger.info('Organization content deletion successful', {
                orgId,
                viewName: req.params.name
            });
            res.status(204).send();
        } else {
            throw new CustomError(404, "Records Not Found", 'Organization not found');
        }
    } catch (error) {
        logger.error('Organization content deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId,
        });
        util.handleError(res, error);
    }
};

const deleteAllOrgContent = async (req, res) => {
    const orgId = req.params.orgId;
    logger.info('Initiate delete all organization content...', {
        orgId,
        viewName: req.params.name
    });
    try {
        const deletedRowsCount = await adminDao.deleteAllOrgContent(orgId, req.params.name, fileName);
        if (deletedRowsCount > 0) {
            logger.info('All organization content deletion successful', {
                orgId,
                viewName: req.params.name
            });
            res.status(204).send();
        } else {
            throw new CustomError(404, "Records Not Found", 'Organization not found');
        }
    } catch (error) {
        logger.error('All organization content deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            viewName: req.params.name
        });
        util.handleError(res, error);
    }
};

const createProvider = async (req, res) => {
    const orgID = req.params.orgId;
    const payload = req.body;
    const rules = util.validateProvider();

    for (let validation of rules) {
        await validation.run(req);
    }
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        return res.status(400).json(util.getErrors(errors));
    }
    const extraKeys = util.rejectExtraProperties(['name', 'providerURL'], payload)
    if (extraKeys.length > 0) {
        return res.status(400).json(new CustomError(400, "Bad Request", `Unexpected properties: ${extraKeys.join(', ')}`));
    }
    try {
        const provider = await adminDao.createProvider(orgID, payload);
        let providerData = {
            orgId: provider[0].dataValues.ORG_ID,
            name: provider[0].dataValues.NAME,
        };
        for (const prop of provider) {
            providerData[prop.dataValues.PROPERTY] = prop.dataValues.VALUE;
        }
        res.status(201).send(providerData);
    } catch (error) {
        logger.error('Provider creation failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID,
            providerName: payload?.name
        });
        util.handleError(res, error);
    }
}

const updateProvider = async (req, res) => {
    try {
        const orgId = req.params.orgId;
        const payload = req.body;
        if (!orgId) {
            logger.warn('Missing required parameter: orgId');
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const rules = util.validateProvider();

        for (let validation of rules) {
            await validation.run(req);
        }
        const errors = validationResult(req);
        if (!errors.isEmpty()) {
            return res.status(400).json(util.getErrors(errors));
        }
        const extraKeys = util.rejectExtraProperties(['name', 'providerURL'], payload)
        if (extraKeys.length > 0) {
            return res.status(400).json(new CustomError(400, "Bad Request", `Unexpected properties: ${extraKeys.join(', ')}`));
        }
        const provider = await adminDao.updateProvider(orgId, payload);
        let providerData = {
            orgId: provider[0][0].dataValues.ORG_ID,
            name: provider[0][0].dataValues.NAME,
        };
        for (const prop of provider) {
            providerData[prop[0].dataValues.PROPERTY] = prop[0].dataValues.VALUE;
        }
        res.status(200).json(providerData);
    } catch (error) {
        logger.error('Provider update failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params.orgId
        });
        util.handleError(res, error);
    }
}

const getProviders = async (req, res) => {
    const orgId = req.params.orgId;
    try {

        if (req.query.name) {
            const providerName = req.query.name;
            return res.status(200).send(await getProvidetByName(orgId, providerName));
        } else {
            const providerList = await getAllProviders(orgId);
            return res.status(200).send(providerList);
        }
    } catch (error) {
        logger.error('Provider fetch failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            providerName: req.query?.name
        });
        util.handleError(res, error);
    }
}

const getProvidetByName = async (orgID, name) => {

    const providerData = await adminDao.getProvider(orgID, name);
    if (providerData.length > 0) {
        const providerResponse = {
            name: providerData[0].dataValues.NAME,
        };
        for (const provider of providerData) {
            providerResponse[provider.dataValues.PROPERTY] = provider.dataValues.VALUE;
        }
        return providerResponse;
    }

}

const getAllProviders = async (orgID) => {

    const providers = await adminDao.getProviders(orgID);
    const providerList = [];
    if (providers.length > 0) {
        for (const provider of providers) {
            const providerData = {
                name: provider.dataValues.NAME,
            };
            for (const [key, value] of Object.entries(provider.dataValues.properties)) {
                providerData[key] = value;
            }
            providerList.push(providerData);
        }
    }
    return providerList;
}

const deleteProvider = async (req, res) => {
    const orgId = req.params.orgId;
    try {
        const providerName = req.query.name;
        let property, deletedRowsCount;
        if (req.query.property) {
            property = req.query.property;
            deletedRowsCount = await adminDao.deleteProviderProperty(orgId, property, providerName);
        } else {
            deletedRowsCount = await adminDao.deleteProvider(orgId, providerName);
        }
        if (!orgId || !providerName) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        if (deletedRowsCount > 0) {
            res.status(204).send();
        } else {
            throw new CustomError(404, "Records Not Found", 'Provider property not found');
        }
    } catch (error) {
        logger.error('Provider deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

function checkAdditionalValues(additionalValues) {

    let defaultConfigs = ["application_access_token_expiry_time", "user_access_token_expiry_time", "id_token_expiry_time", "refresh_token_expiry_time"];
    const props = additionalValues;
    for (const key in additionalValues) {
        if (defaultConfigs.includes(key)) {
            props[key] = parseInt(additionalValues[key]);
        }
    }
    return props;

}

const createCPApplicationOnBehalfOfUser = async (cpApplicationName, owner, cpOrgId, patToken) => {
    logger.info('Creating control plane application', {
        cpApplicationName
    });
    let headers = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${patToken}`
    }

    try {
        //create control plane application
        url = `${controlPlaneGwUrl}/applications?preserveOwner=true`;
        const cpAppCreationResponse = await util.apiRequest('POST', url, headers, {
            name: cpApplicationName,
            throttlingPolicy: 'Unlimited',
            tokenType: 'JWT',
            owner: owner,
            groups: [],
            attributes: {},
            subscriptionScopes: []
        }, cpOrgId);
        return cpAppCreationResponse.data;
    } catch (error) {
        //application already exists
        logger.error('Application Creation Failed in CP', {
            error: error.message,
            stack: error.stack,
            cpApplicationName
        });
        if (error.statusCode && error.statusCode === 409) {
            try {
                logger.info('Application already exists in control plane, retrieving existing application', {
                    orgId: cpOrgId,
                    cpApplicationName
                });
                const cpAppResponse = await util.apiRequest('GET', `${controlPlaneGwUrl}/applications?query=${cpApplicationName}`, headers, {}, cpOrgId);
                return cpAppResponse.data.list[0];
            } catch (error) {
                logger.error('Error occurred while fetching application', {
                    error: error.message,
                    cpApplicationName
                });
                throw error;
            }
        } else {
            throw error;
        }
    }
}

const createCPSubscriptionOnBehalfOfUser = async (apiId, cpAppID, policyName, cpOrgId, patToken) => {
    logger.info('Creating control plane subscription', {
        apiId,
        cpAppID,
        policyName
    });
    const headers = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${patToken}`
    }

    try {
        const requestBody = {
            apiId: apiId,
            applicationId: cpAppID,
            throttlingPolicy: policyName
        };
        let url = `${controlPlaneGwUrl}/subscriptions`;
        const cpSubscribeResponse = await util.apiRequest('POST', url, headers, requestBody, cpOrgId);
        return cpSubscribeResponse.data;
    } catch (error) {
        if (error.statusCode && error.statusCode === 409) {
            const response = await util.apiRequest('GET', `${controlPlaneGwUrl}/subscriptions?apiId=${apiId}&applicationId=${cpAppID}`, headers, null, cpOrgId);
            return response.data.list[0];
        }
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            apiId,
            cpAppID
        });
        throw error;
    }
}

const createAppKeyMappingOnBehalfOfUser = async (cpAppID, keymanager, clientId, keyType, cpOrgId, patToken) => {
    logger.debug('Creating control plane application key mapping', {
        cpAppID,
        keymanager,
        clientId,
        keyType
    });
    let headers = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${patToken}`
    }
    try {
        const requestBody = {
            "consumerKey": clientId,
            "keyType": keyType,
            "keyManager": keymanager
        };
        let url = `${controlPlaneGwUrl}/applications/${cpAppID}/map-keys`;
        const cpSubscribeResponse = await util.apiRequest('POST', url, headers, requestBody, cpOrgId);
        return cpSubscribeResponse.data;
    } catch (error) {
        if (error.statusCode && error.statusCode === 409) {
            const response = await util.apiRequest('GET', `${controlPlaneGwUrl}/applications/${cpAppID}/oauth-keys`, headers, null, cpOrgId);

            // Validate response structure
            if (!response.data || !Array.isArray(response.data.list)) {
                throw new CustomError(500, "Internal Server Error", "Invalid response structure from control plane");
            }

            // Validate each key mapping has required fields
            let selectedKeyMapping = null;
            for (const keyMapping of response.data.list) {
                if (keyMapping.keyManager === keymanager && keyMapping.keyType === keyType && keyMapping.consumerKey === clientId) {
                    selectedKeyMapping = keyMapping;
                    break;
                }
            }
            if (!selectedKeyMapping) {
                throw new CustomError(500, "Internal Server Error", "Key Mapping creation failed");
            }
            return selectedKeyMapping;
        }
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            cpAppID
        });
        throw error;
    }
}

const getAPIMKeyManagersBehalfOfUser = async (cpOrgId, patToken) => {

    let headers = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${patToken}`
    }
    let url = `${controlPlaneGwUrl}/key-managers?devPortalAppEnv=prod`;
    const keymanagersResponse = await util.apiRequest('GET', url, headers, null, cpOrgId);

    return keymanagersResponse.data.list;
}

const createCPApplication = async (req, cpApplicationName) => {
    logger.info('Creating control plane application', {
        cpApplicationName
    });
    try {
        //create control plane application
        const cpAppCreationResponse = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications`, {
            'Content-Type': 'application/json'
        }, {
            name: cpApplicationName,
            throttlingPolicy: 'Unlimited',
            tokenType: 'JWT',
            groups: [],
            attributes: {},
            subscriptionScopes: []
        });
        return cpAppCreationResponse;
    } catch (error) {
        //application already exists
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            cpApplicationName
        });
        if (error.statusCode && error.statusCode === 409) {
            try {
                logger.info('Application already exists in control plane, retrieving existing application', {
                    orgId: req.params?.orgId,
                    cpApplicationName
                });
                const cpAppResponse = await invokeApiRequest(req, 'GET', `${controlPlaneUrl}/applications?query=${cpApplicationName}`, {}, {});
                return cpAppResponse.list[0];
            } catch (error) {
                logger.error('Error occurred while fetching application', {
                    error: error.message,
                    cpApplicationName
                });
                throw error;
            }
        } else {
            throw error;
        }
    }
}

const createCPSubscription = async (req, apiId, cpAppID, policyDetails, billingData = null) => {
    logger.info('Creating control plane subscription', {
        apiId,
        cpAppID,
        policyDetails: policyDetails.dataValues ? policyDetails.dataValues.POLICY_NAME : policyDetails,
        billingData: billingData ? { customerId: billingData.customerId, subscriptionId: billingData.subscriptionId } : null
    });
    try {
        const requestBody = {
            apiId: apiId,
            applicationId: cpAppID,
            throttlingPolicy: policyDetails.dataValues ? policyDetails.dataValues.POLICY_NAME : policyDetails
        };

        // Add billing metadata if available (for paid subscriptions)
        if (billingData) {
            requestBody.billingMetadata = {
                billingCustomerId: billingData.customerId,
                billingSubscriptionId: billingData.subscriptionId,
            };
        }

        const cpSubscribeResponse = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/subscriptions`, {}, requestBody);
        return cpSubscribeResponse;
    } catch (error) {
        if (error.statusCode && error.statusCode === 409) {
            const response = await invokeApiRequest(req, 'GET', `${controlPlaneUrl}/subscriptions?apiId=${apiId}&applicationId=${cpAppID}`, {});
            return response.list[0];
        }
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            apiId,
            cpAppID
        });
        throw error;
    }
}

const getApplicationKeyMap = async (orgId, appId, userId) => {

    const appIDResponse = await adminDao.getApplication(orgId, appId, userId);
    if (!appIDResponse) {
        throw new CustomError(404, "Records Not Found", 'Application not found');
    }
    const appKeyMappings = await adminDao.getKeyMapping(orgId, appId);
    if (appKeyMappings) {
        const appMappingDTO = new ApplicationDTO(appKeyMappings);
        return appMappingDTO;
    } else {
        const application = await adminDao.getApplication(orgId, appId, userId);
        return new ApplicationDTO(application.dataValues);
    }

}

function parseApplicationDataFromRequest(req) {
    const file = req.files?.application?.[0];
    if (file?.buffer) {
        let parsed;
        try {
            parsed = yaml.load(file.buffer.toString('utf8'));
        } catch (e) {
            throw new CustomError(400, "Bad Request", `Invalid application YAML: ${e.message}`);
        }
        if (!parsed || typeof parsed !== 'object') {
            throw new CustomError(400, "Bad Request", "Invalid application YAML: expected an object");
        }
        const spec = parsed.spec || {};
        const name = spec.displayName || parsed.metadata?.name;
        if (!name) {
            throw new CustomError(400, "Bad Request", "Missing application name");
        }
        if (!spec.description) {
            throw new CustomError(400, "Bad Request", "Missing required application field: description");
        }
        return {
            name,
            description: spec.description,
            type: "WEB"
        };
    }
    return req.body;
}

module.exports = {
    createOrganization,
    updateOrganization,
    deleteOrganization,
    createOrgContent,
    updateOrgContent,
    getOrgContent,
    deleteOrgContent,
    deleteAllOrgContent,
    createIdentityProvider,
    updateIdentityProvider,
    getIdentityProvider,
    deleteIdentityProvider,
    getOrganizations,
    getAllOrganizations,
    createProvider,
    updateProvider,
    getProviders,
    getAllProviders,
    deleteProvider,
    getProvidetByName,
    getApplicationKeyMap,
    checkAdditionalValues,
    createCPApplication,
    createCPSubscription,
    createCPApplicationOnBehalfOfUser,
    createCPSubscriptionOnBehalfOfUser,
    createAppKeyMappingOnBehalfOfUser,
    getAPIMKeyManagersBehalfOfUser
};

