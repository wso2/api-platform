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
 * software distributed under an
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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// LazyResourceHandler handles lazy resource updates received via xDS
type LazyResourceHandler struct {
	lazyResourceStore *policy.LazyResourceStore
	logger            *slog.Logger
}

// NewLazyResourceHandler creates a new lazy resource handler
func NewLazyResourceHandler(lazyResourceStore *policy.LazyResourceStore, logger *slog.Logger) *LazyResourceHandler {
	return &LazyResourceHandler{
		lazyResourceStore: lazyResourceStore,
		logger:            logger,
	}
}

// HandleLazyResourceUpdate processes lazy resource state received via xDS
func (h *LazyResourceHandler) HandleLazyResourceUpdate(ctx context.Context, resources map[string]*anypb.Any) error {
	h.logger.Info("Received lazy resource state via xDS", "resource_count", len(resources))

	for resourceName, resource := range resources {
		if resource.TypeUrl != LazyResourceTypeURL {
			slog.WarnContext(ctx, "Skipping resource with unexpected type",
				"expected", LazyResourceTypeURL,
				"actual", resource.TypeUrl)
			continue
		}

		// Unmarshal google.protobuf.Struct from the Any
		// The xDS server double-wraps: res.Value contains serialized Any,
		// which in turn contains the serialized Struct
		innerAny := &anypb.Any{}
		if err := proto.Unmarshal(resource.Value, innerAny); err != nil {
			return fmt.Errorf("failed to unmarshal inner Any from resource: %w", err)
		}

		// Now unmarshal the Struct from the inner Any's Value
		lazyResourceStruct := &structpb.Struct{}
		if err := proto.Unmarshal(innerAny.Value, lazyResourceStruct); err != nil {
			return fmt.Errorf("failed to unmarshal lazy resource struct from inner Any: %w", err)
		}

		// Convert Struct to JSON then to LazyResourceStateResource
		jsonBytes, err := protojson.Marshal(lazyResourceStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal lazy resource struct to JSON: %w", err)
		}

		var lazyResourceState LazyResourceStateResource
		if err := json.Unmarshal(jsonBytes, &lazyResourceState); err != nil {
			return fmt.Errorf("failed to unmarshal lazy resource state for %s: %w", resourceName, err)
		}

		h.logger.Info("Processing lazy resource state",
			"version", lazyResourceState.Version,
			"resource_count", len(lazyResourceState.Resources))

		// Replace all lazy resources with the new state (state-of-the-world approach)
		if err := h.replaceAllLazyResources(lazyResourceState.Resources); err != nil {
			return fmt.Errorf("failed to replace lazy resources for %s: %w", resourceName, err)
		}
	}

	return nil
}

// replaceAllLazyResources replaces all lazy resources with the new state (state-of-the-world approach)
func (h *LazyResourceHandler) replaceAllLazyResources(resourceDataList []LazyResourceData) error {
	h.logger.Info("Replacing all lazy resources with new state", "resource_count", len(resourceDataList))

	// Convert to SDK format
	resources := make([]*policy.LazyResource, 0, len(resourceDataList))
	for _, data := range resourceDataList {
		resource := &policy.LazyResource{
			ID:           data.ID,
			ResourceType: data.ResourceType,
			Resource:     data.Resource,
		}
		resources = append(resources, resource)
	}

	// Replace all resources atomically
	if err := h.lazyResourceStore.ReplaceAll(resources); err != nil {
		return fmt.Errorf("failed to replace lazy resources: %w", err)
	}

	// Debug logging: print all resources received via xDS
	for _, r := range resources {
		h.logger.Debug("Lazy resource loaded via xDS",
			"id", r.ID,
			"resource_type", r.ResourceType)
	}

	h.logger.Info("Successfully replaced all lazy resources with new state",
		"resource_count", len(resourceDataList))

	return nil
}

// LazyResourceStateResource represents the complete state of lazy resources for the policy engine
type LazyResourceStateResource struct {
	Resources []LazyResourceData `json:"resources"`
	Version   int64              `json:"version"`
	Timestamp int64              `json:"timestamp"`
}

// LazyResourceData represents a lazy resource in the state resource
type LazyResourceData struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

