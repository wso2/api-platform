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

package transform

import (
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// certificateRows serves the certificate lookups the translator makes.
type certificateRows struct {
	storage.Storage
	certs []*models.StoredCertificate
}

func (c *certificateRows) ListCertificates() ([]*models.StoredCertificate, error) {
	return c.certs, nil
}

func (c *certificateRows) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	var out []*models.StoredCertificate
	for _, cert := range c.certs {
		if cert.EffectiveUsage() == usage {
			out = append(out, cert)
		}
	}
	return out, nil
}

func (c *certificateRows) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	for _, cert := range c.certs {
		if cert.Name == name {
			return cert, nil
		}
	}
	return nil, fmt.Errorf("certificate %q not found", name)
}

func httpsUpstreamDefinition(name, host string, tls *map[string]interface{}) api.UpstreamDefinition {
	return api.UpstreamDefinition{Name: name, Tls: tls, Upstreams: []struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{{Url: "https://" + host + ":8443"}}}
}

// restAPIWithEveryUpstreamShape has an inline https main and sandbox, a
// definition without a tls block, one whose tls block only sets trustedCAs,
// one whose tls block only sets verifyHostName, and one naming an identity.
func restAPIWithEveryUpstreamShape(uuid, kind string, source any) *models.StoredConfig {
	trusted := map[string]interface{}{"trustedCAs": []interface{}{"backend-ca"}}
	verifyOnly := map[string]interface{}{"verifyHostName": false}
	named := map[string]interface{}{"identity": "partner-identity"}
	defs := []api.UpstreamDefinition{
		httpsUpstreamDefinition("plain", uuid+"-plain.backend", nil),
		httpsUpstreamDefinition("trusted", uuid+"-trusted.backend", &trusted),
		httpsUpstreamDefinition("verify", uuid+"-verify.backend", &verifyOnly),
		httpsUpstreamDefinition("named", uuid+"-named.backend", &named),
	}
	spec := api.APIConfigData{
		DisplayName:         uuid,
		Context:             "/" + uuid,
		Version:             "v1.0",
		UpstreamDefinitions: &defs,
		Operations:          []api.Operation{{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/hello")}},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main:    api.Upstream{Url: ptrStr("https://" + uuid + "-main.backend:8443")},
			Sandbox: &api.Upstream{Url: ptrStr("https://" + uuid + "-sandbox.backend:8443")},
		},
	}
	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                kind,
		Handle:              uuid,
		DesiredState:        models.StateDeployed,
		Configuration:       api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: uuid}, Spec: spec},
		SourceConfiguration: source,
	}
}

// encryptedTestKey returns an encryption manager and a private key
// ciphertext it decrypts, as a usage: identity row stores it.
func encryptedTestKey(t *testing.T) (*encryption.ProviderManager, string) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "v1.key")
	key := make([]byte, aesgcm.AESKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, key, 0o600))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider, err := aesgcm.NewAESGCMProvider([]aesgcm.KeyConfig{{Version: "v1", FilePath: keyPath}}, logger)
	require.NoError(t, err)
	mgr, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, logger)
	require.NoError(t, err)
	payload, err := mgr.Encrypt([]byte("-----BEGIN PRIVATE KEY-----\nkey\n-----END PRIVATE KEY-----\n"))
	require.NoError(t, err)
	return mgr, encryption.MarshalPayload(payload)
}

// presentedClientCertificates maps each cluster to the SDS secrets its
// endpoints present as a client certificate.
func presentedClientCertificates(t *testing.T, translator *xds.Translator, configs []*models.StoredConfig) map[string][]string {
	t.Helper()
	resources, err := translator.TranslateConfigs(configs, "")
	require.NoError(t, err)
	out := map[string][]string{}
	for _, res := range resources[resource.ClusterType] {
		c := res.(*cluster.Cluster)
		var names []string
		for _, tsm := range c.GetTransportSocketMatches() {
			var tlsCtx tlsv3.UpstreamTlsContext
			require.NoError(t, tsm.GetTransportSocket().GetTypedConfig().UnmarshalTo(&tlsCtx))
			for _, sds := range tlsCtx.GetCommonTlsContext().GetTlsCertificateSdsSecretConfigs() {
				names = append(names, sds.GetName())
			}
		}
		out[c.GetName()] = names
	}
	return out
}

