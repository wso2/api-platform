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

const { findOrCreateSafe } = require('./findOrCreateHelper');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'user_organization_mappings';

/**
 * Record that this user belongs to this org. No-op if already recorded.
 * PRIMARY KEY is (portal_id, user_uuid, org_uuid).
 */
const ensureMapping = async (userUuid, orgUuid) => {
    const portalId = getPortalId();
    await findOrCreateSafe(
        TABLE,
        { portal_id: portalId, user_uuid: userUuid, org_uuid: orgUuid },
        { portal_id: portalId, user_uuid: userUuid, org_uuid: orgUuid }
    );
};

module.exports = {
    ensureMapping,
};
