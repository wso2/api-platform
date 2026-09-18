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

package server

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
)

// agentProxiesPath and fetchAgentCardPath are absolute request paths including
// the base path from the spec's first server entry, which is how the validator's
// router resolves an operation.
const (
	agentProxiesPath   = "/api/v0.9/agent-proxies"
	fetchAgentCardPath = "/api/v0.9/agent-proxies/fetch-agent-card"
)

// newSpecValidator builds a request validator over the shipped spec. This is a
// build-time fixture check only: no OpenAPI validation middleware runs at
// request time, so the schemas below are documentation and codegen input, and
// the service layer remains responsible for enforcing them.
func newSpecValidator(t *testing.T) validator.Validator {
	t.Helper()

	data, err := os.ReadFile(realSpecPath)
	if err != nil {
		t.Fatalf("read %q: %v", realSpecPath, err)
	}
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		t.Fatalf("parse %q: %v", realSpecPath, err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		t.Fatalf("build validator for %q: %v", realSpecPath, errs)
	}
	return v
}

func postJSON(t *testing.T, path, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

// minimalAgentProxy is the smallest accepted create payload: an existing A2A
// agent, defaults everywhere, and a passthrough card by omission.
const minimalAgentProxy = `{
  "displayName": "Weather Agent",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "upstream": { "main": { "url": "http://weather-agent:9000" } },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "transports": [ { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" } ]
  }
}`

// fullAgentProxy exercises both transports, upstream auth, a common policy, a
// per-operation policy, a managed public card and a passthrough protected card.
// The card content carries an unregistered extension key to confirm free-form
// card content is still accepted.
const fullAgentProxy = `{
  "id": "weather-agent",
  "displayName": "Weather Agent",
  "description": "Provides forecasts and severe-weather alerts",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "vhost": "agents.gw.com",
  "upstream": {
    "main": {
      "url": "http://weather-agent:9000",
      "auth": { "type": "api-key", "header": "X-API-Key", "value": "{{ secret \"weather-upstream\" }}" }
    }
  },
  "resilience": { "idleTimeout": "5s" },
  "associatedGateways": [ { "id": "ai-gw-prod" } ],
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "transports": [
      { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" },
      { "protocolBinding": "HTTP+JSON", "pathPrefix": "/rest" }
    ],
    "operationConfigs": {
      "policies": [
        { "name": "jwt-auth", "version": "v1",
          "params": { "issuer": "https://idp.example.com", "requiredScopes": ["a2a.invoke"] } }
      ],
      "operations": [
        { "name": "SendMessage",
          "policies": [ { "name": "advanced-ratelimit", "version": "v1" } ],
          "resilience": { "timeout": "30s" } }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
        "policies": [ { "name": "cors", "version": "v1" } ],
        "content": {
          "name": "Weather Agent",
          "version": "1.0.0",
          "x-vendor-extension": { "anything": true },
          "supportedInterfaces": [
            { "protocolBinding": "JSONRPC", "url": "https://agent-proxies.gw.com/weather/rpc", "protocolVersion": "1.0" }
          ],
          "securityRequirements": [
            { "schemes": { "gateway-jwt": { "list": ["a2a.invoke"] } } }
          ]
        }
      },
      "protected": { "mode": "passthrough", "rewriteUrls": true }
    }
  }
}`

// TestPlatformAPISpecIsAValidOpenAPIDocument validates the shipped spec against
// the OpenAPI 3.1 meta-schema — unknown keys, bad path templates, unresolvable
// $refs and malformed schema objects all surface here. The spec is codegen input
// and the source of the scope registry, so a document that only *looks* parseable
// would take both of those down quietly.
func TestPlatformAPISpecIsAValidOpenAPIDocument(t *testing.T) {
	ok, errs := newSpecValidator(t).ValidateDocument()
	if !ok {
		for _, e := range errs {
			t.Errorf("%s: %s", e.Message, e.Reason)
		}
	}
}

// TestAgentProxyCreateFixtures pins the create contract against the schemas the
// spec actually ships, rather than against the prose describing them.
func TestAgentProxyCreateFixtures(t *testing.T) {
	v := newSpecValidator(t)

	valid := map[string]string{
		"minimal":                     minimalAgentProxy,
		"full with managed card":      fullAgentProxy,
		"passthrough public explicit": withA2A(`"agentCard": { "public": { "mode": "passthrough", "rewriteUrls": false } }`),
		"managed protected card":      withA2A(`"agentCard": { "protected": { "mode": "managed", "content": { "name": "Weather Agent" } } }`),
	}
	for name, body := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if ok, errs := v.ValidateHttpRequest(postJSON(t, agentProxiesPath, body)); !ok {
				t.Errorf("payload should be accepted but was rejected: %v", errs)
			}
		})
	}

	invalid := map[string]string{
		// protocol and its matching named configuration block are both required,
		// and the protocol must be one this API registers.
		"no protocol": `{"displayName":"A","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://a:1"}},
			"a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`,
		"protocol without a2a block": `{"displayName":"A","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://a:1"}},
			"protocol":"a2a"}`,
		"unsupported protocol": `{"displayName":"A","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://a:1"}},
			"protocol":"mcp","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`,
		"unregistered a2a protocol version": withA2AVersion(`"9.9"`),

		// The card lives under a2a.agentCard and nowhere else.
		"card at the resource root": withRoot(`"agentCard": { "public": { "mode": "passthrough" } }`),

		// Managed and passthrough are disjoint branches.
		"managed public card without content":     withA2A(`"agentCard": { "public": { "mode": "managed" } }`),
		"managed public card with rewriteUrls":    withA2A(`"agentCard": { "public": { "mode": "managed", "rewriteUrls": true, "content": {} } }`),
		"passthrough public card with content":    withA2A(`"agentCard": { "public": { "mode": "passthrough", "content": {} } }`),
		"protected card without an explicit mode": withA2A(`"agentCard": { "protected": { "rewriteUrls": true } }`),

		// Three deliberate absences: no card-serving signing option anywhere, and
		// no path or policies on the protected card, which is an A2A operation
		// rather than a discovery route.
		"signing on the public card":     withA2A(`"agentCard": { "public": { "mode": "passthrough", "signing": { "enabled": true } } }`),
		"path on the protected card":     withA2A(`"agentCard": { "protected": { "mode": "passthrough", "path": "/card" } }`),
		"policies on the protected card": withA2A(`"agentCard": { "protected": { "mode": "passthrough", "policies": [] } }`),
		"top-level policies":             withRoot(`"policies": [ { "name": "cors", "version": "v1" } ]`),

		// The pre-nesting flat shape and the generic wrapper are both rejected.
		"legacy flat protocolVersion": withRoot(`"protocolVersion": "1.0"`),
		"generic protocolConfig wrapper": `{"displayName":"A","version":"v1.0","projectId":"p","upstream":{"main":{"url":"http://a:1"}},
			"protocol":"a2a","protocolConfig":{"protocolVersion":"1.0"}}`,

		"unknown field inside a2a":  withA2A(`"somethingElse": true`),
		"unknown operation name":    withA2A(`"operationConfigs": { "operations": [ { "name": "SendTelepathy" } ] }`),
		"unknown transport binding": withA2ATransport(`{"protocolBinding":"GRPC"}`),
		"three transports":          withA2ATransport(`{"protocolBinding":"JSONRPC"},{"protocolBinding":"HTTP+JSON"},{"protocolBinding":"JSONRPC","pathPrefix":"/x"}`),
	}
	for name, body := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			if ok, _ := v.ValidateHttpRequest(postJSON(t, agentProxiesPath, body)); ok {
				t.Errorf("payload should be rejected but was accepted")
			}
		})
	}
}

