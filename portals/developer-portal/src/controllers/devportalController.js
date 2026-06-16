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
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const util = require('../utils/util');
const passport = require('passport');
const { Strategy: CustomStrategy } = require('passport-custom');
const adminDao = require('../dao/admin');
const constants = require('../utils/constants');
const { ApplicationDTO } = require('../dto/application');
const { Sequelize } = require("sequelize");
const { trackAppCreationStart, trackAppCreationEnd, trackAppDeletion, trackGenerateKey, trackGenerateCredentials } = require('../utils/telemetry');
const yaml = require('js-yaml');
const kmDao = require('../dao/keyManager');
const { getKeyManagerAdapter } = require('../adapters/keyManager');
const { CustomError } = require('../utils/errors/customErrors');
// ***** POST / DELETE / PUT Functions ***** (Only work in production)

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
            throw new CustomError(400, "Bad Request", "Missing required application field: name");
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

// ***** Save Application *****

const saveApplication = async (req, res) => {
    try {
        const orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        const applicationData = parseApplicationDataFromRequest(req);
        trackAppCreationStart({ orgId: orgID, appName: applicationData.name, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined }, req);
        const application = await adminDao.createApplication(orgID, req.user.sub, applicationData);
        trackAppCreationEnd({ orgId: orgID, appName: applicationData.name, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined }, req);
        return res.status(201).json(new ApplicationDTO(application.dataValues));
    } catch (error) {
        logger.error('Error occurred while creating the application', {
            orgId: req.user[constants.ORG_IDENTIFIER],
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
        const applicationData = parseApplicationDataFromRequest(req);
        const [updatedRows, updatedApp] = await adminDao.updateApplication(orgID, appID, req.user.sub, applicationData);
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

const revokeAppKeyMappings = async (orgID, appID) => {
    const { ApplicationKeyMapping } = require('../models/application');
    const mappings = await ApplicationKeyMapping.findAll({
        where: { APP_ID: appID, ORG_ID: orgID },
    });
    const succeededMappingIds = [];
    const failedMappingIds = [];
    for (const mapping of mappings) {
        if (mapping.KM_ID && mapping.AS_CLIENT_ID) {
            try {
                const kmRecord = await kmDao.getKeyManager(mapping.KM_ID);
                const adapter = getKeyManagerAdapter(kmRecord);
                await adapter.deleteOAuthClient(mapping.AS_CLIENT_ID);
                succeededMappingIds.push(mapping.MAPPING_ID);
            } catch (err) {
                logger.warn('Failed to revoke OAuth client during application deletion', {
                    appId: appID,
                    clientId: mapping.AS_CLIENT_ID,
                    kmId: mapping.KM_ID,
                    errorMessage: err.message,
                });
                failedMappingIds.push(mapping.MAPPING_ID);
            }
        } else {
            // No OAuth client to revoke — safe to remove the local mapping.
            succeededMappingIds.push(mapping.MAPPING_ID);
        }
    }
    await adminDao.deleteAppMappingsByIds(orgID, succeededMappingIds);
    if (failedMappingIds.length > 0) {
        throw new Error(
            `Failed to revoke OAuth clients for ${failedMappingIds.length} mapping(s) ` +
            `(mappingIds: ${failedMappingIds.join(', ')}). Application deletion aborted.`
        );
    }
};

const deleteApplication = async (req, res) => {
    try {
        const orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        const applicationId = req.params.applicationId;
        try {
            await revokeAppKeyMappings(orgID, applicationId);
            const appDeleteResponse = await adminDao.deleteApplication(orgID, applicationId, req.user.sub);
            if (appDeleteResponse === 0) {
                throw new Sequelize.EmptyResultError("Resource not found to delete");
            } else {
                trackAppDeletion({ orgId: orgID, appId: applicationId, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined }, req);
                res.status(200).send("Resouce Deleted Successfully");
            }
        } catch (error) {
            if (error.statusCode === 404) {
                await revokeAppKeyMappings(orgID, applicationId);
                const appDeleteResponse = await adminDao.deleteApplication(orgID, applicationId, req.user.sub);
                if (appDeleteResponse === 0) {
                    throw new Sequelize.EmptyResultError("Resource not found to delete");
                } else {
                    trackAppDeletion({ orgId: orgID, appId: applicationId, idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined }, req);
                    return res.status(200).send("Resouce Deleted Successfully");
                }
            }
            logger.error('Error occurred while deleting the application', {
                orgId: orgID,
                appId: applicationId,
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

const generateKeys = async (req, res) => {
    let orgID, appID, userID;
    try {
        orgID = await adminDao.getOrgId(req.user[constants.ORG_IDENTIFIER]);
        appID = req.params.applicationId;
        userID = req[constants.USER_ID] || req.user?.sub;
        logger.info('Initiate create application key mapping...', { orgId: orgID, appId: appID });
        const {
            keyManager: kmName,
            keyType: rawKeyType,
            grantTypesToBeSupported,
            callbackUrl,
            scopes,
            additionalProperties: additionalProps,
        } = req.body;

        const kmRecord = await kmDao.getKeyManagerByName(orgID, kmName);
        const adapter = getKeyManagerAdapter(kmRecord);

        const grantTypes = grantTypesToBeSupported || ['client_credentials'];
        const redirectUris = callbackUrl ? [callbackUrl] : [];
        const resolvedScopes = scopes || ['default'];
        const resolvedProps = additionalProps || {};

        const sanitize = (s) => String(s).replace(/[^a-zA-Z0-9]/g, '_').replace(/_+/g, '_').replace(/^_|_$/g, '');
        const keyType = (rawKeyType || 'PRODUCTION').toUpperCase();
        const clientName = `${sanitize(userID)}_${sanitize(appID)}_${keyType}`;

        const oauthClient = await adapter.createOAuthClient(clientName, grantTypes, redirectUris, resolvedScopes, resolvedProps);

        const responseData = {
            consumerKey: oauthClient.clientId,
            consumerSecret: oauthClient.clientSecret,
            keyManager: kmName,
            tokenEndpoint: kmRecord.TOKEN_ENDPOINT,
            supportedGrantTypes: kmRecord.SUPPORTED_GRANT_TYPES,
            additionalProperties: oauthClient.additionalProperties,
            subscriptionScopes: oauthClient.subscriptionScopes || [],
        };

        const appKeyMapping = {
            orgID,
            appID,
            kmID: kmRecord.KM_ID,
            asClientID: responseData.consumerKey,
            keyType: rawKeyType || 'PRODUCTION',
            additionalProperties: responseData.additionalProperties || {},
        };
        let keyMappingRecord;
        try {
            keyMappingRecord = await adminDao.upsertApplicationKeyMapping(appKeyMapping);
        } catch (dbError) {
            if (oauthClient) {
                await adapter.deleteOAuthClient(oauthClient.clientId).catch((cleanupErr) => {
                    logger.warn('Failed to roll back OAuth client after DB error', {
                        clientId: oauthClient.clientId,
                        errorMessage: cleanupErr.message,
                    });
                });
            }
            throw dbError;
        }

        responseData.keyMappingId = keyMappingRecord?.dataValues?.MAPPING_ID;

        trackGenerateCredentials({
            orgId: orgID,
            appName: appID,
            idpId: req.isAuthenticated() ? userID : undefined
        }, req);
        return res.status(200).json(responseData);
    } catch (error) {
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID,
            appId: appID
        });
        return util.handleError(res, error);
    }
};

const generateOAuthKeys = async (req, res) => {
    try {
        const applicationId = req.params.applicationId;
        const keyMappingId = req.params.keyMappingId;

        const { ApplicationKeyMapping } = require('../models/application');
        const keyMapping = await ApplicationKeyMapping.findOne({
            where: { MAPPING_ID: keyMappingId, APP_ID: applicationId },
        });
        if (!keyMapping || !keyMapping.KM_ID) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found or missing key manager reference' });
        }
        const kmRecord = await kmDao.getKeyManager(keyMapping.KM_ID);
        const adapter = getKeyManagerAdapter(kmRecord);
        const { consumerSecret, scopes, validityPeriod } = req.body;
        const tokenResult = await adapter.generateToken(
            keyMapping.AS_CLIENT_ID,
            consumerSecret,
            scopes || ['default'],
            validityPeriod || 3600
        );
        const responseData = {
            accessToken: tokenResult.accessToken,
            validityTime: tokenResult.expiresIn,
            tokenScopes: tokenResult.scope ? tokenResult.scope.split(' ') : [],
        };

        trackGenerateKey({
            orgId: req.user[constants.ORG_ID],
            appId: applicationId,
            idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
        }, req);
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

        const { ApplicationKeyMapping } = require('../models/application');
        const keyMapping = await ApplicationKeyMapping.findOne({
            where: { MAPPING_ID: keyMappingId, APP_ID: applicationId },
        });
        if (!keyMapping || !keyMapping.KM_ID) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found or missing key manager reference' });
        }
        const kmRecord = await kmDao.getKeyManager(keyMapping.KM_ID);
        const adapter = getKeyManagerAdapter(kmRecord);
        await adapter.deleteOAuthClient(keyMapping.AS_CLIENT_ID);
        await ApplicationKeyMapping.update(
            { AS_CLIENT_ID: null, KM_ID: null },
            { where: { MAPPING_ID: keyMappingId, APP_ID: applicationId } }
        );
        res.status(200).json({ message: 'OAuth client revoked successfully' });
    } catch (error) {
        logger.error("Error occurred while revoking the OAuth keys", {
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

        const { ApplicationKeyMapping } = require('../models/application');
        const keyMapping = await ApplicationKeyMapping.findOne({
            where: { MAPPING_ID: keyMappingId, APP_ID: applicationId },
        });
        if (keyMapping && keyMapping.KM_ID && keyMapping.AS_CLIENT_ID) {
            const kmRecord = await kmDao.getKeyManager(keyMapping.KM_ID);
            const adapter = getKeyManagerAdapter(kmRecord);
            await adapter.deleteOAuthClient(keyMapping.AS_CLIENT_ID);
        }
        await ApplicationKeyMapping.destroy({ where: { MAPPING_ID: keyMappingId, APP_ID: applicationId } });
        res.status(200).json({ message: 'OAuth client cleaned up successfully' });
    } catch (error) {
        logger.error("Error occurred while cleaning up the OAuth keys", {
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

        const { ApplicationKeyMapping } = require('../models/application');
        const keyMapping = await ApplicationKeyMapping.findOne({
            where: { MAPPING_ID: keyMappingId, APP_ID: applicationId },
        });
        if (!keyMapping || !keyMapping.KM_ID) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found or missing key manager reference' });
        }
        const kmRecord = await kmDao.getKeyManager(keyMapping.KM_ID);
        const adapter = getKeyManagerAdapter(kmRecord);
        const updatedGrantTypes = tokenDetails.supportedGrantTypes || tokenDetails.grantTypesToBeSupported;
        const result = await adapter.updateOAuthClient(
            keyMapping.AS_CLIENT_ID,
            updatedGrantTypes,
            tokenDetails.callbackUrl ? [tokenDetails.callbackUrl] : [],
            tokenDetails.scopes,
            tokenDetails.additionalProperties
        );

        if (result?.additionalProperties) {
            await ApplicationKeyMapping.update(
                { ADDITIONAL_PROPERTIES: result.additionalProperties },
                { where: { MAPPING_ID: keyMappingId, APP_ID: applicationId } }
            );
        }

        res.status(200).json({ message: 'OAuth client updated successfully' });
    } catch (error) {
        logger.error("Error occurred while updating the OAuth keys", {
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
    generateKeys,
    generateOAuthKeys,
    revokeOAuthKeys,
    updateOAuthKeys,
    cleanUp,
    login
};
