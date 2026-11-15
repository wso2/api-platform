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

package it

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// RuntimeLogSteps asserts on what the gateway runtime actually logged.
//
// Reached for only when a behaviour has no HTTP-visible signature. That is not a loophole —
// it is the correct observable for a decision whose whole point is to be invisible to the
// client. Model failover answering an all-suspended chain is the motivating case: refusing
// before any attempt and exhausting a chain of attempts return byte-identical responses by
// design, so the response cannot distinguish them and only the record can.
//
// Prefer an HTTP assertion wherever one exists. A log line is a weaker contract than a
// response: it can be reworded without breaking a build, so a scenario resting on one is
// asserting something less durable than it looks.
type RuntimeLogSteps struct {
	// mark is the log length noted by the "I note..." step. Scoping to what was written
	// AFTER it is what stops a match being satisfied by an earlier scenario's output —
	// the stack is shared for the whole suite, so an unscoped grep would pass on
	// somebody else's line and keep passing after the behaviour under test regressed.
	mark int
}

// RegisterRuntimeLogSteps registers the runtime-log step definitions.
func RegisterRuntimeLogSteps(ctx *godog.ScenarioContext) {
	r := &RuntimeLogSteps{}
	ctx.Step(`^I note the current gateway runtime log position$`, r.iNoteTheLogPosition)
	ctx.Step(`^the gateway runtime log since then should contain "([^"]*)"$`, r.theLogSinceThenShouldContain)
	ctx.Step(`^the gateway runtime log since then should not contain "([^"]*)"$`, r.theLogSinceThenShouldNotContain)
	ctx.Step(`^the gateway runtime log since then should contain "([^"]*)" exactly (\d+) times$`, r.theLogSinceThenShouldContainExactly)
}

// runtimeLog returns everything the gateway-runtime container has written so far.
func runtimeLog() (string, error) {
	cmdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "docker", "logs", "it-gateway-runtime").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to read gateway runtime logs: %w", err)
	}
	return string(out), nil
}

func (r *RuntimeLogSteps) iNoteTheLogPosition() error {
	current, err := runtimeLog()
	if err != nil {
		return err
	}
	r.mark = len(current)
	return nil
}

// since returns the log written after the noted position.
//
// The container buffers, so a line written moments ago may not be readable yet. Rather than a
// blanket sleep this retries briefly for the caller that is waiting for something to appear;
// the negative step skips the wait, since waiting for an absence only slows the suite down.
func (r *RuntimeLogSteps) since(waitFor string) (string, error) {
	deadline := time.Now().Add(15 * time.Second)
	for {
		full, err := runtimeLog()
		if err != nil {
			return "", err
		}
		tail := full
		if r.mark <= len(full) {
			tail = full[r.mark:]
		}
		if waitFor == "" || strings.Contains(tail, waitFor) || time.Now().After(deadline) {
			return tail, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (r *RuntimeLogSteps) theLogSinceThenShouldContain(expected string) error {
	tail, err := r.since(expected)
	if err != nil {
		return err
	}
	if !strings.Contains(tail, expected) {
		return fmt.Errorf("gateway runtime log does not contain %q since the noted position (%d bytes written since)",
			expected, len(tail))
	}
	return nil
}

func (r *RuntimeLogSteps) theLogSinceThenShouldNotContain(unexpected string) error {
	tail, err := r.since("")
	if err != nil {
		return err
	}
	if strings.Contains(tail, unexpected) {
		return fmt.Errorf("gateway runtime log contains %q since the noted position, and must not", unexpected)
	}
	return nil
}

// sinceAtLeast waits until `expected` has been written at least n times after the noted
// position, or the deadline passes. Separate from since() because a single occurrence is not
// the condition: an assertion about a REPEATED line would otherwise pass on the first one and
// never observe whether the repeat happened at all.
func (r *RuntimeLogSteps) sinceAtLeast(expected string, n int) (string, int, error) {
	deadline := time.Now().Add(15 * time.Second)
	for {
		full, err := runtimeLog()
		if err != nil {
			return "", 0, err
		}
		tail := full
		if r.mark <= len(full) {
			tail = full[r.mark:]
		}
		count := strings.Count(tail, expected)
		if count >= n || time.Now().After(deadline) {
			return tail, count, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// theLogSinceThenShouldContainExactly pins a count rather than a floor.
//
// Exactness is deliberate where the count IS the property — an attempt sequence, for
// instance. "At least once" would be satisfied by a log that emits two lines per attempt as
// readily as by one attempt logged twice, so a floor cannot tell a repeat from a formatting
// detail. An exact count fails loudly if either changes, which is the correct outcome: both
// would invalidate what the scenario claims to observe.
//
// Waits for the count to REACH n, then settles before judging, so a still-buffering log is
// not mistaken for a smaller sequence and a late extra line is not missed.
func (r *RuntimeLogSteps) theLogSinceThenShouldContainExactly(expected string, n int) error {
	if _, _, err := r.sinceAtLeast(expected, n); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	tail, err := r.since("")
	if err != nil {
		return err
	}
	if count := strings.Count(tail, expected); count != n {
		return fmt.Errorf("gateway runtime log contains %q %d times since the noted position, want exactly %d (%d bytes written since)",
			expected, count, n, len(tail))
	}
	return nil
}
