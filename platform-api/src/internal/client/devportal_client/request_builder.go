/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package devportal_client

import (
	"io"
	"net/http"
)

// RequestBuilder provides a fluent API for building HTTP requests with optional features
type RequestBuilder struct {
	client    *DevPortalClient
	method    string
	url       string
	body      interface{}
	preflight bool
	headers   map[string]string
}

// NewRequest creates a new RequestBuilder for the given method and URL
func (c *DevPortalClient) NewRequest(method, url string) *RequestBuilder {
	return &RequestBuilder{
		client:  c,
		method:  method,
		url:     url,
		headers: make(map[string]string),
	}
}

// WithJSONBody sets the request body as JSON
func (rb *RequestBuilder) WithJSONBody(body interface{}) *RequestBuilder {
	rb.body = body
	return rb
}

// WithPreflightCheck enables connectivity check before request creation
func (rb *RequestBuilder) WithPreflightCheck() *RequestBuilder {
	rb.preflight = true
	return rb
}

// WithHeader adds a custom header to the request
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// Build creates the HTTP request with all configured options
func (rb *RequestBuilder) Build() (*http.Request, error) {
	// Perform preflight check if requested
	if rb.preflight {
		if err := rb.client.checkEndpointConnectivity(rb.url); err != nil {
			return nil, err
		}
	}

	// Create the base request
	req, err := rb.client.newJSONRequest(rb.method, rb.url, rb.body)
	if err != nil {
		return nil, err
	}

	// Add custom headers
	for key, value := range rb.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// BuildMultipart creates a multipart request (for file uploads)
func (rb *RequestBuilder) BuildMultipart(body io.Reader, contentType string) (*http.Request, error) {
	// Perform preflight check if requested
	if rb.preflight {
		if err := rb.client.checkEndpointConnectivity(rb.url); err != nil {
			return nil, err
		}
	}

	// Create multipart request
	req, err := http.NewRequest(rb.method, rb.url, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Add custom headers
	for key, value := range rb.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}
