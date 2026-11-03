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

package utils

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const testAgentID = "0f8e7d6c-5b4a-4392-8a1b-0c9d8e7f6a5b"

// agentZip builds an archive with the given entries, in order.
func agentZip(t *testing.T, entries map[string]string, order ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range order {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(entries[name]))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// controlPlaneAgentZip is the archive the control plane's gateway-internal API
// serves for testAgentID: one entry, agent-{id}.yaml.
func controlPlaneAgentZip(t *testing.T) []byte {
	name := "agent-" + testAgentID + ".yaml"
	return agentZip(t, map[string]string{name: "kind: Agent\n"}, name)
}

type agentFetchResponse struct {
	status      int
	contentType string
	// noContentType sends no Content-Type at all. Leaving contentType empty is
	// not enough: net/http would sniff one from the body.
	noContentType bool
	body          []byte
}

// newAgentFetchService stands up a control plane that answers every request with
// resp, and returns a service pointed at it plus a counter of requests received.
func newAgentFetchService(t *testing.T, resp agentFetchResponse, maxBytes int64) (*APIUtilsService, *int32, *http.Request) {
	t.Helper()
	var calls int32
	captured := &http.Request{}
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		*captured = *r.Clone(r.Context())
		switch {
		case resp.noContentType:
			w.Header()["Content-Type"] = nil
		case resp.contentType != "":
			w.Header().Set("Content-Type", resp.contentType)
		}
		w.WriteHeader(resp.status)
		_, _ = w.Write(resp.body)
	}))
	t.Cleanup(server.Close)

	svc := NewAPIUtilsService(PlatformAPIConfig{
		BaseURL:          server.URL + "/api/internal/v1",
		Token:            "gw-token",
		MaxResponseBytes: maxBytes,
	}, testHTTPClient(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, &calls, captured
}

func TestFetchAgentDefinition_ReturnsControlPlaneArtifact(t *testing.T) {
	zipData := controlPlaneAgentZip(t)
	svc, calls, req := newAgentFetchService(t, agentFetchResponse{
		status: http.StatusOK, contentType: "application/zip", body: zipData,
	}, 0)

	got, err := svc.FetchAgentDefinition(testAgentID)
	require.NoError(t, err)
	assert.Equal(t, zipData, got)

	// The gateway-internal route, keyed by artifact UUID, authenticated with the
	// gateway's API key — never a publisher endpoint or OAuth token.
	assert.EqualValues(t, 1, atomic.LoadInt32(calls))
	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/api/internal/v1/agents/"+testAgentID, req.URL.Path)
	assert.Equal(t, "gw-token", req.Header.Get("api-key"))
	assert.Equal(t, "application/zip", req.Header.Get("Accept"))
	assert.Empty(t, req.Header.Get("Authorization"))

	// And the result is what the deploy path extracts next.
	yamlData, err := svc.ExtractYAMLFromZip(got)
	require.NoError(t, err)
	assert.Equal(t, "kind: Agent\n", string(yamlData))
}

// A media type with parameters is still a ZIP.
func TestFetchAgentDefinition_AcceptsZipMediaTypeWithParameters(t *testing.T) {
	svc, _, _ := newAgentFetchService(t, agentFetchResponse{
		status: http.StatusOK, contentType: "application/zip; charset=binary", body: controlPlaneAgentZip(t),
	}, 0)

	_, err := svc.FetchAgentDefinition(testAgentID)
	require.NoError(t, err)
}

// The id is interpolated into the request path, so anything but a canonical
// UUID is refused before a request is made — a crafted id must not be able to
// steer the fetch to another internal resource.
func TestFetchAgentDefinition_RefusesNonUUIDBeforeRequesting(t *testing.T) {
	ids := map[string]string{
		"empty":            "",
		"public handle":    "weather-agent",
		"path traversal":   "../apis/" + testAgentID,
		"encoded slash":    "..%2Fapis%2F" + testAgentID,
		"query injection":  testAgentID + "?x=1",
		"braced form":      "{" + testAgentID + "}",
		"urn form":         "urn:uuid:" + testAgentID,
		"unhyphenated":     strings.ReplaceAll(testAgentID, "-", ""),
		"trailing segment": testAgentID + "/api-keys",
	}
	for name, id := range ids {
		t.Run(name, func(t *testing.T) {
			svc, calls, _ := newAgentFetchService(t, agentFetchResponse{
				status: http.StatusOK, contentType: "application/zip", body: controlPlaneAgentZip(t),
			}, 0)

			_, err := svc.FetchAgentDefinition(id)
			require.ErrorIs(t, err, ErrInvalidAgentArtifact)
			assert.Zero(t, atomic.LoadInt32(calls), "no request may be built from an invalid id")
		})
	}
}

// Every way the fetch can come back wrong is an error, so the deploy path acks a
// failure instead of applying whatever arrived.
func TestFetchAgentDefinition_RejectsWrongResponses(t *testing.T) {
	otherID := "11111111-2222-4333-8444-555555555555"
	cases := map[string]struct {
		resp           agentFetchResponse
		wantInvalid    bool
		wantErrContain string
	}{
		"not deployed on this gateway": {
			resp:           agentFetchResponse{status: http.StatusNotFound, contentType: "application/json", body: []byte(`{"code":404}`)},
			wantErrContain: "status 404",
		},
		"unauthorized": {
			resp:           agentFetchResponse{status: http.StatusUnauthorized, contentType: "application/json", body: []byte(`{"code":401}`)},
			wantErrContain: "status 401",
		},
		"server error": {
			resp:           agentFetchResponse{status: http.StatusInternalServerError, body: []byte("boom")},
			wantErrContain: "status 500",
		},
		"json instead of zip": {
			resp:        agentFetchResponse{status: http.StatusOK, contentType: "application/json", body: []byte(`{"kind":"Agent"}`)},
			wantInvalid: true,
		},
		"missing content type": {
			resp:        agentFetchResponse{status: http.StatusOK, noContentType: true, body: controlPlaneAgentZip(t)},
			wantInvalid: true,
		},
		"zip content type, not a zip": {
			resp:        agentFetchResponse{status: http.StatusOK, contentType: "application/zip", body: []byte("kind: Agent\n")},
			wantInvalid: true,
		},
		"empty archive": {
			resp:        agentFetchResponse{status: http.StatusOK, contentType: "application/zip", body: agentZip(t, nil)},
			wantInvalid: true,
		},
		"archive for another agent": {
			resp: agentFetchResponse{status: http.StatusOK, contentType: "application/zip",
				body: agentZip(t, map[string]string{"agent-" + otherID + ".yaml": "kind: Agent\n"}, "agent-"+otherID+".yaml")},
			wantInvalid: true,
		},
		"archive of another kind": {
			resp: agentFetchResponse{status: http.StatusOK, contentType: "application/zip",
				body: agentZip(t, map[string]string{"mcp-proxy-" + testAgentID + ".yaml": "kind: Mcp\n"}, "mcp-proxy-"+testAgentID+".yaml")},
			wantInvalid: true,
		},
		"extra entries": {
			resp: agentFetchResponse{status: http.StatusOK, contentType: "application/zip",
				body: agentZip(t, map[string]string{
					"agent-" + testAgentID + ".yaml": "kind: Agent\n",
					"extra.yaml":                     "kind: RestApi\n",
				}, "agent-"+testAgentID+".yaml", "extra.yaml")},
			wantInvalid: true,
		},
		"entry in a directory": {
			resp: agentFetchResponse{status: http.StatusOK, contentType: "application/zip",
				body: agentZip(t, map[string]string{"x/agent-" + testAgentID + ".yaml": "kind: Agent\n"}, "x/agent-"+testAgentID+".yaml")},
			wantInvalid: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _, _ := newAgentFetchService(t, tc.resp, 0)

			got, err := svc.FetchAgentDefinition(testAgentID)
			require.Error(t, err)
			assert.Nil(t, got)
			if tc.wantInvalid {
				assert.ErrorIs(t, err, ErrInvalidAgentArtifact)
			}
			if tc.wantErrContain != "" {
				assert.Contains(t, err.Error(), tc.wantErrContain)
			}
		})
	}
}

