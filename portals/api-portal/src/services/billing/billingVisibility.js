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

/*
 * Which paid plans an API may offer. A paid plan is shown and can be subscribed to only when the
 * organization has a billing configuration and the plan has a billing API plan for the API
 * (created, with everything it needs, when the plan was enabled for the API). Otherwise there is
 * nothing to bill it with, so it is not offered at all. Free plans are not affected.
 */

const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingApiPlanDao = require('../../dao/billingApiPlanDao');

/** The uuids of the paid plans the API may offer now. */
async function sellablePlanUuids(orgId, apiUuid) {
    if (!apiUuid) return new Set();
    const conn = await billingConfigurationDao.get(orgId);
    if (!conn) return new Set();
    return billingApiPlanDao.listPlanIds(orgId, apiUuid);
}

/** Whether the API may offer the paid plan now. */
async function canSellPlan(orgId, apiUuid, planUuid) {
    return (await sellablePlanUuids(orgId, apiUuid)).has(planUuid);
}

module.exports = { sellablePlanUuids, canSellPlan };
