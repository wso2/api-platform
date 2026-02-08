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

package xdsclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// Client is the xDS client that subscribes to policy chain configurations via ADS
type Client struct {
	config           *Config
	handler          *ResourceHandler
	reconnectManager *ReconnectManager

	// Connection state
	mu     sync.RWMutex
	state  ClientState
	conn   *grpc.ClientConn
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient
	// Track versions separately for each resource type to avoid version confusion
	policyChainVersion  string
	apiKeyVersion       string
	lazyResourceVersion string
	currentNonce        string

	// Lifecycle management
	ctx         context.Context
	cancel      context.CancelFunc
	stopOnce    sync.Once
	stoppedCh   chan struct{}
	reconnectCh chan struct{}
}

// NewClient creates a new xDS client
func NewClient(config *Config, k *kernel.Kernel, reg *registry.PolicyRegistry) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:           config,
		handler:          NewResourceHandler(k, reg),
		reconnectManager: NewReconnectManager(config),
		state:            StateDisconnected,
		ctx:              ctx,
		cancel:           cancel,
		stoppedCh:        make(chan struct{}),
		reconnectCh:      make(chan struct{}, 1),
	}, nil
}

// Start starts the xDS client and begins receiving configuration updates
func (c *Client) Start() error {
	slog.InfoContext(c.ctx, "Starting xDS client",
		"server", c.config.ServerAddress,
		"node_id", c.config.NodeID)

	go c.run()

	return nil
}

// Stop gracefully stops the xDS client
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		slog.InfoContext(c.ctx, "Stopping xDS client")

		c.setState(StateStopped)
		c.cancel()

		// Close connection if open
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()

		close(c.stoppedCh)
		slog.InfoContext(c.ctx, "xDS client stopped")
	})
}

// Wait blocks until the client is fully stopped
func (c *Client) Wait() {
	<-c.stoppedCh
}

// GetState returns the current client state
func (c *Client) GetState() ClientState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// setState updates the client state
func (c *Client) setState(state ClientState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != state {
		oldState := c.state
		c.state = state

		// Update metrics based on state
		switch state {
		case StateConnected:
			metrics.XDSConnectionState.WithLabelValues("connected").Set(1)
			metrics.GRPCConnectionsActive.WithLabelValues("xds").Inc()
			slog.InfoContext(c.ctx, "Client state changed",
				"old_state", oldState,
				"new_state", state)
		case StateDisconnected, StateStopped:
			metrics.XDSConnectionState.WithLabelValues("connected").Set(0)
			metrics.GRPCConnectionsActive.WithLabelValues("xds").Dec()
			slog.InfoContext(c.ctx, "Client state changed",
				"old_state", oldState,
				"new_state", state)
		case StateReconnecting:
			metrics.XDSConnectionState.WithLabelValues("connected").Set(0)
			slog.InfoContext(c.ctx, "Client state changed",
				"old_state", oldState,
				"new_state", state)
		default:
			slog.InfoContext(c.ctx, "Client state changed",
				"old_state", oldState,
				"new_state", state)
		}
	}
}

// run is the main client loop
func (c *Client) run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Attempt to connect and run stream
		if err := c.connectAndRun(); err != nil {
			if c.ctx.Err() != nil {
				// Context cancelled, exit gracefully
				return
			}

			slog.ErrorContext(c.ctx, "Stream error, will reconnect", "error", err)
		}

		// Check if we should stop
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Wait before reconnecting
		c.setState(StateReconnecting)
		if err := c.reconnectManager.WaitWithContext(c.ctx); err != nil {
			// Context cancelled during wait
			return
		}
	}
}

