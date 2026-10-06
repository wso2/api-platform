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

package xds

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tlsinspectorv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

const scopeTestHTTPSPort = 8443

// scopeAPI describes one deployed REST API for the client certificate
// request tests.
type scopeAPI struct {
	uuid        string
	main        string // vhosts.main; "" leaves vhosts unset
	sandboxURL  bool   // adds a sandbox upstream
	sandboxHost string // vhosts.sandbox; "" leaves it unset
	mtls        string // "", "api" or "operation": where mtls-auth attaches
}

func (a scopeAPI) stored() *models.StoredConfig {
	ctx := "/" + a.uuid
	var cfg *models.StoredConfig
	if a.mtls == "operation" {
		cfg = makeRestAPIWithOperationLevelMTLSAuth(a.uuid, a.uuid, ctx)
	} else {
		cfg = makeRestAPI(a.uuid, a.uuid, ctx)
	}
	rest := cfg.Configuration.(api.RestAPI)
	if a.mtls == "api" {
		rest.Spec.Policies = &[]api.Policy{{Name: "mtls-auth", Version: "v1"}}
	}
	if a.main != "" || a.sandboxHost != "" {
		rest.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: a.main}
		if a.sandboxHost != "" {
			rest.Spec.Vhosts.Sandbox = api.Ptr(a.sandboxHost)
		}
	}
	if a.sandboxURL {
		rest.Spec.Upstream.Sandbox = &api.Upstream{Url: api.Ptr("http://sandbox-backend:8080")}
	}
	cfg.Configuration = rest
	cfg.SourceConfiguration = rest
	return cfg
}

func storedConfigs(apis ...scopeAPI) []*models.StoredConfig {
	out := make([]*models.StoredConfig, 0, len(apis))
	for _, a := range apis {
		out = append(out, a.stored())
	}
	return out
}

func newScopeTestTranslator(t *testing.T, db *fakeSDSStorage) *Translator {
	t.Helper()
	return newScopeTestTranslatorFor(t, db, config.ClientCertificateRequestMtlsHostnames)
}

// newScopeTestTranslatorFor builds the test translator with
// router.downstream_tls.client_certificate_request set to request.
func newScopeTestTranslatorFor(t *testing.T, db *fakeSDSStorage, request string) *Translator {
	t.Helper()
	routerCfg := testRouterConfig()
	routerCfg.HTTPSEnabled = true
	routerCfg.HTTPSPort = scopeTestHTTPSPort
	routerCfg.DownstreamTLS.ClientCertificateRequest = request
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(createTestLogger(), routerCfg, db, cfg)
	require.NoError(t, err)
	return translator
}

func clientPool(t *testing.T, relay bool) *fakeSDSStorage {
	t.Helper()
	certs := []*models.StoredCertificate{{
		UUID: "client-1", Name: "partner-ca", Usage: models.CertificateUsageDownstream,
		Role: models.CertificateRoleClient, Certificate: pki.NewRootCA(t, "Scope Client CA").PEM(),
	}}
	if relay {
		certs = append(certs, relayEntry(t))
	}
	return &fakeSDSStorage{certs: certs}
}

func relayEntry(t *testing.T) *models.StoredCertificate {
	t.Helper()
	return &models.StoredCertificate{
		UUID: "relay-1", Name: "edge-lb", Usage: models.CertificateUsageDownstream,
		Role: models.CertificateRoleRelay, Certificate: pki.NewRootCA(t, "Scope Relay CA").PEM(),
	}
}

func translateHTTPSListener(t *testing.T, translator *Translator, configs []*models.StoredConfig) *listener.Listener {
	t.Helper()
	resources, err := translator.TranslateConfigs(configs, "")
	require.NoError(t, err)
	return findListenerByPort(t, resources[resource.ListenerType], scopeTestHTTPSPort)
}

