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
const sequelize = require('../db/sequelizeConfig');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const labelDao = require('../dao/labelDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { parseApiMetadataFromYamlFile, prepareApiDefinitionForStorage } = require('./apiMetadataService');

const DEFINITION_CANDIDATES = ['definition.yaml', 'definition.yml', 'definition.json', 'definition.graphql', 'definition.wsdl'];
const SAMPLES_DIR = path.join(process.cwd(), 'samples', 'apis');
const MCP_SAMPLES_DIR = path.join(process.cwd(), 'samples', 'mcps');
const SCHEMA_DEFINITION_FILE = constants.FILE_NAME.SCHEMA_DEFINITION_YAML_FILE_NAME;

function findDefinitionPath(apiDir) {
    for (const name of DEFINITION_CANDIDATES) {
        const candidate = path.join(apiDir, name);
        if (fs.existsSync(candidate)) return candidate;
    }
    return null;
}

/**
 * Recursively read docs/ under an API directory.
 * Top-level .md files → TYPE = DOC_Other
 * Files in a subdirectory (e.g. docs/FAQ/) → TYPE = DOC_FAQ
 * Returns [{ type, fileName, content }]
 */
function readDocFiles(docsDir, subDir) {
    const docs = [];
    if (!fs.existsSync(docsDir)) return docs;
    for (const entry of fs.readdirSync(docsDir)) {
        if (entry.startsWith('.')) continue;
        const full = path.join(docsDir, entry);
        if (fs.statSync(full).isDirectory()) {
            docs.push(...readDocFiles(full, entry));
        } else if (/\.(md|MD)$/.test(entry)) {
            const docType = subDir || constants.DOC_TYPES.DOCS.OTHER;
            docs.push({
                type: constants.DOC_TYPES.DOC_ID + docType,
                fileName: entry,
                content: Buffer.from(fs.readFileSync(full)),
            });
        }
    }
    return docs;
}

/**
 * Deploy all sample APIs from samples/apis/ into the given org.
 * Already-existing APIs (UniqueConstraintError) are skipped silently.
 * Returns an array of { name, status ('ok'|'exists'|'failed'), apiId?, error? }.
 */
async function seedSampleAPIs(orgId) {
    if (!fs.existsSync(SAMPLES_DIR)) {
        logger.warn('samples/apis directory not found — skipping sample seeding', { SAMPLES_DIR });
        return [];
    }

    const entries = fs.readdirSync(SAMPLES_DIR)
        .filter(e => {
            const p = path.join(SAMPLES_DIR, e);
            return fs.statSync(p).isDirectory() && fs.existsSync(path.join(p, 'api.yaml'));
        });

    const results = [];

    for (const entry of entries) {
        const apiDir = path.join(SAMPLES_DIR, entry);
        let apiName = entry;
        let apiId;

        try {
            const yamlBuffer = Buffer.from(fs.readFileSync(path.join(apiDir, 'api.yaml')));
            const apiMetadata = parseApiMetadataFromYamlFile('api.yaml', yamlBuffer);
            apiName = apiMetadata.apiInfo.apiName || entry;

            if (await apiDao.existsByNameVersion(orgId, apiName, apiMetadata.apiInfo.apiVersion)) {
                results.push({ name: apiName, status: 'exists' });
                continue;
            }

            // Load definition file if present
            let apiDefinitionFile = null;
            let apiFileName = '';
            const defPath = findDefinitionPath(apiDir);
            if (defPath) {
                const defName = path.basename(defPath);
                const defBuffer = Buffer.from(fs.readFileSync(defPath));
                try {
                    const prepared = prepareApiDefinitionForStorage(defName, defBuffer);
                    apiDefinitionFile = prepared.apiDefinitionFile;
                    apiFileName = prepared.apiDefinitionFileName;
                } catch (prepErr) {
                    // Non-standard type (e.g. WSDL): store raw
                    logger.debug(`prepareApiDefinitionForStorage skipped for ${entry}: ${prepErr.message}`);
                    apiDefinitionFile = defBuffer;
                    apiFileName = defName;
                }
            }

            await sequelize.transaction(async (t) => {
                const created = await apiDao.create(orgId, apiMetadata, constants.SYSTEM_ACTOR, t);
                apiId = created.dataValues.UUID;

                // Subscription plan mappings (skip unknown plans — don't fail the whole deployment)
                if (Array.isArray(apiMetadata.subscriptionPlans) && apiMetadata.subscriptionPlans.length) {
                    const mappings = [];
                    for (const p of apiMetadata.subscriptionPlans) {
                        const plan = await subscriptionPlanDao.getByName(orgId, p.planName);
                        if (plan) mappings.push({ apiID: apiId, planID: plan.UUID });
                    }
                    if (mappings.length) await subscriptionPlanDao.createApiMapping(mappings, apiId, constants.SYSTEM_ACTOR, t);
                }

                // Label mappings
                const labels = Array.isArray(apiMetadata.apiInfo.labels) && apiMetadata.apiInfo.labels.length
                    ? apiMetadata.apiInfo.labels
                    : ['default'];
                await labelDao.createApiMapping(orgId, apiId, labels, constants.SYSTEM_ACTOR, t);

                // Definition file
                if (apiDefinitionFile) {
                    const isGraphQL = apiMetadata.apiInfo.apiType === constants.API_TYPE.GRAPHQL;
                    const storedName = isGraphQL ? constants.FILE_NAME.API_DEFINITION_GRAPHQL : apiFileName;
                    await apiFileDao.store(apiDefinitionFile, storedName, apiId, constants.DOC_TYPES.API_DEFINITION, constants.SYSTEM_ACTOR, t);
                }

                // Documentation files from docs/
                const docs = readDocFiles(path.join(apiDir, 'docs'), '');
                if (docs.length) {
                    await apiFileDao.storeMany(docs, apiId, constants.SYSTEM_ACTOR, t);
                }
            });

            results.push({ name: apiName, handle: apiMetadata.apiInfo.apiHandle, status: 'ok', apiId });
            logger.info('Seeded sample API', { orgId, apiName, apiId });

        } catch (err) {
            results.push({ name: apiName, status: 'failed', error: err.message });
            logger.error('Failed to seed sample API', { orgId, entry, error: err.message });
        }
    }

    return results;
}

/**
 * Deploy all sample MCP servers from samples/mcps/ into the given org.
 * Each subdirectory must contain api.yaml and optionally schemaDefinition.yaml and docs/.
 * Returns an array of { name, status ('ok'|'exists'|'failed'), apiId?, error? }.
 */
async function seedSampleMCPs(orgId) {
    if (!fs.existsSync(MCP_SAMPLES_DIR)) {
        logger.warn('samples/mcps directory not found — skipping MCP seeding', { MCP_SAMPLES_DIR });
        return [];
    }

    const entries = fs.readdirSync(MCP_SAMPLES_DIR)
        .filter(e => {
            const p = path.join(MCP_SAMPLES_DIR, e);
            return fs.statSync(p).isDirectory() && fs.existsSync(path.join(p, 'api.yaml'));
        });

    const results = [];

    for (const entry of entries) {
        const mcpDir = path.join(MCP_SAMPLES_DIR, entry);
        let apiName = entry;
        let apiId;

        try {
            const yamlBuffer = Buffer.from(fs.readFileSync(path.join(mcpDir, 'api.yaml')));
            const apiMetadata = parseApiMetadataFromYamlFile('api.yaml', yamlBuffer);
            apiName = apiMetadata.apiInfo.apiName || entry;

            if (await apiDao.existsByNameVersion(orgId, apiName, apiMetadata.apiInfo.apiVersion)) {
                results.push({ name: apiName, status: 'exists' });
                continue;
            }

            const schemaPath = path.join(mcpDir, SCHEMA_DEFINITION_FILE);
            const schemaBuffer = fs.existsSync(schemaPath)
                ? Buffer.from(fs.readFileSync(schemaPath))
                : null;

            await sequelize.transaction(async (t) => {
                const created = await apiDao.create(orgId, apiMetadata, constants.SYSTEM_ACTOR, t);
                apiId = created.dataValues.API_UUID;

                // Subscription plan mappings (skip unknown plans — don't fail the whole deployment)
                if (Array.isArray(apiMetadata.subscriptionPlans) && apiMetadata.subscriptionPlans.length) {
                    const mappings = [];
                    for (const p of apiMetadata.subscriptionPlans) {
                        const plan = await subscriptionPlanDao.getByName(orgId, p.planName);
                        if (plan) mappings.push({ apiID: apiId, planID: plan.PLAN_UUID });
                    }
                    if (mappings.length) await subscriptionPlanDao.createApiMapping(mappings, apiId, constants.SYSTEM_ACTOR, t);
                }

                // Label mappings
                const labels = Array.isArray(apiMetadata.apiInfo.labels) && apiMetadata.apiInfo.labels.length
                    ? apiMetadata.apiInfo.labels
                    : ['default'];
                await labelDao.createApiMapping(orgId, apiId, labels, constants.SYSTEM_ACTOR, t);

                // Schema definition (tools/resources/prompts)
                if (schemaBuffer) {
                    await apiFileDao.store(
                        schemaBuffer,
                        constants.FILE_NAME.SCHEMA_DEFINITION_YAML_FILE_NAME,
                        apiId,
                        constants.DOC_TYPES.SCHEMA_DEFINITION,
                        constants.SYSTEM_ACTOR,
                        t
                    );
                }

                // Documentation files from docs/
                const docs = readDocFiles(path.join(mcpDir, 'docs'), '');
                if (docs.length) {
                    await apiFileDao.storeMany(docs, apiId, constants.SYSTEM_ACTOR, t);
                }
            });

            results.push({ name: apiName, handle: apiMetadata.apiInfo.apiHandle, status: 'ok', apiId });
            logger.info('Seeded sample MCP', { orgId, apiName, apiId });

        } catch (err) {
            results.push({ name: apiName, status: 'failed', error: err.message });
            logger.error('Failed to seed sample MCP', { orgId, entry, error: err.message });
        }
    }

    return results;
}

module.exports = { seedSampleAPIs, seedSampleMCPs };
