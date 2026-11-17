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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func dynamicEndpointV11Definitions() map[string]models.PolicyDefinition {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"targetUpstream":    map[string]interface{}{"type": "string", config.BackendRoutingAnnotation: config.BackendRoutingName},
			"backendExpression": map[string]interface{}{"type": "string", config.BackendRoutingAnnotation: config.BackendRoutingExpression},
		},
	}
	unannotated := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"targetUpstream": map[string]interface{}{"type": "string"}},
	}
	return map[string]models.PolicyDefinition{
		"dynamic-endpoint|v1.1.0": {Name: "dynamic-endpoint", Version: "v1.1.0", Parameters: &params},
		"dynamic-endpoint|v0.9.0": {Name: "dynamic-endpoint", Version: "v0.9.0", Parameters: &unannotated},
	}
}

func dynamicEndpointPolicy(params map[string]interface{}) api.Policy {
	return api.Policy{Name: "dynamic-endpoint", Version: "v1", Params: &params}
}

const helloRouteKey = "GET|/test/hello|main.local"

func TestRestAPITransformer_BackendRoutingMakesTheRouteDivertible(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, dynamicEndpointV11Definitions())

	t.Run("expression mode at API level", func(t *testing.T) {
		rdc, err := transformer.Transform(makeRestAPIStoredConfig([]api.Policy{dynamicEndpointPolicy(map[string]interface{}{
			"backendExpression": "x-backend",
			"backendSelector":   map[string]interface{}{"matchLabels": map[string]interface{}{"team": "orders"}},
		})}, nil))
		require.NoError(t, err)
		route := rdc.Routes[helloRouteKey]
		require.NotNil(t, route)
		assert.True(t, route.Upstream.UseClusterHeader)
		assert.Equal(t, route.Upstream.ClusterKey, route.Upstream.DefaultCluster,
			"a request no policy diverts goes to the API's own upstream")
	})

	t.Run("fixed target at operation level", func(t *testing.T) {
		rdc, err := transformer.Transform(makeRestAPIStoredConfig(nil, []api.Policy{dynamicEndpointPolicy(map[string]interface{}{
			"targetUpstream": "tenant-a",
		})}))
		require.NoError(t, err)
		assert.True(t, rdc.Routes[helloRouteKey].Upstream.UseClusterHeader)
	})
}

func TestRestAPITransformer_RoutesWithoutBackendRoutingKeepTheirShape(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, dynamicEndpointV11Definitions())

	t.Run("no policy", func(t *testing.T) {
		rdc, err := transformer.Transform(makeRestAPIStoredConfig(nil, nil))
		require.NoError(t, err)
		route := rdc.Routes[helloRouteKey]
		assert.False(t, route.Upstream.UseClusterHeader)
		assert.Empty(t, route.Upstream.DefaultCluster)
	})

	t.Run("targetUpstream of a policy version without the annotation", func(t *testing.T) {
		rdc, err := transformer.Transform(makeRestAPIStoredConfig([]api.Policy{{
			Name: "dynamic-endpoint", Version: "v0", Params: &map[string]interface{}{"targetUpstream": "alt"},
		}}, nil))
		require.NoError(t, err)
		assert.False(t, rdc.Routes[helloRouteKey].Upstream.UseClusterHeader,
			"only an annotated parameter changes the route's shape")
	})
}

type backendList []*models.StoredBackend

func (l backendList) ListBackends() []*models.StoredBackend { return l }

func TestBackendClusterSource_ConvertsEveryBackend(t *testing.T) {
	weight := 70
	basePath := "/v1"
	good := &models.StoredBackend{UUID: "u-1", Configuration: api.BackendRequest{
		Metadata: api.Metadata{Name: "tenant-a"},
		Spec: api.BackendConfiguration{BasePath: &basePath, Endpoints: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://a1:9080", Weight: &weight}, {Url: "http://a2:9081"}}},
	}}
	mixed := &models.StoredBackend{UUID: "u-2", Configuration: api.BackendRequest{
		Metadata: api.Metadata{Name: "mixed"},
		Spec: api.BackendConfiguration{Endpoints: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "https://a:443"}, {Url: "http://b:80"}}},
	}}

	clusters := NewBackendClusterSource(backendList{good, mixed}, nil).BackendClusters()
	require.Len(t, clusters, 1, "a Backend that cannot be converted is skipped")
	c := clusters[0]
	assert.Equal(t, "tenant-a", c.Name)
	assert.Equal(t, "backend_tenant-a", c.ClusterName)
	assert.Equal(t, "/v1", c.Upstream.BasePath)
	require.Len(t, c.Upstream.Endpoints, 2)
	assert.Equal(t, "a1", c.Upstream.Endpoints[0].Host)
	assert.Equal(t, 9081, c.Upstream.Endpoints[1].Port)
	assert.Equal(t, 70, *c.Upstream.Endpoints[0].Weight)
}

// recordingBackendList records the built-cluster reports it receives.
type recordingBackendList struct {
	backendList
	built []string
}

func (l *recordingBackendList) ClustersBuilt(names []string) { l.built = names }

// The source passes the translator's report of the clusters it built on to a lister that records
// it, and ignores it for one that does not.
func TestBackendClusterSource_PassesTheBuiltClustersToTheLister(t *testing.T) {
	recorder := &recordingBackendList{}
	NewBackendClusterSource(recorder, nil).BackendClustersBuilt([]string{"tenant-a"})
	assert.Equal(t, []string{"tenant-a"}, recorder.built)

	assert.NotPanics(t, func() {
		NewBackendClusterSource(backendList{}, nil).BackendClustersBuilt([]string{"tenant-a"})
	})
}
