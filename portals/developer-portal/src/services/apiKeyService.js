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
const { publish } = require('./webhooks/eventPublisher');
const subDao = require('../dao/subscriptionDao');
const platformClient = require('./platformApiClient');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');

function isPlatformApiPath(gatewayType) {
    return gatewayType === 'wso2/api-platform' && !!(config.platformApi?.baseUrl);
}

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
        gatewayType: dv.GATEWAY_TYPE || null,
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
        gatewayType: dv.GATEWAY_TYPE || null,
        apiName: dv.API_NAME || null,
        apiVersion: dv.API_VERSION || null,
        apiRefId: dv.REFERENCE_ID || ''
    };
}

async function resolveSubscription(orgId, subscriptionId) {
    if (!subscriptionId) return null;
    const sub = await subDao.getById(orgId, subscriptionId);
    if (!sub) return null;
    const policy = sub.DP_SUBSCRIPTION_POLICY;
    return {
        ref_id: sub.SUB_ID,
        plan_ref_id: policy ? (policy.REF_ID || null) : null,
        plan_name: policy ? (policy.POLICY_NAME || policy.DISPLAY_NAME || null) : null
    };
}

/**
 * Generate a new API key. Returns { keyId, name, plaintext, expiresAt, status }.
 * The plaintext is shown to the caller exactly once and never persisted.
 */
async function generate({ orgId, apiId, subscriptionId, name, expiresAt, actor, userToken }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const normalizedName = parseAndValidateName(name);
    if (!normalizedName) throw Object.assign(new Error('name must match ^[a-z0-9][a-z0-9_-]{0,127}$'), { status: 400 });

    const expiry = parseExpiresAt(expiresAt);
    if (!expiry.ok) throw Object.assign(new Error(expiry.description), { status: 400 });

    const api = await resolveApi(orgId, apiId);
    if (api.error) throw Object.assign(new Error(api.error.message), { status: api.error.status });

    let plaintext = generateSecret();
    const subscription = await resolveSubscription(orgId, subscriptionId);
    let keyId;

    if (isPlatformApiPath(api.gatewayType)) {
        // Call Platform API first; if it fails, nothing is persisted in devportal DB.
        await platformClient.createApiKey(userToken, {
            apiRefId: api.apiRefId,
            apiKey: plaintext,
            name: normalizedName,
        });
        try {
            await sequelize.transaction(async (t) => {
                const key = await apiKeyDao.create(
                    { apiId: api.apiId, subscriptionId, orgId, name: normalizedName,
                      expiresAt: expiry.date, createdBy: actor },
                    t
                );
                keyId = key.KEY_ID;
            });
        } catch (err) {
            plaintext = '\0'.repeat(plaintext.length);
            throw err;
        }
    } else {
        try {
            await sequelize.transaction(async (t) => {
                const key = await apiKeyDao.createKey(
                    { apiId: api.apiId, subscriptionId, orgId, name: normalizedName,
                      expiresAt: expiry.date, createdBy: actor },
                    t
                );
                keyId = key.KEY_ID;

                await publish('apikey.generated',
                    {
                        key_id: keyId,
                        name: normalizedName,
                        expires_at: expiry.date ? expiry.date.toISOString() : null,
                        api: { name: api.apiName, version: api.apiVersion, ref_id: api.apiRefId },
                        ...(subscription && { subscription })
                    },
                    { transaction: t, orgId, gatewayType: api.gatewayType,
                      aggregateType: 'apikey', aggregateId: keyId, plaintextKey: plaintext }
                );
            });
        } catch (err) {
            plaintext = '\0'.repeat(plaintext.length);
            throw err;
        }
    }

    logger.info('[apiKeyService] key generated', { keyId, orgId, apiId, actor });
    return { keyId, name: normalizedName, key: plaintext, expiresAt: expiry.date, status: 'ACTIVE' };
}

/**
 * Regenerate an existing key: same keyId, new secret, status stays ACTIVE.
 * The old secret is silently invalidated at the gateway side via the event.
 */
async function regenerate({ orgId, keyId, actor, userToken }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });
    if (existing.STATUS === 'REVOKED') throw Object.assign(new Error('Cannot regenerate a revoked key'), { status: 409 });

    const apiInfo = await resolveApiDirect(orgId, existing.API_ID);
    const gatewayType = apiInfo ? apiInfo.gatewayType : null;
    let plaintext = generateSecret();
    const subscription = await resolveSubscription(orgId, existing.SUBSCRIPTION_ID);

    if (isPlatformApiPath(gatewayType)) {
        try {
            await platformClient.updateApiKey(userToken, {
                apiRefId: apiInfo.apiRefId,
                keyName: existing.NAME,
                apiKey: plaintext,
            });
        } catch (err) {
            plaintext = '\0'.repeat(plaintext.length);
            throw err;
        }
    } else {
        try {
            await sequelize.transaction(async (t) => {
                await publish('apikey.regenerated',
                    {
                        key_id: keyId,
                        name: existing.NAME,
                        expires_at: existing.EXPIRES_AT ? new Date(existing.EXPIRES_AT).toISOString() : null,
                        api: { name: apiInfo ? apiInfo.apiName : null, version: apiInfo ? apiInfo.apiVersion : null, ref_id: apiInfo ? apiInfo.apiRefId : '' },
                        ...(subscription && { subscription })
                    },
                    { transaction: t, orgId, gatewayType,
                      aggregateType: 'apikey', aggregateId: keyId, plaintextKey: plaintext }
                );
            });
        } catch (err) {
            plaintext = '\0'.repeat(plaintext.length);
            throw err;
        }
    }

    logger.info('[apiKeyService] key regenerated', { keyId, orgId, actor });
    return { keyId, name: existing.NAME, key: plaintext, expiresAt: existing.EXPIRES_AT, status: 'ACTIVE' };
}

/**
 * Revoke a key. Fires apikey.revoked so gateways can reject it immediately.
 */
async function revoke({ orgId, keyId, actor, userToken }) {
    if (config.readOnlyMode) throw Object.assign(new Error('Read-only mode'), { status: 403 });

    const existing = await apiKeyDao.get(orgId, keyId);
    if (!existing) throw Object.assign(new Error('API key not found'), { status: 404 });

    const revokeApiInfo = await resolveApiDirect(orgId, existing.API_ID);
    const gatewayType = revokeApiInfo ? revokeApiInfo.gatewayType : null;
    const subscription = await resolveSubscription(orgId, existing.SUBSCRIPTION_ID);

    if (isPlatformApiPath(gatewayType)) {
        await platformClient.revokeApiKey(userToken, {
            apiRefId: revokeApiInfo.apiRefId,
            keyName: existing.NAME,
        });
        await sequelize.transaction(async (t) => {
            const revoked = await apiKeyDao.revoke(orgId, keyId, t);
            if (!revoked) throw Object.assign(new Error('Key already revoked or not found'), { status: 409 });
        });
    } else {
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
                { transaction: t, orgId, gatewayType,
                  aggregateType: 'apikey', aggregateId: keyId }
            );
        });
    }

    logger.info('[apiKeyService] key revoked', { keyId, orgId, actor });
}

async function list(orgId, filters) {
    return apiKeyDao.list(orgId, filters);
}

module.exports = { generate, regenerate, revoke, list };
