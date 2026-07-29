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

const orgDao = require('../dao/organizationDao');
const labelDao = require('../dao/labelDao');
const viewDao = require('../dao/viewDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const { config } = require('../config/configLoader');
const orgContext = require('../utils/orgContext');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const db = require('../db/driver');
const { NotFoundError } = require('../utils/errors/customErrors');

/**
 * Seeds this instance's organization and its dependent resources on startup.
 * Each resource is checked/created individually so an existing org with
 * missing defaults is repaired without skipping the rest of the seed.
 *
 * The organization is the one named by config.organization.handle — the single org
 * this instance serves (see src/utils/orgContext.js). Lookup is by exact handle,
 * not orgDao.get()'s handle→display_name→idp_ref_id ladder: in a shared
 * multi-organization database the looser match could resolve to a *different*
 * organization that happens to carry this handle as its display name, and the
 * seeder would then adopt that row as this instance's org.
 */
async function seedDefaultOrg() {
    const orgName = orgContext.getHandle();
    if (!orgName) return;

    const payload = {
        displayName: orgContext.getDisplayName(),
        handle: orgName,
        idpRefId: orgName,
        configuration: { devportalMode: constants.DEVPORTAL_MODE.DEFAULT },
        createdBy: constants.SYSTEM_ACTOR,
    };

    let orgId;
    try {
        const existing = await orgDao.getByHandle(orgName);
        orgId = existing.uuid;
    } catch (notFound) {
        if (!(notFound instanceof NotFoundError)) {
            // Rethrow rather than continue: without this row the portal has no
            // organization to scope anything to, so every request would 500 while the
            // process still reported itself healthy. startServer awaits this before
            // binding the listener, so the failure stops startup instead.
            logger.error('Failed to look up default organization', {
                error: notFound.message,
                operation: 'seedDefaultOrg',
            });
            throw notFound;
        }
        try {
            const organization = await orgDao.create(payload);
            orgId = organization.uuid;
        } catch (createError) {
            logger.error('Failed to seed default organization', {
                error: createError.message,
                stack: createError.stack,
                operation: 'seedDefaultOrg',
            });
            throw createError;
        }
    }

    let labelId;
    try {
        const label = await labelDao.update(orgId, { handle: 'default', displayName: 'default' }, constants.SYSTEM_ACTOR);
        labelId = label.uuid;
    } catch (error) {
        logger.error('Failed to seed default label', {
            error: error.message,
            operation: 'seedDefaultOrg',
        });
        return;
    }

    let viewId;
    try {
        const view = await viewDao.update(orgId, 'default', 'default', constants.SYSTEM_ACTOR);
        viewId = view.uuid;
    } catch (error) {
        logger.error('Failed to seed default view', {
            error: error.message,
            operation: 'seedDefaultOrg',
        });
        return;
    }

    try {
        await labelDao.addToView(orgId, labelId, viewId, constants.SYSTEM_ACTOR);
    } catch (error) {
        if (!db.isDuplicateKeyError(error)) {
            logger.error('Failed to seed label-view link', {
                error: error.message,
                operation: 'seedDefaultOrg',
            });
            return;
        }
    }

    if (config.organization.autoCreateSubscriptionPlans) {
        for (const plan of constants.DEFAULT_SUBSCRIPTION_PLANS) {
            try {
                await subscriptionPlanDao.createMany(orgId, [plan], constants.SYSTEM_ACTOR);
            } catch (error) {
                if (!db.isDuplicateKeyError(error)) {
                    logger.error('Failed to seed subscription plan', {
                        error: error.message,
                        operation: 'seedDefaultOrg',
                        plan: plan.displayName,
                    });
                }
            }
        }
    }

    // Drop any uuid orgContext resolved before the org row existed, so the first
    // request after startup re-reads it rather than reusing a pre-seed miss.
    orgContext.resetCache();

    logger.info('Org: organization seeded ✓', { handle: orgName });
}

module.exports = { seedDefaultOrg };
