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
	"log/slog"
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
)

// CombinedCache combines policy and API key caches to provide a unified xDS cache interface
// It implements cache.Cache interface by delegating to underlying caches
type CombinedCache struct {
	policyCache cache.Cache
	apiKeyCache cache.Cache
	logger      *slog.Logger
	mu          sync.RWMutex
	watchers    map[int64]*combinedWatcher
	watcherID   int64
}

// combinedWatcher manages watchers for both policy and API key caches
type combinedWatcher struct {
	id            int64
	request       *cache.Request
	streamState   stream.StreamState
	responseChan  chan cache.Response
	policyCancel  func()
	apiKeyCancel  func()
	combinedCache *CombinedCache
	done          chan struct{} // done channel to signal goroutine cancellation
}

// NewCombinedCache creates a new combined cache that merges policy and API key caches
// Returns a cache.Cache interface implementation
func NewCombinedCache(policyCache cache.Cache, apiKeyCache cache.Cache, logger *slog.Logger) cache.Cache {
	if policyCache == nil || apiKeyCache == nil {
		panic("policyCache and apiKeyCache must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CombinedCache{
		policyCache: policyCache,
		apiKeyCache: apiKeyCache,
		logger:      logger,
		watchers:    make(map[int64]*combinedWatcher),
		watcherID:   0,
	}
}

// CreateWatch creates a watch for resources in both policy and API key caches
// Implements cache.ConfigWatcher interface
func (c *CombinedCache) CreateWatch(request *cache.Request, streamState stream.StreamState, responseChan chan cache.Response) func() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.watcherID++
	watcherID := c.watcherID

	watcher := &combinedWatcher{
		id:            watcherID,
		request:       request,
		streamState:   streamState,
		responseChan:  responseChan,
		combinedCache: c,
		done:          make(chan struct{}),
	}

	c.watchers[watcherID] = watcher

	c.logger.Debug("Creating combined watch",
		slog.Int64("watcher_id", watcherID),
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()))

	// Create separate response channels for each cache to avoid recursion
	policyResponseChan := make(chan cache.Response, 1)
	apiKeyResponseChan := make(chan cache.Response, 1)

	// Create watches on both underlying caches with separate channels
	watcher.policyCancel = c.policyCache.CreateWatch(request, streamState, policyResponseChan)
	watcher.apiKeyCancel = c.apiKeyCache.CreateWatch(request, streamState, apiKeyResponseChan)

	// Start a response multiplexer to handle responses from both caches
	go c.handleCombinedResponses(watcherID, policyResponseChan, apiKeyResponseChan, responseChan, watcher.done)

	// Return cancel function
	return func() {
		c.cancelWatch(watcherID)
	}
}

