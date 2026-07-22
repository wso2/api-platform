/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const crypto = require('crypto');
const db = require('../db/driver');
const apiKeyDao = require('../dao/apiKeyDao');
const apiDao = require('../dao/apiDao');
const applicationDao = require('../dao/applicationDao');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const { publish } = require('./webhooks/eventPublisher');
const subDao = require('../dao/subscriptionDao');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');

const KEY_HANDLE_PATTERN = /^[a-z0-9][a-z0-9_-]{0,127}$/;
const EXPIRES_AT_HAS_TZ = /(?:Z|[+-]\d{2}:\d{2})$/;
const MIN_EXPIRY_MS = Date.UTC(1970, 0, 1);
const MAX_EXPIRY_MS = Date.UTC(2100, 11, 31, 23, 59, 59, 999);

function generateSecret() {
    return crypto.randomBytes(32).toString('base64url');
}

function parseAndValidateHandle(raw) {
    if (typeof raw !== 'string') return null;
    const n = raw.trim();
    return KEY_HANDLE_PATTERN.test(n) ? n : null;
}

function parseExpiresAt(raw) {
    if (raw === undefined || raw === null || raw === '') return { ok: true, date: null };
    let ms;
    if (typeof raw === 'number' && Number.isFinite(raw)) {
        ms = raw < 1e12 ? Math.floor(raw * 1000) : Math.floor(raw);
    } else if (typeof raw === 'string') {
        const s = raw.trim();
        if (!s) return { ok: true, date: null };
        const asNum = Number(s);
        if (Number.isFinite(asNum) && String(asNum) === s) {
            ms = asNum < 1e12 ? Math.floor(asNum * 1000) : Math.floor(asNum);
        } else {
            if (!EXPIRES_AT_HAS_TZ.test(s)) {
                return { ok: false, description: 'expiresAt must include timezone (Z or +HH:MM)' };
            }
            ms = Date.parse(s);
        }
    } else {
        return { ok: false, description: 'expiresAt must be a string, number, or omitted' };
    }
    if (Number.isNaN(ms)) return { ok: false, description: 'expiresAt is invalid' };
    if (ms < MIN_EXPIRY_MS || ms > MAX_EXPIRY_MS) return { ok: false, description: 'expiresAt is out of allowed range' };
    return { ok: true, date: new Date(ms) };
}

async function resolveApi(orgId, apiId) {
    const rows = await apiDao.get(orgId, apiId);
    if (!rows || rows.length === 0) {
        return { error: { status: 404, message: 'API not found' } };
    }
    const row = rows[0];
    return {
        id: row.uuid,
        name: row.name || null,
        version: row.version || null,
        refId: row.ref_id || '',
        type: row.type || null
    };
}

async function resolveApiDirect(orgId, apiId) {
    const rows = await apiDao.getByCondition({ orgId, uuid: apiId });
    if (!rows || rows.length === 0) return null;
    const row = rows[0];
    return {
        name: row.name || null,
        version: row.version || null,
        refId: row.ref_id || '',
        type: row.type || null
    };
}

async function resolveSubscription(orgId, subscriptionId) {
    if (!subscriptionId) return null;
    const sub = await subDao.getById(orgId, subscriptionId);
    if (!sub) return null;
    const plan = sub.dp_subscription_plan;
    return {
        ref_id: sub.uuid,
        plan_ref_id: plan ? (plan.ref_id || null) : null,
        plan_name: plan ? (plan.display_name || null) : null
    };
}

/**
 * Validate that an app exists, belongs to orgId, and was created by actor.
 * Returns { id, display_name, handle } or null. Throws 404 only when an appId was actually given.
 */
async function resolveApp(orgId, appId, actor) {
    if (!appId) return null;
    const app = await applicationDao.get(orgId, appId, actor);
    if (!app) throw Object.assign(new Error('Application not found'), { status: 404 });
    return { id: app.uuid, display_name: app.display_name, handle: app.handle };
}

function applicationOf(key) {
    const app = key.dp_api_key_app_mapping?.dp_application;
    return app ? { id: app.uuid, display_name: app.display_name, handle: app.handle } : null;
}

/**
 * Publish apikey.application_updated for a single key — { key_id, handle, display_name, api, application }.
 * `application` is { id, display_name, handle } when associated, or null when cleared.
 */
async function publishKeyApplicationUpdated(orgId, keyId, handle, displayName, api, application, transaction) {
    await publish('apikey.application_updated',
        { key_id: keyId, handle, display_name: displayName, api, application },
        { transaction, orgId, aggregateType: 'apikey', aggregateId: keyId }
    );
}

