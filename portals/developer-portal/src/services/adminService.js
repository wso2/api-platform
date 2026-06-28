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
const orgDao = require('../dao/organizationDao');
const appDao = require('../dao/applicationDao');
const apiDao = require('../dao/apiDao');
const labelDao = require('../dao/labelDao');
const viewDao = require('../dao/viewDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const util = require('../utils/util');
const fs = require('fs');
const path = require('path');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const constants = require('../utils/constants');
const sequelize = require('../db/sequelizeConfig');
const { ApplicationDTO } = require('../dto/applicationDto');
const APIDTO = require('../dto/apiDto');
const { config } = require('../config/configLoader');
const yaml = require('js-yaml');
const { Sequelize } = require("sequelize");
const { trackGenerateCredentials, trackSubscribeApi, trackUnsubscribeApi } = require('../utils/telemetryUtil');
const kmDao = require('../dao/keyManagerDao');
const { getKeyManagerAdapter } = require('../adapters/keyManager');

function mapYamlToOrganization(parsed) {
    const { metadata = {}, spec = {} } = parsed;
    return {
        orgHandle: metadata.name,
        orgName: spec.displayName,
        organizationIdentifier: spec.organizationIdentifier,
        cpRefId: spec.cpRefId,
        businessOwner: spec.businessOwner,
        businessOwnerContact: spec.businessOwnerContact,
        businessOwnerEmail: spec.businessOwnerEmail,
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
        if (!Array.isArray(spec.views) || spec.views.some(v => typeof v !== 'object' || !v.handle)) {
            throw new Sequelize.ValidationError("Invalid organization YAML: 'spec.views' must be an array of objects with a 'handle' field");
        }
    }
    const organization = mapYamlToOrganization(parsed);
    // Required-field validation for the YAML upload path. The OpenAPI validator only
    // checks that the multipart file field is present; it cannot inspect the file's
    // contents, so the required fields from OrganizationCreate/UpdateRequest are
    // enforced here. Keep this list in sync with those spec schemas.
    const requiredFields = ['orgName', 'orgHandle', 'organizationIdentifier'];
    const missingFields = requiredFields.filter((field) => !organization[field]);
    if (missingFields.length > 0) {
        throw new Sequelize.ValidationError(
            `Invalid organization YAML: missing required field(s): ${missingFields.join(', ')}`
        );
    }
    return organization;
}

const createOrganization = async (req, res) => {
    if (req.files?.organization?.[0]) {
        try {
            req.body = parseOrganizationFromYamlFile(req.files.organization[0].buffer);
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    logger.info('Initiate organization creation...');

    const payload = req.body;
    payload.orgConfig = {
        devportalMode: constants.DEVPORTAL_MODE.DEFAULT,
    };
    const userId = util.resolveActor(req);
    payload.createdBy = userId;

    let organization = "";
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            organization = await orgDao.create(payload, t);
            const orgId = organization.UUID;
            logger.info('Organization created successfully', {
                orgId,
                orgName: organization.NAME
            });

            // Labels: use YAML-defined if provided, else fall back to default
            const labelDefs = payload.labels?.length
                ? payload.labels
                : [{ name: 'default', displayName: 'default' }];

            const createdLabels = await labelDao.createMany(orgId, labelDefs, userId, t);
            logger.info('Labels created successfully', { orgId });

            // Build name→UUID map for view→label linking
            const labelMap = {};
            createdLabels.forEach(l => { labelMap[l.dataValues.NAME] = l.dataValues.UUID; });

            // Views: use YAML-defined if provided, else fall back to default
            const viewDefs = payload.views?.length
                ? payload.views
                : [{ handle: 'default', name: 'default', labels: [labelDefs[0].name] }];

            for (const viewDef of viewDefs) {
                const viewResponse = await viewDao.create(orgId, viewDef, userId, t);
                const viewID = viewResponse.dataValues.UUID;
                for (const lName of (viewDef.labels || [])) {
                    const labelId = labelMap[lName];
                    if (!labelId) {
                        throw new Sequelize.ValidationError(
                            `Invalid organization YAML: view '${viewDef.handle}' references unknown label '${lName}'`
                        );
                    }
                    await labelDao.addToView(orgId, labelId, viewID, userId, t);
                }
            }
            logger.info('Views created successfully', { orgId });

            //store default subscription plans
            if (config.generateDefaultSubPlans) {
                await subscriptionPlanDao.createMany(orgId, constants.DEFAULT_SUBSCRIPTION_PLANS, userId, t);
            }
            logger.info('Default subscription plans created successfully', {
                orgId
            });

        });

        const orgCreationResponse = {
            orgId: organization.UUID,
            orgName: organization.NAME,
            businessOwner: organization.BUSINESS_OWNER,
            businessOwnerContact: organization.BUSINESS_OWNER_CONTACT,
            businessOwnerEmail: organization.BUSINESS_OWNER_EMAIL,
            orgHandle: organization.HANDLE,
            organizationIdentifier: organization.IDP_REF_ID,
            cpRefId: organization.CP_REF_ID,
            orgConfiguration: organization.dataValues.CONFIGURATION
        };
        logger.info('Organization creation flow completed successfully', {
            orgId: orgCreationResponse.orgId,
            orgName: orgCreationResponse.orgName,
        });
        logUserAction('ORG_CREATED', req, { orgId: orgCreationResponse.orgId, orgName: orgCreationResponse.orgName });
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
        res.status(200).json(util.toPaginatedList(orgList, req));
    } catch (error) {
        util.handleError(res, error);
    }
};

