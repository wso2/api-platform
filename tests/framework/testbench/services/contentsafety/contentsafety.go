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

// Package contentsafety provides a deterministic Azure AI Content Safety-compatible service.
package contentsafety

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Port is the container port used by the testbench.
const Port = 3005

// AnalyzePath is the Azure text-analysis route.
const AnalyzePath = "/contentsafety/text:analyze"

// SubscriptionKey is the API key accepted by the test service.
const SubscriptionKey = "test-subscription-key"

// keyHeader is Azure's subscription-key header.
const keyHeader = "Ocp-Apim-Subscription-Key"

const maxRequestBodySize = 1 << 20

type analyzeRequest struct {
	Text       string   `json:"text"`
	Categories []string `json:"categories"`
	// HaltOnBlocklistHit is accepted for wire compatibility; scoring is deterministic.
	HaltOnBlocklistHit bool `json:"haltOnBlocklistHit"`
	// OutputType is accepted for wire compatibility; the standard analysis shape is returned.
	OutputType string `json:"outputType"`
}

type categoryAnalysis struct {
	Category string `json:"category"`
	Severity int    `json:"severity"`
}

type analyzeResponse struct {
	CategoriesAnalysis []categoryAnalysis `json:"categoriesAnalysis"`
}

// severityFlagged is the score assigned to a matching keyword.
const severityFlagged = 6

// Service implements testbench.Service.
type Service struct{}

// New returns a new content-safety service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "content-safety" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the analysis and health endpoints.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc(AnalyzePath, s.analyze)
	return mux
}

func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodGet) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{
		"status":  "ok",
		"service": "content-safety",
	})
}

func (s *Service) analyze(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get(keyHeader) != SubscriptionKey {
		w.Header().Set("Content-Type", "application/json")
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid or missing API key",
		})
		return
	}

	var req analyzeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	lowerText := strings.ToLower(req.Text)

	// Preserve the simulated provider-error scenario used by integration tests.
	if strings.Contains(lowerText, "error") && strings.Contains(lowerText, "simulate") {
		http.Error(w, "Simulated Azure Content Safety API error", http.StatusInternalServerError)
		return
	}

	// Preserve the requested category order and emit [] rather than null when empty.
	resp := analyzeResponse{CategoriesAnalysis: make([]categoryAnalysis, 0, len(req.Categories))}
	for _, category := range req.Categories {
		resp.CategoriesAnalysis = append(resp.CategoriesAnalysis, categoryAnalysis{
			Category: category,
			Severity: severity(lowerText, category),
		})
	}

	writeJSON(w, resp)
}

// severity scores one category against already-lowercased text.
func severity(lowerText, category string) int {
	// Matching is intentionally substring-based to preserve the legacy mock's behavior.
	for _, keyword := range keywordsForCategory(category) {
		if strings.Contains(lowerText, keyword) {
			return severityFlagged
		}
	}
	return 0
}

func keywordsForCategory(category string) []string {
	switch category {
	case "Hate":
		return []string{"hate", "racist", "discrimination", "bigot"}
	case "Sexual":
		return []string{"sexual", "explicit", "inappropriate", "nsfw"}
	case "SelfHarm":
		return []string{"self-harm", "suicide", "cutting"}
	case "Violence":
		return []string{"violence", "violent", "kill", "murder", "attack"}
	default:
		return nil
	}
}

func methodOnly(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	decoder := json.NewDecoder(limitedBody)
	err := decoder.Decode(dst)
	if err == nil {
		err = ensureEOF(decoder)
	}
	closeErr := limitedBody.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func ensureEOF(decoder *json.Decoder) error {
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, payload any) {
	writeJSONStatus(w, http.StatusOK, payload)
}

func writeJSONStatus(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("content-safety: failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("content-safety: failed to write response: %v", err)
	}
}
