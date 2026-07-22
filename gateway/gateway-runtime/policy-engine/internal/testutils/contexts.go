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
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
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

// NewTestRequestContext creates a RequestContext with default test values.
func NewTestRequestContext() *policy.RequestContext {
	headers := policy.NewHeaders(map[string][]string{"content-type": {"application/json"}})
	return &policy.RequestContext{
		SharedContext: NewTestSharedContext(),
		Headers:       headers,
		Path:          "/api/v1/users/123",
		Method:        "GET",
		Authority:     "api.example.com",
		Scheme:        "https",
		Downstream:    &policy.DownstreamContext{Request: &policy.DownstreamRequest{Headers: policy.NewHeaders(headers.GetAll())}},
	}
}

// NewTestRequestContextWithHeaders creates a RequestContext with custom headers.
func NewTestRequestContextWithHeaders(headers map[string][]string) *policy.RequestContext {
	wrapped := policy.NewHeaders(headers)
	return &policy.RequestContext{
		SharedContext: NewTestSharedContext(),
		Headers:       wrapped,
		Path:          "/test/path",
		Method:        "GET",
		Authority:     "test.example.com",
		Scheme:        "https",
		Downstream:    &policy.DownstreamContext{Request: &policy.DownstreamRequest{Headers: policy.NewHeaders(wrapped.GetAll())}},
	}
}

// NewTestResponseContext creates a ResponseContext with default test values.
func NewTestResponseContext() *policy.ResponseContext {
	reqCtx := NewTestRequestContext()
	responseHeaders := policy.NewHeaders(map[string][]string{"content-type": {"application/json"}})
	return &policy.ResponseContext{
		SharedContext:   reqCtx.SharedContext,
		RequestHeaders:  reqCtx.Headers,
		RequestBody:     reqCtx.Body,
		RequestPath:     reqCtx.Path,
		RequestMethod:   reqCtx.Method,
		ResponseHeaders: responseHeaders,
		ResponseStatus:  200,
		Downstream:      &policy.DownstreamContext{Request: &policy.DownstreamRequest{Headers: policy.NewHeaders(reqCtx.Headers.GetAll())}},
		Upstream:        &policy.UpstreamResponseContext{Response: &policy.UpstreamResponse{Headers: policy.NewHeaders(responseHeaders.GetAll())}},
	}
}

// NewTestResponseContextWithStatus creates a ResponseContext with a custom status code.
func NewTestResponseContextWithStatus(status int) *policy.ResponseContext {
	ctx := NewTestResponseContext()
	ctx.ResponseStatus = status
	return ctx
}
