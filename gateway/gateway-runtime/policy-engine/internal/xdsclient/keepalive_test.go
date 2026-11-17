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

package xdsclient

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
)

func TestValidate_KeepaliveTimeBelowServerMinTime(t *testing.T) {
	config := createValidTestConfig()
	config.KeepaliveTime = MinKeepaliveTime - time.Second

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keepalive time must be at least 5s")
}

func TestValidate_KeepaliveTimeAtServerMinTime(t *testing.T) {
	config := createValidTestConfig()
	config.KeepaliveTime = MinKeepaliveTime

	assert.NoError(t, config.Validate())
}

func TestValidate_KeepaliveTimeoutNotPositive(t *testing.T) {
	config := createValidTestConfig()
	config.KeepaliveTimeout = 0

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keepalive timeout must be positive")
}

func TestNewDefaultConfig_SetsKeepalive(t *testing.T) {
	config := NewDefaultConfig("localhost:18001")

	assert.Equal(t, DefaultKeepaliveTime, config.KeepaliveTime)
	assert.Equal(t, DefaultKeepaliveTimeout, config.KeepaliveTimeout)
	assert.NoError(t, config.Validate())
}

func TestKeepaliveParams(t *testing.T) {
	config := createValidTestConfig()
	config.KeepaliveTime = 45 * time.Second
	config.KeepaliveTimeout = 7 * time.Second

	assert.Equal(t, keepalive.ClientParameters{
		Time:                45 * time.Second,
		Timeout:             7 * time.Second,
		PermitWithoutStream: true,
	}, keepaliveParams(config))
}

// TestClient_ReconnectsWhenConnectionGoesSilent reproduces a connection that
// dies without a FIN or RST reaching the client (controller pod gone,
// conntrack entry dropped): a proxy between client and server stops
// forwarding but keeps both sockets open. The client must notice through its
// keepalive PING and open a new stream; without keepalive it would stay
// blocked in Recv on the first stream forever.
func TestClient_ReconnectsWhenConnectionGoesSilent(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for a keepalive timeout (~11s)")
	}

	server := newSilentADSServer(t)
	proxy := newFreezableProxy(t, server.addr)

	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = proxy.addr()
	config.InitialReconnectDelay = 100 * time.Millisecond
	config.MaxReconnectDelay = 200 * time.Millisecond
	// gRPC-Go raises a client keepalive Time below 10s to 10s.
	config.KeepaliveTime = 10 * time.Second
	config.KeepaliveTimeout = time.Second

	client, err := NewClient(config, k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(client.Stop)

	select {
	case <-server.streams:
	case <-time.After(10 * time.Second):
		t.Fatal("client never opened the first ADS stream")
	}

	// Positive control: while the connection is healthy, the PINGs are
	// answered and the client keeps its stream.
	select {
	case <-server.streams:
		t.Fatal("client reopened the stream although the connection was healthy")
	case <-time.After(12 * time.Second):
	}

	proxy.freeze()

	select {
	case <-server.streams:
	case <-time.After(20 * time.Second):
		t.Fatal("client did not reconnect after its connection went silent")
	}
}

// silentADSServer accepts ADS streams and never responds, the same as a
// controller with no config change to push.
type silentADSServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	addr    string
	streams chan struct{}
}

func newSilentADSServer(t *testing.T) *silentADSServer {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &silentADSServer{addr: lis.Addr().String(), streams: make(chan struct{}, 10)}
	// Mirrors gateway-controller's policy xDS server enforcement policy.
	grpcServer := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}))
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, s)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)
	return s
}

func (s *silentADSServer) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	s.streams <- struct{}{}
	for {
		if _, err := stream.Recv(); err != nil {
			return err
		}
	}
}

// freezableProxy forwards TCP between client and server. After freeze, the
// connections it already accepted stop forwarding in both directions but
// stay open; connections accepted later forward normally.
type freezableProxy struct {
	lis    net.Listener
	target string
	mu     sync.Mutex
	frozen chan struct{}
}

func newFreezableProxy(t *testing.T, target string) *freezableProxy {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &freezableProxy{lis: lis, target: target, frozen: make(chan struct{})}
	go p.serve()
	t.Cleanup(func() { _ = lis.Close() })
	return p
}

func (p *freezableProxy) addr() string { return p.lis.Addr().String() }

func (p *freezableProxy) freeze() {
	p.mu.Lock()
	defer p.mu.Unlock()
	close(p.frozen)
	p.frozen = make(chan struct{})
}

func (p *freezableProxy) serve() {
	for {
		down, err := p.lis.Accept()
		if err != nil {
			return
		}
		up, err := net.Dial("tcp", p.target)
		if err != nil {
			_ = down.Close()
			continue
		}
		p.mu.Lock()
		frozen := p.frozen
		p.mu.Unlock()
		go pipeUntilFrozen(up, down, frozen)
		go pipeUntilFrozen(down, up, frozen)
	}
}

// pipeUntilFrozen copies src to dst until frozen is closed, then stops
// reading without closing either side, so the peer sees a silent connection.
func pipeUntilFrozen(dst io.Writer, src io.Reader, frozen <-chan struct{}) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		select {
		case <-frozen:
			<-make(chan struct{}) // hold the connection open, forward nothing
		default:
		}
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
