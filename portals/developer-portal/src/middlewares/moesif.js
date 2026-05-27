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
 * @fileoverview Moesif analytics middleware and utilities for API usage tracking.
 * This module provides middleware for automatically tracking API requests and a utility
 * function for tracking custom events in the developer portal.
 */

const moesif = require('moesif-nodejs');
const { config } = require('../config/configLoader');

/**
 * Default URI used when request object is not available.
 * Fallback value for event tracking when no request context exists.
 * @constant {string}
 */
const DEFAULT_URI = 'https://devportal.bijira.dev';

/**
 * Moesif middleware instance configured with the application ID from config.
 * This middleware automatically tracks HTTP requests and responses for analytics.
 * @type {Function}
 */
const middleware = config.moesifAppId ? moesif({
    applicationId: config.moesifAppId,
    debug: false,
}) : null;

/**
 * Tracks a custom event or action in Moesif analytics.
 * This function sends custom events with user context and metadata to Moesif
 * for analytics and monitoring purposes.
 * 
 * @param {string} name - The name/identifier of the action or event being tracked
 * @param {Object} [properties={}] - Additional properties/metadata to include with the event
 * @param {Object} [req] - Express request object to extract user and request information
 * 
 * @example
 * // Track a simple event
 * trackEvent('api_key_generated', { apiId: '123' }, req);
 */
function trackEvent(name, properties, req) {
    if (!middleware) {
        return;
    }
    middleware.sendAction({
        actionName: name,
        userId: '',
        companyId: '',
        metadata: {
            ...properties,
        },
        request: {
            time: new Date(),
            uri: req ? req.originalUrl || req.url : DEFAULT_URI,
        },
    });
}

/**
 * Module exports for Moesif analytics functionality.
 * @namespace
 */
module.exports = {
    /**
     * Moesif middleware for automatic HTTP request/response tracking.
     * Use this middleware in Express routes to automatically track API usage.
     * @type {Function}
     * @example
     * const { moesifMiddleware } = require('./middlewares/moesif');
     * app.use(moesifMiddleware);
     */
    moesifMiddleware: middleware,
    
    /**
     * Function to track custom events and actions.
     * @type {Function}
     * @see trackEvent
     */
    trackEvent,
};