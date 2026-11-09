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
 * Moesif keys set for the whole portal in config ([api_portal.billing.moesif]
 * collector_application_id and management_key) instead of entered in billing setup. The config
 * loader checks they are set together, so this sees both or neither. They are never stored.
 */

function moesifConfig() {
    return require('../config/configLoader').config.billing?.moesif || {};
}

/** The keys set in config, named as the billing setup request names them, or null when none are set. */
function deploymentSettings() {
    const { collectorApplicationId, managementKey } = moesifConfig();
    if (!collectorApplicationId || !managementKey) return null;
    return { moesifCollectorApplicationId: collectorApplicationId, moesifManagementKey: managementKey };
}

/** The names of the keys set in config. */
function deploymentFields() {
    return Object.keys(deploymentSettings() || {});
}

/** Whether the key `name` is set in config. */
function suppliedByDeployment(name) {
    return deploymentFields().includes(name);
}

module.exports = {
    deploymentSettings,
    deploymentFields,
    suppliedByDeployment,
};
