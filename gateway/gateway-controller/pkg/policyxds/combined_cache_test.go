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
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCache is a mock implementation of cache.Cache for testing
type mockCache struct {
	createWatchCalled      bool
	createDeltaWatchCalled bool
	fetchCalled            bool
	fetchResponse          cache.Response
	fetchError             error
	watchCancelFunc        func()
	deltaWatchCancelFunc   func()
	responseChan           chan cache.Response
	supportsDeltaWatch     bool
}

func newMockCache() *mockCache {
	return &mockCache{
		watchCancelFunc:      func() {},
		deltaWatchCancelFunc: func() {},
		responseChan:         make(chan cache.Response, 1),
		supportsDeltaWatch:   false,
	}
}

func (m *mockCache) CreateWatch(request *cache.Request, streamState stream.StreamState, responseChan chan cache.Response) func() {
	m.createWatchCalled = true
	// Send response if available
	if m.responseChan != nil {
		go func() {
			select {
			case resp := <-m.responseChan:
				responseChan <- resp
			case <-time.After(10 * time.Millisecond):
			}
		}()
	}
	return m.watchCancelFunc
}

func (m *mockCache) CreateDeltaWatch(request *cache.DeltaRequest, streamState stream.StreamState, responseChan chan cache.DeltaResponse) func() {
	m.createDeltaWatchCalled = true
	return m.deltaWatchCancelFunc
}

func (m *mockCache) Fetch(ctx context.Context, request *cache.Request) (cache.Response, error) {
	m.fetchCalled = true
	return m.fetchResponse, m.fetchError
}

// mockDeltaCache is a mock that supports delta watch
type mockDeltaCache struct {
	*mockCache
}

func newMockDeltaCache() *mockDeltaCache {
	return &mockDeltaCache{
		mockCache: newMockCache(),
	}
}

func (m *mockDeltaCache) CreateDeltaWatch(request *cache.DeltaRequest, streamState stream.StreamState, responseChan chan cache.DeltaResponse) func() {
	m.createDeltaWatchCalled = true
	return m.deltaWatchCancelFunc
}

// mockResponse implements cache.Response for testing
type mockResponse struct {
	version string
	request *cache.Request
	err     error
}

func (m *mockResponse) GetDiscoveryResponse() (*discoveryv3.DiscoveryResponse, error) {
	return &discoveryv3.DiscoveryResponse{
		VersionInfo: m.version,
		TypeUrl:     "test-type",
	}, m.err
}

func (m *mockResponse) GetRequest() *cache.Request {
	return m.request
}

func (m *mockResponse) GetVersion() (string, error) {
	return m.version, m.err
}

func (m *mockResponse) GetContext() context.Context {
	return context.Background()
}

func TestNewCombinedCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	policyCache := newMockCache()
	apiKeyCache := newMockCache()
	lazyResourceCache := newMockCache()

	t.Run("creates combined cache successfully", func(t *testing.T) {
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)
		assert.NotNil(t, cc)
	})

	t.Run("panics with nil policy cache", func(t *testing.T) {
		assert.Panics(t, func() {
			NewCombinedCache(nil, apiKeyCache, lazyResourceCache, logger)
		})
	})

	t.Run("panics with nil api key cache", func(t *testing.T) {
		assert.Panics(t, func() {
			NewCombinedCache(policyCache, nil, lazyResourceCache, logger)
		})
	})

	t.Run("panics with nil lazy resource cache", func(t *testing.T) {
		assert.Panics(t, func() {
			NewCombinedCache(policyCache, apiKeyCache, nil, logger)
		})
	})

	t.Run("uses default logger when nil", func(t *testing.T) {
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, nil)
		assert.NotNil(t, cc)
	})
}

