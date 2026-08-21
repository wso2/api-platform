/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
package authenticators

import (
	"fmt"
	"net/http"

	"github.com/wso2/go-httpkit/httpclient"
	"github.com/wso2/go-httpkit/netguard"
)

// NewDefaultJWKSHTTPClient returns an SSRF-guarded outbound *http.Client suitable for
// AuthConfig.HTTPClient, for a caller that has no other shared client to reuse. It
// permits private/in-cluster addresses (a common JWKS deployment target) while still
// blocking loopback/link-local/cloud-metadata ranges, per ssrf-prevention.md.
func NewDefaultJWKSHTTPClient() (*http.Client, error) {
	cfg := httpclient.DefaultConfig()
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()

	client, err := httpclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build default JWKS HTTP client: %w", err)
	}
	return client, nil
}
