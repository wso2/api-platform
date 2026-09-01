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

// Package analytics provides a partitioned Moesif-compatible analytics collector
// for integration tests.
package analytics

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3007

// maxBodyBytes bounds the decompressed size of one ingest request.
const maxBodyBytes = 8 << 20

// maxPartitionKeyLen bounds the block key path segment.
const maxPartitionKeyLen = 64

// Event is the Moesif-shaped event buffered by the collector.
type Event struct {
	Request      RequestDetails  `json:"request"`
	Response     ResponseDetails `json:"response"`
	UserID       string          `json:"user_id,omitempty"`
	CompanyID    string          `json:"company_id,omitempty"`
	SessionToken string          `json:"session_token,omitempty"`
	Metadata     interface{}     `json:"metadata,omitempty"`
	Direction    string          `json:"direction,omitempty"`
	Weight       int             `json:"weight,omitempty"`
	Tags         string          `json:"tags,omitempty"`
}

// RequestDetails is the request half of an event.
type RequestDetails struct {
	Time             time.Time         `json:"time"`
	URI              string            `json:"uri"`
	Verb             string            `json:"verb"`
	Headers          map[string]string `json:"headers,omitempty"`
	APIVersion       string            `json:"api_version,omitempty"`
	IPAddress        string            `json:"ip_address,omitempty"`
	Body             interface{}       `json:"body,omitempty"`
	TransferEncoding string            `json:"transfer_encoding,omitempty"`
}

