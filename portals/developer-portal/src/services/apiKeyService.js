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
const sequelize = require('../db/sequelizeConfig');
const apiKeyDao = require('../dao/apiKeyDao');
const apiDao = require('../dao/apiDao');
const applicationDao = require('../dao/applicationDao');
const { publish } = require('./webhooks/eventPublisher');
const subDao = require('../dao/subscriptionDao');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');

const KEY_NAME_PATTERN = /^[a-z0-9][a-z0-9_-]{0,127}$/;
const EXPIRES_AT_HAS_TZ = /(?:Z|[+-]\d{2}:\d{2})$/;
const MIN_EXPIRY_MS = Date.UTC(1970, 0, 1);
const MAX_EXPIRY_MS = Date.UTC(2100, 11, 31, 23, 59, 59, 999);

function generateSecret() {
    return 'ak_' + crypto.randomBytes(32).toString('base64').replace(/[+/=]/g, (c) =>
        c === '+' ? '-' : c === '/' ? '_' : '');
}

function parseAndValidateName(raw) {
    if (typeof raw !== 'string') return null;
    const n = raw.trim();
    return KEY_NAME_PATTERN.test(n) ? n : null;
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
    const dv = row.dataValues || row;
    return {
        apiId: dv.API_ID,
        apiName: dv.API_NAME || null,
        apiVersion: dv.API_VERSION || null,
        apiRefId: dv.REFERENCE_ID || ''
    };
}

async function resolveApiDirect(orgId, apiId) {
    const rows = await apiDao.getByCondition({ API_ID: apiId, ORG_ID: orgId });
    if (!rows || rows.length === 0) return null;
    const dv = rows[0].dataValues || rows[0];
    return {
        apiName: dv.API_NAME || null,
        apiVersion: dv.API_VERSION || null,
        apiRefId: dv.REFERENCE_ID || ''
    };
}

async function resolveSubscription(orgId, subscriptionId) {
    if (!subscriptionId) return null;
    const sub = await subDao.getById(orgId, subscriptionId);
    if (!sub) return null;
    const plan = sub.DP_SUBSCRIPTION_PLAN;
    return {
        ref_id: sub.SUB_ID,
        plan_ref_id: plan ? (plan.REF_ID || null) : null,
        plan_name: plan ? (plan.PLAN_NAME || plan.DISPLAY_NAME || null) : null
    };
}

/**
 * Validate that an app exists, belongs to orgId, and was created by actor.
 * Returns { id, name } or null. Throws 404 only when an appId was actually given.
 */
async function resolveApp(orgId, appId, actor) {
    if (!appId) return null;
    const app = await applicationDao.get(orgId, appId, actor);
    if (!app) throw Object.assign(new Error('Application not found'), { status: 404 });
    return { id: app.APP_ID, name: app.NAME };
}

function applicationOf(key) {
    const app = key.DP_APPLICATION;
    return app ? { id: app.APP_ID, name: app.NAME } : null;
}

/**
 * Publish apikey.application_updated for a single key — { key_id, application }.
 * `application` is { id, name } when associated, or null when cleared.
 */
async function publishKeyApplicationUpdated(orgId, keyId, application, transaction) {
    await publish('apikey.application_updated',
        { key_id: keyId, application },
        { transaction, orgId, aggregateType: 'apikey', aggregateId: keyId }
    );
}

/**
 * Fan out an application-level change (rename or delete) to every key currently
 * associated with that app, as individual per-key apikey.application_updated events.
 * `application` is { id, name } for a rename, or null for a delete.
 */
async function notifyApplicationKeysChanged(orgId, appId, application, transaction) {
    if (!appId) return;
    const keys = await apiKeyDao.list(orgId, { appId }, transaction);
    for (const key of keys) {
        await publishKeyApplicationUpdated(orgId, key.KEY_ID, application, transaction);
    }
}

/**
 * Generate a new API key. Returns { keyId, name, plaintext, expiresAt, status }.
 * The plaintext is shown to the caller exactly once and never persisted.
 */
