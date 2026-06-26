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

// Package extension implements the event-gateway build-time extension for gateway-controller.
package extension

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
	gwextension "github.com/wso2/api-platform/gateway/gateway-controller/pkg/extension"
	evhandlers "github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/handlers"
	"github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/eventprocessors"
	"github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/service"
	evstorage "github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/storage"
)

const managementAPIBasePath = "/api/management/v1"

// EventGatewayExtension implements gateway-controller's extension.Extension interface.
type EventGatewayExtension struct {
	db  evstorage.EventStorage
	svc *gwextension.Services
	wss *service.WebhookSecretService
}

// NewEventGatewayExtension returns a new EventGatewayExtension ready to pass to server.Run.
func NewEventGatewayExtension() *EventGatewayExtension {
	return &EventGatewayExtension{}
}

// Initialize is called after core infrastructure is ready.
// It sets up event storage, loads webhook secrets from the database, and creates the service.
func (e *EventGatewayExtension) Initialize(svc *gwextension.Services) error {
	dbType := svc.Config.Controller.Storage.Type
	rawDB := svc.Storage.GetDB()

	e.svc = svc
	e.db = evstorage.NewEventSQLStore(rawDB, svc.GatewayID, dbType)

	if err := e.db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize event storage: %w", err)
	}

	e.wss = service.NewWebhookSecretService(
		e.db,
		svc.WebhookSecretStore,
		svc.WebhookSecretSnapshotManager,
		svc.EncryptionProviderManager,
		svc.EventHub,
		svc.GatewayID,
		svc.Logger,
	)

	if svc.EncryptionProviderManager != nil {
		if err := loadWebhookSecretsFromDatabase(e.db, svc); err != nil {
			svc.Logger.Error("Failed to load webhook secrets from database", slog.Any("error", err))
			return fmt.Errorf("failed to load webhook secrets from database: %w", err)
		}
	}

	return nil
}

// RegisterRoutes registers all event API HTTP routes on the gin router.
func (e *EventGatewayExtension) RegisterRoutes(router *gin.Engine, svc *gwextension.Services) {
	eventServer := evhandlers.NewEventAPIServer(svc, e.wss)
	eventServer.RegisterRoutes(router, managementAPIBasePath)
}

// RegisterEventProcessors registers the webhook secret event processor on the listener.
func (e *EventGatewayExtension) RegisterEventProcessors(listener *eventlistener.EventListener, svc *gwextension.Services) {
	processor := eventprocessors.NewWebhookSecretProcessor(
		e.db,
		svc.WebhookSecretStore,
		svc.WebhookSecretSnapshotManager,
		svc.EncryptionProviderManager,
		svc.Logger,
	)
	listener.RegisterProcessor(processor)
}

// AuthRoles returns the route → roles authorization mappings for event API routes.
func (e *EventGatewayExtension) AuthRoles() map[string][]string {
	return map[string][]string{
		"POST /websub-apis":       {"admin", "developer"},
		"GET /websub-apis":        {"admin", "developer"},
		"GET /websub-apis/:id":    {"admin", "developer"},
		"PUT /websub-apis/:id":    {"admin", "developer"},
		"DELETE /websub-apis/:id": {"admin", "developer"},

		"POST /webbroker-apis":       {"admin", "developer"},
		"GET /webbroker-apis":        {"admin", "developer"},
		"GET /webbroker-apis/:id":    {"admin", "developer"},
		"PUT /webbroker-apis/:id":    {"admin", "developer"},
		"DELETE /webbroker-apis/:id": {"admin", "developer"},

		"POST /websub-apis/:id/api-keys":                        {"admin", "consumer"},
		"GET /websub-apis/:id/api-keys":                         {"admin", "consumer"},
		"PUT /websub-apis/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /websub-apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /websub-apis/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		"POST /websub-apis/:id/secrets":                        {"admin", "consumer"},
		"GET /websub-apis/:id/secrets":                         {"admin", "consumer"},
		"DELETE /websub-apis/:id/secrets/:secretName":          {"admin", "consumer"},
		"POST /websub-apis/:id/secrets/:secretName/regenerate": {"admin", "consumer"},

		"POST /webbroker-apis/:id/api-keys":                        {"admin", "consumer"},
		"GET /webbroker-apis/:id/api-keys":                         {"admin", "consumer"},
		"PUT /webbroker-apis/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /webbroker-apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /webbroker-apis/:id/api-keys/:apiKeyName":          {"admin", "consumer"},
	}
}

// loadWebhookSecretsFromDatabase bulk-loads all webhook secrets from the DB into the in-memory store.
func loadWebhookSecretsFromDatabase(db evstorage.EventStorage, svc *gwextension.Services) error {
	secrets, err := db.GetAllWebhookSecrets()
	if err != nil {
		return err
	}
	for _, ws := range secrets {
		payload, err := encryption.UnmarshalPayload(string(ws.Ciphertext))
		if err != nil {
			svc.Logger.Warn("Failed to unmarshal ciphertext for webhook secret",
				slog.String("secret_uuid", ws.UUID), slog.Any("error", err))
			continue
		}
		plaintext, err := svc.EncryptionProviderManager.Decrypt(payload)
		if err != nil {
			svc.Logger.Warn("Failed to decrypt webhook secret",
				slog.String("secret_uuid", ws.UUID), slog.Any("error", err))
			continue
		}
		if err := svc.WebhookSecretStore.Store(ws.ArtifactUUID, ws.Name, string(plaintext)); err != nil {
			svc.Logger.Warn("Failed to store webhook secret in memory", slog.Any("error", err))
		}
	}
	if svc.WebhookSecretSnapshotManager != nil {
		if err := svc.WebhookSecretSnapshotManager.RefreshSnapshot(); err != nil {
			svc.Logger.Warn("Failed to refresh webhook secret snapshot after load", slog.Any("error", err))
		}
	}
	svc.Logger.Info("Loaded webhook secrets from database", slog.Int("count", len(secrets)))
	return nil
}
