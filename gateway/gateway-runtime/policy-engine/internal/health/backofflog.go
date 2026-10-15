/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package health

import (
	"log/slog"
	"sort"
	"time"
)

// failureState tracks one check that is currently down.
type failureState struct {
	failingSince time.Time
	failedCount  int
	interval     time.Duration
	nextLogAt    time.Time
}

// backoffLogger turns check results into log lines. A check that goes down is logged at
// once and then again at growing intervals while it stays down, so a long outage does not
// flood the log; recovery is logged once. It is not safe for concurrent use; the Checker
// calls it with its lock held.
type backoffLogger struct {
	health   *slog.Logger
	debug    *slog.Logger
	initial  time.Duration
	max      time.Duration
	failures map[string]*failureState
}

func newBackoffLogger(health, debug *slog.Logger, initial, max time.Duration) *backoffLogger {
	return &backoffLogger{
		health:   health,
		debug:    debug,
		initial:  initial,
		max:      max,
		failures: make(map[string]*failureState),
	}
}

// observe records one completed evaluation and logs the state changes and due repeats it
// implies. The health logger only ever receives the safe failure reason; the raw cause
// goes to the debug logger.
func (b *backoffLogger) observe(res Result, now time.Time) {
	names := make([]string, 0, len(res.Checks))
	for name := range res.Checks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		chk := res.Checks[name]
		st, failing := b.failures[name]

		switch {
		case chk.Status == StatusDown && !failing:
			since := chk.Since
			if since.IsZero() {
				since = now
			}
			b.failures[name] = &failureState{
				failingSince: since,
				failedCount:  1,
				interval:     b.initial,
				nextLogAt:    now.Add(b.initial),
			}
			b.health.Error("health check failed", "check", name, "reason", chk.Error)
			b.logCause(name, chk)

		case chk.Status == StatusDown:
			st.failedCount++
			if now.Before(st.nextLogAt) {
				continue
			}
			b.health.Error("health check failed",
				"check", name,
				"reason", chk.Error,
				"failing_for", now.Sub(st.failingSince).Round(time.Second).String(),
				"failed_probes", st.failedCount,
			)
			b.logCause(name, chk)
			st.interval = min(2*st.interval, b.max)
			st.nextLogAt = now.Add(st.interval)

		case failing:
			b.health.Info("health check recovered",
				"check", name,
				"down_for", now.Sub(st.failingSince).Round(time.Second).String(),
			)
			delete(b.failures, name)
		}
	}

	for name := range b.failures {
		if _, ok := res.Checks[name]; !ok {
			delete(b.failures, name)
		}
	}
}

// logCause writes the raw check error at debug level. It never reaches the health logger.
func (b *backoffLogger) logCause(name string, chk CheckResult) {
	if chk.cause != nil {
		b.debug.Debug("health check error detail", "check", name, "error", chk.cause.Error())
	}
}
