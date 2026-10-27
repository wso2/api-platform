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

package platformapi

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestArtifactPath(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		handle  string
		want    string
		wantErr bool
	}{
		{name: "template", kind: "LlmProviderTemplate", handle: "tmpl-1", want: "/llm-provider-templates/tmpl-1"},
		{name: "provider", kind: "LlmProvider", handle: "prov-1", want: "/llm-providers/prov-1"},
		{name: "proxy", kind: "LlmProxy", handle: "proxy-1", want: "/llm-proxies/proxy-1"},
		{name: "mcp", kind: "Mcp", handle: "mcp-1", want: "/mcp-proxies/mcp-1"},
		{name: "rest api", kind: "RestApi", handle: "api-1", want: "/rest-apis/api-1"},
		{name: "gateway agent is an agent proxy", kind: "Agent", handle: "agent-1", want: "/agent-proxies/agent-1"},
		{name: "control-plane kind is not a gateway kind", kind: "AgentProxy", handle: "x", wantErr: true},
		{name: "unknown kind", kind: "WebSubApi", handle: "x", wantErr: true},
		{name: "empty kind", kind: "", handle: "x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := artifactPath(tt.kind, tt.handle)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDeploymentStatusMatches(t *testing.T) {
	tests := []struct {
		name string
		resp *httpx.Response
		want string
		ok   bool
	}{
		{
			name: "matching status present",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[{"status":"DEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   true,
		},
		{
			name: "different status present",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[{"status":"UNDEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "multiple deployments, one matches",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[{"status":"ARCHIVED"},{"status":"DEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   true,
		},
		{
			name: "empty list",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[]}`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "non-200 status",
			resp: &httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(`{"list":[{"status":"DEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "malformed body",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`not json`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "nil response",
			resp: nil,
			want: "DEPLOYED",
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.ok, deploymentStatusMatches(tt.resp, tt.want))
		})
	}
}

func TestRegisterCreatedArtifactRegistersCleanupInDependencyOrder(t *testing.T) {
	ctx, reg := trackingContext(t)
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 1, time.Millisecond)

	for kind, id := range map[string]string{"LlmProvider": "prov-1", "LlmProxy": "proxy-1", "Mcp": "mcp-1"} {
		require.NoError(t, RegisterCreatedArtifact(ctx, nil, funnel, kind, id))
	}
	require.Equal(t, []string{"prov-1"}, pendingIDs(reg, platformProviderKind))
	require.Equal(t, []string{"proxy-1"}, pendingIDs(reg, platformProxyKind))
	require.Equal(t, []string{"mcp-1"}, pendingIDs(reg, platformMCPKind))

	// A proxy deletes before the provider it references; both delete before an associated
	// gateway, which deletes before the project.
	require.Less(t, platformProxyKind.Order, platformProviderKind.Order)
	require.Less(t, platformProviderKind.Order, platformGatewayKind.Order)
	require.Less(t, platformMCPKind.Order, platformGatewayKind.Order)
	require.Less(t, platformGatewayKind.Order, platformProjectKind.Order)

	require.ErrorContains(t, RegisterCreatedArtifact(ctx, nil, funnel, "RestApi", "api-1"), "no control-plane cleanup")
	require.ErrorContains(t, RegisterCreatedArtifact(ctx, nil, funnel, "", "x"), "no control-plane cleanup")
}

func TestIsLastProjectValidation(t *testing.T) {
	tests := []struct {
		name string
		resp *httpx.Response
		want bool
	}{
		{
			name: "last project",
			resp: &httpx.Response{StatusCode: http.StatusBadRequest,
				Body: []byte(`{"code":"VALIDATION_FAILED","message":"Organization must have at least one project"}`)},
			want: true,
		},
		{
			name: "different validation",
			resp: &httpx.Response{StatusCode: http.StatusBadRequest,
				Body: []byte(`{"code":"VALIDATION_FAILED","message":"Project has associated MCP proxies"}`)},
			want: false,
		},
		{
			name: "different status",
			resp: &httpx.Response{StatusCode: http.StatusInternalServerError,
				Body: []byte(`{"code":"VALIDATION_FAILED","message":"Organization must have at least one project"}`)},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isLastProjectValidation(tt.resp))
		})
	}
}

func TestActorCredentials(t *testing.T) {
	for who, want := range map[string]actor.Credentials{
		"":          actor.Administrator(),
		"admin":     actor.Administrator(),
		" admin ":   actor.Administrator(),
		"publisher": actor.Publisher(),
		"developer": actor.Developer(),
	} {
		got, err := actorCredentials(who)
		require.NoError(t, err, who)
		require.Equal(t, want, got, who)
	}
	for _, who := range []string{"narrow", "consumer", "Admin"} {
		_, err := actorCredentials(who)
		require.ErrorContains(t, err, "unknown control-plane actor", who)
	}
}

// trackingContext returns a runner context with a fresh cleanup registry installed.
func trackingContext(t *testing.T) (context.Context, *cleanup.Registry) {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	reg := cleanup.NewRegistry(slog.Default())
	require.NoError(t, cleanup.Install(ctx, reg))
	return ctx, reg
}

func pendingIDs(reg *cleanup.Registry, kind cleanup.Kind) []string {
	var ids []string
	for _, res := range reg.Pending() {
		if res.Kind.Name == kind.Name {
			ids = append(ids, res.ID)
		}
	}
	return ids
}

func created(location string) *httpx.Response {
	return &httpx.Response{StatusCode: http.StatusCreated, Headers: http.Header{"Location": []string{location}}}
}

func TestTrackControlPlaneResourceRegistersWhatTheControlPlaneCreated(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}

	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, []byte(`{"name":"d","gatewayId":"it-gateway"}`),
		created("/api/v0.9/agent-proxies/weather/deployments/dep-1")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil,
		created("/api/v0.9/agent-proxies/weather/api-keys/key%2D1")))

	require.Equal(t, []string{"weather"}, pendingIDs(reg, platformAgentProxyKind))
	require.Equal(t, []string{"weather\x00dep-1\x00it-gateway"}, pendingIDs(reg, platformAgentDeploymentKind))
	require.Equal(t, []string{"weather\x00key-1"}, pendingIDs(reg, platformAgentAPIKeyKind), "the Location is unescaped")

	// Cleanup order: keys, then deployments, then the Agent proxy that owns both.
	require.Less(t, platformAgentAPIKeyKind.Order, platformAgentDeploymentKind.Order)
	require.Less(t, platformAgentDeploymentKind.Order, platformAgentProxyKind.Order)
	require.Less(t, platformAgentProxyKind.Order, platformProjectKind.Order)
	require.Less(t, platformAgentProxyKind.Order, platformSecretKind.Order)

	// A repeated registration of the same resource is not a second record.
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather")))
	require.Len(t, pendingIDs(reg, platformAgentProxyKind), 1)
}

