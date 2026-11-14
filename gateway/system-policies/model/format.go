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

package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Inbound formats: the wire format an endpoint's clients speak. Every body this
// policy writes — an error, the catalog — is in the endpoint's format, because its
// clients are that format's SDKs and parse nothing else. The controller names the
// same formats (pkg/modelgateway/inbound), and a test on each side pins the
// strings.
const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
	FormatGemini    = "gemini"
)

var knownFormats = map[string]bool{FormatOpenAI: true, FormatAnthropic: true, FormatGemini: true}

// parseFormat reads a block's format. Absent is OpenAI, the one format a
// controller that predates the field compiles; an unknown one is refused, as any
// block this version cannot honour is.
func parseFormat(block map[string]any) (string, error) {
	raw, present := block[ParamFormat]
	if !present {
		return FormatOpenAI, nil
	}
	format, ok := raw.(string)
	if !ok || !knownFormats[format] {
		return "", fmt.Errorf("%s must be one of openai, anthropic, gemini", ParamFormat)
	}
	return format, nil
}

// errorResponse builds one refusal in the endpoint's format. code is the stable
// machine-readable reason (OpenAI's `code`), param the request member it concerns.
//
// The answers this policy writes author their own bodies and describe no
// policy.FaultDetails. That is deliberate: the engine's formatter renders a
// described fault in its own shape, and it has no slot for an OpenAI `param` or
// error code such as model_not_found. A body with no Fault beside it is an authored
// decision the formatter leaves alone, so the client receives exactly this.
func errorResponse(format string, status int, message, param, code string) policy.ImmediateResponse {
	var body any
	switch format {
	case FormatAnthropic:
		body = anthropicErrorBody{Type: "error", Error: anthropicError{Type: anthropicErrorType(status), Message: message}}
	case FormatGemini:
		body = geminiErrorBody{Error: geminiError{Code: status, Message: message, Status: googleStatus(status)}}
	default:
		e := openAIError{Message: message, Type: openAIErrorType(status)}
		if param != "" {
			e.Param = &param
		}
		if code != "" {
			e.Code = &code
		}
		body = openAIErrorBody{Error: e}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		// Unreachable for structs of strings; a constant is still better than an
		// empty body on an error path.
		encoded = []byte(`{"error":{"message":"The request could not be processed."}}`)
	}
	return policy.ImmediateResponse{
		StatusCode: status,
		Headers: map[string]string{
			"content-type":  contentTypeJSON,
			"cache-control": cacheControlNoStore,
		},
		Body: encoded,
	}
}

// openAIErrorBody is the OpenAI error envelope.
type openAIErrorBody struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

// openAIErrorType is the OpenAI error class for a status, as OpenAI answers it.
func openAIErrorType(status int) string {
	if status >= 400 && status < 500 {
		return openAIInvalidRequest
	}
	return openAIServerError
}

// anthropicErrorBody is the Anthropic Messages API error envelope.
type anthropicErrorBody struct {
	Type  string         `json:"type"`
	Error anthropicError `json:"error"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// anthropicErrorType is the Anthropic error type for a status, as the Messages API
// documents them.
func anthropicErrorType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	default:
		if status >= 400 && status < 500 {
			return "invalid_request_error"
		}
		return "api_error"
	}
}

// geminiErrorBody is the Google API error envelope the Gemini API answers with.
type geminiErrorBody struct {
	Error geminiError `json:"error"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// googleStatus is the canonical Google RPC status name for an HTTP status.
func googleStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_ARGUMENT"
	case http.StatusUnauthorized:
		return "UNAUTHENTICATED"
	case http.StatusForbidden:
		return "PERMISSION_DENIED"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "ABORTED"
	case http.StatusTooManyRequests:
		return "RESOURCE_EXHAUSTED"
	case http.StatusNotImplemented:
		return "UNIMPLEMENTED"
	case http.StatusServiceUnavailable:
		return "UNAVAILABLE"
	case http.StatusGatewayTimeout:
		return "DEADLINE_EXCEEDED"
	default:
		if status >= 400 && status < 500 {
			return "FAILED_PRECONDITION"
		}
		return "INTERNAL"
	}
}

// catalogOwner is the owned_by of every model in an OpenAI catalog. The catalog
// describes what this gateway serves, not who trains the model behind it.
const catalogOwner = "wso2"

// geminiGenerationMethods are the methods a Gemini endpoint serves each Model by.
var geminiGenerationMethods = []string{"generateContent", "streamGenerateContent"}

// renderCatalog renders the catalog entries in the endpoint's format. entries are
// already sorted and filtered.
func renderCatalog(format string, entries []catalogEntry) ([]byte, error) {
	switch format {
	case FormatAnthropic:
		type model struct {
			Type        string `json:"type"`
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CreatedAt   string `json:"created_at"`
		}
		out := struct {
			Data    []model `json:"data"`
			HasMore bool    `json:"has_more"`
			FirstID *string `json:"first_id"`
			LastID  *string `json:"last_id"`
		}{Data: make([]model, 0, len(entries))}
		for _, e := range entries {
			out.Data = append(out.Data, model{Type: "model", ID: e.name, DisplayName: e.displayName,
				CreatedAt: time.Unix(e.created, 0).UTC().Format(time.RFC3339)})
		}
		if len(entries) > 0 {
			first, last := entries[0].name, entries[len(entries)-1].name
			out.FirstID, out.LastID = &first, &last
		}
		return json.Marshal(out)
	case FormatGemini:
		type model struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		}
		out := struct {
			Models []model `json:"models"`
		}{Models: make([]model, 0, len(entries))}
		for _, e := range entries {
			out.Models = append(out.Models, model{Name: "models/" + e.name, DisplayName: e.displayName,
				SupportedGenerationMethods: geminiGenerationMethods})
		}
		return json.Marshal(out)
	default:
		type model struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}
		out := struct {
			Object string  `json:"object"`
			Data   []model `json:"data"`
		}{Object: "list", Data: make([]model, 0, len(entries))}
		for _, e := range entries {
			out.Data = append(out.Data, model{ID: e.name, Object: "model", Created: e.created, OwnedBy: catalogOwner})
		}
		return json.Marshal(out)
	}
}
