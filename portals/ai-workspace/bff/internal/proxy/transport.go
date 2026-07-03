/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewTransport builds an *http.Transport for upstream calls with explicit
// timeouts and connection pooling. skipVerify accepts the Platform API's
// self-signed certificate (same posture as the old nginx proxy_ssl_verify off).
func NewTransport(skipVerify bool) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// #nosec G402 — the Platform API uses a self-signed cert in the default
		// deployment; verification is opt-in via PLATFORM_API_TLS_SKIP_VERIFY=false.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
	}
}
