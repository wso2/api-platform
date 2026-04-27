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

package hub

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	bindingcfg "github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	requestPathMetadataKey   = "request_path"
	requestMethodMetadataKey = "request_method"
)

// SubscribeToRequestHeaderContext maps a subscribe request message to a policy request header context.
func SubscribeToRequestHeaderContext(msg *connectors.Message, binding *ChannelBinding) *policy.RequestHeaderContext {
	headers := normalizeHeaders(msg.Headers)
	hubPath := requestPath(msg, defaultPhasePath(binding, "subscribe"))
	method := requestMethod(msg, "SUBSCRIBE")

	return &policy.RequestHeaderContext{
		SharedContext: newSharedContext(binding, hubPath),
		Headers:       policy.NewHeaders(headers),
		Path:          hubPath,
		Method:        method,
		Authority:     binding.Vhost,
		Scheme:        "event",
		Vhost:         binding.Vhost,
	}
}

// MessageToRequestHeaderContext maps an event message to a policy request header context.
func MessageToRequestHeaderContext(msg *connectors.Message, binding *ChannelBinding) *policy.RequestHeaderContext {
	headers := normalizeHeaders(msg.Headers)
	channelPath := requestPath(msg, defaultPhasePath(binding, "inbound"))
	method := requestMethod(msg, directionToMethod(binding.Mode, "inbound"))

	return &policy.RequestHeaderContext{
		SharedContext: newSharedContext(binding, channelPath),
		Headers:       policy.NewHeaders(headers),
		Path:          channelPath,
		Method:        method,
		Authority:     binding.Vhost,
		Scheme:        "event",
		Vhost:         binding.Vhost,
	}
}

// MessageToRequestContext maps an event message to a policy request body context.
func MessageToRequestContext(msg *connectors.Message, binding *ChannelBinding) *policy.RequestContext {
	headers := normalizeHeaders(msg.Headers)
	channelPath := requestPath(msg, defaultPhasePath(binding, "inbound"))
	method := requestMethod(msg, directionToMethod(binding.Mode, "inbound"))

	return &policy.RequestContext{
		SharedContext: newSharedContext(binding, channelPath),
		Headers:       policy.NewHeaders(headers),
		Body:          &policy.Body{Content: msg.Value, EndOfStream: true, Present: len(msg.Value) > 0},
		Path:          channelPath,
		Method:        method,
		Authority:     binding.Vhost,
		Scheme:        "event",
		Vhost:         binding.Vhost,
	}
}

// MessageToResponseHeaderContext maps an event message to a policy response header context.
func MessageToResponseHeaderContext(msg *connectors.Message, binding *ChannelBinding) *policy.ResponseHeaderContext {
	headers := normalizeHeaders(msg.Headers)
	reqPath := requestPath(msg, defaultPhasePath(binding, "outbound"))
	reqMethod := requestMethod(msg, directionToMethod(binding.Mode, "outbound"))

	return &policy.ResponseHeaderContext{
		SharedContext:   newSharedContext(binding, reqPath),
		RequestPath:     reqPath,
		RequestMethod:   reqMethod,
		ResponseHeaders: policy.NewHeaders(headers),
		ResponseStatus:  200,
	}
}

// MessageToResponseContext maps an event message to a policy response body context.
func MessageToResponseContext(msg *connectors.Message, binding *ChannelBinding) *policy.ResponseContext {
	headers := normalizeHeaders(msg.Headers)
	reqPath := requestPath(msg, defaultPhasePath(binding, "outbound"))
	reqMethod := requestMethod(msg, directionToMethod(binding.Mode, "outbound"))

	return &policy.ResponseContext{
		SharedContext:   newSharedContext(binding, reqPath),
		RequestPath:     reqPath,
		RequestMethod:   reqMethod,
		ResponseHeaders: policy.NewHeaders(headers),
		ResponseBody:    &policy.Body{Content: msg.Value, EndOfStream: true, Present: len(msg.Value) > 0},
		ResponseStatus:  200,
	}
}

func normalizeHeaders(headers map[string][]string) map[string][]string {
	normalized := make(map[string][]string, len(headers))
	for k, v := range headers {
		normalized[strings.ToLower(k)] = v
	}
	return normalized
}

