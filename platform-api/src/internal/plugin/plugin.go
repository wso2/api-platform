/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Package plugin defines the compile-time pluggable module system for platform-api.
//
// Plugins are registered at startup via init() functions gated by Go build tags.
// Each plugin receives shared platform infrastructure (DB, config, event hub, core
// repositories and services) and wires up its own handlers, services, and schema.
//
// Build tags:
//   - OSS build (default):        go build ./...
//   - Experimental build:         go build -tags experimental ./...
package plugin

import (
	"context"
	"log/slog"
	"net/http"

	"platform-api/src/config"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"

	"github.com/wso2/api-platform/common/eventhub"
)

// Plugin is the interface that all compile-time pluggable modules must implement.
type Plugin interface {
	// Name returns a short identifier for the plugin (e.g. "eventgateway").
	Name() string

	// Init receives shared platform infrastructure and wires the plugin's own
	// internal components (repos, services, handlers). Called once at startup
	// before routes are registered.
	Init(deps *Deps) error

	// RegisterRoutes adds the plugin's HTTP routes to the shared mux.
	// Only called after Init has succeeded.
	RegisterRoutes(mux *http.ServeMux)

	// OpenAPISpec returns the plugin's OpenAPI 3.x YAML bytes used to merge
	// scope requirements into the platform scope registry. Return nil if the
	// plugin registers no routes that require scope enforcement.
	OpenAPISpec() []byte

	// Shutdown is called during graceful server shutdown.
	Shutdown(ctx context.Context) error
}

// Deps is the set of shared platform dependencies injected into every plugin at
// startup. Plugins must treat all fields as read-only infrastructure and must not
// replace or modify the shared objects.
type Deps struct {
	DB       *database.DB
	Config   *config.Server
	Logger   *slog.Logger
	EventHub eventhub.EventHub

	// ArtifactTableRegistry allows plugins to register their own artifact-backed
	// tables so that cross-table UNION queries in the core repos include them.
	ArtifactTableRegistry *repository.ArtifactTableRegistry

	// Core repositories shared with plugins (interfaces).
	GatewayRepo    repository.GatewayRepository
	OrgRepo        repository.OrganizationRepository
	ProjectRepo    repository.ProjectRepository
	ArtifactRepo   repository.ArtifactRepository
	DeploymentRepo repository.DeploymentRepository
	APIKeyRepo     repository.APIKeyRepository
	AuditRepo      repository.AuditRepository
	SecretRepo     repository.SecretRepository
	APIRepo        repository.APIRepository

	// Core services shared with plugins.
	GatewayEventsService *service.GatewayEventsService
	APIKeyService        *service.APIKeyService

	// DBEncryptionKey is the derived hex key used for encrypted DB columns.
	DBEncryptionKey string
}

// HmacSecretServicer is the minimal interface for WebSub API HMAC secret
// operations used by the internal gateway handler. Defined here (neutral package)
// to avoid an import cycle between internal/handler and plugins/.
type HmacSecretServicer interface {
	ListByArtifactUUID(artifactUUID string) ([]*model.WebSubAPIHmacSecret, error)
	DecryptSecret(s *model.WebSubAPIHmacSecret) (string, error)
}

// EventArtifactPlugin is an optional interface that event-related plugins may
// implement to contribute their repositories back to core platform services
// (GatewayInternalAPIService, ProjectService, OrganizationService).
//
// The server type-asserts each registered plugin to this interface after Init
// and calls the appropriate setter methods on the core services.
type EventArtifactPlugin interface {
	Plugin
	GetWebSubAPIRepo() repository.WebSubAPIRepository
	GetWebBrokerAPIRepo() repository.WebBrokerAPIRepository
	GetHmacSecretService() HmacSecretServicer
}
