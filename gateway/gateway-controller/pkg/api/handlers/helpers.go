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

package handlers

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Helper functions to convert values to pointers

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func uuidToOpenAPIUUID(id string) (*openapi_types.UUID, error) {
	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	openapiUUID := openapi_types.UUID(parsedUUID)
	return &openapiUUID, nil
}

// convertHandleToUUID converts a handle string to an OpenAPI UUID pointer
// Returns nil if the conversion fails (should not happen in normal operation)
func convertHandleToUUID(handle string) *openapi_types.UUID {
	uuid, err := uuidToOpenAPIUUID(handle)
	if err != nil {
		return nil
	}
	return uuid
}

func statusPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
