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
const config = require(process.cwd() + '/config.json');
const controlPlaneUrl = config.controlPlane.url;
const { invokeApiRequest } = require('../utils/util');
const { Sequelize } = require("sequelize");
const { trackGenerateCredentials, trackSubscribeApi, trackUnsubscribeApi } = require('../utils/telemetry');

const createOrganization = async (req, res) => {
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
        devportalMode: constants.API_TYPE.DEFAULT,
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

            // create default label
            const labels = await apiDao.createLabels(organization.ORG_ID, [{ name: 'default', displayName: 'default' }], t);
            const labelId = labels[0].dataValues.LABEL_ID;
            logger.info('Default label created successfully', {
                orgId
            });

            //create default view
            const viewResponse = await apiDao.addView(orgId, { name: 'default', displayName: 'default' }, t);
            const viewID = viewResponse.dataValues.VIEW_ID;
            logger.info('Default view created successfully', {
                orgId
            });

            await apiDao.addLabel(orgId, labelId, viewID, t);
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
        const [, updatedOrg] = await adminDao.updateOrganization(payload);
        logger.info('Organization update successful', {
            orgId
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

const createDevPortalApplication = async (req, res) => {
    const orgId = req.params.orgId;
    logger.info('Initiate create application...', {
        orgId: orgId,
        ...req.body
    });
    try {
        const userID = req[constants.USER_ID]
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const applicationData = req.body;
        try {
            const application = await adminDao.createApplication(orgId, userID, applicationData);
            res.status(201).send(new ApplicationDTO(application.dataValues));
        } catch (error) {
            logger.error('Provider creation failed during application creation', {
                error: error.message,
                orgId
            });
            util.handleError(res, error);
        }
    } catch (error) {
        logger.error('Application creation failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            applicationName: req.body?.name
        });
        util.handleError(res, error);
    }
}

const updateDevPortalApplication = async (req, res) => {
    const { orgId, appId } = req.params;
    logger.info('Initiate update application...', {
        orgId: orgId,
        appId: appId,
        ...req.body
    });
    try {
        const userId = req[constants.USER_ID]
        const applicationData = req.body;
        if (!orgId) {
            logger.warn('Missing required parameter: orgId');
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
        }
        const [updatedRows, updatedApp] = await adminDao.updateApplication(orgId, appId, userId, applicationData);
        if (!updatedRows) {
            throw new Sequelize.EmptyResultError("No record found to update");
        }
        res.status(200).send(new ApplicationDTO(updatedApp[0].dataValues));
    } catch (error) {
        logger.error('Application update failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            applicationId: req.params.applicationId
        });
        util.handleError(res, error);
    }
}

const getDevPortalApplications = async (req, res) => {

    const orgID = req.params.orgId;
    const userID = req[constants.USER_ID]

    try {
        const applications = await adminDao.getApplications(orgID, userID);
        // Create response object
        if (applications.length > 0) {
            const appResponse = applications.map((app) => new ApplicationDTO(app));
            res.status(200).send(appResponse);
        } else {
            throw new CustomError(404, "Records Not Found", 'Applications not found');
        }
    } catch (error) {
        logger.error('Application retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID
        });
        util.handleError(res, error);
    }
}

const getAllApplications = async (orgID, userID) => {

    const applications = await adminDao.getApplications(orgID, userID);
    let appList = [];
    // Create response object
    if (applications.length > 0) {
        appList = applications.map((app) => new ApplicationDTO(app));
    }
    return appList;
}

const getDevPortalApplicationDetails = async (req, res) => {

    const orgID = req.params.orgId;
    const appID = req.params.appId;
    const userID = req[constants.USER_ID]
    try {
        const application = await adminDao.getApplication(orgID, appID, userID);
        // Create response object
        if (application) {
            const appResponse = new ApplicationDTO(application.dataValues);
            res.status(200).send(appResponse);
        } else {
            throw new CustomError(404, "Records Not Found", 'Applications not found');
        }
    } catch (error) {
        logger.error('Application retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID
        });
        util.handleError(res, error);
    }

}