// handleCombinedResponses multiplexes responses from both caches
// This prevents recursion and handles response deduplication
func (c *CombinedCache) handleCombinedResponses(watcherID int64, policyResponseChan, apiKeyResponseChan chan cache.Response,
	mainResponseChan chan cache.Response, done chan struct{}) {
	defer func() {
		c.logger.Debug("Response handler goroutine exiting", slog.Int64("watcher_id", watcherID))
	}()

	var lastPolicyVersion, lastApiKeyVersion string

	for {
		select {
		case <-done:
			c.logger.Debug("Watcher cancelled, exiting response handler", slog.Int64("watcher_id", watcherID))
			return

		case response, ok := <-policyResponseChan:
			if !ok {
				c.logger.Debug("Policy response channel closed", slog.Int64("watcher_id", watcherID))
				return
			}

			// Handle nil response only if we haven't sent initial response yet
			if response == nil {
				// Don't create continuous empty responses - this causes the loop
				c.logger.Debug("Policy cache has no data, skipping nil response",
					slog.Int64("watcher_id", watcherID))
				continue
			}

			// Check if this is a duplicate response
			version, err := response.GetVersion()
			if err != nil {
				version = "unknown"
			}

			if version != lastPolicyVersion {
				lastPolicyVersion = version
				c.logger.Debug("Forwarding policy cache response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))

				select {
				case mainResponseChan <- response:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					c.logger.Warn("Timeout sending policy response, client may be slow",
						slog.Int64("watcher_id", watcherID),
						slog.String("version", version))
				}
			} else {
				c.logger.Debug("Skipping duplicate policy response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))
			}

		case response, ok := <-apiKeyResponseChan:
			if !ok {
				c.logger.Debug("API key response channel closed", slog.Int64("watcher_id", watcherID))
				return
			}

			// Handle nil response only if we haven't sent initial response yet
			if response == nil {
				// Don't create continuous empty responses - this causes the loop
				c.logger.Debug("API key cache has no data, skipping nil response",
					slog.Int64("watcher_id", watcherID))
				continue
			}

			// Check if this is a duplicate response
			version, err := response.GetVersion()
			if err != nil {
				version = "unknown"
			}

			if version != lastApiKeyVersion {
				lastApiKeyVersion = version
				c.logger.Debug("Forwarding API key cache response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))

				select {
				case mainResponseChan <- response:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					c.logger.Warn("Timeout sending API key response, client may be slow",
						slog.Int64("watcher_id", watcherID),
						slog.String("version", version))
				}
			} else {
				c.logger.Debug("Skipping duplicate API key response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))
			}
		}

		// Check if watcher still exists
		c.mu.RLock()
		_, exists := c.watchers[watcherID]
		c.mu.RUnlock()

		if !exists {
			c.logger.Debug("Watcher removed, exiting response handler", slog.Int64("watcher_id", watcherID))
			return
		}
	}
}

// CreateDeltaWatch creates a delta watch for incremental xDS updates
// Implements cache.ConfigWatcher interface
func (c *CombinedCache) CreateDeltaWatch(request *cache.DeltaRequest, streamState stream.StreamState, responseChan chan cache.DeltaResponse) func() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.watcherID++
	watcherID := c.watcherID

	c.logger.Debug("Creating combined delta watch",
		slog.Int64("watcher_id", watcherID),
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()))

	// Create delta watches on both underlying caches
	var policyCancel, apiKeyCancel func()

	// Try to create delta watch on policy cache if it supports it
	if deltaWatcher, ok := c.policyCache.(interface {
		CreateDeltaWatch(*cache.DeltaRequest, stream.StreamState, chan cache.DeltaResponse) func()
	}); ok {
		policyCancel = deltaWatcher.CreateDeltaWatch(request, streamState, c.createDeltaResponseHandler(watcherID, "policy", responseChan))
		c.logger.Debug("Policy cache supports delta watch", slog.Int64("watcher_id", watcherID))
	} else {
		c.logger.Debug("Policy cache does not support delta watch, skipping", slog.Int64("watcher_id", watcherID))
	}

	// Try to create delta watch on API key cache if it supports it
	if deltaWatcher, ok := c.apiKeyCache.(interface {
		CreateDeltaWatch(*cache.DeltaRequest, stream.StreamState, chan cache.DeltaResponse) func()
	}); ok {
		apiKeyCancel = deltaWatcher.CreateDeltaWatch(request, streamState, c.createDeltaResponseHandler(watcherID, "apikey", responseChan))
		c.logger.Debug("API key cache supports delta watch", slog.Int64("watcher_id", watcherID))
	} else {
		c.logger.Debug("API key cache does not support delta watch, skipping", slog.Int64("watcher_id", watcherID))
	}

	// If neither cache supports delta watch, we could fall back to regular watch
	// but for now we'll just return a no-op cancel function
	if policyCancel == nil && apiKeyCancel == nil {
		c.logger.Warn("Neither underlying cache supports delta watch",
			slog.Int64("watcher_id", watcherID),
			slog.String("type_url", request.TypeUrl))
	}

	// Return cancel function
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		if policyCancel != nil {
			policyCancel()
		}
		if apiKeyCancel != nil {
			apiKeyCancel()
		}

		c.logger.Debug("Canceled combined delta watch", slog.Int64("watcher_id", watcherID))
	}
}

// Fetch fetches resources from both caches and combines them
// Implements cache.ConfigFetcher interface
func (c *CombinedCache) Fetch(ctx context.Context, request *cache.Request) (cache.Response, error) {
	c.logger.Debug("Fetching from combined cache",
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()),
		slog.Any("resource_names", request.ResourceNames))

	// Try to fetch from policy cache first
	if response, err := c.policyCache.Fetch(ctx, request); err == nil {
		version, versionErr := response.GetVersion()
		if versionErr != nil {
			version = "unknown"
		}
		c.logger.Debug("Fetched from policy cache",
			slog.String("version", version))
		return response, nil
	}

	// If not found in policy cache, try API key cache
	if response, err := c.apiKeyCache.Fetch(ctx, request); err == nil {
		version, versionErr := response.GetVersion()
		if versionErr != nil {
			version = "unknown"
		}
		c.logger.Debug("Fetched from API key cache",
			slog.String("version", version))
		return response, nil
	}

	// If not found in either cache, return empty response
	c.logger.Debug("Resource not found in either cache",
		slog.String("type_url", request.TypeUrl),
		slog.Any("resource_names", request.ResourceNames))

	return &cache.RawResponse{
		Version:   "0",
		Resources: nil,
		Request:   request,
	}, nil
}

// createDeltaResponseHandler creates a response handler for delta responses
func (c *CombinedCache) createDeltaResponseHandler(watcherID int64, cacheType string, mainResponseChan chan cache.DeltaResponse) chan cache.DeltaResponse {
	responseChan := make(chan cache.DeltaResponse, 1)

	go func() {
		for response := range responseChan {
			c.logger.Debug("Received delta response from cache",
				slog.Int64("watcher_id", watcherID),
				slog.String("cache_type", cacheType))

			// Forward the delta response to the main response channel
			select {
			case mainResponseChan <- response:
				c.logger.Debug("Forwarded delta response from combined cache",
					slog.Int64("watcher_id", watcherID),
					slog.String("cache_type", cacheType))
			default:
				c.logger.Warn("Failed to forward delta response, channel full",
					slog.Int64("watcher_id", watcherID),
					slog.String("cache_type", cacheType))
			}
		}
	}()

	return responseChan
}

// cancelWatch cancels a watch by ID
func (c *CombinedCache) cancelWatch(watcherID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	watcher, exists := c.watchers[watcherID]
	if !exists {
		c.logger.Debug("Watcher already removed",
			slog.Int64("watcher_id", watcherID))
		return
	}

	c.logger.Debug("Canceling combined watch",
		slog.Int64("watcher_id", watcherID))

	// Signal the goroutine to stop by closing the done channel
	close(watcher.done)

	// Remove the watcher from the map first to prevent new responses
	delete(c.watchers, watcherID)

	// Cancel both underlying watchers if they exist
	if watcher.policyCancel != nil {
		watcher.policyCancel()
	}
	if watcher.apiKeyCancel != nil {
		watcher.apiKeyCancel()
	}
}
