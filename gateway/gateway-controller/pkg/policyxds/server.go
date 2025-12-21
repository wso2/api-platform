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

package policyxds

import (
	"context"
	"fmt"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"net"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Server is the policy xDS gRPC server
type Server struct {
	grpcServer        *grpc.Server
	xdsServer         server.Server
	snapshotManager   *SnapshotManager
	apiKeySnapshotMgr *apikeyxds.APIKeySnapshotManager
	port              int
	tlsConfig         *TLSConfig
	logger            *zap.Logger
}

// TLSConfig holds TLS configuration for the server
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

// ServerOption is a functional option for configuring the Server
type ServerOption func(*Server)

// WithTLS enables TLS with the provided certificate and key files
func WithTLS(certFile, keyFile string) ServerOption {
	return func(s *Server) {
		s.tlsConfig = &TLSConfig{
			Enabled:  true,
			CertFile: certFile,
			KeyFile:  keyFile,
		}
	}
}

// NewServer creates a new policy xDS server
func NewServer(snapshotManager *SnapshotManager, apiKeySnapshotMgr *apikeyxds.APIKeySnapshotManager, port int, logger *zap.Logger, opts ...ServerOption) *Server {
	s := &Server{
		snapshotManager:   snapshotManager,
		apiKeySnapshotMgr: apiKeySnapshotMgr,
		port:              port,
		logger:            logger,
		tlsConfig:         &TLSConfig{Enabled: false},
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Build gRPC server options
	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Add TLS credentials if enabled
	if s.tlsConfig.Enabled {
		creds, err := credentials.NewServerTLSFromFile(s.tlsConfig.CertFile, s.tlsConfig.KeyFile)
		if err != nil {
			logger.Fatal("Failed to load TLS credentials", zap.Error(err))
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
		logger.Info("TLS enabled for Policy xDS server",
			zap.String("cert_file", s.tlsConfig.CertFile),
			zap.String("key_file", s.tlsConfig.KeyFile))
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// Create combined cache that handles both policy chains and API key state
	policyCache := snapshotManager.GetCache()
	apiKeyCache := apiKeySnapshotMgr.GetCache()
	combinedCache := NewCombinedCache(policyCache, apiKeyCache, logger)

	callbacks := &serverCallbacks{logger: logger}
	xdsServer := server.NewServer(context.Background(), combinedCache, callbacks)

	// Register ADS (Aggregated Discovery Service) for policy distribution
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)

	s.grpcServer = grpcServer
	s.xdsServer = xdsServer

	return s
}

// Start starts the policy xDS gRPC server in a blocking manner
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	protocol := "insecure"
	if s.tlsConfig.Enabled {
		protocol = "TLS"
	}
	s.logger.Info("Starting Policy xDS server",
		zap.Int("port", s.port),
		zap.String("protocol", protocol))

	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully stops the policy xDS server
func (s *Server) Stop() {
	s.logger.Info("Stopping Policy xDS server")
	s.grpcServer.GracefulStop()
}

// serverCallbacks implements xDS server callbacks for logging and debugging
type serverCallbacks struct {
	logger *zap.Logger
}

// OnStreamOpen is called when a new stream is opened
func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	cb.logger.Info("Policy xDS stream opened",
		zap.Int64("stream_id", streamID),
		zap.String("type_url", typeURL))
	return nil
}

// OnStreamClosed is called when a stream is closed
func (cb *serverCallbacks) OnStreamClosed(streamID int64, node *core.Node) {
	cb.logger.Info("Policy xDS stream closed",
		zap.Int64("stream_id", streamID),
		zap.String("node_id", node.GetId()))
}

// OnStreamRequest is called when a discovery request is received
func (cb *serverCallbacks) OnStreamRequest(streamID int64, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Info("Policy xDS stream request",
		zap.Int64("stream_id", streamID),
		zap.String("type_url", req.GetTypeUrl()),
		zap.String("version", req.GetVersionInfo()),
		zap.Strings("resource_names", req.GetResourceNames()))
	return nil
}

// OnStreamResponse is called when a discovery response is sent
func (cb *serverCallbacks) OnStreamResponse(ctx context.Context, streamID int64, req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	cb.logger.Info("Policy xDS stream response",
		zap.Int64("stream_id", streamID),
		zap.String("type_url", resp.GetTypeUrl()),
		zap.String("version", resp.GetVersionInfo()),
		zap.Int("resource_count", len(resp.GetResources())))
}

// OnFetchRequest is called when a fetch request is received
func (cb *serverCallbacks) OnFetchRequest(ctx context.Context, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Debug("Policy xDS fetch request",
		zap.String("type_url", req.GetTypeUrl()),
		zap.Strings("resource_names", req.GetResourceNames()))
	return nil
}

// OnFetchResponse is called when a fetch response is sent
func (cb *serverCallbacks) OnFetchResponse(req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	cb.logger.Debug("Policy xDS fetch response",
		zap.String("type_url", resp.GetTypeUrl()),
		zap.String("version", resp.GetVersionInfo()),
		zap.Int("resource_count", len(resp.GetResources())))
}

// OnDeltaStreamOpen is called when a delta stream is opened
func (cb *serverCallbacks) OnDeltaStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	cb.logger.Debug("Policy xDS delta stream opened",
		zap.Int64("stream_id", streamID),
		zap.String("type_url", typeURL))
	return nil
}

// OnDeltaStreamClosed is called when a delta stream is closed
func (cb *serverCallbacks) OnDeltaStreamClosed(streamID int64, node *core.Node) {
	cb.logger.Debug("Policy xDS delta stream closed",
		zap.Int64("stream_id", streamID),
		zap.String("node_id", node.GetId()))
}

// OnStreamDeltaRequest is called when a delta discovery request is received
func (cb *serverCallbacks) OnStreamDeltaRequest(streamID int64, req *discoverygrpc.DeltaDiscoveryRequest) error {
	cb.logger.Debug("Policy xDS delta stream request",
		zap.Int64("stream_id", streamID),
		zap.String("type_url", req.GetTypeUrl()))
	return nil
}

// OnStreamDeltaResponse is called when a delta discovery response is sent
func (cb *serverCallbacks) OnStreamDeltaResponse(streamID int64, req *discoverygrpc.DeltaDiscoveryRequest, resp *discoverygrpc.DeltaDiscoveryResponse) {
	cb.logger.Debug("Policy xDS delta stream response",
		zap.Int64("stream_id", streamID),
		zap.String("type_url", resp.GetTypeUrl()),
		zap.Int("resource_count", len(resp.GetResources())))
}
