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

package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3015

// APIKey is the only key the service accepts, as "Authorization: Bearer <APIKey>".
const APIKey = "test-jev-key"

// SystemOnePath is the Jev endpoint the policies call, appended to their base URL.
const SystemOnePath = "/v1/systemone"

// Default answers for questions a request carries no marker for.
const (
	// DefaultNoul is the probability a noul question answers.
	DefaultNoul = 0.05
	// DefaultScore is the scale position a score question answers.
	DefaultScore = 0.0
	// DefaultConfidence is the confidence reported with every score and choice answer.
	DefaultConfidence = 0.9
)

// Modes, the path segment after the partition.
const (
	// ModeOK answers every question.
	ModeOK = "ok"
	// ModeError answers 500.
	ModeError = "error"
	// ModeSlow answers after the slow delay, so a short policy timeout expires first.
	ModeSlow = "slow"
	// ModeRateLimitOnce answers the partition's first request with 429 and every later one normally.
	ModeRateLimitOnce = "ratelimit-once"
	// ModeOverloadedOnce answers the partition's first request with 529 and every later one normally.
	ModeOverloadedOnce = "overloaded-once"
	// ModeRateLimitAlways answers every request with 429.
	ModeRateLimitAlways = "ratelimit-always"
	// ModeInvalidJSON answers 200 with a body that is not JSON.
	ModeInvalidJSON = "invalid-json"
	// ModeMissingAnswer answers every question except the first, in key order.
	ModeMissingAnswer = "missing-answer"
)

// StatusOverloaded is the status Jev uses when it is overloaded.
const StatusOverloaded = 529

// defaultSlowDelay is longer than the one-second timeouts scenarios set, and well inside the
// testbench's 30-second write timeout.
const defaultSlowDelay = 3 * time.Second

// maxBodyBytes bounds one Jev request.
const maxBodyBytes = 1 << 20

// maxPartitions bounds how many partitions the service holds.
const maxPartitions = 4096

// maxRetainedRequests bounds the requests one partition keeps; the count stays exact beyond it.
const maxRetainedRequests = 256

// markerPattern matches "jev:<question key>=<value>" in the text Jev is asked about.
var markerPattern = regexp.MustCompile(`jev:([A-Za-z0-9_.-]+)=([A-Za-z0-9_.:@,-]+)`)

var modes = map[string]bool{
	ModeOK: true, ModeError: true, ModeSlow: true, ModeRateLimitOnce: true, ModeOverloadedOnce: true,
	ModeRateLimitAlways: true, ModeInvalidJSON: true, ModeMissingAnswer: true,
}

// Request is one recorded call to the System One endpoint.
type Request struct {
	Mode          string          `json:"mode"`
	Authorization string          `json:"authorization"`
	Model         string          `json:"model"`
	State         json.RawMessage `json:"state"`
	QuestionKeys  []string        `json:"questionKeys"`
	Questions     json.RawMessage `json:"questions"`
}

type partition struct {
	count    int
	requests []Request
}

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	mu         sync.Mutex
	partitions map[string]*partition
	slowDelay  time.Duration
}

// New returns a new Jev service.
func New() *Service { return newWithSlowDelay(defaultSlowDelay) }

func newWithSlowDelay(delay time.Duration) *Service {
	return &Service{partitions: map[string]*partition{}, slowDelay: delay}
}

