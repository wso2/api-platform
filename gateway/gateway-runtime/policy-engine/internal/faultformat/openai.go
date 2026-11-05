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

package faultformat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// OpenAIErrorKinds returns the API kinds whose errors render in the OpenAI shape. Unlike
// supportedKinds they follow policy_engine.llm_errors.enable_openai_format.
func OpenAIErrorKinds() []policy.APIKind {
	return []policy.APIKind{policy.APIKindLlmProvider, policy.APIKindLlmProxy}
}

// ─── OpenAI ──────────────────────────────────────────────────────────────────

// openAIRenderer emits the OpenAI error envelope:
//
//	{"error":{"message":"…","type":"…","param":null,"code":"…"|null}}
//
// `param` and `code` are always present, as in the reference API. Gateway extras (error_id,
// guardrail) go inside `error`, so the top level stays what SDKs expect.
type openAIRenderer struct{}

func (openAIRenderer) ContentType() string { return "application/json" }

type openAIErrorObject struct {
	Message   string                     `json:"message"`
	Type      string                     `json:"type"`
	Param     *string                    `json:"param"`
	Code      *string                    `json:"code"`
	ErrorID   string                     `json:"error_id,omitempty"`
	Guardrail *openAIGuardrailErrorBlock `json:"guardrail,omitempty"`
}

// openAIGuardrailErrorBlock is guardrailBlock plus the direction, which lives on FaultDetails.
type openAIGuardrailErrorBlock struct {
	guardrailBlock
	Direction string `json:"direction,omitempty"`
}

func openAIGuardrailFor(e policy.FaultDetails) *openAIGuardrailErrorBlock {
	g := guardrailFor(e)
	if g == nil {
		return nil
	}
	return &openAIGuardrailErrorBlock{
		guardrailBlock: *g,
		Direction:      strings.ToUpper(e.Direction),
	}
}

type openAIErrorBody struct {
	Error openAIErrorObject `json:"error"`
}

func (openAIRenderer) Render(in RenderInput) []byte {
	e := in.Err
	var code *string
	if e.Code != "" {
		code = &e.Code
	}
	obj := openAIErrorObject{
		Message:   openAIMessage(e, in.Status),
		Type:      openAITypeFor(e.Type, in.Status),
		Code:      code,
		ErrorID:   in.ErrorID,
		Guardrail: openAIGuardrailFor(e),
	}
	body, err := json.Marshal(openAIErrorBody{Error: obj})
	if err != nil && obj.Guardrail != nil {
		// Only unencodable guardrail assessments get here: drop the block, keep the rest.
		obj.Guardrail = nil
		body, err = json.Marshal(openAIErrorBody{Error: obj})
	}
	if err != nil {
		// A terse valid envelope beats an empty body to a client that parses JSON.
		return []byte(`{"error":{"message":"An unexpected error occurred.","type":"server_error","param":null,"code":null}}`)
	}
	return body
}

