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

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// CreateWebSubAPI implements ServerInterface.CreateWebSubAPI
// (POST /websub-apis)
func (s *APIServer) CreateWebSubAPI(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// ListWebSubAPIs implements ServerInterface.ListWebSubAPIs
// (GET /websub-apis)
func (s *APIServer) ListWebSubAPIs(c *gin.Context, params api.ListWebSubAPIsParams) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// GetWebSubAPIById implements ServerInterface.GetWebSubAPIById
// (GET /websub-apis/{id})
func (s *APIServer) GetWebSubAPIById(c *gin.Context, id string) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// UpdateWebSubAPI implements ServerInterface.UpdateWebSubAPI
// (PUT /websub-apis/{id})
func (s *APIServer) UpdateWebSubAPI(c *gin.Context, id string) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// DeleteWebSubAPI implements ServerInterface.DeleteWebSubAPI
// (DELETE /websub-apis/{id})
func (s *APIServer) DeleteWebSubAPI(c *gin.Context, id string) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}
