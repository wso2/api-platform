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

package contentsafety

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Port is the container port used by the testbench.
const Port = 3005

// AnalyzePath is the Azure text-analysis route.
const AnalyzePath = "/contentsafety/text:analyze"

// SubscriptionKey is the API key accepted by the test service.
const SubscriptionKey = "test-subscription-key"

// APIVersion is the only api-version query value the service accepts.
const APIVersion = "2024-09-01"

// MaxTextLength is the most characters of text Azure analyses in one request.
const MaxTextLength = 10000

// Azure's output types; FourSeverityLevels is the default.
const (
	outputFourSeverityLevels  = "FourSeverityLevels"
	outputEightSeverityLevels = "EightSeverityLevels"
)

// categories are Azure's text categories, in Azure's order.
var categories = []string{"Hate", "SelfHarm", "Sexual", "Violence"}

// severityMarker matches "<category>-severity-<n>" in lowercased text.
var severityMarker = regexp.MustCompile(`\b(hate|sexual|selfharm|violence)-severity-([0-7])\b`)

// simulatedFailure is a fixed answer for text that asks for one, so a test can make the service
// fail in a way the policy must handle.
type simulatedFailure struct {
	trigger string
	status  int
	body    string
}

// simulatedFailures are checked in order against the lowercased text. None contains "error", so
// they never overlap the simulated 500 below.
var simulatedFailures = []simulatedFailure{
	{"simulate malformed json", http.StatusOK, `{"categoriesAnalysis":[{"category":"Hate",`},
	{"simulate missing analysis", http.StatusOK, `{"blocklistsMatch":[]}`},
	{"simulate invalid analysis", http.StatusOK, `{"categoriesAnalysis":"Violence"}`},
	{"simulate bad request", http.StatusBadRequest,
		`{"error":{"code":"InvalidRequestBody","message":"Simulated invalid request."}}`},
	{"simulate forbidden", http.StatusForbidden,
		`{"error":{"code":"Forbidden","message":"Simulated access denied."}}`},
}

// simulatedFailureFor returns the simulated failure the text asks for, if any.
func simulatedFailureFor(lowerText string) (simulatedFailure, bool) {
	for _, failure := range simulatedFailures {
		if strings.Contains(lowerText, failure.trigger) {
			return failure, true
		}
	}
	return simulatedFailure{}, false
}

// mixedAnalysisTrigger asks for a categoriesAnalysis whose real entries follow nonObjectEntries.
const mixedAnalysisTrigger = "simulate mixed analysis"

// nonObjectEntries are categoriesAnalysis entries that are not objects; each would block the
// request if a client read it as a Violence analysis of severity 7.
var nonObjectEntries = []any{"Violence", 7, nil, true, []any{"Violence", 7}}

// keyHeader is Azure's subscription-key header.
const keyHeader = "Ocp-Apim-Subscription-Key"

const maxRequestBodySize = 1 << 20

type analyzeRequest struct {
	Text       string   `json:"text"`
	Categories []string `json:"categories"`
	// HaltOnBlocklistHit is accepted for wire compatibility; no blocklists exist.
	HaltOnBlocklistHit bool   `json:"haltOnBlocklistHit"`
	OutputType         string `json:"outputType"`
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
			"error":   "unauthorized",
			"message": "Invalid or expired credentials.",
		})
		return
	}

	if version := r.URL.Query().Get("api-version"); version != APIVersion {
		writeInvalidRequestBody(w, fmt.Sprintf("api-version %q is not supported", version))
		return
	}

	var req analyzeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondToDecodeError(w, err)
		return
	}
	if n := utf8.RuneCountInString(req.Text); n > MaxTextLength {
		writeInvalidRequestBody(w, fmt.Sprintf("text has %d characters, more than the %d allowed", n, MaxTextLength))
		return
	}
	requested, err := requestedCategories(req.Categories)
	if err != nil {
		writeInvalidRequestBody(w, err.Error())
		return
	}
	eightLevels, err := isEightSeverityLevels(req.OutputType)
	if err != nil {
		writeInvalidRequestBody(w, err.Error())
		return
	}

	lowerText := strings.ToLower(req.Text)

	if failure, ok := simulatedFailureFor(lowerText); ok {
		w.WriteHeader(failure.status)
		if _, err := w.Write([]byte(failure.body)); err != nil {
			log.Printf("content-safety: failed to write simulated failure: %v", err)
		}
		return
	}

	// Preserve the simulated provider-error scenario used by integration tests.
	if strings.Contains(lowerText, "error") && strings.Contains(lowerText, "simulate") {
		http.Error(w, "Simulated Azure Content Safety API error", http.StatusInternalServerError)
		return
	}

	// Preserve the requested category order.
	resp := analyzeResponse{CategoriesAnalysis: make([]categoryAnalysis, 0, len(requested))}
	for _, category := range requested {
		score := severity(lowerText, category)
		if !eightLevels {
			score -= score % 2
		}
		resp.CategoriesAnalysis = append(resp.CategoriesAnalysis, categoryAnalysis{
			Category: category,
			Severity: score,
		})
	}

	if strings.Contains(lowerText, mixedAnalysisTrigger) {
		entries := append([]any(nil), nonObjectEntries...)
		for _, analysis := range resp.CategoriesAnalysis {
			entries = append(entries, analysis)
		}
		writeJSON(w, map[string]any{"categoriesAnalysis": entries})
		return
	}
	writeJSON(w, resp)
}

// requestedCategories validates the requested categories; none requested means all of them.
func requestedCategories(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return categories, nil
	}
	for i, category := range requested {
		if !slices.Contains(categories, category) {
			return nil, fmt.Errorf("category %q is not supported", category)
		}
		if slices.Contains(requested[:i], category) {
			return nil, fmt.Errorf("category %q is requested more than once", category)
		}
	}
	return requested, nil
}

// isEightSeverityLevels validates outputType and reports whether it asks for eight levels.
func isEightSeverityLevels(outputType string) (bool, error) {
	switch outputType {
	case "", outputFourSeverityLevels:
		return false, nil
	case outputEightSeverityLevels:
		return true, nil
	default:
		return false, fmt.Errorf("outputType %q is not supported", outputType)
	}
}

// severity scores one category against already-lowercased text on the eight-level scale.
func severity(lowerText, category string) int {
	for _, marker := range severityMarker.FindAllStringSubmatch(lowerText, -1) {
		if marker[1] == strings.ToLower(category) {
			score, _ := strconv.Atoi(marker[2])
			return score
		}
	}
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
	if strings.ToUpper(r.Method) == method {
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

// writeInvalidRequestBody rejects a request that does not match the API, as Azure does.
func writeInvalidRequestBody(w http.ResponseWriter, message string) {
	writeJSONStatus(w, http.StatusBadRequest, map[string]any{
		"error": map[string]string{"code": "InvalidRequestBody", "message": message},
	})
}

// respondToDecodeError reports an oversized body as 413, distinct from other
// malformed-JSON failures, which report as 400.
func respondToDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "Invalid JSON request", http.StatusBadRequest)
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
