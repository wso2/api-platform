/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
'use strict';

const { Sequelize } = require('sequelize');
const sequelize = require('../db/sequelize');
const adminDao = require('../dao/admin');
const apiDao = require('../dao/apiMetadata');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');

/**
 * Seeds the default organization on startup when config.defaultOrgName is set.
 * Idempotent — a UniqueConstraintError (org already exists) is silently skipped.
 * Mirrors the transaction logic in adminService.createOrganization.
 */
async function seedDefaultOrg() {
    const orgName = config.defaultOrgName;
    if (!orgName) return;

    try {
        await sequelize.transaction(async (t) => {
            const payload = {
                orgName,
                orgHandle: orgName,
                roleClaimName: config.roleClaim,
                groupsClaimName: config.groupsClaim,
                organizationClaimName: config.orgIDClaim,
                organizationIdentifier: orgName,
                adminRole: config.adminRole,
                subscriberRole: config.subscriberRole,
                superAdminRole: config.superAdminRole,
                orgConfig: { devportalMode: constants.DEVPORTAL_MODE.DEFAULT },
            };

            const organization = await adminDao.createOrganization(payload, t);
            const orgId = organization.ORG_ID;

            const createdLabels = await apiDao.createLabels(
                orgId, [{ name: 'default', displayName: 'default' }], t
            );
            const labelId = createdLabels[0].dataValues.LABEL_ID;

            const viewResponse = await apiDao.addView(
                orgId, { name: 'default', displayName: 'default' }, t
            );
            const viewId = viewResponse.dataValues.VIEW_ID;

            await apiDao.addLabel(orgId, labelId, viewId, t);
            await adminDao.createProvider(
                orgId, { name: 'WSO2', providerURL: config.controlPlane.url }, t
            );

            if (config.generateDefaultSubPolicies) {
                await apiDao.bulkCreateSubscriptionPolicies(
                    orgId, constants.DEFAULT_SUBSCRIPTION_PLANS, t
                );
            }
        });

        logger.info(`Default organization '${orgName}' seeded successfully`);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            logger.info(`Default organization '${orgName}' already exists, skipping seed`);
            return;
        }
        logger.error('Failed to seed default organization', {
            error: error.message,
            stack: error.stack,
            operation: 'seedDefaultOrg'
        });
    }
}

module.exports = { seedDefaultOrg };
