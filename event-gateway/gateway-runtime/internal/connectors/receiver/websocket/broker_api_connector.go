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

package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// BrokerApiOptions holds configuration for WebBrokerApi receiver.
type BrokerApiOptions struct {
	Port                int
	ConsumerGroupPrefix string
	Topics              []string // Topics to subscribe to from broker
}

// WebBrokerApiReceiver implements protocol mediation for WebBrokerApi.
// Each WebSocket connection gets:
//   - Dedicated Kafka consumer (unique consumer group)
//   - Dedicated Kafka producer
//   - Inbound Go channel (client → broker)
//   - Outbound Go channel (broker → client)
type WebBrokerApiReceiver struct {
	channel      connectors.ChannelInfo
	processor    connectors.MessageProcessor
	brokerDriver connectors.BrokerDriver
	opts         BrokerApiOptions
	mu           sync.Mutex
	connections  map[string]*brokerApiConnection // connID → connection
	ctx          context.Context
}

// brokerApiConnection represents a single WebSocket connection with bidirectional channels.
type brokerApiConnection struct {
	connID        string
	ws            *websocket.Conn
	inbound       chan *connectors.Message // client → broker
	outbound      chan *connectors.Message // broker → client
	kafkaConsumer connectors.Receiver
	cancel        context.CancelFunc
	closed        bool
	mu            sync.Mutex
}

// NewBrokerApiReceiver creates a WebSocket receiver for WebBrokerApi protocol mediation.
func NewBrokerApiReceiver(cfg connectors.ReceiverConfig, opts BrokerApiOptions) (connectors.Receiver, error) {
	e := &WebBrokerApiReceiver{
		channel:      cfg.Channel,
		processor:    cfg.Processor,
		brokerDriver: cfg.BrokerDriver,
		opts:         opts,
		connections:  make(map[string]*brokerApiConnection),
	}

	// Register upgrade handler on shared mux.
	cfg.Mux.HandleFunc(cfg.Channel.Context, e.handleUpgrade)

	slog.Info("WebBrokerApi receiver registered HTTP handler",
		"channel", cfg.Channel.Name,
		"path", cfg.Channel.Context,
		"mode", cfg.Channel.Mode,
		"port", opts.Port)

	return e, nil
}

// Start initializes the receiver.
func (e *WebBrokerApiReceiver) Start(ctx context.Context) error {
	e.ctx = ctx

	// Ensure all topics exist in Kafka.
	if len(e.opts.Topics) > 0 {
		slog.Info("Ensuring Kafka topics exist",
			"channel", e.channel.Name,
			"topics", e.opts.Topics)
		if err := e.brokerDriver.EnsureTopics(ctx, e.opts.Topics); err != nil {
			return fmt.Errorf("failed to ensure kafka topics: %w", err)
		}
		slog.Info("Kafka topics verified",
			"channel", e.channel.Name,
			"topics", e.opts.Topics)
	} else {
		slog.Warn("No Kafka topics configured for WebBrokerApi",
			"channel", e.channel.Name)
	}

	slog.Info("WebBrokerApi WebSocket receiver started",
		"channel", e.channel.Name,
		"context", e.channel.Context,
		"topics", e.opts.Topics,
		"listening_on", fmt.Sprintf("ws://0.0.0.0:%d%s", e.opts.Port, e.channel.Context))
	return nil
}

// Stop closes all connections.
func (e *WebBrokerApiReceiver) Stop(ctx context.Context) error {
	e.mu.Lock()
	snapshot := make(map[string]*brokerApiConnection, len(e.connections))
	for k, v := range e.connections {
		snapshot[k] = v
	}
	e.mu.Unlock()

	for _, conn := range snapshot {
		e.closeConnection(conn)
	}

	return nil
}

