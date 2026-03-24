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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"
)

// CombinedCache combines policy, API key, lazy resource, subscription, and route config caches to provide a unified xDS cache interface
// It implements cache.Cache interface by delegating to underlying caches
type CombinedCache struct {
	policyCache       cache.Cache
	apiKeyCache       cache.Cache
	lazyResourceCache cache.Cache
	subscriptionCache cache.Cache
	routeConfigCache  cache.Cache
	logger            *slog.Logger
	mu                sync.RWMutex
	watchers          map[int64]*combinedWatcher
	watcherID         int64
}

// combinedWatcher manages watchers for policy, API key, lazy resource, subscription, and route config caches
type combinedWatcher struct {
	id                 int64
	request            *cache.Request
	subscription       cache.Subscription
	responseChan       chan cache.Response
	policyCancel       func()
	apiKeyCancel       func()
	lazyResourceCancel func()
	subscriptionCancel func()
	routeConfigCancel  func()
	combinedCache      *CombinedCache
	done               chan struct{} // done channel to signal goroutine cancellation
}

// NewCombinedCache creates a new combined cache that merges policy, API key, lazy resource, subscription, and route config caches.
// Returns a cache.Cache interface implementation.
func NewCombinedCache(policyCache cache.Cache, apiKeyCache cache.Cache, lazyResourceCache cache.Cache, subscriptionCache cache.Cache, routeConfigCache cache.Cache, logger *slog.Logger) cache.Cache {
	if policyCache == nil || apiKeyCache == nil || lazyResourceCache == nil || subscriptionCache == nil {
		panic("policyCache, apiKeyCache, lazyResourceCache, and subscriptionCache must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CombinedCache{
		policyCache:       policyCache,
		apiKeyCache:       apiKeyCache,
		lazyResourceCache: lazyResourceCache,
		subscriptionCache: subscriptionCache,
		routeConfigCache:  routeConfigCache,
		logger:            logger,
		watchers:          make(map[int64]*combinedWatcher),
		watcherID:         0,
	}
}

func newSotwSubscription(request *cache.Request) cache.Subscription {
	subscription := stream.NewSotwSubscription(request.GetResourceNames(), false)
	return &subscription
}

func newDeltaSubscription(request *cache.DeltaRequest) cache.Subscription {
	subscription := stream.NewDeltaSubscription(
		request.GetResourceNamesSubscribe(),
		request.GetResourceNamesUnsubscribe(),
		request.GetInitialResourceVersions(),
		false,
	)
	return &subscription
}

type cacheSelection struct {
	name      string
	cache     cache.Cache
	deltaName string
}

func (c *CombinedCache) cachesForType(typeURL string) []cacheSelection {
	switch typeURL {
	case PolicyChainTypeURL:
		return []cacheSelection{{name: "policy", cache: c.policyCache, deltaName: "policy"}}
	case apikeyxds.APIKeyStateTypeURL:
		return []cacheSelection{{name: "api_key", cache: c.apiKeyCache, deltaName: "apikey"}}
	case lazyresourcexds.LazyResourceTypeURL:
		return []cacheSelection{{name: "lazy_resource", cache: c.lazyResourceCache, deltaName: "lazyresource"}}
	case subscriptionxds.SubscriptionStateTypeURL:
		return []cacheSelection{{name: "subscription", cache: c.subscriptionCache, deltaName: "subscription"}}
	case RouteConfigTypeURL:
		if c.routeConfigCache == nil {
			return nil
		}
		return []cacheSelection{{name: "route_config", cache: c.routeConfigCache, deltaName: "routeconfig"}}
	default:
		caches := []cacheSelection{
			{name: "policy", cache: c.policyCache, deltaName: "policy"},
			{name: "api_key", cache: c.apiKeyCache, deltaName: "apikey"},
			{name: "lazy_resource", cache: c.lazyResourceCache, deltaName: "lazyresource"},
		}
		if c.subscriptionCache != nil {
			caches = append(caches, cacheSelection{name: "subscription", cache: c.subscriptionCache, deltaName: "subscription"})
		}
		if c.routeConfigCache != nil {
			caches = append(caches, cacheSelection{name: "route_config", cache: c.routeConfigCache, deltaName: "routeconfig"})
		}
		return caches
	}
}

// CreateWatch creates a watch for resources in both policy and API key caches
// Implements cache.ConfigWatcher interface
func (c *CombinedCache) CreateWatch(request *cache.Request, subscription cache.Subscription, responseChan chan cache.Response) (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if subscription == nil {
		subscription = newSotwSubscription(request)
	}

	c.watcherID++
	watcherID := c.watcherID

	watcher := &combinedWatcher{
		id:            watcherID,
		request:       request,
		subscription:  subscription,
		responseChan:  responseChan,
		combinedCache: c,
		done:          make(chan struct{}),
	}

	c.watchers[watcherID] = watcher

	c.logger.Debug("Creating combined watch",
		slog.Int64("watcher_id", watcherID),
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()))

	selectedCaches := c.cachesForType(request.TypeUrl)
	var policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, subscriptionResponseChan, routeConfigResponseChan chan cache.Response

	for _, selectedCache := range selectedCaches {
		responseChannel := make(chan cache.Response, 1)
		cancelFn, watchErr := selectedCache.cache.CreateWatch(request, subscription, responseChannel)
		if watchErr != nil {
			if watcher.policyCancel != nil {
				watcher.policyCancel()
			}
			if watcher.apiKeyCancel != nil {
				watcher.apiKeyCancel()
			}
			if watcher.lazyResourceCancel != nil {
				watcher.lazyResourceCancel()
			}
			if watcher.subscriptionCancel != nil {
				watcher.subscriptionCancel()
			}
			if watcher.routeConfigCancel != nil {
				watcher.routeConfigCancel()
			}
			delete(c.watchers, watcherID)
			return nil, watchErr
		}

		switch selectedCache.name {
		case "policy":
			policyResponseChan = responseChannel
			watcher.policyCancel = cancelFn
		case "api_key":
			apiKeyResponseChan = responseChannel
			watcher.apiKeyCancel = cancelFn
		case "lazy_resource":
			lazyResourceResponseChan = responseChannel
			watcher.lazyResourceCancel = cancelFn
		case "subscription":
			subscriptionResponseChan = responseChannel
			watcher.subscriptionCancel = cancelFn
		case "route_config":
			routeConfigResponseChan = responseChannel
			watcher.routeConfigCancel = cancelFn
		}
	}

	// Start a response multiplexer to handle responses from all caches
	go c.handleCombinedResponses(watcherID, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, subscriptionResponseChan, routeConfigResponseChan, responseChan, watcher.done)

	// Return cancel function
	return func() {
		c.cancelWatch(watcherID)
	}, nil
}

