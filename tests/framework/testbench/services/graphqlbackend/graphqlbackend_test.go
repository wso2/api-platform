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

package graphqlbackend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEchoReturnsTheRequestBodyVerbatim(t *testing.T) {
	body := `{"data":{"countries":[{"code":"US"}]},"errors":[{"message":"partial failure"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, body, recorder.Body.String())
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

func TestEchoWithNoBodyReturnsEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Body.String())
}

func TestEchoHonorsTheStatusCodeQueryParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/?statusCode=500", strings.NewReader(`{"errors":[{"message":"boom"}]}`))
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, `{"errors":[{"message":"boom"}]}`, recorder.Body.String())
}

func TestEchoIgnoresAnOutOfRangeStatusCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/?statusCode=999999", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHealthEndpointReportsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok","service":"graphql-backend"}`, recorder.Body.String())
}

func TestServiceMetadata(t *testing.T) {
	svc := New()
	require.Equal(t, "graphql-backend", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.False(t, svc.Stateful())
}
