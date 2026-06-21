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

// Package controllerext defines the extensibility seam between the base gateway-controller
// and optional feature modules (such as the event-gateway-controller).
package controllerext

import (
	"context"
	"log/slog"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/webhooksecretxds"
)

// ExtensionDeps holds the core components passed to every extension hook.
// All fields are populated by the base controller before calling extension methods.
type ExtensionDeps struct {
	DB                        storage.Storage
	ConfigStore               *storage.ConfigStore
	Log                       *slog.Logger
	Config                    *config.Config
	EncryptionProviderManager *encryption.ProviderManager // nil when no encryption providers configured
	EventHub                  eventhub.EventHub
	GatewayID                 string
}

// NamedXDSCache pairs a debug label with an xDS cache contributed by an extension.
// The policyxds server adds all entries to its CombinedCache without needing to know
// what kind of resources the cache contains.
type NamedXDSCache struct {
	Name  string
	Cache cache.Cache
}

// EventGatewayWiring carries the event-gateway-specific managers created by an
// extension's InitXDS. The base controller (pkg/bootstrap) passes these values
// to controlplane.NewClient and handlers.NewAPIServer, which accept nil-safe pointers
// and interfaces for these fields. NoOpExtension leaves all fields as zero (nil).
type EventGatewayWiring struct {
	// SubscriptionSnapshotUpdater is used by the control plane client and API server
	// to refresh the subscription xDS snapshot after subscription changes.
	SubscriptionSnapshotUpdater utils.SubscriptionSnapshotUpdater
	// WebhookSecretStore is the in-memory store for decrypted webhook secrets.
	WebhookSecretStore *webhooksecret.WebhookSecretStore
	// WebhookSecretSnapshotManager pushes webhook secret updates to xDS clients.
	WebhookSecretSnapshotManager *webhooksecretxds.SnapshotManager
	// WebhookSecretService exposes webhook secret CRUD for the REST API.
	WebhookSecretService *utils.WebhookSecretService
}

// ExtensionXDS holds the optional xDS caches produced by an extension during InitXDS,
// plus optional wiring components for base controller call sites.
// The base controller and this package have no knowledge of what the caches contain;
// only the extension that creates them knows their semantics.
type ExtensionXDS struct {
	ExtraCaches    []NamedXDSCache
	EventGatewayWiring EventGatewayWiring
}

// ExtraEventProcessor handles one or more eventhub.EventType values that the base
// EventListener does not cover. Extensions register processors via ExtraEventProcessors.
type ExtraEventProcessor interface {
	HandlesEventType(t eventhub.EventType) bool
	Process(ctx context.Context, event eventhub.Event)
}

// ControllerExtension is the extensibility seam between the base controller and
// optional feature sets. Implement this interface in a separate Go module and pass
// the implementation to RunController to add new capabilities without modifying
// the base controller source.
type ControllerExtension interface {
	// Name returns a short identifier used in logs.
	Name() string

	// AdditionalSchemaSQL returns zero or more SQL strings to execute against the
	// database after the base schema has been applied. Use this to create
	// extension-specific tables. backend is "sqlite" or "postgres". Return nil when
	// no extra schema is needed.
	AdditionalSchemaSQL(backend string) []string

	// InitXDS creates extension-owned xDS managers. Called once, after storage is
	// initialised and before startup hydration. The returned ExtensionXDS caches
	// are registered with the policyxds server via WithExtraCache options.
	InitXDS(ctx context.Context, deps ExtensionDeps) (*ExtensionXDS, error)

	// LoadOnStartup seeds extension in-memory state from the database. Called after
	// encryption providers are initialised so secrets can be decrypted at startup.
	LoadOnStartup(ctx context.Context, deps ExtensionDeps, xds *ExtensionXDS) error

	// ExtraEventProcessors returns event processors to register with the EventListener
	// for event types the base listener does not handle.
	ExtraEventProcessors(deps ExtensionDeps, xds *ExtensionXDS) []ExtraEventProcessor

	// RegisterRoutes mounts extension-owned HTTP handlers onto the Gin router.
	// Called after the base routes are registered.
	RegisterRoutes(router *gin.Engine, deps ExtensionDeps, xds *ExtensionXDS) error

	// AdditionalResourceRoles returns extra method+path → role mappings to merge into
	// the auth middleware configuration. Use this to protect extension-owned routes
	// with the same role system as the base controller. Return nil when unused.
	AdditionalResourceRoles() map[string][]string

	// Shutdown is called during graceful shutdown so the extension can release resources.
	Shutdown(ctx context.Context)
}

// NoOpExtension satisfies ControllerExtension with empty implementations.
// The base gateway-controller uses this when no extension is registered.
type NoOpExtension struct{}

func (NoOpExtension) Name() string { return "noop" }
func (NoOpExtension) AdditionalSchemaSQL(_ string) []string { return nil }
func (NoOpExtension) InitXDS(_ context.Context, _ ExtensionDeps) (*ExtensionXDS, error) {
	return &ExtensionXDS{}, nil
}
func (NoOpExtension) LoadOnStartup(_ context.Context, _ ExtensionDeps, _ *ExtensionXDS) error {
	return nil
}
func (NoOpExtension) ExtraEventProcessors(_ ExtensionDeps, _ *ExtensionXDS) []ExtraEventProcessor {
	return nil
}
func (NoOpExtension) RegisterRoutes(_ *gin.Engine, _ ExtensionDeps, _ *ExtensionXDS) error {
	return nil
}
func (NoOpExtension) AdditionalResourceRoles() map[string][]string { return nil }
func (NoOpExtension) Shutdown(_ context.Context)                   {}
