//go:build experimental

/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

// Package eventgateway is a compile-time plugin that adds WebSub and WebBroker
// API support to the platform-api server. It is compiled only when the
// "experimental" build tag is set.
package eventgateway

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	eghandler "github.com/wso2/api-platform/platform-api/plugins/eventgateway/handler"
	egrepo "github.com/wso2/api-platform/platform-api/plugins/eventgateway/repository"
	egservice "github.com/wso2/api-platform/platform-api/plugins/eventgateway/service"
)

// eventgatewayArtifactTables lists the artifact-backed tables owned by this plugin.
// They are registered into the shared ArtifactTableRegistry during Init so that
// the core repository UNION queries include them.
var eventgatewayArtifactTables = []repository.ArtifactTableEntry{
	{
		Table:     "websub_apis",
		KindAlias: constants.WebSubApi,
		KindKeys:  []string{"websub-api", constants.WebSubApi},
	},
	{
		Table:     "webbroker_apis",
		KindAlias: constants.WebBrokerApi,
		KindKeys:  []string{"webbroker-api", constants.WebBrokerApi},
	},
}

//go:embed openapi.yaml
var openapiSpec []byte

//go:embed schema/schema.sqlite.sql
var schemaSQLite []byte

//go:embed schema/schema.postgres.sql
var schemaPostgres []byte

//go:embed schema/schema.sqlserver.sql
var schemaSQLServer []byte

// EventGatewayPlugin is the compile-time plugin for WebSub and WebBroker APIs.
type EventGatewayPlugin struct {
	cfg *config.Server

	websubAPIRepo    *egrepo.WebSubAPIRepo
	webbrokerAPIRepo *egrepo.WebBrokerAPIRepo
	hmacSecretRepo   *egrepo.WebSubAPIHmacSecretRepo
	hmacSecretSvc    *egservice.WebSubAPIHmacSecretService

	websubAPIHandler           *eghandler.WebSubAPIHandler
	websubAPIDeploymentHandler *eghandler.WebSubAPIDeploymentHandler
	websubAPIHmacSecretHandler *eghandler.WebSubAPIHmacSecretHandler
	websubAPIKeyHandler        *eghandler.WebSubAPIKeyHandler
	webbrokerAPIHandler        *eghandler.WebBrokerAPIHandler
	webbrokerDeploymentHandler *eghandler.WebBrokerAPIDeploymentHandler
	webbrokerAPIKeyHandler     *eghandler.WebBrokerAPIKeyHandler
}

// New returns a new EventGatewayPlugin instance.
func New() *EventGatewayPlugin {
	return &EventGatewayPlugin{}
}

// Name returns the plugin identifier.
func (p *EventGatewayPlugin) Name() string {
	return "eventgateway"
}

