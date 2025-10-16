/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package websocket

import (
	"sync/atomic"
	"time"
)

// DeliveryStats tracks event delivery statistics in memory using atomic operations.
// This structure provides operational visibility into event delivery success/failure
// rates without requiring database persistence.
//
// Design rationale: Atomic counters enable lock-free concurrent updates from multiple
// goroutines handling event delivery. Stats reset on server restart, which is acceptable
// for operational monitoring (persistent metrics can be added via Prometheus later).
type DeliveryStats struct {
	// TotalEventsSent tracks the cumulative number of events sent to all gateways.
	// Updated atomically using atomic.AddInt64() to ensure thread-safety.
	TotalEventsSent int64

	// FailedDeliveries tracks the cumulative number of event delivery failures.
	// A failure occurs when:
	//   - Gateway is not connected (no active WebSocket connection)
	//   - Send operation fails (e.g., connection closed during send)
	//   - Payload exceeds maximum size limit
	FailedDeliveries int64

	// LastFailureTime records the timestamp of the most recent delivery failure.
	// Not atomic - updated under lock or during single-threaded stats query.
	LastFailureTime time.Time

	// LastFailureReason contains a human-readable description of the most recent failure.
	// Examples: "gateway not connected", "send timeout", "payload too large"
	// Not atomic - updated under lock or during single-threaded stats query.
	LastFailureReason string
}

// IncrementTotalSent atomically increments the total events sent counter.
// This method is thread-safe and can be called concurrently from multiple goroutines.
func (s *DeliveryStats) IncrementTotalSent() {
	atomic.AddInt64(&s.TotalEventsSent, 1)
}

// IncrementFailed atomically increments the failed deliveries counter and
// records the failure details.
//
// Parameters:
//   - reason: Human-readable description of why the delivery failed
//
// Note: LastFailureTime and LastFailureReason updates are not atomic. This is
// acceptable for monitoring purposes where exact synchronization is not critical.
func (s *DeliveryStats) IncrementFailed(reason string) {
	atomic.AddInt64(&s.FailedDeliveries, 1)
	s.LastFailureTime = time.Now()
	s.LastFailureReason = reason
}

// GetTotalSent returns the current value of total events sent.
// Uses atomic load to ensure visibility of concurrent updates.
func (s *DeliveryStats) GetTotalSent() int64 {
	return atomic.LoadInt64(&s.TotalEventsSent)
}

// GetFailedCount returns the current value of failed deliveries.
// Uses atomic load to ensure visibility of concurrent updates.
func (s *DeliveryStats) GetFailedCount() int64 {
	return atomic.LoadInt64(&s.FailedDeliveries)
}

// GetSuccessRate calculates the percentage of successful event deliveries.
// Returns 100.0 if no events have been sent (avoiding division by zero).
func (s *DeliveryStats) GetSuccessRate() float64 {
	total := s.GetTotalSent()
	if total == 0 {
		return 100.0
	}
	failed := s.GetFailedCount()
	successful := total - failed
	return (float64(successful) / float64(total)) * 100.0
}
