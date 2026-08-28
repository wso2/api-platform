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
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/tlsauth"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Bounds on the policy xDS gRPC server (go-network-service-hardening.md
// directive 2 / go-control-plane-xds-security.md directive 5) -- unbounded
// defaults let one client (a misbehaving/compromised policy-engine) exhaust
// memory or the stream-slot budget every other connection depends on.
const (
	policyXDSMaxMessageSize       = 16 * 1024 * 1024
	policyXDSMaxConcurrentStreams = 1000
)

// WebhookSecretCacheProvider is the extension point through which an external
// event-gateway-controller binary supplies the xDS cache backing webhook-secret
// (HMAC) resources. Core never implements this interface itself; it is only
// ever satisfied by a webhooksecretxds.SnapshotManager living outside this
// module.
type WebhookSecretCacheProvider interface {
	GetCache() cache.Cache
}

// Server is the policy xDS gRPC server
type Server struct {
	grpcServer               *grpc.Server
	xdsServer                server.Server
	snapshotManager          *SnapshotManager
	apiKeySnapshotMgr        *apikeyxds.APIKeySnapshotManager
	lazyResourceSnapshotMgr  *lazyresourcexds.LazyResourceSnapshotManager
	subscriptionSnapshotMgr  *subscriptionxds.SnapshotManager
	webhookSecretSnapshotMgr WebhookSecretCacheProvider
	port                     int
	mtls                     *serverMTLS
	onFirstConnect           chan struct{}
	logger                   *slog.Logger
}

// serverMTLS holds the resolved mutual-TLS state for the policy xDS server.
type serverMTLS struct {
	tlsConfig         *tls.Config
	allowedIdentities map[string]bool
}

// ServerOption is a functional option for configuring the Server
type ServerOption func(*Server)

// WithMTLS enables mutual TLS on the policy xDS gRPC server (serving the
// policy-engine). tlsConfig must come from config.BuildXDSServerTLSConfig,
// which already sets ClientAuth: tls.RequireAndVerifyClientCert --
// server-only TLS is not offered here because this channel carries
// per-tenant API-key hashes, subscription state, and full policy chains,
// so authenticating only the server side is not enough
// (go-control-plane-xds-security.md directive 2). allowedIdentities
// restricts accepted streams to peers whose certificate identity
// (tlsauth.PeerIdentity) is in the list.
func WithMTLS(tlsConfig *tls.Config, allowedIdentities []string) ServerOption {
	return func(s *Server) {
		s.mtls = &serverMTLS{
			tlsConfig:         tlsConfig,
			allowedIdentities: tlsauth.AllowedSet(allowedIdentities),
		}
	}
}

// WithOnFirstConnect sets a channel that will be closed when the first xDS client connects
func WithOnFirstConnect(ch chan struct{}) ServerOption {
	return func(s *Server) {
		s.onFirstConnect = ch
	}
}

// NewServer creates a new policy xDS server
func NewServer(snapshotManager *SnapshotManager, apiKeySnapshotMgr *apikeyxds.APIKeySnapshotManager, lazyResourceSnapshotMgr *lazyresourcexds.LazyResourceSnapshotManager, subscriptionSnapshotMgr *subscriptionxds.SnapshotManager, webhookSecretSnapshotMgr WebhookSecretCacheProvider, port int, logger *slog.Logger, opts ...ServerOption) *Server {
	s := &Server{
		snapshotManager:          snapshotManager,
		apiKeySnapshotMgr:        apiKeySnapshotMgr,
		lazyResourceSnapshotMgr:  lazyResourceSnapshotMgr,
		subscriptionSnapshotMgr:  subscriptionSnapshotMgr,
		webhookSecretSnapshotMgr: webhookSecretSnapshotMgr,
		port:                     port,
		logger:                   logger,
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
		grpc.MaxRecvMsgSize(policyXDSMaxMessageSize),
		grpc.MaxSendMsgSize(policyXDSMaxMessageSize),
		grpc.MaxConcurrentStreams(policyXDSMaxConcurrentStreams),
	}

	// Add mTLS credentials if enabled
	var allowedIdentities map[string]bool
	if s.mtls != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(s.mtls.tlsConfig)))
		allowedIdentities = s.mtls.allowedIdentities
		logger.Info("mTLS enabled for Policy xDS server")
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// Create combined cache that handles policy chains, route configs, API key state, lazy resources, subscription state, event channel configs, and webhook secrets
	policyCache := snapshotManager.GetPolicyCache()
	routeConfigCache := snapshotManager.GetRouteCache()
	eventChannelCache := snapshotManager.GetEventChannelCache()
	apiKeyCache := apiKeySnapshotMgr.GetCache()
	lazyResourceCache := lazyResourceSnapshotMgr.GetCache()
	subscriptionCache := subscriptionSnapshotMgr.GetCache()
	var webhookSecretCache cache.Cache
	if s.webhookSecretSnapshotMgr != nil {
		webhookSecretCache = s.webhookSecretSnapshotMgr.GetCache()
	}
	combinedCache := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, subscriptionCache, routeConfigCache, eventChannelCache, webhookSecretCache, logger)

	callbacks := &serverCallbacks{
		logger:            logger,
		activeStreams:     make(map[int64]bool),
		onFirstConnect:    s.onFirstConnect,
		pendingNonces:     make(map[int64]string),
		allowedIdentities: allowedIdentities,
	}
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
	if s.mtls != nil {
		protocol = "mTLS"
	}
	s.logger.Info("Starting Policy xDS server",
		slog.Int("port", s.port),
		slog.String("protocol", protocol))

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
	logger            *slog.Logger
	activeStreams     map[int64]bool
	activeStreamsMu   sync.Mutex
	onFirstConnect    chan struct{}
	firstConnectOnce  sync.Once
	pendingNonces     map[int64]string // stream_id -> last sent nonce
	allowedIdentities map[string]bool  // nil when mTLS is not configured -- no identity check performed
}

