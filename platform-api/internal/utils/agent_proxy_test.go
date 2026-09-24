/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// updateAgentProxyGolden rewrites the golden files from the current builder
// output: go test ./internal/utils -run TestAgentProxyDeploymentYAMLGolden -update-agent-golden
var updateAgentProxyGolden = flag.Bool("update-agent-golden", false, "rewrite Agent proxy deployment YAML golden files")

const agentProxyTestProjectUUID = "0b7d3c1e-5f2a-4c8e-9d6b-1a2b3c4d5e6f"

// agentProxyMinimalRequest is the Section 1 minimal request: an existing A2A
// agent, defaults everywhere, passthrough card, no operationConfigs.
const agentProxyMinimalRequest = `{
  "displayName": "Weather Agent",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "upstream": {
    "main": {
      "url": "http://weather-agent:9000"
    }
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "transports": [
      {
        "protocolBinding": "JSONRPC",
        "pathPrefix": "/rpc"
      }
    ]
  }
}`

// agentProxyFullRequest is the Section 1 fuller request: both transports,
// upstream auth, a per-operation rate limit, a managed public card and a
// passthrough protected card.
const agentProxyFullRequest = `{
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
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "{{ secret \"weather-upstream\" }}"
      }
    }
  },
  "resilience": {
    "idleTimeout": "5s"
  },
  "associatedGateways": [
    {
      "id": "ai-gw-prod"
    }
  ],
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "transports": [
      {
        "protocolBinding": "JSONRPC",
        "pathPrefix": "/rpc"
      },
      {
        "protocolBinding": "HTTP+JSON",
        "pathPrefix": "/rest"
      }
    ],
    "operationConfigs": {
      "policies": [
        {
          "name": "jwt-auth",
          "version": "v1",
          "params": {
            "issuer": "https://idp.example.com",
            "requiredScopes": [
              "a2a.invoke"
            ]
          }
        }
      ],
      "operations": [
        {
          "name": "SendMessage",
          "policies": [
            {
              "name": "advanced-ratelimit",
              "version": "v1",
              "params": {
                "quotas": [
                  {
                    "name": "send-message-limit",
                    "limits": [
                      {
                        "limit": 100,
                        "duration": "1m"
                      }
                    ]
                  }
                ]
              }
            }
          ]
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
        "policies": [
          {
            "name": "cors",
            "version": "v1"
          }
        ],
        "content": {
          "name": "Weather Agent",
          "description": "Provides forecasts and severe-weather alerts",
          "version": "1.0.0",
          "supportedInterfaces": [
            {
              "protocolBinding": "JSONRPC",
              "url": "https://agent-proxies.gw.com/weather/rpc",
              "protocolVersion": "1.0"
            },
            {
              "protocolBinding": "HTTP+JSON",
              "url": "https://agent-proxies.gw.com/weather/rest",
              "protocolVersion": "1.0"
            }
          ],
          "capabilities": {
            "streaming": true,
            "extendedAgentCard": true
          },
          "defaultInputModes": [
            "text/plain"
          ],
          "defaultOutputModes": [
            "text/plain"
          ],
          "skills": [
            {
              "id": "forecast",
              "name": "Forecast",
              "description": "Multi-day forecast for a location",
              "tags": [
                "weather"
              ]
            }
          ],
          "securityRequirements": [
            {
              "schemes": {
                "gateway-jwt": {
                  "list": [
                    "a2a.invoke"
                  ]
                }
              }
            }
          ],
          "securitySchemes": {
            "gateway-jwt": {
              "openIdConnectSecurityScheme": {
                "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
              }
            }
          }
        }
      },
      "protected": {
        "mode": "passthrough",
        "rewriteUrls": true
      }
    }
  }
}`

// agentProxyCardVariantsRequest covers what the two Section 1 requests do not:
// omitted operationConfigs alongside an explicit card block, a passthrough public
// card with an explicit rewriteUrls: false and a managed protected card, and a
// per-transport default pathPrefix.
const agentProxyCardVariantsRequest = `{
  "id": "travel-agent",
  "displayName": "Travel Agent",
  "version": "v2.1",
  "projectId": "default-project",
  "upstream": {
    "main": {
      "url": "https://travel-agent.internal:8443/a2a"
    }
  },
  "resilience": {
    "timeout": "30s"
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "transports": [
      {
        "protocolBinding": "HTTP+JSON"
      }
    ],
    "agentCard": {
      "public": {
        "rewriteUrls": false
      },
      "protected": {
        "mode": "managed",
        "content": {
          "name": "Travel Agent",
          "version": "2.1.0",
          "supportedInterfaces": [
            {
              "protocolBinding": "HTTP+JSON",
              "url": "https://agents.example.com/",
              "protocolVersion": "1.0"
            }
          ],
          "capabilities": {
            "streaming": false
          },
          "skills": []
        }
      }
    }
  }
}`

