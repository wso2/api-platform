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

import "time"

// DevPortalConfig contains per-DevPortal configuration used to create clients
type DevPortalConfig struct {
	BaseURL    string        // full base URL including scheme, e.g. https://devportal.example
	APIKey     string        // API key or token to use in requests
	HeaderName string        // header name for API key (defaults to x-wso2-api-key if empty)
	Timeout    time.Duration // per-request timeout
	MaxRetries int           // max retry attempts for transient errors
	// Future fields: TLSConfig, ProxyURL, TokenProvider, etc.
}

// DefaultHeaderName is used when no header name is provided
const DefaultHeaderName = "x-wso2-api-key"

// DefaultTimeout is the default client timeout
const DefaultTimeout = 10 * time.Second

// DefaultMaxRetries is the default retry attempts for transient errors
const DefaultMaxRetries = 3