// The response is read against the configured ceiling, not buffered unbounded.
func TestFetchAgentDefinition_RejectsOversizedResponse(t *testing.T) {
	zipData := controlPlaneAgentZip(t)
	svc, _, _ := newAgentFetchService(t, agentFetchResponse{
		status: http.StatusOK, contentType: "application/zip", body: zipData,
	}, int64(len(zipData)-1))

	_, err := svc.FetchAgentDefinition(testAgentID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")
}

// A small archive can inflate to far more than the response ceiling; the
// extracted entry is held to the same ceiling.
func TestExtractYAMLFromZip_BoundsDecompressedEntry(t *testing.T) {
	svc := NewAPIUtilsService(PlatformAPIConfig{MaxResponseBytes: 1024},
		testHTTPClient(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Highly compressible: well under the ceiling compressed, well over it
	// inflated.
	inflated := strings.Repeat("a", 64*1024)
	zipData := agentZip(t, map[string]string{"agent.yaml": inflated}, "agent.yaml")
	require.Less(t, len(zipData), 1024, "precondition: the archive itself is within the ceiling")

	_, err := svc.ExtractYAMLFromZip(zipData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")

	// At the ceiling is still fine.
	atLimit := agentZip(t, map[string]string{"agent.yaml": strings.Repeat("a", 1024)}, "agent.yaml")
	got, err := svc.ExtractYAMLFromZip(atLimit)
	require.NoError(t, err)
	assert.Len(t, got, 1024)
}

// Agents are pushed after every kind in the DP->CP push order, mirroring the
// control plane's import order (platform-api utils.artifactImportOrder), rather
// than landing on the unknown-kind fallback.
func TestArtifactPushRank_AgentIsRankedExplicitly(t *testing.T) {
	unknown := artifactPushRank("NotAKind")
	agent := artifactPushRank(models.KindAgent)
	if agent == unknown {
		t.Fatalf("artifactPushRank(%q) = %d, the unknown-kind fallback; want an explicit rank", models.KindAgent, agent)
	}
	for _, kind := range []string{
		models.KindLlmProviderTemplate, models.KindLlmProvider, models.KindLlmProxy,
		models.KindMcp, models.KindRestApi, models.KindWebSubApi, models.KindWebBrokerApi,
	} {
		if r := artifactPushRank(kind); r >= agent {
			t.Errorf("artifactPushRank(%q) = %d, want it ahead of Agent (%d)", kind, r, agent)
		}
	}
}
