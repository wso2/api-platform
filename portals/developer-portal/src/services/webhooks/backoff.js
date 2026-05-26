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
const { config } = require('../../config/configLoader');

const DEFAULT_BACKOFF_SECONDS = [60, 300, 900, 1800, 3600, 7200, 14400, 28800];

function getBackoffSeconds() {
    const delivery = config.webhooks && config.webhooks.delivery;
    return (delivery && delivery.backoff) || DEFAULT_BACKOFF_SECONDS;
}

function getMaxAttempts() {
    const delivery = config.webhooks && config.webhooks.delivery;
    return (delivery && delivery.maxAttempts) || 8;
}

/**
 * Returns the Date at which the next retry should be attempted.
 * @param {number} attemptCount — number of attempts already made (0-based before this attempt)
 */
function nextAttemptAt(attemptCount) {
    const schedule = getBackoffSeconds();
    const delaySec = schedule[Math.min(attemptCount, schedule.length - 1)];
    return new Date(Date.now() + delaySec * 1000);
}

module.exports = { nextAttemptAt, getMaxAttempts };
