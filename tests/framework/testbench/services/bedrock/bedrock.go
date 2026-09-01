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

// Package bedrock provides a deterministic AWS Bedrock ApplyGuardrail-compatible service.
package bedrock

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Port is the container port used by the testbench.
const Port = 3006

// Service implements the Bedrock testbench service.
type Service struct{}

// New returns a new Bedrock service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "bedrock" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// applyGuardrailRequest is the subset of the ApplyGuardrail request the decisions read.
type applyGuardrailRequest struct {
	Source  string `json:"source"`
	Content []struct {
		Text *struct {
			Text string `json:"text"`
		} `json:"text,omitempty"`
	} `json:"content"`
}

type applyGuardrailResponse struct {
	Action      string           `json:"action"`
	Outputs     []map[string]any `json:"outputs"`
	Assessments []assessment     `json:"assessments"`
}

type assessment struct {
	ContentPolicy              *contentPolicyAssessment              `json:"contentPolicy,omitempty"`
	SensitiveInformationPolicy *sensitiveInformationPolicyAssessment `json:"sensitiveInformationPolicy,omitempty"`
}

type contentPolicyAssessment struct {
	Filters []contentFilter `json:"filters"`
}

type contentFilter struct {
	Type       string `json:"type"`
	Confidence string `json:"confidence"`
	Action     string `json:"action"`
}

type sensitiveInformationPolicyAssessment struct {
	PiiEntities []piiEntity `json:"piiEntities,omitempty"`
}

type piiEntity struct {
	Type   string `json:"type"`
	Match  string `json:"match"`
	Action string `json:"action"`
}

var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	// The SDK supplies the guardrail ID and version in the request path.
	guardrailPathRegex = regexp.MustCompile(`^/guardrail/[^/]+/version/[^/]+/apply$`)
)

// Handler returns the ApplyGuardrail endpoint.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !guardrailPathRegex.MatchString(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.apply(w, r)
	})
	return mux
}

func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{
		"status":  "ok",
		"service": "bedrock",
	})
}

// apply returns a deterministic guardrail decision for the supplied text.
func (s *Service) apply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req applyGuardrailRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	var content []string
	for _, item := range req.Content {
		if item.Text != nil {
			content = append(content, item.Text.Text)
		}
	}
	text := strings.Join(content, " ")
	if text == "" {
		writeJSON(w, none())
		return
	}

	lower := strings.ToLower(text)

	// Preserve the simulated provider-error scenario used by integration tests.
	if strings.Contains(lower, "error") && strings.Contains(lower, "simulate") {
		http.Error(w, "Simulated AWS Bedrock Guardrail error", http.StatusInternalServerError)
		return
	}

	// Keyword matching intentionally uses substrings to preserve the legacy mock's behavior.
	if strings.Contains(lower, "violence") || strings.Contains(lower, "hate") ||
		strings.Contains(lower, "illegal") {
		writeJSON(w, applyGuardrailResponse{
			Action:  "GUARDRAIL_INTERVENED",
			Outputs: []map[string]any{{"text": "Content blocked due to policy violation"}},
			Assessments: []assessment{{
				ContentPolicy: &contentPolicyAssessment{
					Filters: []contentFilter{{Type: "VIOLENCE", Confidence: "HIGH", Action: "BLOCKED"}},
				},
			}},
		})
		return
	}

	// Check masking before redaction because both match example.com addresses.
	if strings.Contains(lower, "mask-") && strings.Contains(lower, "@example.com") {
		if emails := emailRegex.FindAllString(text, -1); len(emails) > 0 {
			writeJSON(w, piiIntervened(replaceEmails(text, emails, "$ANONYMIZED_EMAIL$"), emails))
			return
		}
	}

	if strings.Contains(lower, "@example.com") {
		if emails := emailRegex.FindAllString(text, -1); len(emails) > 0 {
			writeJSON(w, piiIntervened(replaceEmails(text, emails, "*****"), emails))
			return
		}
	}

	writeJSON(w, none())
}

func none() applyGuardrailResponse {
	// Keep empty collections non-nil so they marshal as [] rather than null.
	return applyGuardrailResponse{
		Action:      "NONE",
		Outputs:     []map[string]any{},
		Assessments: []assessment{},
	}
}

func piiIntervened(outputText string, emails []string) applyGuardrailResponse {
	entities := make([]piiEntity, 0, len(emails))
	for _, email := range emails {
		entities = append(entities, piiEntity{Type: "EMAIL", Match: email, Action: "ANONYMIZED"})
	}
	return applyGuardrailResponse{
		Action:  "GUARDRAIL_INTERVENED",
		Outputs: []map[string]any{{"text": outputText}},
		Assessments: []assessment{{
			SensitiveInformationPolicy: &sensitiveInformationPolicyAssessment{
				PiiEntities: entities,
			},
		}},
	}
}

func replaceEmails(text string, emails []string, replacement string) string {
	for _, email := range emails {
		text = strings.ReplaceAll(text, email, replacement)
	}
	return text
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}
