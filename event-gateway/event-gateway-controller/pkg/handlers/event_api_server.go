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
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/constants"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/common/eventhub"
	gwapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/extension"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/service"
)

// EventAPIServer handles WebSub and WebBroker API HTTP routes.
type EventAPIServer struct {
	svc                  *extension.Services
	webhookSecretService *service.WebhookSecretService
	apiKeyService        *utils.APIKeyService
	httpClient           *http.Client
}

// NewEventAPIServer creates a new EventAPIServer.
func NewEventAPIServer(
	svc *extension.Services,
	webhookSecretService *service.WebhookSecretService,
) *EventAPIServer {
	return &EventAPIServer{
		svc:                  svc,
		webhookSecretService: webhookSecretService,
		apiKeyService: utils.NewAPIKeyService(
			svc.ConfigStore, svc.Storage, svc.APIKeyXDSManager,
			&svc.Config.APIKey, svc.EventHub, svc.GatewayID,
		),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterRoutes registers all event API HTTP routes on the provided gin router.
func (s *EventAPIServer) RegisterRoutes(router *gin.Engine, basePath string) {
	mg := router.Group(basePath)
	mg.POST("/websub-apis", s.CreateWebSubAPI)
	mg.GET("/websub-apis", s.ListWebSubAPIs)
	mg.GET("/websub-apis/:id", s.GetWebSubAPIById)
	mg.PUT("/websub-apis/:id", s.UpdateWebSubAPI)
	mg.DELETE("/websub-apis/:id", s.DeleteWebSubAPI)
	mg.POST("/websub-apis/:id/api-keys", s.CreateWebSubAPIKey)
	mg.GET("/websub-apis/:id/api-keys", s.ListWebSubAPIKeys)
	mg.PUT("/websub-apis/:id/api-keys/:apiKeyName", s.UpdateWebSubAPIKey)
	mg.POST("/websub-apis/:id/api-keys/:apiKeyName/regenerate", s.RegenerateWebSubAPIKey)
	mg.DELETE("/websub-apis/:id/api-keys/:apiKeyName", s.RevokeWebSubAPIKey)
	mg.POST("/websub-apis/:id/secrets", s.CreateWebSubAPISecret)
	mg.GET("/websub-apis/:id/secrets", s.ListWebSubAPISecrets)
	mg.POST("/websub-apis/:id/secrets/:secretName/regenerate", s.RegenerateWebSubAPISecret)
	mg.DELETE("/websub-apis/:id/secrets/:secretName", s.DeleteWebSubAPISecret)

	mg.POST("/webbroker-apis", s.CreateWebBrokerAPI)
	mg.GET("/webbroker-apis", s.ListWebBrokerAPIs)
	mg.GET("/webbroker-apis/:id", s.GetWebBrokerAPIById)
	mg.PUT("/webbroker-apis/:id", s.UpdateWebBrokerAPI)
	mg.DELETE("/webbroker-apis/:id", s.DeleteWebBrokerAPI)
	mg.POST("/webbroker-apis/:id/api-keys", s.CreateWebBrokerAPIKey)
	mg.GET("/webbroker-apis/:id/api-keys", s.ListWebBrokerAPIKeys)
	mg.PUT("/webbroker-apis/:id/api-keys/:apiKeyName", s.UpdateWebBrokerAPIKey)
	mg.POST("/webbroker-apis/:id/api-keys/:apiKeyName/regenerate", s.RegenerateWebBrokerAPIKey)
	mg.DELETE("/webbroker-apis/:id/api-keys/:apiKeyName", s.RevokeWebBrokerAPIKey)
}

func (s *EventAPIServer) extractAuthenticatedUser(c *gin.Context, operation string, correlationID string) (*commonmodels.AuthContext, bool) {
	val, exists := c.Get(constants.AuthContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gwapi.ErrorResponse{Status: "error", Message: "Authentication context not available"})
		return nil, false
	}
	user, ok := val.(commonmodels.AuthContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Invalid authentication context"})
		return nil, false
	}
	return &user, true
}

func (s *EventAPIServer) bindRequestBody(c *gin.Context, request interface{}) error {
	ct := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)
	if ct == "application/yaml" || ct == "text/yaml" {
		return c.ShouldBindYAML(request)
	}
	return c.ShouldBindJSON(request)
}

func buildResourceResponse(cfg any, stored *models.StoredConfig) any {
	id := stored.Handle
	state := gwapi.ResourceStatusState(stored.DesiredState)
	createdAt := stored.CreatedAt
	updatedAt := stored.UpdatedAt
	status := gwapi.ResourceStatus{
		Id:        &id,
		State:     &state,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if stored.DeployedAt != nil {
		deployedAt := *stored.DeployedAt
		status.DeployedAt = &deployedAt
	}
	switch v := cfg.(type) {
	case gwapi.WebSubAPI:
		v.Status = &status
		return v
	case gwapi.WebBrokerApi:
		v.Status = &status
		return v
	}
	return cfg
}

func (s *EventAPIServer) publishEvent(eventType eventhub.EventType, action, entityID, correlationID string, log *slog.Logger) error {
	event := eventhub.Event{
		EventType: eventType,
		Action:    action,
		EntityID:  entityID,
		EventID:   correlationID,
		EventData: eventhub.EmptyEventData,
	}
	if err := s.svc.EventHub.PublishEvent(s.svc.GatewayID, event); err != nil {
		log.Error("Failed to publish event",
			slog.String("event_type", string(eventType)),
			slog.Any("error", err))
		return err
	}
	return nil
}
