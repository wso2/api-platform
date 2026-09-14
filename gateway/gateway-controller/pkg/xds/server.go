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
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"log/slog"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/tlsauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Bounds on the main xDS gRPC server (go-network-service-hardening.md
// directive 2 / go-control-plane-xds-security.md directive 5) -- unbounded
// defaults let one client (a misbehaving/compromised Envoy) exhaust memory
// or the stream-slot budget every other connection depends on. xDS
// snapshots can carry many routes/clusters, so the message ceiling is set
// well above gRPC's 4MB default.
const (
	xdsMaxMessageSize       = 16 * 1024 * 1024
	xdsMaxConcurrentStreams = 1000
)

// Server is the xDS gRPC server
type Server struct {
	grpcServer      *grpc.Server
	xdsServer       server.Server
	snapshotManager *SnapshotManager
	port            int
	mtls            *serverMTLS
	logger          *slog.Logger
}

// serverMTLS holds the resolved mutual-TLS state for the main xDS server.
type serverMTLS struct {
	tlsConfig         *tls.Config
	allowedIdentities map[string]bool
}

// ServerOption is a functional option for configuring the xDS Server.
type ServerOption func(*serverOptions)

type serverOptions struct {
	mtls *serverMTLS
}

// WithMTLS enables mutual TLS on the main xDS gRPC server (serving Envoy).
// tlsConfig must come from config.BuildXDSServerTLSConfig, which already
// sets ClientAuth: tls.RequireAndVerifyClientCert -- server-only TLS is not
// offered here because this server distributes SDS secrets and full
// route/cluster config, so authenticating only the server side is not
// enough (go-control-plane-xds-security.md directive 2). allowedIdentities
// restricts accepted streams to peers whose certificate identity
// (tlsauth.PeerIdentity) is in the list.
func WithMTLS(tlsConfig *tls.Config, allowedIdentities []string) ServerOption {
	return func(o *serverOptions) {
		o.mtls = &serverMTLS{
			tlsConfig:         tlsConfig,
			allowedIdentities: tlsauth.AllowedSet(allowedIdentities),
		}
	}
}

// NewServer creates a new xDS server
func NewServer(snapshotManager *SnapshotManager, sdsSecretManager *SDSSecretManager, port int, logger *slog.Logger, onFirstConnect chan struct{}, opts ...ServerOption) *Server {
	var o serverOptions
	for _, opt := range opts {
		opt(&o)
	}

	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.MaxRecvMsgSize(xdsMaxMessageSize),
		grpc.MaxSendMsgSize(xdsMaxMessageSize),
		grpc.MaxConcurrentStreams(xdsMaxConcurrentStreams),
	}

	var allowedIdentities map[string]bool
	if o.mtls != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(o.mtls.tlsConfig)))
		allowedIdentities = o.mtls.allowedIdentities
		logger.Info("mTLS enabled for main xDS server")
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// Create xDS server with the snapshot cache (shared with SDS)
	cache := snapshotManager.GetCache()
	callbacks := NewServerCallbacks(logger, onFirstConnect, allowedIdentities)
	xdsServer := server.NewServer(context.Background(), cache, callbacks)

	// Register xDS services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)

	// Register SDS service (shares the same cache and server as main xDS)
	if sdsSecretManager != nil {
		secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, xdsServer)
		logger.Info("SDS service registered on main xDS server")
	}

	return &Server{
		grpcServer:      grpcServer,
		xdsServer:       xdsServer,
		snapshotManager: snapshotManager,
		port:            port,
		mtls:            o.mtls,
		logger:          logger,
	}
}

// Start starts the xDS gRPC server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	protocol := "insecure"
	if s.mtls != nil {
		protocol = "mTLS"
	}
	s.logger.Info("Starting xDS server", slog.Int("port", s.port), slog.String("protocol", protocol))

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
	logger            *slog.Logger
	activeStreams     map[int64]string // stream_id -> node_id
	activeStreamsMu   sync.Mutex
	onFirstConnect    chan struct{}
	firstConnectOnce  sync.Once
	pendingNonces     map[int64]string // stream_id -> last sent nonce
	allowedIdentities map[string]bool  // nil when mTLS is not configured -- no identity check performed
}

