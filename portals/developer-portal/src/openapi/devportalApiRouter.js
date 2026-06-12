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
 *
 */

/*
 * Spec-driven /devportal router.
 *
 * Wires the OpenAPI spec at docs/devportal-openapi-spec-v1.yaml into the
 * Express app. Pipeline per request:
 *
 *   1. authResolver        — populates req.auth (legacy auth-mode parity)
 *   2. OpenApiValidator    — request schema check + security handler dispatch
 *                            + operationId-based handler routing
 *   3. operation handler   — thin shim from src/openapi/handlers/<tag>.js
 *
 * Operations whose tag has no handler module yet, or whose operationId is
 * not exported by the matching module, fall through to a 501 stub. This is
 * intentional for the phased migration: routes can light up tag-by-tag
 * without forcing a big-bang cut-over.
 *
 */

const path = require('path');
const fs = require('fs');
const yaml = require('js-yaml');
const express = require('express');
const OpenApiValidator = require('express-openapi-validator');

const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { authResolver, OAuth2Security, apiKeyAuth } = require('./middleware/auth');

const SPEC_PATH = path.join(__dirname, '..', '..', 'docs', 'devportal-openapi-spec-v1.yaml');
const HANDLERS_DIR = path.join(__dirname, 'handlers');

// Top-level path segments that belong to the devportal API surface, derived
// from the spec (e.g. 'o', 'applications', 'organizations', 'login', 'apis',
// 'temp-arazzo-file'). The router is mounted at '/', so it sees every request;
// this lets us pass rendered page routes (/:orgName/views/...) straight through
// with next('router') so neither authResolver nor the validator touch them.
let API_FIRST_SEGMENTS;
function apiFirstSegments() {
    if (!API_FIRST_SEGMENTS) {
        const doc = yaml.load(fs.readFileSync(SPEC_PATH, 'utf8'));
        API_FIRST_SEGMENTS = new Set(
            Object.keys(doc.paths || {})
                .map((p) => p.split('/')[1])
                .filter(Boolean)
        );
    }
    return API_FIRST_SEGMENTS;
}

// Map an OpenAPI tag like "Identity Providers" to a handler-file basename
// like "identityProviders".
function tagToFileName(tag) {
    const words = String(tag).trim().split(/\s+/).filter(Boolean);
    if (words.length === 0) return 'misc';
    return words
        .map((w, i) => {
            const lower = w.toLowerCase();
            if (i === 0) return lower;
            return lower.charAt(0).toUpperCase() + lower.slice(1);
        })
        .join('');
}

function notImplementedHandler(operationId, tag) {
    return (req, res) => {
        logger.warn('OpenAPI router: operation not yet wired in new path', {
            operationId,
            tag,
            method: req.method,
            url: req.originalUrl,
        });
        res.status(501).json({
            code: 501,
            message: 'Not Implemented',
            description:
                `Operation '${operationId}' (tag '${tag}') has no handler in src/openapi/handlers.`,
        });
    };
}

function isMissingHandlerModule(err, modulePath) {
    if (err.code !== 'MODULE_NOT_FOUND') return false;
    const firstLine = String(err.message || '').split('\n')[0];
    return firstLine === `Cannot find module '${modulePath}'`;
}

function operationResolver(handlersPath, route, apiDoc) {
    const pathKey = route.openApiRoute.substring(route.basePath.length);
    const schema = apiDoc.paths[pathKey][route.method.toLowerCase()];
    const operationId = schema.operationId;
    const tag = (schema.tags && schema.tags[0]) || 'Misc';
    const fileBase = tagToFileName(tag);
    const modulePath = path.join(handlersPath, `${fileBase}.js`);

    let mod;
    try {
        mod = require(modulePath);
    } catch (err) {
        if (isMissingHandlerModule(err, modulePath)) {
            return notImplementedHandler(operationId, tag);
        }
        throw err;
    }
    const handler = mod[operationId];
    if (typeof handler !== 'function') {
        return notImplementedHandler(operationId, tag);
    }
    // express-openapi-validator uses multer.any() internally, which stores uploaded
    // files in req.files as a flat array [{fieldname, buffer, ...}, ...].
    // Service code expects the multer.fields() shape: { fieldname: [file, ...] }.
    // Normalize here so no service file needs to know which format it received.
    return (req, res, next) => {
        if (Array.isArray(req.files)) {
            const byField = {};
            for (const file of req.files) {
                (byField[file.fieldname] = byField[file.fieldname] || []).push(file);
            }
            req.files = byField;
        }
        return handler(req, res, next);
    };
}

