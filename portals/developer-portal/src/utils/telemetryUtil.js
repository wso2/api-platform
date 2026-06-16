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
 * @fileoverview Telemetry tracking utilities for the API Developer Portal.
 * This module provides functions to track various user actions and events
 * for analytics and monitoring purposes. All tracking functions automatically
 * include timestamp and handle missing parameters with default values.
 */

const { trackEventWithDefaults } = require('./telemetryHelper');

/**
 * Default value used when a property is not available or undefined.
 * @constant {string}
 */
const NA = 'N/A';

/**
 * Enumeration of event names used for telemetry tracking.
 * These constants ensure consistent event naming across the application.
 * @enum {string}
 * @readonly
 */
const EventNames = {
    LOGIN: 'login',
    LOGOUT: 'logout',
    APP_CREATION_START: 'application-create-start',
    APP_CREATION_END: 'application-create-end',
    APP_DELETION: 'application-delete',
    GENERATE_CREDENTIALS: 'generate-credentials',
    GENERATE_KEY: 'generate-key',
    SUBSCRIBE_API: 'subscribe-api',
    UNSUBSCRIBE_API: 'unsubscribe-api',
    HOME_PAGE_VISIT: 'home-page-visit',
};

/**
 * Tracks user login events for telemetry purposes.
 * Records when a user successfully logs into the developer portal,
 * including organization context and timestamp.
 * 
 * @param {Object} params - Login tracking parameters
 * @param {string} [orgName] - Name of the organization the user is logging into
 * @param {Object} req - Express request object containing user session information
 */
function trackLoginTrigger({ orgName }, req) {
    trackEventWithDefaults({
        name: EventNames.LOGIN,
        properties: {
            organization: orgName || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks user logout events for telemetry purposes.
 * Records when a user logs out of the developer portal,
 * including organization context and timestamp.
 * 
 * @param {string} [orgName] - Name of the organization the user is logging out from
 * @param {Object} req - Express request object containing user session information
 */
function trackLogoutTrigger({ orgName }, req) {
    trackEventWithDefaults({
        name: EventNames.LOGOUT,
        properties: {
            organization: orgName || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks the initiation of application creation process.
 * Records when a user starts creating a new application in the developer portal.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appName] - Name of the application being created
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackAppCreationStart({ orgId, appName, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.APP_CREATION_START,
        properties: {
            orgId: orgId || NA,
            appName: appName || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks the completion of application creation process.
 * Records when a user successfully completes creating a new application.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appName] - Name of the application that was created
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 * 
 * @example
 * trackAppCreationEnd({
 *   orgId: 'org-123',
 *   appName: 'MyApp',
 *   idpId: 'idp-456'
 * }, req);
 */
function trackAppCreationEnd({ orgId, appName, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.APP_CREATION_END,
        properties: {
            orgId: orgId || NA,
            appName: appName || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks application deletion events.
 * Records when a user deletes an existing application from the developer portal.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appId] - Unique identifier of the application being deleted
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackAppDeletion({ orgId, appId, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.APP_DELETION,
        properties: {
            orgId: orgId || NA,
            appId: appId || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks credential generation events.
 * Records when a user generates new credentials for an application.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appName] - Name of the application for which credentials are generated
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackGenerateCredentials({ orgId, appName, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.GENERATE_CREDENTIALS,
        properties: {
            orgId: orgId || NA,
            appName: appName || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks API key generation events.
 * Records when a user generates a new API key for an application.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appId] - Application identifier for which the key is generated
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackGenerateKey({ orgId, appId, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.GENERATE_KEY,
        properties: {
            orgId: orgId || NA,
            appId: appId || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks API subscription events.
 * Records when a user subscribes an application to an API.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appId] - Application identifier that is subscribing
 * @param {string} [apiId] - API identifier being subscribed to
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackSubscribeApi({ orgId, appId, apiId, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.SUBSCRIBE_API,
        properties: {
            orgId: orgId || NA,
            appId: appId || NA,
            apiId: apiId || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks API unsubscription events.
 * Records when a user unsubscribes an application from an API.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [appId] - Application identifier that is unsubscribing
 * @param {string} [apiRefId] - API reference identifier being unsubscribed from
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackUnsubscribeApi({ orgId, appId, apiRefId, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.UNSUBSCRIBE_API,
        properties: {
            orgId: orgId || NA,
            appId: appId || NA,
            apiRefId: apiRefId || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Tracks home page visit events.
 * Records when a user visits the developer portal home page.
 * 
 * @param {string} [orgId] - Organization identifier
 * @param {string} [idpId] - Identity provider identifier
 * @param {Object} req - Express request object containing user session information
 */
function trackHomePageVisit({ orgId, idpId }, req) {
    trackEventWithDefaults({
        name: EventNames.HOME_PAGE_VISIT,
        properties: {
            orgId: orgId || NA,
            idpId: idpId || NA,
            timestamp: new Date().toISOString()
        }
    }, req);
}

/**
 * Module exports for telemetry tracking functions.
 * @namespace TelemetryModule
 */
module.exports = {
    /** @see trackLoginTrigger */
    trackLoginTrigger,
    /** @see trackLogoutTrigger */
    trackLogoutTrigger,
    /** @see trackAppCreationStart */
    trackAppCreationStart,
    /** @see trackAppCreationEnd */
    trackAppCreationEnd,
    /** @see trackAppDeletion */
    trackAppDeletion,
    /** @see trackGenerateCredentials */
    trackGenerateCredentials,
    /** @see trackGenerateKey */
    trackGenerateKey,
    /** @see trackSubscribeApi */
    trackSubscribeApi,
    /** @see trackUnsubscribeApi */
    trackUnsubscribeApi,
    /** @see trackHomePageVisit */
    trackHomePageVisit
};
