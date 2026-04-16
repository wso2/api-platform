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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"
)

// CombinedCache combines policy, API key, lazy resource, subscription, route config, and event channel caches to provide a unified xDS cache interface
// It implements cache.Cache interface by delegating to underlying caches
type CombinedCache struct {
	policyCache        cache.Cache
	apiKeyCache        cache.Cache
	lazyResourceCache  cache.Cache
	subscriptionCache  cache.Cache
	routeConfigCache   cache.Cache
	eventChannelCache  cache.Cache
	logger             *slog.Logger
	mu                 sync.RWMutex
	watchers           map[int64]*combinedWatcher
	watcherID          int64
}

// combinedWatcher manages watchers for policy, API key, lazy resource, subscription, route config, and event channel caches
type combinedWatcher struct {
	id                    int64
	request               *cache.Request
	subscription          cache.Subscription
	responseChan          chan cache.Response
	policyCancel          func()
	apiKeyCancel          func()
	lazyResourceCancel    func()
	subscriptionCancel    func()
	routeConfigCancel     func()
	eventChannelCancel    func()
	combinedCache         *CombinedCache
	done                  chan struct{} // done channel to signal goroutine cancellation
}

// NewCombinedCache creates a new combined cache that merges policy, API key, lazy resource, subscription, route config, and event channel caches.
// Returns a cache.Cache interface implementation.
func NewCombinedCache(policyCache cache.Cache, apiKeyCache cache.Cache, lazyResourceCache cache.Cache, subscriptionCache cache.Cache, routeConfigCache cache.Cache, eventChannelCache cache.Cache, logger *slog.Logger) cache.Cache {
	if policyCache == nil || apiKeyCache == nil || lazyResourceCache == nil || subscriptionCache == nil {
		panic("policyCache, apiKeyCache, lazyResourceCache, and subscriptionCache must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CombinedCache{
		policyCache:        policyCache,
		apiKeyCache:        apiKeyCache,
		lazyResourceCache:  lazyResourceCache,
		subscriptionCache:  subscriptionCache,
		routeConfigCache:   routeConfigCache,
		eventChannelCache:  eventChannelCache,
		logger:             logger,
		watchers:           make(map[int64]*combinedWatcher),
		watcherID:          0,
	}
}

// CreateWatch creates a watch for resources in both policy and API key caches
// Implements cache.ConfigWatcher interface
func (c *CombinedCache) CreateWatch(request *cache.Request, subscription cache.Subscription, responseChan chan cache.Response) (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	var (
		policyResponseChan       chan cache.Response
		apiKeyResponseChan       chan cache.Response
		lazyResourceResponseChan chan cache.Response
		subscriptionResponseChan chan cache.Response
		routeConfigResponseChan  chan cache.Response
		eventChannelResponseChan chan cache.Response
		err                      error
	)

	switch request.TypeUrl {
	case PolicyChainTypeURL:
		policyResponseChan = make(chan cache.Response, 1)
		watcher.policyCancel, err = c.policyCache.CreateWatch(request, subscription, policyResponseChan)
		if err != nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("create policy watch: %w", err)
		}
	case RouteConfigTypeURL:
		if c.routeConfigCache == nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("route config cache is not configured for type %s", request.TypeUrl)
		}
		routeConfigResponseChan = make(chan cache.Response, 1)
		watcher.routeConfigCancel, err = c.routeConfigCache.CreateWatch(request, subscription, routeConfigResponseChan)
		if err != nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("create route config watch: %w", err)
		}
	case apikeyxds.APIKeyStateTypeURL:
		apiKeyResponseChan = make(chan cache.Response, 1)
		watcher.apiKeyCancel, err = c.apiKeyCache.CreateWatch(request, subscription, apiKeyResponseChan)
		if err != nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("create api key watch: %w", err)
		}
	case lazyresourcexds.LazyResourceTypeURL:
		lazyResourceResponseChan = make(chan cache.Response, 1)
		watcher.lazyResourceCancel, err = c.lazyResourceCache.CreateWatch(request, subscription, lazyResourceResponseChan)
		if err != nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("create lazy resource watch: %w", err)
		}
	case subscriptionxds.SubscriptionStateTypeURL:
		subscriptionResponseChan = make(chan cache.Response, 1)
		watcher.subscriptionCancel, err = c.subscriptionCache.CreateWatch(request, subscription, subscriptionResponseChan)
		if err != nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("create subscription watch: %w", err)
		}
	case EventChannelConfigTypeURL:
		if c.eventChannelCache == nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("event channel cache is not configured for type %s", request.TypeUrl)
		}
		eventChannelResponseChan = make(chan cache.Response, 1)
		watcher.eventChannelCancel, err = c.eventChannelCache.CreateWatch(request, subscription, eventChannelResponseChan)
		if err != nil {
			delete(c.watchers, watcherID)
			return nil, fmt.Errorf("create event channel watch: %w", err)
		}
	default:
		delete(c.watchers, watcherID)
		return nil, fmt.Errorf("unsupported combined cache type %s", request.TypeUrl)
	}

	// Start a response multiplexer to handle responses from all caches
	go c.handleCombinedResponses(watcherID, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, subscriptionResponseChan, routeConfigResponseChan, eventChannelResponseChan, responseChan, watcher.done)

	// Return cancel function
	return func() {
		c.cancelWatch(watcherID)
	}, nil
}

