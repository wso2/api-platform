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

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	sdkutils "github.com/wso2/api-platform/sdk/core/utils"
)

// BuildUserURLClientConfig derives the client configuration for destinations
// that come from tenant or user input. It reuses [policy_engine.http_client]'s
// pooling, timeout, TLS and proxy settings and applies
// controller.server.network_security.access_control as its policy, combined with
// policy_engine.http_client.destination_guard when that is enabled so the existing setting can
// only make the result stricter (netguard.Stricter). With access control off the
// result is the [policy_engine.http_client] configuration unchanged.
func BuildUserURLClientConfig(hc HTTPClientConfig, ac AccessControlConfig) (httpclient.Config, error) {
	base, err := BuildHTTPClientConfig(hc)
	if err != nil {
		return httpclient.Config{}, err
	}
	if !ac.Enabled {
		return base, nil
	}
	policy, err := ac.GuardConfig().Build()
	if err != nil {
		return httpclient.Config{}, fmt.Errorf("controller.server.network_security.access_control: %w", err)
	}
	if base.DestinationGuard.Enabled {
		policy = netguard.Stricter(policy, base.DestinationGuard.Policy)
	}
	base.DestinationGuard = httpclient.DestinationGuardConfig{Enabled: true, Policy: policy, MaxRedirects: ac.MaxRedirects}
	if base.Proxy.Mode != "" && base.Proxy.Mode != "none" && base.Proxy.Egress == httpclient.ProxyEgressUnset {
		if hc.Proxy.Egress == "manual_connect" {
			base.Proxy.Egress = httpclient.ProxyEgressManualCONNECT
		} else {
			// The destination_guard section is off, so egress was never required. Delegated keeps
			// such a deployment working: the proxy's egress controls govern the
			// destination's address; its hostname and scheme are still enforced here.
			base.Proxy.Egress = httpclient.ProxyEgressDelegated
		}
	}
	return base, nil
}

// NewUserURLClientFactory returns the factory and checker the policy engine
// installs with sdkutils.SetUserURLClientFactory, built from base (see
// BuildUserURLClientConfig). Refusals from either surface as errors matching
// sdkutils.ErrDestinationNotAllowed.
func NewUserURLClientFactory(base httpclient.Config) (sdkutils.UserURLClientFactory, sdkutils.UserURLChecker) {
	factory := func(opts sdkutils.UserURLClientOptions) (*http.Client, error) {
		cfg := base
		if opts.Timeout > 0 {
			cfg.Timeouts.Overall = opts.Timeout
			cfg.Timeouts.ResponseHeader = opts.Timeout
		}
		if opts.InsecureSkipVerify {
			cfg.TLS.InsecureSkipVerify = true // #nosec G402 -- a policy parameter's explicit opt-out
			cfg.TLS.InsecureSkipVerifyAcknowledged = true
		}
		if opts.RootCAs != nil {
			cfg.TLS.RootCAs = opts.RootCAs
		}
		if opts.GetClientCertificate != nil {
			cfg.TLS.GetClientCertificate = opts.GetClientCertificate
		}
		if opts.MaxConnsPerHost > 0 {
			cfg.Pooling.MaxConnsPerHost = opts.MaxConnsPerHost
			cfg.Pooling.MaxIdleConnsPerHost = opts.MaxConnsPerHost
			cfg.Pooling.MaxIdleConns = opts.MaxConnsPerHost
		}
		client, err := httpclient.New(cfg)
		if err != nil {
			return nil, err
		}
		client.Transport = refusalTranslator{next: client.Transport}
		if !opts.FollowRedirects {
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		} else if client.CheckRedirect != nil {
			check := client.CheckRedirect
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return translateRefusal(check(req, via))
			}
		}
		return client, nil
	}

	checker := func(ctx context.Context, rawURL string) error {
		if !base.DestinationGuard.Enabled {
			return nil
		}
		return translateRefusal(base.DestinationGuard.Policy.CheckURL(ctx, rawURL, true))
	}
	return factory, checker
}

// translateRefusal adds sdkutils.ErrDestinationNotAllowed to a netguard
// refusal, so a policy (which does not import httpkit) can test for it.
func translateRefusal(err error) error {
	if err != nil && errors.Is(err, netguard.ErrDestinationNotAllowed) {
		return fmt.Errorf("%w: %w", sdkutils.ErrDestinationNotAllowed, err)
	}
	return err
}

type refusalTranslator struct{ next http.RoundTripper }

func (r refusalTranslator) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	return resp, translateRefusal(err)
}
