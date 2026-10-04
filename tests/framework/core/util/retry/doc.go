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

// Package retry owns every deadline-bounded wait in the suites. Nothing sleeps; the
// framework polls. Hand-rolled loops are the main cause of flaky parallel tests, and
// the design this is ported from accumulated 31 copies of the same loop before
// consolidation — one of which had silently drifted onto a hardcoded window that
// ignored the shared ceiling and hid a CI failure.
//
// One loop, three contracts. Picking a contract IS the statement of intent, the same
// way the tcontext read intents are:
//
//	Until          - the result IS the assertion target. Returns the LAST result so
//	                 the caller publishes it and asserts the exact value itself.
//	                 Side-effect free. Retries only transient connectivity, so a bad
//	                 context key or payload fails fast instead of being masked as a
//	                 timeout.
//	OrHeal         - a self-healing gate for PREREQUISITE state only, never an
//	                 assertion target. Waits the full propagation window, then
//	                 re-fires the triggering action, because runtime-propagation
//	                 events are at-most-once and a dropped one can only be fixed by
//	                 re-emitting it. Treats every error as not-ready — during warm-up
//	                 "not ready" and "probe failed" are indistinguishable — and fails
//	                 the test on exhaustion. Logs each heal so occurrences stay
//	                 countable.
//	SettledCount   - the assertion target is a monotonic counter's FINAL value. Polls
//	                 until the value has been unchanged for a quiet period, then the
//	                 caller asserts both settled and the exact count. Necessary
//	                 because Until can only ask "has it reached N", so accept-on-N
//	                 passes the instant the counter touches N and an extra arrival a
//	                 moment later is invisible — an over-count wearing an exact
//	                 assertion.
//
// Using OrHeal for an assertion target swallows fail-fast errors. Using Until for
// prerequisite state never re-fires a dropped event. Using Until for a final count
// silently permits an over-count.
//
// Every deadline is FLOORED at the single propagation ceiling, so no call site can
// drift below the shared value. An unexpected 401 inside a loop must be reported with
// the credential's identifier, not just retried: a rejected credential is revoked and
// can never recover by being replayed, and re-minting it would hide a product defect.
package retry