// ResponseDetails is the response half of an event.
type ResponseDetails struct {
	Time             time.Time         `json:"time"`
	Status           int               `json:"status"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             interface{}       `json:"body,omitempty"`
	IPAddress        string            `json:"ip_address,omitempty"`
	TransferEncoding string            `json:"transfer_encoding,omitempty"`
}

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	// mu guards the partition map.
	mu sync.RWMutex

	partitions map[string]*partition
}

type partition struct {
	mu     sync.RWMutex
	events []Event
}

// New returns a new analytics service.
func New() *Service { return &Service{partitions: map[string]*partition{}} }

// Name returns the service registration name.
func (s *Service) Name() string { return "analytics" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("POST /v1/events", s.scoped(s.ingestOne))
	routes.HandleFunc("POST /v1/events/batch", s.scoped(s.ingestBatch))
	routes.HandleFunc("GET /test/events", s.scoped(s.readEvents))
	routes.HandleFunc("GET /test/events/count", s.scoped(s.readCount))
	routes.HandleFunc("POST /test/reset", s.scoped(s.reset))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))

	return boundedGzipBodies(partitionRouter(routes))
}

// partitionCtxKey carries the validated block key from the router to the handler.
type partitionCtxKey struct{}

// partitionRouter validates and removes the leading block-key path segment.
func partitionRouter(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, rest, err := splitPartition(r.URL.Path)
		if err != nil {
			http.Error(w, "analytics collector: "+err.Error(), http.StatusBadRequest)
			return
		}

		scoped := r.Clone(context.WithValue(r.Context(), partitionCtxKey{}, key))
		scoped.URL.Path = rest
		scoped.URL.RawPath = ""
		inner.ServeHTTP(w, scoped)
	})
}

// splitPartition returns the block key and the remaining route path.
func splitPartition(path string) (key, rest string, err error) {
	trimmed := strings.TrimPrefix(path, "/")
	segment, remainder, found := strings.Cut(trimmed, "/")
	if segment == "" {
		return "", "", fmt.Errorf("every route is partitioned by block, so a request needs a "+
			"leading /<block> segment; got %q", path)
	}
	if isReservedPartitionKey(segment) {
		return "", "", fmt.Errorf("%q is a route root, not a block key: this collector partitions "+
			"by block, so the path is /<block>%s — a caller configured with the unpartitioned "+
			"base URL lands here", segment, path)
	}
	if len(segment) > maxPartitionKeyLen {
		return "", "", fmt.Errorf("block key %q is longer than %d characters, so it is not a block "+
			"key", segment, maxPartitionKeyLen)
	}
	for _, c := range segment {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return "", "", fmt.Errorf("block key %q contains %q; a block key is lowercase letters, "+
				"digits and dashes only", segment, string(c))
		}
	}
	if !found {
		return segment, "/", nil
	}
	return segment, "/" + remainder, nil
}

func isReservedPartitionKey(key string) bool {
	switch key {
	case "v1", "test", "testbench":
		return true
	default:
		return false
	}
}

func (s *Service) getPartition(key string, create bool) *partition {
	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	if p != nil || !create {
		return p
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if p = s.partitions[key]; p == nil {
		p = &partition{}
		s.partitions[key] = p
	}
	return p
}

func (s *Service) events(key string) []Event {
	p := s.getPartition(key, false)
	if p == nil {
		return []Event{}
	}
	p.mu.RLock()
	events := append([]Event(nil), p.events...)
	p.mu.RUnlock()
	return events
}

func (s *Service) count(key string) int {
	p := s.getPartition(key, false)
	if p == nil {
		return 0
	}
	p.mu.RLock()
	count := len(p.events)
	p.mu.RUnlock()
	return count
}

// scoped adapts a partition-aware handler to http.HandlerFunc.
func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := r.Context().Value(partitionCtxKey{}).(string)
		if !ok || key == "" {
			http.Error(w, "analytics collector: internal error: a request reached a handler "+
				"with no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// ingestOne is POST /<block>/v1/events.
func (s *Service) ingestOne(key string, w http.ResponseWriter, r *http.Request) {
	var event Event
	if err := decodeJSONBody(r, &event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	p := s.getPartition(key, true)
	p.mu.Lock()
	p.events = append(p.events, event)
	p.mu.Unlock()

	log.Printf("analytics[%s]: received single event: %s %s -> %d",
		key, event.Request.Verb, event.Request.URI, event.Response.Status)

	// Keep the ingest response compatible with the publisher.
	writeJSONStatus(w, http.StatusCreated, map[string]string{"status": "success"})
}

// ingestBatch is POST /<block>/v1/events/batch.
func (s *Service) ingestBatch(key string, w http.ResponseWriter, r *http.Request) {
	var events []Event
	if err := decodeJSONBody(r, &events); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	p := s.getPartition(key, true)
	p.mu.Lock()
	p.events = append(p.events, events...)
	p.mu.Unlock()

	log.Printf("analytics[%s]: received batch of %d events", key, len(events))

	writeJSONStatus(w, http.StatusCreated, map[string]interface{}{
		"status": "success",
		"count":  len(events),
	})
}

// readEvents is GET /<block>/test/events.
func (s *Service) readEvents(key string, w http.ResponseWriter, _ *http.Request) {
	events := s.events(key)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, events)
}

// readCount is GET /<block>/test/events/count.
func (s *Service) readCount(key string, w http.ResponseWriter, _ *http.Request) {
	count := s.count(key)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]int{"count": count})
}

// reset clears the events for one block.
func (s *Service) reset(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	delete(s.partitions, key)
	s.mu.Unlock()

	log.Printf("analytics[%s]: reset all events", key)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"status": "reset"})
}

// health returns the partitioned service health status.
func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{
		"status":  "ok",
		"service": "analytics",
	})
}

// boundedGzipBodies decompresses gzip requests and limits the readable body size.
func boundedGzipBodies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				log.Printf("analytics: error creating gzip reader: %v", err)
				http.Error(w, "Failed to decompress gzip body", http.StatusBadRequest)
				return
			}
			limited := http.MaxBytesReader(w, zr, maxBodyBytes)
			r.Body = &bodyReadCloser{
				Reader:  limited,
				closers: []io.Closer{limited, r.Body},
			}
			r.Header.Del("Content-Encoding")
		} else {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

type bodyReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (b *bodyReadCloser) Close() error {
	var firstErr error
	for _, closer := range b.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func decodeJSONBody(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		_ = r.Body.Close()
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		_ = r.Body.Close()
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return r.Body.Close()
}

func writeJSON(w http.ResponseWriter, payload any) {
	writeJSONStatus(w, http.StatusOK, payload)
}

func writeJSONStatus(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("analytics: failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("analytics: failed to write response: %v", err)
	}
}
