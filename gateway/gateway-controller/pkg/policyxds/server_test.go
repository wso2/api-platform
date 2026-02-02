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

package policyxds

import (
	"context"
	"io"
	"log/slog"
	"testing"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestServerCallbacks_OnStreamOpen(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	err := cb.OnStreamOpen(context.Background(), 123, "test-type-url")
	assert.NoError(t, err)
}

func TestServerCallbacks_OnStreamClosed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	node := &core.Node{Id: "test-node-id"}

	// Should not panic
	cb.OnStreamClosed(123, node)
}

func TestServerCallbacks_OnStreamRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	req := &discoverygrpc.DiscoveryRequest{
		TypeUrl:       "test-type-url",
		VersionInfo:   "v1",
		ResourceNames: []string{"resource1", "resource2"},
		Node:          &core.Node{Id: "test-node"},
	}

	err := cb.OnStreamRequest(123, req)
	assert.NoError(t, err)
}

func TestServerCallbacks_OnStreamResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	req := &discoverygrpc.DiscoveryRequest{
		TypeUrl: "test-type-url",
		Node:    &core.Node{Id: "test-node"},
	}

	resp := &discoverygrpc.DiscoveryResponse{
		TypeUrl:     "test-type-url",
		VersionInfo: "v1",
		Resources:   []*anypb.Any{{TypeUrl: "test"}},
	}

	// Should not panic
	cb.OnStreamResponse(context.Background(), 123, req, resp)
}

func TestServerCallbacks_OnFetchRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	req := &discoverygrpc.DiscoveryRequest{
		TypeUrl:       "test-type-url",
		ResourceNames: []string{"resource1"},
		Node:          &core.Node{Id: "test-node"},
	}

	err := cb.OnFetchRequest(context.Background(), req)
	assert.NoError(t, err)
}

func TestServerCallbacks_OnFetchResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	req := &discoverygrpc.DiscoveryRequest{
		TypeUrl: "test-type-url",
		Node:    &core.Node{Id: "test-node"},
	}

	resp := &discoverygrpc.DiscoveryResponse{
		TypeUrl:     "test-type-url",
		VersionInfo: "v1",
		Resources:   []*anypb.Any{{TypeUrl: "test1"}, {TypeUrl: "test2"}},
	}

	// Should not panic
	cb.OnFetchResponse(req, resp)
}

func TestServerCallbacks_OnDeltaStreamOpen(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	err := cb.OnDeltaStreamOpen(context.Background(), 456, "delta-type-url")
	assert.NoError(t, err)
}

func TestServerCallbacks_OnDeltaStreamClosed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	node := &core.Node{Id: "delta-test-node"}

	// Should not panic
	cb.OnDeltaStreamClosed(456, node)
}

func TestServerCallbacks_OnStreamDeltaRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	req := &discoverygrpc.DeltaDiscoveryRequest{
		TypeUrl: "delta-type-url",
		Node:    &core.Node{Id: "delta-test-node"},
	}

	err := cb.OnStreamDeltaRequest(789, req)
	assert.NoError(t, err)
}

func TestServerCallbacks_OnStreamDeltaResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cb := &serverCallbacks{logger: logger}

	req := &discoverygrpc.DeltaDiscoveryRequest{
		TypeUrl: "delta-type-url",
		Node:    &core.Node{Id: "delta-test-node"},
	}

	resp := &discoverygrpc.DeltaDiscoveryResponse{
		TypeUrl:   "delta-type-url",
		Resources: []*discoverygrpc.Resource{{Name: "resource1"}, {Name: "resource2"}},
	}

	// Should not panic
	cb.OnStreamDeltaResponse(789, req, resp)
}

func TestWithTLS(t *testing.T) {
	t.Run("enables TLS configuration", func(t *testing.T) {
		s := &Server{}
		opt := WithTLS("/path/to/cert.pem", "/path/to/key.pem")
		opt(s)

		assert.NotNil(t, s.tlsConfig)
		assert.True(t, s.tlsConfig.Enabled)
		assert.Equal(t, "/path/to/cert.pem", s.tlsConfig.CertFile)
		assert.Equal(t, "/path/to/key.pem", s.tlsConfig.KeyFile)
	})
}

func TestTLSConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		config := &TLSConfig{}
		assert.False(t, config.Enabled)
		assert.Empty(t, config.CertFile)
		assert.Empty(t, config.KeyFile)
	})

	t.Run("with values", func(t *testing.T) {
		config := &TLSConfig{
			Enabled:  true,
			CertFile: "cert.pem",
			KeyFile:  "key.pem",
		}
		assert.True(t, config.Enabled)
		assert.Equal(t, "cert.pem", config.CertFile)
		assert.Equal(t, "key.pem", config.KeyFile)
	})
}