const deleteDevPortalApplication = async (req, res) => {
    const { orgId, appId } = req.params;
    logger.info('Initiate delete application...', {
        orgId: orgId,
        appId: appId
    });
    const userID = req[constants.USER_ID]
    try {
        const appDeleteResponse = await adminDao.deleteApplication(orgId, appId, userID);
        if (appDeleteResponse === 0) {
            throw new Sequelize.EmptyResultError("Resource not found to delete");
        } else {
            res.status(200).send("Resouce Deleted Successfully");
        }
    } catch (error) {
        logger.error('Application deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params?.orgId,
            applicationId: req.params?.applicationId
        });
        util.handleError(res, error);
    }
}

const createSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    logger.info('Initiate create subscription...', {
        orgId: orgID,
        ...req.body
    });
    try {
        let isShared;
        let sharedApp = [];
        let nonSharedApp = [];
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            try {
                sharedApp = await adminDao.getApplicationKeyMapping(orgID, req.body.applicationID, true);
                nonSharedApp = await adminDao.getApplicationKeyMapping(orgID, req.body.applicationID, false);
                if (sharedApp.length > 0) {
                    isShared = true;
                    const response = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/subscriptions`, {}, {
                        apiId: req.body.apiReferenceID,
                        applicationId: sharedApp[0].dataValues.CP_APP_REF,
                        throttlingPolicy: req.body.policyName
                    });
                    await handleSubscribe(orgID, req.body.applicationID, sharedApp[0].dataValues.API_REF_ID, sharedApp[0].dataValues.SUBSCRIPTION_REF_ID, response, isShared, t);
                } else if (nonSharedApp.length > 0) {
                    isShared = false;
                    const response = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/subscriptions`, {}, {
                        apiId: req.body.apiReferenceID,
                        applicationId: nonSharedApp[0].dataValues.CP_APP_REF,
                        throttlingPolicy: req.body.policyName
                    });
                    await handleSubscribe(orgID, req.body.applicationID, nonSharedApp[0].dataValues.API_REF_ID, nonSharedApp[0].dataValues.SUBSCRIPTION_REF_ID, response, isShared, t);
                }
                await adminDao.createSubscription(orgID, req.body, t);
                trackSubscribeApi({
                    orgId: orgID,
                    appId: req.body.applicationID,
                    apiId: req.body.apiId,
                    idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
                });
                return res.status(200).json({ message: 'Subscribed successfully' });

            } catch (error) {
                try {
                    if (error.statusCode && error.statusCode === 409) {
                        const appRef = sharedApp.length > 0 ? sharedApp[0] : nonSharedApp[0];
                        const response = await invokeApiRequest(req, 'GET', `${controlPlaneUrl}/subscriptions?applicationId=${appRef.dataValues.CP_APP_REF}`, {}, {});
                        /** Handle both scenario where a reference application in cp is created but no subscriptions avaiable 
                         * (update existing row) & a reference application in cp is created & a subscriptions for a different 
                         * API already exisits (create new row) **/
                        for (const subscription of response.list) {
                            if (subscription.apiId === req.body.apiReferenceID) {
                                logger.info('Subscription already exists in control plane, updating database', {
                                    orgId: req.params?.orgId,
                                    apiId: req.params?.apiId,
                                    applicationId: req.params?.applicationId
                                });
                                await handleSubscribe(orgID, req.body.applicationID, appRef.dataValues.API_REF_ID, appRef.dataValues.SUBSCRIPTION_REF_ID, subscription, sharedApp.length > 0 ? true : false, t);
                                await adminDao.createSubscription(orgID, req.body, t);
                                return res.status(200).json({ message: 'Subscribed successfully' });
                            }
                        }
                    }
                } catch (error) {
                    logger.error('Error occurred while retrieving API subscription', {
                        error: error.message,
                        orgId: req.params?.orgId,
                        apiId: req.params?.apiId,
                        applicationId: req.params?.applicationId
                    });
                    return util.handleError(res, error);
                }

                logger.error('Error occurred while subscribing to API', {
                    error: error.message,
                    orgId: req.params?.orgId,
                    apiId: req.params?.apiId,
                    applicationId: req.params?.applicationId
                });
                return util.handleError(res, error);
            }
        });
    } catch (error) {
        logger.error('Error occurred while subscribing to API', {
            error: error.message,
            orgId: req.params?.orgId,
            apiId: req.params?.apiId,
            applicationId: req.params?.applicationId
        });
        return util.handleError(res, error);
    }
}

const updateSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    logger.info('Initiate update subscription...', {
        orgId: orgID,
        ...req.body
    });
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            try {
                let app = await adminDao.getApplicationKeyMapping(orgID, req.body.applicationID, true);
                if (app.length === 0) {
                    app = await adminDao.getApplicationKeyMapping(orgID, req.body.applicationID, false);
                }
                if (app.length > 0) {
                    const cpAppRef = app[0].dataValues.CP_APP_REF;
                    let appAPIMapping;
                    appAPIMapping = await adminDao.getApplicationAPIMapping(orgID, req.body.applicationID, req.body.apiReferenceID, cpAppRef, true);
                    if (!appAPIMapping.length > 0) {
                        appAPIMapping = await adminDao.getApplicationAPIMapping(orgID, req.body.applicationID, req.body.apiReferenceID, cpAppRef, false);
                    }
                    if (appAPIMapping.length > 0) {
                        let throttlingPolicy = "";
                        const subscruibedPolicy = await apiDao.getSubscriptionPolicy(req.body.policyId, orgID);
                        if (subscruibedPolicy) {
                            throttlingPolicy = subscruibedPolicy.dataValues.POLICY_NAME;
                        }
                        const subscriptionID = appAPIMapping[0].dataValues.SUBSCRIPTION_REF_ID;
                        const response = await invokeApiRequest(req, 'PUT', `${controlPlaneUrl}/subscriptions/${subscriptionID}`, {}, {
                            apiId: req.body.apiReferenceID,
                            applicationId: cpAppRef,
                            requestedThrottlingPolicy: req.body.policyName,
                            subscriptionId: subscriptionID,
                            status: 'UNBLOCKED',
                            throttlingPolicy: throttlingPolicy
                        });
                    }
                }
                await adminDao.updateSubscription(orgID, req.body, t);
                return res.status(201).json({ message: 'Updated subscription successfully' });
            } catch (error) {
                logger.error('Error occurred while subscribing to API', {
                    error: error.message,
                    orgId: req.params?.orgId,
                    apiId: req.params?.apiId,
                    applicationId: req.params?.applicationId
                });
                return util.handleError(res, error);
            }
        });
    } catch (error) {
        logger.error('Error occurred while subscribing to API', {
            error: error.message,
            orgId: req.params?.orgId,
            apiId: req.params?.apiId,
            applicationId: req.params?.applicationId
        });
        return util.handleError(res, error);
    }
}

async function handleSubscribe(orgID, applicationID, apiRefID, subRefID, response, isShared, t) {
    if (apiRefID && subRefID) {
        await adminDao.createApplicationKeyMapping({
            orgID: orgID,
            appID: applicationID,
            cpAppRef: response.applicationId,
            apiRefID: response.apiId,
            subscriptionRefID: response.subscriptionId,
            sharedToken: isShared,
            tokenType: isShared ? constants.TOKEN_TYPES.OAUTH : constants.TOKEN_TYPES.API_KEY
        }, t);
    } else {
        await adminDao.updateApplicationKeyMapping(null, {
            orgID: orgID,
            appID: applicationID,
            cpAppRef: response.applicationId,
            apiRefID: response.apiId,
            subscriptionRefID: response.subscriptionId,
            sharedToken: isShared,
            tokenType: isShared ? constants.TOKEN_TYPES.OAUTH : constants.TOKEN_TYPES.API_KEY
        }, t);

    }
}

