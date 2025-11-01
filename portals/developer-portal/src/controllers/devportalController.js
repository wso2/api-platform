/* eslint-disable no-undef */
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
const { invokeApiRequest, invokeGraphQLRequest } = require('../utils/util');
const config = require(process.cwd() + '/config');
const logger = require('../config/logger');
const controlPlaneUrl = config.controlPlane.url;
const controlPlaneGraphqlUrl = config.controlPlane.graphqlURL;
const util = require('../utils/util');
const passport = require('passport');
const { Strategy: CustomStrategy } = require('passport-custom');
const adminDao = require('../dao/admin');
const constants = require('../utils/constants');
const { ApplicationDTO } = require('../dto/application');
const { Sequelize } = require("sequelize");
const adminService = require('../services/adminService');
const apiDao = require('../dao/apiMetadata');
const { trackAppCreationStart, trackAppCreationEnd, trackAppDeletion, trackGenerateKey } = require('../utils/telemetry');

// ***** POST / DELETE / PUT Functions ***** (Only work in production)

// ***** Save Application *****

const saveApplication = async (req, res) => {
    try {
        const orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        trackAppCreationStart({ orgId: orgID, appName: req.body.name, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined });
        const application = await adminDao.createApplication(orgID, req.user.sub, req.body);
        trackAppCreationEnd({ orgId: orgID, appName: req.body.name, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined });
        return res.status(201).json(new ApplicationDTO(application.dataValues));
    } catch (error) {
        logger.error('Error occurred while creating the application', { 
            orgId: req.user[constants.ORG_IDENTIFIER],
            appName: req.body.name,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

// ***** Update Application *****

const updateApplication = async (req, res) => {
    try {
        const orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        const appID = req.params.applicationId;
        const [updatedRows, updatedApp] = await adminDao.updateApplication(orgID, appID, req.user.sub, req.body);
        if (!updatedRows) {
            throw new Sequelize.EmptyResultError("No record found to update");
        }
        res.status(200).send(new ApplicationDTO(updatedApp[0].dataValues));
    } catch (error) {
        logger.error("Error occurred while updating the application", { 
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

// ***** Delete Application *****

const deleteApplication = async (req, res) => {
    try {
        const orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        const applicationId = req.params.applicationId;
        try {
            //delete the CP application
            //TODO: handle non-shared scenarios
            const app = await adminDao.getApplicationKeyMapping(orgID, applicationId, true);
            if (app.length > 0) {
                cpAppID = app[0].dataValues.CP_APP_REF;
                await invokeApiRequest(req, 'DELETE', `${controlPlaneUrl}/applications/${cpAppID}`, {}, {});
            }
            const appDeleteResponse = await adminDao.deleteApplication(orgID, applicationId, req.user.sub);
            if (appDeleteResponse === 0) {
                throw new Sequelize.EmptyResultError("Resource not found to delete");
            } else {
                trackAppDeletion({ orgId: orgID, appId: applicationId, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined });
                res.status(200).send("Resouce Deleted Successfully");
            }
        } catch (error) {
            if (error.statusCode === 404) {
                const appDeleteResponse = await adminDao.deleteApplication(orgID, applicationId, req.user.sub);
                if (appDeleteResponse === 0) {
                    throw new Sequelize.EmptyResultError("Resource not found to delete");
                } else {
                    res.status(200).send("Resouce Deleted Successfully");
                }
            }
            logger.error('Error occurred while deleting the application', { 
                orgId: orgID,
                appId: appID,
                error: error.message, 
                stack: error.stack
            });
            util.handleError(res, error);
        }
    } catch (error) {
        logger.error('Error occurred while deleting the application', { 
            appId: req.params.appId,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
}

// ***** Save Application *****

const resetThrottlingPolicy = async (req, res) => {
    try {
        const applicationId = req.params.applicationId;
        const { userName } = req.body;
        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications/${applicationId}/reset-throttle-policy`, {
            'Content-Type': 'application/json'
        }, {
            userName
        });
        res.status(200).json({ message: responseData.message });
    } catch (error) {
        logger.error("Error occurred while resetting the application", { 
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

// ***** Generate API Keys *****

const generateAPIKeys = async (req, res) => {
    try {
        const requestBody = req.body;
        const apiID = requestBody.apiId;
        const orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        let cpAppID = requestBody.applicationId;

        const nonSharedKeyMapping = await adminDao.getApplicationAPIMapping(orgID, requestBody.devportalAppId, apiID, cpAppID, false);
        const sharedKeyMapping = await adminDao.getApplicationAPIMapping(orgID, requestBody.devportalAppId, apiID, cpAppID, true);

        if (!(nonSharedKeyMapping.length > 0 || sharedKeyMapping.length > 0)) { 
            const cpApp = await adminService.createCPApplication(req, requestBody.devportalAppId);
            cpAppID = cpApp.applicationId;

            const apiSubscription = await adminService.createCPSubscription(req, apiID, cpAppID, requestBody.subscriptionPlan);

            const appKeyMappping = {
                orgID: orgID,
                appID: requestBody.devportalAppId,
                cpAppRef: cpAppID,
                apiRefID: apiSubscription.apiId,
                subscriptionRefID: apiSubscription.subscriptionId,
                sharedToken: false,
                tokenType: constants.TOKEN_TYPES.API_KEY
            }
            await adminDao.createApplicationKeyMapping(appKeyMappping);
        } else if (!(nonSharedKeyMapping[0]?.dataValues.SUBSCRIPTION_REF_ID || sharedKeyMapping[0]?.dataValues.SUBSCRIPTION_REF_ID)) {
            const apiSubscription = await adminService.createCPSubscription(req, apiID, cpAppID, requestBody.subscriptionPlan);
            const appKeyMappping = {
                orgID: orgID,
                appID: requestBody.devportalAppId,
                cpAppRef: cpAppID,
                apiRefID: apiSubscription.apiId,
                subscriptionRefID: apiSubscription.subscriptionId,
                sharedToken: false,
                tokenType: constants.TOKEN_TYPES.API_KEY
            }
            await adminDao.updateApplicationKeyMapping(apiSubscription.apiId, appKeyMappping);
        }
        
        const query = `
        query ($orgUuid: String!, $projectId: String!) {
          environments(orgUuid: $orgUuid, projectId: $projectId) {
            name
            templateId
          }
        }
      `;

        const variables = {
            orgUuid: req.user[constants.ORG_IDENTIFIER],
            projectId: requestBody.projectID
        };

        const orgDetails = await invokeGraphQLRequest(req, `${controlPlaneGraphqlUrl}`, query, variables, {});
        const environments = orgDetails?.data?.environments || [];
        const apiHandle = await apiDao.getAPIHandle(orgID, req.body.apiId);

        requestBody.name = apiHandle + "-" + cpAppID;
        requestBody.environmentTemplateId = environments.find(env => env.name === 'Production').templateId;
        requestBody.applicationId = cpAppID;
        delete requestBody.projectID;
        delete requestBody.devportalAppId;

        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/api-keys/generate`, {
            'Content-Type': 'application/json'
        }, requestBody);
        responseData.appRefId = cpAppID;
        res.status(200).json(responseData);
    } catch (error) {
        logger.error('Error occurred while generating API keys', { 
            apiId: req.body.apiId,
            appId: req.body.devportalAppId,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

const revokeAPIKeys = async (req, res) => {
    const apiKeyID = req.params.apiKeyID;
    try {
        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/api-keys/${apiKeyID}/revoke`, {}, {});
        // await adminDao.deleteAppKeyMapping(await adminDao.getOrgId((req.user[constants.ORG_IDENTIFIER])), req.body.applicationId, req.body.apiRefID);
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while revoking the API key", { 
            apiKeyID,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
}

const regenerateAPIKeys = async (req, res) => {
    const apiKeyID = req.params.apiKeyID;
    try {
        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/api-keys/${apiKeyID}/regenerate`, {}, {});
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while revoking the API key", { 
            apiKeyID,
            error: error.message, 
            stack: error.stack 
        });
        util.handleError(res, error);
    }
}

const generateApplicationKeys = async (req, res) => {
    try {
        const applicationId = req.params.applicationId;
        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications/${applicationId}/generate-keys`, {}, req.body);
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while generating the application keys", { 
            appId: req.params.applicationId,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

const generateOAuthKeys = async (req, res) => {
    try {
        const applicationId = req.params.applicationId;
        const keyMappingId = req.params.keyMappingId;
        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications/${applicationId}/oauth-keys/${keyMappingId}/generate-token`, {}, req.body);
        trackGenerateKey({
            orgId: req.user[constants.ORG_ID],
            appId: applicationId,
            idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
        });
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while generating the OAuth keys", { 
            appId: req.params.applicationId,
            error: error.message, 
            stack: error.stack 
        });
        util.handleError(res, error);
    }
};

const revokeOAuthKeys = async (req, res) => {
    try {
        const applicationId = req.params.applicationId;
        const keyMappingId = req.params.keyMappingId;
        const responseData = await invokeApiRequest(req, 'DELETE', `${controlPlaneUrl}/applications/${applicationId}/oauth-keys/${keyMappingId}`, {}, {});
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while generating the OAuth keys", { 
            appId: req.params.applicationId,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

const cleanUp = async (req, res) => {
    try {
        const applicationId = req.params.applicationId;
        const keyMappingId = req.params.keyMappingId;
        const responseData = await invokeApiRequest(req, 'POST', `${controlPlaneUrl}/applications/${applicationId}/oauth-keys/${keyMappingId}/clean-up`, {}, req.body);
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while generating the OAuth keys", { 
            appId: req.params.applicationId,
            error: error.message, 
            stack: error.stack
        });
        util.handleError(res, error);
    }
};

const updateOAuthKeys = async (req, res) => {
    let tokenDetails = req.body;
    try {
        const applicationId = req.params.applicationId;
        const keyMappingId = req.params.keyMappingId;
        const responseData = await invokeApiRequest(req, 'PUT', `${controlPlaneUrl}/applications/${applicationId}/oauth-keys/${keyMappingId}`, {}, tokenDetails);
        res.status(200).json(responseData);
    } catch (error) {
        logger.error("Error occurred while generating the OAuth keys", { 
            appId: req.params.applicationId,
            error: error.message, 
            stack: error.stack, 
        });
        util.handleError(res, error);
    }
};

const login = async (req, res) => {
    const username = req.body.username;
    const password = req.body.password;

    const defaultUser = config.defaultAuth.users.find(user => user.username === username && user.password === password);
    passport.use(
        'default-auth',
        new CustomStrategy((req, done) => {
            if (defaultUser) {
                const user = { ...defaultUser };
                return done(null, user);
            } else {
                return done(null, false, { message: 'Invalid credentials' });
            }
        })
    );

    passport.authenticate('default-auth', (err, user, info) => {
        if (err) {
            logger.error("Error occurred while logging in", { 
                error: err.message, 
                stack: err.stack
            });
            return util.handleError(res, err);
        }
        if (!user) {
            return res.status(401).json({ message: 'Invalid credentials' });
        }
        req.logIn(user, (err) => {
            if (err) {
                return util.handleError(res, err);
            }
            res.status(200).json({ message: 'Login successful' });
        });
    })(req, res);
};

module.exports = {
    saveApplication,
    updateApplication,
    deleteApplication,
    resetThrottlingPolicy,
    generateAPIKeys,
    generateApplicationKeys,
    generateOAuthKeys,
    revokeOAuthKeys,
    updateOAuthKeys,
    cleanUp,
    login,
    revokeAPIKeys,
    regenerateAPIKeys
};