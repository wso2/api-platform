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

package eventgateway

import (
	"context"
	_ "embed"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controllerext"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/webhooksecretxds"
)

//go:embed sql/event-gateway-db.sql
var sqliteSchema string

//go:embed sql/event-gateway-db.postgres.sql
var postgresSchema string

// Extension implements controllerext.ControllerExtension and adds WebSub/WebBroker
// subscription management and webhook HMAC secret management to the base gateway-controller.
type Extension struct {
	subscriptionSnapshotManager  *subscriptionxds.SnapshotManager
	webhookSecretStore           *webhooksecret.WebhookSecretStore
	webhookSecretSnapshotManager *webhooksecretxds.SnapshotManager
	webhookSecretService         *utils.WebhookSecretService
}

// NewExtension creates a new EventGateway Extension.
func NewExtension() *Extension {
	return &Extension{}
}

func (e *Extension) Name() string { return "event-gateway" }

// AdditionalSchemaSQL returns the event-gateway DDL for the given backend ("sqlite" or "postgres").
func (e *Extension) AdditionalSchemaSQL(backend string) []string {
	if backend == "postgres" {
		return []string{postgresSchema}
	}
	return []string{sqliteSchema}
}

// InitXDS creates the subscription and webhook-secret xDS managers and populates the
// EventGatewayWiring fields that the base controller's controlplane client and API server
// consume through nil-safe pointer checks.
func (e *Extension) InitXDS(ctx context.Context, deps controllerext.ExtensionDeps) (*controllerext.ExtensionXDS, error) {
	e.subscriptionSnapshotManager = subscriptionxds.NewSnapshotManager(deps.DB, deps.Log)

	e.webhookSecretStore = webhooksecret.GetStoreInstance()
	e.webhookSecretSnapshotManager = webhooksecretxds.NewSnapshotManager(e.webhookSecretStore, deps.Log)

	if deps.EncryptionProviderManager != nil {
		e.webhookSecretService = utils.NewWebhookSecretService(
			deps.DB,
			deps.EncryptionProviderManager,
			e.webhookSecretStore,
			deps.EventHub,
			deps.GatewayID,
			deps.Log,
		)
	} else {
		deps.Log.Warn("No encryption providers configured; webhook secret service unavailable")
	}

	return &controllerext.ExtensionXDS{
		ExtraCaches: []controllerext.NamedXDSCache{
			{Name: "subscription", Cache: e.subscriptionSnapshotManager.GetCache()},
			{Name: "webhook-secret", Cache: e.webhookSecretSnapshotManager.GetCache()},
		},
		EventGatewayWiring: controllerext.EventGatewayWiring{
			SubscriptionSnapshotUpdater:  e.subscriptionSnapshotManager,
			WebhookSecretStore:           e.webhookSecretStore,
			WebhookSecretSnapshotManager: e.webhookSecretSnapshotManager,
			WebhookSecretService:         e.webhookSecretService,
		},
	}, nil
}

// LoadOnStartup seeds in-memory state from the database. Webhook secrets are decrypted
// and loaded into the store; the subscription snapshot is built from active subscriptions.
func (e *Extension) LoadOnStartup(ctx context.Context, deps controllerext.ExtensionDeps, _ *controllerext.ExtensionXDS) error {
	if deps.EncryptionProviderManager != nil {
		if err := storage.LoadWebhookSecretsFromDatabase(deps.DB, deps.EncryptionProviderManager, e.webhookSecretStore); err != nil {
			return err
		}
		if err := e.webhookSecretSnapshotManager.RefreshSnapshot(); err != nil {
			deps.Log.Warn("Failed to build initial webhook secret xDS snapshot", slog.Any("error", err))
		}
	}

	loadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return e.subscriptionSnapshotManager.UpdateSnapshot(loadCtx)
}

// ExtraEventProcessors returns processors for subscription and webhook-secret events.
func (e *Extension) ExtraEventProcessors(deps controllerext.ExtensionDeps, _ *controllerext.ExtensionXDS) []controllerext.ExtraEventProcessor {
	return []controllerext.ExtraEventProcessor{
		NewSubscriptionProcessor(e.subscriptionSnapshotManager, deps.Log),
		NewWebhookSecretProcessor(deps.DB, e.webhookSecretStore, e.webhookSecretSnapshotManager, deps.EncryptionProviderManager, deps.Log),
	}
}

// RegisterRoutes is a no-op for now; event-gateway routes are already registered by the
// base controller's APIServer (which accepts nil-safe wiring for webhook secret operations).
func (e *Extension) RegisterRoutes(_ *gin.Engine, _ controllerext.ExtensionDeps, _ *controllerext.ExtensionXDS) error {
	return nil
}

// AdditionalResourceRoles returns nil because subscription/webhook-secret routes are
// already included in the base auth config.
func (e *Extension) AdditionalResourceRoles() map[string][]string { return nil }

// Shutdown is a no-op; snapshot managers and stores do not hold external connections.
func (e *Extension) Shutdown(_ context.Context) {}
