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
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

func graphqlUpstreamStrPtr(s string) *string { return &s }

// containsFieldError reports whether errs has a ValidationError for the given field.
func containsFieldError(errs []config.ValidationError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

// TestValidateGraphQLUpstream_RefOnly pins the fix for the gap where a
// ref-only upstream produced only the misleading "Upstream URL is required"
// error, with nothing telling the caller that ref itself is unsupported for
// GraphQLApi (it has no upstreamDefinitions list to resolve it against).
func TestValidateGraphQLUpstream_RefOnly(t *testing.T) {
	up := &api.Upstream{Ref: graphqlUpstreamStrPtr("some-def")}

	errs := validateGraphQLUpstream("main", up)

	if !containsFieldError(errs, "spec.upstream.main.ref") {
		t.Errorf("expected a spec.upstream.main.ref error for a ref-only upstream, got: %+v", errs)
	}
	if !containsFieldError(errs, "spec.upstream.main.url") {
		t.Errorf("expected a spec.upstream.main.url error (missing) for a ref-only upstream, got: %+v", errs)
	}
}

// TestValidateGraphQLUpstream_URLAndRef_RefRejected pins the other half of the
// gap: url+ref together used to be accepted outright, with ref silently
// ignored downstream by resolveUpstreamURL's early return on a non-empty Url
// (transform/restapi.go). A ref alongside a valid url must still be rejected.
func TestValidateGraphQLUpstream_URLAndRef_RefRejected(t *testing.T) {
	up := &api.Upstream{
		Url: graphqlUpstreamStrPtr("http://backend.example.com:8080/graphql"),
		Ref: graphqlUpstreamStrPtr("some-def"),
	}

	errs := validateGraphQLUpstream("main", up)

	if !containsFieldError(errs, "spec.upstream.main.ref") {
		t.Errorf("expected a spec.upstream.main.ref error when ref is set alongside a valid url, got: %+v", errs)
	}
	if containsFieldError(errs, "spec.upstream.main.url") {
		t.Errorf("did not expect a url error when url is valid, got: %+v", errs)
	}
}

// TestValidateGraphQLUpstream_URLOnly_NoRefError is the control case: a
// direct url with no ref must produce no errors at all.
func TestValidateGraphQLUpstream_URLOnly_NoRefError(t *testing.T) {
	up := &api.Upstream{Url: graphqlUpstreamStrPtr("https://backend.example.com/graphql")}

	errs := validateGraphQLUpstream("main", up)

	if len(errs) != 0 {
		t.Errorf("expected no validation errors for a valid direct url, got: %+v", errs)
	}
}
