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
const axios = require('axios');
const https = require('https');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const util = require('../utils/util');
const orgDao = require('../dao/organizationDao');
const appDao = require('../dao/applicationDao');
const apiKeyService = require('../services/apiKeyService');
const { publish } = require('../services/webhooks/eventPublisher');
const sequelize = require('../db/sequelizeConfig');
const constants = require('../utils/constants');
const { ApplicationDTO } = require('../dto/applicationDto');
const { Sequelize } = require("sequelize");
const { trackAppCreationStart, trackAppCreationEnd, trackAppDeletion, trackGenerateKey, trackGenerateCredentials } = require('../utils/telemetryUtil');
const yaml = require('js-yaml');
const kmDao = require('../dao/keyManagerDao');
const { getKeyManagerAdapter } = require('../adapters/keyManager');
const { CustomError } = require('../utils/errors/customErrors');
const { extractPlatformJwtClaims } = require('../utils/platformJwt');
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
        };
    }
    return req.body;
}

// ***** Save Application *****

const listApplications = async (req, res) => {
    const orgID = String(req.params.orgId || '').replace(/[\r\n]/g, '');
    const userID = req.auth?.userId || req.user?.sub;
    try {
        const applications = await appDao.list(orgID, userID);
        return res.status(200).json(util.toPaginatedList(applications.map(a => new ApplicationDTO(a.dataValues)), req));
    } catch (error) {
        logger.error('Error occurred while listing applications', { orgId: orgID, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

const saveApplication = async (req, res) => {
    const orgID = String(req.params.orgId || '').replace(/[\r\n]/g, '');
    const userID = req.auth?.userId || req.user?.sub;
    try {
        const applicationData = parseApplicationDataFromRequest(req);
        trackAppCreationStart({ orgId: orgID, appName: applicationData.name, idpId: userID }, req);
        const application = await appDao.create(orgID, userID, applicationData);
        trackAppCreationEnd({ orgId: orgID, appName: applicationData.name, idpId: userID }, req);
        const createdApp = application.dataValues;
        try {
            await sequelize.transaction((t) => publish('application.created',
                { application_id: createdApp.UUID, name: createdApp.NAME, description: createdApp.DESCRIPTION },
                { transaction: t, orgId: orgID, aggregateType: 'application', aggregateId: createdApp.UUID }
            ));
        } catch (pubErr) {
            logger.warn('Failed to publish application.created', { orgId: orgID, appId: createdApp.UUID, error: pubErr.message });
        }
        return res.status(201).json(new ApplicationDTO(createdApp));
    } catch (error) {
        logger.error('Error occurred while creating the application', { orgId: orgID, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

// ***** Update Application *****

const updateApplication = async (req, res) => {
    const orgID = String(req.params.orgId || '').replace(/[\r\n]/g, '');
    const userID = req.auth?.userId || req.user?.sub;
    try {
        const appID = req.params.applicationId;
        const applicationData = parseApplicationDataFromRequest(req);
        const [updatedRows, updatedApp] = await appDao.update(orgID, appID, userID, applicationData);
        if (!updatedRows) {
            throw new Sequelize.EmptyResultError("No record found to update");
        }
        try {
            const renamedApp = updatedApp[0].dataValues;
            await sequelize.transaction(async (t) => {
                await publish('application.updated',
                    { application_id: appID, name: renamedApp.NAME, description: renamedApp.DESCRIPTION },
                    { transaction: t, orgId: orgID, aggregateType: 'application', aggregateId: appID }
                );
                await apiKeyService.notifyApplicationKeysChanged(orgID, appID, { id: appID, name: renamedApp.NAME }, t);
            });
        } catch (pubErr) {
            logger.warn('Failed to publish webhook events after app update', { orgId: orgID, appId: appID, error: pubErr.message });
        }
        res.status(200).send(new ApplicationDTO(updatedApp[0].dataValues));
    } catch (error) {
        logger.error("Error occurred while updating the application", { orgId: orgID, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

// ***** Delete Application *****

const revokeAppKeyMappings = async (orgID, appID) => {
    const { ApplicationKeyMapping } = require('../models/application');
    const mappings = await ApplicationKeyMapping.findAll({
        where: { APP_UUID: appID },
    });
    const succeededMappingIds = [];
    const failedMappingIds = [];
    for (const mapping of mappings) {
        if (mapping.KM_UUID && mapping.AS_CLIENT_ID) {
            try {
                const kmRecord = await kmDao.get(mapping.KM_UUID);
                const adapter = getKeyManagerAdapter(kmRecord);
                await adapter.deleteOAuthClient(mapping.AS_CLIENT_ID);
                succeededMappingIds.push(mapping.UUID);
            } catch (err) {
                logger.warn('Failed to revoke OAuth client during application deletion', {
                    appId: appID,
                    clientId: mapping.AS_CLIENT_ID,
                    kmId: mapping.KM_UUID,
                    errorMessage: err.message,
                });
                failedMappingIds.push(mapping.UUID);
            }
        } else {
            // No OAuth client to revoke — safe to remove the local mapping.
            succeededMappingIds.push(mapping.UUID);
        }
    }
    await appDao.deleteMappingsByIds(orgID, succeededMappingIds);
    if (failedMappingIds.length > 0) {
        throw new Error(
            `Failed to revoke OAuth clients for ${failedMappingIds.length} mapping(s) ` +
            `(mappingIds: ${failedMappingIds.join(', ')}). Application deletion aborted.`
        );
    }
};

/**
 * Publishes application.deleted + a per-key apikey.application_updated(null) for each
 * previously-associated key. Must be called only after the application row (and its
 * APP_UUID references) have actually been deleted — best-effort, never throws.
 */
const publishApplicationDeletedEvents = async (orgID, applicationId, appToDelete, affectedKeyIds) => {
    try {
        await sequelize.transaction(async (t) => {
            if (appToDelete) {
                await publish('application.deleted',
                    { application_id: applicationId, name: appToDelete.NAME },
                    { transaction: t, orgId: orgID, aggregateType: 'application', aggregateId: applicationId }
                );
            }
            for (const keyId of affectedKeyIds) {
                await apiKeyService.publishKeyApplicationUpdated(orgID, keyId, null, t);
            }
        });
    } catch (pubErr) {
        logger.warn('Failed to publish webhook events after app deletion', { orgId: orgID, appId: applicationId, error: pubErr.message });
    }
};

/**
 * Snapshots the app name + currently-associated key IDs and deletes the application row,
 * all inside one transaction — so the snapshot exactly matches what's actually deleted,
 * with no race window for a concurrent associate/dissociate call to go unnoticed.
 */
const deleteApplicationAndSnapshotKeys = async (orgID, applicationId, userID) => {
    let appToDelete = null;
    let affectedKeyIds = [];
    await sequelize.transaction(async (t) => {
        appToDelete = await appDao.get(orgID, applicationId, userID, t);
        const associatedKeys = await apiKeyService.list(orgID, { appId: applicationId }, t);
        affectedKeyIds = associatedKeys.map((k) => k.UUID);
        await appDao.delete(orgID, applicationId, userID, t);
    });
    return { appToDelete, affectedKeyIds };
};

const deleteApplication = async (req, res) => {
    const userID = req.auth?.userId || req.user?.sub;
    const applicationId = req.params.applicationId;
    const orgID = String(req.params.orgId || '').replace(/[\r\n]/g, '');
    try {
        const ownedApp = await appDao.get(orgID, applicationId, userID);
        if (!ownedApp) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        try {
            await revokeAppKeyMappings(orgID, applicationId);
            const { appToDelete, affectedKeyIds } = await deleteApplicationAndSnapshotKeys(orgID, applicationId, userID);
            trackAppDeletion({ orgId: orgID, appId: applicationId, idpId: userID }, req);
            await publishApplicationDeletedEvents(orgID, applicationId, appToDelete, affectedKeyIds);
            res.status(200).send("Resource Deleted Successfully");
        } catch (error) {
            if (error.statusCode === 404) {
                await revokeAppKeyMappings(orgID, applicationId);
                const { appToDelete, affectedKeyIds } = await deleteApplicationAndSnapshotKeys(orgID, applicationId, userID);
                trackAppDeletion({ orgId: orgID, appId: applicationId, idpId: userID }, req);
                await publishApplicationDeletedEvents(orgID, applicationId, appToDelete, affectedKeyIds);
                return res.status(200).send("Resource Deleted Successfully");
            }
            logger.error('Error occurred while deleting the application', { orgId: orgID, appId: applicationId, error: error.message, stack: error.stack });
            util.handleError(res, error);
        }
    } catch (error) {
        logger.error('Error occurred while deleting the application', { appId: applicationId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
}

const generateKeys = async (req, res) => {
    let orgID, appID, userID;
    try {
        orgID = req.params.orgId;
        appID = req.params.applicationId;
        userID = req.auth?.userId || req[constants.USER_ID] || req.user?.sub;
        logger.info('Initiate create application key mapping...', { orgId: orgID, appId: appID });
        const {
            keyManager: kmName,
            keyType: rawKeyType,
            grantTypesToBeSupported,
            callbackUrl,
            scopes,
            additionalProperties: additionalProps,
        } = req.body;

        const kmRecord = await kmDao.getByName(orgID, kmName);
        const adapter = getKeyManagerAdapter(kmRecord);

        const grantTypes = grantTypesToBeSupported || ['client_credentials'];
        const redirectUris = callbackUrl ? [callbackUrl] : [];
        const resolvedScopes = scopes || ['default'];
        const resolvedProps = additionalProps || {};

        const sanitize = (s) => String(s).replace(/[^a-zA-Z0-9]/g, '_').replace(/_+/g, '_').replace(/^_|_$/g, '');
        const keyType = (rawKeyType || constants.KEY_TYPE.PRODUCTION).toUpperCase();
        if (!Object.values(constants.KEY_TYPE).includes(keyType)) {
            return res.status(400).json({ message: `Invalid keyType. Must be one of: ${Object.values(constants.KEY_TYPE).join(', ')}.` });
        }
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
            kmID: kmRecord.UUID,
            asClientID: responseData.consumerKey,
            keyType,
            additionalProperties: responseData.additionalProperties || {},
            createdBy: userID,
        };
        let keyMappingRecord;
        try {
            keyMappingRecord = await appDao.upsertKeyMapping(appKeyMapping);
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

        responseData.keyMappingId = keyMappingRecord?.dataValues?.UUID;

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
            where: { UUID: keyMappingId, APP_UUID: applicationId },
        });
        if (!keyMapping || !keyMapping.KM_UUID) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found or missing key manager reference' });
        }
        const kmRecord = await kmDao.get(keyMapping.KM_UUID);
        if (!kmRecord) {
            return util.handleError(res, { statusCode: 404, message: 'Key manager not found' });
        }
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
            orgId: req.user[constants.ORG_UUID],
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
            where: { UUID: keyMappingId, APP_UUID: applicationId },
        });
        if (!keyMapping || !keyMapping.KM_UUID) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found or missing key manager reference' });
        }
        const kmRecord = await kmDao.get(keyMapping.KM_UUID);
        if (!kmRecord) {
            return util.handleError(res, { statusCode: 404, message: 'Key manager not found' });
        }
        const adapter = getKeyManagerAdapter(kmRecord);
        await adapter.deleteOAuthClient(keyMapping.AS_CLIENT_ID);
        await ApplicationKeyMapping.update(
            { AS_CLIENT_ID: null, KM_UUID: null },
            { where: { UUID: keyMappingId, APP_UUID: applicationId } }
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
            where: { UUID: keyMappingId, APP_UUID: applicationId },
        });
        if (keyMapping && keyMapping.KM_UUID && keyMapping.AS_CLIENT_ID) {
            const kmRecord = await kmDao.get(keyMapping.KM_UUID);
            if (kmRecord) {
                const adapter = getKeyManagerAdapter(kmRecord);
                await adapter.deleteOAuthClient(keyMapping.AS_CLIENT_ID);
            }
        }
        await ApplicationKeyMapping.destroy({ where: { UUID: keyMappingId, APP_UUID: applicationId } });
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
            where: { UUID: keyMappingId, APP_UUID: applicationId },
        });
        if (!keyMapping || !keyMapping.KM_UUID) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found or missing key manager reference' });
        }
        const kmRecord = await kmDao.get(keyMapping.KM_UUID);
        if (!kmRecord) {
            return util.handleError(res, { statusCode: 404, message: 'Key manager not found' });
        }
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
                { where: { UUID: keyMappingId, APP_UUID: applicationId } }
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
    const { username, password } = req.body;

    const platformApiUrl = config.platformApi?.baseUrl;
    if (!platformApiUrl) {
        return res.status(503).json({ message: 'Authentication service not configured' });
    }

    let platformToken;
    try {
        const response = await axios.post(
            `${platformApiUrl}/api/portal/v0.9/auth/login`,
            new URLSearchParams({ username, password }).toString(),
            {
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                httpsAgent: new https.Agent({ rejectUnauthorized: !config.platformApi?.insecure }),
                timeout: 10000,
            }
        );
        platformToken = response.data.token;
    } catch (error) {
        if (error.response?.status === 401) {
            return res.status(401).json({ message: 'Invalid credentials' });
        }
        logger.error('Platform API login request failed', { error: error.message });
        return res.status(503).json({ message: 'Authentication service unavailable' });
    }

    const claims = extractPlatformJwtClaims(platformToken, null);
    if (!claims?.org_handle) {
        logger.error('Platform API token missing required claims', { operation: 'devportalLogin' });
        return res.status(503).json({ message: 'Authentication service error' });
    }

    const scopes = claims.scopes;
    const adminRole = config.identityProvider.adminRole || 'admin';
    const subscriberRole = config.identityProvider.subscriberRole || 'Internal/subscriber';
    const isAdmin = scopes.some(s => s.endsWith('_manage'));

    const profile = {
        firstName: claims.username || username,
        lastName: '',
        email: claims.email || username,
        [constants.ROLES.ORGANIZATION_CLAIM]: claims.org_handle || '',
        [constants.ROLES.ROLE_CLAIM]: isAdmin ? [adminRole] : [subscriberRole],
        [constants.ROLES.GROUP_CLAIM]: [],
        [constants.USER_ID]: claims.sub || username,
        accessToken: platformToken,
        refreshToken: null,
        authorizedOrgs: [claims.org_handle || ''],
        userOrg: claims.org_handle || '',
        isAdmin,
        isSuperAdmin: false,
        isLocalAuth: true,
    };

    req.session.regenerate((err) => {
        if (err) {
            logger.error('Session regeneration failed', { error: err.message });
            return util.handleError(res, err);
        }
        req.logIn(profile, (loginErr) => {
            if (loginErr) {
                logger.error('Login session error', { error: loginErr.message });
                return util.handleError(res, loginErr);
            }
            req.session.save(() => res.status(200).json({ message: 'Login successful' }));
        });
    });
};
module.exports = {
    listApplications,
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
