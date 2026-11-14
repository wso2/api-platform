/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package attemptverdict

import (
	"log/slog"
	"sync"
	"sync/atomic"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
)

// Sink receives decoded verdicts.
//
// An interface rather than a direct call because the eventual consumer is the model-failover
// policy, which the engine cannot reference: policies depend on the engine through the SDK, never
// the other way round. Delivering a verdict into a policy therefore needs an SDK-level hook,
// which needs an SDK release — tracked as T030k3. Until that lands this seam terminates in
// LoggingSink, so the transport can be built, tested and verified at runtime now rather than
// waiting on a published interface.
type Sink interface {
	// RecordAttemptVerdict is called once per failed attempt, off the request path: the entry
	// arrives on Envoy's batched access-log stream about a second after the request finished.
	// Implementations must not block — one slow sink stalls the stream for every other.
	RecordAttemptVerdict(Verdict)
}

// Consumer decodes access-log entries and forwards the ones carrying a verdict.
type Consumer struct {
	sink Sink

	// seen/forwarded make the path observable without a verdict consumer attached, which is
	// what lets T030k1's plumbing be verified end-to-end before T030k3 exists.
	seen      atomic.Uint64
	forwarded atomic.Uint64
}

// NewConsumer builds a Consumer delivering to sink. A nil sink falls back to LoggingSink.
func NewConsumer(sink Sink) *Consumer {
	if sink == nil {
		sink = &LoggingSink{}
	}
	return &Consumer{sink: sink}
}

// Name identifies this consumer in the ALS handler's logs.
func (c *Consumer) Name() string { return "failover-attempt-verdict" }

// Consume decodes one entry and forwards it if it carries an upstream-failure verdict.
//
// Entries with no such flag — every healthy request, which is nearly all of them when the
// collector is enabled and the sink is therefore unfiltered — are dropped after a few field
// reads and never allocate.
func (c *Consumer) Consume(entry *accesslogv3.HTTPAccessLogEntry) {
	c.seen.Add(1)
	verdict, ok := FromAccessLogEntry(entry)
	if !ok {
		return
	}
	c.forwarded.Add(1)
	c.sink.RecordAttemptVerdict(verdict)
}

// Stats reports how many entries this consumer has seen and how many carried a verdict.
func (c *Consumer) Stats() (seen, forwarded uint64) {
	return c.seen.Load(), c.forwarded.Load()
}

// LoggingSink is the placeholder terminus for verdicts until T030k3 delivers them to the policy.
//
// It is deliberately not silent. A verdict reaching here is a real failure Envoy observed and
// the policy could not see, so an operator reading logs learns about a provider truncating its
// streams even while the delivery hook is still unbuilt — and the integration test that proves
// the transport works has something to assert against.
type LoggingSink struct {
	mu      sync.Mutex
	lastOne Verdict
}

// RecordAttemptVerdict logs the verdict and retains the most recent one for tests.
func (s *LoggingSink) RecordAttemptVerdict(v Verdict) {
	s.mu.Lock()
	s.lastOne = v
	s.mu.Unlock()

	// The request id is Envoy's own correlation id, not a credential or a user identifier.
	attrs := []any{
		"request_id", v.RequestID,
		"upstream_cluster", v.UpstreamCluster,
		"response_flags", v.Flags,
		"response_code", v.ResponseCode,
	}
	// Warn, not Info, when the response was already committed: that is precisely the case the
	// policy grades wrongly today, so it is worth an operator's attention even before the
	// delivery hook exists. A pre-commitment failure surfaced to the client as a 5xx and the
	// policy already graded it, so it is merely informational here.
	if v.CommittedBeforeFailing() {
		slog.Warn("failover attempt failed after its response was committed; "+
			"the policy cannot see this from response headers", attrs...)
		return
	}
	slog.Info("failover attempt failed upstream", attrs...)
}

// Last returns the most recently recorded verdict.
func (s *LoggingSink) Last() Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastOne
}
