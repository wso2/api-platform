// Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// AnalyticsEvent represents a Moesif-compatible analytics event
type AnalyticsEvent struct {
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

// RequestDetails represents request information
type RequestDetails struct {
	Time          time.Time         `json:"time"`
	URI           string            `json:"uri"`
	Verb          string            `json:"verb"`
	Headers       map[string]string `json:"headers,omitempty"`
	APIVersion    string            `json:"api_version,omitempty"`
	IPAddress     string            `json:"ip_address,omitempty"`
	Body          interface{}       `json:"body,omitempty"`
	TransferEncoding string         `json:"transfer_encoding,omitempty"`
}

// ResponseDetails represents response information
type ResponseDetails struct {
	Time          time.Time         `json:"time"`
	Status        int               `json:"status"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          interface{}       `json:"body,omitempty"`
	IPAddress     string            `json:"ip_address,omitempty"`
	TransferEncoding string         `json:"transfer_encoding,omitempty"`
}

// gzipMiddleware decompresses gzip-encoded request bodies
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request body is gzip-compressed
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				log.Printf("Error creating gzip reader: %v", err)
				http.Error(w, "Failed to decompress gzip body", http.StatusBadRequest)
				return
			}
			defer gzipReader.Close()
			
			// Replace the request body with the decompressed version
			r.Body = io.NopCloser(gzipReader)
			// Remove the Content-Encoding header since we've decompressed
			r.Header.Del("Content-Encoding")
		}
		next.ServeHTTP(w, r)
	})
}

// MockCollector stores analytics events in memory
type MockCollector struct {
	mu     sync.RWMutex
	events []AnalyticsEvent
}

func NewMockCollector() *MockCollector {
	return &MockCollector{
		events: make([]AnalyticsEvent, 0),
	}
}

// HandleEvent handles single event submission (POST /v1/events)
func (mc *MockCollector) HandleEvent(w http.ResponseWriter, r *http.Request) {
	var event AnalyticsEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	mc.mu.Lock()
	mc.events = append(mc.events, event)
	mc.mu.Unlock()

	log.Printf("Received single event: %s %s -> %d", event.Request.Verb, event.Request.URI, event.Response.Status)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleEventBatch handles batch event submission (POST /v1/events/batch)
func (mc *MockCollector) HandleEventBatch(w http.ResponseWriter, r *http.Request) {
	var events []AnalyticsEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	mc.mu.Lock()
	mc.events = append(mc.events, events...)
	mc.mu.Unlock()

	log.Printf("Received batch of %d events", len(events))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(events),
	})
}

// GetEvents returns all stored events (GET /test/events)
func (mc *MockCollector) GetEvents(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mc.events)
}

// GetEventCount returns the count of stored events (GET /test/events/count)
func (mc *MockCollector) GetEventCount(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": len(mc.events)})
}

// ResetEvents clears all stored events (POST /test/reset)
func (mc *MockCollector) ResetEvents(w http.ResponseWriter, r *http.Request) {
	mc.mu.Lock()
	mc.events = make([]AnalyticsEvent, 0)
	mc.mu.Unlock()

	log.Println("Reset all events")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

// HealthCheck returns service status (GET /test/health)
func (mc *MockCollector) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"service": "mock-analytics-collector",
	})
}

func main() {
	collector := NewMockCollector()
	r := mux.NewRouter()
	
	// Apply gzip middleware to all routes
	r.Use(gzipMiddleware)

	// Moesif-compatible endpoints
	r.HandleFunc("/v1/events", collector.HandleEvent).Methods("POST")
	r.HandleFunc("/v1/events/batch", collector.HandleEventBatch).Methods("POST")

	// Test helper endpoints
	r.HandleFunc("/test/events", collector.GetEvents).Methods("GET")
	r.HandleFunc("/test/events/count", collector.GetEventCount).Methods("GET")
	r.HandleFunc("/test/reset", collector.ResetEvents).Methods("POST")
	r.HandleFunc("/test/health", collector.HealthCheck).Methods("GET")

	log.Println("Mock Analytics Collector starting on :8080")
	log.Println("Moesif API endpoints:")
	log.Println("  POST /v1/events - Submit single event")
	log.Println("  POST /v1/events/batch - Submit batch of events")
	log.Println("Test endpoints:")
	log.Println("  GET /test/events - Get all events")
	log.Println("  GET /test/events/count - Get event count")
	log.Println("  POST /test/reset - Clear all events")
	log.Println("  GET /test/health - Health check")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
