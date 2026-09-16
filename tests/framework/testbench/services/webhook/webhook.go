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

// Package webhook provides a block-partitioned HTTP receiver for API Portal deliveries.
package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3012

const maxDeliveryBodyBytes = 10 << 20

const maxRetainedDeliveries = 1024

type delivery struct {
	Headers map[string][]string `json:"headers"`
	Body    json.RawMessage     `json:"body"`
}

type Service struct {
	mu         sync.RWMutex
	partitions map[string]*partition
}

type partition struct {
	mu         sync.RWMutex
	deliveries []delivery
}

// New returns a new partitioned webhook receiver.
func New() *Service { return &Service{partitions: map[string]*partition{}} }

func (s *Service) Name() string { return "webhook" }

func (s *Service) Port() int { return Port }

func (s *Service) Stateful() bool { return true }

func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("POST /webhook", s.scoped(s.receive))
	routes.HandleFunc("GET /test/deliveries", s.scoped(s.list))
	routes.HandleFunc("GET /test/event", s.scoped(s.event))
	routes.HandleFunc("POST /test/reset", s.scoped(s.reset))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "webhook: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

func (s *Service) receive(key string, w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDeliveryBodyBytes+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "webhook: reading request body failed", http.StatusBadRequest)
		return
	}
	if len(body) > maxDeliveryBodyBytes {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "webhook: request body is not JSON", http.StatusBadRequest)
		return
	}
	s.record(key, delivery{Headers: r.Header.Clone(), Body: append(json.RawMessage(nil), body...)})
	status := http.StatusOK
	if raw := r.URL.Query().Get("status"); raw == "500" {
		status = http.StatusInternalServerError
	}
	w.WriteHeader(status)
}

func (s *Service) list(key string, w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	items := []delivery{}
	if p != nil {
		p.mu.RLock()
		items = append(items, p.deliveries...)
		p.mu.RUnlock()
	}
	if want := r.URL.Query().Get("eventType"); want != "" {
		filtered := make([]delivery, 0, len(items))
		for _, item := range items {
			var envelope struct {
				EventType string `json:"event_type"`
			}
			if json.Unmarshal(item.Body, &envelope) == nil && envelope.EventType == want {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func (s *Service) event(key string, w http.ResponseWriter, r *http.Request) {
	want := r.URL.Query().Get("eventType")
	wantBody := r.URL.Query().Get("contains")
	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	if p == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i := len(p.deliveries) - 1; i >= 0; i-- {
		var envelope struct {
			EventType string `json:"event_type"`
		}
		if json.Unmarshal(p.deliveries[i].Body, &envelope) == nil && envelope.EventType == want &&
			(wantBody == "" || strings.Contains(string(p.deliveries[i].Body), wantBody)) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(p.deliveries[i].Body)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) reset(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	delete(s.partitions, key)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","service":"webhook"}`))
}

func (s *Service) record(key string, item delivery) {
	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	if p == nil {
		s.mu.Lock()
		p = s.partitions[key]
		if p == nil {
			p = &partition{}
			s.partitions[key] = p
		}
		s.mu.Unlock()
	}
	p.mu.Lock()
	p.deliveries = append(p.deliveries, item)
	if len(p.deliveries) > maxRetainedDeliveries {
		p.deliveries = p.deliveries[len(p.deliveries)-maxRetainedDeliveries:]
	}
	p.mu.Unlock()
}
