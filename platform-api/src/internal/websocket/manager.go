/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"platform-api/src/internal/repository"

	"github.com/google/uuid"
)

// Manager handles the lifecycle of gateway WebSocket connections.
// It maintains an in-memory registry of active connections, manages heartbeats,
// and handles graceful/ungraceful disconnections.
//
// Design rationale: sync.Map provides thread-safe concurrent access optimized
// for read-heavy workloads (event delivery lookups). The registry maps gateway IDs
// to slices of connections to support multiple connections per gateway (clustering).
type Manager struct {
	// connections maps gatewayID -> []*Connection
	// Supports multiple connections per gateway ID for clustering scenarios
	connections sync.Map

	// mu protects the connectionCount and maxConnections fields
	mu sync.RWMutex

	// connectionCount tracks the total number of active connections across all gateways
	connectionCount int

	// maxConnections enforces a limit on concurrent connections (default 1000)
	maxConnections int

	// heartbeatInterval specifies how often to send ping frames (default 20s)
	heartbeatInterval time.Duration

	// heartbeatTimeout specifies when to consider a connection dead (default 30s)
	heartbeatTimeout time.Duration

	// maxConnectionsPerOrg enforces per-organization connection limits
	maxConnectionsPerOrg int

	// gatewayRepo provides access to gateway data for org-scoped connection counting
	gatewayRepo repository.GatewayRepository

	// slogger is the structured logger instance
	slogger *slog.Logger

	// shutdownCtx is used to signal graceful shutdown to all connection goroutines
	shutdownCtx context.Context
	shutdownFn  context.CancelFunc

	// wg tracks active connection handler goroutines for graceful shutdown
	wg sync.WaitGroup

	// metricsLogEnabled controls whether periodic metrics logging is active
	metricsLogEnabled bool
	// metricsLogInterval is the duration between metrics log entries
	metricsLogInterval time.Duration

	// Atomic counters for metrics (reset each tick)
	successfulConnections int64
	failedConnections     int64
	disconnections        int64
	eventsSent            int64

	// Connection lifecycle hooks — called synchronously on connect/disconnect.
	// onConnect fires for every new connection; onDisconnect fires for every removal.
	// Both are optional (nil means no-op).
	// These fields are written only by SetConnectionHooks, which must be called during
	// initialization before the gateway accepts connections to avoid concurrent access.
	onConnect    func(gatewayID string) error
	onDisconnect func(gatewayID string) error
}

// ManagerConfig contains configuration parameters for the connection manager
type ManagerConfig struct {
	MaxConnections       int           // Maximum concurrent connections (default 1000)
	HeartbeatInterval    time.Duration // Ping interval (default 20s)
	HeartbeatTimeout     time.Duration // Pong timeout (default 30s)
	MaxConnectionsPerOrg int           // Maximum connections per organization (default 3)
	MetricsLogEnabled    bool          // Enable periodic metrics logging (default true)
	MetricsLogInterval   time.Duration // Interval between metrics log entries (default 10s)
}

type OrgConnectionStats struct {
	OrganizationID string `json:"organizationId"`
	CurrentCount   int    `json:"currentCount"`
	MaxAllowed     int    `json:"maxAllowed"`
}

// DefaultManagerConfig returns sensible default configuration values
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxConnections:       1000,
		HeartbeatInterval:    20 * time.Second,
		HeartbeatTimeout:     30 * time.Second,
		MaxConnectionsPerOrg: 3,
		MetricsLogEnabled:    true,
		MetricsLogInterval:   10 * time.Second,
	}
}

