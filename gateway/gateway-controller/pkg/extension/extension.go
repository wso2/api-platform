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

// Package extension defines the build-time extension interface for the gateway-controller.
// Consumers (e.g. event-gateway-controller) import this package, implement Extension,
// and pass their implementation to server.Run so that additional REST API surfaces,
// event processors, and auth roles are registered at startup without modifying the core.
package extension

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/webhooksecretxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// Services exposes all shared infrastructure built during gateway-controller startup.
// Extensions receive this struct so they can build their own handlers and processors
// on top of the core without reconstructing dependencies.
type Services struct {
	// Core storage (excluding webhook-secret operations — those belong to extensions).
	Storage     storage.Storage
	ConfigStore *storage.ConfigStore
	GatewayID   string
	Logger      *slog.Logger
	Config      *config.Config

	// xDS managers.
	SnapshotManager *xds.SnapshotManager
	PolicyManager   *policyxds.PolicyManager

	// EventHub for publishing and subscribing to replica-sync events.
	EventHub eventhub.EventHub

	// Encryption and secret management.
	SecretService             *secrets.SecretService
	EncryptionProviderManager *encryption.ProviderManager

	// Webhook-secret xDS infrastructure (store is populated by the extension's Initialize).
	WebhookSecretStore           *webhooksecret.WebhookSecretStore
	WebhookSecretSnapshotManager *webhooksecretxds.SnapshotManager

	// Pre-built core services — extensions can use these directly instead of rebuilding them.
	APIDeploymentService *utils.APIDeploymentService
	APIKeyService        *utils.APIKeyService
	APIKeyXDSManager     *apikeyxds.APIKeyStateManager
	ControlPlaneClient   controlplane.ControlPlaneClient
	RouterConfig         *config.RouterConfig
	Validator            config.Validator
	PolicyDefinitions    map[string]models.PolicyDefinition
}

// Extension is implemented by build-time plugins such as event-gateway-controller.
// The gateway-controller's server.Run calls each registered Extension in order during startup.
type Extension interface {
	// Initialize is called after core infrastructure (DB, encryption, xDS servers) is ready
	// but before HTTP routes are registered. Use it for DB schema migrations, loading initial
	// state into shared stores (e.g. webhook secrets), or any other pre-serve setup.
	Initialize(svc *Services) error

	// RegisterRoutes registers additional HTTP routes on the Gin router.
	// Called after core routes are registered, before the HTTP server starts serving.
	RegisterRoutes(router *gin.Engine, svc *Services)

	// RegisterEventProcessors registers additional EventHub event processors on the listener.
	// Called after the EventListener is created but before Start().
	RegisterEventProcessors(listener *eventlistener.EventListener, svc *Services)

	// AuthRoles returns additional route → roles mappings merged into the auth middleware config.
	// Keys use the format "METHOD /relative-path", e.g. "POST /websub-apis".
	// An empty map (or nil) is valid when no additional auth rules are needed.
	AuthRoles() map[string][]string
}
