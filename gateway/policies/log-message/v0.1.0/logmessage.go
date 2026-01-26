/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package logmessage

import (
	"encoding/json"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	HeaderXRequestID      = "x-request-id"
	FieldNamePayload      = "payload"
	FieldNameHeaders      = "headers"
	ErrMsgMissingReqID    = "<request-id-unavailable>"
	MediationFlowRequest  = "REQUEST"
	MediationFlowResponse = "RESPONSE"
	MediationFlowFault    = "FAULT"
)

// LogMessagePolicy implements logging of request/response payloads and headers
type LogMessagePolicy struct{}

var ins = &LogMessagePolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *LogMessagePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers for logging
		RequestBodyMode:    policy.BodyModeBuffer,    // Need request body for logging
		ResponseHeaderMode: policy.HeaderModeProcess, // Process response headers for logging
		ResponseBodyMode:   policy.BodyModeBuffer,    // Need response body for logging
	}
}

// OnRequest logs the request message
func (p *LogMessagePolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Extract response-specific parameters
	logRequestPayload, _ := params["logRequestPayload"].(bool)
	logRequestHeaders, _ := params["logRequestHeaders"].(bool)
	excludedRequestHeaders, _ := params["excludedRequestHeaders"].(string)

	// Skip logging if both response payload and headers are disabled
	if !logRequestPayload && !logRequestHeaders {
		return policy.UpstreamRequestModifications{}
	}

	// Create log record
	logRecord := LogRecord{
		MediationFlow: MediationFlowRequest,
		RequestID:     p.getRequestID(ctx.Headers),
		HTTPMethod:    ctx.Method,
		ResourcePath:  ctx.Path,
	}

	// Log payload if enabled
	if logRequestPayload && ctx.Body != nil && ctx.Body.Present && len(ctx.Body.Content) > 0 {
		logRecord.Payload = string(ctx.Body.Content)
	}

	// Log headers if enabled
	if logRequestHeaders {
		logRecord.Headers = p.buildHeadersMap(ctx.Headers, excludedRequestHeaders)
	}

	// Log the message
	p.logMessage(logRecord)

	// Continue with the request unchanged
	return policy.UpstreamRequestModifications{}
}

// OnResponse logs the response message
func (p *LogMessagePolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Extract response-specific parameters
	logResponsePayload, _ := params["logResponsePayload"].(bool)
	logResponseHeaders, _ := params["logResponseHeaders"].(bool)
	excludedResponseHeaders, _ := params["excludedResponseHeaders"].(string)

	// Skip logging if both response payload and headers are disabled
	if !logResponsePayload && !logResponseHeaders {
		return policy.UpstreamResponseModifications{}
	}

	// Create log record
	logRecord := LogRecord{
		MediationFlow: MediationFlowResponse,
		RequestID:     p.getResponseRequestID(ctx.ResponseHeaders),
		HTTPMethod:    ctx.RequestMethod,
		ResourcePath:  ctx.RequestPath,
	}

	// Log payload if enabled
	if logResponsePayload && ctx.ResponseBody != nil && ctx.ResponseBody.Present && len(ctx.ResponseBody.Content) > 0 {
		logRecord.Payload = string(ctx.ResponseBody.Content)
	}

	// Log headers if enabled
	if logResponseHeaders {
		logRecord.Headers = p.buildHeadersMap(ctx.ResponseHeaders, excludedResponseHeaders)
	}

	// Log the message
	p.logMessage(logRecord)

	// Continue with the response unchanged
	return policy.UpstreamResponseModifications{}
}

// LogRecord represents the structure of log data
type LogRecord struct {
	MediationFlow string                 `json:"mediation-flow"`
	RequestID     string                 `json:"request-id"`
	HTTPMethod    string                 `json:"http-method"`
	ResourcePath  string                 `json:"resource-path"`
	Payload       string                 `json:"payload,omitempty"`
	Headers       map[string]interface{} `json:"headers,omitempty"`
}

// getRequestID extracts request ID from request headers
func (p *LogMessagePolicy) getRequestID(headers *policy.Headers) string {
	if requestIDs := headers.Get(HeaderXRequestID); len(requestIDs) > 0 {
		return requestIDs[0]
	}
	return ErrMsgMissingReqID
}

// getResponseRequestID extracts request ID from response headers
func (p *LogMessagePolicy) getResponseRequestID(headers *policy.Headers) string {
	if requestIDs := headers.Get(HeaderXRequestID); len(requestIDs) > 0 {
		return requestIDs[0]
	}
	return ErrMsgMissingReqID
}

// buildHeadersMap builds a map of headers for logging, excluding sensitive ones
func (p *LogMessagePolicy) buildHeadersMap(headers *policy.Headers, excludedHeadersStr string) map[string]interface{} {
	headersMap := make(map[string]interface{})
	excludedHeaders := p.parseExcludedHeaders(excludedHeadersStr)

	headers.Iterate(func(name string, values []string) {
		lowerName := strings.ToLower(name)

		// Skip excluded headers
		if _, excluded := excludedHeaders[lowerName]; excluded {
			return // continue iteration
		}

		// Mask authorization header by default
		if lowerName == "authorization" {
			headersMap[name] = "***"
			return
		}

		// Add header to map
		if len(values) == 1 {
			headersMap[name] = values[0]
		} else {
			headersMap[name] = values
		}
	})

	return headersMap
}

// parseExcludedHeaders parses the comma-separated excluded headers string
func (p *LogMessagePolicy) parseExcludedHeaders(excludedHeadersStr string) map[string]struct{} {
	excludedHeaders := make(map[string]struct{})

	if excludedHeadersStr == "" {
		return excludedHeaders
	}

	headers := strings.Split(excludedHeadersStr, ",")
	for _, header := range headers {
		trimmed := strings.ToLower(strings.TrimSpace(header))
		if trimmed != "" {
			excludedHeaders[trimmed] = struct{}{}
		}
	}

	return excludedHeaders
}

// logMessage logs the structured log record using slog at INFO level
func (p *LogMessagePolicy) logMessage(record LogRecord) {
	logData, err := json.Marshal(record)
	if err != nil {
		slog.Error("Failed to marshal log record", "error", err)
		return
	}

	slog.Info(string(logData))
}
