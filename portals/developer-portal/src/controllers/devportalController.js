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
        const displayName = spec.displayName;
        if (!displayName) {
            throw new CustomError(400, "Bad Request", "Missing required application field: displayName");
        }
        return {
            displayName,
            description: spec.description,
            handle: spec.handle || parsed.metadata?.name,
        };
    }
    return req.body;
}

// ***** Save Application *****

const listApplications = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = req.auth?.userId || req.user?.sub;
    try {
        const applications = await appDao.list(orgId, userId);
        return res.status(200).json(util.toPaginatedList(applications.map(a => new ApplicationDTO(a.dataValues)), req));
    } catch (error) {
        logger.error('Error occurred while listing applications', { orgId: orgId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

const saveApplication = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = req.auth?.userId || req.user?.sub;
    try {
        const applicationData = parseApplicationDataFromRequest(req);
        trackAppCreationStart({ orgId: orgId, appName: applicationData.displayName, idpId: userId }, req);
        const application = await appDao.create(orgId, userId, applicationData);
        trackAppCreationEnd({ orgId: orgId, appName: applicationData.displayName, idpId: userId }, req);
        const createdApp = application.dataValues;
        try {
            await sequelize.transaction((t) => publish('application.created',
                { application_id: createdApp.uuid, display_name: createdApp.display_name, handle: createdApp.handle, description: createdApp.description, type: 'web' },
                { transaction: t, orgId: orgId, aggregateType: 'application', aggregateId: createdApp.uuid }
            ));
        } catch (pubErr) {
            logger.warn('Failed to publish application.created', { orgId: orgId, appId: createdApp.uuid, error: pubErr.message });
        }
        return res.status(201).json(new ApplicationDTO(createdApp));
    } catch (error) {
        logger.error('Error occurred while creating the application', { orgId: orgId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

// ***** Update Application *****

const updateApplication = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = req.auth?.userId || req.user?.sub;
    try {
        const appId = req.params.applicationId;
        const applicationData = parseApplicationDataFromRequest(req);
        const [updatedRows, updatedApp] = await appDao.update(orgId, appId, userId, applicationData);
        if (!updatedRows) {
            throw new Sequelize.EmptyResultError("No record found to update");
        }
        try {
            const renamedApp = updatedApp[0].dataValues;
            await sequelize.transaction(async (t) => {
                await publish('application.updated',
                    { application_id: appId, display_name: renamedApp.display_name, handle: renamedApp.handle, description: renamedApp.description, type: 'web' },
                    { transaction: t, orgId: orgId, aggregateType: 'application', aggregateId: appId }
                );
            });
        } catch (pubErr) {
            logger.warn('Failed to publish webhook events after app update', { orgId: orgId, appId: appId, error: pubErr.message });
        }
        res.status(200).send(new ApplicationDTO(updatedApp[0].dataValues));
    } catch (error) {
        logger.error("Error occurred while updating the application", { orgId: orgId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

// ***** Delete Application *****

const revokeAppKeyMappings = async (orgId, appId) => {
    const { ApplicationKeyMapping } = require('../models/application');
    const mappings = await ApplicationKeyMapping.findAll({
        where: { app_uuid: appId },
    });
    const mappingIds = mappings.map((mapping) => mapping.uuid);
    await appDao.deleteMappingsByIds(orgId, mappingIds);
};

/**
 * Publishes application.deleted + a per-key apikey.application_updated(null) for each
 * previously-associated key. Must be called only after the application row (and its
 * app_uuid references) have actually been deleted — best-effort, never throws.
 */
const publishApplicationDeletedEvents = async (orgId, applicationId, appToDelete, affectedKeys) => {
    try {
        await sequelize.transaction(async (t) => {
            if (appToDelete) {
                await publish('application.deleted',
                    { application_id: applicationId, display_name: appToDelete.display_name, handle: appToDelete.handle },
                    { transaction: t, orgId: orgId, aggregateType: 'application', aggregateId: applicationId }
                );
            }
            for (const key of affectedKeys) {
                const meta = key.dp_api_metadata;
                const api = { name: meta.name || null, version: meta.version || null, ref_id: meta.ref_id || '', type: meta.type || null };
                await apiKeyService.publishKeyApplicationUpdated(orgId, key.uuid, key.name, api, null, t);
            }
        });
    } catch (pubErr) {
        logger.warn('Failed to publish webhook events after app deletion', { orgId: orgId, appId: applicationId, error: pubErr.message });
    }
};

/**
 * Snapshots the app name + currently-associated keys and deletes the application row,
 * all inside one transaction — so the snapshot exactly matches what's actually deleted,
 * with no race window for a concurrent associate/dissociate call to go unnoticed.
 */
const deleteApplicationAndSnapshotKeys = async (orgId, applicationId, userId) => {
    let appToDelete = null;
    let affectedKeys = [];
    await sequelize.transaction(async (t) => {
        appToDelete = await appDao.get(orgId, applicationId, userId, t);
        affectedKeys = await apiKeyService.list(orgId, { appId: applicationId }, t);
        await appDao.delete(orgId, applicationId, userId, t);
    });
    return { appToDelete, affectedKeys };
};

const getApplication = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = req.auth?.userId || req.user?.sub;
    const applicationId = req.params.applicationId;
    try {
        const app = await appDao.get(orgId, applicationId, userId);
        if (!app) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        return res.status(200).json(new ApplicationDTO(app.dataValues));
    } catch (error) {
        logger.error('Error occurred while getting the application', { orgId, appId: applicationId, error: error.message });
        util.handleError(res, error);
    }
};

const deleteApplication = async (req, res) => {
    const userId = req.auth?.userId || req.user?.sub;
    const applicationId = req.params.applicationId;
    const orgId = req.orgId || '';
    try {
        const ownedApp = await appDao.get(orgId, applicationId, userId);
        if (!ownedApp) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        try {
            await revokeAppKeyMappings(orgId, applicationId);
            const { appToDelete, affectedKeys } = await deleteApplicationAndSnapshotKeys(orgId, applicationId, userId);
            trackAppDeletion({ orgId: orgId, appId: applicationId, idpId: userId }, req);
            await publishApplicationDeletedEvents(orgId, applicationId, appToDelete, affectedKeys);
            res.status(200).send("Resource Deleted Successfully");
        } catch (error) {
            if (error.statusCode === 404) {
                await revokeAppKeyMappings(orgId, applicationId);
                const { appToDelete, affectedKeys } = await deleteApplicationAndSnapshotKeys(orgId, applicationId, userId);
                trackAppDeletion({ orgId: orgId, appId: applicationId, idpId: userId }, req);
                await publishApplicationDeletedEvents(orgId, applicationId, appToDelete, affectedKeys);
                return res.status(200).send("Resource Deleted Successfully");
            }
            logger.error('Error occurred while deleting the application', { orgId: orgId, appId: applicationId, error: error.message, stack: error.stack });
            util.handleError(res, error);
        }
    } catch (error) {
        logger.error('Error occurred while deleting the application', { appId: applicationId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
}

const generateKeys = async (req, res) => {
    let orgId, appId, userId;
    try {
        orgId = req.orgId;
        appId = req.params.applicationId;
        userId = req.auth?.userId || req[constants.USER_ID] || req.user?.sub;
        logger.info('Initiate create application key mapping...', { orgId: orgId, appId: appId });
        const {
            keyManager: kmName,
            type: rawKeyType,
            consumerKey,
        } = req.body;

        if (!consumerKey) {
            return res.status(400).json({ message: 'consumerKey is required.' });
        }

        const kmRecord = await kmDao.getByName(orgId, kmName);
        if (!kmRecord) {
            return res.status(404).json({ message: `Key manager '${kmName}' not found.` });
        }

        const keyType = (rawKeyType || constants.KEY_TYPE.PRODUCTION).toUpperCase();
        if (!Object.values(constants.KEY_TYPE).includes(keyType)) {
            return res.status(400).json({ message: `Invalid type. Must be one of: ${Object.values(constants.KEY_TYPE).join(', ')}.` });
        }

        const appKeyMapping = {
            orgId,
            appId,
            kmId: kmRecord.uuid,
            asClientId: consumerKey,
            type: keyType,
            createdBy: userId,
        };
        const keyMappingRecord = await appDao.upsertKeyMapping(appKeyMapping);

        const responseData = {
            consumerKey,
            keyManager: kmName,
            type: keyType,
            tokenEndpoint: kmRecord.token_endpoint,
            keyMappingId: keyMappingRecord?.dataValues?.uuid,
        };

        trackGenerateCredentials({
            orgId: orgId,
            appName: appId,
            idpId: req.isAuthenticated() ? userId : undefined
        }, req);
        return res.status(200).json(responseData);
    } catch (error) {
        logger.error('key mapping create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgId,
            appId: appId
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
            where: { uuid: keyMappingId, app_uuid: applicationId },
        });
        if (!keyMapping || !keyMapping.km_uuid) {
            throw new CustomError(404, 'Key mapping not found or missing key manager reference');
        }
        const kmRecord = await kmDao.get(keyMapping.km_uuid);
        if (!kmRecord) {
            throw new CustomError(404, 'Key manager not found');
        }
        const { consumerSecret, scopes, validityPeriod } = req.body;
        const tokenResult = await generateToken(
            kmRecord.token_endpoint,
            keyMapping.as_client_id,
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
            where: { uuid: keyMappingId, app_uuid: applicationId },
        });
        if (!deletedRows) {
            throw new CustomError(404, 'Key mapping not found');
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

module.exports = {
    listApplications,
    getApplication,
    saveApplication,
    updateApplication,
    deleteApplication,
    generateKeys,
    generateOAuthKeys,
    revokeOAuthKeys,
};