func TestCombinedCache_CreateWatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("creates watch on all caches", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}
		responseChan := make(chan cache.Response, 1)

		cancel := cc.CreateWatch(request, stream.NewStreamState(false, nil), responseChan)
		assert.NotNil(t, cancel)

		// Wait for watches to be created
		time.Sleep(50 * time.Millisecond)

		assert.True(t, policyCache.createWatchCalled)
		assert.True(t, apiKeyCache.createWatchCalled)
		assert.True(t, lazyResourceCache.createWatchCalled)

		// Call cancel
		cancel()
	})

	t.Run("returns cancel function that works", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		policyCancelCalled := false
		apiKeyCancelCalled := false
		lazyResourceCancelCalled := false

		policyCache.watchCancelFunc = func() { policyCancelCalled = true }
		apiKeyCache.watchCancelFunc = func() { apiKeyCancelCalled = true }
		lazyResourceCache.watchCancelFunc = func() { lazyResourceCancelCalled = true }

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}
		responseChan := make(chan cache.Response, 1)

		cancel := cc.CreateWatch(request, stream.NewStreamState(false, nil), responseChan)
		cancel()

		// Wait for cancel to propagate
		time.Sleep(50 * time.Millisecond)

		assert.True(t, policyCancelCalled)
		assert.True(t, apiKeyCancelCalled)
		assert.True(t, lazyResourceCancelCalled)
	})
}

func TestCombinedCache_CreateDeltaWatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("creates delta watch when caches support it", func(t *testing.T) {
		policyCache := newMockDeltaCache()
		apiKeyCache := newMockDeltaCache()
		lazyResourceCache := newMockDeltaCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.DeltaRequest{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}
		responseChan := make(chan cache.DeltaResponse, 1)

		cancel := cc.CreateDeltaWatch(request, stream.NewStreamState(false, nil), responseChan)
		assert.NotNil(t, cancel)

		assert.True(t, policyCache.createDeltaWatchCalled)
		assert.True(t, apiKeyCache.createDeltaWatchCalled)
		assert.True(t, lazyResourceCache.createDeltaWatchCalled)

		cancel()
	})

	t.Run("handles caches that don't support delta watch", func(t *testing.T) {
		// Create a mock cache that implements cache.Cache but not the specific
		// CreateDeltaWatch interface used in type assertion
		policyCache := &struct {
			cache.Cache
		}{newMockCache()}

		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.DeltaRequest{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}
		responseChan := make(chan cache.DeltaResponse, 1)

		cancel := cc.CreateDeltaWatch(request, stream.NewStreamState(false, nil), responseChan)
		assert.NotNil(t, cancel)

		// Should not panic - this tests the fallback path when type assertion fails
		cancel()
	})

	t.Run("cancel function calls underlying cancels", func(t *testing.T) {
		policyCache := newMockDeltaCache()
		apiKeyCache := newMockDeltaCache()
		lazyResourceCache := newMockDeltaCache()

		policyCancelCalled := false
		apiKeyCancelCalled := false
		lazyResourceCancelCalled := false

		policyCache.deltaWatchCancelFunc = func() { policyCancelCalled = true }
		apiKeyCache.deltaWatchCancelFunc = func() { apiKeyCancelCalled = true }
		lazyResourceCache.deltaWatchCancelFunc = func() { lazyResourceCancelCalled = true }

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.DeltaRequest{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}
		responseChan := make(chan cache.DeltaResponse, 1)

		cancel := cc.CreateDeltaWatch(request, stream.NewStreamState(false, nil), responseChan)
		cancel()

		assert.True(t, policyCancelCalled)
		assert.True(t, apiKeyCancelCalled)
		assert.True(t, lazyResourceCancelCalled)
	})
}

func TestCombinedCache_Fetch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("fetches from policy cache first", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		expectedResponse := &mockResponse{version: "policy-v1"}
		policyCache.fetchResponse = expectedResponse
		policyCache.fetchError = nil

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.True(t, policyCache.fetchCalled)
	})

	t.Run("falls back to api key cache if policy cache fails", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		policyCache.fetchError = assert.AnError
		expectedResponse := &mockResponse{version: "apikey-v1"}
		apiKeyCache.fetchResponse = expectedResponse
		apiKeyCache.fetchError = nil

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.True(t, policyCache.fetchCalled)
		assert.True(t, apiKeyCache.fetchCalled)
	})

	t.Run("falls back to lazy resource cache if api key cache fails", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		policyCache.fetchError = assert.AnError
		apiKeyCache.fetchError = assert.AnError
		expectedResponse := &mockResponse{version: "lazy-v1"}
		lazyResourceCache.fetchResponse = expectedResponse
		lazyResourceCache.fetchError = nil

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.True(t, policyCache.fetchCalled)
		assert.True(t, apiKeyCache.fetchCalled)
		assert.True(t, lazyResourceCache.fetchCalled)
	})

	t.Run("returns empty response if all caches fail", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		policyCache.fetchError = assert.AnError
		apiKeyCache.fetchError = assert.AnError
		lazyResourceCache.fetchError = assert.AnError

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)

		version, _ := response.GetVersion()
		assert.Equal(t, "0", version)
	})
}

