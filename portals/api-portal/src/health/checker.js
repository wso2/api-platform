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
const { createTransitionLogger } = require('./transitionLog');

// Reason reported for a check that threw instead of returning one. The thrown error's
// own text is never reported: it may carry driver detail such as a host or a user.
const CHECK_FAILED = 'check failed';

/** Renders epoch milliseconds in UTC as RFC 3339 with second precision. */
function formatTime(ms) {
    return new Date(Math.floor(ms / 1000) * 1000).toISOString().replace('.000Z', 'Z');
}

/**
 * Runs the registered health checks and turns them into one result shared by /health
 * and /ready. A result is reused for `cacheTtlMs`, and callers arriving while an
 * evaluation is running wait for that evaluation instead of starting another, so the
 * probe rate never multiplies the work done by the checks.
 *
 * A check is an async function that resolves to nothing when its dependency is up, or
 * to a short, safe reason string when it is down. Each runs under its own timeout.
 */
class HealthChecker {
    constructor({ cacheTtlMs, now = Date.now, transitionLogger = createTransitionLogger() } = {}) {
        this.cacheTtlMs = cacheTtlMs;
        this.now = now;
        this.transitions = transitionLogger;
        this.checks = new Map();
        // When each check that is currently down went down, kept across evaluations.
        this.since = new Map();
        this.cached = null;
        this.inflight = null;
    }

    /** Registers a check under `name`, bounded by `timeoutMs` when one is given. */
    register(name, fn, { timeoutMs } = {}) {
        this.checks.set(name, { fn, timeoutMs });
    }

    /**
     * Resolves to `{ healthy, status: 'ok' | 'unhealthy', timestamp, checks }`, where
     * each check is `{ status: 'up' }` or `{ status: 'down', error, since }`.
     */
    evaluate() {
        if (this.cached && this.now() - this.cached.evaluatedAtMs < this.cacheTtlMs) {
            return Promise.resolve(this.cached.result);
        }
        if (!this.inflight) {
            this.inflight = this.runChecks().finally(() => {
                this.inflight = null;
            });
        }
        return this.inflight;
    }

    async runChecks() {
        const names = [...this.checks.keys()];
        const reasons = await Promise.all(names.map((name) => this.runOne(this.checks.get(name))));
        const nowMs = this.now();

        const observed = {};
        const checks = {};
        let healthy = true;
        names.forEach((name, i) => {
            const reason = reasons[i];
            if (!reason) {
                this.since.delete(name);
                observed[name] = { status: 'up' };
                checks[name] = { status: 'up' };
                return;
            }
            healthy = false;
            if (!this.since.has(name)) this.since.set(name, nowMs);
            const sinceMs = this.since.get(name);
            observed[name] = { status: 'down', error: reason, sinceMs };
            checks[name] = { status: 'down', error: reason, since: formatTime(sinceMs) };
        });

        this.transitions.observe(observed, nowMs);
        const result = {
            healthy,
            status: healthy ? 'ok' : 'unhealthy',
            timestamp: formatTime(nowMs),
            checks,
        };
        this.cached = { evaluatedAtMs: nowMs, result };
        return result;
    }

    /** Runs one check and resolves to its failure reason, or null when it is up. */
    async runOne({ fn, timeoutMs }) {
        let timer;
        const attempt = Promise.resolve()
            .then(fn)
            .then((reason) => reason || null, () => CHECK_FAILED);
        if (!timeoutMs) return attempt;
        const timeout = new Promise((resolve) => {
            timer = setTimeout(() => resolve(`timed out after ${formatDuration(timeoutMs)}`), timeoutMs);
            timer.unref?.();
        });
        try {
            return await Promise.race([attempt, timeout]);
        } finally {
            clearTimeout(timer);
        }
    }
}

module.exports = { HealthChecker, formatTime };