func TestTrackControlPlaneResourceIgnoresWhatIsNotATrackedCreate(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	for _, tc := range []struct {
		method string
		resp   *httpx.Response
	}{
		{http.MethodPost, &httpx.Response{StatusCode: http.StatusOK, Headers: http.Header{"Location": []string{"/api/v0.9/agent-proxies/a"}}}},
		{http.MethodPost, &httpx.Response{StatusCode: http.StatusBadRequest}},
		{http.MethodPost, created("")},
		{http.MethodPut, created("/api/v0.9/agent-proxies/a")},
		{http.MethodPut, created("/api/v0.9/rest-apis/a")},
		{http.MethodPost, created("/api/v0.9/agent-proxies/a/builds/b")},
		{http.MethodPost, created("/api/v0.9/rest-apis/a/builds/b")},
		{http.MethodPost, created("/api/v0.9/rest-apis/a/api-keys/k")},
		{http.MethodPost, created("/api/v0.9/organizations/o")},
		{http.MethodPost, created("/api/v0.9/subscriptions/s")},
		{http.MethodPost, created("/api/v1/rest-apis/a")},
	} {
		require.NoError(t, s.trackControlPlaneResource(ctx, tc.method, nil, tc.resp))
	}
	require.Empty(t, reg.Pending())
}

func TestTrackControlPlaneResourceRequiresTheDeploymentGateway(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	for _, body := range [][]byte{nil, []byte(`{`), []byte(`{"name":"d"}`), []byte(`{"gatewayId":"  "}`)} {
		err := s.trackControlPlaneResource(ctx, http.MethodPost, body, created("/api/v0.9/agent-proxies/a/deployments/d"))
		require.Error(t, err, string(body))
	}
	require.Empty(t, reg.Pending())
}

func TestTrackControlPlaneResourceDeregistersWhatTheScenarioDeleted(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/other")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
		created("/api/v0.9/agent-proxies/weather/deployments/dep-1")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
		created("/api/v0.9/agent-proxies/weather/deployments/dep-12")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather/api-keys/k")))

	deleted := func(url string) *httpx.Response {
		return &httpx.Response{StatusCode: http.StatusNoContent, URL: url}
	}
	// A failed delete leaves the record in place.
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		&httpx.Response{StatusCode: http.StatusConflict, URL: "https://cp/api/v0.9/agent-proxies/weather/deployments/dep-1"}))
	require.Len(t, pendingIDs(reg, platformAgentDeploymentKind), 2)

	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/agent-proxies/weather/deployments/dep-1")))
	require.Equal(t, []string{"weather\x00dep-12\x00gw"}, pendingIDs(reg, platformAgentDeploymentKind),
		"only the deleted deployment is dropped, not one whose id shares its prefix")
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/agent-proxies/weather/api-keys/k")))
	require.Empty(t, pendingIDs(reg, platformAgentAPIKeyKind))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/agent-proxies/weather?force=true")))
	require.Equal(t, []string{"other"}, pendingIDs(reg, platformAgentProxyKind))
}

func TestTrackControlPlaneResourceRegistersEveryTrackedCollection(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	for collection := range platformCollectionKinds {
		require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/"+collection+"/x")))
	}
	for collection, kind := range platformCollectionKinds {
		require.Equal(t, []string{"x"}, pendingIDs(reg, kind), collection)
	}

	for _, collection := range []string{"rest-apis", "graphql-apis", "mcp-proxies", "llm-providers", "llm-proxies"} {
		require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
			created("/api/v0.9/"+collection+"/x/deployments/d")))
	}
	require.ElementsMatch(t, []string{
		"rest-apis\x00x\x00d\x00gw", "graphql-apis\x00x\x00d\x00gw", "mcp-proxies\x00x\x00d\x00gw",
		"llm-providers\x00x\x00d\x00gw", "llm-proxies\x00x\x00d\x00gw",
	}, pendingIDs(reg, platformDeploymentKind))

	// A deployment undeploys before the artifact that owns it deletes, and an artifact deletes
	// before the gateway, project and portal it depends on.
	for _, artifact := range []cleanup.Kind{platformRESTAPIKind, platformGraphQLAPIKind, platformMCPKind,
		platformProviderKind, platformProxyKind} {
		require.Less(t, platformDeploymentKind.Order, artifact.Order, artifact.Name)
		require.Less(t, artifact.Order, platformGatewayKind.Order, artifact.Name)
		require.Less(t, artifact.Order, platformProjectKind.Order, artifact.Name)
	}
	require.Less(t, platformRESTAPIKind.Order, platformAPIPortalKind.Order)
	require.Less(t, platformGraphQLAPIKind.Order, platformAPIPortalKind.Order)
}

func TestTrackControlPlaneResourceRequiresTheGatewayOfEveryDeployment(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	for _, body := range [][]byte{nil, []byte(`{`), []byte(`{"name":"d"}`)} {
		err := s.trackControlPlaneResource(ctx, http.MethodPost, body, created("/api/v0.9/rest-apis/a/deployments/d"))
		require.Error(t, err, string(body))
	}
	require.Empty(t, reg.Pending())
}

func TestTrackControlPlaneResourceDeregistersDeletedPlatformResources(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/rest-apis/a")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, nil, created("/api/v0.9/rest-apis/b")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
		created("/api/v0.9/rest-apis/a/deployments/d1")))
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
		created("/api/v0.9/rest-apis/a/deployments/d12")))

	deleted := func(url string) *httpx.Response {
		return &httpx.Response{StatusCode: http.StatusNoContent, URL: url}
	}
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		&httpx.Response{StatusCode: http.StatusConflict, URL: "https://cp/api/v0.9/rest-apis/a"}))
	require.Len(t, pendingIDs(reg, platformRESTAPIKind), 2, "a failed delete leaves the record in place")

	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/rest-apis/a/deployments/d1")))
	require.Equal(t, []string{"rest-apis\x00a\x00d12\x00gw"}, pendingIDs(reg, platformDeploymentKind),
		"only the deleted deployment is dropped, not one whose id shares its prefix")
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/rest-apis/a")))
	require.Equal(t, []string{"b"}, pendingIDs(reg, platformRESTAPIKind))
}

