
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
const constants = require('../utils/constants');

class APIDTO {
    constructor(api) {
        this.apiID = api.ID;
        this.apiReferenceID = api.REF_ID;
        this.apiHandle = api.HANDLE;
        this.dataSource = api.DATA_SOURCE;
        this.apiInfo = new APIInfo(api);
        this.endPoints = new Endpoints(api);
        
        if (api.DP_SUBSCRIPTION_PLANs) {
            this.subscriptionPlans = api.DP_SUBSCRIPTION_PLANs.map(plan => new APISubscriptionPlan(plan));
        }
        if (api.DP_APPLICATIONs && api.DP_APPLICATIONs.length > 0) {
            this.planID = api.DP_APPLICATIONs[0]?.DP_API_SUBSCRIPTION?.dataValues?.PLAN_ID;
        }
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class APIInfo {
    constructor(apiInfo) {
        this.apiName = apiInfo.NAME;
        this.apiTitle = apiInfo.METADATA_SEARCH?.apiInfo?.apiTitle || null;
        this.remotes = apiInfo.METADATA_SEARCH?.apiInfo?.remotes || [];
        this.apiVersion = apiInfo.VERSION;
        this.apiDescription = apiInfo.DESCRIPTION;
        this.apiType = apiInfo.TYPE;
        this.apiStatus = apiInfo.STATUS;
        this.agentVisibility = apiInfo.AGENT_VISIBILITY || 'VISIBLE';
        if (apiInfo.addedLabels) {
            this.addedLabels = apiInfo.addedLabels;
        }
        if (apiInfo.removedLabels) {
            this.removedLabels = apiInfo.removedLabels;
        }
        if (apiInfo.BUSINESS_OWNER || apiInfo.TECHNICAL_OWNER) {
            this.owners = new Owner(apiInfo);
        }
        if (apiInfo.DP_API_CONTENTs) {
            const images = apiInfo.DP_API_CONTENTs.filter(content => content.TYPE === constants.DOC_TYPES.IMAGES);
            this.apiImageMetadata = getAPIImages(images);
        }
        if (apiInfo.TAGS) {
            this.tags = apiInfo.TAGS.split(" ");
        }
        if (apiInfo.DP_LABELs) {
            this.labels = apiInfo.DP_LABELs.map(label => label.dataValues ? label.dataValues.NAME : label);
        }
    }
}

class APISubscriptionPlan {
    constructor(apiSubscriptionPlan) {
        this.planName = apiSubscriptionPlan.NAME;
        this.displayName = apiSubscriptionPlan.DISPLAY_NAME;
        this.requestCount = apiSubscriptionPlan.REQUEST_COUNT;
        this.planID = apiSubscriptionPlan.ID;
        this.description = apiSubscriptionPlan.DESCRIPTION;
    }
}

class Owner {
    constructor(api) {
        this.technicalOwner = api.TECHNICAL_OWNER;
        this.businessOwner = api.BUSINESS_OWNER;
        if (api.BUSINESS_OWNER_EMAIL) {
            this.businessOwnerEmail = api.BUSINESS_OWNER_EMAIL;
        }
        if (api.TECHNICAL_OWNER_EMAIL) {
            this.technicalOwnerEmail = api.TECHNICAL_OWNER_EMAIL;
        }
    }
}

class Endpoints {
    constructor(api) {
        this.sandboxURL = api.SANDBOX_URL;
        this.productionURL = api.PRODUCTION_URL;
    }
}

class APIImages {
    constructor(data = {}) {
        Object.assign(this, data);
    }
}

const getAPIImages = (apiImages) => {
    let images = {}
    apiImages.forEach(element => {
        images[element.LOOKUP_KEY] = element.FILE_NAME;
    });
    return new APIImages(images);
}

module.exports = APIDTO;
