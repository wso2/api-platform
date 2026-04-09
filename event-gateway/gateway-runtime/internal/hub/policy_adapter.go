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
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// MessageToRequestHeaderContext maps an event message to a policy request header context.
func MessageToRequestHeaderContext(msg *connectors.Message, binding *ChannelBinding) *policy.RequestHeaderContext {
	headers := make(map[string][]string)
	for k, v := range msg.Headers {
		headers[strings.ToLower(k)] = v
	}

	channelPath := path.Join(binding.Context, binding.Name)
	method := directionToMethod(binding.Mode, "inbound")

	return &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{
			RequestID: uuid.New().String(),
			Metadata:  make(map[string]interface{}),
			APIKind:   policy.APIKindWebSubApi,
		},
		Headers:   policy.NewHeaders(headers),
		Path:      channelPath,
		Method:    method,
		Authority: binding.Vhost,
		Scheme:    "event",
		Vhost:     binding.Vhost,
	}
}

// MessageToRequestContext maps an event message to a policy request body context.
func MessageToRequestContext(msg *connectors.Message, binding *ChannelBinding) *policy.RequestContext {
	headers := make(map[string][]string)
	for k, v := range msg.Headers {
		headers[strings.ToLower(k)] = v
	}

	channelPath := path.Join(binding.Context, binding.Name)
	method := directionToMethod(binding.Mode, "inbound")

	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{
			RequestID: uuid.New().String(),
			Metadata:  make(map[string]interface{}),
			APIKind:   policy.APIKindWebSubApi,
		},
		Headers:   policy.NewHeaders(headers),
		Body:      &policy.Body{Content: msg.Value, EndOfStream: true, Present: len(msg.Value) > 0},
		Path:      channelPath,
		Method:    method,
		Authority: binding.Vhost,
		Scheme:    "event",
		Vhost:     binding.Vhost,
	}
}

// MessageToResponseHeaderContext maps an event message to a policy response header context.
func MessageToResponseHeaderContext(msg *connectors.Message, binding *ChannelBinding) *policy.ResponseHeaderContext {
	headers := make(map[string][]string)
	for k, v := range msg.Headers {
		headers[strings.ToLower(k)] = v
	}

	return &policy.ResponseHeaderContext{
		SharedContext: &policy.SharedContext{
			RequestID: uuid.New().String(),
			Metadata:  make(map[string]interface{}),
			APIKind:   policy.APIKindWebSubApi,
		},
		ResponseHeaders: policy.NewHeaders(headers),
		ResponseStatus:  200,
	}
}

// MessageToResponseContext maps an event message to a policy response body context.
func MessageToResponseContext(msg *connectors.Message, binding *ChannelBinding) *policy.ResponseContext {
	headers := make(map[string][]string)
	for k, v := range msg.Headers {
		headers[strings.ToLower(k)] = v
	}

	return &policy.ResponseContext{
		SharedContext: &policy.SharedContext{
			RequestID: uuid.New().String(),
			Metadata:  make(map[string]interface{}),
			APIKind:   policy.APIKindWebSubApi,
		},
		ResponseHeaders: policy.NewHeaders(headers),
		ResponseBody:    &policy.Body{Content: msg.Value, EndOfStream: true, Present: len(msg.Value) > 0},
		ResponseStatus:  200,
	}
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
