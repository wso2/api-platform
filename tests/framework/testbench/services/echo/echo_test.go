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

package echo

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReflectEchoesMethodPathArgsHeadersAndBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/models/gpt-4/chat?model=original&prompt=hello",
		strings.NewReader(`{"model":"original"}`))
	req.Header.Set("X-Custom", "value")
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, http.MethodPost, body["method"])
	require.Equal(t, "/models/gpt-4/chat", body["path"])
	require.Contains(t, body["url"], "model=original")
	require.Equal(t, "value", body["headers"].(map[string]any)["X-Custom"])
	require.Equal(t, `{"model":"original"}`, body["data"])
	require.Equal(t, "original", body["json"].(map[string]any)["model"])
}

func TestReflectWithEmptyBodyStillReturnsData(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "", body["data"])
	require.Nil(t, body["json"])
}

func TestReflectStatusCodeOverrideReturnsRequestedStatus(t *testing.T) {
	for _, code := range []int{200, 429, 500, 599} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/chat?statusCode="+strconv.Itoa(code), strings.NewReader(`{}`))
			recorder := httptest.NewRecorder()

			New().Handler().ServeHTTP(recorder, req)

			require.Equal(t, code, recorder.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, "/chat", body["path"])
		})
	}
}

func TestReflectStatusCodeOverrideIgnoresInvalidValues(t *testing.T) {
	for _, raw := range []string{"not-a-number", "199", "600", ""} {
		t.Run(raw, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/chat?statusCode="+raw, nil)
			recorder := httptest.NewRecorder()

			New().Handler().ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

func TestGzippedReflectionHonoursStatusCodeOverride(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/gzip?statusCode=503", strings.NewReader(`{"a":1}`))
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "gzip", recorder.Header().Get("Content-Encoding"))
	zr, err := gzip.NewReader(recorder.Body)
	require.NoError(t, err)
	decoded, err := io.ReadAll(zr)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(decoded, &body))
	require.Equal(t, "/gzip", body["path"])
}

func TestStatusEndpointReturnsRequestedCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/status/418", nil)
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusTeapot, recorder.Code)
}

func TestStatusEndpointRejectsOutOfRangeCode(t *testing.T) {
	for _, path := range []string{"/status/199", "/status/600", "/status/not-a-number"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			recorder := httptest.NewRecorder()

			New().Handler().ServeHTTP(recorder, req)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestStaticJSONReturnsFixedShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Sample Slide Show")
}

func TestServiceMetadata(t *testing.T) {
	s := New()
	require.Equal(t, "echo", s.Name())
	require.Equal(t, Port, s.Port())
	require.False(t, s.Stateful())
}
