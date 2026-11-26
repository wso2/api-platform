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

package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestIsValidEndpointName(t *testing.T) {
	for name, want := range map[string]bool{
		"tenant-a":              true,
		"abc":                   true,
		"a1-b2-c3":              true,
		strings.Repeat("a", 40): true,
		"":                      false,
		"ab":                    false,
		strings.Repeat("a", 41): false,
		"Tenant-A":              false,
		"tenant_a":              false,
		"tenant.a":              false,
		"-tenant":               false,
		"tenant-":               false,
		"tenant--a":             false,
		"tenant a":              false,
		"tenant-a\n":            false,
	} {
		assert.Equalf(t, want, IsValidEndpointName(name), "IsValidEndpointName(%q)", name)
	}
}

func TestEndpointClusterNameUsesTheReservedPrefix(t *testing.T) {
	assert.Equal(t, "endpoint_tenant-a", EndpointClusterName("tenant-a"))
	// API clusters are named upstream_... (cluster_... on the legacy path), so
	// the prefix alone keeps an Endpoint cluster from colliding with one.
	for _, apiCluster := range []string{"upstream_main_host_80", "upstream_RestApi_id_def", "cluster_http_host"} {
		assert.False(t, strings.HasPrefix(apiCluster, EndpointClusterPrefix))
	}
}

func TestStoredEndpointAccessors(t *testing.T) {
	basePath := "/v1"
	weight := 80
	labels := map[string]string{"team": "orders"}
	tls := map[string]interface{}{"identity": "partner"}
	connect := "2s"
	endpoint := &StoredEndpoint{Configuration: api.EndpointConfiguration{
		Metadata: api.Metadata{Name: "tenant-a", Labels: &labels},
		Spec: api.EndpointConfigurationData{
			BasePath: &basePath,
			Timeout:  &api.UpstreamTimeout{Connect: &connect},
			Tls:      &tls,
		},
	}}
	endpoint.Configuration.Spec.Upstreams = append(endpoint.Configuration.Spec.Upstreams, struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{Url: "https://orders-tenant-a:9443", Weight: &weight})

	assert.Equal(t, "tenant-a", endpoint.GetHandle())
	assert.Equal(t, "endpoint_tenant-a", endpoint.ClusterName())
	assert.Equal(t, "/v1", endpoint.BasePath())

	got := endpoint.Labels()
	assert.Equal(t, labels, got)
	got["team"] = "changed"
	assert.Equal(t, "orders", labels["team"], "Labels must return a copy")

	def := endpoint.UpstreamDefinition()
	assert.Equal(t, "tenant-a", def.Name)
	assert.Equal(t, &basePath, def.BasePath)
	assert.Equal(t, &tls, def.Tls)
	require.NotNil(t, def.Timeout)
	assert.Equal(t, &connect, def.Timeout.Connect)
	require.Len(t, def.Upstreams, 1)
	assert.Equal(t, "https://orders-tenant-a:9443", def.Upstreams[0].Url)
	assert.Equal(t, &weight, def.Upstreams[0].Weight)
}

func TestStoredEndpointDefaults(t *testing.T) {
	endpoint := &StoredEndpoint{Configuration: api.EndpointConfiguration{Metadata: api.Metadata{Name: "tenant-a"}}}
	assert.Equal(t, "/", endpoint.BasePath(), "an Endpoint with no basePath forwards from the root")
	assert.NotNil(t, endpoint.Labels())
	assert.Empty(t, endpoint.Labels())

	empty := ""
	endpoint.Configuration.Spec.BasePath = &empty
	assert.Equal(t, "/", endpoint.BasePath())
}
