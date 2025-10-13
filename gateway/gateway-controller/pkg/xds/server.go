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
	"context"
	"fmt"
	"net"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// Server is the xDS gRPC server
type Server struct {
	grpcServer      *grpc.Server
	xdsServer       server.Server
	snapshotManager *SnapshotManager
	port            int
	logger          *zap.Logger
}

// NewServer creates a new xDS server
func NewServer(snapshotManager *SnapshotManager, port int, logger *zap.Logger) *Server {
	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	// Create xDS server with the snapshot cache
	cache := snapshotManager.GetCache()
	callbacks := &serverCallbacks{logger: logger}
	xdsServer := server.NewServer(context.Background(), cache, callbacks)

	// Register xDS services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)

	return &Server{
		grpcServer:      grpcServer,
		xdsServer:       xdsServer,
		snapshotManager: snapshotManager,
		port:            port,
		logger:          logger,
	}
}

// Start starts the xDS gRPC server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	s.logger.Info("Starting xDS server", zap.Int("port", s.port))

	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully stops the xDS server
func (s *Server) Stop() {
	s.logger.Info("Stopping xDS server")
	s.grpcServer.GracefulStop()
}

// serverCallbacks implements server.Callbacks
type serverCallbacks struct {
	logger *zap.Logger
}

func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
	cb.logger.Info("xDS stream opened", zap.Int64("stream_id", id), zap.String("type", typ))
	return nil
}

func (cb *serverCallbacks) OnStreamClosed(id int64, node *core.Node) {
	cb.logger.Info("xDS stream closed", zap.Int64("stream_id", id))
}

func (cb *serverCallbacks) OnStreamRequest(id int64, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Debug("xDS stream request",
		zap.Int64("stream_id", id),
		zap.String("type_url", req.TypeUrl),
		zap.String("version", req.VersionInfo),
	)
	return nil
}

func (cb *serverCallbacks) OnStreamResponse(ctx context.Context, id int64, req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	cb.logger.Debug("xDS stream response",
		zap.Int64("stream_id", id),
		zap.String("type_url", resp.TypeUrl),
		zap.String("version", resp.VersionInfo),
		zap.Int("num_resources", len(resp.Resources)),
	)
}

func (cb *serverCallbacks) OnFetchRequest(ctx context.Context, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Debug("xDS fetch request", zap.String("type_url", req.TypeUrl))
	return nil
}

func (cb *serverCallbacks) OnFetchResponse(req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	cb.logger.Debug("xDS fetch response",
		zap.String("type_url", resp.TypeUrl),
		zap.String("version", resp.VersionInfo),
	)
}

func (cb *serverCallbacks) OnDeltaStreamOpen(ctx context.Context, id int64, typ string) error {
	return nil
}

func (cb *serverCallbacks) OnDeltaStreamClosed(id int64, node *core.Node) {
}

func (cb *serverCallbacks) OnStreamDeltaRequest(id int64, req *discoverygrpc.DeltaDiscoveryRequest) error {
	return nil
}

func (cb *serverCallbacks) OnStreamDeltaResponse(id int64, req *discoverygrpc.DeltaDiscoveryRequest, resp *discoverygrpc.DeltaDiscoveryResponse) {
}
