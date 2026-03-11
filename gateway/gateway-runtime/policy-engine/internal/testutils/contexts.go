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

package testutils

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// NewTestSharedContext creates a SharedContext with default test values.
func NewTestSharedContext() *policy.SharedContext {
	return &policy.SharedContext{
		RequestID:     "test-request-id",
		APIName:       "TestAPI",
		APIVersion:    "v1.0",
		APIContext:    "/api",
		OperationPath: "/users/{id}",
		Metadata:      map[string]interface{}{},
	}
}

// NewTestRequestHeaderContext creates a RequestHeaderContext with default test values.
func NewTestRequestHeaderContext() *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
		SharedContext: NewTestSharedContext(),
		Headers:       policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		Path:          "/api/v1/users/123",
		Method:        "GET",
		Authority:     "api.example.com",
		Scheme:        "https",
	}
}

// NewTestRequestHeaderContextWithHeaders creates a RequestHeaderContext with custom headers.
func NewTestRequestHeaderContextWithHeaders(headers map[string][]string) *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
		SharedContext: NewTestSharedContext(),
		Headers:       policy.NewHeaders(headers),
		Path:          "/test/path",
		Method:        "GET",
		Authority:     "test.example.com",
		Scheme:        "https",
	}
}

// NewTestRequestBodyContext creates a RequestBodyContext with default test values.
func NewTestRequestBodyContext() *policy.RequestContext {
	sharedCtx := NewTestSharedContext()
	return &policy.RequestContext{
		SharedContext: sharedCtx,
		Headers:       policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		Path:          "/api/v1/users/123",
		Method:        "GET",
		Authority:     "api.example.com",
		Scheme:        "https",
	}
}

// NewTestResponseHeaderContext creates a ResponseHeaderContext with default test values.
func NewTestResponseHeaderContext() *policy.ResponseHeaderContext {
	reqHdrCtx := NewTestRequestHeaderContext()
	return &policy.ResponseHeaderContext{
		SharedContext:   reqHdrCtx.SharedContext,
		RequestHeaders:  reqHdrCtx.Headers,
		RequestPath:     reqHdrCtx.Path,
		RequestMethod:   reqHdrCtx.Method,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseStatus:  200,
	}
}

// NewTestResponseHeaderContextWithStatus creates a ResponseHeaderContext with a custom status code.
func NewTestResponseHeaderContextWithStatus(status int) *policy.ResponseHeaderContext {
	ctx := NewTestResponseHeaderContext()
	ctx.ResponseStatus = status
	return ctx
}

// NewTestResponseBodyContext creates a ResponseBodyContext with default test values.
func NewTestResponseBodyContext() *policy.ResponseContext {
	reqHdrCtx := NewTestRequestHeaderContext()
	return &policy.ResponseContext{
		SharedContext:   reqHdrCtx.SharedContext,
		RequestHeaders:  reqHdrCtx.Headers,
		RequestPath:     reqHdrCtx.Path,
		RequestMethod:   reqHdrCtx.Method,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseStatus:  200,
	}
}