const getSubscription = async (req, res) => {

    const orgID = req.params.orgId;
    const subID = req.params.subscriptionId;
    try {
        const subscription = await adminDao.getSubscription(orgID, subID);
        // Create response object
        if (subscription) {
            res.status(200).send(new SubscriptionDTO(subscription.dataValues));
        } else {
            throw new CustomError(404, "Records Not Found", 'Subscriptions not found');
        }
    } catch (error) {
        logger.error('Subscription retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID,
            subId: subID
        });
        util.handleError(res, error);
    }
}

const getAllSubscriptions = async (req, res) => {

    const orgID = req.params.orgId;
    const appID = req.query.appId ? req.query.appId : "";
    const apiID = req.query.apiId ? req.query.apiId : "";
    try {
        const subscriptions = await adminDao.getSubscriptions(orgID, appID, apiID);
        let subList = [];
        // Create response object
        if (subscriptions.length > 0) {
            subList = subscriptions.map((sub) => new SubscriptionDTO(sub));
        }
        res.status(200).send(subList);
    } catch (error) {
        logger.error('Subscription retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID,
            appId: appID,
            apiId: apiID
        });
        util.handleError(res, error);
    }
}

const deleteSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    const subID = req.params.subscriptionId;
    logger.info('Initiate delete subscription...', {
        orgId: orgID,
        subId: subID
    });
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const subscription = await adminDao.getSubscription(orgID, subID, t);
            const subDeleteResponse = await adminDao.deleteSubscription(orgID, subID, t);
            if (subDeleteResponse === 0) {
                throw new Sequelize.EmptyResultError("Resource not found to delete");
            } else {
                //get subscription reference for control plane
                const subIDList = await adminDao.getAPISubscriptionReference(orgID, subscription.dataValues.APP_ID, subscription.dataValues.REFERENCE_ID, t);
                //delete subscription from control plane
                for (const subscription of subIDList) {
                    const subscriptionID = subscription.dataValues?.SUBSCRIPTION_REF_ID;
                    await invokeApiRequest(req, 'DELETE', `${controlPlaneUrl}/subscriptions/${subscriptionID}`, {}, {})
                    await adminDao.deleteAppKeyMapping(orgID, subDeleteResponse.APP_ID, subscriptionID, t);
                }
                res.status(200).send("Resouce Deleted Successfully");
            }
        });
    } catch (error) {
        logger.error('Subscription deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params?.orgId,
            subscriptionId: req.params?.subscriptionId
        });
        util.handleError(res, error);
    }
}

