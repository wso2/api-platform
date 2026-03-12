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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// SubscriptionStateHandler handles subscription state received via xDS.
type SubscriptionStateHandler struct {
	store  *policyenginev1.SubscriptionStore
	logger *slog.Logger
}

// NewSubscriptionStateHandler creates a new SubscriptionStateHandler.
func NewSubscriptionStateHandler(store *policyenginev1.SubscriptionStore, logger *slog.Logger) *SubscriptionStateHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SubscriptionStateHandler{
		store:  store,
		logger: logger,
	}
}

// HandleSubscriptionState processes subscription state resources received via xDS.
func (h *SubscriptionStateHandler) HandleSubscriptionState(ctx context.Context, resources map[string]*anypb.Any) error {
	h.logger.Info("Received subscription state via xDS", "resource_count", len(resources))

	// An empty snapshot should clear local subscription state to avoid retaining stale ACTIVE entries.
	if len(resources) == 0 {
		if h.store == nil {
			h.logger.Warn("Empty subscription snapshot received but store is nil; skipping state clear")
			return fmt.Errorf("subscription snapshot received but store is nil (misconfiguration); cannot apply state")
		}
		h.logger.Info("Empty subscription snapshot received; clearing local subscription store")
		h.store.ReplaceAll(nil)
		return nil
	}

	var allSubscriptions []policyenginev1.SubscriptionData

	for resourceName, resource := range resources {
		if resource == nil {
			h.logger.ErrorContext(ctx, "subscription state: nil resource",
				"resource_name", resourceName)
			return fmt.Errorf("nil subscription state resource: %s", resourceName)
		}
		if resource.TypeUrl != SubscriptionStateTypeURL {
			// Treat unexpected type URLs as a hard error so the ADS stream can NACK
			// instead of ACKing and potentially wiping valid state.
			h.logger.ErrorContext(ctx, "subscription state: unexpected resource type URL",
				"expected", SubscriptionStateTypeURL,
				"actual", resource.TypeUrl,
				"resource_name", resourceName)
			return fmt.Errorf("unexpected subscription state resource type: got %s, want %s", resource.TypeUrl, SubscriptionStateTypeURL)
		}

		subStruct := &structpb.Struct{}
		if err := proto.Unmarshal(resource.Value, subStruct); err != nil {
			// Fallback: some control-plane versions may wrap the Struct in an inner Any.
			innerAny := &anypb.Any{}
			if errAny := proto.Unmarshal(resource.Value, innerAny); errAny == nil {
				if errStruct := proto.Unmarshal(innerAny.Value, subStruct); errStruct == nil {
					// Successfully decoded via inner Any wrapper.
				} else {
					return fmt.Errorf("failed to unmarshal subscription struct from inner Any: %w", errStruct)
				}
			} else {
				return fmt.Errorf("failed to unmarshal subscription struct from resource: %w", err)
			}
		}

		jsonBytes, err := protojson.Marshal(subStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal subscription struct to JSON: %w", err)
		}

		var state policyenginev1.SubscriptionStateResource
		if err := json.Unmarshal(jsonBytes, &state); err != nil {
			// Treat malformed resources as a hard error so the ADS stream can NACK
			// and retry instead of silently ACKing and leaving stale state.
			return fmt.Errorf("failed to unmarshal subscription state for resource %s: %w", resourceName, err)
		}

		h.logger.Info("Processing subscription state",
			"subscription_count", len(state.Subscriptions),
			"version", state.Version)

		allSubscriptions = append(allSubscriptions, state.Subscriptions...)
	}

	// Apply the merged "state of the world" snapshot atomically.
	if h.store == nil {
		h.logger.Warn("Subscription snapshot received but store is nil; skipping state application")
		return fmt.Errorf("subscription snapshot received but store is nil (misconfiguration); cannot apply state")
	}
	h.store.ReplaceAll(allSubscriptions)

	return nil
}
