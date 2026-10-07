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

// OpenAIErrorKinds returns the API kinds whose errors render in the OpenAI shape.
//
// Unlike supportedKinds these are NOT enabled by default: an LLM API has shipped with the error
// bodies its policies write, so switching them is an operator decision —
// policy_engine.llm_openai_compatible_errors.enabled. The kernel adds these to its KindSet when the
// option is on.
func OpenAIErrorKinds() []policy.APIKind {
	return []policy.APIKind{policy.APIKindLlmProvider, policy.APIKindLlmProxy}
}

// ─── OpenAI ──────────────────────────────────────────────────────────────────

// openAIRenderer emits the error envelope the OpenAI API returns and its SDKs parse:
//
//	{"error":{"message":"…","type":"…","param":null,"code":"…"|null}}
//
// `param` and `code` are always present, null when unknown, because the reference API always
// sends them and some clients index them unconditionally. Gateway extras (error_id, guardrail)
// go INSIDE `error`: the SDKs keep the whole object, so they stay reachable, and the top level
// stays exactly what a client expects.
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

// openAIGuardrailErrorBlock preserves the shipped guardrail response contract. Direction
// lives on FaultDetails rather than GuardrailDetails because it applies to every failure, so
// the shared guardrailBlock cannot carry it by itself.
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
	body, err := json.Marshal(openAIErrorBody{Error: openAIErrorObject{
		Message:   openAIMessage(e, in.Status),
		Type:      openAITypeFor(e.Type, in.Status),
		Code:      code,
		ErrorID:   in.ErrorID,
		Guardrail: openAIGuardrailFor(e),
	}})
	if err != nil {
		// Only an assessment map the guardrail filled with something unmarshalable gets here.
		// A terse valid envelope beats an empty body to a client that parses JSON.
		return []byte(`{"error":{"message":"An unexpected error occurred.","type":"server_error","param":null,"code":null}}`)
	}
	return body
}

// openAIMessage is fallbackMessage with one difference: a status text such as "Too Many
// Requests" says more to an SDK user than the generic sentence, and the OpenAI shape always
// has a status to read it from.
func openAIMessage(e policy.FaultDetails, status int) string {
	if e.Message != "" {
		return e.Message
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return fallbackMessage(e)
}

// OpenAI error `type` values. The SDKs choose their exception class from the status, not from
// this, so it is informational — but clients log and branch on it, so the spellings are the
// reference API's.
const (
	openAITypeInvalidRequest = "invalid_request_error"
	openAITypeAuthentication = "authentication_error"
	openAITypePermission     = "permission_error"
	openAITypeNotFound       = "not_found_error"
	openAITypeRateLimit      = "rate_limit_error"
	openAITypeServer         = "server_error"
)

// openAITypeFor maps the gateway's failure class onto an OpenAI type, falling back to the
// status when the class is absent or not one with a clear counterpart.
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

// OpenAIPolicyBodyMessage reshapes a body the PRODUCING policy wrote without describing the
// failure: it returns the message to describe the failure with instead, and true when the body
// is to be replaced by the OpenAI envelope rather than kept.
//
// Every other shape treats such a body as the policy's decision, because their clients can
// read a plain JSON body. An OpenAI SDK cannot: it reads `error.message`, and a
// `{"error":"Unauthorized"}` from a policy written for REST APIs surfaces as an error with no
// message. The operator turning llm_openai_compatible_errors on is the decision that every LLM
// error is to be readable by those SDKs, and the policy catalogue has not all moved its
// messages into FaultDetails yet — so the caller describes the failure with the body's own
// message and the formatter renders that. A body already in the OpenAI envelope is kept:
// reshaping it could only lose what its author put in `param` or `code`.
func OpenAIPolicyBodyMessage(body []byte) (string, bool) {
	if isOpenAIBody(body) {
		return "", false
	}
	return legacyMessage(body), true
}

// isOpenAIBody reports whether a body already is the OpenAI envelope, so reshaping it could
// only lose what its author put in `param` or `code`.
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

// maxLegacyMessageBytes caps a message lifted from a plain-text body. A policy's plain-text
// error is a sentence; anything longer is a document, and carrying it whole into a message
// field helps no one.
const maxLegacyMessageBytes = 512

// legacyMessage lifts a human-readable message out of a body a policy wrote in its own shape,
// returning "" when there is nothing usable — the renderer then falls back to the status text.
//
// Field order is most-specific first: `message` is the human summary in the catalogue's most
// common shape ({"error":"Unauthorized","message":"…"}), where `error` is only a short name.
// A field whose value is not a string is skipped rather than stringified: the guardrails put
// an assessment object under `message`, and that is not a sentence.
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
