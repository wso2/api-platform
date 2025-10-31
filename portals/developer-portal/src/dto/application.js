
/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
class ApplicationDTO {
    constructor(app) {
        this.id = app.APP_ID;
        this.name = app.NAME;
        this.description = app.DESCRIPTION;
        this.type = app.TYPE;
        if (app.DP_APP_KEY_MAPPINGs) {
            this.appMap = app.DP_APP_KEY_MAPPINGs.map(map => new AppMappingDTO(map));
        }
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class SubscriptionDTO {
    constructor(sub) {
        this.id = sub.SUB_ID;
        this.appId = sub.APP_ID;
        this.apiId = sub.REFERENCE_ID;
        this.policyId = sub.POLICY_ID;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class AppMappingDTO {
    constructor(map) {
        this.appRefID = map.CP_APP_REF;
        this.token = map.TOKEN_TYPE;
        this.shared = map.SHARED_TOKEN;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

module.exports = {
    ApplicationDTO,
    SubscriptionDTO
};