/**
 * Generate a new API key. Returns { keyId, id, displayName, key, expiresAt, status }.
 * The plaintext is shown to the caller exactly once and never persisted.
 */
async function generate({ orgId, apiId, subscriptionId, appId, handle, displayName, expiresAt, actor }) {
    if (config.server.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const normalizedHandle = parseAndValidateHandle(handle);
    if (!normalizedHandle) throw Object.assign(new Error('id must match ^[a-z0-9][a-z0-9_-]{0,127}$'), { status: 400 });
    const normalizedDisplayName = typeof displayName === 'string' && displayName.trim() ? displayName.trim() : normalizedHandle;

    const expiry = parseExpiresAt(expiresAt);
    if (!expiry.ok) throw Object.assign(new Error(expiry.description), { status: 400 });

    const api = await resolveApi(orgId, apiId);
    if (api.error) throw Object.assign(new Error(api.error.message), { status: api.error.status });

    const application = await resolveApp(orgId, appId, actor);

    let plaintext = generateSecret();
    const subscription = await resolveSubscription(orgId, subscriptionId);
    let keyId;
    let audit;

    try {
        await db.withTransaction(async (t) => {
            const key = await apiKeyDao.create(
                { apiId: api.id, subscriptionId, appId: application ? application.id : null, orgId,
                  handle: normalizedHandle, displayName: normalizedDisplayName, expiresAt: expiry.date, createdBy: actor },
                t
            );
            keyId = key.uuid;
            audit = await userIdpReferenceDao.buildSingleAuditFields(key);

            await publish('apikey.generated',
                {
                    key_id: keyId,
                    handle: normalizedHandle,
                    display_name: normalizedDisplayName,
                    expires_at: expiry.date ? expiry.date.toISOString() : null,
                    api: { name: api.name, version: api.version, ref_id: api.refId, type: api.type },
                    ...(subscription && { subscription }),
                    ...(application && { application })
                },
                { transaction: t, orgId,
                  aggregateType: 'apikey', aggregateId: keyId, secretFields: { key: plaintext } }
            );

            if (application) {
                await publishKeyApplicationUpdated(orgId, keyId, normalizedHandle, normalizedDisplayName,
                    { name: api.name, version: api.version, ref_id: api.refId, type: api.type },
                    application, t);
            }
        });
    } catch (err) {
        plaintext = '\0'.repeat(plaintext.length);
        throw err;
    }

    logger.info('API key generated', { keyId, orgId, apiId, appId: application ? application.id : null, actor });
    return { keyId, id: normalizedHandle, displayName: normalizedDisplayName, key: plaintext, expiresAt: expiry.date, status: constants.API_KEY_STATUS.ACTIVE, ...audit };
}

/**
 * Regenerate an existing key: same keyId, new secret, status stays ACTIVE.
 * The old secret is silently invalidated by whatever consumes the webhook event.
 */
async function regenerate({ orgId, apiId, keyId, expiresAt, actor }) {
    if (config.server.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (apiId && existing.api_uuid !== apiId) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (existing.status === constants.API_KEY_STATUS.REVOKED) throw Object.assign(new Error('Cannot regenerate a revoked key'), { status: 409 });

    const expiry = parseExpiresAt(expiresAt);
    if (!expiry.ok) throw Object.assign(new Error(expiry.description), { status: 400 });
    const newExpiresAt = expiresAt === undefined ? existing.expires_at : expiry.date;

    const apiInfo = await resolveApiDirect(orgId, existing.api_uuid);
    let plaintext = generateSecret();
    const subscription = await resolveSubscription(orgId, existing.subscription_uuid);
    const application = applicationOf(existing);

    try {
        await db.withTransaction(async (t) => {
            if (expiresAt !== undefined) {
                const updated = await apiKeyDao.updateExpiry(orgId, keyId, newExpiresAt, actor, t);
                if (!updated) throw Object.assign(new Error('Cannot regenerate a revoked key'), { status: 409 });
            }

            await publish('apikey.regenerated',
                {
                    key_id: keyId,
                    handle: existing.handle,
                    display_name: existing.display_name,
                    expires_at: newExpiresAt ? new Date(newExpiresAt).toISOString() : null,
                    api: { name: apiInfo ? apiInfo.name : null, version: apiInfo ? apiInfo.version : null, ref_id: apiInfo ? apiInfo.refId : '', type: apiInfo ? apiInfo.type : null },
                    ...(subscription && { subscription }),
                    ...(application && { application })
                },
                { transaction: t, orgId,
                  aggregateType: 'apikey', aggregateId: keyId, secretFields: { key: plaintext } }
            );
        });
    } catch (err) {
        plaintext = '\0'.repeat(plaintext.length);
        throw err;
    }

    logger.info('API key regenerated', { keyId, orgId, actor });
    return { keyId, id: existing.handle, displayName: existing.display_name, key: plaintext, expiresAt: newExpiresAt, status: constants.API_KEY_STATUS.ACTIVE };
}

/**
 * Revoke a key. Fires apikey.revoked so webhook subscribers can reject it immediately.
 */
async function revoke({ orgId, apiId, keyId, actor }) {
    if (config.server.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (apiId && existing.api_uuid !== apiId) throw Object.assign(new Error('API key not found'), { status: 404 });

    const revokeApiInfo = await resolveApiDirect(orgId, existing.api_uuid);
    const subscription = await resolveSubscription(orgId, existing.subscription_uuid);

    await db.withTransaction(async (t) => {
        const revoked = await apiKeyDao.revoke(orgId, keyId, actor, t);
        if (!revoked) throw Object.assign(new Error('Key already revoked or not found'), { status: 409 });

        await publish('apikey.revoked',
            {
                key_id: keyId,
                handle: existing.handle,
                display_name: existing.display_name,
                api: { name: revokeApiInfo ? revokeApiInfo.name : null, version: revokeApiInfo ? revokeApiInfo.version : null, ref_id: revokeApiInfo ? revokeApiInfo.refId : '', type: revokeApiInfo ? revokeApiInfo.type : null },
                ...(subscription && { subscription })
            },
            { transaction: t, orgId,
              aggregateType: 'apikey', aggregateId: keyId }
        );
    });

    logger.info('API key revoked', { keyId, orgId, actor });
}

async function list(orgId, filters, transaction) {
    return apiKeyDao.list(orgId, filters, transaction);
}

/**
 * Associate (or re-associate) an existing key with an app. Optional, analytics-only —
 * does not affect the key's validity. Publishes one apikey.application_updated event
 * for this key; the previously-associated app (if any) needs no event of its own since
 * none of its other keys are affected.
 */
async function associateApplication({ orgId, apiId, keyId, appId, actor }) {
    if (config.server.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (apiId && existing.api_uuid !== apiId) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (existing.status === constants.API_KEY_STATUS.REVOKED) throw Object.assign(new Error('Cannot associate a revoked key'), { status: 409 });

    const application = await resolveApp(orgId, appId, actor);
    if (!application) throw Object.assign(new Error('appId is required'), { status: 400 });

    await db.withTransaction(async (t) => {
        const updated = await apiKeyDao.setApplication(orgId, keyId, application.id, actor, t, { activeOnly: true });
        if (!updated) throw Object.assign(new Error('API key not found'), { status: 404 });

        const meta = existing.dp_api_metadata;
        const api = { name: meta.name || null, version: meta.version || null, ref_id: meta.ref_id || '', type: meta.type || null };
        await publishKeyApplicationUpdated(orgId, keyId, existing.handle, existing.display_name, api, application, t);
    });

    logger.info('API key associated to application', { keyId, orgId, appId: application.id, actor });
    return { keyId, application: { id: application.handle, displayName: application.display_name } };
}

/**
 * Remove a key's app association, if any. No-op (but not an error) if the key
 * had no app associated.
 */
async function removeApplicationAssociation({ orgId, apiId, keyId, actor }) {
    if (config.server.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (apiId && existing.api_uuid !== apiId) throw Object.assign(new Error('API key not found'), { status: 404 });

    if (!existing.dp_api_key_app_mapping) return { keyId, application: null };

    await db.withTransaction(async (t) => {
        await apiKeyDao.setApplication(orgId, keyId, null, actor, t);
        const meta = existing.dp_api_metadata;
        const api = { name: meta.name || null, version: meta.version || null, ref_id: meta.ref_id || '', type: meta.type || null };
        await publishKeyApplicationUpdated(orgId, keyId, existing.handle, existing.display_name, api, null, t);
    });

    logger.info('API key application association removed', { keyId, orgId, actor });
    return { keyId, application: null };
}

module.exports = {
    generate, regenerate, revoke, list,
    associateApplication, removeApplicationAssociation,
    publishKeyApplicationUpdated
};
