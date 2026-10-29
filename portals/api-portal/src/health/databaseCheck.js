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
const { failureKind } = require('./dbFailure');

const UNREACHABLE = 'database unreachable';
const POOL_EXHAUSTED = 'database connection pool exhausted';

/**
 * Reports whether a pool snapshot is saturated: at least `ratio` of the maximum in use
 * with callers waiting. Single-connection and unlimited pools, and adapters without a
 * pool (null), are never saturated.
 *
 *   pg:    total >= ceil(ratio * max), idle == 0 and waiting > 0
 *   mssql: borrowed >= ceil(ratio * max) and pending > 0
 */
function isSaturated(stats, ratio) {
    if (!stats || !Number.isInteger(stats.max) || stats.max <= 1) return false;
    const threshold = Math.ceil(ratio * stats.max);
    if ('waiting' in stats) {
        return stats.total >= threshold && stats.idle === 0 && stats.waiting > 0;
    }
    if ('pending' in stats) {
        return stats.borrowed >= threshold && stats.pending > 0;
    }
    return false;
}

/**
 * Builds the traffic-aware `database` check. Rather than querying the database on every
 * evaluation, it judges it by the outcomes of the portal's own queries:
 *
 *   1. With at least `minAttempts` attempts in the window, it is down when the share
 *      of connection failures reaches `failureRatio`, and up otherwise.
 *   2. When neither the application nor the check itself has touched the database for
 *      `idlePeriod`, it runs one SELECT 1 of its own, bounded by `checkTimeout`, and
 *      that result decides.
 *   3. Otherwise the last decided state stands; with none yet, rule 2 applies.
 *   4. An up result is turned down once the pool has stayed saturated for
 *      `poolWaitLimit`.
 *
 * So a busy instance never adds health queries, and an idle one adds at most one per
 * `idlePeriod`. The check resolves to null when up, or to a fixed reason when down;
 * driver text never appears in it.
 *
 * @param {object} deps
 * @param {{ snapshot: Function }} deps.window   the driver's outcome window
 * @param {(opts: {timeoutMs: number}) => Promise<void>} deps.ping  the driver's health ping
 * @param {() => object|null} deps.poolStats     the driver's pool counters
 * @param {object} deps.settings  checkTimeoutMs, idlePeriodMs, minAttempts, failureRatio,
 *                                poolSaturation, poolWaitLimitMs
 */
function createDatabaseCheck({ window, ping, poolStats, settings, now = Date.now }) {
    const timedOut = `timed out after ${formatDuration(settings.checkTimeoutMs)}`;
    // The last state rules 1 and 2 decided, kept for rule 3.
    let decided = null;
    // When the pool became saturated, or null while it is not.
    let saturatedSince = null;

    const reasonFor = (kind) => (kind === 'timeout' ? timedOut : UNREACHABLE);

    async function decide(nowMs) {
        const snap = window.snapshot();
        if (snap.attempts >= settings.minAttempts) {
            const down = snap.failures / snap.attempts >= settings.failureRatio;
            return { up: !down, reason: down ? reasonFor(snap.lastFailureKind) : null };
        }
        const quiet = (last) => last === null || nowMs - last >= settings.idlePeriodMs;
        if (decided && !(quiet(snap.lastActivity) && quiet(snap.lastPing))) {
            return decided;
        }
        try {
            await ping({ timeoutMs: settings.checkTimeoutMs });
            return { up: true, reason: null };
        } catch (err) {
            return { up: false, reason: err?.timedOut ? timedOut : reasonFor(failureKind(err)) };
        }
    }

    return async function databaseCheck() {
        decided = await decide(now());
        if (!decided.up) {
            return decided.reason;
        }
        const nowMs = now();
        if (!isSaturated(poolStats(), settings.poolSaturation)) {
            saturatedSince = null;
            return null;
        }
        if (saturatedSince === null) saturatedSince = nowMs;
        return nowMs - saturatedSince >= settings.poolWaitLimitMs ? POOL_EXHAUSTED : null;
    };
}

module.exports = { createDatabaseCheck, isSaturated };
