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
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// EndpointClusterSource lists the gateway-wide Endpoints in the in-memory store
// as cluster models for the Envoy translator. It converts each Endpoint with
// UpstreamClusterFromDefinition, the same converter an API's upstreamDefinitions
// go through, so an Endpoint gets the same weighted, TLS-aware cluster a
// definition with those fields would.
type EndpointClusterSource struct {
	store  *storage.ConfigStore
	logger *slog.Logger
}

// NewEndpointClusterSource creates a source reading Endpoints from store.
func NewEndpointClusterSource(store *storage.ConfigStore, logger *slog.Logger) *EndpointClusterSource {
	if logger == nil {
		logger = slog.Default()
	}
	return &EndpointClusterSource{store: store, logger: logger}
}

// EndpointClusters implements xds.EndpointClusterSource. An Endpoint that
// cannot be converted is logged and left out; it was valid when stored, so
// this only happens for a row written by something other than this gateway.
func (s *EndpointClusterSource) EndpointClusters() []xds.EndpointCluster {
	endpoints := s.store.GetAllEndpoints()
	clusters := make([]xds.EndpointCluster, 0, len(endpoints))
	for _, endpoint := range endpoints {
		cluster, err := EndpointUpstreamCluster(endpoint)
		if err != nil {
			s.logger.Error("Skipping endpoint that cannot be turned into a cluster",
				slog.String("endpoint", endpoint.GetHandle()), slog.Any("error", err))
			continue
		}
		clusters = append(clusters, xds.EndpointCluster{Name: endpoint.GetHandle(), Cluster: cluster})
	}
	return clusters
}

// EndpointUpstreamCluster converts one Endpoint into its cluster model.
func EndpointUpstreamCluster(endpoint *models.StoredEndpoint) (*models.UpstreamCluster, error) {
	return UpstreamClusterFromDefinition(endpoint.UpstreamDefinition())
}
