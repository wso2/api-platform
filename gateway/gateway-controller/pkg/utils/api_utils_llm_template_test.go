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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fetchTemplateYAML = `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: acme-llm
spec:
  displayName: Acme Inference
`

func templateZip(t *testing.T, entryName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(entryName)
	require.NoError(t, err)
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// newTemplateFetchService points an APIUtilsService at a control plane that
// answers every request with status and body, recording the requests it saw.
func newTemplateFetchService(t *testing.T, status int, body []byte) (*APIUtilsService, *atomic.Int32, *http.Request) {
	t.Helper()
	var calls atomic.Int32
	captured := &http.Request{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		*captured = *r.Clone(r.Context())
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	svc := NewAPIUtilsService(PlatformAPIConfig{
		BaseURL: server.URL + "/api/internal/v1",
		Token:   "gw-token",
	}, testHTTPClient(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, &calls, captured
}

func TestFetchLLMProviderTemplateDefinition_ReturnsTemplate(t *testing.T) {
	svc, calls, req := newTemplateFetchService(t, http.StatusOK,
		templateZip(t, "llm-provider-template-acme-llm.yaml", fetchTemplateYAML))

	yamlData, err := svc.FetchLLMProviderTemplateDefinition("acme-llm")

	require.NoError(t, err)
	assert.Equal(t, fetchTemplateYAML, string(yamlData))
	assert.EqualValues(t, 1, calls.Load())
	assert.Equal(t, "/api/internal/v1/llm-provider-templates/acme-llm", req.URL.Path)
	assert.Equal(t, "gw-token", req.Header.Get("api-key"), "the gateway authenticates with its token")
}

func TestFetchLLMProviderTemplateDefinition_NotFound(t *testing.T) {
	svc, _, _ := newTemplateFetchService(t, http.StatusNotFound, []byte(`{"code":404}`))

	_, err := svc.FetchLLMProviderTemplateDefinition("acme-llm")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrControlPlaneNotFound, "a 404 is reported as not found")
	assert.Contains(t, err.Error(), "request failed with status 404", "the error text is unchanged")
}

func TestFetchLLMProviderTemplateDefinition_ServerErrorIsNotNotFound(t *testing.T) {
	svc, _, _ := newTemplateFetchService(t, http.StatusInternalServerError, []byte("boom"))

	_, err := svc.FetchLLMProviderTemplateDefinition("acme-llm")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrControlPlaneNotFound), "only a 404 is not found")
	assert.Contains(t, err.Error(), "request failed with status 500: boom")
}

func TestFetchLLMProviderTemplateDefinition_InvalidHandleNeverRequested(t *testing.T) {
	svc, calls, _ := newTemplateFetchService(t, http.StatusOK, nil)

	for _, handle := range []string{"", "../secrets", "acme/llm", "acme llm", "acme%2Fllm"} {
		_, err := svc.FetchLLMProviderTemplateDefinition(handle)
		assert.ErrorIs(t, err, ErrInvalidLLMProviderTemplateArtifact, "handle %q", handle)
	}
	assert.Zero(t, calls.Load(), "an invalid handle never reaches the control plane")
}

func TestFetchLLMProviderTemplateDefinition_RejectsWrongArtifacts(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"not a zip", []byte("plain text")},
		{"no yaml entry", nil},
		{"unparsable yaml", nil},
		{"wrong kind", nil},
		{"wrong name", nil},
	}
	cases[1].body = templateZip(t, "readme.txt", "nothing here")
	cases[2].body = templateZip(t, "llm-provider-template-acme-llm.yaml", "kind: [unterminated")
	cases[3].body = templateZip(t, "llm-provider-template-acme-llm.yaml",
		"apiVersion: gateway.api-platform.wso2.com/v1\nkind: LlmProvider\nmetadata:\n  name: acme-llm\n")
	cases[4].body = templateZip(t, "llm-provider-template-acme-llm.yaml",
		"apiVersion: gateway.api-platform.wso2.com/v1\nkind: LlmProviderTemplate\nmetadata:\n  name: other\n")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newTemplateFetchService(t, http.StatusOK, tc.body)
			_, err := svc.FetchLLMProviderTemplateDefinition("acme-llm")
			assert.ErrorIs(t, err, ErrInvalidLLMProviderTemplateArtifact)
		})
	}
}

func TestControlPlaneStatusError_MatchesNotFoundOnlyFor404(t *testing.T) {
	notFound := &controlPlaneStatusError{msg: "x request failed with status 404: nope", status: http.StatusNotFound}
	forbidden := &controlPlaneStatusError{msg: "x request failed with status 403: no", status: http.StatusForbidden}

	assert.ErrorIs(t, notFound, ErrControlPlaneNotFound)
	assert.False(t, errors.Is(forbidden, ErrControlPlaneNotFound))
	assert.False(t, errors.Is(notFound, ErrInvalidLLMProviderTemplateArtifact), "it matches nothing else")
	assert.Equal(t, "x request failed with status 404: nope", notFound.Error())
}
