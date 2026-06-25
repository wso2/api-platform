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
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const labelDao = require('../dao/labelDao');
const viewDao = require('../dao/viewDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');

/**
 * Seeds the default organization and its dependent resources on startup.
 * Each resource is checked/created individually so an existing org with
 * missing defaults is repaired without skipping the rest of the seed.
 */
async function seedDefaultOrg() {
    const orgName = config.defaultOrgName;
    if (!orgName) return;

    const payload = {
        orgName,
        orgHandle: orgName,
        organizationIdentifier: orgName,
        orgConfig: { devportalMode: constants.DEVPORTAL_MODE.DEFAULT },
    };

    let orgId;
    try {
        const existing = await orgDao.get(orgName);
        orgId = existing.ID;
    } catch (notFound) {
        if (!(notFound instanceof Sequelize.EmptyResultError)) {
            logger.error('Failed to look up default organization', {
                error: notFound.message,
                operation: 'seedDefaultOrg',
            });
            return;
        }
        try {
            const organization = await orgDao.create(payload);
            orgId = organization.ID;
        } catch (createError) {
            logger.error('Failed to seed default organization', {
                error: createError.message,
                stack: createError.stack,
                operation: 'seedDefaultOrg',
            });
            return;
        }
    }

    let labelId;
    try {
        const label = await labelDao.update(orgId, { name: 'default', displayName: 'default' });
        labelId = label.dataValues.ID;
    } catch (error) {
        logger.error('Failed to seed default label', {
            error: error.message,
            operation: 'seedDefaultOrg',
        });
        return;
    }

    let viewId;
    try {
        const view = await viewDao.update(orgId, 'default', 'default');
        viewId = view.dataValues.ID;
    } catch (error) {
        logger.error('Failed to seed default view', {
            error: error.message,
            operation: 'seedDefaultOrg',
        });
        return;
    }

    try {
        await labelDao.addToView(orgId, labelId, viewId);
    } catch (error) {
        if (!(error instanceof Sequelize.UniqueConstraintError)) {
            logger.error('Failed to seed label-view link', {
                error: error.message,
                operation: 'seedDefaultOrg',
            });
            return;
        }
    }

    if (config.generateDefaultSubPlans) {
        for (const plan of constants.DEFAULT_SUBSCRIPTION_PLANS) {
            try {
                await subscriptionPlanDao.createMany(orgId, [plan]);
            } catch (error) {
                if (!(error instanceof Sequelize.UniqueConstraintError)) {
                    logger.error('Failed to seed subscription plan', {
                        error: error.message,
                        operation: 'seedDefaultOrg',
                        plan: plan.name,
                    });
                }
            }
        }
    }

    logger.info('Org: default organization seeded ✓');
}

module.exports = { seedDefaultOrg };
