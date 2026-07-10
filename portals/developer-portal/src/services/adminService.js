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
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
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
const kmDao = require('../dao/keyManagerDao');

function mapYamlToOrganization(parsed) {
    const { metadata = {}, spec = {} } = parsed;
    return {
        handle: metadata.name,
        displayName: spec.displayName,
        idpRefId: spec.idpRefId,
        cpRefId: spec.cpRefId,
        businessOwner: spec.businessOwner,
        businessOwnerContact: spec.businessOwnerContact,
        businessOwnerEmail: spec.businessOwnerEmail,
        configuration: spec.configuration || null,
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
        if (!Array.isArray(spec.labels) || spec.labels.some(l => typeof l !== 'object' || !l.id)) {
            throw new Sequelize.ValidationError("Invalid organization YAML: 'spec.labels' must be an array of objects with an 'id' field");
        }
    }
    if (spec.views !== undefined && spec.views !== null) {
        if (!Array.isArray(spec.views) || spec.views.some(v => typeof v !== 'object' || typeof v.id !== 'string' || !v.id.trim())) {
            throw new Sequelize.ValidationError("Invalid organization YAML: 'spec.views' must be an array of objects with a non-empty 'id' field");
        }
    }
    const organization = mapYamlToOrganization(parsed);
    // Required-field validation for the YAML upload path. The OpenAPI validator only
    // checks that the multipart file field is present; it cannot inspect the file's
    // contents, so the required fields from OrganizationCreate/UpdateRequest are
    // enforced here. Keep this list in sync with those spec schemas.
    const requiredFields = ['displayName', 'handle', 'idpRefId'];
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
    if (payload.id) {
        payload.handle = payload.id;
    }
    payload.configuration = {
        devportalMode: constants.DEVPORTAL_MODE.DEFAULT,
        ...(payload.configuration || {}),
    };
    const userId = util.resolveActor(req);
    payload.createdBy = userId;

    let organization = "";
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            organization = await orgDao.create(payload, t);
            const orgId = organization.uuid;
            logger.info('Organization created successfully', {
                orgId,
                orgName: organization.display_name
            });

            // Labels: use YAML-defined if provided, else fall back to default
            const labelDefs = payload.labels?.length
                ? payload.labels
                : [{ id: 'default', displayName: 'default' }];

            const createdLabels = await labelDao.createMany(orgId, labelDefs.map(l => ({ ...l, handle: l.id })), userId, t);
            logger.info('Labels created successfully', { orgId });

            // Build handle→UUID map for view→label linking
            const labelMap = {};
            createdLabels.forEach(l => { labelMap[l.dataValues.handle] = l.dataValues.uuid; });

            // Views: use YAML-defined if provided, else fall back to default
            if (payload.views?.length) {
                for (const viewDef of payload.views) {
                    if (!viewDef.id || typeof viewDef.id !== 'string') {
                        throw new Sequelize.ValidationError(
                            "Invalid organization payload: each entry in 'views' must have a non-empty 'id'"
                        );
                    }
                }
            }
            const viewDefs = (payload.views?.length
                ? payload.views
                : [{ id: 'default', displayName: 'default', labels: [labelDefs[0].id] }]
            ).map(v => ({ ...v, handle: v.id }));

            for (const viewDef of viewDefs) {
                const viewResponse = await viewDao.create(orgId, viewDef, userId, t);
                const viewId = viewResponse.dataValues.uuid;
                for (const lName of (viewDef.labels || [])) {
                    const labelId = labelMap[lName];
                    if (!labelId) {
                        throw new Sequelize.ValidationError(
                            `Invalid organization YAML: view '${viewDef.id}' references unknown label '${lName}'`
                        );
                    }
                    await labelDao.addToView(orgId, labelId, viewId, userId, t);
                }
            }
            logger.info('Views created successfully', { orgId });

            //store default subscription plans
            if (config.organization.autoCreateSubscriptionPlans) {
                await subscriptionPlanDao.createMany(orgId, constants.DEFAULT_SUBSCRIPTION_PLANS, userId, t);
            }
            logger.info('Default subscription plans created successfully', {
                orgId
            });

        });

        let orgAudit;
        try {
            orgAudit = await userIdpReferenceDao.buildSingleAuditFields(organization.dataValues);
        } catch (auditError) {
            logger.error('Audit field resolution failed after organization creation', {
                error: auditError.message,
                orgId: organization.handle
            });
            orgAudit = { createdAt: organization.dataValues.created_at, updatedAt: organization.dataValues.updated_at };
        }
        const orgCreationResponse = {
            id: organization.handle,
            displayName: organization.display_name,
            businessOwner: organization.business_owner,
            businessOwnerContact: organization.business_owner_contact,
            businessOwnerEmail: organization.business_owner_email,
            idpRefId: organization.idp_ref_id,
            cpRefId: organization.cp_ref_id,
            configuration: organization.dataValues.configuration,
            ...orgAudit,
        };
        logger.info('Organization creation flow completed successfully', {
            orgId: orgCreationResponse.id,
            orgName: orgCreationResponse.displayName,
        });
        logUserAction('ORG_CREATED', req, {
            orgId: orgCreationResponse.id,
            orgName: orgCreationResponse.displayName,
            resourceUuid: organization.uuid,
            resourceType: 'organization',
            orgUuid: organization.uuid,
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
        res.status(200).json(util.toPaginatedList(orgList, req));
    } catch (error) {
        util.handleError(res, error);
    }
};

