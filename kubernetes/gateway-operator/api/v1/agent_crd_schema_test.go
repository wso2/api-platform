/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"sigs.k8s.io/yaml"
)

// agentWorkedExample is the Agent worked example (agent-schema.yaml examples[0],
// mirrored in management-openapi.yaml's AgentConfigurationRequest example),
// expressed as an Agent CR. It exercises every branch of the spec: both
// transports, common and per-operation policies, a managed public card with a
// custom path, card policies, free-form card content with nested objects, a
// string-typed protocol version, an empty list, a false value, and signing.
const agentWorkedExample = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: weather-agent-v1-0
spec:
  displayName: Weather Agent
  version: v1.0
  context: /weather
  vhost: agents.example.com
  upstream:
    url: https://weather.internal
  resilience:
    timeout: 30s
    idleTimeout: 5m
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
        - protocolBinding: HTTP+JSON
          pathPrefix: /rest
      policies:
        - name: jwt-auth
          version: v1
          params:
            issuer: https://idp.example.com
            requiredScopes:
              - a2a.invoke
      operations:
        - name: SendMessage
          policies:
            - name: advanced-ratelimit
              version: v1
              params:
                quotas:
                  - name: send-message-limit
                    limits:
                      - limit: 100
                        duration: 1m
    agentCard:
      public:
        mode: managed
        path: /.well-known/agent-card.json
        policies:
          - name: cors
            version: v1
        content:
          name: Weather Agent
          description: Provides weather information
          version: 1.0.0
          supportedInterfaces:
            - protocolBinding: JSONRPC
              protocolVersion: "1.0"
              url: https://agents.example.com/weather/rpc
            - protocolBinding: HTTP+JSON
              protocolVersion: "1.0"
              url: https://agents.example.com/weather/rest
          capabilities:
            streaming: true
            pushNotifications: false
          securitySchemes:
            gateway-jwt:
              openIdConnectSecurityScheme:
                openIdConnectUrl: https://idp.example.com/.well-known/openid-configuration
          securityRequirements:
            - schemes:
                gateway-jwt:
                  list:
                    - a2a.invoke
          defaultInputModes:
            - text/plain
          defaultOutputModes:
            - text/plain
          extensions: []
          skills:
            - id: get_weather
              name: Get weather
              description: Gets weather information
              tags:
                - weather
        signing:
          enabled: true
