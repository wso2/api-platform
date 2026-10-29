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

const { isConnectionFailure, failureKind } = require('./dbFailure');

const BUCKET_MS = 1000;

/**
 * Outcomes of this process's own database operations over a sliding window, kept in
 * one-second buckets so recording is O(1) and old outcomes expire without a sweep.
 *
 * Alongside the counts it remembers when the application last used the database and
 * when the health check last pinged it. A ping counts as an attempt but never as
 * application activity, so health probes cannot make an idle instance look busy.
 */
class OutcomeWindow {
    constructor({ windowMs, now = Date.now } = {}) {
        this.now = now;
        this.size = Math.max(1, Math.ceil(windowMs / BUCKET_MS));
        this.buckets = Array.from({ length: this.size }, () => ({ second: -1, attempts: 0, failures: 0 }));
        this.lastActivity = null;
        this.lastPing = null;
        this.lastFailureKind = null;
    }

    bucketFor(nowMs) {
        const second = Math.floor(nowMs / BUCKET_MS);
        const bucket = this.buckets[second % this.size];
        if (bucket.second !== second) {
            bucket.second = second;
            bucket.attempts = 0;
            bucket.failures = 0;
        }
        return bucket;
    }

    /**
     * Records one operation. `failure` is 'timeout' or 'unreachable' for a counted
     * failure, or null for any other outcome. `ping` marks the health check's own
     * round trip; `activity: false` marks an outcome no caller asked for, such as an
     * idle pooled connection dropping.
     */
    record({ failure = null, ping = false, activity = !ping } = {}) {
        const nowMs = this.now();
        const bucket = this.bucketFor(nowMs);
        bucket.attempts += 1;
        if (failure) {
            bucket.failures += 1;
            this.lastFailureKind = failure;
        }
        if (ping) this.lastPing = nowMs;
        if (activity) this.lastActivity = nowMs;
    }

    /** Records an operation that succeeded. */
    recordSuccess(options = {}) {
        this.record({ ...options, failure: null });
    }

    /** Records an operation that failed with `err`, counting it only if it is connection-class. */
    recordError(err, options = {}) {
        this.record({ ...options, failure: isConnectionFailure(err) ? failureKind(err) : null });
    }

    /** Sums the outcomes still inside the window. */
    snapshot() {
        const nowMs = this.now();
        const oldest = Math.floor(nowMs / BUCKET_MS) - this.size + 1;
        let attempts = 0;
        let failures = 0;
        for (const bucket of this.buckets) {
            if (bucket.second >= oldest) {
                attempts += bucket.attempts;
                failures += bucket.failures;
            }
        }
        return {
            attempts,
            failures,
            lastActivity: this.lastActivity,
            lastPing: this.lastPing,
            lastFailureKind: this.lastFailureKind,
        };
    }
}

module.exports = { OutcomeWindow };
