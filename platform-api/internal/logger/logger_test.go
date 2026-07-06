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

package logger

import "testing"

func TestShortenSource(t *testing.T) {
	// shortenSource must derive the module-relative path from the tail after the
	// "platform-api" module dir; whatever absolute prefix precedes it is
	// irrelevant. Prove that by running each case under several unrelated roots.
	roots := []string{"", "/build", "/home/x/dev", "/var/lib/ci/workspace"}

	relCases := []struct {
		name string
		tail string // path from the "platform-api" module dir onward
		want string
	}{
		{"internal package strips internal/", "platform-api/internal/server/server.go", "server/server.go"},
		{"cmd entrypoint kept", "platform-api/cmd/main.go", "cmd/main.go"},
		{"facade package kept", "platform-api/platform/platform.go", "platform/platform.go"},
	}
	for _, tc := range relCases {
		for _, root := range roots {
			in := root + "/" + tc.tail
			t.Run(tc.name+" @ "+root, func(t *testing.T) {
				if got := shortenSource(in); got != tc.want {
					t.Errorf("shortenSource(%q) = %q, want %q", in, got, tc.want)
				}
			})
		}
	}

	t.Run("module cache path drops @version", func(t *testing.T) {
		in := "/anywhere/go/pkg/mod/github.com/wso2/api-platform/platform-api@v0.9.15/internal/service/api.go"
		if got := shortenSource(in); got != "service/api.go" {
			t.Errorf("shortenSource(%q) = %q, want %q", in, got, "service/api.go")
		}
	})

	t.Run("outside module returned as-is", func(t *testing.T) {
		const in = "/anywhere/common/eventhub/sqlbackend.go"
		if got := shortenSource(in); got != in {
			t.Errorf("shortenSource(%q) = %q, want %q (unchanged)", in, got, in)
		}
	})
}