// Name returns the service registration name.
func (s *Service) Name() string { return "jev" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service retains the requests it receives.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /test/requests", s.scoped(s.recorded))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	routes.HandleFunc("POST /", s.scoped(s.systemOne))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "jev: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

type systemOneRequest struct {
	State     json.RawMessage            `json:"state"`
	Model     string                     `json:"model"`
	Questions map[string]json.RawMessage `json:"questions"`
}

type question struct {
	Type     string          `json:"type"`
	Criteria json.RawMessage `json:"criteria"`
}

// systemOne serves POST /<mode>/v1/systemone.
func (s *Service) systemOne(key string, w http.ResponseWriter, r *http.Request) {
	mode, ok := strings.CutSuffix(r.URL.Path, SystemOnePath)
	mode = strings.TrimPrefix(mode, "/")
	if !ok || strings.Contains(mode, "/") || !modes[mode] {
		http.Error(w, fmt.Sprintf("jev: expected POST /<mode>%s with a known mode", SystemOnePath), http.StatusNotFound)
		return
	}

	req, err := decodeRequest(w, r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "jev: request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "jev: "+err.Error(), http.StatusBadRequest)
		return
	}
	questionsJSON, _ := json.Marshal(req.Questions)
	position, recorded := s.record(key, Request{
		Mode:          mode,
		Authorization: r.Header.Get("Authorization"),
		Model:         req.Model,
		State:         req.State,
		QuestionKeys:  sortedKeys(req.Questions),
		Questions:     questionsJSON,
	})
	if !recorded {
		http.Error(w, "jev: partition budget exhausted", http.StatusInsufficientStorage)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+APIKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}

	switch mode {
	case ModeError:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "simulated Jev failure"})
		return
	case ModeRateLimitAlways:
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
		return
	case ModeRateLimitOnce:
		if position == 1 {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
			return
		}
	case ModeOverloadedOnce:
		if position == 1 {
			writeJSON(w, StatusOverloaded, map[string]string{"error": "overloaded"})
			return
		}
	case ModeInvalidJSON:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not JSON"))
		return
	case ModeSlow:
		if !wait(r.Context(), s.slowDelay) {
			return
		}
	}

	answers, err := answer(req, markers(req.State))
	if err != nil {
		http.Error(w, "jev: "+err.Error(), http.StatusBadRequest)
		return
	}
	if mode == ModeMissingAnswer {
		if keys := sortedKeys(req.Questions); len(keys) > 0 {
			delete(answers, keys[0])
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"answers": answers,
		"usage":   map[string]int{"input_tokens": len(req.State), "output_tokens": len(req.Questions)},
	})
}

func decodeRequest(w http.ResponseWriter, r *http.Request) (systemOneRequest, error) {
	var req systemOneRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return req, fmt.Errorf("invalid JSON request: %w", err)
	}
	if len(req.State) == 0 || string(req.State) == "null" {
		return req, fmt.Errorf("'state' is required")
	}
	if len(req.Questions) == 0 {
		return req, fmt.Errorf("'questions' must name at least one question")
	}
	return req, nil
}

// record stores a request and returns its 1-based position in the partition.
func (s *Service) record(key string, req Request) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.partitions[key]
	if p == nil {
		if len(s.partitions) >= maxPartitions {
			return 0, false
		}
		p = &partition{}
		s.partitions[key] = p
	}
	p.count++
	if len(p.requests) < maxRetainedRequests {
		p.requests = append(p.requests, req)
	}
	return p.count, true
}

// recorded reports GET /test/requests as {"count": n, "requests": [...]}, oldest first.
func (s *Service) recorded(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	count, requests := 0, []Request{}
	if p := s.partitions[key]; p != nil {
		count = p.count
		requests = append(requests, p.requests...)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"count": count, "requests": requests})
}

func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "jev"})
}

// markers returns the "jev:<key>=<value>" markers in the state, last one winning.
func markers(state json.RawMessage) map[string]string {
	text := string(state)
	var s string
	if json.Unmarshal(state, &s) == nil {
		text = s
	}
	found := map[string]string{}
	for _, m := range markerPattern.FindAllStringSubmatch(text, -1) {
		found[m[1]] = m[2]
	}
	return found
}

