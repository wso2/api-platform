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

package eventlistener

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
)

const (
	retryBaseDelay = 2 * time.Second
	retryMaxDelay  = time.Minute
)

// renderRetries replays create/update events whose render failed because a
// secret store was unavailable. The event cursor has already moved past such
// an event, so without a replay the replica keeps its old config until the
// next change. A replay goes through the normal handler, which re-reads the
// latest stored revision, so it never applies a stale one. Nothing is applied
// until the render succeeds, so the artifact is never served unresolved.
type renderRetries struct {
	mu       sync.Mutex
	attempts map[string]int
	queued   map[string]bool
	ch       chan eventhub.Event
	base     time.Duration
	max      time.Duration
}

func newRenderRetries() *renderRetries {
	return &renderRetries{
		attempts: map[string]int{},
		queued:   map[string]bool{},
		ch:       make(chan eventhub.Event, 64),
		base:     retryBaseDelay,
		max:      retryMaxDelay,
	}
}

func retryKey(event eventhub.Event) string {
	return string(event.EventType) + "/" + event.EntityID
}

// delay doubles from base per attempt and stops growing at max.
func (r *renderRetries) delay(attempt int) time.Duration {
	d := r.base
	for i := 1; i < attempt && d < r.max; i++ {
		d *= 2
	}
	return min(d, r.max)
}

// retryIfUnavailable schedules a replay of event when err says a secret store
// was unavailable, and reports whether it did. Only one replay per entity is
// pending at a time.
func (l *EventListener) retryIfUnavailable(event eventhub.Event, err error) bool {
	if l.retries == nil || !errors.Is(err, vault.ErrUnavailable) {
		return false
	}
	key := retryKey(event)
	r := l.retries
	r.mu.Lock()
	if r.queued[key] {
		r.mu.Unlock()
		return true
	}
	r.attempts[key]++
	attempt := r.attempts[key]
	r.queued[key] = true
	r.mu.Unlock()

	delay := r.delay(attempt)
	l.logger.Warn("Secret store unavailable; will retry the event",
		slog.String("event_type", string(event.EventType)),
		slog.String("entity_id", event.EntityID),
		slog.Int("attempt", attempt),
		slog.Duration("retry_in", delay),
		slog.Any("error", err))

	replay := eventhub.Event{
		EventType: event.EventType,
		Action:    "UPDATE",
		EntityID:  event.EntityID,
		EventID:   event.EventID,
	}
	time.AfterFunc(delay, func() {
		r.mu.Lock()
		r.queued[key] = false
		r.mu.Unlock()
		select {
		case r.ch <- replay:
		case <-l.ctx.Done():
		}
	})
	return true
}

// clearRetry forgets the backoff for an entity once its event was applied or
// the entity was deleted.
func (l *EventListener) clearRetry(event eventhub.Event) {
	if l.retries == nil {
		return
	}
	l.retries.mu.Lock()
	delete(l.retries.attempts, retryKey(event))
	l.retries.mu.Unlock()
}

// RetryDeferred queues stored artifacts that startup left out because a secret
// store was unavailable. Each is applied through the normal event handler once
// its secrets resolve. Call it after Start.
func (l *EventListener) RetryDeferred(configs []*models.StoredConfig) {
	for _, cfg := range configs {
		eventType, ok := eventTypeForKind(cfg.Kind)
		if !ok {
			continue
		}
		event := eventhub.Event{EventType: eventType, Action: "UPDATE", EntityID: cfg.UUID, EventID: "startup-deferred"}
		l.retryIfUnavailable(event, vault.ErrUnavailable)
	}
}

func eventTypeForKind(kind string) (eventhub.EventType, bool) {
	switch kind {
	case models.KindRestApi, models.KindGraphQLApi:
		return eventhub.EventTypeAPI, true
	case models.KindMcp:
		return eventhub.EventTypeMCPProxy, true
	case models.KindLlmProvider:
		return eventhub.EventTypeLLMProvider, true
	case models.KindLlmProxy:
		return eventhub.EventTypeLLMProxy, true
	case models.KindAgent:
		return eventhub.EventTypeAgent, true
	default:
		return "", false
	}
}
