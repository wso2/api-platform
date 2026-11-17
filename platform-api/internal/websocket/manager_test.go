/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type stubTransport struct{}

func (stubTransport) Send([]byte) error                    { return nil }
func (stubTransport) Close(int, string) error              { return nil }
func (stubTransport) SetReadDeadline(time.Time) error      { return nil }
func (stubTransport) SetWriteDeadline(time.Time) error     { return nil }
func (stubTransport) EnablePongHandler(func(string) error) {}
func (stubTransport) SendPing() error                      { return nil }

// syncBuffer lets the manager's logger and the test read the log concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newTestManager(t *testing.T, maxConns, maxPerGateway int) (*Manager, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	cfg := DefaultManagerConfig()
	cfg.MaxConnections = maxConns
	cfg.MaxConnectionsPerGateway = maxPerGateway
	cfg.MetricsLogEnabled = false
	// Keep the heartbeat goroutines idle for the life of the test.
	cfg.HeartbeatInterval = time.Hour
	cfg.HeartbeatTimeout = time.Hour
	m := NewManager(cfg, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(m.Shutdown)
	return m, logs
}

func register(t *testing.T, m *Manager, gatewayID string) *Connection {
	t.Helper()
	conn, err := m.Register(gatewayID, stubTransport{}, "token", "org-1")
	if err != nil {
		t.Fatalf("Register(%s) = %v, want success", gatewayID, err)
	}
	return conn
}

func limitWarnings(logs *syncBuffer) int {
	return strings.Count(logs.String(), "level=WARN msg=\"Gateway reached its per-gateway connection limit")
}

func TestDefaultManagerConfigLeavesPerGatewayConnectionsUnlimited(t *testing.T) {
	if got := DefaultManagerConfig().MaxConnectionsPerGateway; got != -1 {
		t.Fatalf("DefaultManagerConfig().MaxConnectionsPerGateway = %d, want -1 (unlimited)", got)
	}
}

func TestRegisterAllowsUnlimitedConnectionsPerGatewayWhenLimitIsMinusOne(t *testing.T) {
	m, logs := newTestManager(t, 1000, -1)
	for i := 0; i < 50; i++ {
		register(t, m, "gw-1")
	}
	if got := len(m.GetConnections("gw-1")); got != 50 {
		t.Fatalf("gateway connections = %d, want 50", got)
	}
	if n := limitWarnings(logs); n != 0 {
		t.Fatalf("limit warnings = %d, want 0 when the limit is unlimited", n)
	}
}

func TestRegisterRejectsConnectionsBeyondThePerGatewayLimit(t *testing.T) {
	m, _ := newTestManager(t, 1000, 2)
	register(t, m, "gw-1")
	register(t, m, "gw-1")

	if _, err := m.Register("gw-1", stubTransport{}, "token", "org-1"); !errors.Is(err, ErrGatewayConnectionLimitReached) {
		t.Fatalf("third Register(gw-1) = %v, want ErrGatewayConnectionLimitReached", err)
	}
	// Positive control: another gateway is unaffected by gw-1's full limit.
	register(t, m, "gw-2")

	if got := len(m.GetConnections("gw-1")); got != 2 {
		t.Fatalf("gw-1 connections = %d, want 2", got)
	}
	if got := m.GetConnectionCount(); got != 3 {
		t.Fatalf("total connections = %d, want 3 (a rejected connection must not take a global slot)", got)
	}
}

func TestRegisterAcceptsAgainOnceAGatewayConnectionCloses(t *testing.T) {
	m, _ := newTestManager(t, 1000, 1)
	first := register(t, m, "gw-1")
	if _, err := m.Register("gw-1", stubTransport{}, "token", "org-1"); !errors.Is(err, ErrGatewayConnectionLimitReached) {
		t.Fatalf("second Register(gw-1) = %v, want ErrGatewayConnectionLimitReached", err)
	}

	m.Unregister("gw-1", first.ConnectionID)
	register(t, m, "gw-1")
}

func TestPerGatewayLimitIsLoggedOncePerTimeItIsReached(t *testing.T) {
	m, logs := newTestManager(t, 1000, 1)
	first := register(t, m, "gw-1")

	for i := 0; i < 3; i++ {
		if _, err := m.Register("gw-1", stubTransport{}, "token", "org-1"); !errors.Is(err, ErrGatewayConnectionLimitReached) {
			t.Fatalf("Register(gw-1) attempt %d = %v, want ErrGatewayConnectionLimitReached", i, err)
		}
	}
	if n := limitWarnings(logs); n != 1 {
		t.Fatalf("limit warnings after 3 rejections = %d, want 1", n)
	}
	if !strings.Contains(logs.String(), "gatewayID=gw-1") || !strings.Contains(logs.String(), "limit=1") {
		t.Fatalf("limit warning should name the gateway and the limit; log:\n%s", logs.String())
	}

	// A freed slot re-arms the warning: reaching the limit again logs again.
	m.Unregister("gw-1", first.ConnectionID)
	register(t, m, "gw-1")
	if _, err := m.Register("gw-1", stubTransport{}, "token", "org-1"); !errors.Is(err, ErrGatewayConnectionLimitReached) {
		t.Fatalf("Register(gw-1) after refill = %v, want ErrGatewayConnectionLimitReached", err)
	}
	if n := limitWarnings(logs); n != 2 {
		t.Fatalf("limit warnings after the limit is reached a second time = %d, want 2", n)
	}
}

