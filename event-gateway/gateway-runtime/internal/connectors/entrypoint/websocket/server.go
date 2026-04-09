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
	"time"

	"github.com/gorilla/websocket"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ServerConfig holds WebSocket server configuration.
type ServerConfig struct {
	PingInterval   time.Duration
	WriteTimeout   time.Duration
	Backpressure   string // "drop-oldest", "block", "close"
	WriteBufferCap int
}

// DefaultServerConfig returns sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		PingInterval:   30 * time.Second,
		WriteTimeout:   10 * time.Second,
		Backpressure:   "drop-oldest",
		WriteBufferCap: 256,
	}
}

// Server is the WebSocket entrypoint server.
type Server struct {
	mu           sync.RWMutex
	config       ServerConfig
	handler      connectors.MessageHandler
	channel      string                              // the channel this server handles
	connections  map[string]map[*connection]struct{} // channel -> connections
	onConnect    func(conn *connection)
	onDisconnect func(conn *connection)
}

// connection represents a single WebSocket connection bound to a channel.
type connection struct {
	conn    *websocket.Conn
	channel string
	send    chan []byte
}

// NewServer creates a new WebSocket server for a single channel.
func NewServer(config ServerConfig, channel string, handler connectors.MessageHandler) *Server {
	return &Server{
		config:      config,
		handler:     handler,
		channel:     channel,
		connections: make(map[string]map[*connection]struct{}),
	}
}

// HandleUpgrade is an HTTP handler that upgrades connections to WebSocket.
func (s *Server) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	conn := &connection{
		conn:    ws,
		channel: s.channel,
		send:    make(chan []byte, s.config.WriteBufferCap),
	}

	s.addConnection(s.channel, conn)

	if s.onConnect != nil {
		s.onConnect(conn)
	}

	go s.readLoop(conn)
	go s.writeLoop(conn)

	slog.Info("WebSocket connection established", "channel", s.channel, "remote", ws.RemoteAddr())
}

// Broadcast sends a message to all connections bound to the given channel.
func (s *Server) Broadcast(channel string, data []byte) {
	s.mu.RLock()
	conns := s.connections[channel]
	s.mu.RUnlock()

	for conn := range conns {
		switch s.config.Backpressure {
		case "drop-oldest":
			select {
			case conn.send <- data:
			default:
				// Drop oldest message
				select {
				case <-conn.send:
				default:
				}
				conn.send <- data
			}
		case "block":
			conn.send <- data
		case "close":
			select {
			case conn.send <- data:
			default:
				s.removeConnection(channel, conn)
				conn.conn.Close()
			}
		default:
			select {
			case conn.send <- data:
			default:
			}
		}
	}
}

// ConnectionCount returns the number of active connections for a channel.
func (s *Server) ConnectionCount(channel string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections[channel])
}

func (s *Server) readLoop(conn *connection) {
	defer func() {
		if s.onDisconnect != nil {
			s.onDisconnect(conn)
		}
		s.removeConnection(conn.channel, conn)
		conn.conn.Close()
	}()

	conn.conn.SetPongHandler(func(string) error {
		return nil
	})

	for {
		_, message, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("WebSocket read error", "channel", conn.channel, "error", err)
			}
			return
		}

		msg := &connectors.Message{
			Value:   message,
			Headers: make(map[string][]string),
			Topic:   conn.channel,
		}

		if err := s.handler(context.Background(), msg); err != nil {
			slog.Error("WebSocket message handler error", "channel", conn.channel, "error", err)
		}
	}
}

func (s *Server) writeLoop(conn *connection) {
	ticker := time.NewTicker(s.config.PingInterval)
	defer func() {
		ticker.Stop()
		conn.conn.Close()
	}()

	for {
		select {
		case message, ok := <-conn.send:
			if err := conn.conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
				return
			}
			if !ok {
				conn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
				return
			}
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) addConnection(channel string, conn *connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.connections[channel]; !ok {
		s.connections[channel] = make(map[*connection]struct{})
	}
	s.connections[channel][conn] = struct{}{}
}

func (s *Server) removeConnection(channel string, conn *connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conns, ok := s.connections[channel]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(s.connections, channel)
		}
	}
}

// CloseAll closes all WebSocket connections gracefully.
func (s *Server) CloseAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for channel, conns := range s.connections {
		for conn := range conns {
			conn.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"))
			conn.conn.Close()
			close(conn.send)
		}
		delete(s.connections, channel)
	}
	fmt.Println("All WebSocket connections closed")
}
