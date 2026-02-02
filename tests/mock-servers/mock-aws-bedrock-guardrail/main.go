// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// ApplyGuardrailRequest represents the AWS Bedrock ApplyGuardrail request format
type ApplyGuardrailRequest struct {
	Source  string                  `json:"source"`
	Content []GuardrailContentBlock `json:"content"`
}

type GuardrailContentBlock struct {
	Text *TextBlock `json:"text,omitempty"`
}

type TextBlock struct {
	Text string `json:"text"`
}

// ApplyGuardrailResponse represents the AWS Bedrock ApplyGuardrail response format
type ApplyGuardrailResponse struct {
	Action      string                   `json:"action"`
	Outputs     []map[string]interface{} `json:"outputs"`
	Assessments []Assessment             `json:"assessments"`
}

type Assessment struct {
	ContentPolicy              *ContentPolicyAssessment              `json:"contentPolicy,omitempty"`
	TopicPolicy                *TopicPolicyAssessment                `json:"topicPolicy,omitempty"`
	WordPolicy                 *WordPolicyAssessment                 `json:"wordPolicy,omitempty"`
	SensitiveInformationPolicy *SensitiveInformationPolicyAssessment `json:"sensitiveInformationPolicy,omitempty"`
}

type ContentPolicyAssessment struct {
	Filters []ContentFilter `json:"filters"`
}

type ContentFilter struct {
	Type       string `json:"type"`
	Confidence string `json:"confidence"`
	Action     string `json:"action"`
}

type TopicPolicyAssessment struct {
	Topics []Topic `json:"topics"`
}

type Topic struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Action string `json:"action"`
}

type WordPolicyAssessment struct {
	CustomWords      []CustomWord      `json:"customWords,omitempty"`
	ManagedWordLists []ManagedWordList `json:"managedWordLists,omitempty"`
}

type CustomWord struct {
	Match  string `json:"match"`
	Action string `json:"action"`
}

type ManagedWordList struct {
	Match  string `json:"match"`
	Type   string `json:"type"`
	Action string `json:"action"`
}

type SensitiveInformationPolicyAssessment struct {
	PiiEntities []PiiEntity `json:"piiEntities,omitempty"`
	Regexes     []Regex     `json:"regexes,omitempty"`
}

type PiiEntity struct {
	Type   string `json:"type"`
	Match  string `json:"match"`
	Action string `json:"action"`
}

type Regex struct {
	Name   string `json:"name"`
	Match  string `json:"match"`
	Action string `json:"action"`
}

// Email regex pattern
var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

// Guardrail apply endpoint pattern: /guardrail/{guardrailId}/version/{guardrailVersion}/apply
var guardrailPathRegex = regexp.MustCompile(`^/guardrail/[^/]+/version/[^/]+/apply$`)

func handleApplyGuardrail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Log request for debugging
	log.Printf("Mock AWS Bedrock Guardrail: %s %s", r.Method, r.URL.Path)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req ApplyGuardrailRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Mock AWS Bedrock Guardrail: Invalid JSON: %s", string(body))
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// Extract text content
	var text string
	if len(req.Content) > 0 && req.Content[0].Text != nil {
		text = req.Content[0].Text.Text
	}

	// Check for empty content
	if text == "" {
		response := ApplyGuardrailResponse{
			Action:      "NONE",
			Outputs:     []map[string]interface{}{},
			Assessments: []Assessment{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	lowerText := strings.ToLower(text)

	// Check for error simulation keyword
	if strings.Contains(lowerText, "error") && strings.Contains(lowerText, "simulate") {
		http.Error(w, "Simulated AWS Bedrock Guardrail error", http.StatusInternalServerError)
		return
	}

	// Check for violation keywords (violence, hate, illegal)
	if strings.Contains(lowerText, "violence") ||
		strings.Contains(lowerText, "hate") ||
		strings.Contains(lowerText, "illegal") {
		response := ApplyGuardrailResponse{
			Action: "GUARDRAIL_INTERVENED",
			Outputs: []map[string]interface{}{
				{"text": "Content blocked due to policy violation"},
			},
			Assessments: []Assessment{
				{
					ContentPolicy: &ContentPolicyAssessment{
						Filters: []ContentFilter{
							{
								Type:       "VIOLENCE",
								Confidence: "HIGH",
								Action:     "BLOCKED",
							},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check for PII with masking (mask- prefix in email)
	if strings.Contains(lowerText, "mask-") && strings.Contains(lowerText, "@example.com") {
		email := extractEmail(text)
		if email != "" {
			maskedText := strings.ReplaceAll(text, email, "$ANONYMIZED_EMAIL$")
			response := ApplyGuardrailResponse{
				Action: "GUARDRAIL_INTERVENED",
				Outputs: []map[string]interface{}{
					{"text": maskedText},
				},
				Assessments: []Assessment{
					{
						SensitiveInformationPolicy: &SensitiveInformationPolicyAssessment{
							PiiEntities: []PiiEntity{
								{
									Type:   "EMAIL",
									Match:  email,
									Action: "ANONYMIZED",
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Check for PII with redaction (any @example.com email)
	if strings.Contains(lowerText, "@example.com") {
		email := extractEmail(text)
		if email != "" {
			redactedText := strings.ReplaceAll(text, email, "*****")
			response := ApplyGuardrailResponse{
				Action: "GUARDRAIL_INTERVENED",
				Outputs: []map[string]interface{}{
					{"text": redactedText},
				},
				Assessments: []Assessment{
					{
						SensitiveInformationPolicy: &SensitiveInformationPolicyAssessment{
							PiiEntities: []PiiEntity{
								{
									Type:   "EMAIL",
									Match:  email,
									Action: "ANONYMIZED",
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Default: safe content
	response := ApplyGuardrailResponse{
		Action:      "NONE",
		Outputs:     []map[string]interface{}{},
		Assessments: []Assessment{},
	}
	json.NewEncoder(w).Encode(response)
}

func extractEmail(text string) string {
	match := emailRegex.FindString(text)
	return match
}

func main() {
	// Handle all requests - the AWS SDK will call /guardrail/{id}/version/{version}/apply
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this matches the guardrail apply endpoint pattern
		if guardrailPathRegex.MatchString(r.URL.Path) && r.Method == http.MethodPost {
			handleApplyGuardrail(w, r)
			return
		}

		// Health check
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		// Log unhandled requests
		log.Printf("Mock AWS Bedrock Guardrail: Unhandled request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	log.Println("Mock AWS Bedrock Guardrail server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