func TestIsDeploymentNotActive(t *testing.T) {
	notActive := []byte(`{"code":"DEPLOYMENT_NOT_ACTIVE","message":"No active deployment"}`)
	require.True(t, isDeploymentNotActive(&httpx.Response{StatusCode: http.StatusConflict, Body: notActive}))
	for _, resp := range []*httpx.Response{
		nil,
		{StatusCode: http.StatusBadRequest, Body: notActive},
		{StatusCode: http.StatusConflict, Body: []byte(`{"code":"DEPLOYMENT_ACTIVE"}`)},
		{StatusCode: http.StatusConflict, Body: []byte(`not json`)},
		{StatusCode: http.StatusConflict},
	} {
		require.False(t, isDeploymentNotActive(resp))
	}
}

func TestImportedAgentProxyCleanup(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}

	require.NoError(t, s.registerImportedAgentProxy(ctx, "imported"))
	require.NoError(t, s.registerImportedAgentProxy(ctx, "other"))
	require.ElementsMatch(t, []string{"imported", "other"}, pendingIDs(reg, platformImportedAgentProxyKind))
	require.Empty(t, pendingIDs(reg, platformAgentProxyKind), "an imported copy is not an authored Agent proxy")
	require.Greater(t, platformImportedAgentProxyKind.Order, cleanup.KindAgent.Order,
		"the imported copy must delete after the gateway Agent it was imported from")

	// A scenario that deletes the imported copy itself drops only that record.
	require.NoError(t, s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		&httpx.Response{StatusCode: http.StatusNoContent, URL: "https://cp/api/v0.9/agent-proxies/imported"}))
	require.Equal(t, []string{"other"}, pendingIDs(reg, platformImportedAgentProxyKind))
}

func TestRegisterImportedCopyIsIdempotentPerKind(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}

	require.NoError(t, s.registerImportedCopy(ctx, "Agent", "imported"))
	require.NoError(t, s.registerImportedCopy(ctx, "Agent", "imported"), "a second observation of the same copy")
	require.NoError(t, s.registerImportedCopy(ctx, "Mcp", "imported"))
	require.NoError(t, s.registerImportedCopy(ctx, "RestApi", "imported"), "kinds with no imported copy are ignored")

	require.Equal(t, []string{"imported"}, pendingIDs(reg, platformImportedAgentProxyKind))
	require.Equal(t, []string{"imported"}, pendingIDs(reg, platformMCPKind))
	require.Len(t, reg.Pending(), 2)
}

func TestImportedAgentProxyCleanupNeedsARegistry(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.ErrorContains(t, (&Steps{}).registerImportedAgentProxy(ctx, "imported"), "no registry")
}

func TestTrackControlPlaneResourceNeedsARegistry(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	s := &Steps{}
	err := s.trackControlPlaneResource(ctx, http.MethodDelete, nil,
		&httpx.Response{StatusCode: http.StatusNoContent, URL: "https://cp/api/v0.9/agent-proxies/a"})
	require.ErrorContains(t, err, "no registry")
}

func TestRequestGatewayID(t *testing.T) {
	got, err := requestGatewayID([]byte(`{"name":"d","base":"current","gatewayId":"it-gateway"}`))
	require.NoError(t, err)
	require.Equal(t, "it-gateway", got)
	for _, body := range []string{``, `[]`, `{"gatewayId":""}`, `{"gatewayId":1}`} {
		_, err := requestGatewayID([]byte(body))
		require.Error(t, err, body)
	}
}

func TestYAMLToJSONKeepsTypedScalars(t *testing.T) {
	got, err := yamlToJSON([]byte("protocol: a2a\na2a:\n  protocolVersion: \"1.0\"\n  operationConfigs:\n    transports:\n      - protocolBinding: JSONRPC\nrewriteUrls: false\n"))
	require.NoError(t, err)
	require.JSONEq(t, `{"protocol":"a2a","a2a":{"protocolVersion":"1.0","operationConfigs":{"transports":[{"protocolBinding":"JSONRPC"}]}},"rewriteUrls":false}`, string(got))

	for _, document := range []string{"- a\n- b\n", "just a scalar\n", "a: [\n"} {
		_, err := yamlToJSON([]byte(document))
		require.Error(t, err, document)
	}
}

func TestTraverseAndRenderValue(t *testing.T) {
	doc := map[string]any{
		"spec": map[string]any{
			"a2a": map[string]any{
				"operationConfigs": map[string]any{
					"transports": []any{map[string]any{"protocolBinding": "JSONRPC"}, map[string]any{"pathPrefix": "/v1"}},
				},
				"agentCard": map[string]any{"public": map[string]any{"rewriteUrls": false}},
			},
			"matrix": []any{[]any{"x", "y"}},
		},
	}
	for path, want := range map[string]string{
		"spec.a2a.operationConfigs.transports[0].protocolBinding": "JSONRPC",
		"spec.a2a.operationConfigs.transports[1].pathPrefix":      "/v1",
		"spec.a2a.agentCard.public.rewriteUrls":                   "false",
		"spec.matrix[0][1]":                                       "y",
		"spec.a2a.agentCard.public":                               `{"rewriteUrls":false}`,
	} {
		got, ok := traverse(doc, path)
		require.True(t, ok, path)
		require.Equal(t, want, renderValue(got), path)
	}
	for _, path := range []string{
		"spec.missing", "spec.a2a.operationConfigs.transports[2]", "spec.a2a.operationConfigs.transports[-1]",
		"spec.a2a[0]", "spec.matrix[x]", "spec.matrix[0", "spec.matrix[0]junk",
	} {
		_, ok := traverse(doc, path)
		require.False(t, ok, path)
	}
	require.Equal(t, "null", renderValue(nil))
	require.Equal(t, "7", renderValue(7))
}