// NewManager creates a new connection manager with the provided configuration
func NewManager(config ManagerConfig, gatewayRepo repository.GatewayRepository, slogger *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		connections:          sync.Map{},
		connectionCount:      0,
		maxConnections:       config.MaxConnections,
		heartbeatInterval:    config.HeartbeatInterval,
		heartbeatTimeout:     config.HeartbeatTimeout,
		maxConnectionsPerOrg: config.MaxConnectionsPerOrg,
		gatewayRepo:          gatewayRepo,
		slogger:              slogger,
		shutdownCtx:          ctx,
		shutdownFn:           cancel,
		metricsLogEnabled:    config.MetricsLogEnabled,
		metricsLogInterval:   config.MetricsLogInterval,
	}
	// Disable metrics logging if the interval is non-positive to prevent
	// time.NewTicker from panicking.
	if mgr.metricsLogInterval <= 0 {
		mgr.metricsLogEnabled = false
		mgr.slogger.Warn("Metrics logging disabled: metricsLogInterval must be positive", "interval", mgr.metricsLogInterval)
	}
	if mgr.metricsLogEnabled {
		mgr.wg.Go(func() { mgr.startMetricsLogger() })
	}
	return mgr
}

// SetConnectionHooks registers optional callbacks invoked on every gateway connect and disconnect.
// onConnect is called after a connection is stored; onDisconnect after it is removed.
// Callbacks are invoked synchronously and must not acquire the manager's locks.
// This method must be called only during initialization, before the gateway begins accepting
// connections. It is not safe to call concurrently with Register or Unregister.
func (m *Manager) SetConnectionHooks(onConnect, onDisconnect func(gatewayID string) error) {
	m.onConnect = onConnect
	m.onDisconnect = onDisconnect
}

// Register adds a new connection to the registry and starts heartbeat monitoring.
// Returns an error if the maximum connection limit is reached.
//
// Parameters:
//   - gatewayID: UUID of the authenticated gateway
//   - transport: Transport implementation for message delivery
//   - authToken: API key used for authentication
//   - orgID: UUID of the organization that owns the gateway
//
// Returns the Connection instance and any error encountered.
//
// Design decision: Support multiple connections per gateway ID by storing
// connections in a slice. This enables gateway clustering where multiple
// instances share the same gateway identity.
func (m *Manager) Register(gatewayID string, transport Transport, authToken string,
	orgID string) (*Connection, error) {

	// Check per-org limit first (count from main connections map)
	orgCount := m.countOrgConnections(orgID)
	if orgCount >= m.maxConnectionsPerOrg {
		return nil, &OrgConnectionLimitError{
			OrganizationID: orgID,
			CurrentCount:   orgCount,
			MaxAllowed:     m.maxConnectionsPerOrg,
		}
	}

	// Check global connection limit
	m.mu.Lock()
	if m.connectionCount >= m.maxConnections {
		m.mu.Unlock()
		return nil, fmt.Errorf("maximum connection limit reached (%d)", m.maxConnections)
	}
	m.connectionCount++
	m.mu.Unlock()

	// Create connection
	connectionID := uuid.New().String()
	conn := NewConnection(gatewayID, connectionID, transport, authToken, orgID)

	// Add connection to registry
	connsInterface, _ := m.connections.LoadOrStore(gatewayID, []*Connection{})
	conns := connsInterface.([]*Connection)
	conns = append(conns, conn)
	m.connections.Store(gatewayID, conns)

	// Start heartbeat monitoring in background
	m.wg.Go(func() { m.monitorHeartbeat(conn) })

	m.IncrementSuccessfulConnections()

	m.slogger.Info("Gateway connected", "gatewayID", gatewayID, "connectionID", connectionID,
		"orgID", orgID, "totalConnections", m.GetConnectionCount(), "orgConnections", m.countOrgConnections(orgID))

	if m.onConnect != nil {
		m.onConnect(gatewayID)
	}

	return conn, nil
}

