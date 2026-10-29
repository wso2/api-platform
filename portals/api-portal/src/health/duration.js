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
 * Durations in the same notation the Go components use for their health settings
 * ("3s", "500ms", "1m30s"), so one value means the same thing in every component's
 * config. Kept free of requires: the config loader uses it during bootstrap.
 */

const UNIT_MS = { ms: 1, s: 1000, m: 60 * 1000, h: 60 * 60 * 1000 };
const DURATION_PATTERN = /^(?:\d+(?:\.\d+)?(?:ms|s|m|h))+$/;
const PART_PATTERN = /(\d+(?:\.\d+)?)(ms|s|m|h)/g;

/**
 * Parses a duration string into milliseconds. Returns NaN for anything that is not a
 * string of number-unit pairs (units ms, s, m, h); a bare number is rejected because
 * its unit would be a guess.
 */
function parseDuration(value) {
    if (typeof value !== 'string') return NaN;
    const text = value.trim();
    if (!DURATION_PATTERN.test(text)) return NaN;
    let total = 0;
    for (const [, amount, unit] of text.matchAll(PART_PATTERN)) {
        total += Number(amount) * UNIT_MS[unit];
    }
    return total;
}

/**
 * Formats milliseconds the way Go prints a time.Duration: "500ms", "2s", "1.5s",
 * "1m5s", "1h0m0s".
 */
function formatDuration(ms) {
    const value = Math.max(0, Math.round(ms));
    if (value === 0) return '0s';
    if (value < 1000) return `${value}ms`;
    const hours = Math.floor(value / UNIT_MS.h);
    const minutes = Math.floor((value % UNIT_MS.h) / UNIT_MS.m);
    const seconds = (value % UNIT_MS.m) / 1000;
    const secondsText = `${Number(seconds.toFixed(3))}s`;
    if (hours > 0) return `${hours}h${minutes}m${secondsText}`;
    if (minutes > 0) return `${minutes}m${secondsText}`;
    return secondsText;
}

module.exports = { parseDuration, formatDuration };