func TestJSONFieldEquals(t *testing.T) {
	ok := &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"status":"DEPLOYED","count":2}`)}
	require.True(t, jsonFieldEquals(ok, "status", "DEPLOYED"))
	require.True(t, jsonFieldEquals(ok, "count", "2"))
	require.False(t, jsonFieldEquals(ok, "status", "DEPLOYING"))
	require.False(t, jsonFieldEquals(ok, "missing", "DEPLOYED"))
	require.False(t, jsonFieldEquals(&httpx.Response{StatusCode: http.StatusNotFound, Body: ok.Body}, "status", "DEPLOYED"))
	require.False(t, jsonFieldEquals(&httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{`)}, "status", "DEPLOYED"))
	require.False(t, jsonFieldEquals(nil, "status", "DEPLOYED"))
}

func TestErrorCode(t *testing.T) {
	require.Equal(t, "GATEWAY_TOKEN_LIMIT_REACHED", errorCode(&httpx.Response{Body: []byte(`{"code":"GATEWAY_TOKEN_LIMIT_REACHED"}`)}))
	require.Empty(t, errorCode(&httpx.Response{Body: []byte(`not json`)}))
	require.Empty(t, errorCode(nil))
}

func TestAgentCardFixtureURLAndCount(t *testing.T) {
	require.Equal(t, "http://testbench:3013/blk/scope-1/ok", agentCardFixtureURL("http://testbench:3013/blk/", "scope-1", "ok"))

	count, err := parseFixtureCount(&httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"scope":"s1","count":3}`)}, "s1")
	require.NoError(t, err)
	require.Equal(t, 3, count)
	zero, err := parseFixtureCount(&httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"scope":"s1","count":0}`)}, "s1")
	require.NoError(t, err)
	require.Zero(t, zero)
	for _, resp := range []*httpx.Response{
		nil,
		{StatusCode: http.StatusBadRequest, Body: []byte(`{"scope":"s1","count":3}`)},
		{StatusCode: http.StatusOK, Body: []byte(`{"scope":"other","count":3}`)},
		{StatusCode: http.StatusOK, Body: []byte(`{"scope":"s1"}`)},
		{StatusCode: http.StatusOK, Body: []byte(`{`)},
	} {
		_, err := parseFixtureCount(resp, "s1")
		require.Error(t, err)
	}
}