// OnStreamOpen is called when a new stream is opened
func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	if cb.allowedIdentities != nil {
		if err := tlsauth.VerifyStreamPeer(ctx, cb.allowedIdentities); err != nil {
			cb.logger.Warn("Policy xDS stream rejected: peer identity not authorized",
				slog.Int64("stream_id", streamID), slog.Any("error", err))
			return err
		}
	}
	cb.logger.Info("Policy xDS stream opened",
		slog.Int64("stream_id", streamID),
		slog.String("type_url", typeURL))
	return nil
}

// OnStreamClosed is called when a stream is closed
func (cb *serverCallbacks) OnStreamClosed(streamID int64, node *core.Node) {
	cb.logger.Info("Policy xDS stream closed",
		slog.Int64("stream_id", streamID),
		slog.String("node_id", node.GetId()))

	cb.activeStreamsMu.Lock()
	defer cb.activeStreamsMu.Unlock()
	delete(cb.activeStreams, streamID)
}

// OnStreamRequest is called when a discovery request is received
func (cb *serverCallbacks) OnStreamRequest(streamID int64, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Info("Policy xDS stream request",
		slog.Int64("stream_id", streamID),
		slog.String("type_url", req.GetTypeUrl()),
		slog.String("version", req.GetVersionInfo()),
		slog.Any("resource_names", req.GetResourceNames()))

	cb.activeStreamsMu.Lock()
	defer cb.activeStreamsMu.Unlock()

	if _, exists := cb.activeStreams[streamID]; !exists {
		cb.activeStreams[streamID] = true
	}

	// Detect ACKs by comparing the nonce in the request with the last sent nonce for this stream
	if req.GetResponseNonce() != "" && req.GetErrorDetail() == nil {
		if pendingNonce, exists := cb.pendingNonces[streamID]; exists && pendingNonce == req.GetResponseNonce() {
			if cb.onFirstConnect != nil {
				cb.firstConnectOnce.Do(func() { close(cb.onFirstConnect) })
			}
		}
	}

	return nil
}

// OnStreamResponse is called when a discovery response is sent
func (cb *serverCallbacks) OnStreamResponse(ctx context.Context, streamID int64, req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	// Track the nonce of the response so we can detect ACKs in OnStreamRequest
	cb.activeStreamsMu.Lock()
	if cb.pendingNonces == nil {
		cb.pendingNonces = make(map[int64]string)
	}
	cb.pendingNonces[streamID] = resp.GetNonce()
	cb.activeStreamsMu.Unlock()

	cb.logger.Info("Policy xDS stream response",
		slog.Int64("stream_id", streamID),
		slog.String("type_url", resp.GetTypeUrl()),
		slog.String("version", resp.GetVersionInfo()),
		slog.Int("resource_count", len(resp.GetResources())))
}

// OnFetchRequest is called when a fetch request is received
func (cb *serverCallbacks) OnFetchRequest(ctx context.Context, req *discoverygrpc.DiscoveryRequest) error {
	cb.logger.Debug("Policy xDS fetch request",
		slog.String("type_url", req.GetTypeUrl()),
		slog.Any("resource_names", req.GetResourceNames()))
	return nil
}

// OnFetchResponse is called when a fetch response is sent
func (cb *serverCallbacks) OnFetchResponse(req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	cb.logger.Debug("Policy xDS fetch response",
		slog.String("type_url", resp.GetTypeUrl()),
		slog.String("version", resp.GetVersionInfo()),
		slog.Int("resource_count", len(resp.GetResources())))
}

// OnDeltaStreamOpen is called when a delta stream is opened
func (cb *serverCallbacks) OnDeltaStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	cb.logger.Debug("Policy xDS delta stream opened",
		slog.Int64("stream_id", streamID),
		slog.String("type_url", typeURL))
	return nil
}

// OnDeltaStreamClosed is called when a delta stream is closed
func (cb *serverCallbacks) OnDeltaStreamClosed(streamID int64, node *core.Node) {
	cb.logger.Debug("Policy xDS delta stream closed",
		slog.Int64("stream_id", streamID),
		slog.String("node_id", node.GetId()))
}

// OnStreamDeltaRequest is called when a delta discovery request is received
func (cb *serverCallbacks) OnStreamDeltaRequest(streamID int64, req *discoverygrpc.DeltaDiscoveryRequest) error {
	cb.logger.Debug("Policy xDS delta stream request",
		slog.Int64("stream_id", streamID),
		slog.String("type_url", req.GetTypeUrl()))
	return nil
}

// OnStreamDeltaResponse is called when a delta discovery response is sent
func (cb *serverCallbacks) OnStreamDeltaResponse(streamID int64, req *discoverygrpc.DeltaDiscoveryRequest, resp *discoverygrpc.DeltaDiscoveryResponse) {
	cb.logger.Debug("Policy xDS delta stream response",
		slog.Int64("stream_id", streamID),
		slog.String("type_url", resp.GetTypeUrl()),
		slog.Int("resource_count", len(resp.GetResources())))
}
