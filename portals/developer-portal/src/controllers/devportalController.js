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
const { generateToken } = require('../services/oauthTokenService');
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
    const mappingIds = mappings.map((mapping) => mapping.UUID);
    await appDao.deleteMappingsByIds(orgID, mappingIds);
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
            consumerKey,
        } = req.body;

        if (!consumerKey) {
            return res.status(400).json({ message: 'consumerKey is required.' });
        }

        const kmRecord = await kmDao.getByName(orgID, kmName);

        const keyType = (rawKeyType || constants.KEY_TYPE.PRODUCTION).toUpperCase();
        if (!Object.values(constants.KEY_TYPE).includes(keyType)) {
            return res.status(400).json({ message: `Invalid keyType. Must be one of: ${Object.values(constants.KEY_TYPE).join(', ')}.` });
        }

        const appKeyMapping = {
            orgID,
            appID,
            kmID: kmRecord.UUID,
            asClientID: consumerKey,
            keyType,
            createdBy: userID,
        };
        const keyMappingRecord = await appDao.upsertKeyMapping(appKeyMapping);

        const responseData = {
            consumerKey,
            keyManager: kmName,
            keyType,
            tokenEndpoint: kmRecord.TOKEN_ENDPOINT,
            keyMappingId: keyMappingRecord?.dataValues?.UUID,
        };

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
        const { consumerSecret, scopes, validityPeriod } = req.body;
        const tokenResult = await generateToken(
            kmRecord.TOKEN_ENDPOINT,
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
        const deletedRows = await ApplicationKeyMapping.destroy({
            where: { UUID: keyMappingId, APP_UUID: applicationId },
        });
        if (!deletedRows) {
            return util.handleError(res, { statusCode: 404, message: 'Key mapping not found' });
        }
        res.status(200).json({ message: 'Application key mapping removed successfully' });
    } catch (error) {
        logger.error("Error occurred while revoking the OAuth keys", {
            appId: req.params.applicationId,
            error: error.message,
            stack: error.stack
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
    login
};
