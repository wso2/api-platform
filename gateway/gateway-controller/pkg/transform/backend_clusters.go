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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// BackendLister lists the gateway-wide Backends, ordered by name.
type BackendLister interface {
	ListBackends() []*models.StoredBackend
}

// BackendClusterSource feeds the xDS translator one cluster per gateway-wide Backend, built by
// the same conversion as an upstreamDefinitions entry.
type BackendClusterSource struct {
	backends BackendLister
	logger   *slog.Logger
}

var (
	_ xds.BackendClusterSource   = (*BackendClusterSource)(nil)
	_ xds.BackendClusterObserver = (*BackendClusterSource)(nil)
)

// BuiltClusterRecorder is implemented by a BackendLister that needs to know which of the
// Backends it listed got a cluster in the translation.
type BuiltClusterRecorder interface {
	ClustersBuilt(names []string)
}

// BackendClustersBuilt passes the names of the Backends whose clusters the translation built on
// to the lister, when it records them. A Backend skipped here, because it could not be
// converted, is never passed to the translator, so it is not among them either.
func (s *BackendClusterSource) BackendClustersBuilt(names []string) {
	if recorder, ok := s.backends.(BuiltClusterRecorder); ok {
		recorder.ClustersBuilt(names)
	}
}

// NewBackendClusterSource creates a BackendClusterSource over the given Backends.
func NewBackendClusterSource(backends BackendLister, logger *slog.Logger) *BackendClusterSource {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackendClusterSource{backends: backends, logger: logger}
}

// BackendClusters converts every Backend; one that cannot be converted is logged and skipped.
func (s *BackendClusterSource) BackendClusters() []xds.BackendCluster {
	stored := s.backends.ListBackends()
	clusters := make([]xds.BackendCluster, 0, len(stored))
	for _, backend := range stored {
		uc, err := UpstreamClusterFromDefinition(backend.UpstreamDefinition())
		if err != nil {
			s.logger.Error("Skipping global backend that cannot be converted to a cluster",
				slog.String("backend", backend.GetHandle()), slog.Any("error", err))
			continue
		}
		clusters = append(clusters, xds.BackendCluster{
			Name:        backend.GetHandle(),
			ClusterName: backend.ClusterName(),
			Upstream:    uc,
		})
	}
	return clusters
}
