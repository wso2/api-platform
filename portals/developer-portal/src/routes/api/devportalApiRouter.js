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
 * Wires the OpenAPI spec at docs/devportal-openapi-spec-v0.9.yaml into the
 * Express app. Pipeline per request:
 *
 *   1. authResolver        — populates req.auth (legacy auth-mode parity)
 *   2. OpenApiValidator    — request schema check + security handler dispatch
 *                            + operationId-based handler routing
 *   3. operation handler   — thin shim from src/routes/api/handlers/<tag>.js
 *
 * Operations whose tag has no handler module yet, or whose operationId is
 * not exported by the matching module, fall through to a 501 stub. This is
 * intentional for the phased migration: routes can light up tag-by-tag
 * without forcing a big-bang cut-over.
 *
 */

const path = require('path');
const fs = require('fs');
const crypto = require('crypto');
const yaml = require('js-yaml');
const express = require('express');
const OpenApiValidator = require('express-openapi-validator');

const { config } = require('../../config/configLoader');
const constants = require('../../utils/constants');
const logger = require('../../config/logger');
const { authResolver, OAuth2Security, apiKeyAuth } = require('../../middlewares/authMiddleware');

const SPEC_PATH = path.join(__dirname, '..', '..', '..', 'docs', 'devportal-openapi-spec-v0.9.yaml');
const HANDLERS_DIR = path.join(__dirname, 'handlers');

// Top-level path segments that belong to the devportal API surface. The router
// is mounted at '/', so it sees every request; this lets us pass rendered page
// routes (/:orgName/views/...) straight through with next('router') so neither
// authResolver nor the validator touch them.
//
// The version base (e.g. '/api/v0.9') lives in the spec's `servers[].url`, so
// express-openapi-validator matches requests against `basePath + pathKey`. We
// mirror that here: the segment we must recognize is the first segment of the
// *combined* route (e.g. 'api' for a server basePath of '/api/v0.9'), not of the
// bare path key. When no server basePath is set we fall back to the path key's
// own first segment.
let API_FIRST_SEGMENTS;
function serverBasePath(url) {
    // Strip scheme://host, keeping only the path portion. Tolerates server URL
    // template variables (e.g. https://localhost:{port}/api/v0.9).
    const noScheme = String(url).replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');
    const slash = noScheme.indexOf('/');
    const p = slash === -1 ? '' : noScheme.slice(slash);
    return p === '/' ? '' : p.replace(/\/$/, '');
}
function apiFirstSegments() {
    if (!API_FIRST_SEGMENTS) {
        const doc = yaml.load(fs.readFileSync(SPEC_PATH, 'utf8'));
        const bases = (doc.servers || []).map((s) => serverBasePath(s.url));
        const effectiveBases = bases.length ? bases : [''];
        API_FIRST_SEGMENTS = new Set();
        for (const base of effectiveBases) {
            for (const p of Object.keys(doc.paths || {})) {
                const seg = `${base}${p}`.split('/')[1];
                if (seg) API_FIRST_SEGMENTS.add(seg);
            }
        }
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
        .join('') + 'Handler';
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
                `Operation '${operationId}' (tag '${tag}') has no handler in src/routes/api/handlers.`,
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
    const cfg = config.developer?.openApiResponseValidation;
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
            fileUploader: {
                storage: require('multer').memoryStorage(),
                limits: { fileSize: config.uploads?.maxBytes || 10485760 },
            },
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
        // Log the path only — the query string may carry tokens/secrets.
        const reqPath = (req.originalUrl || '').split('?')[0];
        // Oversize uploads surface as 413 with a generic message.
        if (err.code === 'LIMIT_FILE_SIZE') {
            logger.warn('Upload rejected: file exceeds size limit', {
                url: reqPath,
                method: req.method,
            });
            return res.status(413).json({
                code: 413,
                message: 'Uploaded file exceeds the maximum allowed size.',
            });
        }
        const status = err.status || 500;
        if (status >= 500) {
            // Return a generic message with a correlation id; the full detail is logged
            // under the same trackingId, never sent to the client.
            const trackingId = crypto.randomUUID();
            logger.error('OpenAPI router error', {
                trackingId,
                error: err.message,
                stack: err.stack,
                url: reqPath,
                method: req.method,
            });
            return res.status(status).json({
                code: status,
                message: 'An unexpected error occurred.',
                tracking_id: trackingId,
            });
        }
        // 4xx validation errors from the OpenAPI validator are safe to pass through.
        logger.warn('OpenAPI router rejected request', {
            error: err.message,
            status,
            url: reqPath,
            method: req.method,
        });
        res.status(status).json({
            code: status,
            message: err.message || 'Request failed',
            errors: err.errors,
        });
    });

    return router;
}

module.exports = build();
