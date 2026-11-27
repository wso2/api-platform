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

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/backend"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func newBackendTestServer(t *testing.T) *APIServer {
	return newBackendTestServerWithValidator(t, nil)
}

// newBackendTestServerWithValidator builds the server with the validator newValidator returns for
// the test database, or the service's default when newValidator is nil.
func newBackendTestServerWithValidator(t *testing.T, newValidator func(storage.Storage) *config.BackendValidator) *APIServer {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"), GatewayID: "gw"}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	es, _ := storage.AsBackendStorage(db)
	store := storage.NewBackendStore()
	syncer := backend.NewSyncer(es, store, nil, backend.NewPublisher(store, nil), logger)
	var validator *config.BackendValidator
	if newValidator != nil {
		validator = newValidator(db)
	}
	s := &APIServer{logger: logger}
	s.SetBackendService(backend.NewService(backend.ServiceConfig{DB: es, Validator: validator, Syncer: syncer, Logger: logger}))
	return s
}

const backendYAML = `apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: tenant-a
spec:
  endpoints:
    - url: http://orders-tenant-a:9080
`

func backendRequest(method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/yaml")
	return r
}

func TestBackendHandlers_StatusCodes(t *testing.T) {
	s := newBackendTestServer(t)

	w := httptest.NewRecorder()
	s.CreateBackend(w, backendRequest(http.MethodPost, "/backends", backendYAML))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created api.Backend
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "tenant-a", *created.Status.Id)

	w = httptest.NewRecorder()
	s.CreateBackend(w, backendRequest(http.MethodPost, "/backends", backendYAML))
	assert.Equal(t, http.StatusConflict, w.Code)

	w = httptest.NewRecorder()
	s.CreateBackend(w, backendRequest(http.MethodPost, "/backends", strings.Replace(backendYAML, "tenant-a", "Bad_Name", 1)))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	require.NotNil(t, errResp.Errors)
	assert.Equal(t, "metadata.name", *(*errResp.Errors)[0].Field)

	w = httptest.NewRecorder()
	s.GetBackendById(w, backendRequest(http.MethodGet, "/backends/nope", ""), "nope")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	s.ListBackends(w, backendRequest(http.MethodGet, "/backends", ""))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"count":1`)

	w = httptest.NewRecorder()
	s.DeleteBackend(w, backendRequest(http.MethodDelete, "/backends/tenant-a", ""), "tenant-a")
	assert.Equal(t, http.StatusOK, w.Code)
}

func backendWarnings(t *testing.T, body []byte) []string {
	t.Helper()
	var resp api.Backend
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotNil(t, resp.Status)
	if resp.Status.Warnings == nil {
		return nil
	}
	var out []string
	for _, w := range *resp.Status.Warnings {
		out = append(out, string(*w.Code)+"@"+*w.Field)
	}
	return out
}

func TestBackendHandlers_CreateAndUpdateReturnTLSWarnings(t *testing.T) {
	s := newBackendTestServerWithValidator(t, func(db storage.Storage) *config.BackendValidator {
		return config.NewBackendValidator(config.NewUpstreamTLSValidator(db, false)).
			WithTrustBundleCheck(func() bool { return false })
	})
	httpsYAML := strings.Replace(backendYAML, "http://orders-tenant-a:9080", "https://orders-tenant-a:9443", 1)

	w := httptest.NewRecorder()
	s.CreateBackend(w, backendRequest(http.MethodPost, "/backends", httpsYAML))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, []string{"TLS_NO_TRUST_SOURCE@spec.endpoints"}, backendWarnings(t, w.Body.Bytes()))

	w = httptest.NewRecorder()
	s.UpdateBackend(w, backendRequest(http.MethodPut, "/backends/tenant-a",
		httpsYAML+"  tls:\n    verifyHostName: false\n"), "tenant-a")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{"TLS_VERIFY_HOSTNAME_DISABLED@spec.tls.verifyHostName", "TLS_NO_TRUST_SOURCE@spec.tls"},
		backendWarnings(t, w.Body.Bytes()))

	w = httptest.NewRecorder()
	s.GetBackendById(w, backendRequest(http.MethodGet, "/backends/tenant-a", ""), "tenant-a")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, backendWarnings(t, w.Body.Bytes()), "warnings belong to the create and update responses")
}

func TestBackendHandlers_UnavailableWithoutAService(t *testing.T) {
	s := &APIServer{logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	w := httptest.NewRecorder()
	s.ListBackends(w, backendRequest(http.MethodGet, "/backends", ""))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// A Backend definition is read through a size bound: an oversized body gets 413 on create and
// update, before anything is parsed or stored.
func TestBackendHandlers_OversizedBodyIs413(t *testing.T) {
	s := newBackendTestServer(t)
	oversized := backendYAML + "# " + strings.Repeat("x", maxBackendRequestBodyBytes) + "\n"

	w := httptest.NewRecorder()
	s.CreateBackend(w, backendRequest(http.MethodPost, "/backends", oversized))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	w = httptest.NewRecorder()
	s.UpdateBackend(w, backendRequest(http.MethodPut, "/backends/tenant-a", oversized), "tenant-a")
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	w = httptest.NewRecorder()
	s.GetBackendById(w, backendRequest(http.MethodGet, "/backends/tenant-a", ""), "tenant-a")
	assert.Equal(t, http.StatusNotFound, w.Code, "nothing was stored")
}