func TestCombinedCache_CancelWatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("canceling non-existent watch does not panic", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		// Should not panic
		cc.cancelWatch(99999)
	})

	t.Run("canceling removes watcher from map", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}
		responseChan := make(chan cache.Response, 1)

		cancel := cc.CreateWatch(request, stream.NewStreamState(false, nil), responseChan)

		// Wait a bit for the watch to be created
		time.Sleep(20 * time.Millisecond)

		// Cancel should remove the watcher
		cancel()

		// Wait a bit for cancel to propagate
		time.Sleep(20 * time.Millisecond)

		// Accessing internal state for verification
		combinedCache := cc.(*CombinedCache)
		combinedCache.mu.RLock()
		watcherCount := len(combinedCache.watchers)
		combinedCache.mu.RUnlock()

		assert.Equal(t, 0, watcherCount)
	})
}

func TestCombinedCache_HandleCombinedResponses(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("forwards policy response to main channel", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send a policy response
		policyResponseChan <- &mockResponse{version: "v1"}

		// Wait for response
		select {
		case resp := <-mainResponseChan:
			version, _ := resp.GetVersion()
			assert.Equal(t, "v1", version)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for response")
		}

		close(done)
	})

	t.Run("skips duplicate policy responses", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 2)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 2)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send same response twice
		policyResponseChan <- &mockResponse{version: "v1"}
		policyResponseChan <- &mockResponse{version: "v1"}

		// Wait for first response
		select {
		case resp := <-mainResponseChan:
			version, _ := resp.GetVersion()
			assert.Equal(t, "v1", version)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for first response")
		}

		// Second response should be skipped
		select {
		case <-mainResponseChan:
			t.Fatal("should not receive duplicate response")
		case <-time.After(150 * time.Millisecond):
			// Expected - no duplicate
		}

		close(done)
	})

	t.Run("handles nil response gracefully", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 2)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send nil response followed by real response
		policyResponseChan <- nil
		policyResponseChan <- &mockResponse{version: "v1"}

		// Should receive only the real response
		select {
		case resp := <-mainResponseChan:
			version, _ := resp.GetVersion()
			assert.Equal(t, "v1", version)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for response")
		}

		close(done)
	})

	t.Run("exits when done channel closed", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		exited := make(chan struct{})
		go func() {
			cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)
			close(exited)
		}()

		// Close done channel
		close(done)

		// Should exit
		select {
		case <-exited:
			// Good
		case <-time.After(200 * time.Millisecond):
			t.Fatal("handler did not exit")
		}
	})

	t.Run("exits when policy channel closed", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		exited := make(chan struct{})
		go func() {
			cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)
			close(exited)
		}()

		// Close policy channel
		close(policyResponseChan)

		// Should exit
		select {
		case <-exited:
			// Good
		case <-time.After(200 * time.Millisecond):
			t.Fatal("handler did not exit")
		}
	})
}

