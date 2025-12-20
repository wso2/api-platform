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
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/policy-engine/policy-engine/internal/constants"
)

// buildAnalyticsStruct converts analytics metadata map to structpb.Struct
// If execCtx is provided, adds system-level metadata (API name, version, etc.) to analytics_data.metadata
func buildAnalyticsStruct(analyticsData map[string]any, execCtx *PolicyExecutionContext) (*structpb.Struct, error) {
	// Start with the analytics data from policies
	fields := make(map[string]*structpb.Value)

	// Add policy-provided analytics data
	for key, value := range analyticsData {
		val, err := structpb.NewValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert analytics value for key %s: %w", key, err)
		}
		fields[key] = val
	}

	// Add system-level metadata if context is provided
	if execCtx != nil && execCtx.requestContext != nil && execCtx.requestContext.SharedContext != nil {
		metadata := make(map[string]interface{})

		sharedCtx := execCtx.requestContext.SharedContext
		if sharedCtx.APIId != "" {
			metadata["api_id"] = sharedCtx.APIId
		}
		if sharedCtx.APIName != "" {
			metadata["api_name"] = sharedCtx.APIName
		}
		if sharedCtx.APIVersion != "" {
			metadata["api_version"] = sharedCtx.APIVersion
		}
		if sharedCtx.APIContext != "" {
			metadata["api_context"] = sharedCtx.APIContext
		}
		if sharedCtx.OperationPath != "" {
			metadata["operation_path"] = sharedCtx.OperationPath
		}

		if len(metadata) > 0 {
			metadataVal, err := structpb.NewValue(metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to convert system metadata: %w", err)
			}
			fields["metadata"] = metadataVal
		}
	}

	return &structpb.Struct{Fields: fields}, nil
}

// buildDynamicMetadata creates the dynamic metadata structure for analytics
func buildDynamicMetadata(analyticsStruct *structpb.Struct) *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			constants.ExtProcFilterName: {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"analytics_data": {
								Kind: &structpb.Value_StructValue{
									StructValue: analyticsStruct,
								},
							},
						},
					},
				},
			},
		},
	}
}

// extractMetadataFromMap extracts the metadata from the route metadata
func extractMetadataFromRouteMetadata(routeMeta RouteMetadata) map[string]interface{} {
	metadata := make(map[string]interface{})
	if routeMeta.APIName != "" {
		metadata["api_name"] = routeMeta.APIName
	}
	if routeMeta.APIVersion != "" {
		metadata["api_version"] = routeMeta.APIVersion
	}
	if routeMeta.Context != "" {
		metadata["api_context"] = routeMeta.Context
	}
	if routeMeta.OperationPath != "" {
		metadata["operation_path"] = routeMeta.OperationPath
	}
	return metadata
}