const getAllOrganizations = async () => {
    const organizations = await orgDao.list();
    const orgList = [];
    if (organizations.length > 0) {
        for (const organization of organizations) {
            orgList.push({
                orgName: organization.dataValues.NAME,
                orgID: organization.dataValues.UUID,
                businessOwner: organization.dataValues.BUSINESS_OWNER,
                businessOwnerContact: organization.dataValues.BUSINESS_OWNER_CONTACT,
                businessOwnerEmail: organization.dataValues.BUSINESS_OWNER_EMAIL,
                orgHandle: organization.HANDLE,
                organizationIdentifier: organization.IDP_REF_ID,
                cpRefId: organization.CP_REF_ID,
                orgConfiguration: organization.dataValues.CONFIGURATION
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
        const payload = req.body;
        payload.orgId = orgId;
        const userId = util.resolveActor(req);
        payload.updatedBy = userId;

        const devportalMode = payload.orgConfiguration?.devportalMode;
        if (devportalMode !== undefined && !Object.values(constants.DEVPORTAL_MODE).includes(devportalMode)) {
            return res.status(400).json({ error: `Invalid devportalMode '${devportalMode}'. Must be one of: ${Object.values(constants.DEVPORTAL_MODE).join(', ')}.` });
        }

        let updatedOrg;
        await sequelize.transaction({ timeout: 60000 }, async (t) => {
            [, updatedOrg] = await orgDao.update(payload, t);
            logger.info('Organization update successful', { orgId });

            // Labels upsert — only if present in payload
            if (payload.labels?.length) {
                for (const label of payload.labels) {
                    await labelDao.update(orgId, label, userId, t);
                }
                logger.info('Labels upserted successfully', { orgId });
            }

            // Views upsert — only if present in payload
            if (payload.views?.length) {
                for (const viewDef of payload.views) {
                    if (!viewDef.handle || typeof viewDef.handle !== 'string') {
                        throw new Sequelize.ValidationError(
                            "Invalid organization payload: each entry in 'views' must have a non-empty 'handle'"
                        );
                    }
                    const view = await viewDao.update(orgId, viewDef.handle, viewDef.name, userId, t);
                    if (Array.isArray(viewDef.labels)) {
                        await viewDao.replaceLabels(orgId, view.dataValues.UUID, viewDef.labels, userId, t);
                    }
                }
                logger.info('Views upserted successfully', { orgId });
            }
        });

        res.status(200).json({
            orgId: updatedOrg[0].dataValues.UUID,
            orgName: updatedOrg[0].dataValues.NAME,
            businessOwner: updatedOrg[0].dataValues.BUSINESS_OWNER,
            businessOwnerContact: updatedOrg[0].dataValues.BUSINESS_OWNER_CONTACT,
            businessOwnerEmail: updatedOrg[0].dataValues.BUSINESS_OWNER_EMAIL,
            orgHandle: updatedOrg[0].dataValues.HANDLE,
            organizationIdentifier: updatedOrg[0].dataValues.IDP_REF_ID,
            cpRefId: updatedOrg[0].dataValues.CP_REF_ID,
            orgConfiguration: updatedOrg[0].dataValues.CONFIGURATION
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
        const deletedRowsCount = await orgDao.delete(orgId);
        if (deletedRowsCount > 0) {
            logger.info('Organization deletion successful', {
                orgId
            });
            logUserAction('ORG_DELETED', req, { orgId });
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

const createOrgContent = async (req, res) => {
    const orgId = req.params.orgId;
    const viewName = req.params.viewName;
    const zipFile = req.files?.file?.[0] ?? req.file;
    const userId = util.resolveActor(req);
    logger.info('Initiate create organization content...', {
        orgId,
        viewName
    });

    const extractPath = path.join(process.cwd(), '..', '.tmp', orgId);
    let tempZipPath;

    try {
        if (!zipFile) {
            throw new CustomError(400, "Bad Request", "Missing required zip file");
        }
        if (zipFile.size > 50 * 1024 * 1024) {
            throw new CustomError(400, "Bad Request", "File size exceeds the 50MB limit");
        }
        let zipPath = zipFile.path;
        if (!zipPath && zipFile.buffer) {
            tempZipPath = path.join(require('os').tmpdir(), `org-content-${orgId}-${Date.now()}.zip`);
            fs.writeFileSync(tempZipPath, zipFile.buffer);
            zipPath = tempZipPath;
        }
        await util.unzipDirectory(zipPath, extractPath);
        const files = await util.readFilesInDirectory(extractPath, orgId, req.protocol, req.get('host'), viewName);
        for (const { filePath, fileName, fileContent, fileType } of files) {
            await createContent(filePath, fileName, fileContent, fileType, orgId, viewName, userId);
        }
        logger.info('Organization content created successfully', {
            orgId,
            viewName
        });
        res.status(201).send({ "orgId": orgId, "fileName": zipFile.originalname });
        fs.rmSync(extractPath, { recursive: true, force: true });
        if (tempZipPath) fs.rmSync(tempZipPath, { force: true });

    } catch (error) {
        logger.error('Organization content creation failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            viewName,
            fileName: zipFile?.originalname
        });
        fs.rmSync(extractPath, { recursive: true, force: true });
        if (tempZipPath) fs.rmSync(tempZipPath, { force: true });
        return util.handleError(res, error);
    }
};

const createContent = async (filePath, fileName, fileContent, fileType, orgId, viewName, userId) => {
    let content;
    // eslint-disable-next-line no-useless-catch
    try {
        if (fileName != null && !fileName.startsWith('.')) {
            content = await orgDao.createContent({
                fileType: fileType,
                fileName: fileName,
                fileContent: fileContent,
                filePath: filePath,
                orgId: orgId,
                viewName: viewName,
                createdBy: userId
            });
        }
    } catch (error) {
        throw error;
    }
    return content;
};

const updateOrgContent = async (req, res) => {
    const orgId = req.params.orgId;
    const viewName = req.params.viewName;
    const zipFile = req.files?.file?.[0] ?? req.file;
    const userId = util.resolveActor(req);
    logger.info('Initiate update organization content...', {
        orgId,
        viewName
    });
    const extractPath = path.join(process.cwd(), '..', '.tmp', orgId);
    let tempZipPath;
    try {
        if (!zipFile) {
            throw new CustomError(400, "Bad Request", "Missing required zip file");
        }
        if (zipFile.size > 50 * 1024 * 1024) {
            throw new CustomError(400, "Bad Request", "File size exceeds the 50MB limit");
        }
        let zipPath = zipFile.path;
        if (!zipPath && zipFile.buffer) {
            tempZipPath = path.join(require('os').tmpdir(), `org-content-${orgId}-${Date.now()}.zip`);
            fs.writeFileSync(tempZipPath, zipFile.buffer);
            zipPath = tempZipPath;
        }
        await util.unzipDirectory(zipPath, extractPath);
        const files = await util.readFilesInDirectory(extractPath, orgId, req.protocol, req.get('host'), viewName);
        for (const { filePath, fileName, fileContent, fileType } of files) {
            if (fileName != null && !fileName.startsWith('.')) {
                const organizationContent = await getOrgContent(orgId, viewName, fileType, fileName, filePath);
                if (organizationContent) {
                    await orgDao.updateContent({
                        fileType: fileType,
                        fileName: fileName,
                        fileContent: fileContent,
                        filePath: filePath,
                        orgId: orgId,
                        viewName: viewName,
                        createdBy: userId
                    });
                } else {
                    logger.info('Content not found during update, creating new content', {
                        orgId,
                        viewName,
                        fileType,
                        fileName,
                        filePath
                    });
                    await createContent(filePath, fileName, fileContent, fileType, orgId, viewName, userId);
                }
            }
        }
        fs.rmSync(extractPath, { recursive: true, force: true });
        if (tempZipPath) fs.rmSync(tempZipPath, { force: true });
        logger.info('Organization content updated successfully', {
            orgId,
            viewName
        });
        res.status(201).send({ "orgId": orgId, "fileName": zipFile.originalname });
    } catch (error) {
        logger.error('Organization content update failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            viewName,
            fileName: zipFile?.originalname
        });
        fs.rmSync(extractPath, { recursive: true, force: true });
        if (tempZipPath) fs.rmSync(tempZipPath, { force: true });
        util.handleError(res, error);
    }
};

const getOrgContent = async (orgId, viewName, fileType, fileName, filePath) => {

    return await orgDao.getContent({
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
        viewName: req.params.viewName
    });
    try {
        const fileName = req.query.fileName;
        let deletedRowsCount;
        if (!req.query.fileName) {
            deletedRowsCount = await orgDao.deleteAllContent(orgId, req.params.viewName);
        } else {
            deletedRowsCount = await orgDao.deleteContent(orgId, req.params.viewName, fileName);
        }
        if (deletedRowsCount > 0) {
            logger.info('Organization content deletion successful', {
                orgId,
                viewName: req.params.viewName
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
        viewName: req.params.viewName
    });
    try {
        const deletedRowsCount = await orgDao.deleteAllContent(orgId, req.params.viewName, fileName);
        if (deletedRowsCount > 0) {
            logger.info('All organization content deletion successful', {
                orgId,
                viewName: req.params.viewName
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
            viewName: req.params.viewName
        });
        util.handleError(res, error);
    }
};

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

const getApplicationKeyMap = async (orgId, appId, userId) => {

    const appIDResponse = await appDao.get(orgId, appId, userId);
    if (!appIDResponse) {
        throw new CustomError(404, "Records Not Found", 'Application not found');
    }
    const appKeyMappings = await appDao.getKeyMapping(orgId, appId);
    if (appKeyMappings) {
        const appMappingDTO = new ApplicationDTO(appKeyMappings);
        return appMappingDTO;
    } else {
        const application = await appDao.get(orgId, appId, userId);
        return new ApplicationDTO(application.dataValues);
    }

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
    getOrganizations,
    getAllOrganizations,
    getApplicationKeyMap,
    checkAdditionalValues
};