// handleCombinedResponses multiplexes responses from all caches
// This prevents recursion and handles response deduplication
func (c *CombinedCache) handleCombinedResponses(watcherID int64, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, subscriptionResponseChan, routeConfigResponseChan chan cache.Response,
	mainResponseChan chan cache.Response, done chan struct{}) {
	defer func() {
		c.logger.Debug("Response handler goroutine exiting", slog.Int64("watcher_id", watcherID))
	}()

	var lastPolicyVersion, lastApiKeyVersion, lastLazyResourceVersion, lastSubscriptionVersion, lastRouteConfigVersion string

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

		case response, ok := <-lazyResourceResponseChan:
			if !ok {
				c.logger.Debug("Lazy resource response channel closed", slog.Int64("watcher_id", watcherID))
				return
			}

			// Handle nil response only if we haven't sent initial response yet
			if response == nil {
				// Don't create continuous empty responses - this causes the loop
				c.logger.Debug("Lazy resource cache has no data, skipping nil response",
					slog.Int64("watcher_id", watcherID))
				continue
			}

			// Check if this is a duplicate response
			version, err := response.GetVersion()
			if err != nil {
				version = "unknown"
			}

			if version != lastLazyResourceVersion {
				lastLazyResourceVersion = version
				c.logger.Debug("Forwarding lazy resource cache response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))

				select {
				case mainResponseChan <- response:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					c.logger.Warn("Timeout sending lazy resource response, client may be slow",
						slog.Int64("watcher_id", watcherID),
						slog.String("version", version))
				}
			} else {
				c.logger.Debug("Skipping duplicate lazy resource response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))
			}

		case response, ok := <-subscriptionResponseChan:
			if !ok {
				c.logger.Debug("Subscription response channel closed", slog.Int64("watcher_id", watcherID))
				return
			}

			if response == nil {
				c.logger.Debug("Subscription cache has no data, skipping nil response",
					slog.Int64("watcher_id", watcherID))
				continue
			}

			version, err := response.GetVersion()
			if err != nil {
				version = "unknown"
			}

			if version != lastSubscriptionVersion {
				lastSubscriptionVersion = version
				c.logger.Debug("Forwarding subscription cache response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))

				select {
				case mainResponseChan <- response:
				case <-time.After(100 * time.Millisecond):
					c.logger.Warn("Timeout sending subscription response, client may be slow",
						slog.Int64("watcher_id", watcherID),
						slog.String("version", version))
				}
			} else {
				c.logger.Debug("Skipping duplicate subscription response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))
			}

		case response, ok := <-routeConfigResponseChan:
			if !ok {
				c.logger.Debug("Route config response channel closed", slog.Int64("watcher_id", watcherID))
				return
			}

			if response == nil {
				c.logger.Debug("Route config cache has no data, skipping nil response",
					slog.Int64("watcher_id", watcherID))
				continue
			}

			version, err := response.GetVersion()
			if err != nil {
				version = "unknown"
			}

			if version != lastRouteConfigVersion {
				lastRouteConfigVersion = version
				c.logger.Debug("Forwarding route config cache response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))

				select {
				case mainResponseChan <- response:
				case <-time.After(100 * time.Millisecond):
					c.logger.Warn("Timeout sending route config response, client may be slow",
						slog.Int64("watcher_id", watcherID),
						slog.String("version", version))
				}
			} else {
				c.logger.Debug("Skipping duplicate route config response",
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
func (c *CombinedCache) CreateDeltaWatch(request *cache.DeltaRequest, subscription cache.Subscription, responseChan chan cache.DeltaResponse) (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if subscription == nil {
		subscription = newDeltaSubscription(request)
	}

	c.watcherID++
	watcherID := c.watcherID

	c.logger.Debug("Creating combined delta watch",
		slog.Int64("watcher_id", watcherID),
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()))

	// Create delta watches on all underlying caches
	var policyCancel, apiKeyCancel, lazyResourceCancel, subscriptionCancel, routeConfigCancel func()

	selectedCaches := c.cachesForType(request.TypeUrl)
	for _, selectedCache := range selectedCaches {
		if deltaWatcher, ok := selectedCache.cache.(interface {
			CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
		}); ok {
			cancelFn, err := deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, selectedCache.deltaName, responseChan))
			if err != nil {
				if policyCancel != nil {
					policyCancel()
				}
				if apiKeyCancel != nil {
					apiKeyCancel()
				}
				if lazyResourceCancel != nil {
					lazyResourceCancel()
				}
				if subscriptionCancel != nil {
					subscriptionCancel()
				}
				if routeConfigCancel != nil {
					routeConfigCancel()
				}
				return nil, err
			}
			switch selectedCache.name {
			case "policy":
				policyCancel = cancelFn
			case "api_key":
				apiKeyCancel = cancelFn
			case "lazy_resource":
				lazyResourceCancel = cancelFn
			case "subscription":
				subscriptionCancel = cancelFn
			case "route_config":
				routeConfigCancel = cancelFn
			}
			c.logger.Debug("Cache supports delta watch",
				slog.Int64("watcher_id", watcherID),
				slog.String("cache_type", selectedCache.name))
		} else {
			c.logger.Debug("Cache does not support delta watch, skipping",
				slog.Int64("watcher_id", watcherID),
				slog.String("cache_type", selectedCache.name))
		}
	}

	// If no cache supports delta watch, we could fall back to regular watch
	// but for now we'll just return a no-op cancel function
	if policyCancel == nil && apiKeyCancel == nil && lazyResourceCancel == nil && subscriptionCancel == nil && routeConfigCancel == nil {
		c.logger.Warn("No underlying cache supports delta watch",
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
		if lazyResourceCancel != nil {
			lazyResourceCancel()
		}
		if subscriptionCancel != nil {
			subscriptionCancel()
		}
		if routeConfigCancel != nil {
			routeConfigCancel()
		}

		c.logger.Debug("Canceled combined delta watch", slog.Int64("watcher_id", watcherID))
	}, nil
}

// Fetch fetches resources from both caches and combines them
// Implements cache.ConfigFetcher interface
func (c *CombinedCache) Fetch(ctx context.Context, request *cache.Request) (cache.Response, error) {
	c.logger.Debug("Fetching from combined cache",
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()),
		slog.Any("resource_names", request.ResourceNames))

	for _, selectedCache := range c.cachesForType(request.TypeUrl) {
		if selectedCache.cache == nil {
			continue
		}
		if response, err := selectedCache.cache.Fetch(ctx, request); err == nil && response != nil {
			version, versionErr := response.GetVersion()
			if versionErr != nil {
				version = "unknown"
			}
			c.logger.Debug("Fetched from cache",
				slog.String("cache_type", selectedCache.name),
				slog.String("version", version))
			return response, nil
		}
	}

	// If not found in any cache, return empty response
	c.logger.Debug("Resource not found in any cache",
		slog.String("type_url", request.TypeUrl),
		slog.Any("resource_names", request.ResourceNames))

	return &cache.RawResponse{
		Version: "0",
		Request: request,
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

	// Cancel all underlying watchers if they exist
	if watcher.policyCancel != nil {
		watcher.policyCancel()
	}
	if watcher.apiKeyCancel != nil {
		watcher.apiKeyCancel()
	}
	if watcher.lazyResourceCancel != nil {
		watcher.lazyResourceCancel()
	}
	if watcher.subscriptionCancel != nil {
		watcher.subscriptionCancel()
	}
	if watcher.routeConfigCancel != nil {
		watcher.routeConfigCancel()
	}
}
