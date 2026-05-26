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

class ImportedApplicationDTO {
    constructor(data) {
        this.type = data.type;
        this.version = data.version;
        this.applicationInfo = new ApplicationInfo(data.data.applicationInfo);
        this.subscribedAPIs = data.data.subscribedAPIs ? data.data.subscribedAPIs.map(api => new SubscribedAPI(api)) : [];
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class ApplicationInfo {
    constructor(appInfo) {
        this.name = appInfo.name;
        this.throttlingPolicy = appInfo.throttlingPolicy;
        this.description = appInfo.description;
        this.status = appInfo.status;
        this.groups = appInfo.groups || [];
        this.keys = appInfo.keys ? appInfo.keys.map(key => new ApplicationKey(key)) : [];
        this.attributes = appInfo.attributes || {};
        this.owner = appInfo.owner;
        this.visibility = appInfo.visibility;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class ApplicationKey {
    constructor(key) {
        this.keyManager = key.keyManager;
        this.consumerKey = key.consumerKey;
        this.consumerSecret = key.consumerSecret;
        this.supportedGrantTypes = key.supportedGrantTypes || [];
        this.callbackUrl = key.callbackUrl;
        this.keyState = key.keyState;
        this.keyType = key.keyType;
        this.mode = key.mode;
        this.additionalProperties = new AdditionalProperties(key.additionalProperties || {});
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class AdditionalProperties {
    constructor(props) {
        this.idTokenExpiryTime = props.id_token_expiry_time;
        this.applicationAccessTokenExpiryTime = props.application_access_token_expiry_time;
        this.userAccessTokenExpiryTime = props.user_access_token_expiry_time;
        this.bypassClientCredentials = props.bypassClientCredentials;
        this.pkceMandatory = props.pkceMandatory;
        this.pkceSupportPlain = props.pkceSupportPlain;
        this.refreshTokenExpiryTime = props.refresh_token_expiry_time;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class SubscribedAPI {
    constructor(api) {
        this.apiId = new ApiIdentifier(api.apiId);
        this.subscriber = new Subscriber(api.subscriber);
        this.throttlingPolicy = api.throttlingPolicy;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class ApiIdentifier {
    constructor(apiId) {
        this.providerName = apiId.providerName;
        this.apiName = apiId.apiName;
        this.version = apiId.version;
        this.uuid = apiId.uuid;
        this.id = apiId.id;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class Subscriber {
    constructor(subscriber) {
        this.name = subscriber.name;
        this.id = subscriber.id;
        this.tenantId = subscriber.tenantId;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

module.exports = {
    ImportedApplicationDTO,
    ApplicationInfo,
    ApplicationKey,
    AdditionalProperties,
    SubscribedAPI,
    ApiIdentifier,
    Subscriber
};