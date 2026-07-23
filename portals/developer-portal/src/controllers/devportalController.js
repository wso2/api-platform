 
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
const logger = require('../config/logger');
const util = require('../utils/util');
const appDao = require('../dao/applicationDao');
const apiKeyService = require('../services/apiKeyService');
const { publish } = require('../services/webhooks/eventPublisher');
const db = require('../db/driver');
const constants = require('../utils/constants');
const { ApplicationDTO } = require('../dto/applicationDto');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const { CustomError, NotFoundError } = require('../utils/errors/customErrors');
const yaml = require('../utils/yaml');
const kmDao = require('../dao/keyManagerDao');
const { generateToken } = require('../services/oauthTokenService');
const { logUserAction } = require('../middlewares/auditLogger');
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
            handle: parsed.metadata?.name,
        };
    }
    const { id, ...rest } = req.body;
    return { ...rest, handle: id };
}

// ***** Save Application *****

const listApplications = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = util.resolveActor(req);
    try {
        const applications = await appDao.list(orgId, userId);
        const auditList = await userIdpReferenceDao.buildListAuditFields(applications);
        const dtos = applications.map((a, i) => new ApplicationDTO(a, auditList[i]));
        return res.status(200).json(util.toPaginatedList(dtos, req));
    } catch (error) {
        logger.error('Error occurred while listing applications', { orgId: orgId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

const saveApplication = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = util.resolveActor(req);
    try {
        const applicationData = parseApplicationDataFromRequest(req);
        const application = await appDao.create(orgId, userId, applicationData);
        const createdApp = application;
        try {
            await db.withTransaction((t) => publish('application.created',
                { application_id: createdApp.uuid, display_name: createdApp.display_name, handle: createdApp.handle, description: createdApp.description, type: 'web' },
                { transaction: t, orgId: orgId, aggregateType: 'application', aggregateId: createdApp.uuid }
            ));
        } catch (pubErr) {
            logger.warn('Failed to publish application.created', { orgId: orgId, appId: createdApp.uuid, error: pubErr.message });
        }
        logUserAction('APPLICATION_CREATED', req, { orgId, appId: createdApp.uuid, resourceUuid: createdApp.uuid, resourceType: 'application' });
        const audit = await userIdpReferenceDao.buildSingleAuditFields(createdApp);
        return res.status(201).json(new ApplicationDTO(createdApp, audit));
    } catch (error) {
        logger.error('Error occurred while creating the application', { orgId: orgId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

// ***** Update Application *****

const updateApplication = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = util.resolveActor(req);
    try {
        const appHandle = req.params.applicationId;
        const appRecord = await appDao.getId(orgId, userId, appHandle);
        if (!appRecord) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        const appId = appRecord.uuid;
        const applicationData = parseApplicationDataFromRequest(req);
        const [updatedRows, updatedApp] = await appDao.update(orgId, appId, userId, applicationData);
        if (!updatedRows) {
            throw new NotFoundError("No record found to update");
        }
        try {
            const renamedApp = updatedApp[0];
            await db.withTransaction(async (t) => {
                await publish('application.updated',
                    { application_id: appId, display_name: renamedApp.display_name, handle: renamedApp.handle, description: renamedApp.description, type: 'web' },
                    { transaction: t, orgId: orgId, aggregateType: 'application', aggregateId: appId }
                );
            });
        } catch (pubErr) {
            logger.warn('Failed to publish webhook events after app update', { orgId: orgId, appId: appId, error: pubErr.message });
        }
        logUserAction('APPLICATION_UPDATED', req, { orgId, appId, resourceUuid: appId, resourceType: 'application' });
        const audit = await userIdpReferenceDao.buildSingleAuditFields(updatedApp[0]);
        res.status(200).send(new ApplicationDTO(updatedApp[0], audit));
    } catch (error) {
        logger.error("Error occurred while updating the application", { orgId: orgId, error: error.message, stack: error.stack });
        util.handleError(res, error);
    }
};

// ***** Delete Application *****

const revokeAppKeyMappings = async (orgId, appId, t) => {
    const mappings = await appDao.getKeyMappings(orgId, appId, t);
    const mappingIds = mappings.map((mapping) => mapping.uuid);
    await appDao.deleteMappingsByIds(orgId, mappingIds, t);
};

/**
 * Publishes application.deleted + a per-key apikey.application_updated(null) for each
 * previously-associated key. Must be called only after the application row (and its
 * app_uuid references) have actually been deleted — best-effort, never throws.
 */
const publishApplicationDeletedEvents = async (orgId, applicationId, appToDelete, affectedKeys) => {
    try {
        await db.withTransaction(async (t) => {
            if (appToDelete) {
                await publish('application.deleted',
                    { application_id: applicationId, display_name: appToDelete.display_name, handle: appToDelete.handle },
                    { transaction: t, orgId: orgId, aggregateType: 'application', aggregateId: applicationId }
                );
            }
            for (const key of affectedKeys) {
                const meta = key.dp_api_metadata;
                const api = { name: meta.name || null, version: meta.version || null, ref_id: meta.ref_id || '', type: meta.type || null };
                await apiKeyService.publishKeyApplicationUpdated(orgId, key.uuid, key.handle, key.display_name, api, null, t);
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
    await db.withTransaction(async (t) => {
        appToDelete = await appDao.get(orgId, applicationId, userId, t);
        affectedKeys = await apiKeyService.list(orgId, { appId: applicationId }, t);
        await revokeAppKeyMappings(orgId, applicationId, t);
        await appDao.delete(orgId, applicationId, userId, t);
    });
    return { appToDelete, affectedKeys };
};

/**
 * Revokes key mappings, deletes the application row, and publishes the resulting
 * deletion events. Shared by deleteApplication's initial attempt and its
 * already-deleted (404) retry path.
 */
const finalizeApplicationDeletion = async (orgId, applicationId, userId, req) => {
    const { appToDelete, affectedKeys } = await deleteApplicationAndSnapshotKeys(orgId, applicationId, userId);
    logUserAction('APPLICATION_DELETED', req, { orgId, appId: applicationId, resourceUuid: applicationId, resourceType: 'application' });
    await publishApplicationDeletedEvents(orgId, applicationId, appToDelete, affectedKeys);
};

const getApplication = async (req, res) => {
    const orgId = req.orgId || '';
    const userId = util.resolveActor(req);
    const applicationHandle = req.params.applicationId;
    try {
        const appRecord = await appDao.getId(orgId, userId, applicationHandle);
        if (!appRecord) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        const app = await appDao.get(orgId, appRecord.uuid, userId);
        if (!app) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        const audit = await userIdpReferenceDao.buildSingleAuditFields(app);
        return res.status(200).json(new ApplicationDTO(app, audit));
    } catch (error) {
        logger.error('Error occurred while getting the application', { orgId, appId: applicationHandle, error: error.message });
        util.handleError(res, error);
    }
};

const deleteApplication = async (req, res) => {
    const userId = util.resolveActor(req);
    const applicationHandle = req.params.applicationId;
    const orgId = req.orgId || '';
    let applicationId;
    try {
        const appRecord = await appDao.getId(orgId, userId, applicationHandle);
        if (!appRecord) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        applicationId = appRecord.uuid;
        const ownedApp = await appDao.get(orgId, applicationId, userId);
        if (!ownedApp) {
            return res.status(404).json({ status: 'error', code: '404', message: 'Application not found' });
        }
        try {
            await finalizeApplicationDeletion(orgId, applicationId, userId, req);
            res.status(200).send("Resource Deleted Successfully");
        } catch (error) {
            if (error.statusCode === 404) {
                await finalizeApplicationDeletion(orgId, applicationId, userId, req);
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
        userId = util.resolveActor(req);
        const appHandle = req.params.applicationId;
        const appRecord = await appDao.getId(orgId, userId, appHandle);
        if (!appRecord) {
            return util.sendError(res, 404, 'Application not found');
        }
        appId = appRecord.uuid;
        logger.info('Initiate create application key mapping...', { orgId: orgId, appId: appId });
        const {
            keyManager: kmName,
            type: rawKeyType,
            consumerKey,
        } = req.body;

        if (!consumerKey) {
            return util.sendError(res, 400, 'consumerKey is required.');
        }

        const kmRecord = await kmDao.getByHandle(orgId, kmName);
        if (!kmRecord) {
            return util.sendError(res, 404, `Key manager '${kmName}' not found.`);
        }

        const keyType = (rawKeyType || constants.KEY_TYPE.PRODUCTION).toUpperCase();
        if (!Object.values(constants.KEY_TYPE).includes(keyType)) {
            return util.sendError(res, 400, `Invalid type. Must be one of: ${Object.values(constants.KEY_TYPE).join(', ')}.`);
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
            keyMappingId: keyMappingRecord?.uuid,
        };

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
        const orgId = req.orgId;
        const userId = util.resolveActor(req);
        const applicationHandle = req.params.applicationId;
        const appRecord = await appDao.getId(orgId, userId, applicationHandle);
        if (!appRecord) {
            throw new CustomError(404, 'Application not found');
        }
        const applicationId = appRecord.uuid;
        const keyMappingId = req.params.keyMappingId;

        const keyMapping = await appDao.getKeyMappingById(applicationId, keyMappingId);
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
        const orgId = req.orgId;
        const userId = util.resolveActor(req);
        const applicationHandle = req.params.applicationId;
        const appRecord = await appDao.getId(orgId, userId, applicationHandle);
        if (!appRecord) {
            throw new CustomError(404, 'Application not found');
        }
        const applicationId = appRecord.uuid;
        const keyMappingId = req.params.keyMappingId;

        const deletedRows = await appDao.deleteKeyMappingById(applicationId, keyMappingId);
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