const createAppKeyMapping = async (req, res) => {
    const orgID = req.params.orgId;
    const userID = req[constants.USER_ID];
    logger.info('Initiate create application key mapping...', {
        orgId: orgID,
        ...req.body
    });
    let cpAppID = "";
    try {
        let responseData;
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const { applicationName, apis, tokenType, tokenDetails, provider, clientID } = req.body;
            const appIDResponse = await adminDao.getApplicationID(orgID, userID, applicationName);
            let appID;
            if (appIDResponse) {
                appID = appIDResponse.dataValues.APP_ID;
            } else {
                return util.handleError(res, new CustomError(404, constants.ERROR_CODE[404], "Application not found"));
            }
            let cpApplicationName;
            //all token types bound to one app if shared

            //unique app name for control plane application
            cpApplicationName = `${appID}`;
            //TODO - handel non-shared token types scenarios
            //create control plane application
            const sharedToken = await adminDao.getApplicationKeyMapping(orgID, appID, true);
            const nonSharedToken = await adminDao.getApplicationKeyMapping(orgID, appID, false);

            if (sharedToken.length !== 0) {
                cpAppID = sharedToken[0].dataValues.CP_APP_REF;
            } else if (nonSharedToken.length !== 0) {
                cpAppID = nonSharedToken[0].dataValues.CP_APP_REF;
            } else {
                const cpAppCreationResponse = await createCPApplication(req, cpApplicationName);
                cpAppID = cpAppCreationResponse.applicationId;
                //create application mapping entry
                const appKeyMappping = {
                    orgID: orgID,
                    appID: appID,
                    cpAppRef: cpAppCreationResponse.applicationId,
                    apiRefID: null,
                    subscriptionRefID: null,
                    sharedToken: true,
                    tokenType: constants.TOKEN_TYPES.OAUTH
                }
                if (sharedToken.length === 0 && nonSharedToken.length === 0) {
                    await adminDao.createApplicationKeyMapping(appKeyMappping, t);
                }
            }
            // add subscription to control plane for each api
            const apiSubscriptions = [];
            const subAPIs = await adminDao.getSubscribedAPIs(orgID, appID);
            for (const sub of subAPIs) {
                const api = new APIDTO(sub);
                const policyDetails = await apiDao.getSubscriptionPolicy(api.policyID, orgID, t);
                const cpSubscribeResponse = await createCPSubscription(req, api.apiReferenceID, cpAppID, policyDetails);
                apiSubscriptions.push(cpSubscribeResponse);
            }
            //create app key mapping
            //TODO: only oauth key shared scenario is considered, need to handle other token types 
            for (const apiSubscription of apiSubscriptions) {
                const appKeyMappping = {
                    orgID: orgID,
                    appID: appID,
                    cpAppRef: cpAppID,
                    apiRefID: apiSubscription.apiId,
                    subscriptionRefID: apiSubscription.subscriptionId,
                    sharedToken: true,
                    tokenType: constants.TOKEN_TYPES.OAUTH
                }
                //check whether key mapping exists
                const sharedKeyMapping = await adminDao.getApplicationAPIMapping(orgID, appID, apiSubscription.apiId, cpAppID, true, t);
                const nonSharedKeyMapping = await adminDao.getApplicationAPIMapping(orgID, appID, apiSubscription.apiId, cpAppID, false, t);

                if (sharedKeyMapping.length === 0 && nonSharedKeyMapping.length === 0) {
                    await adminDao.createApplicationKeyMapping(appKeyMappping, t);
                }
            }

            //delete app key mapping entries with no api id ref
            if (apiSubscriptions.length > 0) {
                logger.info('Deleting app key mapping entries with no API ID reference', {
                    orgId: req.params?.orgId,
                    applicationId: req.params?.applicationId
                });
                await adminDao.deleteAppKeyMapping(orgID, appID, null, t);
            }

            tokenDetails.additionalProperties = checkAdditionalValues(tokenDetails.additionalProperties);
            //TODO: need to support both key types
            tokenDetails.keyType = "PRODUCTION";

            if (tokenDetails.keyManager.startsWith(constants.KEY_MANAGERS.INTERNAL_KEY_MANAGER) || tokenDetails.keyManager.startsWith(constants.KEY_MANAGERS.RESIDENT_KEY_MANAGER) || tokenDetails.keyManager.startsWith(constants.KEY_MANAGERS.APP_DEV_STS_KEY_MANAGER)) {
                //generate oauth key
                responseData = await generateOAuthKey(req, cpAppID, tokenDetails);
            } else {
                responseData = await mapKeys(req, clientID, tokenDetails.keyManager, cpAppID);
            }
            // Add the appRefId to the response data
            responseData.appRefId = cpAppID;
            const cpApp = await invokeApiRequest(req, 'GET', `${controlPlaneUrl}/applications/${cpAppID}`, {}, {});
            responseData.subscriptionScopes = cpApp.subscriptionScopes;

            let subscriptionScopes = [];
            if (Array.isArray(cpApp?.subscriptionScopes)) {
                for (const scope of cpApp?.subscriptionScopes) {
                    subscriptionScopes.push(scope.key);
                }
            }
            responseData.subscriptionScopes = subscriptionScopes;
        });
        trackGenerateCredentials({
            orgId: orgID,
            appName: req.body.applicationName,
            idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
        });
        return res.status(200).json(responseData);
    } catch (error) {
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params?.orgId
        });
        //delete control plane application
        if (cpAppID) {
            await invokeApiRequest(req, 'DELETE', `${controlPlaneUrl}/applications/${cpAppID}`, {}, {});
            await adminDao.deleteAppMappings(orgID, cpAppID);
        }
        return util.handleError(res, error);
    }
}