func TestInternalArtifactID(t *testing.T) {
	listing := &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"deployments":[
		{"artifactId":"uuid-rest","deploymentId":"dep-rest","kind":"RestApi"},
		{"artifactId":"uuid-agent","deploymentId":"dep-agent","kind":"Agent"},
		{"artifactId":"","deploymentId":"dep-blank","kind":"Agent"}]}`)}
	got, err := internalArtifactID(listing, "dep-agent")
	require.NoError(t, err)
	require.Equal(t, "uuid-agent", got)

	_, err = internalArtifactID(listing, "dep-rest")
	require.ErrorContains(t, err, "want Agent", "a control-plane AgentProxy must be listed in the gateway's vocabulary")
	_, err = internalArtifactID(listing, "dep-blank")
	require.ErrorContains(t, err, "without an artifact id")
	_, err = internalArtifactID(listing, "dep-missing")
	require.ErrorContains(t, err, "does not include")
	_, err = internalArtifactID(&httpx.Response{StatusCode: http.StatusUnauthorized}, "dep-agent")
	require.Error(t, err)
	_, err = internalArtifactID(&httpx.Response{StatusCode: http.StatusOK, Body: []byte(`[`)}, "dep-agent")
	require.Error(t, err)
}

func zipArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range entries {
		file, err := writer.Create(name)
		require.NoError(t, err)
		_, err = file.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func tarGzArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	writer := tar.NewWriter(gz)
	require.NoError(t, writer.WriteHeader(&tar.Header{Name: "dep-1/", Typeflag: tar.TypeDir, Mode: 0o755}))
	for name, content := range entries {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := writer.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

const agentDeploymentYAML = "apiVersion: gateway.api-platform.wso2.com/v1\nkind: Agent\nspec:\n  a2a:\n    protocolVersion: \"1.0\"\n"

func publishedBody(t *testing.T, body []byte) context.Context {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{StatusCode: http.StatusOK, Body: body}))
	return ctx
}

func TestArchiveAssertions(t *testing.T) {
	s := &Steps{}
	t.Run("zip", func(t *testing.T) {
		ctx := publishedBody(t, zipArchive(t, map[string]string{"agent-u1.yaml": agentDeploymentYAML}))
		require.NoError(t, s.archiveContainsOnly(ctx, "ZIP", "agent-u1.yaml"))
		require.ErrorContains(t, s.archiveContainsOnly(ctx, "ZIP", "agent-u2.yaml"), "want only")
		require.Error(t, s.archiveContainsOnly(ctx, "gzip tar", "agent-u1.yaml"))
		require.NoError(t, s.archivedFieldIs(ctx, "kind", "Agent"))
		require.NoError(t, s.archivedFieldIs(ctx, "spec.a2a.protocolVersion", "1.0"))
		require.ErrorContains(t, s.archivedFieldIs(ctx, "kind", "AgentProxy"), `expected "AgentProxy"`)
		require.ErrorContains(t, s.archivedFieldIs(ctx, "spec.protocol", "a2a"), "has no field")
		require.NoError(t, s.archivedFieldAbsent(ctx, "spec.protocol"))
		require.ErrorContains(t, s.archivedFieldAbsent(ctx, "kind"), "should be absent")
	})
	t.Run("gzip tar", func(t *testing.T) {
		ctx := publishedBody(t, tarGzArchive(t, map[string]string{"dep-1/agent-u1.yaml": agentDeploymentYAML}))
		require.NoError(t, s.archiveContainsOnly(ctx, "gzip tar", "dep-1/agent-u1.yaml"), "directory entries are not documents")
		require.NoError(t, s.archivedFieldIs(ctx, "kind", "Agent"))
	})
	t.Run("more than one document", func(t *testing.T) {
		ctx := publishedBody(t, zipArchive(t, map[string]string{"a.yaml": agentDeploymentYAML, "b.yaml": agentDeploymentYAML}))
		require.ErrorContains(t, s.archiveContainsOnly(ctx, "ZIP", "a.yaml"), "want only")
		require.ErrorContains(t, s.archivedFieldIs(ctx, "kind", "Agent"), "want exactly one")
	})
	t.Run("not an archive", func(t *testing.T) {
		ctx := publishedBody(t, []byte(`{"code":404}`))
		require.ErrorContains(t, s.archiveContainsOnly(ctx, "ZIP", "a.yaml"), "not a ZIP")
		require.Error(t, s.archivedFieldIs(ctx, "kind", "Agent"))
		_, err := archiveEntries("rar", nil)
		require.ErrorContains(t, err, "unsupported archive format")
	})
	t.Run("not YAML", func(t *testing.T) {
		ctx := publishedBody(t, zipArchive(t, map[string]string{"agent.yaml": "kind: [\n"}))
		require.ErrorContains(t, s.archivedFieldIs(ctx, "kind", "Agent"), "is not YAML")
	})
}

func TestAgentProxyLocationPattern(t *testing.T) {
	for raw, want := range map[string][]string{
		"/api/v0.9/agent-proxies/weather":                          {"weather", "", ""},
		"https://cp/api/v0.9/agent-proxies/weather/deployments/d1": {"weather", "deployments", "d1"},
		"/api/v0.9/agent-proxies/weather/api-keys/k1?x=1":          {"weather", "api-keys", "k1"},
	} {
		match := agentProxyLocationPattern.FindStringSubmatch(locationPath(raw))
		require.NotNil(t, match, raw)
		require.Equal(t, want, match[1:], raw)
	}
	for _, raw := range []string{"", "/api/v0.9/agent-proxies", "/api/v0.9/agent-proxies/", "/api/v0.9/mcp-proxies/a",
		"/api/v0.9/agent-proxies/a/deployments", "/api/v0.9/agent-proxies/a/deployments/d/undeploy", "%zz"} {
		require.Nil(t, agentProxyLocationPattern.FindStringSubmatch(locationPath(raw)), raw)
	}
	require.True(t, strings.HasPrefix(locationPath("https://cp/api/v0.9/agent-proxies/a%2Fb"), "/api/v0.9/agent-proxies/a/b"))
}

func valuesTable(rows ...[]string) *godog.Table {
	type cell struct {
		Value string `json:"value"`
	}
	type tableRow struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []tableRow `json:"rows"`
	}{}
	for _, row := range rows {
		encoded := tableRow{}
		for _, value := range row {
			encoded.Cells = append(encoded.Cells, cell{Value: value})
		}
		payload.Rows = append(payload.Rows, encoded)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("marshal table fixture: %v", err))
	}
	var table godog.Table
	if err := json.Unmarshal(raw, &table); err != nil {
		panic(fmt.Sprintf("unmarshal table fixture: %v", err))
	}
	return &table
}

func TestPublicationTypeAndPath(t *testing.T) {
	apiType, err := publicationType("RestApi")
	require.NoError(t, err)
	require.Equal(t, "rest-api", apiType)
	for _, kind := range []string{"Mcp", "LlmProvider", "", "restapi"} {
		_, err := publicationType(kind)
		require.ErrorContains(t, err, "not supported", kind)
	}

	require.Equal(t, "/api-portals/portal-1/apis/rest-api/api-1", publicationPath("portal-1", "rest-api", "api-1"))
	require.Equal(t, "/api-portals/a%2Fb/apis/rest-api/c%20d", publicationPath("a/b", "rest-api", "c d"),
		"handles are escaped so one cannot address another path")
}

func TestSplitPublicationID(t *testing.T) {
	portal, apiType, api, err := splitPublicationID("portal-1/rest-api/api-1")
	require.NoError(t, err)
	require.Equal(t, "portal-1", portal)
	require.Equal(t, "rest-api", apiType)
	require.Equal(t, "api-1", api)

	for _, id := range []string{"", "portal-1", "portal-1/api-1", "/rest-api/api-1", "portal-1//api-1", "portal-1/rest-api/", "a/b/c/d"} {
		_, _, _, err := splitPublicationID(id)
		require.ErrorContains(t, err, "malformed publication id", id)
	}
}

func TestPublicationCleanupOrderUnpublishesBeforeTheAPIAndPortalAreDeleted(t *testing.T) {
	require.Greater(t, platformPublicationKind.Order, platformDeploymentKind.Order)
	require.Less(t, platformPublicationKind.Order, platformRESTAPIKind.Order,
		"the listing is unpublished while the API still exists")
	require.Greater(t, platformAPIPortalKind.Order, platformPublicationKind.Order,
		"the portal registration outlives the unpublish that needs it")
	require.Greater(t, platformAPIPortalKind.Order, platformRESTAPIKind.Order)
	require.Less(t, platformAPIPortalKind.Order, platformGatewayKind.Order)
}

func TestRegisterPublicationRecordsThePortalAndAPIForCleanup(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}

	require.NoError(t, s.registerPublication(ctx, "portal-1", "rest-api", "api-1"))
	require.NoError(t, s.registerPublication(ctx, "portal-1", "rest-api", "api-2"))
	require.ElementsMatch(t, []string{"portal-1/rest-api/api-1", "portal-1/rest-api/api-2"}, pendingIDs(reg, platformPublicationKind))
}

func TestDraftFromTable(t *testing.T) {
	ctx, _ := trackingContext(t)
	local, _ := tcontext.LocalOf(ctx)
	local.Set("planHandle", "plan-1")

	body, err := draftFromTable(ctx, valuesTable(
		[]string{"displayName", "Weather"},
		[]string{"version", "v1.0.3"},
		[]string{"subscriptionPlanIds", `["${CTX:planHandle}"]`},
	))
	require.NoError(t, err)
	require.JSONEq(t, `{"displayName":"Weather","version":"v1.0.3","subscriptionPlanIds":["plan-1"]}`, string(body),
		"a JSON value is decoded and a field the table does not name is absent")

	for name, table := range map[string]*godog.Table{
		"nil table":     nil,
		"empty table":   valuesTable(),
		"three cells":   valuesTable([]string{"a", "b", "c"}),
		"empty key":     valuesTable([]string{" ", "b"}),
		"duplicate key": valuesTable([]string{"a", "1"}, []string{"a", "2"}),
	} {
		_, err := draftFromTable(ctx, table)
		require.Error(t, err, name)
	}

	body, err = draftFromTable(ctx, valuesTable(
		[]string{"displayName", "[Beta] Orders API"},
		[]string{"description", `{not json}`},
	))
	require.NoError(t, err)
	require.JSONEq(t, `{"displayName":"[Beta] Orders API","description":"{not json}"}`, string(body),
		"text that starts with a bracket but is not JSON stays a string")
}

func TestMinimalOpenAPIDefinitionIsAValidDocument(t *testing.T) {
	body, err := minimalOpenAPIDefinition("weather")
	require.NoError(t, err)

	var document struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]any `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(body, &document))
	require.Equal(t, "3.0.3", document.OpenAPI)
	require.Equal(t, "weather", document.Info.Title)
	require.NotEmpty(t, document.Info.Version)
	require.NotEmpty(t, document.Paths)
}