func chainTLSContext(t *testing.T, fc *listener.FilterChain) *tlsv3.DownstreamTlsContext {
	t.Helper()
	typedConfig := fc.GetTransportSocket().GetTypedConfig()
	require.NotNil(t, typedConfig)
	var tlsCtx tlsv3.DownstreamTlsContext
	require.NoError(t, typedConfig.UnmarshalTo(&tlsCtx))
	return &tlsCtx
}

func TestTranslator_ClientCertificateRequest_Mode(t *testing.T) {
	tests := []struct {
		name      string
		apis      []scopeAPI
		emptyPool bool
		relay     bool
		wantMode  clientCertMode
		wantNames []string
	}{
		{
			name:     "no API attaches mtls-auth",
			apis:     []scopeAPI{{uuid: "plain", main: "public.example.com"}},
			wantMode: clientCertOff,
		},
		{
			name:      "empty client-CA pool",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}},
			emptyPool: true,
			wantMode:  clientCertOff,
		},
		{
			name:      "one API on one hostname",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com"},
		},
		{
			name: "several APIs, sorted and de-duplicated",
			apis: []scopeAPI{
				{uuid: "settle", main: "settle.example.com", mtls: "api"},
				{uuid: "pay", main: "pay.example.com", mtls: "api"},
				{uuid: "pay-v2", main: "pay.example.com", mtls: "api"},
			},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com", "settle.example.com"},
		},
		{
			name:      "semicolon-separated hostnames",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com; Pay-EU.example.com", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay-eu.example.com", "pay.example.com"},
		},
		{
			name:      "leading wildcard hostname",
			apis:      []scopeAPI{{uuid: "pay", main: "*.partners.example.com", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"*.partners.example.com"},
		},
		{
			name:      "hostname with a port",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com:8443", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com"},
		},
		{
			name:      "operation-level attachment",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "operation"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com"},
		},
		{
			name: "APIs without mtls-auth never add hostnames",
			apis: []scopeAPI{
				{uuid: "pay", main: "pay.example.com", mtls: "api"},
				{uuid: "plain", main: "public.example.com"},
				{uuid: "plain-default"},
			},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com"},
		},
		{
			name:     "API without vhosts uses the main default",
			apis:     []scopeAPI{{uuid: "pay", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "API naming the main default explicitly",
			apis:     []scopeAPI{{uuid: "pay", main: "localhost", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name: "one default-hostname API among scoped ones",
			apis: []scopeAPI{
				{uuid: "pay", main: "pay.example.com", mtls: "api"},
				{uuid: "legacy", mtls: "operation"},
			},
			wantMode: clientCertEverywhere,
		},
		{
			name:      "sandbox upstream with its own hostname",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com", sandboxURL: true, sandboxHost: "pay-sandbox.example.com", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay-sandbox.example.com", "pay.example.com"},
		},
		{
			name:     "sandbox upstream on the sandbox default",
			apis:     []scopeAPI{{uuid: "pay", main: "pay.example.com", sandboxURL: true, mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:      "sandbox hostname without a sandbox upstream is not served",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com", sandboxHost: "sandbox-*", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com"},
		},
		{
			name:     "catch-all hostname",
			apis:     []scopeAPI{{uuid: "pay", main: "*", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "suffix wildcard hostname",
			apis:     []scopeAPI{{uuid: "pay", main: "pay-*", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "inner wildcard hostname",
			apis:     []scopeAPI{{uuid: "pay", main: "pay.*.example.com", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "IP address hostname",
			apis:     []scopeAPI{{uuid: "pay", main: "10.0.0.5", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "relay entry in the pool",
			apis:     []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}},
			relay:    true,
			wantMode: clientCertEverywhere,
		},
		{
			name:     "gateway-default sentinel as vhosts.main",
			apis:     []scopeAPI{{uuid: "pay", main: constants.VHostGatewayDefault, mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "gateway-default sentinel as vhosts.sandbox",
			apis:     []scopeAPI{{uuid: "pay", main: "pay.example.com", sandboxURL: true, sandboxHost: constants.VHostGatewayDefault, mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:      "sandbox hostname with surrounding whitespace",
			apis:      []scopeAPI{{uuid: "pay", main: "pay.example.com", sandboxURL: true, sandboxHost: " pay-sandbox.example.com ", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay-sandbox.example.com", "pay.example.com"},
		},
		{
			name:     "trailing dot",
			apis:     []scopeAPI{{uuid: "pay", main: "pay.example.com.", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:     "non-DNS characters",
			apis:     []scopeAPI{{uuid: "pay", main: "pay_api.example.com", mtls: "api"}},
			wantMode: clientCertEverywhere,
		},
		{
			name:      "uppercase hostname is lower-cased",
			apis:      []scopeAPI{{uuid: "pay", main: "PAY.Example.COM", mtls: "api"}},
			wantMode:  clientCertScoped,
			wantNames: []string{"pay.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := clientPool(t, tt.relay)
			if tt.emptyPool {
				db = &fakeSDSStorage{}
			}
			translator := newScopeTestTranslator(t, db)

			got, err := translator.clientCertificateRequest(storedConfigs(tt.apis...))
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, got.mode)
			assert.Equal(t, tt.wantNames, got.serverNames)
		})
	}
}

// A pool entry stored without a role is a client authority, not a relay.
func TestTranslator_ClientCertificateRequest_EntryWithoutRoleIsClient(t *testing.T) {
	db := clientPool(t, false)
	db.certs[0].Role = ""
	translator := newScopeTestTranslator(t, db)

	got, err := translator.clientCertificateRequest(storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"}))
	require.NoError(t, err)
	assert.Equal(t, clientCertScoped, got.mode)
}

// Scoping follows the rendered configuration, not the stored source.
func TestTranslator_ClientCertificateRequest_UsesRenderedConfiguration(t *testing.T) {
	translator := newScopeTestTranslator(t, clientPool(t, false))
	configs := storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"})
	source := configs[0].Configuration.(api.RestAPI)
	source.Spec.Vhosts = &struct {
		Main    string  `json:"main" yaml:"main"`
		Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	}{Main: `{{ env "PAY_HOST" }}`}
	configs[0].SourceConfiguration = source

	got, err := translator.clientCertificateRequest(configs)
	require.NoError(t, err)
	assert.Equal(t, clientCertScoped, got.mode)
	assert.Equal(t, []string{"pay.example.com"}, got.serverNames)
}

// An undeployed mtls-auth API contributes neither asking nor hostnames.
func TestTranslator_ClientCertificateRequest_UndeployedAPIIgnored(t *testing.T) {
	translator := newScopeTestTranslator(t, clientPool(t, false))
	configs := storedConfigs(
		scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"},
		scopeAPI{uuid: "legacy", mtls: "api"},
	)
	configs[1].DesiredState = models.StateUndeployed

	got, err := translator.clientCertificateRequest(configs)
	require.NoError(t, err)
	assert.Equal(t, clientCertScoped, got.mode)
	assert.Equal(t, []string{"pay.example.com"}, got.serverNames)
}

// tlsTransportSocket wraps a downstream TLS context in the TLS transport
// socket, built here rather than by the code under test.
func tlsTransportSocket(t *testing.T, translator *Translator, requestClientCertificate bool) *core.TransportSocket {
	t.Helper()
	tlsCtx, err := translator.createDownstreamTLSContext(requestClientCertificate)
	require.NoError(t, err)
	tlsAny, err := anypb.New(tlsCtx)
	require.NoError(t, err)
	return &core.TransportSocket{
		Name:       "envoy.transport_sockets.tls",
		ConfigType: &core.TransportSocket_TypedConfig{TypedConfig: tlsAny},
	}
}

// singleChainHTTPSListener builds the HTTPS listener with one unnamed,
// unmatched filter chain from the HTTP listener of the same translation: the
// same address, buffer limit and network filters, on the HTTPS port, with
// the TLS Inspector and a TLS transport socket that asks for a client
// certificate when requestClientCertificate is true.
func singleChainHTTPSListener(t *testing.T, translator *Translator, httpListener *listener.Listener, requestClientCertificate bool) *listener.Listener {
	t.Helper()
	want := proto.Clone(httpListener).(*listener.Listener)
	want.Name = fmt.Sprintf("listener_https_%d", scopeTestHTTPSPort)
	want.GetAddress().GetSocketAddress().PortSpecifier = &core.SocketAddress_PortValue{PortValue: scopeTestHTTPSPort}
	inspectorAny, err := anypb.New(&tlsinspectorv3.TlsInspector{})
	require.NoError(t, err)
	want.ListenerFilters = []*listener.ListenerFilter{{
		Name:       "envoy.filters.listener.tls_inspector",
		ConfigType: &listener.ListenerFilter_TypedConfig{TypedConfig: inspectorAny},
	}}
	require.Len(t, want.GetFilterChains(), 1)
	want.FilterChains[0].TransportSocket = tlsTransportSocket(t, translator, requestClientCertificate)
	return want
}

// OFF and EVERYWHERE build one filter chain with no name and no match, whose
// TLS context asks every connection for a client certificate or none.
func TestTranslator_HTTPSListener_SingleChainModes(t *testing.T) {
	tests := []struct {
		name  string
		apis  []scopeAPI
		relay bool
		asks  bool
	}{
		{name: "off", apis: []scopeAPI{{uuid: "plain", main: "public.example.com"}}},
		{name: "everywhere by default hostname", apis: []scopeAPI{{uuid: "pay", mtls: "api"}}, asks: true},
		{name: "everywhere by relay", apis: []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}}, relay: true, asks: true},
		{name: "everywhere by gateway-default sentinel", apis: []scopeAPI{{uuid: "pay", main: constants.VHostGatewayDefault, mtls: "api"}}, asks: true},
		{name: "everywhere by trailing dot", apis: []scopeAPI{{uuid: "pay", main: "pay.example.com.", mtls: "api"}}, asks: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator := newScopeTestTranslator(t, clientPool(t, tt.relay))
			resources, err := translator.TranslateConfigs(storedConfigs(tt.apis...), "")
			require.NoError(t, err)
			got := findListenerByPort(t, resources[resource.ListenerType], scopeTestHTTPSPort)
			httpListener := findListenerByPort(t, resources[resource.ListenerType], translator.routerConfig.ListenerPort)

			want := singleChainHTTPSListener(t, translator, httpListener, tt.asks)
			assert.True(t, proto.Equal(want, got), "HTTPS listener differs from the single-chain listener")
			require.Len(t, got.GetFilterChains(), 1)
			assert.Nil(t, got.GetFilterChains()[0].GetFilterChainMatch())
			assert.Empty(t, got.GetFilterChains()[0].GetName())
		})
	}
}

func TestTranslator_HTTPSListener_Scoped(t *testing.T) {
	translator := newScopeTestTranslator(t, clientPool(t, false))
	got := translateHTTPSListener(t, translator, storedConfigs(
		scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"},
		scopeAPI{uuid: "plain"},
	))

	chains := got.GetFilterChains()
	require.Len(t, chains, 2)
	asking, fallback := chains[0], chains[1]

	assert.Equal(t, askingFilterChainName, asking.GetName())
	assert.Equal(t, []string{"pay.example.com"}, asking.GetFilterChainMatch().GetServerNames())
	assert.Equal(t, defaultFilterChainName, fallback.GetName())
	assert.Nil(t, fallback.GetFilterChainMatch(), "the default chain must catch every other SNI and a missing SNI")
	assert.Nil(t, got.GetDefaultFilterChain())

	t.Run("asking chain carries the asking TLS context", func(t *testing.T) {
		assert.True(t, proto.Equal(tlsTransportSocket(t, translator, true), asking.GetTransportSocket()))

		tlsCtx := chainTLSContext(t, asking)
		assert.Equal(t, SecretNameDownstreamClientCA, tlsCtx.GetCommonTlsContext().GetValidationContextSdsSecretConfig().GetName())
		require.NotNil(t, tlsCtx.GetRequireClientCertificate())
		assert.False(t, tlsCtx.GetRequireClientCertificate().GetValue())
		assert.True(t, tlsCtx.GetDisableStatelessSessionResumption())
		assert.True(t, tlsCtx.GetDisableStatefulSessionResumption())
	})

	t.Run("default chain never asks and resumes sessions", func(t *testing.T) {
		assert.True(t, proto.Equal(tlsTransportSocket(t, translator, false), fallback.GetTransportSocket()))

		tlsCtx := chainTLSContext(t, fallback)
		assert.Nil(t, tlsCtx.GetCommonTlsContext().GetValidationContextType())
		assert.Nil(t, tlsCtx.GetRequireClientCertificate())
		assert.False(t, tlsCtx.GetDisableStatelessSessionResumption())
		assert.False(t, tlsCtx.GetDisableStatefulSessionResumption())
		assert.Equal(t, SecretNameDownstreamListenerCert, tlsCtx.GetCommonTlsContext().GetTlsCertificateSdsSecretConfigs()[0].GetName())
	})

	t.Run("both chains carry identical network filters", func(t *testing.T) {
		require.Len(t, asking.GetFilters(), len(fallback.GetFilters()))
		for i := range asking.GetFilters() {
			assert.True(t, proto.Equal(asking.GetFilters()[i], fallback.GetFilters()[i]), "filter %d differs", i)
		}
		for _, fc := range chains {
			manager := &hcm.HttpConnectionManager{}
			require.NoError(t, fc.GetFilters()[0].GetTypedConfig().UnmarshalTo(manager))
			assert.Equal(t, SharedRouteConfigName, manager.GetRds().GetRouteConfigName())
			assert.True(t, manager.GetNormalizePath().GetValue())
			assert.True(t, manager.GetMergeSlashes())
			assert.Equal(t, hcm.HttpConnectionManager_SANITIZE_SET, manager.GetForwardClientCertDetails())
			assert.NotNil(t, manager.GetSetCurrentClientCertDetails())
		}
	})

	t.Run("TLS inspector is present", func(t *testing.T) {
		require.Len(t, got.GetListenerFilters(), 1)
		assert.Equal(t, "envoy.filters.listener.tls_inspector", got.GetListenerFilters()[0].GetName())
	})

	t.Run("snapshot still serves the client-CA secret", func(t *testing.T) {
		assert.True(t, SnapshotReferencesSDSSecret(nil, []types.Resource{got}, SecretNameDownstreamClientCA))
	})
}

// The same inputs in any order produce the same HTTPS listener.
func TestTranslator_HTTPSListener_Scoped_Deterministic(t *testing.T) {
	apis := []scopeAPI{
		{uuid: "settle", main: "settle.example.com;pay.example.com", mtls: "api"},
		{uuid: "pay", main: "pay.example.com", mtls: "operation"},
		{uuid: "plain", main: "public.example.com"},
	}
	reversed := []scopeAPI{apis[2], apis[1], apis[0]}

	translator := newScopeTestTranslator(t, clientPool(t, false))
	first := translateHTTPSListener(t, translator, storedConfigs(apis...))
	second := translateHTTPSListener(t, translator, storedConfigs(reversed...))

	assert.True(t, proto.Equal(first, second))
	assert.Equal(t, []string{"pay.example.com", "settle.example.com"},
		first.GetFilterChains()[0].GetFilterChainMatch().GetServerNames())
}

// A relay entry turns asking on for every connection; removing it scopes
// asking again.
func TestTranslator_HTTPSListener_RelayEntryAddedAndRemoved(t *testing.T) {
	db := clientPool(t, false)
	translator := newScopeTestTranslator(t, db)
	configs := storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"})

	require.Len(t, translateHTTPSListener(t, translator, configs).GetFilterChains(), 2)

	db.certs = append(db.certs, relayEntry(t))
	everywhere := translateHTTPSListener(t, translator, configs)
	require.Len(t, everywhere.GetFilterChains(), 1)
	assert.NotNil(t, chainTLSContext(t, everywhere.GetFilterChains()[0]).GetCommonTlsContext().GetValidationContextSdsSecretConfig())

	db.certs = db.certs[:1]
	scoped := translateHTTPSListener(t, translator, configs)
	require.Len(t, scoped.GetFilterChains(), 2)
	assert.Equal(t, []string{"pay.example.com"}, scoped.GetFilterChains()[0].GetFilterChainMatch().GetServerNames())
}

// Removing the last mtls-auth API leaves one chain that never asks.
func TestTranslator_HTTPSListener_LastMTLSAPIRemoved(t *testing.T) {
	translator := newScopeTestTranslator(t, clientPool(t, false))
	pay := scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"}
	plain := scopeAPI{uuid: "plain"}

	require.Len(t, translateHTTPSListener(t, translator, storedConfigs(pay, plain)).GetFilterChains(), 2)

	off := translateHTTPSListener(t, translator, storedConfigs(plain))
	require.Len(t, off.GetFilterChains(), 1)
	assert.Nil(t, chainTLSContext(t, off.GetFilterChains()[0]).GetCommonTlsContext().GetValidationContextType())
}

// The HTTP listener has one chain whatever the HTTPS listener's mode.
func TestTranslator_HTTPListener_UnaffectedByScoping(t *testing.T) {
	translator := newScopeTestTranslator(t, clientPool(t, false))
	resources, err := translator.TranslateConfigs(storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"}), "")
	require.NoError(t, err)

	httpListener := findListenerByPort(t, resources[resource.ListenerType], translator.routerConfig.ListenerPort)
	want, _, err := translator.createListener(nil, false, false)
	require.NoError(t, err)
	assert.True(t, proto.Equal(want, httpListener))
}

// A change of mode is logged once, with the hostname count and no hostname.
func TestTranslator_ClientCertificateRequest_LogsModeChange(t *testing.T) {
	var buf bytes.Buffer
	translator := newScopeTestTranslator(t, clientPool(t, false))
	translator.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	changes := func() []string {
		var out []string
		for _, line := range strings.Split(buf.String(), "\n") {
			if strings.Contains(line, "client certificate request changed") {
				out = append(out, line)
			}
		}
		return out
	}

	scoped := storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com;pay-eu.example.com", mtls: "api"})
	translateHTTPSListener(t, translator, scoped)
	translateHTTPSListener(t, translator, scoped)
	require.Len(t, changes(), 1)
	assert.Contains(t, changes()[0], "mode=SCOPED")
	assert.Contains(t, changes()[0], "hostname_count=2")
	assert.Contains(t, changes()[0], "client_certificate_request=mtls_hostnames")
	assert.NotContains(t, changes()[0], "pay.example.com")

	translateHTTPSListener(t, translator, storedConfigs(scopeAPI{uuid: "pay", mtls: "api"}))
	require.Len(t, changes(), 2)
	assert.Contains(t, changes()[1], "mode=EVERYWHERE")

	translateHTTPSListener(t, translator, nil)
	require.Len(t, changes(), 3)
	assert.Contains(t, changes()[2], "mode=OFF")
}

// A listener build that fails neither logs nor records the new mode, so the
// next successful build logs it.
func TestTranslator_ClientCertificateRequest_FailedBuildNotRecorded(t *testing.T) {
	var buf bytes.Buffer
	translator := newScopeTestTranslator(t, clientPool(t, false))
	translator.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	scoped := storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"})

	scriptPath := translator.routerConfig.LuaScriptPath
	translator.routerConfig.LuaScriptPath = t.TempDir() + "/missing.lua"
	_, err := translator.TranslateConfigs(scoped, "")
	require.Error(t, err)
	assert.NotContains(t, buf.String(), "client certificate request changed")
	assert.Equal(t, clientCertOff, translator.lastClientCertRequest.mode)

	translator.routerConfig.LuaScriptPath = scriptPath
	translateHTTPSListener(t, translator, scoped)
	assert.Equal(t, 1, strings.Count(buf.String(), "client certificate request changed"))
	assert.Contains(t, buf.String(), "mode=SCOPED")
}

// With all_connections, every case that asks builds the single asking chain
// the listener builds when it asks everywhere, and every case that asks
// nothing stays OFF.
func TestTranslator_HTTPSListener_AllConnections(t *testing.T) {
	tests := []struct {
		name      string
		apis      []scopeAPI
		emptyPool bool
		relay     bool
		asks      bool
	}{
		{name: "scopable hostname", apis: []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}}, asks: true},
		{name: "several scopable hostnames", apis: []scopeAPI{
			{uuid: "pay", main: "pay.example.com;pay-eu.example.com", mtls: "api"},
			{uuid: "settle", main: "*.settle.example.com", mtls: "operation"},
			{uuid: "plain", main: "public.example.com"},
		}, asks: true},
		{name: "default hostname", apis: []scopeAPI{{uuid: "pay", mtls: "api"}}, asks: true},
		{name: "relay entry", apis: []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}}, relay: true, asks: true},
		{name: "no API attaches mtls-auth", apis: []scopeAPI{{uuid: "plain", main: "public.example.com"}}},
		{name: "no API deployed"},
		{name: "empty client-CA pool", apis: []scopeAPI{{uuid: "pay", main: "pay.example.com", mtls: "api"}}, emptyPool: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := clientPool(t, tt.relay)
			if tt.emptyPool {
				db = &fakeSDSStorage{}
			}
			translator := newScopeTestTranslatorFor(t, db, config.ClientCertificateRequestAllConnections)
			configs := storedConfigs(tt.apis...)

			req, err := translator.clientCertificateRequest(configs)
			require.NoError(t, err)
			if tt.asks {
				assert.Equal(t, clientCertRequest{mode: clientCertEverywhere}, req)
			} else {
				assert.Equal(t, clientCertRequest{mode: clientCertOff}, req)
			}

			resources, err := translator.TranslateConfigs(configs, "")
			require.NoError(t, err)
			got := findListenerByPort(t, resources[resource.ListenerType], scopeTestHTTPSPort)
			httpListener := findListenerByPort(t, resources[resource.ListenerType], translator.routerConfig.ListenerPort)
			want := singleChainHTTPSListener(t, translator, httpListener, tt.asks)
			assert.True(t, proto.Equal(want, got), "HTTPS listener differs from the single-chain listener")
		})
	}
}

// The mode-change log carries the configured client_certificate_request.
func TestTranslator_ClientCertificateRequest_LogsSetting(t *testing.T) {
	var buf bytes.Buffer
	translator := newScopeTestTranslatorFor(t, clientPool(t, false), config.ClientCertificateRequestAllConnections)
	translator.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	translateHTTPSListener(t, translator, storedConfigs(scopeAPI{uuid: "pay", main: "pay.example.com", mtls: "api"}))
	require.Equal(t, 1, strings.Count(buf.String(), "client certificate request changed"))
	assert.Contains(t, buf.String(), "mode=EVERYWHERE")
	assert.Contains(t, buf.String(), "hostname_count=0")
	assert.Contains(t, buf.String(), "client_certificate_request=all_connections")
}
