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

// Self-healing gate for prerequisite state.

package retry

import (
	"context"
	"fmt"
	"time"
)

// Trigger re-fires the action whose effect is being waited for.
type Trigger func(ctx context.Context) error

// Probe reports whether the awaited state has arrived.
type Probe func(ctx context.Context) (bool, error)

// HealOptions tune a self-healing gate.
type HealOptions struct {
	Options
	// What names the awaited state, for diagnostics.
	What string
	// MaxHeals bounds how many times the trigger is re-fired.
	MaxHeals int
	// HealAfter is how long to wait before re-firing.
	HealAfter time.Duration
}

// OrHeal waits for prerequisite state, re-firing the trigger if it does not arrive, because
// propagation events are at-most-once. Every probe error counts as not-ready. Not side-effect
// free, so never use it for an assertion target.
func OrHeal(ctx context.Context, opts HealOptions, probe Probe, trigger Trigger) error {
	what := opts.What
	if what == "" {
		what = "prerequisite state"
	}
	healAfter := opts.HealAfter
	if healAfter <= 0 {
		healAfter = time.Minute
	}
	maxHeals := opts.MaxHeals
	if maxHeals < 0 {
		maxHeals = 0
	}

	start := time.Now()
	log := opts.logger()

	for heal := 0; ; heal++ {
		window := time.Now().Add(healAfter)
		attempts := 0

		for time.Now().Before(window) {
			attempts++
			ready, err := probe(ctx)
			if err == nil && ready {
				if heal > 0 {
					log.Warn("self-heal: prerequisite arrived only after re-triggering",
						"what", what, "heals", heal, "elapsed", time.Since(start).Round(time.Millisecond))
				}
				return nil
			}

			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("retry: waiting for %s cancelled after %d attempt(s): %w",
					what, attempts, ctxErr)
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry: waiting for %s cancelled: %w", what, ctx.Err())
			case <-time.After(pacing(time.Since(start), opts.interval())):
			}
		}

		if heal >= maxHeals {
			return fmt.Errorf("retry: %s did not arrive within %s after %d re-trigger(s); "+
				"propagation events are at-most-once, so this is either a dropped event or a product failure",
				what, time.Since(start).Round(time.Second), heal)
		}

		log.Warn("self-heal: re-triggering", "what", what, "attempt", heal+1,
			"elapsed", time.Since(start).Round(time.Millisecond))
		if err := trigger(ctx); err != nil {
			return fmt.Errorf("retry: re-triggering %s failed: %w", what, err)
		}
	}
}
