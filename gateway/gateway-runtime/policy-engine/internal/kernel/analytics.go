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

package kernel

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
)

// Constants for analytics metadata
const (
	Wso2MetadataPrefix = "x-wso2-"
	APIIDKey           = Wso2MetadataPrefix + "api-id"
	APINameKey         = Wso2MetadataPrefix + "api-name"
	APIVersionKey      = Wso2MetadataPrefix + "api-version"
	APITypeKey         = Wso2MetadataPrefix + "api-type"
	APIContextKey      = Wso2MetadataPrefix + "api-context"
	OperationPathKey   = Wso2MetadataPrefix + "operation-path"
	APIKindKey         = Wso2MetadataPrefix + "api-kind"
	ProjectIDKey       = Wso2MetadataPrefix + "project-id"
)

// convertToStructValue converts a value to structpb.Value, handling complex types like map[string][]string
func convertToStructValue(value any) (*structpb.Value, error) {
	// Try direct conversion first (works for simple types)
	val, err := structpb.NewValue(value)
	if err == nil {
		return val, nil
	}

	// If direct conversion fails, serialize to JSON string for complex types
	// This handles cases like map[string][]string which protobuf doesn't support directly
	jsonBytes, jsonErr := json.Marshal(value)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal value to JSON: %w", jsonErr)
	}

	return structpb.NewStringValue(string(jsonBytes)), nil
}

// buildAnalyticsStruct converts analytics metadata map to structpb.Struct
// If execCtx is provided, adds system-level metadata (API name, version, etc.) to analytics_data.metadata
func buildAnalyticsStruct(analyticsData map[string]any, execCtx *PolicyExecutionContext) (*structpb.Struct, error) {
	// Start with the analytics data from policies
	fields := make(map[string]*structpb.Value)

	// Add policy-provided analytics data
	for key, value := range analyticsData {
		val, err := convertToStructValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert analytics value for key %s: %w", key, err)
		}
		fields[key] = val
	}

	// Add system-level metadata if context is provided
	if execCtx != nil && execCtx.requestContext != nil && execCtx.requestContext.SharedContext != nil {

		sharedCtx := execCtx.requestContext.SharedContext
		if sharedCtx.APIId != "" {
			fields[APIIDKey] = structpb.NewStringValue(sharedCtx.APIId)
		}
		if sharedCtx.APIName != "" {
			fields[APINameKey] = structpb.NewStringValue(sharedCtx.APIName)
		}
		if sharedCtx.APIVersion != "" {
			fields[APIVersionKey] = structpb.NewStringValue(sharedCtx.APIVersion)
		}
		if sharedCtx.APIContext != "" {
			fields[APIContextKey] = structpb.NewStringValue(sharedCtx.APIContext)
		}
		if sharedCtx.OperationPath != "" {
			fields[OperationPathKey] = structpb.NewStringValue(sharedCtx.OperationPath)
		}
		if sharedCtx.APIKind != "" {
			fields[APIKindKey] = structpb.NewStringValue(sharedCtx.APIKind)
		}
		if sharedCtx.ProjectID != "" {
			fields[ProjectIDKey] = structpb.NewStringValue(sharedCtx.ProjectID)
		}
	}

	return &structpb.Struct{Fields: fields}, nil
}

// extractMetadataFromRouteMetadata extracts the metadata from the route metadata
func extractMetadataFromRouteMetadata(routeMeta RouteMetadata) map[string]interface{} {
	metadata := make(map[string]interface{})
	if routeMeta.APIName != "" {
		metadata[APINameKey] = routeMeta.APIName
	}
	if routeMeta.APIVersion != "" {
		metadata[APIVersionKey] = routeMeta.APIVersion
	}
	if routeMeta.Context != "" {
		metadata[APIContextKey] = routeMeta.Context
	}
	if routeMeta.OperationPath != "" {
		metadata[OperationPathKey] = routeMeta.OperationPath
	}
	if routeMeta.APIKind != "" {
		metadata[APIKindKey] = routeMeta.APIKind
	}
	if routeMeta.ProjectID != "" {
		metadata[ProjectIDKey] = routeMeta.ProjectID
	}
	return metadata
}