// Init wires all repositories, services, and handlers for the event gateway plugin.
func (p *EventGatewayPlugin) Init(deps *plugin.Deps) error {
	db := deps.DB
	cfg := deps.Config
	p.cfg = cfg
	logger := deps.Logger
	apiUtil := &utils.APIUtil{}

	// Register this plugin's artifact-backed tables so core UNION queries include them.
	for _, entry := range eventgatewayArtifactTables {
		deps.ArtifactTableRegistry.Register(entry)
	}

	// Schema DDL is applied only for SQLite (local/demo deployments). For
	// other drivers the operator must pre-provision the schema; auto-running
	// DDL at startup against an external database is a security risk.
	if db.Driver() == database.DriverSQLite {
		schemaDDL := p.selectSchema(db)
		if schemaDDL != "" {
			if err := db.InitSchemaSQL(schemaDDL, logger); err != nil {
				return fmt.Errorf("eventgateway: failed to apply schema: %w", err)
			}
		}
	}

	// Repositories.
	websubRepo := egrepo.NewWebSubAPIRepo(db, deps.ArtifactTableRegistry)
	webbrokerRepo := egrepo.NewWebBrokerAPIRepo(db, deps.ArtifactTableRegistry)
	hmacSecretRepo := egrepo.NewWebSubAPIHmacSecretRepo(db)

	p.websubAPIRepo = websubRepo
	p.webbrokerAPIRepo = webbrokerRepo
	p.hmacSecretRepo = hmacSecretRepo

	// Services.
	websubAPISvc := egservice.NewWebSubAPIService(
		websubRepo,
		deps.ProjectRepo,
		deps.GatewayRepo,
		deps.GatewayEventsService,
		apiUtil,
		logger,
		deps.AuditRepo,
		cfg,
		deps.IdentityService,
	)

	websubDeploymentSvc := egservice.NewWebSubAPIDeploymentService(
		websubRepo,
		deps.DeploymentRepo,
		deps.GatewayRepo,
		deps.OrgRepo,
		deps.ArtifactRepo,
		deps.APIRepo,
		deps.APIKeyRepo,
		deps.GatewayEventsService,
		cfg,
		logger,
	)

	webbrokerAPISvc := egservice.NewWebBrokerAPIService(
		webbrokerRepo,
		deps.ProjectRepo,
		deps.GatewayRepo,
		deps.GatewayEventsService,
		apiUtil,
		logger,
		deps.AuditRepo,
		cfg,
		deps.IdentityService,
	)

	webbrokerDeploymentSvc := egservice.NewWebBrokerAPIDeploymentService(
		webbrokerRepo,
		deps.DeploymentRepo,
		deps.GatewayRepo,
		deps.OrgRepo,
		deps.ArtifactRepo,
		deps.APIRepo,
		deps.APIKeyRepo,
		deps.GatewayEventsService,
		cfg,
		logger,
	)

	// HMAC secret service — optional; warn and continue if disabled.
	hmacSecretSvc, err := egservice.NewWebSubAPIHmacSecretService(
		hmacSecretRepo,
		websubRepo,
		deps.GatewayEventsService,
		deps.GatewayRepo,
		deps.DBEncryptionKey,
		logger,
	)
	if err != nil {
		logger.Warn("eventgateway: HMAC secret service disabled", "reason", err.Error())
		hmacSecretSvc = nil
	}
	p.hmacSecretSvc = hmacSecretSvc

	// Handlers.
	p.websubAPIHandler = eghandler.NewWebSubAPIHandler(websubAPISvc, deps.IdentityService, logger)
	p.websubAPIDeploymentHandler = eghandler.NewWebSubAPIDeploymentHandler(websubDeploymentSvc, deps.IdentityService, logger)
	p.websubAPIHmacSecretHandler = eghandler.NewWebSubAPIHmacSecretHandler(hmacSecretSvc, deps.IdentityService, logger)
	p.websubAPIKeyHandler = eghandler.NewWebSubAPIKeyHandler(websubAPISvc, deps.APIKeyService, deps.IdentityService, logger)
	p.webbrokerAPIHandler = eghandler.NewWebBrokerAPIHandler(webbrokerAPISvc, deps.IdentityService, logger)
	p.webbrokerDeploymentHandler = eghandler.NewWebBrokerAPIDeploymentHandler(webbrokerDeploymentSvc, deps.IdentityService, logger)
	p.webbrokerAPIKeyHandler = eghandler.NewWebBrokerAPIKeyHandler(webbrokerAPISvc, deps.APIKeyService, deps.IdentityService, logger)

	return nil
}

// RegisterRoutes adds all event gateway HTTP routes to the shared mux.
func (p *EventGatewayPlugin) RegisterRoutes(mux *http.ServeMux) {
	p.websubAPIHandler.RegisterRoutes(mux)
	p.websubAPIDeploymentHandler.RegisterRoutes(mux)
	p.websubAPIHmacSecretHandler.RegisterRoutes(mux)
	p.websubAPIKeyHandler.RegisterRoutes(mux)
	p.webbrokerAPIHandler.RegisterRoutes(mux)
	p.webbrokerDeploymentHandler.RegisterRoutes(mux)
	p.webbrokerAPIKeyHandler.RegisterRoutes(mux)
}