func newSharedContext(binding *ChannelBinding, operationPath string) *policy.SharedContext {
	return &policy.SharedContext{
		RequestID:     uuid.New().String(),
		Metadata:      make(map[string]interface{}),
		APIId:         binding.APIID,
		APIName:       binding.Name,
		APIVersion:    binding.Version,
		APIKind:       policy.APIKindWebSubApi,
		APIContext:    binding.Context,
		OperationPath: operationPath,
	}
}

func requestPath(msg *connectors.Message, fallback string) string {
	if msg != nil && msg.Metadata != nil {
		if path, ok := msg.Metadata[requestPathMetadataKey].(string); ok && path != "" {
			return path
		}
	}
	return fallback
}

func requestMethod(msg *connectors.Message, fallback string) string {
	if msg != nil && msg.Metadata != nil {
		if method, ok := msg.Metadata[requestMethodMetadataKey].(string); ok && method != "" {
			return method
		}
	}
	return fallback
}

func defaultPhasePath(binding *ChannelBinding, phase string) string {
	if binding == nil {
		return ""
	}

	switch binding.Mode {
	case "websub":
		basePath := bindingcfg.WebSubApiBasePath(binding.Context, binding.Version)
		switch phase {
		case "subscribe", "outbound":
			return basePath + "/hub"
		case "inbound":
			return basePath + "/webhook-receiver"
		}
	}

	return path.Join(binding.Context, binding.Name)
}

// ApplyRequestHeaderResult applies the header result back to the message.
// Returns an error if unsupported HTTP-only actions are present.
func ApplyRequestHeaderResult(result *engine.RequestHeaderResult, msg *connectors.Message) error {
	if result == nil {
		return nil
	}

	if msg.Headers == nil {
		msg.Headers = make(map[string][]string)
	}

	for k, v := range result.HeadersToSet {
		msg.Headers[strings.ToLower(k)] = []string{v}
	}
	for _, k := range result.HeadersToRemove {
		delete(msg.Headers, strings.ToLower(k))
	}

	return nil
}

// ApplyRequestBodyResult applies the body result back to the message.
func ApplyRequestBodyResult(result *engine.RequestBodyResult, msg *connectors.Message) error {
	if result == nil {
		return nil
	}

	if msg.Headers == nil {
		msg.Headers = make(map[string][]string)
	}

	for k, v := range result.HeadersToSet {
		msg.Headers[strings.ToLower(k)] = []string{v}
	}
	for _, k := range result.HeadersToRemove {
		delete(msg.Headers, strings.ToLower(k))
	}
	if result.Body != nil {
		msg.Value = result.Body
	}

	return nil
}

// ApplyResponseHeaderResult applies the response header result back to the message.
func ApplyResponseHeaderResult(result *engine.ResponseHeaderResult, msg *connectors.Message) error {
	if result == nil {
		return nil
	}

	if msg.Headers == nil {
		msg.Headers = make(map[string][]string)
	}

	for k, v := range result.HeadersToSet {
		msg.Headers[strings.ToLower(k)] = []string{v}
	}
	for _, k := range result.HeadersToRemove {
		delete(msg.Headers, strings.ToLower(k))
	}

	return nil
}

// ApplyResponseBodyResult applies the response body result back to the message.
func ApplyResponseBodyResult(result *engine.ResponseBodyResult, msg *connectors.Message) error {
	if result == nil {
		return nil
	}

	if msg.Headers == nil {
		msg.Headers = make(map[string][]string)
	}

	for k, v := range result.HeadersToSet {
		msg.Headers[strings.ToLower(k)] = []string{v}
	}
	for _, k := range result.HeadersToRemove {
		delete(msg.Headers, strings.ToLower(k))
	}
	if result.Body != nil {
		msg.Value = result.Body
	}

	return nil
}

// IsUnsupportedAction checks if a policy result contains HTTP-only actions
// that are not supported in the event-gateway context.
func IsUnsupportedAction(description string) error {
	return fmt.Errorf("unsupported action in event-gateway context: %s", description)
}

func directionToMethod(mode, direction string) string {
	switch mode {
	case "websub":
		if direction == "inbound" {
			return "SUB"
		}
		return "DELIVER"
	case "protocol-mediation":
		if direction == "inbound" {
			return "PUBLISH"
		}
		return "DELIVER"
	}
	return "UNKNOWN"
}
