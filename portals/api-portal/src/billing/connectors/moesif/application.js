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
'use strict';

/*
 * Which Moesif application a Collector Application Id or a management key is for. Kept apart from
 * client.js so the config loader can check the configured keys before the rest of billing loads.
 */

const APPLICATION = /^\d+:\d+$/;

/** A base64url JSON object (a segment of a Moesif id or key), or null when it is not one. */
function decodeSegment(segment) {
    try {
        const value = JSON.parse(Buffer.from(segment, 'base64url').toString('utf8'));
        return value && typeof value === 'object' && !Array.isArray(value) ? value : null;
    } catch (_err) {
        return null;
    }
}

/**
 * The Moesif application ("524:75") a Collector Application ID belongs to, or null when the value
 * is not a Collector Application ID. The ID's first segment is JSON naming the application.
 */
function collectorApplication(applicationId) {
    const claims = decodeSegment(String(applicationId || '').trim().split('.')[0]);
    return claims && typeof claims.app === 'string' && APPLICATION.test(claims.app) ? claims.app : null;
}

/** The Moesif application a management key is for (its `app` claim), or null when it does not say. */
function keyApplication(managementKey) {
    const parts = String(managementKey || '').trim().split('.');
    const claims = parts.length === 3 ? decodeSegment(parts[1]) : null;
    return claims && typeof claims.app === 'string' && claims.app ? claims.app : null;
}

/**
 * Whether a management key says it is for another Moesif application than `application`. A key
 * that does not name its application is not.
 */
function keyForOtherApplication(managementKey, application) {
    const keyApp = keyApplication(managementKey);
    return Boolean(keyApp) && APPLICATION.test(application) && APPLICATION.test(keyApp) && keyApp !== application;
}

module.exports = { collectorApplication, keyApplication, keyForOtherApplication };
