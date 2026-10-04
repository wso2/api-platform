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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package apiportal

import (
	"context"
	"testing"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func portalResponseContext(response *httpx.Response) context.Context {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("api-portal"))
	if err := tcontext.Set(ctx, keyAPIPortalResponse, response); err != nil {
		panic(err)
	}
	return ctx
}

func TestAPIPortalResponseStatus(t *testing.T) {
	ctx := portalResponseContext(&httpx.Response{StatusCode: 302})
	steps := &Steps{}

	if err := steps.apiPortalResponseStatus(ctx, "302"); err != nil {
		t.Fatalf("apiPortalResponseStatus() returned an error: %v", err)
	}
	if err := steps.apiPortalResponseStatus(ctx, "200"); err == nil {
		t.Fatal("apiPortalResponseStatus() accepted an unexpected status")
	}
}

func TestAPIPortalResponseBodyMatch(t *testing.T) {
	ctx := portalResponseContext(&httpx.Response{Body: []byte("plain response")})
	steps := &Steps{}

	if err := steps.apiPortalResponseBodyMatch(ctx, "plain", true); err != nil {
		t.Fatalf("apiPortalResponseBodyMatch() returned an error: %v", err)
	}
	if err := steps.apiPortalResponseBodyMatch(ctx, "html", false); err != nil {
		t.Fatalf("apiPortalResponseBodyMatch() negative assertion returned an error: %v", err)
	}
}

func TestPortalListingMatches(t *testing.T) {
	tests := []struct {
		name           string
		texts          []string
		mustContain    string
		mustNotContain string
		want           bool
	}{
		{name: "required value present", texts: []string{"REST API", "GraphQL API"}, mustContain: "GraphQL", want: true},
		{name: "required value absent", texts: []string{"REST API"}, mustContain: "GraphQL", want: false},
		{name: "excluded value absent", texts: []string{"REST API"}, mustNotContain: "GraphQL", want: true},
		{name: "excluded value present", texts: []string{"REST API", "GraphQL API"}, mustNotContain: "GraphQL", want: false},
		{name: "required present and excluded absent", texts: []string{"REST API", "GraphQL API"}, mustContain: "GraphQL", mustNotContain: "MCP", want: true},
		{name: "required present while excluded present", texts: []string{"REST API", "GraphQL API", "MCP Server"}, mustContain: "GraphQL", mustNotContain: "MCP", want: false},
		{name: "both expectations empty", texts: nil, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portalListingMatches(tt.texts, tt.mustContain, tt.mustNotContain); got != tt.want {
				t.Fatalf("portalListingMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