async function generate({ orgId, apiId, subscriptionId, appId, name, expiresAt, actor }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const normalizedName = parseAndValidateName(name);
    if (!normalizedName) throw Object.assign(new Error('name must match ^[a-z0-9][a-z0-9_-]{0,127}$'), { status: 400 });

    const expiry = parseExpiresAt(expiresAt);
    if (!expiry.ok) throw Object.assign(new Error(expiry.description), { status: 400 });

    const api = await resolveApi(orgId, apiId);
    if (api.error) throw Object.assign(new Error(api.error.message), { status: api.error.status });

    const application = await resolveApp(orgId, appId, actor);

    let plaintext = generateSecret();
    const subscription = await resolveSubscription(orgId, subscriptionId);
    let keyId;

    try {
        await sequelize.transaction(async (t) => {
            const key = await apiKeyDao.create(
                { apiId: api.apiId, subscriptionId, appId: application ? application.id : null, orgId,
                  name: normalizedName, expiresAt: expiry.date, createdBy: actor },
                t
            );
            keyId = key.KEY_ID;

            await publish('apikey.generated',
                {
                    key_id: keyId,
                    name: normalizedName,
                    expires_at: expiry.date ? expiry.date.toISOString() : null,
                    api: { name: api.apiName, version: api.apiVersion, ref_id: api.apiRefId },
                    ...(subscription && { subscription }),
                    ...(application && { application })
                },
                { transaction: t, orgId,
                  aggregateType: 'apikey', aggregateId: keyId, plaintextKey: plaintext }
            );

            if (application) {
                await publishKeyApplicationUpdated(orgId, keyId, application, t);
            }
        });
    } catch (err) {
        plaintext = '\0'.repeat(plaintext.length);
        throw err;
    }

    logger.info('API key generated', { keyId, orgId, apiId, appId: application ? application.id : null, actor });
    return { keyId, name: normalizedName, key: plaintext, expiresAt: expiry.date, status: 'ACTIVE' };
}

/**
 * Regenerate an existing key: same keyId, new secret, status stays ACTIVE.
 * The old secret is silently invalidated by whatever consumes the webhook event.
 */
async function regenerate({ orgId, keyId, actor }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (existing.STATUS === 'REVOKED') throw Object.assign(new Error('Cannot regenerate a revoked key'), { status: 409 });

    const apiInfo = await resolveApiDirect(orgId, existing.API_ID);
    let plaintext = generateSecret();
    const subscription = await resolveSubscription(orgId, existing.SUBSCRIPTION_ID);
    const application = applicationOf(existing);

    try {
        await sequelize.transaction(async (t) => {
            await publish('apikey.regenerated',
                {
                    key_id: keyId,
                    name: existing.NAME,
                    expires_at: existing.EXPIRES_AT ? new Date(existing.EXPIRES_AT).toISOString() : null,
                    api: { name: apiInfo ? apiInfo.apiName : null, version: apiInfo ? apiInfo.apiVersion : null, ref_id: apiInfo ? apiInfo.apiRefId : '' },
                    ...(subscription && { subscription }),
                    ...(application && { application })
                },
                { transaction: t, orgId,
                  aggregateType: 'apikey', aggregateId: keyId, plaintextKey: plaintext }
            );
        });
    } catch (err) {
        plaintext = '\0'.repeat(plaintext.length);
        throw err;
    }

    logger.info('API key regenerated', { keyId, orgId, actor });
    return { keyId, name: existing.NAME, key: plaintext, expiresAt: existing.EXPIRES_AT, status: 'ACTIVE' };
}

/**
 * Revoke a key. Fires apikey.revoked so webhook subscribers can reject it immediately.
 */
async function revoke({ orgId, keyId, actor }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });

    const revokeApiInfo = await resolveApiDirect(orgId, existing.API_ID);
    const subscription = await resolveSubscription(orgId, existing.SUBSCRIPTION_ID);

    await sequelize.transaction(async (t) => {
        const revoked = await apiKeyDao.revoke(orgId, keyId, t);
        if (!revoked) throw Object.assign(new Error('Key already revoked or not found'), { status: 409 });

        await publish('apikey.revoked',
            {
                key_id: keyId,
                name: existing.NAME,
                api: { name: revokeApiInfo ? revokeApiInfo.apiName : null, version: revokeApiInfo ? revokeApiInfo.apiVersion : null, ref_id: revokeApiInfo ? revokeApiInfo.apiRefId : '' },
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
async function associateApplication({ orgId, keyId, appId, actor }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (existing.STATUS === 'REVOKED') throw Object.assign(new Error('Cannot associate a revoked key'), { status: 409 });

    const application = await resolveApp(orgId, appId, actor);
    if (!application) throw Object.assign(new Error('appId is required'), { status: 400 });

    await sequelize.transaction(async (t) => {
        const updated = await apiKeyDao.setApplication(orgId, keyId, application.id, t, { activeOnly: true });
        if (!updated) throw Object.assign(new Error('API key not found'), { status: 404 });

        await publishKeyApplicationUpdated(orgId, keyId, application, t);
    });

    logger.info('API key associated to application', { keyId, orgId, appId: application.id, actor });
    return { keyId, application };
}

/**
 * Remove a key's app association, if any. No-op (but not an error) if the key
 * had no app associated.
 */
async function removeApplicationAssociation({ orgId, keyId, actor }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });

    if (!existing.APP_ID) return { keyId, application: null };

    await sequelize.transaction(async (t) => {
        await apiKeyDao.setApplication(orgId, keyId, null, t);
        await publishKeyApplicationUpdated(orgId, keyId, null, t);
    });

    logger.info('API key application association removed', { keyId, orgId, actor });
    return { keyId, application: null };
}

module.exports = {
    generate, regenerate, revoke, list,
    associateApplication, removeApplicationAssociation, notifyApplicationKeysChanged,
    publishKeyApplicationUpdated
};