func TestCombinedCache_CreateDeltaResponseHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("forwards delta responses", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		mainResponseChan := make(chan cache.DeltaResponse, 1)
		responseChan := cc.createDeltaResponseHandler(1, "test", mainResponseChan)

		// Send a delta response
		responseChan <- &cache.RawDeltaResponse{}

		// Should receive it in main channel
		select {
		case <-mainResponseChan:
			// Good
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for response")
		}

		close(responseChan)
	})

	t.Run("warns on timeout sending delta response", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		// Create a channel with no buffer to block
		mainResponseChan := make(chan cache.DeltaResponse)
		responseChan := cc.createDeltaResponseHandler(1, "test", mainResponseChan)

		// Send a delta response (this should trigger timeout warning)
		responseChan <- &cache.RawDeltaResponse{}

		// Give it time to timeout
		time.Sleep(150 * time.Millisecond)

		close(responseChan)
	})

	t.Run("handles policy response timeout", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		// Create unbuffered channel to trigger timeout
		mainResponseChan := make(chan cache.Response)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send a policy response (should timeout since mainResponseChan is unbuffered and no one reading)
		policyResponseChan <- &mockResponse{version: "v1"}

		// Give time for timeout
		time.Sleep(150 * time.Millisecond)

		close(done)
	})

	t.Run("handles api key response timeout", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		// Create unbuffered channel to trigger timeout
		mainResponseChan := make(chan cache.Response)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send an API key response (should timeout since mainResponseChan is unbuffered)
		apiKeyResponseChan <- &mockResponse{version: "v1"}

		// Give time for timeout
		time.Sleep(150 * time.Millisecond)

		close(done)
	})

	t.Run("handles lazy resource response timeout", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		// Create unbuffered channel to trigger timeout
		mainResponseChan := make(chan cache.Response)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send a lazy resource response (should timeout since mainResponseChan is unbuffered)
		lazyResourceResponseChan <- &mockResponse{version: "v1"}

		// Give time for timeout
		time.Sleep(150 * time.Millisecond)

		close(done)
	})

	t.Run("exits when api key channel closed", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		exited := make(chan struct{})
		go func() {
			cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)
			close(exited)
		}()

		// Close API key channel
		close(apiKeyResponseChan)

		// Should exit
		select {
		case <-exited:
			// Good
		case <-time.After(200 * time.Millisecond):
			t.Fatal("handler did not exit")
		}
	})

	t.Run("skips nil api key response", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 2)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		go cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)

		// Send nil response followed by real response
		apiKeyResponseChan <- nil
		apiKeyResponseChan <- &mockResponse{version: "v1"}

		// Should receive only the real response
		select {
		case resp := <-mainResponseChan:
			version, _ := resp.GetVersion()
			assert.Equal(t, "v1", version)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for response")
		}

		close(done)
	})

	t.Run("exits when lazy resource channel closed", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()
		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger).(*CombinedCache)

		policyResponseChan := make(chan cache.Response, 1)
		apiKeyResponseChan := make(chan cache.Response, 1)
		lazyResourceResponseChan := make(chan cache.Response, 1)
		mainResponseChan := make(chan cache.Response, 1)
		done := make(chan struct{})

		exited := make(chan struct{})
		go func() {
			cc.handleCombinedResponses(1, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, mainResponseChan, done)
			close(exited)
		}()

		// Close lazy resource channel
		close(lazyResourceResponseChan)

		// Should exit
		select {
		case <-exited:
			// Good
		case <-time.After(200 * time.Millisecond):
			t.Fatal("handler did not exit")
		}
	})
}

func TestCombinedCache_Fetch_VersionErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("handles version error from policy cache", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		// Return response but with version error
		policyCache.fetchResponse = &mockResponseWithVersionError{}
		policyCache.fetchError = nil

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handles version error from api key cache", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		policyCache.fetchError = assert.AnError
		apiKeyCache.fetchResponse = &mockResponseWithVersionError{}
		apiKeyCache.fetchError = nil

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handles version error from lazy resource cache", func(t *testing.T) {
		policyCache := newMockCache()
		apiKeyCache := newMockCache()
		lazyResourceCache := newMockCache()

		policyCache.fetchError = assert.AnError
		apiKeyCache.fetchError = assert.AnError
		lazyResourceCache.fetchResponse = &mockResponseWithVersionError{}
		lazyResourceCache.fetchError = nil

		cc := NewCombinedCache(policyCache, apiKeyCache, lazyResourceCache, logger)

		request := &cache.Request{
			TypeUrl: "test-type",
			Node:    &core.Node{Id: "test-node"},
		}

		response, err := cc.Fetch(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})
}

// mockResponseWithVersionError implements cache.Response but returns error on GetVersion
type mockResponseWithVersionError struct{}

func (m *mockResponseWithVersionError) GetVersion() (string, error) {
	return "", assert.AnError
}

func (m *mockResponseWithVersionError) GetRequest() *cache.Request {
	return &cache.Request{}
}

func (m *mockResponseWithVersionError) GetContext() context.Context {
	return context.Background()
}

func (m *mockResponseWithVersionError) GetDiscoveryResponse() (*discoveryv3.DiscoveryResponse, error) {
	return &discoveryv3.DiscoveryResponse{}, nil
}