async function mapKeys(req, clientID, keyManager, applicationId) {
    logger.info('Mapping existing client ID with application', {
        applicationId
    });
    const body = {
        "consumerKey": clientID,
        "keyManager": keyManager,
        "keyType": 'PRODUCTION',
    };
    try {
        return await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications/${applicationId}/map-keys`, {}, body);
    } catch (error) {
        logger.error('Error occurred while mapping keys', {
            error: error.message,
            applicationId
        });
        throw error;
    }
};

async function generateOAuthKey(req, cpAppID, tokenDetails) {
    logger.info('Generating OAuth key for application', {
        cpAppID
    });
    try {
        return await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications/${cpAppID}/generate-keys`, {}, tokenDetails);
    } catch (error) {
        try {
            if (error.statusCode && error.statusCode === 409) {
                logger.info('OAuth key already exists in control plane, retrieving existing key', {
                    cpAppID
                });
                const response = await invokeApiRequest(req, 'GET', `${controlPlaneUrl}/applications/${cpAppID}/keys`, {});
                return response.list[0];
            } else {
                throw error;
            }
        } catch (error) {
            logger.error('Error occurred while generating API key', {
                error: error.message,
                cpAppID
            });
            throw error;
        }
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

const createCPSubscription = async (req, apiId, cpAppID, policyDetails) => {
    logger.info('Creating control plane subscription', {
        apiId,
        cpAppID,
        policyDetails: policyDetails.dataValues ? policyDetails.dataValues.POLICY_NAME : policyDetails
    });
    try {
        const requestBody = {
            apiId: apiId,
            applicationId: cpAppID,
            throttlingPolicy: policyDetails.dataValues ? policyDetails.dataValues.POLICY_NAME : policyDetails
        };
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

const retriveAppKeyMappings = async (req, res) => {

    const { orgId, appId } = req.params;
    const userID = req[constants.USER_ID] ? req[constants.USER_ID] : "";
    try {
        const appIDResponse = await adminDao.getApplication(orgId, appId, userID);
        if (!appIDResponse) {
            throw new CustomError(404, "Records Not Found", 'Application not found');
        }
        const appKeyMappings = await adminDao.getKeyMapping(orgId, appId);
        res.status(200).send(appKeyMappings);
    } catch (error) {
        logger.error('key mapping retrieve error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
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

const unsubscribeAPI = async (req, res) => {
    const orgID = req.params.orgId;
    logger.info('Initiate unsubscribe from API...', {
        orgId: orgID,
        ...req.query
    });
    try {
        const { appID, apiReferenceID, subscriptionID } = req.query;
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const sharedToken = await adminDao.getApplicationKeyMapping(orgID, appID, true);
            const nonSharedToken = await adminDao.getApplicationKeyMapping(orgID, appID, false);
            logger.info('Unsubscribing from API', {
                apiReferenceID,
                subscriptionId: req.params?.subscriptionId
            });
            try {
                if (nonSharedToken.length > 0) {
                    logger.info('Deleting non-shared app key mapping entries', {
                        apiReferenceID,
                        subscriptionId: req.params?.subscriptionId
                    });
                    for (const dataValues of nonSharedToken) {
                        if (dataValues.API_REF_ID === apiReferenceID) {
                            await invokeApiRequest(req, 'DELETE', `${controlPlaneUrl}/subscriptions/${dataValues.SUBSCRIPTION_REF_ID}`, {}, {})
                            await handleUnsubscribe(nonSharedToken, sharedToken, orgID, appID, apiReferenceID, t);
                        }
                    }
                }
                if (sharedToken.length > 0) {
                    logger.info('Deleting shared app key mapping entries', {
                        apiReferenceID,
                        subscriptionId: req.params?.subscriptionId
                    });
                    for (const dataValues of sharedToken) {
                        if (dataValues.API_REF_ID === apiReferenceID) {
                            await invokeApiRequest(req, 'DELETE', `${controlPlaneUrl}/subscriptions/${dataValues.SUBSCRIPTION_REF_ID}`, {}, {})
                            await handleUnsubscribe(nonSharedToken, sharedToken, orgID, appID, apiReferenceID, t);
                        }
                    };
                }
                await adminDao.deleteSubscription(orgID, subscriptionID, t);
                trackUnsubscribeApi({
                    orgId: orgID,
                    appId: appID,
                    apiRefId: apiReferenceID,
                    idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
                });
                return res.status(204).send();
            } catch (error) {
                try {
                    if (error.statusCode && error.statusCode === 404) {
                        logger.info('Subscription not found in control plane, deleting from database', {
                            subscriptionId: req.params?.subscriptionId
                        });
                        await handleUnsubscribe(nonSharedToken, sharedToken, orgID, appID, apiReferenceID, t);
                        await adminDao.deleteSubscription(orgID, subscriptionID, t);
                        trackUnsubscribeApi({
                            orgId: orgID,
                            appId: appID,
                            apiRefId: apiReferenceID,
                            idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
                        });
                        return res.status(204).send();
                    }
                } catch (error) {
                    logger.error('Error occurred while deleting subscription', {
                        error: error.message,
                        subscriptionId: req.params?.subscriptionId
                    });
                    return util.handleError(res, error);
                }
                logger.error('Error occurred while unsubscribing from API', {
                    error: error.message,
                    subscriptionId: req.params?.subscriptionId
                });
                return util.handleError(res, error);
            }
        });
    } catch (error) {
        logger.error('Error occurred while unsubscribing from API', {
            error: error.message,
            subscriptionId: req.params?.subscriptionId
        });
        return util.handleError(res, error);
    }
}

async function handleUnsubscribe(nonSharedToken, sharedToken, orgID, appID, apiRefID, t) {
    try {
        if (sharedToken.length === 1 && nonSharedToken.length === 0) {
            logger.info('Updating shared app key mapping entries', {
                orgID,
                appID,
                apiRefID
            });
            await adminDao.updateApplicationKeyMapping(apiRefID, {
                orgID: sharedToken[0].dataValues.ORG_ID,
                appID: sharedToken[0].dataValues.APP_ID,
                cpAppRef: sharedToken[0].dataValues.CP_APP_REF,
                apiRefID: null,
                subscriptionRefID: null,
                sharedToken: true,
                tokenType: constants.TOKEN_TYPES.OAUTH
            }, t);
        } else if (nonSharedToken.length === 1 && sharedToken.length === 0) {
            logger.info('Updating non-shared app key mapping entries', {
                orgID,
                appID,
                apiRefID
            });
            await adminDao.updateApplicationKeyMapping(apiRefID, {
                orgID: nonSharedToken[0].dataValues.ORG_ID,
                appID: nonSharedToken[0].dataValues.APP_ID,
                cpAppRef: nonSharedToken[0].dataValues.CP_APP_REF,
                apiRefID: null,
                subscriptionRefID: null,
                sharedToken: false,
                tokenType: constants.TOKEN_TYPES.API_KEY
            }, t);
        } else {
            if (sharedToken.length > 0 || nonSharedToken.length > 0) {
                await adminDao.deleteAppKeyMapping(orgID, appID, apiRefID, t);
            }
        }
    } catch (error) {
        logger.error('Transaction failed during unsubscribing', {
            error: error.message,
            orgID,
            appID,
            apiRefID
        });
        throw error;
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
    createDevPortalApplication,
    updateDevPortalApplication,
    getDevPortalApplications,
    getDevPortalApplicationDetails,
    deleteDevPortalApplication,
    getAllApplications,
    createSubscription,
    updateSubscription,
    getSubscription,
    getAllSubscriptions,
    deleteSubscription,
    unsubscribeAPI,
    createAppKeyMapping,
    retriveAppKeyMappings,
    getApplicationKeyMap,
    checkAdditionalValues,
    createCPApplication,
    createCPSubscription
};
