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
	"testing"

	"github.com/wso2/api-platform/cli/utils"
)

func TestProviderListPath_NoOrgWithPagination(t *testing.T) {
	base := utils.AIWorkspaceLLMProvidersPath

	if got := ProviderListPath(ListQuery{}); got != base {
		t.Fatalf("no pagination: got %q, want %q", got, base)
	}
	if got := ProviderListPath(ListQuery{Limit: "50"}); got != base+"?limit=50" {
		t.Fatalf("limit only: got %q", got)
	}
	if got := ProviderListPath(ListQuery{Limit: "50", Offset: "10"}); got != base+"?limit=50&offset=10" {
		t.Fatalf("limit+offset: got %q", got)
	}
	if got := ProviderListPath(ListQuery{Offset: "10"}); got != base+"?offset=10" {
		t.Fatalf("offset only: got %q", got)
	}
}

func TestProviderByIDPath_NoQuery(t *testing.T) {
	want := utils.AIWorkspaceLLMProvidersPath + "/wso2-claude"
	if got := ProviderByIDPath("wso2-claude"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestProxyListPath_KeepsProjectIdThenAmpersand(t *testing.T) {
	// Project-scoped list still uses ?projectId=... and & for pagination.
	got := ProxyListPath("proj-1", ListQuery{Limit: "5"})
	want := utils.AIWorkspaceLLMProxiesPath + "?projectId=proj-1&limit=5"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