func TestConcurrentRegistrationsNeverExceedThePerGatewayLimit(t *testing.T) {
	const limit = 5
	m, _ := newTestManager(t, 1000, limit)

	var wg sync.WaitGroup
	var accepted atomic.Int64
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Register("gw-1", stubTransport{}, "token", "org-1"); err == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := accepted.Load(); got != limit {
		t.Fatalf("accepted registrations = %d, want %d", got, limit)
	}
	if got := len(m.GetConnections("gw-1")); got != limit {
		t.Fatalf("gateway connections = %d, want %d", got, limit)
	}
	if got := m.GetConnectionCount(); got != limit {
		t.Fatalf("total connections = %d, want %d", got, limit)
	}
}

// Without a per-gateway limit, concurrent registrations for one gateway used to
// drop each other's connections from the registry while still counting them.
func TestConcurrentRegistrationsKeepRegistryAndCountInStep(t *testing.T) {
	m, _ := newTestManager(t, 10000, -1)

	var wg sync.WaitGroup
	conns := make(chan *Connection, 500)
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c, err := m.Register("gw-1", stubTransport{}, "token", "org-1"); err == nil {
				conns <- c
			}
		}()
	}
	wg.Wait()
	close(conns)

	if got, count := len(m.GetConnections("gw-1")), m.GetConnectionCount(); got != 500 || count != 500 {
		t.Fatalf("registry holds %d connections, count is %d; want 500 and 500", got, count)
	}

	for c := range conns {
		wg.Add(1)
		go func(c *Connection) {
			defer wg.Done()
			m.Unregister("gw-1", c.ConnectionID)
		}(c)
	}
	wg.Wait()
	if got, count := len(m.GetConnections("gw-1")), m.GetConnectionCount(); got != 0 || count != 0 {
		t.Fatalf("after unregistering all, registry holds %d and count is %d; want 0 and 0", got, count)
	}
}

func TestRegisterStillEnforcesTheGlobalLimit(t *testing.T) {
	m, _ := newTestManager(t, 2, -1)
	register(t, m, "gw-1")
	register(t, m, "gw-2")
	if _, err := m.Register("gw-3", stubTransport{}, "token", "org-1"); !errors.Is(err, ErrMaxConnectionsReached) {
		t.Fatalf("Register(gw-3) = %v, want ErrMaxConnectionsReached", err)
	}
}

// TestCloseConnectionsForTokenClosesOnlyThatTokensSessions: revoking one token
// ends the sessions it opened and leaves the gateway's other-token sessions open.
func TestCloseConnectionsForTokenClosesOnlyThatTokensSessions(t *testing.T) {
	m, _ := newTestManager(t, 10, -1)
	revoked1, err := m.Register("gw-1", stubTransport{}, "token-a", "org-1")
	if err != nil {
		t.Fatalf("Register = %v", err)
	}
	revoked2, err := m.Register("gw-1", stubTransport{}, "token-a", "org-1")
	if err != nil {
		t.Fatalf("Register = %v", err)
	}
	kept, err := m.Register("gw-1", stubTransport{}, "token-b", "org-1")
	if err != nil {
		t.Fatalf("Register = %v", err)
	}
	other, err := m.Register("gw-2", stubTransport{}, "token-a", "org-1")
	if err != nil {
		t.Fatalf("Register = %v", err)
	}

	if got := m.CloseConnectionsForToken("gw-1", "token-a"); got != 2 {
		t.Fatalf("CloseConnectionsForToken = %d, want 2", got)
	}
	for _, c := range []*Connection{revoked1, revoked2} {
		if !c.IsClosed() {
			t.Errorf("connection %s opened with the revoked token is still open", c.ConnectionID)
		}
		if err := c.Send([]byte("event")); !errors.Is(err, ErrConnectionClosed) {
			t.Errorf("Send on revoked session = %v, want ErrConnectionClosed", err)
		}
	}
	if kept.IsClosed() || other.IsClosed() {
		t.Fatal("a session not opened with the revoked token of that gateway was closed")
	}
	if conns := m.GetConnections("gw-1"); len(conns) != 1 || conns[0] != kept {
		t.Fatalf("gw-1 connections = %v, want only the token-b session", conns)
	}
	if got := m.CloseConnectionsForToken("gw-1", "token-a"); got != 0 {
		t.Fatalf("second CloseConnectionsForToken = %d, want 0", got)
	}
}