// agentProxyFromRequest builds the persisted model for a request the way the
// service does, then round-trips the configuration through JSON exactly as the
// repository stores and reloads it — so the builder sees what it sees in
// production, including JSON numbers decoded as float64.
func agentProxyFromRequest(t *testing.T, handle, body string) *model.AgentProxy {
	t.Helper()
	req, err := dto.DecodeAgentProxyRequest([]byte(body))
	require.NoError(t, err)

	stored, err := json.Marshal(dto.AgentProxyConfigurationFromRequest(req))
	require.NoError(t, err)
	var configuration model.AgentProxyConfiguration
	require.NoError(t, json.Unmarshal(stored, &configuration))

	proxy := &model.AgentProxy{
		UUID:             "6a1f9e0c-2b3d-4e5f-8a9b-0c1d2e3f4a5b",
		Handle:           handle,
		OrganizationUUID: "c2d3e4f5-a6b7-4c8d-9e0f-1a2b3c4d5e6f",
		ProjectUUID:      agentProxyTestProjectUUID,
		Name:             req.DisplayName,
		Protocol:         model.AgentProxyProtocol(req.Protocol),
		Version:          req.Version,
		CreatedBy:        "alice",
		UpdatedBy:        "bob",
		Configuration:    configuration,
		Origin:           constants.OriginCP,
	}
	if req.Description != nil {
		proxy.Description = *req.Description
	}
	return proxy
}

func TestAgentProxyDeploymentYAMLGolden(t *testing.T) {
	cases := []struct {
		name   string
		handle string
		body   string
	}{
		{"minimal", "weather-agent", agentProxyMinimalRequest},
		{"full", "weather-agent", agentProxyFullRequest},
		{"card_variants", "travel-agent", agentProxyCardVariantsRequest},
	}
	util := &AgentProxyUtils{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := util.GenerateAgentProxyDeploymentYAML(agentProxyFromRequest(t, tc.handle, tc.body))
			require.NoError(t, err)

			golden := filepath.Join("testdata", "agent_proxy", tc.name+".yaml")
			if *updateAgentProxyGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
				require.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
			}
			want, err := os.ReadFile(golden)
			require.NoError(t, err, "golden file missing; run with -update-agent-golden")
			assert.Equal(t, string(want), got, "generated YAML differs from %s", golden)
		})
	}
}

// renderAgentYAMLTree marshals the built artifact and decodes it generically, so
// assertions are about the emitted document rather than the Go structs.
func renderAgentYAMLTree(t *testing.T, proxy *model.AgentProxy) map[string]any {
	t.Helper()
	out, err := (&AgentProxyUtils{}).GenerateAgentProxyDeploymentYAML(proxy)
	require.NoError(t, err)
	var tree map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(out), &tree))
	return tree
}

func mapAt(t *testing.T, tree map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := tree
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		require.Truef(t, ok, "expected a mapping at %q", k)
		cur = next
	}
	return cur
}

func TestBuildAgentProxyDeploymentYAML_GatewayEnvelope(t *testing.T) {
	tree := renderAgentYAMLTree(t, agentProxyFromRequest(t, "weather-agent", agentProxyFullRequest))

	assert.Equal(t, constants.GatewayApiVersion, tree["apiVersion"])
	assert.Equal(t, "Agent", tree["kind"], "the gateway artifact kind is Agent, never the CP kind")
	metadata := mapAt(t, tree, "metadata")
	assert.Equal(t, "weather-agent", metadata["name"])
	assert.Equal(t, map[string]any{"projectId": agentProxyTestProjectUUID}, metadata["labels"])

	spec := mapAt(t, tree, "spec")
	assert.Equal(t, "Weather Agent", spec["displayName"])
	assert.Equal(t, "v1.0", spec["version"])
	assert.Equal(t, "/weather", spec["context"])
	assert.Equal(t, "agents.gw.com", spec["vhost"])
	assert.Equal(t, map[string]any{"idleTimeout": "5s"}, spec["resilience"])
}