// Unregister removes a connection from the registry and closes it gracefully.
// This method is idempotent - calling it multiple times is safe.
//
// Parameters:
//   - gatewayID: UUID of the gateway
//   - connectionID: Unique identifier of the connection to remove
func (m *Manager) Unregister(gatewayID, connectionID string) {
	connsInterface, ok := m.connections.Load(gatewayID)
	if !ok {
		return // Gateway not found
	}

	conns := connsInterface.([]*Connection)
	var updatedConns []*Connection
	var removed *Connection

	// Filter out the connection to remove
	for _, conn := range conns {
		if conn.ConnectionID == connectionID {
			removed = conn
		} else {
			updatedConns = append(updatedConns, conn)
		}
	}

	if removed == nil {
		return // Connection not found
	}

	// Update or delete the gateway entry
	if len(updatedConns) == 0 {
		m.connections.Delete(gatewayID)
	} else {
		m.connections.Store(gatewayID, updatedConns)
	}

	// Close the connection gracefully
	if err := removed.Close(1000, "normal closure"); err != nil {
		m.slogger.Error("Failed to close connection", "gatewayID", gatewayID, "connectionID", connectionID, "error", err)
	}

	if m.onDisconnect != nil {
		m.onDisconnect(gatewayID)
	}

	// Decrement connection count
	m.mu.Lock()
	m.connectionCount--
	m.mu.Unlock()

	m.IncrementDisconnections()

	m.slogger.Info("Gateway disconnected", "gatewayID", gatewayID, "connectionID", connectionID,
		"orgID", removed.OrganizationID, "totalConnections", m.GetConnectionCount())
}

// GetConnections retrieves all connections for a specific gateway ID.
// Returns an empty slice if the gateway has no active connections.
//
// Thread-safe for concurrent access.
func (m *Manager) GetConnections(gatewayID string) []*Connection {
	connsInterface, ok := m.connections.Load(gatewayID)
	if !ok {
		return []*Connection{}
	}
	return connsInterface.([]*Connection)
}

// GetAllConnections returns all active connections across all gateways.
// Used by the stats API to provide operational visibility.
//
// Returns a map of gatewayID -> []*Connection
func (m *Manager) GetAllConnections() map[string][]*Connection {
	result := make(map[string][]*Connection)
	m.connections.Range(func(key, value interface{}) bool {
		gatewayID := key.(string)
		conns := value.([]*Connection)
		result[gatewayID] = conns
		return true // Continue iteration
	})
	return result
}

// GetConnectionCount returns the total number of active connections
func (m *Manager) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionCount
}

// countOrgConnections counts the number of connections for a specific organization
// by fetching the org's gateways and only counting connections for those gateway IDs.
func (m *Manager) countOrgConnections(orgID string) int {
	gateways, err := m.gatewayRepo.GetByOrganizationID(orgID)
	if err != nil {
		m.slogger.Error("Failed to fetch gateways for org", "orgID", orgID, "error", err)
		return 0
	}

	count := 0
	for _, gw := range gateways {
		if connsInterface, ok := m.connections.Load(gw.ID); ok {
			conns := connsInterface.([]*Connection)
			count += len(conns)
		}
	}
	return count
}

// monitorHeartbeat periodically sends ping frames and detects connection death.
// Runs in a background goroutine for each connection.
//
// Parameters:
//   - conn: The connection to monitor
//
// The goroutine exits when:
//   - The connection is explicitly closed
//   - Heartbeat timeout is detected (no pong received)
//   - Manager shutdown is triggered
func (m *Manager) monitorHeartbeat(conn *Connection) {
	ticker := time.NewTicker(m.heartbeatInterval)
	defer ticker.Stop()

	// Configure pong handler to update heartbeat timestamp
	conn.Transport.EnablePongHandler(func(appData string) error {
		conn.UpdateHeartbeat()
		return nil
	})

	for {
		select {
		case <-m.shutdownCtx.Done():
			// Graceful shutdown triggered
			return

		case <-ticker.C:
			// Check if connection is already closed
			if conn.IsClosed() {
				return
			}

			// Check for heartbeat timeout
			if time.Since(conn.GetLastHeartbeat()) > m.heartbeatTimeout {
				m.slogger.Warn("Heartbeat timeout detected", "gatewayID", conn.GatewayID,
					"connectionID", conn.ConnectionID, "lastHeartbeat", conn.GetLastHeartbeat())
				m.Unregister(conn.GatewayID, conn.ConnectionID)
				return
			}

			// Send ping frame
			if err := conn.Transport.SendPing(); err != nil {
				m.slogger.Error("Failed to send ping", "gatewayID", conn.GatewayID,
					"connectionID", conn.ConnectionID, "error", err)
				m.Unregister(conn.GatewayID, conn.ConnectionID)
				return
			}
		}
	}
}

