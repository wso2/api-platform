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

const { telemetryClient } = require('./telemetryClient');
const { config } = require('../config/configLoader');

const WSO2_EMAIL_SUFFIX = '@wso2.com';

const DEFAULT_PROPERTIES = {
    context: 'devportal',
    origin: 'bijira',
};

function trackEventWithDefaults(event, req) {
    if (!config.telemetry?.enabled) {
        return;
    }

    const email = req?.session?.passport?.user?.email || '';
    const isWSO2User = !!email.endsWith(WSO2_EMAIL_SUFFIX);
    const enrichedEvent = {
        name: event.name,
        properties: {
            isWSO2User: isWSO2User,
            ...DEFAULT_PROPERTIES,
            ...(event.properties || {}),
        },
    };

    telemetryClient.trackEvent(enrichedEvent);
}

/**
 * Module exports for telemetry helper functionality.
 * @namespace TelemetryHelper
 */
module.exports = {
    /**
     * Function to track events with default properties across multiple telemetry services.
     * @function
     * @see trackEventWithDefaults
     */
    trackEventWithDefaults,
};
