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

/**
 * DTO for webhook subscriber responses.
 * Never exposes the subscriber's secret in API responses.
 */
class WebhookSubscriberDTO {
    constructor(sub) {
        this.id = sub.UUID;
        this.orgId = sub.ORG_UUID;
        this.name = sub.NAME;
        this.url = sub.TARGET_URL;
        this.enabled = sub.ENABLED;
        this.events = sub.EVENT_PATTERNS || [];
        this.timeoutMs = sub.TIMEOUT_MS;
        this.hasSecret = !!sub.SECRET_ENC;
        this.hasPublicKey = !!sub.PUBLIC_KEY;
    }
}

module.exports = { WebhookSubscriberDTO };
