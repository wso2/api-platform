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
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// Options holds WebSocket-specific configuration passed at registration time.
type Options struct {
	Port                int
	ConsumerGroupPrefix string
}

// WebSocketReceiver is a single-channel WebSocket receiver.
// For protocol mediation each client connection gets a dedicated broker-driver
// consumer, giving 1:1 passthrough between web-friendly and broker-friendly protocols.
type WebSocketReceiver struct {
	server       *Server
	channel      connectors.ChannelInfo
	processor    connectors.MessageProcessor
	brokerDriver connectors.BrokerDriver
	opts         Options
	mu           sync.Mutex
	consumers    map[*connection]connectors.Receiver
	ctx          context.Context
}

// NewReceiver creates a WebSocket receiver for a single channel.
// It registers its handler on the shared HTTP mux provided in cfg.
func NewReceiver(cfg connectors.ReceiverConfig, opts Options) (connectors.Receiver, error) {
	e := &WebSocketReceiver{
		channel:      cfg.Channel,
		processor:    cfg.Processor,
		brokerDriver: cfg.BrokerDriver,
		opts:         opts,
		consumers:    make(map[*connection]connectors.Receiver),
	}

	wsConfig := DefaultServerConfig()
	server := NewServer(wsConfig, cfg.Channel.Name, func(ctx context.Context, msg *connectors.Message) error {
		processed, shortCircuited, err := cfg.Processor.ProcessInbound(ctx, cfg.Channel.Name, msg)
		if err != nil {
			return err
		}
		if shortCircuited {
			return nil
		}
		return cfg.BrokerDriver.Publish(ctx, cfg.Channel.BrokerDriverTopic, processed)
	})

	// Set connection lifecycle callbacks for 1:1 passthrough.
	server.onConnect = e.onConnect
	server.onDisconnect = e.onDisconnect
	e.server = server

	// Register handler on shared mux under the channel's context path.
	cfg.Mux.HandleFunc(cfg.Channel.Context+"/", server.HandleUpgrade)

	return e, nil
}

// Start initializes the receiver. The HTTP server is managed by the runtime.
func (e *WebSocketReceiver) Start(ctx context.Context) error {
	e.ctx = ctx
	slog.Info("WebSocket receiver started",
		"channel", e.channel.Name,
		"context", e.channel.Context,
	)
	return nil
}

// Stop closes all connections and stops per-connection consumers.
func (e *WebSocketReceiver) Stop(ctx context.Context) error {
	e.mu.Lock()
	snapshot := make(map[*connection]connectors.Receiver, len(e.consumers))
	for k, v := range e.consumers {
		snapshot[k] = v
	}
	e.mu.Unlock()

	for _, consumer := range snapshot {
		if err := consumer.Stop(ctx); err != nil {
			slog.Error("Failed to stop per-connection consumer", "error", err)
		}
	}

	e.server.CloseAll()
	return nil
}

// onConnect creates a dedicated broker-driver consumer for the new connection (1:1 passthrough).
func (e *WebSocketReceiver) onConnect(conn *connection) {
	groupID := e.opts.ConsumerGroupPrefix + "-ws-" + uuid.New().String()
	consumer, err := e.brokerDriver.Subscribe(groupID, []string{e.channel.BrokerDriverTopic},
		func(ctx context.Context, msg *connectors.Message) error {
			processed, shortCircuited, err := e.processor.ProcessOutbound(ctx, e.channel.Name, msg)
			if err != nil {
				return err
			}
			if shortCircuited {
				return nil
			}
			select {
			case conn.send <- processed.Value:
			default:
				slog.Warn("Dropping message for slow consumer", "channel", e.channel.Name)
			}
			return nil
		})
	if err != nil {
		slog.Error("Failed to create per-connection consumer",
			"channel", e.channel.Name,
			"error", err,
		)
		return
	}

	ctx := e.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := consumer.Start(ctx); err != nil {
		slog.Error("Failed to start per-connection consumer",
			"channel", e.channel.Name,
			"error", err,
		)
		return
	}

	e.mu.Lock()
	e.consumers[conn] = consumer
	e.mu.Unlock()

	slog.Info("Created 1:1 consumer for connection",
		"channel", e.channel.Name,
		"consumer_group", groupID,
	)
}

// onDisconnect stops and removes the consumer for the disconnected connection.
func (e *WebSocketReceiver) onDisconnect(conn *connection) {
	e.mu.Lock()
	consumer, ok := e.consumers[conn]
	delete(e.consumers, conn)
	e.mu.Unlock()

	if ok && consumer != nil {
		if err := consumer.Stop(context.Background()); err != nil {
			slog.Error("Failed to stop per-connection consumer", "error", err)
		}
	}
}