// answer builds an answer for every question, from its marker or the type's default.
func answer(req systemOneRequest, marked map[string]string) (map[string]any, error) {
	answers := make(map[string]any, len(req.Questions))
	for key, raw := range req.Questions {
		var q question
		if err := json.Unmarshal(raw, &q); err != nil {
			return nil, fmt.Errorf("question %q is not an object", key)
		}
		marker, hasMarker := marked[key]
		var (
			a   any
			err error
		)
		switch q.Type {
		case "noul":
			a, err = noulAnswer(marker, hasMarker)
		case "score":
			a, err = scoreAnswer(marker, hasMarker)
		case "choice":
			a, err = choiceAnswer(q.Criteria, marker, hasMarker)
		default:
			err = fmt.Errorf("unsupported type %q", q.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("question %q: %w", key, err)
		}
		answers[key] = a
	}
	return answers, nil
}

func noulAnswer(marker string, hasMarker bool) (map[string]any, error) {
	value := DefaultNoul
	if hasMarker {
		v, err := strconv.ParseFloat(marker, 64)
		if err != nil {
			return nil, fmt.Errorf("noul marker %q is not a number", marker)
		}
		value = v
	}
	return map[string]any{"type": "noul", "noul": value}, nil
}

// scoreAnswer reads "<score>" or "<score>@<confidence>".
func scoreAnswer(marker string, hasMarker bool) (map[string]any, error) {
	score, confidence := DefaultScore, DefaultConfidence
	if hasMarker {
		scoreText, confidenceText, withConfidence := strings.Cut(marker, "@")
		v, err := strconv.ParseFloat(scoreText, 64)
		if err != nil {
			return nil, fmt.Errorf("score marker %q is not <score> or <score>@<confidence>", marker)
		}
		score = v
		if withConfidence {
			c, err := strconv.ParseFloat(confidenceText, 64)
			if err != nil {
				return nil, fmt.Errorf("score marker %q has an invalid confidence", marker)
			}
			confidence = c
		}
	}
	return map[string]any{"type": "score", "score": score, "confidence": confidence}, nil
}

// choiceAnswer reads "<option>@<probability>", which spreads the rest evenly over the other
// options, or "<option>:<p>,<option>:<p>,...". Without a marker every option is equally likely.
// The choice is the most probable option, ties going to the first in key order.
func choiceAnswer(criteria json.RawMessage, marker string, hasMarker bool) (map[string]any, error) {
	var options map[string]any
	if err := json.Unmarshal(criteria, &options); err != nil || len(options) == 0 {
		return nil, fmt.Errorf("choice criteria must be an object of options")
	}
	names := sortedKeys(options)
	probabilities := map[string]float64{}
	switch {
	case !hasMarker:
		for _, name := range names {
			probabilities[name] = 1 / float64(len(names))
		}
	case strings.Contains(marker, ":"):
		for _, pair := range strings.Split(marker, ",") {
			name, text, ok := strings.Cut(pair, ":")
			p, err := strconv.ParseFloat(text, 64)
			if !ok || name == "" || err != nil {
				return nil, fmt.Errorf("choice marker %q is not <option>:<p>,<option>:<p>", marker)
			}
			probabilities[name] = p
		}
	default:
		chosen, text, ok := strings.Cut(marker, "@")
		p, err := strconv.ParseFloat(text, 64)
		if !ok || chosen == "" || err != nil {
			return nil, fmt.Errorf("choice marker %q is not <option>@<probability>", marker)
		}
		probabilities[chosen] = p
		var others []string
		for _, name := range names {
			if name != chosen {
				others = append(others, name)
			}
		}
		for _, name := range others {
			probabilities[name] = (1 - p) / float64(len(others))
		}
	}
	choice := ""
	for _, name := range sortedKeys(probabilities) {
		if choice == "" || probabilities[name] > probabilities[choice] {
			choice = name
		}
	}
	return map[string]any{
		"type": "choice", "choice": choice, "probabilities": probabilities, "confidence": DefaultConfidence,
	}, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// wait blocks for delay unless the request ends first, reporting whether the delay elapsed.
func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("jev: failed to encode response: %v", err)
	}
}
