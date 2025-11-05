/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

/**
 * Middleware for audit logging user interactions
 * @param {Object} options - Configuration options for audit logging
 * @param {Array} options.excludePaths - Paths to exclude from audit logging (e.g., health checks)
 * @param {Array} options.sensitiveFields - Fields to redact from request body
 */
function auditMiddleware(options = {}) {
    const { 
        excludePaths = ['/health', '/metrics', '/favicon.ico'],
        sensitiveFields = ['password', 'token', 'secret', 'key', 'authorization']
    } = options;

    return (req, res, next) => {
        const startTime = Date.now();
        
        // Skip audit logging for excluded paths
        if (excludePaths.some(path => req.path.startsWith(path))) {
            return next();
        }

        // Capture original end method
        const originalEnd = res.end;
        
        // Override res.end to capture response data
        res.end = function(chunk, encoding) {
            const endTime = Date.now();
            const duration = endTime - startTime;
            
            // Redact sensitive information from request body
            const sanitizedBody = sanitizeObject(req.body || {}, sensitiveFields);

            // Create and log audit log entry
            const auditEntry = {
                requestBody: Object.keys(sanitizedBody).length > 0 ? sanitizedBody : "",
                queryParams: Object.keys(req.query || {}).length > 0 ? req.query : "",
            };
            logger.audit(`${req.method} ${req.originalUrl || req.url} - ${res.statusCode} - ${duration}ms`, auditEntry);
            
            // Call original end method
            originalEnd.call(this, chunk, encoding);
        };
        next();
    };
}

/**
 * Get client IP address
 * @param {Object} req - Express request object
 * @returns {string} Client IP address
 */
function getClientIP(req) {
    return req.headers['x-forwarded-for'] || 
           req.headers['x-real-ip'] || 
           req.connection.remoteAddress || 
           req.socket.remoteAddress ||
           (req.connection.socket ? req.connection.socket.remoteAddress : null) ||
           'unknown';
}

/**
 * Sanitize object by removing or redacting sensitive fields
 * @param {Object} obj - Object to sanitize
 * @param {Array} sensitiveFields - Array of field names to redact
 * @returns {Object} Sanitized object
 */
function sanitizeObject(obj, sensitiveFields) {
    if (!obj || typeof obj !== 'object') {
        return obj;
    }
    const sanitized = {};
    for (const [key, value] of Object.entries(obj)) {
        const lowerKey = key.toLowerCase();
        
        // Check if field is sensitive
        if (sensitiveFields.some(field => lowerKey.includes(field.toLowerCase()))) {
            sanitized[key] = '[REDACTED]';
        } else if (typeof value === 'object' && value !== null) {
            // Recursively sanitize nested objects
            sanitized[key] = sanitizeObject(value, sensitiveFields);
        } else {
            sanitized[key] = value;
        }
    }
    return sanitized;
}

/**
 * Enhanced audit logging for specific actions
 * @param {string} action - Action being performed
 * @param {Object} req - Express request object
 * @param {Object} additionalData - Additional data to include in audit log
 */
function logUserAction(action, req, additionalData = {}) {
    const userId = req.user ? req.user.id || req.user.username || 'unknown' : 'anonymous';
    const sessionId = req.sessionID || 'no-session';
    
    const auditEntry = {
        action: action,
        userId: userId,
        sessionId: sessionId,
        timestamp: new Date().toISOString(),
        ...additionalData
    };
    logger.audit(`USER_ACTION: ${action} - User: ${userId}`, auditEntry);
}

module.exports = {
    auditMiddleware,
    logUserAction
};