const getAllOrganizations = async () => {
    const organizations = await orgDao.list();
    const orgList = [];
    if (organizations.length > 0) {
        const auditList = await userIdpReferenceDao.buildListAuditFields(organizations.map(o => o.dataValues));
        organizations.forEach((organization, i) => {
            orgList.push({
                displayName: organization.dataValues.display_name,
                id: organization.dataValues.handle,
                businessOwner: organization.dataValues.business_owner,
                businessOwnerContact: organization.dataValues.business_owner_contact,
                businessOwnerEmail: organization.dataValues.business_owner_email,
                idpRefId: organization.idp_ref_id,
                cpRefId: organization.cp_ref_id,
                configuration: organization.dataValues.configuration,
                ...auditList[i],
            });
        });
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
        if (payload.id) {
            payload.handle = payload.id;
        }
        payload.orgId = orgId;
        const userId = util.resolveActor(req);
        payload.updatedBy = userId;

        const devportalMode = payload.configuration?.devportalMode;
        if (devportalMode !== undefined && !Object.values(constants.DEVPORTAL_MODE).includes(devportalMode)) {
            return res.status(400).json({ error: `Invalid devportalMode '${devportalMode}'. Must be one of: ${Object.values(constants.DEVPORTAL_MODE).join(', ')}.` });
        }

        let updatedOrg;
        await sequelize.transaction({ timeout: 60000 }, async (t) => {
            const existingOrg = await orgDao.get(orgId, t);
            const resolvedOrgId = existingOrg.uuid;
            [, updatedOrg] = await orgDao.update(payload, t);
            logger.info('Organization update successful', { orgId });

            // Labels upsert — only if present in payload
            if (payload.labels?.length) {
                for (const label of payload.labels) {
                    await labelDao.update(resolvedOrgId, { ...label, handle: label.id }, userId, t);
                }
                logger.info('Labels upserted successfully', { orgId });
            }

            // Views upsert — only if present in payload
            if (payload.views?.length) {
                for (const viewDef of payload.views) {
                    if (!viewDef.id || typeof viewDef.id !== 'string') {
                        throw new Sequelize.ValidationError(
                            "Invalid organization payload: each entry in 'views' must have a non-empty 'id'"
                        );
                    }
                    const view = await viewDao.update(resolvedOrgId, viewDef.id, viewDef.displayName, userId, t);
                    if (Array.isArray(viewDef.labels)) {
                        await viewDao.replaceLabels(resolvedOrgId, view.dataValues.uuid, viewDef.labels, userId, t);
                    }
                }
                logger.info('Views upserted successfully', { orgId });
            }
        });

        let updatedOrgAudit;
        try {
            updatedOrgAudit = await userIdpReferenceDao.buildSingleAuditFields(updatedOrg[0].dataValues);
        } catch (auditError) {
            logger.error('Audit field resolution failed after organization update', {
                error: auditError.message,
                orgId
            });
            updatedOrgAudit = { createdAt: updatedOrg[0].dataValues.created_at, updatedAt: updatedOrg[0].dataValues.updated_at };
        }
        res.status(200).json({
            id: updatedOrg[0].dataValues.handle,
            displayName: updatedOrg[0].dataValues.display_name,
            businessOwner: updatedOrg[0].dataValues.business_owner,
            businessOwnerContact: updatedOrg[0].dataValues.business_owner_contact,
            businessOwnerEmail: updatedOrg[0].dataValues.business_owner_email,
            idpRefId: updatedOrg[0].dataValues.idp_ref_id,
            cpRefId: updatedOrg[0].dataValues.cp_ref_id,
            configuration: updatedOrg[0].dataValues.configuration,
            ...updatedOrgAudit,
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
        // Resolved before delete: dp_audit.org_uuid has ON DELETE CASCADE, so once the
        // org is gone this uuid can no longer satisfy that FK — the ORG_DELETED audit
        // insert below will be dropped (caught, logged, non-fatal), same limitation
        // platform-api's own audit table has for its own org-delete cascade.
        const orgUuid = await orgDao.getId(orgId);
        const deletedRowsCount = await sequelize.transaction({ timeout: 60000 }, (t) => orgDao.delete(orgId, t));
        if (deletedRowsCount > 0) {
            logger.info('Organization deletion successful', {
                orgId
            });
            logUserAction('ORG_DELETED', req, { orgId, resourceUuid: orgUuid, resourceType: 'organization', orgUuid });
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

const createContent = async (filePath, fileName, fileContent, fileType, orgId, viewName, userId, t) => {
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
            }, t);
        }
    } catch (error) {
        throw error;
    }
    return content;
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

const applyTheme = async (req, res) => {
    const orgId = req.orgId;
    const viewName = req.params.viewId;
    const zipFile = req.files?.file?.[0] ?? req.file;
    const userId = util.resolveActor(req);
    const extractPath = path.join(process.cwd(), '..', '.tmp', `${orgId}-${viewName}-${Date.now()}`);
    let tempZipPath;
    try {
        if (!zipFile) {
            throw new CustomError(400, 'Bad Request', 'Missing required zip file');
        }
        const maxUploadBytes = config.uploads?.maxBytes || 10485760;
        if (zipFile.size > maxUploadBytes) {
            throw new CustomError(413, 'Payload Too Large', 'Uploaded file exceeds the maximum allowed size.');
        }
        let zipPath = zipFile.path;
        if (!zipPath && zipFile.buffer) {
            tempZipPath = path.join(require('os').tmpdir(), `org-content-${orgId}-${Date.now()}.zip`);
            fs.writeFileSync(tempZipPath, zipFile.buffer);
            zipPath = tempZipPath;
        }
        await util.unzipDirectory(zipPath, extractPath);
        const files = await util.readFilesInDirectory(extractPath, orgId, req.protocol, req.get('host'), viewName);
        await sequelize.transaction(async (t) => {
            await orgDao.deleteAllContent(orgId, viewName, t);
            for (const { filePath, fileName, fileContent, fileType } of files) {
                await createContent(filePath, fileName, fileContent, fileType, orgId, viewName, userId, t);
            }
        });
        fs.rmSync(extractPath, { recursive: true, force: true });
        if (tempZipPath) fs.rmSync(tempZipPath, { force: true });
        const organization = await orgDao.getByUuid(orgId);
        res.status(200).json({ id: organization.handle, fileName: zipFile.originalname });
    } catch (error) {
        logger.error('Apply theme failed', { error: error.message, stack: error.stack, orgId, viewName });
        fs.rmSync(extractPath, { recursive: true, force: true });
        if (tempZipPath) fs.rmSync(tempZipPath, { force: true });
        util.handleError(res, error);
    }
};

const resetTheme = async (req, res) => {
    const orgId = req.orgId;
    const viewName = req.params.viewId;
    try {
        await orgDao.deleteAllContent(orgId, viewName);
        res.status(204).send();
    } catch (error) {
        logger.error('Reset theme failed', { error: error.message, stack: error.stack, orgId, viewName });
        util.handleError(res, error);
    }
};

module.exports = {
    createOrganization,
    updateOrganization,
    deleteOrganization,
    getOrgContent,
    applyTheme,
    resetTheme,
    getOrganizations,
    getAllOrganizations,
    getApplicationKeyMap,
    checkAdditionalValues
};
