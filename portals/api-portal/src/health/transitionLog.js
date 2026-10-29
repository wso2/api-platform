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

const { formatDuration } = require('./duration');

/**
 * Turns health results into log lines, one per change of state: a check going down is
 * logged once at error level, and its recovery once at info level. Nothing is logged
 * while a check stays in the same state, so a long outage produces exactly two lines.
 * Only the safe failure reason a check reports is ever logged.
 *
 * `logger` defaults to the portal logger, loaded on first use so this module can be
 * required without resolving the portal config.
 */
function createTransitionLogger({ logger } = {}) {
    // When each check that is currently down started failing, in epoch milliseconds.
    const failingSince = new Map();
    const log = () => logger || require('../config/logger');

    return {
        /**
         * Records one completed evaluation. `checks` maps a check name to
         * `{ status: 'up' | 'down', error?, sinceMs? }`; `nowMs` is the evaluation time.
         */
        observe(checks, nowMs) {
            for (const name of Object.keys(checks).sort()) {
                const check = checks[name];
                const failing = failingSince.has(name);
                if (check.status === 'down' && !failing) {
                    failingSince.set(name, check.sinceMs ?? nowMs);
                    log().error('health check failed', { check: name, reason: check.error });
                } else if (check.status === 'up' && failing) {
                    const downFor = Math.round((nowMs - failingSince.get(name)) / 1000) * 1000;
                    failingSince.delete(name);
                    log().info('health check recovered', { check: name, down_for: formatDuration(downFor) });
                }
            }
        },
    };
}

module.exports = { createTransitionLogger };
