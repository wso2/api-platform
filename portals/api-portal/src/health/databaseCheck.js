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
// A warning, not a failure: a saturated pool is load, and the database still answers.
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
 *   2. With fewer attempts than that, it runs one SELECT 1 of its own per `idlePeriod`,
 *      bounded by `checkTimeout`, and that result decides. It does so even while the
 *      portal uses the database, since traffic too thin for rule 1 cannot show an outage
 *      or a recovery, and a decision left standing would never expire.
 *   3. Between those pings the last decided state stands; with none yet, rule 2
 *      applies.
 *   4. An up result carries a pool-exhaustion warning once the pool has stayed
 *      saturated for `poolWaitLimit`. It never turns the check down: a full pool on a
 *      healthy database is load, and taking busy instances out of service would only
 *      push that load onto the others.
 *
 * Two signals take precedence over older traffic. Once the pool has raised a connection
 * error since the last ping, the check pings at once, whatever rule 1 would say from
 * outcomes that may predate the error. And while a failed ping is what took the check
 * down, rule 1 counts only the outcomes recorded since that ping, so successes from
 * before the outage cannot outvote it.
 *
 * So a busy instance never adds health queries, and any other adds at most one per
 * `idlePeriod`. The check resolves to null when up, to `{ warning }` when up with a
 * saturated pool, or to a fixed reason when down; driver text never appears in it.
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
        // A connection error the pool raised since the last ping means the decided
        // state, and the outcomes rule 1 would judge, may be stale, so it is checked
        // again at once.
        const poolErrorSincePing = snap.lastPoolError !== null &&
            (snap.lastPing === null || snap.lastPoolError > snap.lastPing);
        // While a failed ping keeps the check down, only what happened after it counts:
        // outcomes from the seconds after the one the ping failed in. The window counts in
        // whole seconds, so that second itself is left out, as it can hold successes
        // recorded before the ping failed.
        const failedPingAt = decided && !decided.up ? decided.failedPingAt ?? null : null;
        if (!poolErrorSincePing) {
            const counted = failedPingAt === null
                ? snap
                : window.snapshot({ sinceMs: (Math.floor(failedPingAt / 1000) + 1) * 1000 });
            if (counted.attempts >= settings.minAttempts) {
                const down = counted.failures / counted.attempts >= settings.failureRatio;
                return down
                    ? { up: false, reason: reasonFor(snap.lastFailureKind), failedPingAt }
                    : { up: true, reason: null };
            }
        }
        const quiet = (last) => last === null || nowMs - last >= settings.idlePeriodMs;
        if (decided && !quiet(snap.lastPing) && !poolErrorSincePing) {
            return decided;
        }
        try {
            await ping({ timeoutMs: settings.checkTimeoutMs });
            return { up: true, reason: null };
        } catch (err) {
            return {
                up: false,
                reason: err?.timedOut ? timedOut : reasonFor(failureKind(err)),
                // When the ping finished, not when it started: outcomes recorded while it
                // was still running do not show a recovery from its failure.
                failedPingAt: now(),
            };
        }
    }

    return async function databaseCheck() {
        decided = await decide(now());
        // The pool is sampled on every evaluation, down or up, so a saturation that
        // ended during an outage does not leave a stale start time behind.
        const nowMs = now();
        const saturated = isSaturated(poolStats(), settings.poolSaturation);
        if (!saturated) {
            saturatedSince = null;
        } else if (saturatedSince === null) {
            saturatedSince = nowMs;
        }
        if (!decided.up) {
            return decided.reason;
        }
        if (!saturated) {
            return null;
        }
        return nowMs - saturatedSince >= settings.poolWaitLimitMs ? { warning: POOL_EXHAUSTED } : null;
    };
}

module.exports = { createDatabaseCheck, isSaturated };
