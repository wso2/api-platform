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
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
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
	connID          string
	ws              *websocket.Conn
	inbound         chan *connectors.Message // client → broker
	outbound        chan *connectors.Message // broker → client
	kafkaConsumer   connectors.Receiver
	cancel          context.CancelFunc
	closed          bool
	mu              sync.Mutex
	channelName     string // Selected channel from X-channel header
	produceChainKey string // Policy chain key for on_produce
	consumeChainKey string // Policy chain key for on_consume
	produceTopic    string // Target Kafka topic for producing messages
	consumeTopic    string // Source Kafka topic for consuming messages
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
	// Extract channel name from X-channel header.
	xChannelHeader := r.Header.Get("X-channel")
	channelName := xChannelHeader
	if channelName == "" {
		slog.Error("Missing X-channel header in WebSocket connection", "api", e.channel.Name, "remote", r.RemoteAddr)
		http.Error(w, "Missing X-channel header", http.StatusBadRequest)
		return
	}

	// Validate channel exists in metadata.
	channelNamesIface, ok := e.channel.Metadata["channelNames"]
	if !ok {
		slog.Error("Missing channelNames in metadata", "api", e.channel.Name, "remote", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	channelNames, ok := channelNamesIface.([]string)
	if !ok {
		slog.Error("Invalid channelNames metadata type", "api", e.channel.Name, "remote", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	channelExists := false
	for _, ch := range channelNames {
		if ch == channelName {
			channelExists = true
			break
		}
	}

	if !channelExists {
		slog.Error("Unknown channel in X-channel header", "api", e.channel.Name, "channel", channelName, "remote", r.RemoteAddr)
		http.Error(w, fmt.Sprintf("Unknown channel: %s", channelName), http.StatusNotFound)
		return
	}

	slog.Debug("[1] WebSocket connection attempted",
		"api", e.channel.Name,
		"channel", channelName,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"upgrade_header", r.Header.Get("Upgrade"),
		"connection_header", r.Header.Get("Connection"))

	// Apply API-level on_connection_init.request policies.
	slog.Debug("[2] Applying API-level onConnectionInit.request policies",
		"api", e.channel.Name,
		"channel", channelName,
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

	// Apply API-level on_connection_init.response policies.
	slog.Debug("[3] Applying API-level onConnectionInit.response policies", "api", e.channel.Name, "channel", channelName)

	respMsg := &connectors.Message{
		Headers: map[string][]string{},
	}
	if _, err := e.processor.ProcessConnectionInitResponse(r.Context(), e.channel.Name, respMsg); err != nil {
		slog.Error("[3] onConnectionInit.response policy failed", "channel", e.channel.Name, "error", err)
		ws.Close()
		return
	}

	// Extract channel-specific policy chain keys from metadata.
	var produceChainKey, consumeChainKey string
	var produceTopic, consumeTopic string
	if channelChainsIface, ok := e.channel.Metadata["channelChains"]; ok {
		// channelChains is stored as map[string]map[string]string
		if channelChainsMap, ok := channelChainsIface.(map[string]map[string]string); ok {
			if chainData, ok := channelChainsMap[channelName]; ok {
				produceChainKey = chainData["ProduceKey"]
				consumeChainKey = chainData["ConsumeKey"]
			}
		}
	}

	// Extract channel topic mappings (produceTo, consumeFrom)
	if channelTopicsIface, ok := e.channel.Metadata["channelTopics"]; ok {
		if channelTopicsMap, ok := channelTopicsIface.(map[string]map[string]string); ok {
			if topicMapping, ok := channelTopicsMap[channelName]; ok {
				produceTopic = topicMapping["produceTo"]
				consumeTopic = topicMapping["consumeFrom"]
			}
		}
	}

	// Determine topics for this channel from metadata.
	// Use the consumeTopic from channel config, fallback to topicToChannel mapping.
	channelTopics := []string{}
	if consumeTopic != "" {
		channelTopics = append(channelTopics, consumeTopic)
	} else {
		// Fallback: extract topics from topicToChannel mapping (legacy)
		topicToChannelIface, _ := e.channel.Metadata["topicToChannel"]
		topicToChannel, _ := topicToChannelIface.(map[string]string)
		for topic, ch := range topicToChannel {
			if ch == channelName {
				channelTopics = append(channelTopics, topic)
			}
		}
	}
	if len(channelTopics) == 0 {
		slog.Warn("No topics found for channel", "api", e.channel.Name, "channel", channelName)
	}

	// Create per-connection resources.
	connID := uuid.New().String()
	ctx, cancel := context.WithCancel(e.ctx)

	conn := &brokerApiConnection{
		connID:          connID,
		ws:              ws,
		inbound:         make(chan *connectors.Message, 256),
		outbound:        make(chan *connectors.Message, 256),
		cancel:          cancel,
		channelName:     channelName,
		produceChainKey: produceChainKey,
		consumeChainKey: consumeChainKey,
		produceTopic:    produceTopic,
		consumeTopic:    consumeTopic,
	}

	// Create unique consumer group for this connection.
	groupID := fmt.Sprintf("%s-ws-%s", e.opts.ConsumerGroupPrefix, connID)
	consumer, err := e.brokerDriver.Subscribe(groupID, channelTopics, func(ctx context.Context, msg *connectors.Message) error {
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

	slog.Debug("[4] WebSocket handshake completed", "connID", connID, "api", e.channel.Name, "channel", channelName, "remote", ws.RemoteAddr(), "consumer_group", groupID, "topics", channelTopics)

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

		slog.Debug("[5] Message received from WebSocket client",
			"connID", conn.connID,
			"api", e.channel.Name,
			"channel", conn.channelName,
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
			// Apply channel-specific on_produce policies.
			slog.Debug("[5] Applying channel onProduce policies",
				"connID", conn.connID,
				"api", e.channel.Name,
				"channel", conn.channelName,
				"chain_key", conn.produceChainKey,
				"message_size", len(msg.Value))

			// Set default topic to normalized channel name (can be overridden by policies)
			if msg.Topic == "" {
				msg.Topic = binding.NormalizeTopicSegment(conn.channelName)
			}

			// Use channel-specific policy chain key via ProcessByChainKey
			processed, shortCircuited, err := e.processor.ProcessByChainKey(ctx, e.channel.Name, conn.produceChainKey, msg)
			if err != nil {
				slog.Error("[5] onProduce policy failed", "connID", conn.connID, "api", e.channel.Name, "channel", conn.channelName, "error", err)
				continue
			}
			if shortCircuited {
				slog.Info("[5] Message dropped by onProduce policy", "connID", conn.connID, "api", e.channel.Name, "channel", conn.channelName)
				continue
			}

			// Determine target topic from channel config
			targetTopic := conn.produceTopic
			if targetTopic == "" {
				// Final fallback: normalized channel name
				targetTopic = binding.NormalizeTopicSegment(conn.channelName)
				slog.Warn("No target topic set in config or by policies, using normalized channel name as default",
					"connID", conn.connID,
					"channel", conn.channelName,
					"topic", targetTopic)
			}

			// Publish to Kafka.
			slog.Debug("[6] Publishing message to Kafka",
				"connID", conn.connID,
				"api", e.channel.Name,
				"channel", conn.channelName,
				"topic", targetTopic,
				"message_size", len(processed.Value))

			if err := e.brokerDriver.Publish(ctx, targetTopic, processed); err != nil {
				slog.Error("[6] Failed to publish to Kafka", "connID", conn.connID, "topic", targetTopic, "error", err)
			} else {
				slog.Debug("[6] Message successfully published to Kafka", "connID", conn.connID, "topic", targetTopic)
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
			slog.Debug("[7] Applying channel onConsume policies",
				"connID", conn.connID,
				"api", e.channel.Name,
				"channel", conn.channelName,
				"chain_key", conn.consumeChainKey,
				"message_size", len(msg.Value))

			// Use channel-specific policy chain key via ProcessByChainKey
			processed, shortCircuited, err := e.processor.ProcessByChainKey(ctx, e.channel.Name, conn.consumeChainKey, msg)
			if err != nil {
				slog.Error("[7] onConsume policy failed", "connID", conn.connID, "api", e.channel.Name, "channel", conn.channelName, "error", err)
				continue
			}
			if shortCircuited {
				slog.Info("[7] Message dropped by onConsume policy", "connID", conn.connID, "api", e.channel.Name, "channel", conn.channelName)
				continue
			}

			slog.Debug("[8] Sending message to WebSocket client",
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
