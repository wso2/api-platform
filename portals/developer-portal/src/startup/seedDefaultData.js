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

const fs = require('fs');
const path = require('path');
const logger = require('../config/logger');
const { Op } = require('sequelize');
const { Organization, OrgContent } = require('../models/organization');
const View = require('../models/views');

// Must match the UUID seeded by artifacts/docker-init/02_seed_default.sql and org_data.sh
const ACME_ORG_ID = '1ba42a09-45c0-40f8-a1bf-e4aa7cde1575';
const DEFAULT_VIEW_NAME = 'default';
const CONTENT_ROOT = path.join(process.cwd(), 'src', 'defaultContent');

// Map a relative file path to the DP_ORGANIZATION_ASSETS FILE_TYPE value
function resolveFileType(relPath, fileName) {
    if (fileName.endsWith('.css')) return 'style';
    if (fileName.endsWith('.md')) return 'markDown';
    if (fileName.endsWith('.hbs')) {
        if (relPath.startsWith('layout/')) return 'layout';
        if (relPath.startsWith('partials/') || relPath.includes('/partials/')) return 'partial';
        if (relPath.startsWith('pages/')) return 'template';
        return 'partial';
    }
    return 'image';
}

// Recursively collect all files under a directory
function collectFiles(dir, base) {
    const results = [];
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const absPath = path.join(dir, entry.name);
        if (entry.isDirectory()) {
            results.push(...collectFiles(absPath, base));
        } else {
            results.push(absPath);
        }
    }
    return results;
}

async function seedDefaultData() {
    try {
        const nonAcmeOrgCount = await Organization.count({
            where: { ORG_ID: { [Op.ne]: ACME_ORG_ID } }
        });
        if (nonAcmeOrgCount > 0) {
            logger.info('seedDefaultData: existing orgs found — skipping', {
                operation: 'seedDefaultData'
            });
            return;
        }

        // Find the default view for ACME org
        const view = await View.findOne({
            where: { ORG_ID: ACME_ORG_ID, NAME: DEFAULT_VIEW_NAME }
        });
        if (!view) {
            logger.warn('seedDefaultData: ACME default view not found — skipping theme seeding', {
                operation: 'seedDefaultData'
            });
            return;
        }
        const viewId = view.VIEW_ID;

        // Check if assets already exist
        const existingCount = await OrgContent.count({ where: { ORG_ID: ACME_ORG_ID } });
        if (existingCount > 0) {
            logger.info(`seedDefaultData: ${existingCount} assets already exist for ACME org — skipping`, {
                operation: 'seedDefaultData'
            });
            return;
        }

        if (!fs.existsSync(CONTENT_ROOT)) {
            logger.warn(`seedDefaultData: defaultContent directory not found at ${CONTENT_ROOT} — skipping`, {
                operation: 'seedDefaultData'
            });
            return;
        }

        const files = collectFiles(CONTENT_ROOT, CONTENT_ROOT);
        const supportedDirs = ['layout', 'pages', 'partials', 'styles', 'images'];
        let inserted = 0;

        for (const absFile of files) {
            const relPath = path.relative(CONTENT_ROOT, absFile).replace(/\\/g, '/');
            const topDir = relPath.split('/')[0];
            if (!supportedDirs.includes(topDir)) continue;

            const fileName = path.basename(absFile);
            const filePath = path.dirname(relPath) === '.' ? '' : path.dirname(relPath);
            const fileType = resolveFileType(relPath, fileName);
            const fileContent = fs.readFileSync(absFile);

            try {
                await OrgContent.upsert({
                    FILE_NAME: fileName,
                    FILE_CONTENT: fileContent,
                    FILE_TYPE: fileType,
                    FILE_PATH: filePath,
                    ORG_ID: ACME_ORG_ID,
                    VIEW_ID: viewId,
                });
                inserted++;
            } catch (err) {
                logger.warn(`seedDefaultData: failed to insert asset ${relPath}`, {
                    error: err.message,
                    operation: 'seedDefaultData'
                });
            }
        }

        logger.info(`seedDefaultData: seeded ${inserted} default theme assets for ACME org`, {
            operation: 'seedDefaultData'
        });
    } catch (err) {
        // Non-fatal — log and continue app startup
        logger.error('seedDefaultData: unexpected error during theme seeding', {
            error: err.message,
            stack: err.stack,
            operation: 'seedDefaultData'
        });
    }
}

module.exports = seedDefaultData;