func TestSelectedPlans(t *testing.T) {
	require.Equal(t, []string{"gold", "silver"}, selectedPlans([]byte(`{"displayName":"x","subscriptionPlanIds":["gold","silver"]}`)))
	for name, draft := range map[string]string{
		"no plans field":      `{"displayName":"x"}`,
		"empty list":          `{"subscriptionPlanIds":[]}`,
		"plans not a list":    `{"subscriptionPlanIds":"gold"}`,
		"not a JSON document": `not json`,
	} {
		require.Empty(t, selectedPlans([]byte(draft)), name)
	}
}

func TestTrackSelectedPlansRemovesEverySelectedPlanFromThePortal(t *testing.T) {
	var tracked []string
	s := &Steps{trackPortalPlan: func(_ context.Context, handle string) error {
		tracked = append(tracked, handle)
		return nil
	}}

	require.NoError(t, s.trackSelectedPlans(context.Background(), []byte(`{"subscriptionPlanIds":["gold","silver"]}`)))
	require.Equal(t, []string{"gold", "silver"}, tracked)

	tracked = nil
	require.NoError(t, s.trackSelectedPlans(context.Background(), []byte(`{"displayName":"x"}`)))
	require.Empty(t, tracked, "a draft that selects no plan tracks nothing")

	failing := &Steps{trackPortalPlan: func(context.Context, string) error { return errors.New("registry unavailable") }}
	require.ErrorContains(t, failing.trackSelectedPlans(context.Background(), []byte(`{"subscriptionPlanIds":["gold"]}`)),
		`plan "gold"`)

	untracked := &Steps{}
	require.ErrorContains(t, untracked.trackSelectedPlans(context.Background(), []byte(`{"subscriptionPlanIds":["gold"]}`)),
		"no API Portal steps")
	require.NoError(t, untracked.trackSelectedPlans(context.Background(), []byte(`{}`)),
		"without selected plans there is nothing to track")
}

func TestUnpublishSettled(t *testing.T) {
	stateConflict := []byte(`{"status":"error","code":"PUBLICATION_STATE_CONFLICT"}`)
	portalConflict := []byte(`{"status":"error","code":"PUBLICATION_PORTAL_CONFLICT"}`)

	for name, tc := range map[string]struct {
		status int
		body   []byte
		want   bool
	}{
		"unpublished":                      {http.StatusNoContent, nil, true},
		"unknown API":                      {http.StatusNotFound, nil, true},
		"nothing to unpublish":             {http.StatusConflict, stateConflict, true},
		"portal still holds keys":          {http.StatusConflict, portalConflict, false},
		"conflict without a readable body": {http.StatusConflict, []byte(`not json`), false},
		"conflict with an empty body":      {http.StatusConflict, nil, false},
		"portal unavailable":               {http.StatusServiceUnavailable, nil, false},
		"forbidden":                        {http.StatusForbidden, nil, false},
	} {
		require.Equal(t, tc.want, unpublishSettled(&httpx.Response{StatusCode: tc.status, Body: tc.body}), name)
	}
}

func TestDocumentIDCarriesTheAPI(t *testing.T) {
	id := documentID("rest-api", "api-1", "doc-1")
	require.Equal(t, "rest-api/api-1/doc-1", id)

	apiType, api, doc, err := splitDocumentID(id)
	require.NoError(t, err)
	require.Equal(t, []string{"rest-api", "api-1", "doc-1"}, []string{apiType, api, doc})
	require.NotEqual(t, documentID("rest-api", "api-1", "doc-1"), documentID("rest-api", "api-2", "doc-1"),
		"the same document handle on two APIs must be two cleanup resources")

	for _, bad := range []string{"", "doc-1", "api-1/doc-1", "rest-api//doc-1", "rest-api/api-1/", "a/b/c/d"} {
		_, _, _, err := splitDocumentID(bad)
		require.ErrorContains(t, err, "malformed document id", bad)
	}
}