`

// TestAgentCRDAcceptsWorkedExample asserts the generated Agent CRD schema
// accepts the worked example, on every served version.
//
// The CRD type and the gateway-controller management-API schema are separately
// maintained representations of one resource with no compile-time link, so this
// is the guard that a marker added on one side does not reject an artifact the
// other side accepts.
//
// Note: CEL x-kubernetes-validations rules are enforced by the API server, not
// by this validator, so they are out of this test's reach. AgentUpstream no
// longer has one — url is required and there is no ref — so its contract is
// structural and covered below.
func TestAgentCRDAcceptsWorkedExample(t *testing.T) {
	obj := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(agentWorkedExample), &obj); err != nil {
		t.Fatalf("unmarshal worked example: %v", err)
	}

	for version, validator := range agentSchemaValidators(t) {
		t.Run(version, func(t *testing.T) {
			if errs := validation.ValidateCustomResource(nil, obj, validator); len(errs) > 0 {
				t.Fatalf("worked example rejected by the %s schema: %v", version, errs.ToAggregate())
			}
		})
	}
}

// TestAgentCRDRejectsInvalidSpecs asserts the constraints mirrored from the
// management-API schema are actually present in the generated CRD — a marker
// silently dropped in generation would otherwise let the CRD accept an artifact
// the gateway-controller then rejects at deploy time, moving the error from
// `kubectl apply` to a status condition.
func TestAgentCRDRejectsInvalidSpecs(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(spec map[string]interface{})
		wantErr string
	}{
		{
			name: "unsupported protocol version",
			mutate: func(spec map[string]interface{}) {
				a2a(spec)["protocolVersion"] = "2.0"
			},
			wantErr: "protocolVersion",
		},
		{
			name: "unknown protocol binding",
			mutate: func(spec map[string]interface{}) {
				transports(spec)[0].(map[string]interface{})["protocolBinding"] = "GRPC"
			},
			wantErr: "protocolBinding",
		},
		{
			name: "unknown canonical operation name",
			mutate: func(spec map[string]interface{}) {
				ops := operationConfigs(spec)["operations"].([]interface{})
				ops[0].(map[string]interface{})["name"] = "SendMessages"
			},
			wantErr: "name",
		},
		{
			name: "unknown public card mode",
			mutate: func(spec map[string]interface{}) {
				publicCard(spec)["mode"] = "rewrite"
			},
			wantErr: "mode",
		},
		{
			name: "a2a block missing",
			mutate: func(spec map[string]interface{}) {
				delete(spec, "a2a")
			},
			wantErr: "a2a",
		},
		{
			name: "transports empty",
			mutate: func(spec map[string]interface{}) {
				operationConfigs(spec)["transports"] = []interface{}{}
			},
			wantErr: "transports",
		},
		{
			name: "version not semantic",
			mutate: func(spec map[string]interface{}) {
				spec["version"] = "1"
			},
			wantErr: "version",
		},
		{
			name: "context with a trailing slash",
			mutate: func(spec map[string]interface{}) {
				spec["context"] = "/weather/"
			},
			wantErr: "context",
		},
		{
			name: "signing enabled flag missing",
			mutate: func(spec map[string]interface{}) {
				publicCard(spec)["signing"] = map[string]interface{}{}
			},
			wantErr: "enabled",
		},
		{
			// An Agent forwards to exactly one upstream and, in passthrough card
			// mode, fetches its card from the same origin, so the
			// gateway-controller requires a url where the other kinds accept a
			// ref instead. Admission has to say the same thing, or a ref-only
			// Agent applies cleanly and then fails at deploy time.
			name: "upstream url missing",
			mutate: func(spec map[string]interface{}) {
				delete(spec["upstream"].(map[string]interface{}), "url")
			},
			wantErr: "url",
		},
		{
			name: "upstream url empty",
			mutate: func(spec map[string]interface{}) {
				spec["upstream"].(map[string]interface{})["url"] = ""
			},
			wantErr: "url",
		},
		{
			// There is no ref form for an Agent, so the schema prunes or rejects
			// one rather than admitting a shape the controller ignores.
			name: "upstream ref instead of url",
			mutate: func(spec map[string]interface{}) {
				upstream := spec["upstream"].(map[string]interface{})
				delete(upstream, "url")
				upstream["ref"] = "weather-pool"
			},
			wantErr: "url",
		},
	}

	validators := agentSchemaValidators(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := map[string]interface{}{}
			if err := yaml.Unmarshal([]byte(agentWorkedExample), &obj); err != nil {
				t.Fatalf("unmarshal worked example: %v", err)
			}
			tc.mutate(obj["spec"].(map[string]interface{}))

			for version, validator := range validators {
				errs := validation.ValidateCustomResource(nil, obj, validator)
				if len(errs) == 0 {
					t.Fatalf("%s schema accepted an invalid spec", version)
				}
				if !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
					t.Errorf("%s schema rejected the spec but not for %q: %v",
						version, tc.wantErr, errs.ToAggregate())
				}
			}
		})
	}
}

func a2a(spec map[string]interface{}) map[string]interface{} {
	return spec["a2a"].(map[string]interface{})
}

func operationConfigs(spec map[string]interface{}) map[string]interface{} {
	return a2a(spec)["operationConfigs"].(map[string]interface{})
}

func transports(spec map[string]interface{}) []interface{} {
	return operationConfigs(spec)["transports"].([]interface{})
}

func publicCard(spec map[string]interface{}) map[string]interface{} {
	return a2a(spec)["agentCard"].(map[string]interface{})["public"].(map[string]interface{})
}

// agentSchemaValidators returns one schema validator per served version of the
// generated Agent CRD, keyed by version name.
func agentSchemaValidators(t *testing.T) map[string]validation.SchemaValidator {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	// api/v1/<thisfile> -> ../../config/crd/bases/...
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "crd", "bases",
		"gateway.api-platform.wso2.com_agents.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run `make manifests`?): %v", path, err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if len(crd.Spec.Versions) == 0 {
		t.Fatalf("%s serves no versions", crd.Name)
	}

	out := make(map[string]validation.SchemaValidator, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			t.Fatalf("version %q of %s has no schema", v.Name, crd.Name)
		}
		internal := &apiextensions.JSONSchemaProps{}
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v.Schema.OpenAPIV3Schema, internal, nil); err != nil {
			t.Fatalf("convert %q schema: %v", v.Name, err)
		}
		validator, _, err := validation.NewSchemaValidator(internal)
		if err != nil {
			t.Fatalf("build validator for %q: %v", v.Name, err)
		}
		out[v.Name] = validator
	}
	return out
}