// connectAndRun establishes connection and runs the ADS stream
func (c *Client) connectAndRun() error {
	startTime := time.Now()
	c.setState(StateConnecting)

	// Establish gRPC connection
	conn, err := c.dial()
	if err != nil {
		metrics.XDSUpdatesTotal.WithLabelValues("failed", "xds").Inc()
		return fmt.Errorf("failed to dial xDS server: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	// Create ADS stream
	client := discoveryv3.NewAggregatedDiscoveryServiceClient(conn)
	stream, err := client.StreamAggregatedResources(c.ctx)
	if err != nil {
		metrics.XDSUpdatesTotal.WithLabelValues("failed", "xds").Inc()
		return fmt.Errorf("failed to create ADS stream: %w", err)
	}

	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()

	c.setState(StateConnected)
	c.reconnectManager.Reset()

	// Record successful connection metrics
	metrics.XDSUpdatesTotal.WithLabelValues("success", "xds").Inc()
	metrics.ContextBuildDurationSeconds.WithLabelValues("xds_connection").Observe(time.Since(startTime).Seconds())

	slog.InfoContext(c.ctx, "Connected to xDS server", "server", c.config.ServerAddress)

	// Send initial subscription request
	if err := c.sendDiscoveryRequest("", ""); err != nil {
		return fmt.Errorf("failed to send initial request: %w", err)
	}

	// Process responses
	return c.processStream(stream)
}

// dial establishes a gRPC connection to the xDS server
func (c *Client) dial() (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.config.ConnectTimeout)
	defer cancel()

	var opts []grpc.DialOption

	// Configure TLS or insecure
	if c.config.TLSEnabled {
		tlsConfig, err := c.loadTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add keepalive parameters
	opts = append(opts,
		grpc.WithBlock(),
	)

	conn, err := grpc.DialContext(ctx, c.config.ServerAddress, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// loadTLSConfig loads TLS configuration from files
func (c *Client) loadTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.config.TLSCertPath, c.config.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caCert, err := os.ReadFile(c.config.TLSCAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// sendDiscoveryRequest sends a DiscoveryRequest to the xDS server
func (c *Client) sendDiscoveryRequest(versionInfo, responseNonce string) error {
	c.mu.RLock()
	stream := c.stream
	policyVersion := c.policyChainVersion
	apiKeyVersion := c.apiKeyVersion
	lazyResourceVersion := c.lazyResourceVersion
	c.mu.RUnlock()

	if stream == nil {
		return fmt.Errorf("stream is not available")
	}

	// Send policy chain subscription with its own version
	policyReq := &discoveryv3.DiscoveryRequest{
		TypeUrl:       PolicyChainTypeURL,
		VersionInfo:   policyVersion,
		ResponseNonce: responseNonce,
		Node: &corev3.Node{
			Id:      c.config.NodeID,
			Cluster: c.config.Cluster,
		},
	}

	slog.DebugContext(c.ctx, "Sending policy chain discovery request",
		"type_url", policyReq.TypeUrl,
		"version", policyVersion,
		"nonce", responseNonce)

	if err := stream.Send(policyReq); err != nil {
		return fmt.Errorf("failed to send policy chain request: %w", err)
	}

	// Send API key subscription with its own version
	apiKeyReq := &discoveryv3.DiscoveryRequest{
		TypeUrl:       APIKeyStateTypeURL,
		VersionInfo:   apiKeyVersion,
		ResponseNonce: responseNonce,
		Node: &corev3.Node{
			Id:      c.config.NodeID,
			Cluster: c.config.Cluster,
		},
	}

	slog.DebugContext(c.ctx, "Sending API key discovery request",
		"type_url", apiKeyReq.TypeUrl,
		"version", apiKeyVersion,
		"nonce", responseNonce)

	if err := stream.Send(apiKeyReq); err != nil {
		return fmt.Errorf("failed to send API key request: %w", err)
	}

	// Send lazy resource subscription with its own version
	lazyResourceReq := &discoveryv3.DiscoveryRequest{
		TypeUrl:       LazyResourceTypeURL,
		VersionInfo:   lazyResourceVersion,
		ResponseNonce: responseNonce,
		Node: &corev3.Node{
			Id:      c.config.NodeID,
			Cluster: c.config.Cluster,
		},
	}

	slog.DebugContext(c.ctx, "Sending lazy resource discovery request",
		"type_url", lazyResourceReq.TypeUrl,
		"version", lazyResourceVersion,
		"nonce", responseNonce)

	if err := stream.Send(lazyResourceReq); err != nil {
		return fmt.Errorf("failed to send lazy resource request: %w", err)
	}

	return nil
}

// processStream processes incoming DiscoveryResponse messages
func (c *Client) processStream(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient) error {
	for {
		// Receive response
		resp, err := stream.Recv()
		if err == io.EOF {
			slog.InfoContext(c.ctx, "xDS stream closed by server")
			return err
		}
		if err != nil {
			return fmt.Errorf("error receiving from stream: %w", err)
		}

		// Process response
		if err := c.handleDiscoveryResponse(resp); err != nil {
			slog.ErrorContext(c.ctx, "Failed to handle discovery response",
				"type_url", resp.TypeUrl,
				"version", resp.VersionInfo,
				"error", err)

			// Send NACK with the appropriate current version for this resource type
			c.mu.RLock()
			currentVersion := ""
			switch resp.TypeUrl {
			case PolicyChainTypeURL:
				currentVersion = c.policyChainVersion
			case APIKeyStateTypeURL:
				currentVersion = c.apiKeyVersion
			case LazyResourceTypeURL:
				currentVersion = c.lazyResourceVersion
			}
			c.mu.RUnlock()

			if sendErr := c.sendDiscoveryRequestForType(resp.TypeUrl, currentVersion, resp.Nonce); sendErr != nil {
				return fmt.Errorf("failed to send NACK: %w", sendErr)
			}

			// Record NACK metric
			metrics.XDSUpdatesTotal.WithLabelValues("nack", "xds").Inc()
			continue
		}

		// Update version for the specific resource type and send ACK
		c.mu.Lock()
		switch resp.TypeUrl {
		case PolicyChainTypeURL:
			c.policyChainVersion = resp.VersionInfo
		case APIKeyStateTypeURL:
			c.apiKeyVersion = resp.VersionInfo
		case LazyResourceTypeURL:
			c.lazyResourceVersion = resp.VersionInfo
		}
		c.currentNonce = resp.Nonce
		c.mu.Unlock()

		// Send ACK for this specific resource type
		if err := c.sendDiscoveryRequestForType(resp.TypeUrl, resp.VersionInfo, resp.Nonce); err != nil {
			return fmt.Errorf("failed to send ACK: %w", err)
		}

		// Record successful ACK metric
		metrics.XDSUpdatesTotal.WithLabelValues("ack", "xds").Inc()

		// Record snapshot size metric
		var resourceType string
		switch resp.TypeUrl {
		case PolicyChainTypeURL:
			resourceType = "policy_chain"
		case APIKeyStateTypeURL:
			resourceType = "api_key_state"
		case LazyResourceTypeURL:
			resourceType = "lazy_resource"
		default:
			resourceType = "unknown"
		}

		// Calculate total size of resources in bytes
		var totalSize int
		for _, resource := range resp.Resources {
			totalSize += proto.Size(resource)
		}

		// Record snapshot size
		metrics.SnapshotSize.WithLabelValues(resourceType).Set(float64(totalSize))

		slog.InfoContext(c.ctx, "Successfully processed and ACKed discovery response",
			"type_url", resp.TypeUrl,
			"version", resp.VersionInfo,
			"num_resources", len(resp.Resources))
	}
}

// sendDiscoveryRequestForType sends a DiscoveryRequest for a specific resource type with its own version
func (c *Client) sendDiscoveryRequestForType(typeURL, versionInfo, responseNonce string) error {
	c.mu.RLock()
	stream := c.stream
	c.mu.RUnlock()

	if stream == nil {
		return fmt.Errorf("stream is not available")
	}

	req := &discoveryv3.DiscoveryRequest{
		TypeUrl:       typeURL,
		VersionInfo:   versionInfo,
		ResponseNonce: responseNonce,
		Node: &corev3.Node{
			Id:      c.config.NodeID,
			Cluster: c.config.Cluster,
		},
	}

	slog.DebugContext(c.ctx, "Sending discovery request for specific type",
		"type_url", typeURL,
		"version", versionInfo,
		"nonce", responseNonce)

	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send request for %s: %w", typeURL, err)
	}

	return nil
}

// handleDiscoveryResponse processes a DiscoveryResponse
func (c *Client) handleDiscoveryResponse(resp *discoveryv3.DiscoveryResponse) error {
	slog.InfoContext(c.ctx, "Received discovery response",
		"type_url", resp.TypeUrl,
		"version", resp.VersionInfo,
		"nonce", resp.Nonce,
		"num_resources", len(resp.Resources))

	// Handle different resource types
	switch resp.TypeUrl {
	case PolicyChainTypeURL:
		// Handle policy chain updates
		return c.handler.HandlePolicyChainUpdate(c.ctx, resp.Resources, resp.VersionInfo)

	case APIKeyStateTypeURL:
		// Handle API key operation updates
		resourceMap := make(map[string]*anypb.Any)
		for i, resource := range resp.Resources {
			resourceMap[fmt.Sprintf("resource_%d", i)] = resource
		}
		return c.handler.apiKeyHandler.HandleAPIKeyOperation(c.ctx, resourceMap)

	case LazyResourceTypeURL:
		// Handle lazy resource updates
		resourceMap := make(map[string]*anypb.Any)
		for i, resource := range resp.Resources {
			resourceMap[fmt.Sprintf("resource_%d", i)] = resource
		}
		return c.handler.lazyResourceHandler.HandleLazyResourceUpdate(c.ctx, resourceMap)

	default:
		return fmt.Errorf("unexpected type URL: %s", resp.TypeUrl)
	}
}
