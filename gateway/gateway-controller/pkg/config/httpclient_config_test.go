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

package config

import "testing"

func TestBuildHTTPClientConfigProxyTLSRequiresSeparateAcknowledgement(t *testing.T) {
	hc := HTTPClientConfig{
		Proxy: HTTPClientProxyConfig{
			Mode:   "url",
			URL:    "https://proxy.example.com:3128",
			Egress: "delegated",
			TLS: HTTPClientProxyTLSConfig{
				InsecureSkipVerify: true,
				// InsecureSkipVerifyAcknowledged intentionally left unset.
			},
		},
	}

	cfg, err := BuildHTTPClientConfig(hc, false)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}
	if cfg.Proxy.ProxyTLS == nil {
		t.Fatal("expected ProxyTLS to be set")
	}
	if !cfg.Proxy.ProxyTLS.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be carried through")
	}
	if cfg.Proxy.ProxyTLS.InsecureSkipVerifyAcknowledged {
		t.Fatal("expected InsecureSkipVerifyAcknowledged to stay false when not explicitly set, independent of InsecureSkipVerify")
	}

	hc.Proxy.TLS.InsecureSkipVerifyAcknowledged = true
	cfg, err = BuildHTTPClientConfig(hc, false)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}
	if !cfg.Proxy.ProxyTLS.InsecureSkipVerifyAcknowledged {
		t.Fatal("expected InsecureSkipVerifyAcknowledged to be carried through once explicitly set")
	}
}

func TestBuildHTTPClientConfigRejectsNegativeMaxResponseBytes(t *testing.T) {
	hc := HTTPClientConfig{
		Timeouts: HTTPClientTimeoutsConfig{MaxResponseBytes: -1},
	}

	if _, err := BuildHTTPClientConfig(hc, false); err == nil {
		t.Fatal("expected error for negative controller.http_client.timeouts.max_response_bytes, got nil")
	}
}
