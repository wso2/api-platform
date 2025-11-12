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
	"net/http"

	"platform-api/src/internal/client"
)

// Client is a lightweight per-DevPortal client. It is stateless and holds the
// configured shared http.Client and DevPortalConfig used to build requests.
type DevPortalClient struct {
	cfg        DevPortalConfig
	httpClient *client.RetryableHTTPClient // retry-enabled HTTP client
	headerName string
	apiKey     string
}

// NewClient creates a new DevPortal client for the provided DevPortalConfig.
func NewDevPortalClient(cfg DevPortalConfig) *DevPortalClient {
	var hc *client.RetryableHTTPClient
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // default retry attempts
	}

	hc = client.NewRetryableHTTPClient(maxRetries, cfg.Timeout)

	header := cfg.HeaderName
	if header == "" {
		header = DefaultHeaderName
	}
	return &DevPortalClient{
		cfg:        cfg,
		httpClient: hc,
		headerName: header,
		apiKey:     cfg.APIKey,
	}
}

// do executes the request with per-request header injection and timeout.
// It will inject the configured API key into headerName if present.
func (c *DevPortalClient) do(req *http.Request) (*http.Response, error) {
	// inject token header (apiKey)
	if c.headerName != "" && c.apiKey != "" {
		req.Header.Set(c.headerName, c.apiKey)
	}
	return c.httpClient.Do(req)
}