// openAIMessage is fallbackMessage, but prefers the status text to the generic sentence.
func openAIMessage(e policy.FaultDetails, status int) string {
	if e.Message != "" {
		return e.Message
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return fallbackMessage(e)
}

// OpenAI error `type` values, spelled as the reference API does.
const (
	openAITypeInvalidRequest = "invalid_request_error"
	openAITypeAuthentication = "authentication_error"
	openAITypePermission     = "permission_error"
	openAITypeNotFound       = "not_found_error"
	openAITypeRateLimit      = "rate_limit_error"
	openAITypeServer         = "server_error"
)

// openAITypeFor maps the fault type onto an OpenAI type, falling back to the status.
func openAITypeFor(faultType string, status int) string {
	switch faultType {
	case policy.FaultTypeAuthentication:
		return openAITypeAuthentication
	case policy.FaultTypeAuthorization:
		return openAITypePermission
	case policy.FaultTypeThrottling:
		return openAITypeRateLimit
	case policy.FaultTypeGuardrail, policy.FaultTypeValidation, policy.FaultTypeRequestSize:
		// OpenAI reports its own content-policy refusals as invalid requests.
		return openAITypeInvalidRequest
	case policy.FaultTypeRouting:
		return openAITypeNotFound
	case policy.FaultTypeUpstream, policy.FaultTypeInternal:
		return openAITypeServer
	}
	switch {
	case status == http.StatusUnauthorized:
		return openAITypeAuthentication
	case status == http.StatusForbidden:
		return openAITypePermission
	case status == http.StatusNotFound:
		return openAITypeNotFound
	case status == http.StatusTooManyRequests:
		return openAITypeRateLimit
	case status >= 400 && status < 500:
		return openAITypeInvalidRequest
	default:
		return openAITypeServer
	}
}

// ─── Reshaping a body a policy wrote ─────────────────────────────────────────

// OpenAIPolicyBodyFault describes a failure from a body the producing policy wrote without a
// FaultDetails, and reports whether the body is to be replaced by the envelope. Other shapes
// keep such a body, but an OpenAI SDK reads only `error.message`. A guardrail's body keeps its
// whole intervention (see legacyGuardrailFault). An OpenAI-shaped or explicitly empty body is
// kept as written.
func OpenAIPolicyBodyFault(body []byte) (policy.FaultDetails, bool) {
	if len(body) == 0 || isOpenAIBody(body) {
		return policy.FaultDetails{}, false
	}
	if fault, ok := legacyGuardrailFault(body); ok {
		return fault, true
	}
	return policy.FaultDetails{Message: legacyMessage(body)}, true
}

// legacyGuardrailFault describes a guardrail's intervention from the body the shipped
// guardrails write:
//
//	{"type":"WORD_COUNT_GUARDRAIL","message":{"action":"GUARDRAIL_INTERVENED",
//	  "interveningGuardrail":"word-count-guardrail","direction":"REQUEST",
//	  "actionReason":"…","assessments":…}}
//
// interveningGuardrail marks the shape. assessments is present only with showAssessment; an
// object is carried as is, anything else under "details".
func legacyGuardrailFault(body []byte) (policy.FaultDetails, bool) {
	var envelope struct {
		Message *struct {
			Action               string          `json:"action"`
			InterveningGuardrail string          `json:"interveningGuardrail"`
			Direction            string          `json:"direction"`
			ActionReason         string          `json:"actionReason"`
			Assessments          json.RawMessage `json:"assessments"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Message == nil {
		return policy.FaultDetails{}, false
	}
	m := envelope.Message
	if strings.TrimSpace(m.InterveningGuardrail) == "" {
		return policy.FaultDetails{}, false
	}
	return policy.FaultDetails{
		Type:      policy.FaultTypeGuardrail,
		Direction: legacyGuardrailDirection(m.Direction),
		Message:   strings.TrimSpace(m.ActionReason),
		Guardrail: &policy.GuardrailDetails{
			InterveningGuardrail: m.InterveningGuardrail,
			Action:               m.Action,
			ActionReason:         m.ActionReason,
			Assessments:          legacyAssessments(m.Assessments),
		},
	}, true
}

// legacyGuardrailDirection maps "REQUEST"/"RESPONSE" onto the SDK's spelling; anything else is "".
func legacyGuardrailDirection(d string) string {
	switch strings.ToUpper(strings.TrimSpace(d)) {
	case "REQUEST":
		return policy.DirectionRequest
	case "RESPONSE":
		return policy.DirectionResponse
	default:
		return ""
	}
}

// legacyAssessments fits assessments into GuardrailDetails.Assessments; absent or null is nil.
func legacyAssessments(raw json.RawMessage) map[string]any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil
	}
	if obj, ok := value.(map[string]any); ok {
		return obj
	}
	return map[string]any{"details": value}
}

// isOpenAIBody reports whether a body is already the OpenAI envelope.
func isOpenAIBody(body []byte) bool {
	var envelope struct {
		Error *struct {
			Message *string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return envelope.Error != nil && envelope.Error.Message != nil
}

// maxLegacyMessageBytes caps a message lifted from a plain-text body.
const maxLegacyMessageBytes = 512

// legacyMessage lifts a message out of a policy's own body, or returns "". Fields are tried
// most specific first (`message` before `error`, which is usually a short name); non-string
// values are skipped.
func legacyMessage(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || !utf8.Valid(trimmed) {
		return ""
	}

	if trimmed[0] == '{' || trimmed[0] == '[' {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &fields); err != nil {
			return ""
		}
		if inner, ok := fields["error"]; ok {
			var nested map[string]json.RawMessage
			if json.Unmarshal(inner, &nested) == nil {
				if s := jsonString(nested["message"]); s != "" {
					return s
				}
			}
		}
		for _, key := range []string{"message", "error_description", "detail", "error", "description"} {
			if s := jsonString(fields[key]); s != "" {
				return s
			}
		}
		return ""
	}

	if trimmed[0] == '<' {
		// An HTML or XML document is not a message.
		return ""
	}
	text := string(trimmed)
	if len(text) > maxLegacyMessageBytes {
		text = text[:maxLegacyMessageBytes]
		// Do not leave a split rune at the cut.
		for len(text) > 0 && !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	return strings.TrimSpace(text)
}

// jsonString returns raw's value when it is a non-empty JSON string, and "" otherwise.
func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}
