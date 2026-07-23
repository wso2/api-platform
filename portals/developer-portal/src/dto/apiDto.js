
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
const { applyAudit } = require('./dtoUtils');

class APIDTO {
    constructor(api, audit) {
        this.id = api.handle;
        this.refId = api.ref_id;
        this.dataSource = api.DATA_SOURCE;
        Object.assign(this, new APIInfo(api));
        this.endPoints = new Endpoints(api);

        if (api.dp_subscription_plans) {
            this.subscriptionPlans = api.dp_subscription_plans.map(plan => new APISubscriptionPlan(plan));
        }

        applyAudit(this, audit);
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
        this.name = apiInfo.name;
        this.title = apiInfo.metadata_search?.title || null;
        this.remotes = apiInfo.metadata_search?.remotes || [];
        this.version = apiInfo.version;
        this.description = apiInfo.description;
        this.type = apiInfo.type;
        this.status = apiInfo.status;
        this.agentVisibility = apiInfo.agent_visibility || 'VISIBLE';
        if (apiInfo.addedLabels) {
            this.addedLabels = apiInfo.addedLabels;
        }
        if (apiInfo.removedLabels) {
            this.removedLabels = apiInfo.removedLabels;
        }
        if (apiInfo.business_owner || apiInfo.technical_owner) {
            this.owners = new Owner(apiInfo);
        }
        if (apiInfo.dp_api_contents) {
            const images = apiInfo.dp_api_contents.filter(content => content.type === constants.DOC_TYPES.IMAGES);
            this.apiImageMetadata = getAPIImages(images);
        }
        if (apiInfo.dp_tags) {
            this.tags = apiInfo.dp_tags.map(tag => tag.dataValues ? tag.dataValues.name : tag);
        }
        if (apiInfo.dp_labels) {
            this.labels = apiInfo.dp_labels.map(label => label.dataValues ? label.dataValues.handle : label);
        }
    }
}

class APISubscriptionPlan {
    constructor(apiSubscriptionPlan) {
        this.displayName = apiSubscriptionPlan.display_name;
        this.id = apiSubscriptionPlan.handle;
        this.description = apiSubscriptionPlan.description;
        this.limits = (apiSubscriptionPlan.limits || []).map(l => ({
            limitType:  l.limit_type,
            timeUnit:   l.time_unit ?? null,
            timeAmount: l.time_amount,
            limitCount: Number(l.limit_count),
        }));
    }
}

class Owner {
    constructor(api) {
        this.technicalOwner = api.technical_owner;
        this.businessOwner = api.business_owner;
        if (api.business_owner_email) {
            this.businessOwnerEmail = api.business_owner_email;
        }
        if (api.technical_owner_email) {
            this.technicalOwnerEmail = api.technical_owner_email;
        }
    }
}

class Endpoints {
    constructor(api) {
        this.sandboxURL = api.sandbox_url;
        this.productionURL = api.production_url;
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
        images[element.lookup_key] = element.file_name;
    });
    return new APIImages(images);
}

module.exports = APIDTO;