// Every API kind's upstreams without a tls identity present the default
// identity, through the real transformers.
func TestDefaultClientCertificate_EveryAPIKind(t *testing.T) {
	routerCfg := testRouterCfg()
	routerCfg.Upstream.TLS.PresentDefaultIdentity = true
	routerCfg.LuaScriptPath = "../../lua/request_transformation.lua"
	systemCfg := &config.Config{Router: *routerCfg}
	mgr, ciphertext := encryptedTestKey(t)
	db := &certificateRows{certs: []*models.StoredCertificate{
		{UUID: "ca-1", Name: "backend-ca", Certificate: pki.NewRootCA(t, "Backend CA").PEM(), Usage: models.CertificateUsageUpstream},
		{UUID: "named-1", Name: "partner-identity", Certificate: pki.NewSelfSignedLeaf(t, "partner").PEM(), Usage: models.CertificateUsageIdentity,
			PrivateKeyCiphertext: ciphertext},
		{UUID: "default-1", Name: "gateway-default", Certificate: pki.NewSelfSignedLeaf(t, "gateway").PEM(),
			Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault, PrivateKeyCiphertext: ciphertext},
	}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	translator, err := xds.NewTranslator(logger, routerCfg, db, systemCfg)
	require.NoError(t, err)
	translator.GetCertStore().SetEncryptionManager(mgr)

	restT := NewRestAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{})
	llmT := NewLLMTransformer(storage.NewConfigStore(), db, routerCfg, systemCfg, map[string]models.PolicyDefinition{}, nil)
	agentT := agentTransformer()
	registry := NewRegistry(restT, llmT, agentT)
	transformers := map[string]models.ConfigTransformer{}
	for _, kind := range []string{models.KindRestApi, models.KindMcp, models.KindLlmProvider, models.KindAgent} {
		transformers[kind] = registry
	}
	translator.SetTransformers(transformers)

	agent := testAgent()
	agent.DesiredState = models.StateDeployed
	configs := []*models.StoredConfig{
		restAPIWithEveryUpstreamShape("rest", models.KindRestApi, nil),
		restAPIWithEveryUpstreamShape("mcp", models.KindMcp, api.MCPProxyConfiguration{}),
		restAPIWithEveryUpstreamShape("llm", models.KindLlmProvider, api.LLMProviderConfiguration{}),
		agent,
	}
	presented := presentedClientCertificates(t, translator, configs)

	defaultSecret := []string{xds.GatewayIdentitySecretName("gateway-default")}
	want := map[string][]string{
		"api-platform/policy-engine":         nil,
		"upstream_main_weather.internal_443": defaultSecret,
	}
	for prefix, kind := range map[string]string{"rest": "RestApi", "mcp": "Mcp", "llm": "LlmProvider"} {
		want["upstream_main_"+prefix+"-main.backend_8443"] = defaultSecret
		want["upstream_sandbox_"+prefix+"-sandbox.backend_8443"] = defaultSecret
		want["upstream_"+kind+"_"+prefix+"_plain"] = defaultSecret
		want["upstream_"+kind+"_"+prefix+"_trusted"] = defaultSecret
		want["upstream_"+kind+"_"+prefix+"_verify"] = defaultSecret
		want["upstream_"+kind+"_"+prefix+"_named"] = []string{xds.GatewayIdentitySecretName("partner-identity")}
	}
	assert.Equal(t, want, presented)
}

// Every upstream TLS model built for an API's traffic is marked as such,
// with or without a tls block.
func TestUpstreamTLSFromParams_MarksAPITraffic(t *testing.T) {
	verifyOnly := map[string]interface{}{"verifyHostName": false}
	for name, tls := range map[string]*map[string]interface{}{"no tls block": nil, "tls block": &verifyOnly} {
		for _, enabled := range []bool{true, false} {
			assert.True(t, upstreamTLSFromParams(tls, enabled).APITraffic, "%s, https %v", name, enabled)
		}
	}
}