// TestFetchAgentCardFixtures pins the two mutually exclusive request forms:
// a direct URL with optional credentials, or a stored Agent proxy handle alone.
func TestFetchAgentCardFixtures(t *testing.T) {
	v := newSpecValidator(t)

	valid := map[string]string{
		"direct url":           `{"url":"http://weather-agent:9000"}`,
		"stored handle":        `{"agentProxyId":"weather-agent"}`,
		"direct url with auth": `{"url":"http://weather-agent:9000","auth":{"type":"api-key","header":"X-API-Key","value":"example-preview-key"}}`,
	}
	for name, body := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if ok, errs := v.ValidateHttpRequest(postJSON(t, fetchAgentCardPath, body)); !ok {
				t.Errorf("payload should be accepted but was rejected: %v", errs)
			}
		})
	}

	invalid := map[string]string{
		"empty object":          `{}`,
		"auth alone":            `{"auth":{"type":"api-key","header":"X-API-Key","value":"k"}}`,
		"handle plus url":       `{"agentProxyId":"weather-agent","url":"http://weather-agent:9000"}`,
		"handle plus auth":      `{"agentProxyId":"weather-agent","auth":{"type":"none"}}`,
		"handle plus null url":  `{"agentProxyId":"weather-agent","url":null}`,
		"handle plus null auth": `{"agentProxyId":"weather-agent","auth":null}`,
		"unknown field":         `{"url":"http://weather-agent:9000","refresh":true}`,
	}
	for name, body := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			if ok, _ := v.ValidateHttpRequest(postJSON(t, fetchAgentCardPath, body)); ok {
				t.Errorf("payload should be rejected but was accepted")
			}
		})
	}
}

// withRoot returns the minimal payload with one extra top-level member.
func withRoot(member string) string {
	return `{"displayName":"A","version":"v1.0","projectId":"default-project","upstream":{"main":{"url":"http://a:1"}},` +
		member + `,"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`
}

// withA2A returns the minimal payload with one extra member inside the a2a block.
func withA2A(member string) string {
	return `{"displayName":"A","version":"v1.0","projectId":"default-project","upstream":{"main":{"url":"http://a:1"}},` +
		`"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],` + member + `}}`
}

// withA2AVersion returns the minimal payload with a substituted a2a.protocolVersion.
func withA2AVersion(version string) string {
	return `{"displayName":"A","version":"v1.0","projectId":"default-project","upstream":{"main":{"url":"http://a:1"}},` +
		`"protocol":"a2a","a2a":{"protocolVersion":` + version + `,"transports":[{"protocolBinding":"JSONRPC"}]}}`
}

// withA2ATransport returns the minimal payload with a substituted transport array body.
func withA2ATransport(transports string) string {
	return `{"displayName":"A","version":"v1.0","projectId":"default-project","upstream":{"main":{"url":"http://a:1"}},` +
		`"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[` + transports + `]}}`
}
