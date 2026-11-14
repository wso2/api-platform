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

package analytics

import (
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// loopbackCarryTTL is how long the provider hop's fields wait for the proxy's own event.
// The provider hop completes first; the proxy's event follows when the response reaches
// the client, so a long stream is the slowest case.
const loopbackCarryTTL = 10 * time.Minute

// loopbackCarry is what the suppressed provider hop of an LLM proxy call knew and the
// proxy's own event does not: the budget property and the cost, which are produced on the
// provider route.
type loopbackCarry struct {
	budget  map[string]interface{}
	llmCost any
	stored  time.Time
}

// loopbackCarryover holds the provider hop's fields by request id until the proxy's own
// event for the same call takes them.
type loopbackCarryover struct {
	mu        sync.Mutex
	entries   map[string]loopbackCarry
	lastSweep time.Time
	now       func() time.Time
}

func newLoopbackCarryover() *loopbackCarryover {
	return &loopbackCarryover{entries: map[string]loopbackCarry{}, now: time.Now}
}

// keep stores what the suppressed provider hop carries, if anything.
func (l *loopbackCarryover) keep(requestID string, event *dto.Event) {
	if l == nil || requestID == "" || event == nil {
		return
	}
	carry := loopbackCarry{}
	if budget, ok := event.Properties[constants.BudgetPropertyKey].(map[string]interface{}); ok && len(budget) > 0 {
		carry.budget = budget
	}
	if md, ok := event.Properties["aiMetadata"].(dto.AIMetadata); ok && md.LLMCost != nil {
		carry.llmCost = md.LLMCost
	}
	if carry.budget == nil && carry.llmCost == nil {
		return
	}
	now := l.now()
	carry.stored = now
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(now)
	l.entries[requestID] = carry
}

// apply gives the proxy's own event what its provider hop carried, where the event has
// none of its own, and forgets the entry.
func (l *loopbackCarryover) apply(requestID string, event *dto.Event) {
	if l == nil || requestID == "" || event == nil || event.API == nil || event.API.APIType != "LlmProxy" {
		return
	}
	l.mu.Lock()
	carry, ok := l.entries[requestID]
	if ok {
		delete(l.entries, requestID)
	}
	l.mu.Unlock()
	if !ok || l.now().Sub(carry.stored) > loopbackCarryTTL {
		return
	}
	if event.Properties == nil {
		event.Properties = map[string]interface{}{}
	}
	if _, has := event.Properties[constants.BudgetPropertyKey]; !has && carry.budget != nil {
		event.Properties[constants.BudgetPropertyKey] = carry.budget
	}
	if carry.llmCost != nil {
		if md, ok := event.Properties["aiMetadata"].(dto.AIMetadata); ok && md.LLMCost == nil {
			md.LLMCost = carry.llmCost
			event.Properties["aiMetadata"] = md
		}
	}
}

// sweepLocked drops entries older than loopbackCarryTTL, at most once a minute.
func (l *loopbackCarryover) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for id, carry := range l.entries {
		if now.Sub(carry.stored) > loopbackCarryTTL {
			delete(l.entries, id)
		}
	}
}