// handleUpgrade handles WebSocket upgrade requests.
func (e *WebBrokerApiReceiver) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	// TODO: Change detailed flow logs ([1]-[8]) to debug level before production deployment
	// These Info-level logs are useful for development/testing but should be Debug in production
	slog.Info("[1] WebSocket connection attempted",
		"channel", e.channel.Name,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"upgrade_header", r.Header.Get("Upgrade"),
		"connection_header", r.Header.Get("Connection"))

	// Apply on_connection_init.request policies.
	slog.Info("[2] Applying onConnectionInit.request policies",
		"channel", e.channel.Name,
		"remote_addr", r.RemoteAddr)

	msg := &connectors.Message{
		Headers: r.Header,
		Metadata: map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		},
	}

	processed, shortCircuited, err := e.processor.ProcessConnectionInitRequest(r.Context(), e.channel.Name, msg)
	if err != nil {
		slog.Error("[2] onConnectionInit.request policy failed", "channel", e.channel.Name, "error", err)
		http.Error(w, "connection init failed", http.StatusForbidden)
		return
	}
	if shortCircuited {
		slog.Warn("[2] Connection rejected by onConnectionInit.request policy", "channel", e.channel.Name)
		// Policy rejected the connection.
		statusCode := http.StatusForbidden
		if sc, ok := processed.Metadata["status_code"].(int); ok {
			statusCode = sc
		}
		for k, vals := range processed.Headers {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(statusCode)
		w.Write(processed.Value)
		return
	}

	// Update request headers from policy result.
	for k, vals := range processed.Headers {
		r.Header.Del(k)
		for _, v := range vals {
			r.Header.Add(k, v)
		}
	}

	// Upgrade to WebSocket.
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	// Apply on_connection_init.response policies (if needed).
	slog.Info("[3] Applying onConnectionInit.response policies", "channel", e.channel.Name)

	respMsg := &connectors.Message{
		Headers: map[string][]string{},
	}
	if _, err := e.processor.ProcessConnectionInitResponse(r.Context(), e.channel.Name, respMsg); err != nil {
		slog.Error("[3] onConnectionInit.response policy failed", "channel", e.channel.Name, "error", err)
		ws.Close()
		return
	}

	// Create per-connection resources.
	connID := uuid.New().String()
	ctx, cancel := context.WithCancel(e.ctx)

	conn := &brokerApiConnection{
		connID:   connID,
		ws:       ws,
		inbound:  make(chan *connectors.Message, 256),
		outbound: make(chan *connectors.Message, 256),
		cancel:   cancel,
	}

	// Create unique consumer group for this connection.
	groupID := fmt.Sprintf("%s-ws-%s", e.opts.ConsumerGroupPrefix, connID)
	consumer, err := e.brokerDriver.Subscribe(groupID, e.opts.Topics, func(ctx context.Context, msg *connectors.Message) error {
		// Kafka message received → outbound channel.
		select {
		case conn.outbound <- msg:
		case <-ctx.Done():
			return ctx.Err()
		default:
			slog.Warn("Outbound channel full, dropping message", "connID", connID)
		}
		return nil
	})
	if err != nil {
		slog.Error("Failed to create per-connection consumer", "connID", connID, "error", err)
		ws.Close()
		cancel()
		return
	}
	conn.kafkaConsumer = consumer

	// Start the consumer.
	if err := consumer.Start(ctx); err != nil {
		slog.Error("Failed to start per-connection consumer", "connID", connID, "error", err)
		ws.Close()
		cancel()
		return
	}

	// Register connection.
	e.mu.Lock()
	e.connections[connID] = conn
	e.mu.Unlock()

	slog.Info("[4] WebSocket handshake completed", "connID", connID, "channel", e.channel.Name, "remote", ws.RemoteAddr(), "consumer_group", groupID)

	// Start goroutines for bidirectional communication.
	go e.inboundLoop(ctx, conn)
	go e.outboundLoop(ctx, conn)
	go e.readLoop(ctx, conn)
}

// readLoop reads WebSocket messages and sends them to the inbound channel.
func (e *WebBrokerApiReceiver) readLoop(ctx context.Context, conn *brokerApiConnection) {
	defer e.closeConnection(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgType, data, err := conn.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("WebSocket read error", "connID", conn.connID, "error", err)
			}
			return
		}

		if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
			continue
		}

		slog.Info("[5] Message received from WebSocket client",
			"connID", conn.connID,
			"channel", e.channel.Name,
			"size_bytes", len(data))

		// Extract headers from WebSocket message (if any).
		// For now, we'll just pass the raw data.
		msg := &connectors.Message{
			Value:   data,
			Headers: make(map[string][]string),
		}

		select {
		case conn.inbound <- msg:
		case <-ctx.Done():
			return
		default:
			slog.Warn("Inbound channel full, dropping message", "connID", conn.connID)
		}
	}
}

