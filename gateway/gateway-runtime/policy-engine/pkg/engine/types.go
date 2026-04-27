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

package engine

import (
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// RequestHeaderResult is the exported result of executing request header policies.
// It omits HTTP-only fields (path/host/query rewrites, upstream name) that have
// no meaning in the event-gateway context.
type RequestHeaderResult struct {
	HeadersToSet      map[string]string
	HeadersToRemove   []string
	ShortCircuited    bool
	ImmediateResponse *ImmediateResponseResult
	TotalDuration     time.Duration
}

// RequestBodyResult is the exported result of executing request body policies.
type RequestBodyResult struct {
	HeadersToSet      map[string]string
	HeadersToRemove   []string
	Body              []byte
	ShortCircuited    bool
	ImmediateResponse *ImmediateResponseResult
	TotalDuration     time.Duration
}

// ResponseHeaderResult is the exported result of executing response header policies.
type ResponseHeaderResult struct {
	HeadersToSet      map[string]string
	HeadersToRemove   []string
	ShortCircuited    bool
	ImmediateResponse *ImmediateResponseResult
	TotalDuration     time.Duration
}

// ResponseBodyResult is the exported result of executing response body policies.
type ResponseBodyResult struct {
	HeadersToSet      map[string]string
	HeadersToRemove   []string
	Body              []byte
	StatusCode        *int
	ShortCircuited    bool
	ImmediateResponse *ImmediateResponseResult
	TotalDuration     time.Duration
}

// ImmediateResponseResult represents a short-circuit response from a policy.
type ImmediateResponseResult struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// mapRequestHeaderResult converts the internal executor result to the exported type.
func mapRequestHeaderResult(r *executor.RequestHeaderExecutionResult) *RequestHeaderResult {
	if r == nil {
		return nil
	}
	res := &RequestHeaderResult{
		ShortCircuited: r.ShortCircuited,
		TotalDuration:  r.TotalExecutionTime,
	}

	if r.FinalAction != nil {
		switch a := r.FinalAction.(type) {
		case policy.UpstreamRequestHeaderModifications:
			res.HeadersToSet = a.HeadersToSet
			res.HeadersToRemove = a.HeadersToRemove
		case policy.ImmediateResponse:
			res.ShortCircuited = true
			res.ImmediateResponse = &ImmediateResponseResult{
				StatusCode: a.StatusCode,
				Headers:    a.Headers,
				Body:       a.Body,
			}
		}
	}

	// Merge all non-short-circuit results
	if !res.ShortCircuited {
		merged := make(map[string]string)
		var removed []string
		for _, pr := range r.Results {
			if pr.Skipped {
				continue
			}
			if mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications); ok {
				for k, v := range mods.HeadersToSet {
					merged[k] = v
				}
				removed = append(removed, mods.HeadersToRemove...)
			}
		}
		if len(merged) > 0 {
			res.HeadersToSet = merged
		}
		if len(removed) > 0 {
			res.HeadersToRemove = removed
		}
	}

	return res
}

// mapRequestBodyResult converts the internal executor result to the exported type.
func mapRequestBodyResult(r *executor.RequestExecutionResult) *RequestBodyResult {
	if r == nil {
		return nil
	}
	res := &RequestBodyResult{
		ShortCircuited: r.ShortCircuited,
		TotalDuration:  r.TotalExecutionTime,
	}

	if r.FinalAction != nil {
		switch a := r.FinalAction.(type) {
		case policy.UpstreamRequestModifications:
			res.HeadersToSet = a.HeadersToSet
			res.HeadersToRemove = a.HeadersToRemove
			res.Body = a.Body
		case policy.ImmediateResponse:
			res.ShortCircuited = true
			res.ImmediateResponse = &ImmediateResponseResult{
				StatusCode: a.StatusCode,
				Headers:    a.Headers,
				Body:       a.Body,
			}
		}
	}

	return res
}

// mapResponseHeaderResult converts the internal executor result to the exported type.
func mapResponseHeaderResult(r *executor.ResponseHeaderExecutionResult) *ResponseHeaderResult {
	if r == nil {
		return nil
	}
	res := &ResponseHeaderResult{
		ShortCircuited: r.ShortCircuited,
		TotalDuration:  r.TotalExecutionTime,
	}

	if r.FinalAction != nil {
		switch a := r.FinalAction.(type) {
		case policy.DownstreamResponseHeaderModifications:
			res.HeadersToSet = a.HeadersToSet
			res.HeadersToRemove = a.HeadersToRemove
		case policy.ImmediateResponse:
			res.ShortCircuited = true
			res.ImmediateResponse = &ImmediateResponseResult{
				StatusCode: a.StatusCode,
				Headers:    a.Headers,
				Body:       a.Body,
			}
		}
	}

	// Merge all non-short-circuit results
	if !res.ShortCircuited {
		merged := make(map[string]string)
		var removed []string
		for _, pr := range r.Results {
			if pr.Skipped {
				continue
			}
			if mods, ok := pr.Action.(policy.DownstreamResponseHeaderModifications); ok {
				for k, v := range mods.HeadersToSet {
					merged[k] = v
				}
				removed = append(removed, mods.HeadersToRemove...)
			}
		}
		if len(merged) > 0 {
			res.HeadersToSet = merged
		}
		if len(removed) > 0 {
			res.HeadersToRemove = removed
		}
	}

	return res
}

// mapResponseBodyResult converts the internal executor result to the exported type.
func mapResponseBodyResult(r *executor.ResponseExecutionResult) *ResponseBodyResult {
	if r == nil {
		return nil
	}
	res := &ResponseBodyResult{
		ShortCircuited: r.ShortCircuited,
		TotalDuration:  r.TotalExecutionTime,
	}

	if r.FinalAction != nil {
		switch a := r.FinalAction.(type) {
		case policy.DownstreamResponseModifications:
			res.HeadersToSet = a.HeadersToSet
			res.HeadersToRemove = a.HeadersToRemove
			res.Body = a.Body
			res.StatusCode = a.StatusCode
		case policy.ImmediateResponse:
			res.ShortCircuited = true
			res.ImmediateResponse = &ImmediateResponseResult{
				StatusCode: a.StatusCode,
				Headers:    a.Headers,
				Body:       a.Body,
			}
		}
	}

	return res
}
