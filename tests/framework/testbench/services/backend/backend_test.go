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

package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsResponseHeadersAreDeterministic(t *testing.T) {
	for _, path := range []string{"/analytics-headers", "/analytics-headers/no-policy"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			recorder := httptest.NewRecorder()

			New().Handler().ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "allowed", recorder.Header().Get("X-Allowed-Response"))
			require.Equal(t, "denied", recorder.Header().Get("X-Denied-Response"))
		})
	}
}

func TestAnalyticsResponseHeadersAreNotAddedToOtherPaths(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ordinary", nil)
	recorder := httptest.NewRecorder()

	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Header().Get("X-Allowed-Response"))
	require.Empty(t, recorder.Header().Get("X-Denied-Response"))
}
