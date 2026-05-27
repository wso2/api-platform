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

/**
 * @fileoverview Telemetry helper utilities for tracking events with default properties.
 * This module provides a centralized way to track events with consistent default
 * metadata across multiple telemetry services (Application Insights and Moesif).
 */

const { telemetryClient } = require('./telemetryClient');
const moesif = require('../middlewares/moesif');
const { config } = require('../config/configLoader');

/**
 * Email suffix used to identify WSO2 internal users.
 * Used to flag internal vs external user activity in analytics.
 * @constant {string}
 */
const WSO2_EMAIL_SUFFIX = '@wso2.com';

/**
 * Default properties that are automatically added to all tracked events.
 * These properties provide consistent context and origin information for analytics.
 */
const DEFAULT_PROPERTIES = {
    context: 'devportal',
    origin: 'bijira',
};

/**
 * Default properties to be sent with Moesif telemetry events.
 */
const MOESIF_DEFAULT_PROPERTIES = {
    product: 'api',
    asset_type: 'console',
    deployment_model: 'saas',
    domain: 'console.bijira.dev',
    context: 'api-portal'
};

/**
 * A mapping of internal event keys to their corresponding Moesif event names.
 * Used to standardize event tracking across the developer portal.
 */
const MOESIF_EVENT_MAP = {
    'login': { name: 'Landing-SignIn-Succeeded' },
    'logout': { name: 'Landing-SignOut-Succeeded' },
    'home-page-visit': { name: 'Portal-Viewed-Home' },
    'application-create-start': { name: 'Application-Created-Start' },
    'application-create-end': { name: 'Application-Created-End' },
    'application-delete': { name: 'Application-Deleted' },
    'generate-credentials': { name: 'Application-Generated-Credentials' },
    'generate-key': { name: 'Application-Generated-Key' },
    'subscribe-api': { name: 'API-Subscribed' },
    'unsubscribe-api': { name: 'API-Unsubscribed' }
}

/**
 * Converts all keys of an object from camelCase to snake_case.
 *
 * @param {Object} obj - The object whose keys need to be converted to snake_case.
 * @returns {Object} A new object with all keys converted to snake_case.
 */
function convertKeysToSnakeCase(obj) {
    return Object.entries(obj).reduce((acc, [key, value]) => {
        const snakeKey = key.replace(/([A-Z])/g, '_$1').toLowerCase();
        acc[snakeKey] = value;
        return acc;
    }, {});
 }

/**
 * Tracks an event with default properties across multiple telemetry services.
 * This function serves as a central point for event tracking, automatically adding
 * default properties and sending events to both Application Insights and Moesif
 * analytics services. Telemetry tracking can be disabled via configuration.
 * 
 * @param {Object} event - The event object to track
 * @param {Object} req - Express request object containing user session and context
 */
function trackEventWithDefaults(event, req) {
    if (!config.telemetry?.enabled) {
        return;
    }
    
    const email = req?.session?.passport?.user?.email || '';
    const isWSO2User = !!email.endsWith(WSO2_EMAIL_SUFFIX);
    const enrichedEvent = {
        name: event.name,
        properties: {
            isWSO2User: isWSO2User,
            ...DEFAULT_PROPERTIES,
            ...(event.properties || {}),
        },
    };

    // AppInsights
    telemetryClient.trackEvent(enrichedEvent);

    // Moesif legacy
    moesif.trackEvent(enrichedEvent.name, enrichedEvent.properties, req);

    // Moesif v2
    const moesifEvent = MOESIF_EVENT_MAP[event.name];
    if (moesifEvent) {
        moesif.trackEvent(moesifEvent.name, {
            is_wso2_user: isWSO2User,
            ...MOESIF_DEFAULT_PROPERTIES,
            ...convertKeysToSnakeCase(event.properties || {}),
            ...(moesifEvent.properties || {}),
        }, req);
    }
}

/**
 * Module exports for telemetry helper functionality.
 * @namespace TelemetryHelper
 */
module.exports = {
    /**
     * Function to track events with default properties across multiple telemetry services.
     * @function
     * @see trackEventWithDefaults
     */
    trackEventWithDefaults,
};
