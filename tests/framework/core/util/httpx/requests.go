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

package httpx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// ResponseKey is the context key holding the response assertion steps read back.
const ResponseKey = "httpResponse"

// Funnel issues the request whose response a following assertion step reads, publishing it
// under ResponseKey.
//
// Clears the stored response before issuing, so a failed request leaves it absent rather than
// stale. For an intermediate read consumed within the same step, use Client directly.
type Funnel struct {
	client     *Client
	maxRetries int
	retryDelay time.Duration
}

// NewFunnel wraps a client.
func NewFunnel(client *Client, maxRetries int, retryDelay time.Duration) *Funnel {
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second
	}
	return &Funnel{client: client, maxRetries: maxRetries, retryDelay: retryDelay}
}

// Client exposes the raw client, for reads that must not publish.
func (f *Funnel) Client() *Client { return f.client }

// execute clears, calls, then publishes. Every method here goes through it.
func (f *Funnel) execute(ctx context.Context, req Request) (*Response, error) {
	tcontext.Remove(ctx, ResponseKey)

	resp, err := f.client.Do(ctx, req, f.maxRetries, f.retryDelay)
	if err != nil {
		return nil, err
	}

	if setErr := tcontext.Set(ctx, ResponseKey, resp); setErr != nil {
		return resp, setErr
	}
	return resp, nil
}

// Get issues a GET and publishes the response.
func (f *Funnel) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	return f.execute(ctx, Request{Method: http.MethodGet, URL: url, Headers: headers})
}

// Post issues a POST and publishes the response.
func (f *Funnel) Post(ctx context.Context, url string, headers map[string]string, body []byte) (*Response, error) {
	return f.execute(ctx, Request{
		Method: http.MethodPost, URL: url, Headers: headers, Body: body,
		ContentType: "application/json",
	})
}

// Put issues a PUT and publishes the response.
func (f *Funnel) Put(ctx context.Context, url string, headers map[string]string, body []byte) (*Response, error) {
	return f.execute(ctx, Request{
		Method: http.MethodPut, URL: url, Headers: headers, Body: body,
		ContentType: "application/json",
	})
}

// Patch issues a PATCH and publishes the response.
func (f *Funnel) Patch(ctx context.Context, url string, headers map[string]string, body []byte) (*Response, error) {
	return f.execute(ctx, Request{
		Method: http.MethodPatch, URL: url, Headers: headers, Body: body,
		ContentType: "application/json",
	})
}

// Delete issues a DELETE and publishes the response.
func (f *Funnel) Delete(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	return f.execute(ctx, Request{Method: http.MethodDelete, URL: url, Headers: headers})
}

// Send issues an arbitrary request and publishes the response.
func (f *Funnel) Send(ctx context.Context, req Request) (*Response, error) {
	return f.execute(ctx, req)
}

// Published returns the response a previous step stored. Absence is an error, not a nil
// response.
func Published(ctx context.Context) (*Response, error) {
	v, ok := tcontext.Get(ctx, ResponseKey)
	if !ok {
		return nil, fmt.Errorf("no response has been published: either no request was made, " +
			"or the step that made it failed before a response arrived")
	}
	resp, ok := v.(*Response)
	if !ok {
		return nil, fmt.Errorf("the published response is %T, not an *httpx.Response", v)
	}
	return resp, nil
}

// ClearPublished removes the stored response, for a between-scenario reset hook.
func ClearPublished(ctx context.Context) {
	tcontext.Remove(ctx, ResponseKey)
}
