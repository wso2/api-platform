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

package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// minimalAgentProxyJSON is the smallest accepted create body: defaults
// everywhere, no card block, one transport.
const minimalAgentProxyJSON = `{
  "displayName": "Weather Agent",
  "version": "v1.0",
  "projectId": "default-project",
  "upstream": { "main": { "url": "http://weather-agent:9000" } },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "transports": [ { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" } ]
  }
}`

func TestDecodeAgentProxyRequestAcceptsMinimalBody(t *testing.T) {
	req, err := DecodeAgentProxyRequest([]byte(minimalAgentProxyJSON))
	if err != nil {
		t.Fatalf("DecodeAgentProxyRequest: %v", err)
	}
	if req.DisplayName != "Weather Agent" || req.ProjectId != "default-project" {
		t.Fatalf("common fields not decoded: %+v", req)
	}
	if req.Protocol != api.A2AAgentProxyProtocolA2a {
		t.Fatalf("protocol = %q, want a2a", req.Protocol)
	}
	if len(req.A2a.Transports) != 1 || req.A2a.Transports[0].ProtocolBinding != "JSONRPC" {
		t.Fatalf("transports not decoded: %+v", req.A2a.Transports)
	}
	// An omitted card block stays omitted — it is never materialized into an
	// explicit default, because a stored explicit block and an absent one mean
	// different things downstream.
	if req.A2a.AgentCard != nil {
		t.Fatalf("agentCard = %+v, want nil for an omitted block", req.A2a.AgentCard)
	}
}

// TestDecodeAgentProxyRequestPreservesCardExtensions is the other half of
// unknown-field rejection: the rejection must stop at the typed boundary.
// Agent Card content and policy parameters are free-form by contract, so a
// vendor extension inside them has to survive untouched.
func TestDecodeAgentProxyRequestPreservesCardExtensions(t *testing.T) {
	body := `{
      "displayName": "Weather Agent",
      "version": "v1.0",
      "projectId": "default-project",
      "upstream": { "main": { "url": "http://weather-agent:9000" } },
      "protocol": "a2a",
      "a2a": {
        "protocolVersion": "1.0",
        "transports": [ { "protocolBinding": "JSONRPC" } ],
        "operationConfigs": {
          "policies": [ { "name": "jwt-auth", "version": "v1", "params": { "x-vendor": { "nested": true } } } ]
        },
        "agentCard": {
          "public": {
            "mode": "managed",
            "content": { "name": "Weather Agent", "x-vendor-custom": { "kept": true }, "signing": "not-a-typed-field" }
          }
        }
      }
    }`

	req, err := DecodeAgentProxyRequest([]byte(body))
	if err != nil {
		t.Fatalf("DecodeAgentProxyRequest: %v", err)
	}

	content := *req.A2a.AgentCard.Public.Content
	ext, ok := content["x-vendor-custom"].(map[string]any)
	if !ok || ext["kept"] != true {
		t.Fatalf("card extension not preserved: %#v", content)
	}
	// "signing" is rejected as a typed card *configuration* property but is an
	// ordinary key inside free-form content; the two must not be confused.
	if _, ok := content["signing"]; !ok {
		t.Fatalf("free-form content key dropped: %#v", content)
	}

	params := *(*req.A2a.OperationConfigs.Policies)[0].Params
	if _, ok := params["x-vendor"]; !ok {
		t.Fatalf("policy params extension not preserved: %#v", params)
	}
}

func TestDecodeAgentProxyRequestRejections(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		contains string
	}{
		{
			name:     "not an object",
			body:     `["a2a"]`,
			contains: "must be a JSON object",
		},
		{
			name:     "missing protocol",
			body:     `{"displayName":"a","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://x"}},"a2a":{"protocolVersion":"1.0","transports":[]}}`,
			contains: "protocol field is required",
		},
		{
			name:     "null protocol",
			body:     `{"protocol":null,"a2a":{}}`,
			contains: "must be a non-empty string",
		},
		{
			name:     "unsupported protocol",
			body:     `{"protocol":"mcp","a2a":{}}`,
			contains: `protocol "mcp" is not supported`,
		},
		{
			name:     "missing matching configuration block",
			body:     `{"displayName":"a","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://x"}},"protocol":"a2a"}`,
			contains: `"a2a" configuration block is required`,
		},
		{
			name:     "generic protocolConfig wrapper",
			body:     `{"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[]},"protocolConfig":{}}`,
			contains: `unsupported fields: "protocolConfig"`,
		},
		{
			name:     "legacy top-level A2A fields",
			body:     `{"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[]},"transports":[],"agentCard":{}}`,
			contains: `unsupported fields: "agentCard", "transports"`,
		},
		{
			// The card configuration types carry a generated UnmarshalJSON, so
			// the decoder's own DisallowUnknownFields cannot see this key —
			// only the structural walk rejects it.
			name:     "signing rejected on a card configuration",
			body:     `{"displayName":"a","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[],"agentCard":{"public":{"mode":"passthrough","signing":{}}}}}`,
			contains: `unsupported fields: "a2a.agentCard.public.signing"`,
		},
		{
			name:     "unknown key inside an array element",
			body:     `{"displayName":"a","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"},{"protocolBinding":"HTTP+JSON","weight":3}]}}`,
			contains: `unsupported fields: "a2a.transports[1].weight"`,
		},
		{
			name:     "path rejected on a protected card",
			body:     `{"displayName":"a","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[],"agentCard":{"protected":{"mode":"passthrough","path":"/x"}}}}`,
			contains: `unsupported fields: "a2a.agentCard.protected.path"`,
		},
		{
			name:     "wrong field type",
			body:     `{"displayName":123,"version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[]}}`,
			contains: `field "displayName" has the wrong type`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeAgentProxyRequest([]byte(tc.body))
			if err == nil {
				t.Fatalf("expected a rejection, got none")
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.contains)
			}
			// error-handling.md: nothing internal ever reaches the caller, and
			// these messages are handed straight to the client.
			for _, leak := range []string{"Go struct", "json:", "api.", "model."} {
				if strings.Contains(err.Error(), leak) {
					t.Fatalf("error %q leaks an internal detail (%q)", err.Error(), leak)
				}
			}
		})
	}
}

// TestAssertSingleProtocolBlock covers the rule that only bites once a second
// protocol is registered: a body carrying one protocol's discriminator and
// another's configuration block. The registered list is supplied here so the
// two-protocol case is exercised today rather than the first time a real second
// protocol lands.
func TestAssertSingleProtocolBlock(t *testing.T) {
	registered := []string{"a2a", "acp"}

	tests := []struct {
		name     string
		body     string
		contains string
	}{
		{
			name: "matching block alone is accepted",
			body: `{"protocol":"a2a","a2a":{}}`,
		},
		{
			name:     "matching block missing",
			body:     `{"protocol":"a2a"}`,
			contains: `"a2a" configuration block is required`,
		},
		{
			name:     "another protocol's block alongside",
			body:     `{"protocol":"a2a","a2a":{},"acp":{}}`,
			contains: "Exactly one protocol configuration block is permitted",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.body), &raw); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}
			err := assertSingleProtocolBlock(raw, model.AgentProxyProtocolA2A, registered)
			if tc.contains == "" {
				if err != nil {
					t.Fatalf("unexpected rejection: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected a rejection")
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.contains)
			}
		})
	}
}