// A block whose scenarios register gateways of their own must still deploy to its own gateway,
// so the block's gateway is fetched by handle rather than taken from the head of the list.
func TestGatewayUUIDResolvesTheBlockGatewayAmongOthers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiBase + "/gateways":
			_, _ = w.Write([]byte(`{"list":[{"id":"tok-gw-abc"},{"id":"it-gateway"}]}`))
		case apiBase + "/gateways/" + blockGatewayHandle:
			_, _ = w.Write([]byte(`{"id":"it-gateway"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	s := &Steps{client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}
	id, err := s.gatewayUUID(context.Background(), server.URL, "token")
	require.NoError(t, err)
	require.Equal(t, "it-gateway", id)
}

func TestCallHeaders(t *testing.T) {
	ctx, _ := trackingContext(t)
	local, _ := tcontext.LocalOf(ctx)
	local.Set("origin", "cross-site")

	headers, err := callHeaders(ctx, controlPlaneCall{
		body:    []byte(`{}`),
		headers: map[string]string{"Accept": "application/json", " Sec-Fetch-Site ": "${CTX:origin}"},
	}, "tok")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"Authorization": "Bearer tok", "Content-Type": "application/json",
		"Accept": "application/json", "Sec-Fetch-Site": "cross-site",
	}, headers)

	headers, err = callHeaders(ctx, controlPlaneCall{}, "")
	require.NoError(t, err)
	require.Empty(t, headers, "no token, no body and no extra header sends nothing")

	_, err = callHeaders(ctx, controlPlaneCall{headers: map[string]string{"X": "${CTX:missing}"}}, "tok")
	require.Error(t, err, "an unresolved placeholder in a header value is an error")
}

func TestCallTokenUsesTheCallsOwnBearer(t *testing.T) {
	s := &Steps{}
	for _, token := range []string{"", "garbage"} {
		got, err := s.callToken(context.Background(), "https://unused.invalid", controlPlaneCall{bearer: &token}, actor.Credentials{})
		require.NoError(t, err)
		require.Equal(t, token, got, "an own bearer is used as it is, even when empty, without a login")
	}
}

func TestMCPToolCallBody(t *testing.T) {
	body, err := mcpToolCallBody("list_projects", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}`, string(body))

	body, err = mcpToolCallBody("get_api", []byte(`{"apiType":"rest-api","apiId":"a1"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_api","arguments":{"apiType":"rest-api","apiId":"a1"}}}`, string(body))

	body, err = mcpToolCallBody("list_projects", []byte("  "))
	require.NoError(t, err, "blank arguments mean none")
	require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}`, string(body))

	for name, arguments := range map[string]string{
		"not JSON":  `{"apiType":`,
		"an array":  `[1]`,
		"a string":  `"x"`,
		"JSON null": `null`,
		"a number":  `7`,
	} {
		_, err := mcpToolCallBody("get_api", []byte(arguments))
		require.ErrorContains(t, err, "not a JSON object", name)
	}
}

func mcpResponse(status int, body string) *httpx.Response {
	return &httpx.Response{StatusCode: status, Body: []byte(body)}
}

func TestMCPToolSucceeded(t *testing.T) {
	require.NoError(t, mcpToolSucceeded(mcpResponse(http.StatusOK, `{"result":{"structuredContent":{"id":"p1"}}}`)))

	for name, resp := range map[string]*httpx.Response{
		"nil":               nil,
		"unauthorized":      mcpResponse(http.StatusUnauthorized, `{}`),
		"not JSON":          mcpResponse(http.StatusOK, `nope`),
		"JSON-RPC error":    mcpResponse(http.StatusOK, `{"error":{"code":-32602,"message":"unknown tool"}}`),
		"tool error":        mcpResponse(http.StatusOK, `{"result":{"isError":true,"content":[{"text":"no"}]}}`),
		"neither field set": mcpResponse(http.StatusOK, `{}`),
	} {
		require.Error(t, mcpToolSucceeded(resp), name)
	}
}

func TestMCPToolFailedWith(t *testing.T) {
	failure := mcpResponse(http.StatusOK, `{"result":{"isError":true,"content":[{"type":"text","text":"The specified gateway could not be found."}]}}`)
	require.NoError(t, mcpToolFailedWith(failure, "The specified gateway could not be found."))
	require.ErrorContains(t, mcpToolFailedWith(failure, "The specified gateway"), "expected \"The specified gateway\"",
		"a sentence must match exactly, not as a prefix")

	for name, resp := range map[string]*httpx.Response{
		"success":        mcpResponse(http.StatusOK, `{"result":{"structuredContent":{}}}`),
		"JSON-RPC error": mcpResponse(http.StatusOK, `{"error":{"message":"unknown tool"}}`),
		"no content":     mcpResponse(http.StatusOK, `{"result":{"isError":true}}`),
		"forbidden":      mcpResponse(http.StatusForbidden, ``),
	} {
		require.Error(t, mcpToolFailedWith(resp, "x"), name)
	}
}

func TestMCPToolNamesAndSameToolNames(t *testing.T) {
	got, err := mcpToolNames(mcpResponse(http.StatusOK, `{"result":{"tools":[{"name":"get_api"},{"name":"list_apis"},{"name":"get_api_schema"}]}}`))
	require.NoError(t, err)
	require.Equal(t, []string{"get_api", "get_api_schema", "list_apis"}, got, "names are sorted")

	require.NoError(t, sameToolNames(got, "list_apis, get_api,get_api_schema"), "order and spacing do not matter")
	require.ErrorContains(t, sameToolNames(got, "get_api, list_apis"), "unexpected [get_api_schema]")
	require.ErrorContains(t, sameToolNames(got, "get_api, get_api_schema, list_apis, deploy_api"), "missing [deploy_api]")
	require.Error(t, sameToolNames(nil, "get_api"))
	require.NoError(t, sameToolNames([]string{}, ""), "an empty list matches an empty set")

	empty, err := mcpToolNames(mcpResponse(http.StatusOK, `{"result":{"tools":[]}}`))
	require.NoError(t, err)
	require.Empty(t, empty)

	for name, resp := range map[string]*httpx.Response{
		"unauthorized":   mcpResponse(http.StatusUnauthorized, ``),
		"JSON-RPC error": mcpResponse(http.StatusOK, `{"error":{"message":"bad"}}`),
		"no result":      mcpResponse(http.StatusOK, `{}`),
	} {
		_, err := mcpToolNames(resp)
		require.Error(t, err, name)
	}
}

// trackMCP calls trackMCPToolResult with a sender that fails the test when it is used.
func trackMCP(t *testing.T, ctx context.Context, s *Steps, tool string, arguments []byte, resp *httpx.Response) error {
	t.Helper()
	return s.trackMCPToolResult(ctx, "", tool, arguments, resp,
		func(context.Context, string, []byte) (*httpx.Response, error) {
			t.Errorf("unexpected compensating call after %s", tool)
			return nil, errors.New("unexpected call")
		})
}

func TestTrackMCPToolResultRegistersAndDropsCleanupRecords(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	ok := func(structured string) *httpx.Response {
		return mcpResponse(http.StatusOK, `{"result":{"structuredContent":`+structured+`}}`)
	}

	require.NoError(t, trackMCP(t, ctx, s, "create_project", []byte(`{"name":"p"}`), ok(`{"id":"proj-1"}`)))
	require.Equal(t, []string{"proj-1"}, pendingIDs(reg, platformProjectKind))

	for apiType, kind := range map[string]string{
		"rest-api": platformRESTAPIKind.Name, "llm-provider": platformProviderKind.Name,
		"llm-proxy": platformProxyKind.Name, "mcp-proxy": platformMCPKind.Name,
	} {
		require.NoError(t, trackMCP(t, ctx, s, "create_api", []byte(`{"apiType":"`+apiType+`","payload":{}}`),
			ok(`{"id":"`+apiType+`-1"}`)), apiType)
		var ids []string
		for _, res := range reg.Pending() {
			if res.Kind.Name == kind {
				ids = append(ids, res.ID)
			}
		}
		require.Equal(t, []string{apiType + "-1"}, ids, apiType)
	}

	require.NoError(t, trackMCP(t, ctx, s, "deploy_api",
		[]byte(`{"apiType":"rest-api","apiId":"rest-api-1","gatewayId":"it-gateway"}`),
		ok(`{"apiId":"rest-api-1","deployments":[{"id":"dep-1","status":"DEPLOYING"}]}`)))
	require.Equal(t, []string{"rest-apis\x00rest-api-1\x00dep-1\x00it-gateway"}, pendingIDs(reg, platformDeploymentKind))

	require.NoError(t, trackMCP(t, ctx, s, "delete_api",
		[]byte(`{"apiType":"rest-api","apiId":"rest-api-1","confirm":true}`), ok(`{"apiType":"rest-api","deleted":"rest-api-1"}`)))
	require.Empty(t, pendingIDs(reg, platformRESTAPIKind))
	require.NoError(t, trackMCP(t, ctx, s, "delete_project", []byte(`{"projectId":"proj-1","confirm":true}`), ok(`{"deleted":"proj-1"}`)))
	require.Empty(t, pendingIDs(reg, platformProjectKind))
}

func TestTrackMCPToolResultLeavesAFailedOrReadOnlyCallAlone(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}

	for name, resp := range map[string]*httpx.Response{
		"nil":            nil,
		"unauthorized":   mcpResponse(http.StatusUnauthorized, ``),
		"not JSON":       mcpResponse(http.StatusOK, `nope`),
		"JSON-RPC error": mcpResponse(http.StatusOK, `{"error":{"message":"unknown tool"}}`),
		"tool error":     mcpResponse(http.StatusOK, `{"result":{"isError":true,"content":[{"text":"no"}]}}`),
	} {
		require.NoError(t, trackMCP(t, ctx, s, "create_project", []byte(`{"name":"p"}`), resp), name)
	}
	require.NoError(t, trackMCP(t, ctx, s, "list_projects", []byte(`{}`),
		mcpResponse(http.StatusOK, `{"result":{"structuredContent":{"items":[],"total":0}}}`)))
	require.Empty(t, reg.Pending())
}

func TestTrackMCPToolResultRefusesWhatItCannotCleanUp(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	ok := func(structured string) *httpx.Response {
		return mcpResponse(http.StatusOK, `{"result":{"structuredContent":`+structured+`}}`)
	}

	require.ErrorContains(t, trackMCP(t, ctx, s, "create_project", []byte(`{"name":"p"}`), ok(`{}`)), "no id")
	require.ErrorContains(t, trackMCP(t, ctx, s, "create_api", []byte(`{"apiType":"rest-api"}`), ok(`{}`)), "no id")
	require.ErrorContains(t, trackMCP(t, ctx, s, "deploy_api", []byte(`{"apiType":"rest-api","apiId":"a","gatewayId":"g"}`),
		ok(`{"deployments":[]}`)), "no deployment id")
	require.ErrorContains(t, trackMCP(t, ctx, s, "deploy_api", []byte(`{"apiType":"rest-api","apiId":"a"}`),
		ok(`{"deployments":[{"id":"d"}]}`)), "names no gatewayId")
	require.Empty(t, reg.Pending())
}

// Each undo call is the one a user would make, and the same user makes it.
func TestTrackMCPToolResultUndoesWhatItCannotTrack(t *testing.T) {
	ok := func(structured string) *httpx.Response {
		return mcpResponse(http.StatusOK, `{"result":{"structuredContent":`+structured+`}}`)
	}
	for name, test := range map[string]struct {
		tool, arguments, structured, undo, undoArguments string
	}{
		"create": {
			"create_api", `{"apiType":"agent-proxy","payload":{}}`, `{"id":"new-1"}`,
			"delete_api", `{"apiType":"agent-proxy","apiId":"new-1","confirm":true}`,
		},
		"deploy": {
			"deploy_api", `{"apiType":"agent-proxy","apiId":"a-1","gatewayId":"gw-1"}`, `{"deployments":[{"id":"d-1"}]}`,
			"undeploy_api", `{"apiType":"agent-proxy","apiId":"a-1","gatewayId":"gw-1","confirm":true}`,
		},
	} {
		ctx, reg := trackingContext(t)
		var who string
		var sent [][]byte
		send := func(_ context.Context, user string, body []byte) (*httpx.Response, error) {
			who = user
			sent = append(sent, body)
			return ok(`{}`), nil
		}
		err := (&Steps{}).trackMCPToolResult(ctx, "publisher", test.tool, []byte(test.arguments), ok(test.structured), send)
		require.ErrorContains(t, err, `no cleanup is defined for apiType "agent-proxy"`, name)
		require.NotContains(t, err.Error(), "compensating", name)
		require.Equal(t, "publisher", who, name)
		wantBody, bodyErr := mcpToolCallBody(test.undo, []byte(test.undoArguments))
		require.NoError(t, bodyErr, name)
		require.Len(t, sent, 1, name)
		require.JSONEq(t, string(wantBody), string(sent[0]), name)
		require.Empty(t, reg.Pending(), name)
	}
}

func TestTrackMCPToolResultReportsAFailedUndo(t *testing.T) {
	ctx, _ := trackingContext(t)
	ok := mcpResponse(http.StatusOK, `{"result":{"structuredContent":{"id":"new-1"}}}`)
	arguments := []byte(`{"apiType":"agent-proxy","payload":{}}`)

	for name, send := range map[string]mcpSender{
		"transport error": func(context.Context, string, []byte) (*httpx.Response, error) { return nil, errors.New("boom") },
		"tool error": func(context.Context, string, []byte) (*httpx.Response, error) {
			return mcpResponse(http.StatusOK, `{"result":{"isError":true,"content":[{"text":"no"}]}}`), nil
		},
	} {
		err := (&Steps{}).trackMCPToolResult(ctx, "", "create_api", arguments, ok, send)
		require.ErrorContains(t, err, `no cleanup is defined for apiType "agent-proxy"`, name)
		require.ErrorContains(t, err, "compensating call failed: delete_api", name)
	}

	err := (&Steps{}).trackMCPToolResult(ctx, "", "delete_api", []byte(`{"apiType":"agent-proxy","apiId":"a","confirm":true}`),
		mcpResponse(http.StatusOK, `{"result":{"structuredContent":{"deleted":"a"}}}`),
		func(context.Context, string, []byte) (*httpx.Response, error) {
			t.Error("a delete leaves nothing to undo")
			return nil, errors.New("unexpected call")
		})
	require.ErrorContains(t, err, "no cleanup is defined")
}
