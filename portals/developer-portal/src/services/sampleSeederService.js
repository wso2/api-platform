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
const { Sequelize } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const labelDao = require('../dao/labelDao');
const subscriptionPolicyDao = require('../dao/subscriptionPolicyDao');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { parseApiMetadataFromYamlFile, prepareApiDefinitionForStorage } = require('./apiMetadataService');

const DEFINITION_CANDIDATES = ['definition.yaml', 'definition.yml', 'definition.json', 'definition.graphql', 'definition.wsdl'];
const SAMPLES_DIR = path.join(process.cwd(), 'samples', 'apis');

function findDefinitionPath(apiDir) {
    for (const name of DEFINITION_CANDIDATES) {
        const candidate = path.join(apiDir, name);
        if (fs.existsSync(candidate)) return candidate;
    }
    return null;
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
                const created = await apiDao.create(orgId, apiMetadata, t);
                apiId = created.dataValues.API_ID;

                // Subscription policy mappings (skip unknown policies — don't fail the whole deployment)
                if (Array.isArray(apiMetadata.subscriptionPolicies) && apiMetadata.subscriptionPolicies.length) {
                    const mappings = [];
                    for (const p of apiMetadata.subscriptionPolicies) {
                        const policy = await subscriptionPolicyDao.getByName(orgId, p.policyName);
                        if (policy) mappings.push({ apiID: apiId, policyID: policy.POLICY_ID });
                    }
                    if (mappings.length) await subscriptionPolicyDao.createApiMapping(mappings, apiId, t);
                }

                // Label mappings
                const labels = Array.isArray(apiMetadata.apiInfo.labels) && apiMetadata.apiInfo.labels.length
                    ? apiMetadata.apiInfo.labels
                    : ['default'];
                await labelDao.createApiMapping(orgId, apiId, labels, t);

                // Definition file
                if (apiDefinitionFile) {
                    const isGraphQL = apiMetadata.apiInfo.apiType === constants.API_TYPE.GRAPHQL;
                    const storedName = isGraphQL ? constants.FILE_NAME.API_DEFINITION_GRAPHQL : apiFileName;
                    await apiFileDao.store(apiDefinitionFile, storedName, apiId, constants.DOC_TYPES.API_DEFINITION, t);
                }
            });

            results.push({ name: apiName, handle: apiMetadata.apiInfo.apiHandle, status: 'ok', apiId });
            logger.info('Seeded sample API', { orgId, apiName, apiId });

        } catch (err) {
            if (err instanceof Sequelize.UniqueConstraintError) {
                results.push({ name: apiName, status: 'exists' });
            } else {
                results.push({ name: apiName, status: 'failed', error: err.message });
                logger.error('Failed to seed sample API', { orgId, entry, error: err.message });
            }
        }
    }

    return results;
}

module.exports = { seedSampleAPIs };