/**
 * Resolve the response-validation strategy from config.
 *
 *   advanced.openApiValidator.validateResponses:
 *     - false        → off (default in production)
 *     - true         → strict — validator throws 500 on response drift
 *     - 'log-only'   → log drift via logger.warn but pass the response through
 *     - (unset)      → on iff designMode.enabled is true
 *
 * Use 'log-only' to surface drift in staging/QA without breaking clients.
 */
function resolveValidateResponsesOpt() {
    const cfg = config.advanced?.openApiValidator?.openApiValidator?.validateResponses;
    if (cfg === 'strict') return true;
    if (cfg === 'off') return false;
    if (cfg === 'log-only' || cfg === 'logOnly') {
        return {
            onError: (err, json, req) => {
                logger.warn('OpenAPI response drift (log-only)', {
                    error: err.message,
                    url: req.originalUrl,
                    method: req.method,
                    errors: err.errors,
                });
            },
        };
    }
    return config.designMode?.enabled ?? false;
}

function build() {
    const router = express.Router();

    // The router is mounted at '/', so it receives every request. Skip anything
    // that isn't a devportal API path (e.g. the rendered /:orgName/views/...
    // page routes) so authResolver and the validator only run for real API
    // requests; next('router') hands the request to the page route tree.
    const apiSegments = apiFirstSegments();
    router.use((req, res, next) => {
        const seg = req.path.split('/')[1] || '';
        if (!apiSegments.has(seg)) return next('router');
        next();
    });

    // Pre-validator: resolve credentials so OAuth2Security/apiKeyAuth handlers
    router.use(authResolver);

    router.use(
        OpenApiValidator.middleware({
            apiSpec: SPEC_PATH,
            validateRequests: { allowUnknownQueryParameters: false },
            validateResponses: resolveValidateResponsesOpt(),
            validateSecurity: {
                handlers: { OAuth2Security, apiKeyAuth },
            },
            operationHandlers: {
                basePath: HANDLERS_DIR,
                resolver: operationResolver,
            },
            // Multipart endpoints in the spec (org content upload, API
            // metadata upload, etc.) are handled by the validator's built-in
            // multer. Memory storage is required: service code reads file.buffer
            // for YAML files and artifact ZIPs (extractFullApiBundleFromUploadedZip,
            // parseApiMetadataFromYamlRequest). extractApiContentFromUploadedZip
            // handles both file.path and file.buffer so memory storage is safe
            // for all endpoints including API content ZIP uploads.
            fileUploader: { storage: require('multer').memoryStorage() },
            // Format strictness — use 'fast' for runtime cost; 'full' is too
            // strict for some of our existing schemas (e.g. uri formats).
            validateFormats: 'fast',
        })
    );

    // Translate validator errors and security-handler thrown errors into the
    // JSON envelope the rest of the portal returns. The validator throws
    // objects with { status, message, errors? }.
    router.use((err, req, res, next) => {
        if (res.headersSent) return next(err);
        const status = err.status || 500;
        if (status >= 500) {
            logger.error('OpenAPI router error', {
                error: err.message,
                stack: err.stack,
                url: req.originalUrl,
                method: req.method,
            });
        } else {
            logger.warn('OpenAPI router rejected request', {
                error: err.message,
                status,
                url: req.originalUrl,
                method: req.method,
            });
        }
        res.status(status).json({
            code: status,
            message: err.message || 'Request failed',
            errors: err.errors,
        });
    });

    return router;
}

module.exports = build();