// handleCombinedResponses multiplexes responses from all caches
// This prevents recursion and handles response deduplication
func (c *CombinedCache) handleCombinedResponses(watcherID int64, policyResponseChan, apiKeyResponseChan, lazyResourceResponseChan, subscriptionResponseChan, routeConfigResponseChan, eventChannelResponseChan chan cache.Response,
	mainResponseChan chan cache.Response, done chan struct{}) {
	defer func() {
		c.logger.Debug("Response handler goroutine exiting", slog.Int64("watcher_id", watcherID))
	}()

	var lastPolicyVersion, lastApiKeyVersion, lastLazyResourceVersion, lastSubscriptionVersion, lastRouteConfigVersion, lastEventChannelVersion string

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

		case response, ok := <-eventChannelResponseChan:
			if !ok {
				c.logger.Debug("Event channel response channel closed", slog.Int64("watcher_id", watcherID))
				return
			}

			if response == nil {
				c.logger.Debug("Event channel cache has no data, skipping nil response",
					slog.Int64("watcher_id", watcherID))
				continue
			}

			version, err := response.GetVersion()
			if err != nil {
				version = "unknown"
			}

			if version != lastEventChannelVersion {
				lastEventChannelVersion = version
				c.logger.Debug("Forwarding event channel cache response",
					slog.Int64("watcher_id", watcherID),
					slog.String("version", version))

				select {
				case mainResponseChan <- response:
				case <-time.After(100 * time.Millisecond):
					c.logger.Warn("Timeout sending event channel response, client may be slow",
						slog.Int64("watcher_id", watcherID),
						slog.String("version", version))
				}
			} else {
				c.logger.Debug("Skipping duplicate event channel response",
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

	c.watcherID++
	watcherID := c.watcherID

	c.logger.Debug("Creating combined delta watch",
		slog.Int64("watcher_id", watcherID),
		slog.String("type_url", request.TypeUrl),
		slog.String("node_id", request.Node.GetId()))

	var policyCancel, apiKeyCancel, lazyResourceCancel, subscriptionCancel, routeConfigCancel, eventChannelCancel func()
	var err error

	switch request.TypeUrl {
	case PolicyChainTypeURL:
		if deltaWatcher, ok := c.policyCache.(interface {
			CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
		}); ok {
			policyCancel, err = deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, "policy", responseChan))
			if err != nil {
				return nil, fmt.Errorf("create policy delta watch: %w", err)
			}
		}
	case RouteConfigTypeURL:
		if c.routeConfigCache != nil {
			if deltaWatcher, ok := c.routeConfigCache.(interface {
				CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
			}); ok {
				routeConfigCancel, err = deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, "routeconfig", responseChan))
				if err != nil {
					return nil, fmt.Errorf("create route config delta watch: %w", err)
				}
			}
		}
	case apikeyxds.APIKeyStateTypeURL:
		if deltaWatcher, ok := c.apiKeyCache.(interface {
			CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
		}); ok {
			apiKeyCancel, err = deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, "apikey", responseChan))
			if err != nil {
				return nil, fmt.Errorf("create api key delta watch: %w", err)
			}
		}
	case lazyresourcexds.LazyResourceTypeURL:
		if deltaWatcher, ok := c.lazyResourceCache.(interface {
			CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
		}); ok {
			lazyResourceCancel, err = deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, "lazyresource", responseChan))
			if err != nil {
				return nil, fmt.Errorf("create lazy resource delta watch: %w", err)
			}
		}
	case subscriptionxds.SubscriptionStateTypeURL:
		if deltaWatcher, ok := c.subscriptionCache.(interface {
			CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
		}); ok {
			subscriptionCancel, err = deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, "subscription", responseChan))
			if err != nil {
				return nil, fmt.Errorf("create subscription delta watch: %w", err)
			}
		}
	case EventChannelConfigTypeURL:
		if c.eventChannelCache != nil {
			if deltaWatcher, ok := c.eventChannelCache.(interface {
				CreateDeltaWatch(*cache.DeltaRequest, cache.Subscription, chan cache.DeltaResponse) (func(), error)
			}); ok {
				eventChannelCancel, err = deltaWatcher.CreateDeltaWatch(request, subscription, c.createDeltaResponseHandler(watcherID, "eventchannel", responseChan))
				if err != nil {
					return nil, fmt.Errorf("create event channel delta watch: %w", err)
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported combined delta cache type %s", request.TypeUrl)
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
		if eventChannelCancel != nil {
			eventChannelCancel()
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

	// If not found in API key cache, try lazy resource cache
	if response, err := c.lazyResourceCache.Fetch(ctx, request); err == nil {
		version, versionErr := response.GetVersion()
		if versionErr != nil {
			version = "unknown"
		}
		c.logger.Debug("Fetched from lazy resource cache",
			slog.String("version", version))
		return response, nil
	}

	// If not found in lazy resource cache, try subscription cache (if configured)
	if c.subscriptionCache != nil {
		if response, err := c.subscriptionCache.Fetch(ctx, request); err == nil && response != nil {
			version, versionErr := response.GetVersion()
			if versionErr != nil {
				version = "unknown"
			}
			c.logger.Debug("Fetched from subscription cache",
				slog.String("version", version))
			return response, nil
		}
	}

	// If not found in subscription cache, try route config cache (if configured)
	if c.routeConfigCache != nil {
		if response, err := c.routeConfigCache.Fetch(ctx, request); err == nil && response != nil {
			version, versionErr := response.GetVersion()
			if versionErr != nil {
				version = "unknown"
			}
			c.logger.Debug("Fetched from route config cache",
				slog.String("version", version))
			return response, nil
		}
	}

	// If not found in route config cache, try event channel cache (if configured)
	if c.eventChannelCache != nil {
		if response, err := c.eventChannelCache.Fetch(ctx, request); err == nil && response != nil {
			version, versionErr := response.GetVersion()
			if versionErr != nil {
				version = "unknown"
			}
			c.logger.Debug("Fetched from event channel cache",
				slog.String("version", version))
			return response, nil
		}
	}

	// If not found in any cache, return empty response
	c.logger.Debug("Resource not found in any cache",
		slog.String("type_url", request.TypeUrl),
		slog.Any("resource_names", request.ResourceNames))

	return cache.NewTestRawResponse(request, "0", nil), nil
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
	if watcher.eventChannelCancel != nil {
		watcher.eventChannelCancel()
	}
}
