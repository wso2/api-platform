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
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
)

// silentADSServer accepts configuration streams and holds them open without sending
// anything, like a controller with nothing to push. It counts streams that end.
type silentADSServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	closed atomic.Int32
}

func (s *silentADSServer) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	defer s.closed.Add(1)
	for {
		if _, err := stream.Recv(); err != nil {
			return nil
		}
	}
}

// freezableProxy forwards TCP connections to a backend and can be frozen: while frozen
// it keeps every socket open but forwards no bytes in either direction, which is what
// a frozen controller looks like from the client's side.
type freezableProxy struct {
	ln      net.Listener
	backend string

	mu       sync.Mutex
	frozen   bool
	unfrozen chan struct{}
	conns    []net.Conn
}

func newFreezableProxy(t *testing.T, backend string) *freezableProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &freezableProxy{ln: ln, backend: backend, unfrozen: make(chan struct{})}
	close(p.unfrozen)
	go p.accept()
	t.Cleanup(p.close)
	return p
}

func (p *freezableProxy) addr() string { return p.ln.Addr().String() }

func (p *freezableProxy) accept() {
	for {
		client, err := p.ln.Accept()
		if err != nil {
			return
		}
		server, err := net.Dial("tcp", p.backend)
		if err != nil {
			_ = client.Close()
			continue
		}
		p.mu.Lock()
		p.conns = append(p.conns, client, server)
		p.mu.Unlock()
		go p.pipe(client, server)
		go p.pipe(server, client)
	}
}

// pipe copies src to dst, holding each chunk while the proxy is frozen.
func (p *freezableProxy) pipe(src, dst net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			p.mu.Lock()
			wait := p.unfrozen
			p.mu.Unlock()
			<-wait
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			_ = dst.Close()
			return
		}
	}
}

func (p *freezableProxy) freeze() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.frozen {
		p.frozen = true
		p.unfrozen = make(chan struct{})
	}
}

func (p *freezableProxy) unfreeze() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.frozen {
		p.frozen = false
		close(p.unfrozen)
	}
}

func (p *freezableProxy) close() {
	p.unfreeze()
	_ = p.ln.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
}

// TestKeepalive_DetectsFrozenControllerAndKeepsIdleConnections runs a real client
// against a real gRPC server through a proxy that can freeze. It takes about 20s:
// grpc-go never pings more often than every 10s, so the idle phase and the detection
// phase each have to outlast one keepalive interval.
func TestKeepalive_DetectsFrozenControllerAndKeepsIdleConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for real keepalive intervals")
	}
	recordConnectionMetrics(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ads := &silentADSServer{}
	// The same keepalive enforcement as the controller's policy xDS server.
	server := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}))
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(server, ads)
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(server.Stop)
	proxy := newFreezableProxy(t, ln.Addr().String())

	cfg := createValidTestConfig()
	cfg.ServerAddress = proxy.addr()
	cfg.ConnectTimeout = 2 * time.Second
	cfg.InitialReconnectDelay = 100 * time.Millisecond
	cfg.MaxReconnectDelay = 200 * time.Millisecond
	cfg.KeepaliveTime = DefaultKeepaliveTime
	cfg.KeepaliveTimeout = time.Second
	k, reg := createTestKernelAndRegistry(t)
	client, err := NewClient(cfg, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(client.Stop)

	require.Eventually(t, func() bool { return client.ConnectionStatus().Connected },
		5*time.Second, 50*time.Millisecond, "client must connect")

	// Idle but healthy: across a full keepalive interval the connection stays up and
	// the server never closes it for pinging too often.
	idleUntil := time.Now().Add(DefaultKeepaliveTime + 2*time.Second)
	for time.Now().Before(idleUntil) {
		require.True(t, client.ConnectionStatus().Connected, "a healthy idle connection must stay up")
		time.Sleep(200 * time.Millisecond)
	}
	assert.Equal(t, int32(0), ads.closed.Load(), "the server must not close a healthy connection")

	// Frozen: no bytes flow, so keepalive pings go unanswered.
	proxy.freeze()
	frozenAt := time.Now()
	require.Eventually(t, func() bool { return !client.ConnectionStatus().Connected },
		cfg.KeepaliveTime+cfg.KeepaliveTimeout+5*time.Second, 100*time.Millisecond,
		"a frozen controller must be detected")
	status := client.ConnectionStatus()
	assert.False(t, status.DisconnectedSince.Before(frozenAt), "the loss is recorded after the freeze")
	t.Logf("frozen controller detected after %s", time.Since(frozenAt).Round(time.Millisecond))

	// Unfrozen: the client reconnects and the outage is cleared.
	proxy.unfreeze()
	require.Eventually(t, func() bool { return client.ConnectionStatus().Connected },
		10*time.Second, 50*time.Millisecond, "the client must reconnect")
	assert.True(t, client.ConnectionStatus().DisconnectedSince.IsZero())
}