func NewServerCallbacks(logger *slog.Logger, onFirstConnect chan struct{}, allowedIdentities map[string]bool) *serverCallbacks {
	return &serverCallbacks{
		logger:            logger,
		activeStreams:     make(map[int64]string),
		onFirstConnect:    onFirstConnect,
		pendingNonces:     make(map[int64]string),
		allowedIdentities: allowedIdentities,
	}
}

func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
	if cb.allowedIdentities != nil {
		if err := tlsauth.VerifyStreamPeer(ctx, cb.allowedIdentities); err != nil {
			cb.logger.Warn("xDS stream rejected: peer identity not authorized",
				slog.Int64("stream_id", id), slog.Any("error", err))
			return err
		}
	}
	cb.logger.Info("xDS stream opened", slog.Int64("stream_id", id), slog.String("type", typ))
	return nil
}

func (cb *serverCallbacks) OnStreamClosed(id int64, node *core.Node) {
	cb.logger.Info("xDS stream closed", slog.Int64("stream_id", id))

	cb.activeStreamsMu.Lock()
	defer cb.activeStreamsMu.Unlock()

	// Remove from active streams and decrement metric using the stored node ID
	// to ensure label consistency with the increment in OnStreamRequest
	if storedNodeID, exists := cb.activeStreams[id]; exists {
		delete(cb.activeStreams, id)
		// Use stored node ID; fallback to "unknown" if empty
		nodeID := storedNodeID
		if nodeID == "" {
			nodeID = "unknown"
		}
		metrics.XDSClientsConnected.WithLabelValues("main", nodeID).Dec()
	}
}

func (cb *serverCallbacks) OnStreamRequest(id int64, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Debug("xDS stream request",
		slog.Int64("stream_id", id),
		slog.String("type_url", req.TypeUrl),
		slog.String("version", req.VersionInfo),
	)

	// Track the node ID when we first see a request
	nodeID := "unknown"
	if req.Node != nil && req.Node.Id != "" {
		nodeID = req.Node.Id
	}

	cb.activeStreamsMu.Lock()
	defer cb.activeStreamsMu.Unlock()

	// Only increment if this is a new stream
	if _, exists := cb.activeStreams[id]; !exists {
		cb.activeStreams[id] = nodeID
		metrics.XDSClientsConnected.WithLabelValues("main", nodeID).Inc()
	}

	// Detect ACKs by comparing the nonce in the request with the last sent nonce for this stream
	if req.ResponseNonce != "" && req.ErrorDetail == nil {
		if pendingNonce, exists := cb.pendingNonces[id]; exists && pendingNonce == req.ResponseNonce {
			if cb.onFirstConnect != nil {
				cb.firstConnectOnce.Do(func() { close(cb.onFirstConnect) })
			}
		}
	}

	metrics.XDSStreamRequestsTotal.WithLabelValues("main", req.TypeUrl, "request").Inc()
	return nil
}

func (cb *serverCallbacks) OnStreamResponse(ctx context.Context, id int64, req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	// Track the nonce of the response so we can detect ACKs in OnStreamRequest
	cb.activeStreamsMu.Lock()
	cb.pendingNonces[id] = resp.Nonce
	cb.activeStreamsMu.Unlock()

	// Determine if this is an ACK or NACK
	status := "ack"
	if req != nil && resp != nil {
		// NACK if error detail is present
		if req.ErrorDetail != nil {
			status = "nack"
		} else if req.ResponseNonce == resp.Nonce {
			// ACK if no error and nonce matches
			status = "ack"
		}
	}

	cb.logger.Debug("xDS stream response",
		slog.Int64("stream_id", id),
		slog.String("type_url", resp.TypeUrl),
		slog.String("version", resp.VersionInfo),
		slog.Int("num_resources", len(resp.Resources)),
		slog.String("status", status),
	)

	metrics.XDSStreamRequestsTotal.WithLabelValues("main", resp.TypeUrl, "response").Inc()
	metrics.XDSSnapshotAckTotal.WithLabelValues("main", "client", status).Inc()
}

func (cb *serverCallbacks) OnFetchRequest(ctx context.Context, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Debug("xDS fetch request", slog.String("type_url", req.TypeUrl))
	return nil
}

func (cb *serverCallbacks) OnFetchResponse(req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	cb.logger.Debug("xDS fetch response",
		slog.String("type_url", resp.TypeUrl),
		slog.String("version", resp.VersionInfo),
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