// OpenAPISpec returns the embedded OpenAPI YAML for scope enforcement.
func (p *EventGatewayPlugin) OpenAPISpec() []byte {
	return openapiSpec
}

// Shutdown performs any cleanup required during graceful server shutdown.
func (p *EventGatewayPlugin) Shutdown(_ context.Context) error {
	return nil
}

// GetWebSubAPIRepo satisfies the EventArtifactPlugin interface.
func (p *EventGatewayPlugin) GetWebSubAPIRepo() repository.WebSubAPIRepository {
	return p.websubAPIRepo
}

// GetWebBrokerAPIRepo satisfies the EventArtifactPlugin interface.
func (p *EventGatewayPlugin) GetWebBrokerAPIRepo() repository.WebBrokerAPIRepository {
	return p.webbrokerAPIRepo
}

// GetHmacSecretService satisfies the EventArtifactPlugin interface.
func (p *EventGatewayPlugin) GetHmacSecretService() plugin.HmacSecretServicer {
	if p.hmacSecretSvc == nil {
		return nil
	}
	return p.hmacSecretSvc
}

// CheckProjectDeletion implements service.ProjectDeletionGuard.
// It blocks deletion when WebSub or WebBroker APIs are still associated with the project.
func (p *EventGatewayPlugin) CheckProjectDeletion(orgID, projectID string) error {
	websubCount, err := p.websubAPIRepo.CountByProject(orgID, projectID)
	if err != nil {
		return err
	}
	if websubCount > 0 {
		return constants.ErrProjectHasAssociatedWebSubAPIs
	}
	webbrokerCount, err := p.webbrokerAPIRepo.CountByProject(orgID, projectID)
	if err != nil {
		return err
	}
	if webbrokerCount > 0 {
		return constants.ErrProjectHasAssociatedWebBrokerAPIs
	}
	return nil
}

// EnrichSubscription implements service.OrgSubscriptionEnricher.
// It adds the WebSub API quota to the organization subscription response.
func (p *EventGatewayPlugin) EnrichSubscription(orgID string, sub *api.OrganizationSubscription) error {
	count, err := p.websubAPIRepo.Count(orgID)
	if err != nil {
		return err
	}
	// A limit <= 0 means unlimited: report only usage, leaving Limit/Remaining
	// unset so consumers treat the WebSub API quota as uncapped.
	quota := &api.OrganizationQuota{Used: count}
	if limit := p.cfg.ArtifactLimits.MaxWebSubAPIsPerOrg; limit > 0 {
		quota.Limit = intPtr(limit)
		quota.Remaining = intPtr(max(limit-count, 0))
	}
	sub.Quotas.WebsubApis = quota
	return nil
}

func intPtr(v int) *int { return &v }

// selectSchema returns the DDL appropriate for the current database driver.
func (p *EventGatewayPlugin) selectSchema(db *database.DB) string {
	driver := strings.ToLower(db.Driver())
	switch driver {
	case database.DriverSQLite:
		return string(schemaSQLite)
	case database.DriverPostgres, database.DriverPGX, database.DriverPostgreSQL:
		return string(schemaPostgres)
	case database.DriverSQLServer, database.DriverMSSQL:
		return string(schemaSQLServer)
	default:
		return ""
	}
}

// Compile-time assertions that EventGatewayPlugin satisfies the required interfaces.
var _ plugin.Plugin = (*EventGatewayPlugin)(nil)
var _ plugin.EventArtifactPlugin = (*EventGatewayPlugin)(nil)

// Compile-time assertion that WebSubAPIHmacSecretService satisfies HmacSecretServicer.
var _ plugin.HmacSecretServicer = (*egservice.WebSubAPIHmacSecretService)(nil)
