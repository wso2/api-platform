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

// MetadataXDSHandler handles metadata XDS updates received via xDS
type MetadataXDSHandler struct {
	metadataXDSStore *policy.MetadataXDSStore
	logger            *slog.Logger
}

// NewMetadataXDSHandler creates a new metadata XDS handler
func NewMetadataXDSHandler(metadataXDSStore *policy.MetadataXDSStore, logger *slog.Logger) *MetadataXDSHandler {
	return &MetadataXDSHandler{
		metadataXDSStore: metadataXDSStore,
		logger:            logger,
	}
}

// HandleMetadataXDSUpdate processes metadata XDS state received via xDS
func (h *MetadataXDSHandler) HandleMetadataXDSUpdate(ctx context.Context, resources map[string]*anypb.Any) error {
	h.logger.Info("Received metadata XDS state via xDS", "resource_count", len(resources))

	for resourceName, resource := range resources {
		if resource.TypeUrl != MetadataXDSTypeURL {
			slog.WarnContext(ctx, "Skipping resource with unexpected type",
				"expected", MetadataXDSTypeURL,
				"actual", resource.TypeUrl)
			continue
		}

		// Unmarshal google.protobuf.Struct directly from the resource Value.
		// The xDS server sends the serialized Struct inside the resource Value.
		metadataXDSStruct := &structpb.Struct{}
		if err := proto.Unmarshal(resource.Value, metadataXDSStruct); err != nil {
			return fmt.Errorf("failed to unmarshal metadata XDS struct from resource: %w", err)
		}

		// Convert Struct to JSON then to MetadataXDSStateResource
		jsonBytes, err := protojson.Marshal(metadataXDSStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata XDS struct to JSON: %w", err)
		}

		var metadataXDSState MetadataXDSStateResource
		if err := json.Unmarshal(jsonBytes, &metadataXDSState); err != nil {
			return fmt.Errorf("failed to unmarshal metadata XDS state for %s: %w", resourceName, err)
		}

		h.logger.Info("Processing metadata XDS state",
			"version", metadataXDSState.Version,
			"resource_count", len(metadataXDSState.Resources))

		// Replace all metadata XDSs with the new state (state-of-the-world approach)
		if err := h.replaceAllMetadataXDSs(metadataXDSState.Resources); err != nil {
			return fmt.Errorf("failed to replace metadata XDSs for %s: %w", resourceName, err)
		}
	}

	return nil
}

// replaceAllMetadataXDSs replaces all metadata XDSs with the new state (state-of-the-world approach)
func (h *MetadataXDSHandler) replaceAllMetadataXDSs(resourceDataList []MetadataXDSData) error {
	h.logger.Info("Replacing all metadata XDSs with new state", "resource_count", len(resourceDataList))

	// Convert to SDK format
	resources := make([]*policy.MetadataXDS, 0, len(resourceDataList))
	for _, data := range resourceDataList {
		resource := &policy.MetadataXDS{
			ID:           data.ID,
			ResourceType: data.ResourceType,
			Resource:     data.Resource,
		}
		resources = append(resources, resource)
	}

	// Replace all resources atomically
	if err := h.metadataXDSStore.ReplaceAll(resources); err != nil {
		return fmt.Errorf("failed to replace metadata XDSs: %w", err)
	}

	// Debug logging: print all resources received via xDS
	for _, r := range resources {
		h.logger.Debug("Metadata XDS loaded via xDS",
			"id", r.ID,
			"resource_type", r.ResourceType)
	}

	h.logger.Info("Successfully replaced all metadata XDSs with new state",
		"resource_count", len(resourceDataList))

	return nil
}

// MetadataXDSStateResource represents the complete state of metadata XDSs for the policy engine
type MetadataXDSStateResource struct {
	Resources []MetadataXDSData `json:"resources"`
	Version   int64              `json:"version"`
	Timestamp int64              `json:"timestamp"`
}

// MetadataXDSData represents a metadata XDS in the state resource
type MetadataXDSData struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