// Shutdown gracefully closes all connections and stops heartbeat monitoring.
// Waits for all connection handler goroutines to exit before returning.
//
// This method should be called during server shutdown to cleanly terminate
// all gateway connections with a normal closure code.
func (m *Manager) Shutdown() {
	m.slogger.Info("Shutting down WebSocket manager...")

	// Signal shutdown to all monitoring goroutines
	m.shutdownFn()

	// Close all connections
	m.connections.Range(func(key, value interface{}) bool {
		gatewayID := key.(string)
		conns := value.([]*Connection)
		for _, conn := range conns {
			if err := conn.Close(1000, "server shutdown"); err != nil {
				m.slogger.Error("Failed to close connection during shutdown", "gatewayID", gatewayID,
					"connectionID", conn.ConnectionID, "error", err)
			}
		}
		return true // Continue iteration
	})

	// Wait for all goroutines to exit
	m.wg.Wait()

	m.slogger.Info("WebSocket manager shutdown complete")
}

// GetOrgConnectionStats returns connection statistics for a specific organization
func (m *Manager) GetOrgConnectionStats(orgID string) OrgConnectionStats {
	return OrgConnectionStats{
		OrganizationID: orgID,
		CurrentCount:   m.countOrgConnections(orgID),
		MaxAllowed:     m.maxConnectionsPerOrg,
	}
}

// CanAcceptOrgConnection checks if the organization can accept a new connection
// without actually adding it. Use this for pre-upgrade validation.
func (m *Manager) CanAcceptOrgConnection(orgID string) bool {
	return m.countOrgConnections(orgID) < m.maxConnectionsPerOrg
}

type metricsPayload struct {
	From                  string `json:"from"`
	To                    string `json:"to"`
	TotalActiveConns      int    `json:"totalActiveConnections"`
	TotalActiveOrgs       int    `json:"totalActiveOrgs"`
	SuccessfulConnections int64  `json:"successfulConnections"`
	FailedConnections     int64  `json:"failedConnections"`
	Disconnections        int64  `json:"disconnections"`
	EventsSent            int64  `json:"eventsSent"`
}

func (m *Manager) IncrementSuccessfulConnections() {
	atomic.AddInt64(&m.successfulConnections, 1)
}

func (m *Manager) IncrementFailedConnections() {
	atomic.AddInt64(&m.failedConnections, 1)
}

func (m *Manager) IncrementDisconnections() {
	atomic.AddInt64(&m.disconnections, 1)
}

func (m *Manager) IncrementTotalEventsSent() {
	atomic.AddInt64(&m.eventsSent, 1)
}

func (m *Manager) countActiveOrgs() int {
	orgs := make(map[string]struct{})
	m.connections.Range(func(key, value interface{}) bool {
		conns := value.([]*Connection)
		for _, conn := range conns {
			orgs[conn.OrganizationID] = struct{}{}
		}
		return true
	})
	return len(orgs)
}

func (m *Manager) startMetricsLogger() {
	ticker := time.NewTicker(m.metricsLogInterval)
	defer ticker.Stop()

	from := time.Now()

	for {
		select {
		case <-m.shutdownCtx.Done():
			return
		case <-ticker.C:
			to := time.Now()

			payload := metricsPayload{
				From:                  from.Format(time.RFC3339),
				To:                    to.Format(time.RFC3339),
				TotalActiveConns:      m.GetConnectionCount(),
				TotalActiveOrgs:       m.countActiveOrgs(),
				SuccessfulConnections: atomic.SwapInt64(&m.successfulConnections, 0),
				FailedConnections:     atomic.SwapInt64(&m.failedConnections, 0),
				Disconnections:        atomic.SwapInt64(&m.disconnections, 0),
				EventsSent:            atomic.SwapInt64(&m.eventsSent, 0),
			}

			data, err := json.Marshal(payload)
			if err != nil {
				m.slogger.Error("Failed to marshal WS metrics", "error", err)
			} else {
				m.slogger.Debug("WS Metrics", "payload", string(data))
			}

			from = to
		}
	}
}
