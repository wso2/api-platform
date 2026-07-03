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
const Audit = require('../models/audit');

/**
 * Records one audit trail entry. Write-only — no list/query function, mirroring
 * platform-api's audit table, which is inspected directly in the database.
 */
const record = async (action, resourceUuid, resourceType, orgUuid, performedBy) => {
    return Audit.create({
        action,
        resource_uuid: resourceUuid,
        resource_type: resourceType,
        org_uuid: orgUuid,
        performed_by: performedBy,
    });
};

module.exports = {
    record,
};