// inboundLoop processes messages from client → broker.
func (e *WebBrokerApiReceiver) inboundLoop(ctx context.Context, conn *brokerApiConnection) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-conn.inbound:
			// Apply on_produce policies.
			slog.Info("[5] Applying onProduce policies",
				"connID", conn.connID,
				"channel", e.channel.Name,
				"message_size", len(msg.Value))

			processed, shortCircuited, err := e.processor.ProcessProduce(ctx, e.channel.Name, msg)
			if err != nil {
				slog.Error("[5] onProduce policy failed", "connID", conn.connID, "channel", e.channel.Name, "error", err)
				continue
			}
			if shortCircuited {
				slog.Info("[5] Message dropped by onProduce policy", "connID", conn.connID, "channel", e.channel.Name)
				continue
			}

			// Determine target topic from processed message.
			// The topic should be set by policies (e.g., map-topics policy).
			targetTopic := processed.Topic
			if targetTopic == "" {
				// Fallback to default topic.
				if len(e.opts.Topics) > 0 {
					targetTopic = e.opts.Topics[0]
				} else {
					slog.Error("No target topic determined for message", "connID", conn.connID)
					continue
				}
			}

			// Publish to Kafka.
			slog.Info("[6] Publishing message to Kafka",
				"connID", conn.connID,
				"channel", e.channel.Name,
				"topic", targetTopic,
				"message_size", len(processed.Value))

			if err := e.brokerDriver.Publish(ctx, targetTopic, processed); err != nil {
				slog.Error("[6] Failed to publish to Kafka", "connID", conn.connID, "topic", targetTopic, "error", err)
			} else {
				slog.Info("[6] Message successfully published to Kafka", "connID", conn.connID, "topic", targetTopic)
			}
		}
	}
}

// outboundLoop processes messages from broker → client.
func (e *WebBrokerApiReceiver) outboundLoop(ctx context.Context, conn *brokerApiConnection) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-conn.outbound:
			slog.Info("[7] Applying onConsume policies",
				"connID", conn.connID,
				"channel", e.channel.Name,
				"message_size", len(msg.Value))

			// Apply on_consume policies.
			processed, shortCircuited, err := e.processor.ProcessConsume(ctx, e.channel.Name, msg)
			if err != nil {
				slog.Error("[7] onConsume policy failed", "connID", conn.connID, "channel", e.channel.Name, "error", err)
				continue
			}
			if shortCircuited {
				slog.Info("[7] Message dropped by onConsume policy", "connID", conn.connID, "channel", e.channel.Name)
				continue
			}

			slog.Info("[8] Sending message to WebSocket client",
				"connID", conn.connID,
				"channel", e.channel.Name,
				"message_size", len(processed.Value))

			// Send to WebSocket client.
			if err := conn.ws.WriteMessage(websocket.BinaryMessage, processed.Value); err != nil {
				slog.Error("[8] Failed to write to WebSocket", "connID", conn.connID, "error", err)
				return
			}
		}
	}
}

// closeConnection closes a connection and cleans up resources.
func (e *WebBrokerApiReceiver) closeConnection(conn *brokerApiConnection) {
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return
	}
	conn.closed = true
	conn.mu.Unlock()

	conn.cancel()

	if conn.kafkaConsumer != nil {
		if err := conn.kafkaConsumer.Stop(context.Background()); err != nil {
			slog.Error("Failed to stop per-connection consumer", "connID", conn.connID, "error", err)
		}
	}

	close(conn.inbound)
	close(conn.outbound)
	conn.ws.Close()

	e.mu.Lock()
	delete(e.connections, conn.connID)
	e.mu.Unlock()

	slog.Info("WebSocket connection closed", "connID", conn.connID, "channel", e.channel.Name)
}
