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
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// EventChannelConfigTypeURL is the xDS type URL for event channel configurations.
	EventChannelConfigTypeURL = "api-platform.wso2.org/v1.EventChannelConfig"
	// APIKeyStateTypeURL is the xDS type URL for API key state snapshots.
	APIKeyStateTypeURL = "api-platform.wso2.org/v1.APIKeyState"

	defaultNodeID  = "event-gateway-node"
	defaultCluster = "event-gateway"

	initialBackoff = 1 * time.Second
	maxBackoff     = 60 * time.Second
	connectTimeout = 10 * time.Second
)

// ResourceHandler is called when resources for the subscribed xDS type are received.
type ResourceHandler func(ctx context.Context, resources []*discoveryv3.Resource, version string) error

// Client connects to the gateway controller's xDS server and subscribes
// to one xDS resource type per ADS stream.
type Client struct {
	serverAddr string
	nodeID     string
	typeURL    string
	handler    ResourceHandler

	mu      sync.RWMutex
	conn    *grpc.ClientConn
	stream  discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient
	version string

	ctx       context.Context
	cancel    context.CancelFunc
	stoppedCh chan struct{}
	stopOnce  sync.Once
}

// NewClient creates a new xDS client for the event gateway.
func NewClient(serverAddr, nodeID, typeURL string, handler ResourceHandler) *Client {
	if nodeID == "" {
		nodeID = defaultNodeID
	}
	if typeURL == "" {
		typeURL = EventChannelConfigTypeURL
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		serverAddr: serverAddr,
		nodeID:     nodeID,
		typeURL:    typeURL,
		handler:    handler,
		ctx:        ctx,
		cancel:     cancel,
		stoppedCh:  make(chan struct{}),
	}
}

// Start begins the xDS client loop in a background goroutine.
func (c *Client) Start() {
	slog.Info("Starting event gateway xDS client",
		"server", c.serverAddr,
		"node_id", c.nodeID,
		"type_url", c.typeURL)
	go c.run()
}

// Stop gracefully stops the client.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		slog.Info("Stopping event gateway xDS client", "type_url", c.typeURL)
		c.cancel()

		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()

		close(c.stoppedCh)
	})
}

// Wait blocks until the client is stopped.
func (c *Client) Wait() {
	<-c.stoppedCh
}

func (c *Client) run() {
	backoff := initialBackoff
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if err := c.connectAndRun(); err != nil {
			if c.ctx.Err() != nil {
				return
			}
			slog.Error("xDS stream error, will reconnect",
				"error", err,
				"backoff", backoff)
		}

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, maxBackoff)
	}
}

func (c *Client) connectAndRun() error {
	dialCtx, dialCancel := context.WithTimeout(c.ctx, connectTimeout)
	defer dialCancel()

	conn, err := grpc.DialContext(dialCtx, c.serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
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

	client := discoveryv3.NewAggregatedDiscoveryServiceClient(conn)
	stream, err := client.StreamAggregatedResources(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to create ADS stream: %w", err)
	}

	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()

	slog.Info("Connected to xDS server",
		"server", c.serverAddr,
		"type_url", c.typeURL)

	// Send initial subscription for the configured resource type.
	if err := c.sendRequest("", ""); err != nil {
		return fmt.Errorf("failed to send initial request: %w", err)
	}

	return c.processStream(stream)
}

func (c *Client) sendRequest(versionInfo, nonce string) error {
	c.mu.RLock()
	stream := c.stream
	c.mu.RUnlock()

	if stream == nil {
		return fmt.Errorf("stream not available")
	}

	req := &discoveryv3.DiscoveryRequest{
		TypeUrl:       c.typeURL,
		VersionInfo:   versionInfo,
		ResponseNonce: nonce,
		Node: &corev3.Node{
			Id:      c.nodeID,
			Cluster: defaultCluster,
		},
	}

	return stream.Send(req)
}

func (c *Client) processStream(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient) error {
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			slog.Info("xDS stream closed by server", "type_url", c.typeURL)
			return err
		}
		if err != nil {
			return fmt.Errorf("error receiving from stream: %w", err)
		}

		if resp.TypeUrl != c.typeURL {
			slog.Warn("Ignoring unexpected resource type", "type_url", resp.TypeUrl)
			continue
		}

		// Convert anypb resources to discovery resources for the handler
		resources := make([]*discoveryv3.Resource, 0, len(resp.Resources))
		for _, any := range resp.Resources {
			resources = append(resources, &discoveryv3.Resource{
				Resource: any,
			})
		}

		if err := c.handler(c.ctx, resources, resp.VersionInfo); err != nil {
			slog.Error("Failed to handle xDS update",
				"type_url", c.typeURL,
				"version", resp.VersionInfo,
				"error", err)

			// NACK: send current version
			c.mu.RLock()
			curVersion := c.version
			c.mu.RUnlock()
			if sendErr := c.sendRequest(curVersion, resp.Nonce); sendErr != nil {
				return fmt.Errorf("failed to send NACK: %w", sendErr)
			}
			continue
		}

		// ACK
		c.mu.Lock()
		c.version = resp.VersionInfo
		c.mu.Unlock()

		if err := c.sendRequest(resp.VersionInfo, resp.Nonce); err != nil {
			return fmt.Errorf("failed to send ACK: %w", err)
		}

		slog.Info("Processed xDS update",
			"type_url", c.typeURL,
			"version", resp.VersionInfo,
			"resources", len(resp.Resources))
	}
}
