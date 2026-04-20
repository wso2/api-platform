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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
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

// mapRenderError checks whether err wraps a *templateengine.RenderError and, if so,
// writes a 400 response with a user-friendly message and returns true.
// operation is used for metrics labelling (e.g. "create", "update").
// Returns false when err is not a RenderError, allowing the caller to proceed.
func mapRenderError(c *gin.Context, operation string, err error) bool {
	var renderErr *templateengine.RenderError
	if !errors.As(err, &renderErr) {
		return false
	}
	metrics.ValidationErrorsTotal.WithLabelValues(operation, "render_failed").Inc()
	var secretErr *funcs.SecretError
	if errors.As(renderErr, &secretErr) {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: secretErr.Error(),
		})
		return true
	}
	var tmplParseErr *templateengine.TemplateParseError
	if errors.As(renderErr, &tmplParseErr) {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: tmplParseErr.Error(),
		})
		return true
	}
	c.JSON(http.StatusBadRequest, api.ErrorResponse{
		Status:  "error",
		Message: fmt.Sprintf("Failed to render configuration: %v", renderErr.Cause),
	})
	return true
}