func TestBuildAgentProxyDeploymentYAML_NoControlPlaneOnlyFields(t *testing.T) {
	tree := renderAgentYAMLTree(t, agentProxyFromRequest(t, "weather-agent", agentProxyFullRequest))

	spec := mapAt(t, tree, "spec")
	allowed := map[string]bool{
		"displayName": true, "version": true, "context": true, "vhost": true,
		"upstream": true, "resilience": true, "a2a": true,
	}
	for key := range spec {
		assert.Truef(t, allowed[key], "unexpected key %q under gateway spec", key)
	}
	for _, cpOnly := range []string{
		"protocol", "kind", "id", "projectId", "description", "associatedGateways",
		"createdBy", "updatedBy", "createdAt", "updatedAt", "origin", "readOnly",
		"transports", "operationConfigs", "agentCard", "configuration",
	} {
		assert.NotContainsf(t, spec, cpOnly, "%q must not be emitted under gateway spec", cpOnly)
	}
	a2a := mapAt(t, spec, "a2a")
	assert.NotContains(t, a2a, "protocol")
	assert.NotContains(t, a2a, "transports", "transports belong under spec.a2a.operationConfigs")
	assert.ElementsMatch(t, []string{"protocolVersion", "operationConfigs", "agentCard"}, keysOf(a2a))
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBuildAgentProxyDeploymentYAML_A2AMapping(t *testing.T) {
	tree := renderAgentYAMLTree(t, agentProxyFromRequest(t, "weather-agent", agentProxyFullRequest))
	a2a := mapAt(t, tree, "spec", "a2a")

	assert.Equal(t, "1.0", a2a["protocolVersion"])

	opConfigs := mapAt(t, a2a, "operationConfigs")
	assert.Equal(t, []any{
		map[string]any{"protocolBinding": "JSONRPC", "pathPrefix": "/rpc"},
		map[string]any{"protocolBinding": "HTTP+JSON", "pathPrefix": "/rest"},
	}, opConfigs["transports"])
	assert.Equal(t, []any{map[string]any{
		"name": "jwt-auth", "version": "v1",
		"params": map[string]any{"issuer": "https://idp.example.com", "requiredScopes": []any{"a2a.invoke"}},
	}}, opConfigs["policies"])

	operations, ok := opConfigs["operations"].([]any)
	require.True(t, ok)
	require.Len(t, operations, 1)
	op := operations[0].(map[string]any)
	assert.Equal(t, "SendMessage", op["name"])
	policies := op["policies"].([]any)
	require.Len(t, policies, 1)
	params := policies[0].(map[string]any)["params"].(map[string]any)
	limit := params["quotas"].([]any)[0].(map[string]any)["limits"].([]any)[0].(map[string]any)["limit"]
	assert.Equal(t, 100, limit, "numeric policy params must stay numeric after the JSON round trip")

	public := mapAt(t, a2a, "agentCard", "public")
	assert.Equal(t, "managed", public["mode"])
	assert.Equal(t, "/.well-known/agent-card.json", public["path"])
	assert.NotContains(t, public, "rewriteUrls")
	content := mapAt(t, public, "content")
	assert.Equal(t, "Weather Agent", content["name"])
	assert.Contains(t, content, "securityRequirements")

	protected := mapAt(t, a2a, "agentCard", "protected")
	assert.Equal(t, map[string]any{"mode": "passthrough", "rewriteUrls": true}, protected)
}

func TestBuildAgentProxyDeploymentYAML_UpstreamCredentialIsCarriedAsStored(t *testing.T) {
	tree := renderAgentYAMLTree(t, agentProxyFromRequest(t, "weather-agent", agentProxyFullRequest))

	assert.Equal(t, map[string]any{
		"url": "http://weather-agent:9000",
		"auth": map[string]any{
			"type":   "api-key",
			"header": "X-API-Key",
			"value":  `{{ secret "weather-upstream" }}`,
		},
	}, mapAt(t, tree, "spec", "upstream"))
}

func TestBuildAgentProxyDeploymentYAML_MinimalOmitsOptionalBlocks(t *testing.T) {
	tree := renderAgentYAMLTree(t, agentProxyFromRequest(t, "weather-agent", agentProxyMinimalRequest))
	spec := mapAt(t, tree, "spec")

	assert.NotContains(t, spec, "vhost")
	assert.NotContains(t, spec, "resilience")
	a2a := mapAt(t, spec, "a2a")
	assert.NotContains(t, a2a, "agentCard", "an omitted card block stays omitted; the gateway defaults it to passthrough")

	// operationConfigs is emitted even though the request omitted it, because the
	// gateway requires transports there — and it carries nothing else.
	assert.Equal(t, map[string]any{
		"transports": []any{map[string]any{"protocolBinding": "JSONRPC", "pathPrefix": "/rpc"}},
	}, mapAt(t, a2a, "operationConfigs"))
}

func TestBuildAgentProxyDeploymentYAML_CardVariantsPreserveOmissionAndFalse(t *testing.T) {
	tree := renderAgentYAMLTree(t, agentProxyFromRequest(t, "travel-agent", agentProxyCardVariantsRequest))
	spec := mapAt(t, tree, "spec")

	assert.NotContains(t, spec, "context", "an omitted context stays omitted")

	opConfigs := mapAt(t, spec, "a2a", "operationConfigs")
	assert.Equal(t, []any{map[string]any{"protocolBinding": "HTTP+JSON"}}, opConfigs["transports"],
		"an omitted pathPrefix stays omitted")
	assert.NotContains(t, opConfigs, "policies")
	assert.NotContains(t, opConfigs, "operations")

	public := mapAt(t, spec, "a2a", "agentCard", "public")
	assert.Equal(t, map[string]any{"rewriteUrls": false}, public,
		"explicit rewriteUrls: false is preserved and no mode/path default is materialized")

	protected := mapAt(t, spec, "a2a", "agentCard", "protected")
	assert.Equal(t, "managed", protected["mode"])
	assert.NotContains(t, protected, "rewriteUrls")
	content := mapAt(t, protected, "content")
	assert.Equal(t, []any{}, content["skills"], "an empty array inside card content is preserved")
	assert.Equal(t, map[string]any{"streaming": false}, content["capabilities"])
}

func TestBuildAgentProxyDeploymentYAML_OmittedPublicBlockWithProtected(t *testing.T) {
	proxy := agentProxyFromRequest(t, "weather-agent", agentProxyMinimalRequest)
	proxy.Configuration.A2A.AgentCard = &model.AgentCardConfig{
		Protected: &model.ProtectedAgentCard{Mode: model.AgentCardModePassthrough},
	}

	card := mapAt(t, renderAgentYAMLTree(t, proxy), "spec", "a2a", "agentCard")
	assert.Equal(t, map[string]any{"protected": map[string]any{"mode": "passthrough"}}, card)
}

func TestBuildAgentProxyDeploymentYAML_RejectsInconsistentModels(t *testing.T) {
	util := &AgentProxyUtils{}

	_, err := util.BuildAgentProxyDeploymentYAML(nil)
	assert.Error(t, err)

	cases := map[string]func(p *model.AgentProxy){
		"unsupported protocol": func(p *model.AgentProxy) { p.Protocol = "mcp" },
		"empty protocol":       func(p *model.AgentProxy) { p.Protocol = "" },
		"missing a2a block":    func(p *model.AgentProxy) { p.Configuration.A2A = nil },
		"missing main":         func(p *model.AgentProxy) { p.Configuration.Upstream.Main = nil },
		"no transports":        func(p *model.AgentProxy) { p.Configuration.A2A.Transports = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			proxy := agentProxyFromRequest(t, "weather-agent", agentProxyMinimalRequest)
			mutate(proxy)
			d, err := util.BuildAgentProxyDeploymentYAML(proxy)
			assert.Error(t, err)
			assert.Nil(t, d)
		})
	}
}

func TestCreateAgentYamlZip_UsesAgentPrefix(t *testing.T) {
	data, err := CreateAgentYamlZip(map[string]string{"6a1f9e0c": "kind: Agent\n"})
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
	assert.Equal(t, "agent-6a1f9e0c.yaml", zr.File[0].Name)

	rc, err := zr.File[0].Open()
	require.NoError(t, err)
	defer rc.Close()
	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "kind: Agent\n", string(body))
}

func TestCreateBatchDeploymentTarGz_AgentProxyUsesAgentPrefix(t *testing.T) {
	data, err := CreateBatchDeploymentTarGz(map[string]*model.DeploymentContent{
		"dep-1": {DeploymentID: "dep-1", ArtifactID: "art-1", Type: constants.AgentProxy, Content: []byte("kind: Agent\n")},
		"dep-2": {DeploymentID: "dep-2", ArtifactID: "art-2", Type: constants.MCPProxy, Content: []byte("kind: Mcp\n")},
	})
	require.NoError(t, err)

	gz, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, hdr.Name)
	}
	assert.ElementsMatch(t, []string{"dep-1/agent-art-1.yaml", "dep-2/mcp-proxy-art-2.yaml"}, names,
		"an AgentProxy must not fall through to the api- prefix")
}
