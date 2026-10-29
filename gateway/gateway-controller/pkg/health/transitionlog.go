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

// transitionLogger turns check results into log lines, one per state change: a check
// that goes down is logged once as failed, and once as recovered when it comes back up.
// Nothing is logged while a check stays in the same state, so an outage of any length
// adds two lines. It is not safe for concurrent use; the Checker calls it with its lock
// held.
type transitionLogger struct {
	health *slog.Logger
	debug  *slog.Logger
	// failingSince holds, for each check that is currently down, when it went down.
	failingSince map[string]time.Time
}

func newTransitionLogger(health, debug *slog.Logger) *transitionLogger {
	return &transitionLogger{
		health:       health,
		debug:        debug,
		failingSince: make(map[string]time.Time),
	}
}

// observe records one completed evaluation and logs the state changes it implies. Every
// check starts out up, so a check that is down at its first evaluation is logged as
// failed. The health logger only ever receives the safe failure reason; the raw cause
// goes to the debug logger. Checks registered as unlogged are skipped entirely.
func (t *transitionLogger) observe(res Result, now time.Time) {
	names := make([]string, 0, len(res.Checks))
	for name, chk := range res.Checks {
		if chk.unlogged {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		chk := res.Checks[name]
		since, failing := t.failingSince[name]

		switch {
		case chk.Status == StatusDown && !failing:
			since = chk.Since
			if since.IsZero() {
				since = now
			}
			t.failingSince[name] = since
			t.health.Error("health check failed", "check", name, "reason", chk.Error)
			t.logCause(name, chk)

		case chk.Status == StatusUp && failing:
			t.health.Info("health check recovered",
				"check", name,
				"down_for", now.Sub(since).Round(time.Second).String(),
			)
			delete(t.failingSince, name)
		}
	}

	for name := range t.failingSince {
		if _, ok := res.Checks[name]; !ok {
			delete(t.failingSince, name)
		}
	}
}

// logCause writes the raw check error at debug level. It never reaches the health logger.
func (t *transitionLogger) logCause(name string, chk CheckResult) {
	if chk.cause != nil {
		t.debug.Debug("health check error detail", "check", name, "error", chk.cause.Error())
	}
}
