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
	"log/slog"
	"net/http"
	"strings"
	"testing"

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

func TestTrackAgentProxyResourceRegistersWhatTheControlPlaneCreated(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}

	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather")))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, []byte(`{"name":"d","gatewayId":"it-gateway"}`),
		created("/api/v0.9/agent-proxies/weather/deployments/dep-1")))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, nil,
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
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather")))
	require.Len(t, pendingIDs(reg, platformAgentProxyKind), 1)
}

func TestTrackAgentProxyResourceIgnoresWhatIsNotAnAgentProxyCreate(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	for _, tc := range []struct {
		method string
		resp   *httpx.Response
	}{
		{http.MethodPost, &httpx.Response{StatusCode: http.StatusOK, Headers: http.Header{"Location": []string{"/api/v0.9/agent-proxies/a"}}}},
		{http.MethodPost, &httpx.Response{StatusCode: http.StatusBadRequest}},
		{http.MethodPost, created("/api/v0.9/rest-apis/a")},
		{http.MethodPost, created("")},
		{http.MethodPut, created("/api/v0.9/agent-proxies/a")},
		{http.MethodPost, created("/api/v0.9/agent-proxies/a/builds/b")},
	} {
		require.NoError(t, s.trackAgentProxyResource(ctx, tc.method, nil, tc.resp))
	}
	require.Empty(t, reg.Pending())
}

func TestTrackAgentProxyResourceRequiresTheDeploymentGateway(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	for _, body := range [][]byte{nil, []byte(`{`), []byte(`{"name":"d"}`), []byte(`{"gatewayId":"  "}`)} {
		err := s.trackAgentProxyResource(ctx, http.MethodPost, body, created("/api/v0.9/agent-proxies/a/deployments/d"))
		require.Error(t, err, string(body))
	}
	require.Empty(t, reg.Pending())
}

func TestTrackAgentProxyResourceDeregistersWhatTheScenarioDeleted(t *testing.T) {
	ctx, reg := trackingContext(t)
	s := &Steps{}
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather")))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/other")))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
		created("/api/v0.9/agent-proxies/weather/deployments/dep-1")))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, []byte(`{"gatewayId":"gw"}`),
		created("/api/v0.9/agent-proxies/weather/deployments/dep-12")))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodPost, nil, created("/api/v0.9/agent-proxies/weather/api-keys/k")))

	deleted := func(url string) *httpx.Response {
		return &httpx.Response{StatusCode: http.StatusNoContent, URL: url}
	}
	// A failed delete leaves the record in place.
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodDelete, nil,
		&httpx.Response{StatusCode: http.StatusConflict, URL: "https://cp/api/v0.9/agent-proxies/weather/deployments/dep-1"}))
	require.Len(t, pendingIDs(reg, platformAgentDeploymentKind), 2)

	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/agent-proxies/weather/deployments/dep-1")))
	require.Equal(t, []string{"weather\x00dep-12\x00gw"}, pendingIDs(reg, platformAgentDeploymentKind),
		"only the deleted deployment is dropped, not one whose id shares its prefix")
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/agent-proxies/weather/api-keys/k")))
	require.Empty(t, pendingIDs(reg, platformAgentAPIKeyKind))
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodDelete, nil,
		deleted("https://cp:9243/api/v0.9/agent-proxies/weather?force=true")))
	require.Equal(t, []string{"other"}, pendingIDs(reg, platformAgentProxyKind))
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
	require.NoError(t, s.trackAgentProxyResource(ctx, http.MethodDelete, nil,
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

func TestTrackAgentProxyResourceNeedsARegistry(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	s := &Steps{}
	err := s.trackAgentProxyResource(ctx, http.MethodDelete, nil,
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
