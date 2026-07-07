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

package aiworkspace

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

func newTestClient(url string) *Client {
	return NewClientWithOptions(&config.AIWorkspace{
		Name: "test-ws",
		URL:  url,
		Auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "test-key"},
	}, false)
}

// TestExists covers the create-or-update probe used by `ap ai-workspace apply`:
// a 2xx means the artifact is present (update), a 404 means it is absent
// (create), and any other status is a real error.
func TestExists(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantExists bool
		wantErr    bool
	}{
		{name: "200 exists", status: http.StatusOK, wantExists: true},
		{name: "404 absent", status: http.StatusNotFound, wantExists: false},
		{name: "500 errors", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			exists, err := client.Exists(ProviderByIDPath("wso2-claude"))

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for status %d, got nil", tc.status)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if exists != tc.wantExists {
				t.Fatalf("Exists = %v, want %v", exists, tc.wantExists)
			}
		})
	}
}
