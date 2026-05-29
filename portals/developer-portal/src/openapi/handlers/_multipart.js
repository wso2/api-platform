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
 * Bridges the validator's multer output to the shape the legacy services
 * expect:
 *   - express-openapi-validator calls multer.any(): req.files is an Array
 *   - the configured disk storage populates file.path, not file.buffer
 *
 * Services in this codebase were written against the legacy router, which used
 * multer.fields() with memory storage and exposed req.files as an object
 * keyed by fieldname, or req.file for single-field uploads. This middleware
 * reshapes req.files into the fields() shape, lazy-reads file.buffer from
 * disk when missing, and sets req.file when a single file was uploaded.
 */
const fs = require('fs');
const fsp = fs.promises;
const logger = require('../../config/logger');

async function adaptMultipart(req, _res, next) {
    if (!Array.isArray(req.files) || req.files.length === 0) return next();

    const grouped = {};
    try {
        for (const f of req.files) {
            if (!f.buffer && f.path) {
                // Async read so we don't block the event loop on large uploads.
                f.buffer = await fsp.readFile(f.path);
                // multer's disk storage leaves the temp file behind; remove it
                // now that the contents are in memory to avoid a disk leak.
                await fsp.unlink(f.path).catch(() => { /* best-effort cleanup */ });
            }
            // Guard against files with no fieldname (e.g. unnamed fields), which
            // would otherwise silently collect under a "undefined" key.
            if (f.fieldname == null) {
                logger.warn(`Skipping uploaded file with no fieldname: ${f.originalname || 'unknown'}`);
                continue;
            }
            (grouped[f.fieldname] ||= []).push(f);
        }
    } catch (err) {
        return next(err);
    }
    req.files = grouped;

    const flat = Object.values(grouped).flat();
    if (flat.length === 1) {
        req.file = flat[0];
    }

    next();
}

module.exports = { adaptMultipart };
