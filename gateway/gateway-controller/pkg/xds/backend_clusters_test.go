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

package xds

import (
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type staticBackendSource []BackendCluster

func (s staticBackendSource) BackendClusters() []BackendCluster { return s }

func clustersByName(t *testing.T, translator *Translator) map[string]*cluster.Cluster {
	t.Helper()
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
	require.NoError(t, err)
	out := map[string]*cluster.Cluster{}
	for _, r := range resources[resourcev3.ClusterType] {
		c := r.(*cluster.Cluster)
		out[c.Name] = c
	}
	return out
}

func TestTranslateConfigs_EmitsOneClusterPerGlobalBackend(t *testing.T) {
	translator := createTestTranslator()
	weightA, weightB := 80, 20
	connect := 2 * time.Second
	translator.SetBackendSource(staticBackendSource{
		{Name: "tenant-a", ClusterName: "backend_tenant-a", Upstream: &models.UpstreamCluster{
			Name: "tenant-a", BasePath: "/v1", ConnectTimeout: &connect,
			Endpoints: []models.Endpoint{{Host: "a1", Port: 9080, Weight: &weightA}, {Host: "a2", Port: 9080, Weight: &weightB}},
		}},
		{Name: "tenant-b", ClusterName: "backend_tenant-b", Upstream: &models.UpstreamCluster{
			Name: "tenant-b", BasePath: "/", Endpoints: []models.Endpoint{{Host: "b", Port: 80}},
		}},
	})

	clusters := clustersByName(t, translator)
	a, ok := clusters["backend_tenant-a"]
	require.True(t, ok, "a Backend's cluster is emitted even when no API routes to it yet")
	lbs := a.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints()
	require.Len(t, lbs, 2, "every endpoint of a weighted Backend is kept")
	assert.Equal(t, uint32(80), lbs[0].GetLoadBalancingWeight().GetValue())
	assert.Equal(t, 2*time.Second, a.GetConnectTimeout().AsDuration())

	_, ok = clusters["backend_tenant-b"]
	assert.True(t, ok)
}

func TestTranslateConfigs_SkipsABackendThatCannotBeBuilt(t *testing.T) {
	translator := createTestTranslator()
	translator.SetBackendSource(staticBackendSource{
		{Name: "empty", ClusterName: "backend_empty", Upstream: &models.UpstreamCluster{Name: "empty"}},
		{Name: "tenant-b", ClusterName: "backend_tenant-b", Upstream: &models.UpstreamCluster{
			Name: "tenant-b", Endpoints: []models.Endpoint{{Host: "b", Port: 80}},
		}},
	})
	clusters := clustersByName(t, translator)
	_, emptyPresent := clusters["backend_empty"]
	_, bPresent := clusters["backend_tenant-b"]
	assert.False(t, emptyPresent)
	assert.True(t, bPresent, "one bad Backend never blocks the others")
}

func TestTranslateConfigs_BackendTrustedCAsUseTheirOwnSecret(t *testing.T) {
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.DisableSslVerification = false // validation contexts are built only with verification on
	translator, err := NewTranslator(createTestLogger(), routerCfg, nil, testConfig())
	require.NoError(t, err)
	translator.SetBackendSource(staticBackendSource{
		{Name: "secure", ClusterName: "backend_secure", Upstream: &models.UpstreamCluster{
			Name: "secure", Endpoints: []models.Endpoint{{Host: "s", Port: 443}},
			TLS: &models.UpstreamTLS{Enabled: true, HasTLSBlock: true, TrustedCANames: []string{"partner-ca"}, VerifyHostName: true},
		}},
	})
	clusters := clustersByName(t, translator)

	refs := translator.GetUpstreamTLSSecretRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "backend_ca:secure", refs[0].validationContextSecretName())
	assert.Equal(t, []string{"partner-ca"}, refs[0].TrustedCANames)

	// The emitted cluster itself must trust that secret, not the gateway-wide bundle.
	secure, ok := clusters["backend_secure"]
	require.True(t, ok)
	sockets := []*corev3.TransportSocket{secure.GetTransportSocket()}
	for _, match := range secure.GetTransportSocketMatches() {
		sockets = append(sockets, match.GetTransportSocket())
	}
	checked := 0
	for _, socket := range sockets {
		if socket == nil {
			continue
		}
		var tlsCtx tlsv3.UpstreamTlsContext
		require.NoError(t, socket.GetTypedConfig().UnmarshalTo(&tlsCtx))
		assert.Equal(t, "backend_ca:secure",
			tlsCtx.GetCommonTlsContext().GetCombinedValidationContext().GetValidationContextSdsSecretConfig().GetName())
		checked++
	}
	assert.Positive(t, checked, "the cluster has a TLS transport socket")
}

// A source entry without an upstream is skipped and logged instead of stopping the translation.
func TestTranslateConfigs_SkipsABackendWithoutAnUpstream(t *testing.T) {
	translator := createTestTranslator()
	translator.SetBackendSource(staticBackendSource{
		{Name: "broken", ClusterName: "backend_broken"},
		{Name: "tenant-b", ClusterName: "backend_tenant-b", Upstream: &models.UpstreamCluster{
			Name: "tenant-b", Endpoints: []models.Endpoint{{Host: "b", Port: 80}},
		}},
	})
	clusters := clustersByName(t, translator)
	_, brokenPresent := clusters["backend_broken"]
	_, bPresent := clusters["backend_tenant-b"]
	assert.False(t, brokenPresent)
	assert.True(t, bPresent)
}

func TestBackendCAValidationContextSecretName_NeverCollidesWithAPISecrets(t *testing.T) {
	assert.NotEqual(t, BackendCAValidationContextSecretName("x"), UpstreamCAValidationContextSecretName("x", ""))
	ref := UpstreamTLSSecretRef{APIHandle: "api", DefinitionName: "def"}
	assert.Equal(t, UpstreamCAValidationContextSecretName("api", "def"), ref.validationContextSecretName())
}

// observedBackendSource records which Backends' clusters each translation reports as built.
type observedBackendSource struct {
	staticBackendSource
	built [][]string
}

func (s *observedBackendSource) BackendClustersBuilt(names []string) {
	s.built = append(s.built, names)
}

// The translator tells an observing source which Backends' clusters it built: one it skipped,
// because it has no upstream or its cluster cannot be built, is not among them.
func TestTranslateConfigs_ReportsTheBackendClustersItBuilt(t *testing.T) {
	translator := createTestTranslator()
	source := &observedBackendSource{staticBackendSource: staticBackendSource{
		{Name: "broken", ClusterName: "backend_broken"},
		{Name: "empty", ClusterName: "backend_empty", Upstream: &models.UpstreamCluster{Name: "empty"}},
		{Name: "tenant-b", ClusterName: "backend_tenant-b", Upstream: &models.UpstreamCluster{
			Name: "tenant-b", Endpoints: []models.Endpoint{{Host: "b", Port: 80}},
		}},
	}}
	translator.SetBackendSource(source)
	clustersByName(t, translator)
	assert.Equal(t, [][]string{{"tenant-b"}}, source.built)

	// A translation that builds none still reports, so nothing stays marked as built.
	source.staticBackendSource = staticBackendSource{{Name: "broken", ClusterName: "backend_broken"}}
	clustersByName(t, translator)
	assert.Equal(t, [][]string{{"tenant-b"}, {}}, source.built)
}
