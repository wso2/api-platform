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

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/registry"
)

// Client is the xDS client that subscribes to policy chain configurations
type Client struct {
	config           *Config
	handler          *ResourceHandler
	reconnectManager *ReconnectManager

	// Connection state
	mu               sync.RWMutex
	state            ClientState
	conn             *grpc.ClientConn
	stream           discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient
	currentVersion   string
	currentNonce     string

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	stopOnce   sync.Once
	stoppedCh  chan struct{}
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
		slog.InfoContext(c.ctx, "Client state changed",
			"old_state", oldState,
			"new_state", state)
	}
}

// run is the main client loop
func (c *Client) run() {
	defer close(c.stoppedCh)

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
	c.setState(StateConnecting)

	// Establish gRPC connection
	conn, err := c.dial()
	if err != nil {
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
		return fmt.Errorf("failed to create ADS stream: %w", err)
	}

	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()

	c.setState(StateConnected)
	c.reconnectManager.Reset()

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
	c.mu.RUnlock()

	if stream == nil {
		return fmt.Errorf("stream is not available")
	}

	req := &discoveryv3.DiscoveryRequest{
		TypeUrl:       PolicyChainTypeURL,
		VersionInfo:   versionInfo,
		ResponseNonce: responseNonce,
		Node: &corev3.Node{
			Id:      c.config.NodeID,
			Cluster: c.config.Cluster,
		},
	}

	slog.DebugContext(c.ctx, "Sending discovery request",
		"type_url", req.TypeUrl,
		"version", versionInfo,
		"nonce", responseNonce)

	return stream.Send(req)
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
			slog.ErrorContext(c.ctx, "Failed to process discovery response",
				"error", err,
				"version", resp.VersionInfo,
				"nonce", resp.Nonce)

			// Send NACK
			if sendErr := c.sendDiscoveryRequest(c.currentVersion, resp.Nonce); sendErr != nil {
				return fmt.Errorf("failed to send NACK: %w", sendErr)
			}

			continue
		}

		// Update current version and nonce
		c.mu.Lock()
		c.currentVersion = resp.VersionInfo
		c.currentNonce = resp.Nonce
		c.mu.Unlock()

		// Send ACK
		if err := c.sendDiscoveryRequest(resp.VersionInfo, resp.Nonce); err != nil {
			return fmt.Errorf("failed to send ACK: %w", err)
		}

		slog.InfoContext(c.ctx, "Successfully processed and ACKed discovery response",
			"version", resp.VersionInfo,
			"num_resources", len(resp.Resources))
	}
}

// handleDiscoveryResponse processes a DiscoveryResponse
func (c *Client) handleDiscoveryResponse(resp *discoveryv3.DiscoveryResponse) error {
	slog.InfoContext(c.ctx, "Received discovery response",
		"type_url", resp.TypeUrl,
		"version", resp.VersionInfo,
		"nonce", resp.Nonce,
		"num_resources", len(resp.Resources))

	// Verify type URL
	if resp.TypeUrl != PolicyChainTypeURL {
		return fmt.Errorf("unexpected type URL: %s, expected: %s", resp.TypeUrl, PolicyChainTypeURL)
	}

	// Handle resource update
	return c.handler.HandlePolicyChainUpdate(c.ctx, resp.Resources, resp.VersionInfo)
}
