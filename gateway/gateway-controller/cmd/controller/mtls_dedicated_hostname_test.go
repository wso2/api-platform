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

package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func storedRestAPI(handle, vhostsMain string, sandbox *string, policies ...api.Policy) *models.StoredConfig {
	cfg := api.RestAPI{Kind: api.RestAPIKindRestApi}
	cfg.Metadata.Name = handle
	cfg.Spec.DisplayName = handle
	cfg.Spec.Version = "v1.0.0"
	cfg.Spec.Context = "/" + handle
	if len(policies) > 0 {
		cfg.Spec.Policies = &policies
	}
	if sandbox != nil {
		url := "http://sandbox:8080"
		cfg.Spec.Upstream.Sandbox = &api.Upstream{Url: &url}
	}
	if vhostsMain != "" || sandbox != nil {
		cfg.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: vhostsMain, Sandbox: sandbox}
	}
	return &models.StoredConfig{UUID: handle + "-id", Kind: models.KindRestApi, Handle: handle, Configuration: cfg}
}

func dedicatedHostnameRouterConfig(required bool) *config.RouterConfig {
	return dedicatedHostnameRouterConfigFor(required, config.ClientCertificateRequestMtlsHostnames)
}

func dedicatedHostnameRouterConfigFor(required bool, request string) *config.RouterConfig {
	return &config.RouterConfig{
		HTTPSEnabled: true,
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*"},
			Sandbox: config.VHostEntry{Default: "sandbox-*"},
		},
		DownstreamTLS: config.DownstreamTLS{
			MtlsRequiresDedicatedHostname: required,
			ClientCertificateRequest:      request,
		},
	}
}

// warnMessages returns the message of every WARN record written to buf.
func warnMessages(t *testing.T, buf *bytes.Buffer) []string {
	t.Helper()
	var messages []string
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		if record["level"] == "WARN" {
			messages = append(messages, record["msg"].(string))
		}
	}
	return messages
}

// A stored API the setting would refuse keeps being served and is named in
// one startup warning.
func TestWarnMtlsAPIsWithoutDedicatedHostname(t *testing.T) {
	mtls := api.Policy{Name: config.MtlsAuthPolicyName, Version: "v1"}
	sandboxDefault := "sandbox-*"
	configs := []*models.StoredConfig{
		storedRestAPI("no-hostname", "", nil, mtls),
		storedRestAPI("own-hostname", "pay.example.com", nil, mtls),
		storedRestAPI("sandbox-default", "shop.example.com", &sandboxDefault, mtls),
		storedRestAPI("both-default", "", &sandboxDefault, mtls),
		storedRestAPI("no-mtls", "", nil),
	}

	runtimeStore := storage.NewRuntimeConfigStore()
	loaded, err := loadRuntimeConfigsFromExistingAPIConfigurations(configs, runtimeStore, nil,
		&fakeRuntimeTransformer{}, newDiscardLogger(), false)
	require.NoError(t, err)
	assert.Equal(t, len(configs), loaded)

	var buf bytes.Buffer
	warnMtlsAPIsWithoutDedicatedHostname(configs, dedicatedHostnameRouterConfig(true), slog.New(slog.NewJSONHandler(&buf, nil)))

	assert.Equal(t, []string{
		"mtls-auth API no-hostname has no dedicated hostname; the HTTPS listener asks every connection " +
			"for a client certificate; update it to set vhosts.main",
		"mtls-auth API sandbox-default has no dedicated hostname; the HTTPS listener asks every connection " +
			"for a client certificate; update it to set vhosts.sandbox",
		"mtls-auth API both-default has no dedicated hostname; the HTTPS listener asks every connection " +
			"for a client certificate; update it to set vhosts.main and vhosts.sandbox",
	}, warnMessages(t, &buf))
	_, served := runtimeStore.Get(storage.Key(models.KindRestApi, "no-hostname"))
	assert.True(t, served, "the violating API keeps being served")
}

func TestWarnMtlsAPIsWithoutDedicatedHostname_SettingOff(t *testing.T) {
	configs := []*models.StoredConfig{
		storedRestAPI("no-hostname", "", nil, api.Policy{Name: config.MtlsAuthPolicyName, Version: "v1"}),
	}
	var buf bytes.Buffer
	warnMtlsAPIsWithoutDedicatedHostname(configs, dedicatedHostnameRouterConfig(false), slog.New(slog.NewJSONHandler(&buf, nil)))
	assert.Empty(t, warnMessages(t, &buf))
}

// With all_connections the requirement has no effect: one warning says so,
// and no stored API is named.
func TestWarnMtlsAPIsWithoutDedicatedHostname_AllConnections(t *testing.T) {
	configs := []*models.StoredConfig{
		storedRestAPI("no-hostname", "", nil, api.Policy{Name: config.MtlsAuthPolicyName, Version: "v1"}),
		storedRestAPI("also-no-hostname", "", nil, api.Policy{Name: config.MtlsAuthPolicyName, Version: "v1"}),
	}
	const combined = "router.downstream_tls.mtls_requires_dedicated_hostname has no effect while " +
		"router.downstream_tls.client_certificate_request is all_connections"

	t.Run("dedicated hostname required", func(t *testing.T) {
		var buf bytes.Buffer
		warnMtlsAPIsWithoutDedicatedHostname(configs,
			dedicatedHostnameRouterConfigFor(true, config.ClientCertificateRequestAllConnections),
			slog.New(slog.NewJSONHandler(&buf, nil)))
		assert.Equal(t, []string{combined}, warnMessages(t, &buf))
	})
	t.Run("dedicated hostname not required", func(t *testing.T) {
		var buf bytes.Buffer
		warnMtlsAPIsWithoutDedicatedHostname(configs,
			dedicatedHostnameRouterConfigFor(false, config.ClientCertificateRequestAllConnections),
			slog.New(slog.NewJSONHandler(&buf, nil)))
		assert.Empty(t, warnMessages(t, &buf))
	})
}
