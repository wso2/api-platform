/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/handler"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	internalvault "github.com/wso2/api-platform/platform-api/internal/vault"
	"github.com/wso2/api-platform/platform-api/internal/webhook"
	"github.com/wso2/api-platform/platform-api/internal/websocket"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/go-httpkit/httputil"
	gohttpkit "github.com/wso2/go-httpkit/middleware"
)

type Server struct {
	mux            *http.ServeMux
	handler        http.Handler // mux wrapped with the full middleware chain
	orgRepo        repository.OrganizationRepository
	projRepo       repository.ProjectRepository
	apiRepo        repository.APIRepository
	gatewayRepo    repository.GatewayRepository
	wsManager      *websocket.Manager // WebSocket connection manager
	timeoutService *service.DeploymentTimeoutService
	dispatcher     *service.EventDispatcher
	eventHub       eventhub.EventHub
	logger         *slog.Logger
}

// validateServerConfig enforces request-security requirements at startup that
// are not covered by config-load validation. All checks are unconditional:
// there is no relaxed/demo mode.
func validateServerConfig(cfg *config.Server) error {
	if slices.Contains(cfg.Listeners.CORS.AllowedOrigins, "*") {
		return fmt.Errorf("cors.allowed_origins must not contain \"*\"; list explicit origins, or leave it empty to disable cross-origin access")
	}
	return nil
}

// StartPlatformAPIServer creates a new server instance with all dependencies initialized
func StartPlatformAPIServer(cfg *config.Server, slogger *slog.Logger) (*Server, error) {
	if err := validateServerConfig(cfg); err != nil {
		slogger.Error("Invalid server configuration", "error", err)
		return nil, err
	}

	// Initialize database using configuration
	db, err := database.NewConnection(&cfg.Database, slogger)
	if err != nil {
		slogger.Error("Failed to connect to database", "error", err)
		return nil, err
	}

	// Schema DDL is executed only for SQLite, which is used for local/demo
	// deployments. For all other (server) drivers the schema must be
	// pre-provisioned by the operator; auto-running DDL against an external
	// database at startup is a security risk.
	if strings.ToLower(cfg.Database.Driver) == "sqlite3" {
		if err := db.InitSchema(cfg.DBSchemaPath, slogger); err != nil {
			slogger.Error("Failed to initialize database schema", "error", err)
			return nil, err
		}
	} else {
		slogger.Debug("Skipping schema DDL execution — schema must be pre-provisioned", "driver", cfg.Database.Driver)
	}

	// Initialize the artifact table registry. Core tables are seeded here; plugins
	// extend it during Init before any request is served.
	artifactTableRegistry := repository.NewArtifactTableRegistry()

	// Initialize repositories
	orgRepo := repository.NewOrganizationRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	apiRepo := repository.NewAPIRepo(db)
	appRepo := repository.NewApplicationRepo(db, artifactTableRegistry)
	gatewayRepo := repository.NewGatewayRepo(db)
	customPolicyRepo := repository.NewCustomPolicyRepo(db)
	artifactRepo := repository.NewArtifactRepo(db, artifactTableRegistry)
	deploymentRepo := repository.NewDeploymentRepo(db, artifactTableRegistry)
	subscriptionRepo := repository.NewSubscriptionRepo(db)
	subscriptionPlanRepo := repository.NewSubscriptionPlanRepo(db)
	llmTemplateRepo := repository.NewLLMProviderTemplateRepo(db)
	llmProviderRepo := repository.NewLLMProviderRepo(db)
	llmProxyRepo := repository.NewLLMProxyRepo(db)
	mcpProxyRepo := repository.NewMCPProxyRepo(db)
	apiKeyRepo := repository.NewAPIKeyRepo(db, artifactTableRegistry)
	auditRepo := repository.NewAuditRepo(db)
	secretRepo := repository.NewSecretRepo(db)
	userIdentityMappingRepo := repository.NewUserIdentityMappingRepo(db)
	userOrgMappingRepo := repository.NewUserOrganizationMappingRepo(db)

	// Seed the file-based organization on startup if file auth mode is selected.
	if cfg.Auth.Mode == config.AuthModeFile {
		if err := seedFileBasedOrg(cfg, orgRepo, slogger); err != nil {
			return nil, fmt.Errorf("failed to seed file-based organization: %w", err)
		}
	}

	// Seed default LLM provider templates into the DB (per organization)
	cfg.LLMTemplateDefinitionsPath = strings.TrimSpace(cfg.LLMTemplateDefinitionsPath)
	defaultTemplates, err := utils.LoadLLMProviderTemplatesFromDirectory(cfg.LLMTemplateDefinitionsPath)
	if err != nil {
		slogger.Warn("Failed to load default LLM provider templates", "path", cfg.LLMTemplateDefinitionsPath, "error", err)
		cleanPath := filepath.Clean(cfg.LLMTemplateDefinitionsPath)
		fallbackPath := ""
		if cleanPath != "" && cleanPath != "." && cleanPath != "src" && !filepath.IsAbs(cleanPath) && !strings.HasPrefix(cleanPath, "src"+string(os.PathSeparator)) {
			fallbackPath = filepath.Join("src", cleanPath)
		}
		if fallbackPath != "" {
			if templates, fallbackErr := utils.LoadLLMProviderTemplatesFromDirectory(fallbackPath); fallbackErr == nil {
				defaultTemplates = templates
				cfg.LLMTemplateDefinitionsPath = fallbackPath
				err = nil
			} else {
				slogger.Warn("Failed to load default LLM provider templates", "path", fallbackPath, "error", fallbackErr)
			}
		}
		if err != nil {
			slogger.Warn("Failed to load default LLM provider templates", "path", cfg.LLMTemplateDefinitionsPath, "error", err)
		}
	}
	llmTemplateSeeder := service.NewLLMTemplateSeeder(llmTemplateRepo, defaultTemplates)
	if len(defaultTemplates) > 0 {
		const pageSize = 200
		offset := 0
		for {
			orgs, listErr := orgRepo.ListOrganizations(pageSize, offset)
			if listErr != nil {
				slogger.Warn("Failed to list organizations for LLM template seeding", "error", listErr)
				break
			}
			if len(orgs) == 0 {
				break
			}
			for _, org := range orgs {
				if org == nil || org.ID == "" {
					continue
				}
				if seedErr := llmTemplateSeeder.SeedForOrg(org.ID); seedErr != nil {
					slogger.Warn("Failed to seed LLM templates for organization", "orgID", org.ID, "error", seedErr)
				}
			}
			offset += pageSize
		}
		slogger.Info("Seeded default LLM provider templates", "count", len(defaultTemplates))
	}

	// Initialize WebSocket manager first (needed for GatewayEventsService)
	wsConfig := websocket.ManagerConfig{
		MaxConnections:     cfg.Listeners.WebSocket.MaxConnections,
		HeartbeatInterval:  20 * time.Second,
		HeartbeatTimeout:   time.Duration(cfg.Listeners.WebSocket.ConnectionTimeout) * time.Second,
		MetricsLogEnabled:  cfg.Listeners.WebSocket.MetricsLogEnabled,
		MetricsLogInterval: time.Duration(cfg.Listeners.WebSocket.MetricsLogInterval) * time.Second,
	}
	wsManager := websocket.NewManager(wsConfig, slogger)

	// Initialize EventHub for multi-replica HA event delivery.
	// Events published here are polled by all platform-api instances; each instance
	// delivers them to gateway WebSocket connections it holds locally.
	eventHub := eventhub.New(db.DB, slogger, eventhub.Config{
		PollInterval:    cfg.EventHub.PollInterval,
		CleanupInterval: cfg.EventHub.CleanupInterval,
		RetentionPeriod: cfg.EventHub.RetentionPeriod,
	})
	if err := eventHub.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize event hub: %w", err)
	}
	dispatcher := service.NewEventDispatcher(eventHub, wsManager, slogger)
	wsManager.SetConnectionHooks(dispatcher.OnGatewayConnected, dispatcher.OnGatewayDisconnected)

	// Initialize utilities
	apiUtil := &utils.APIUtil{}

	// Initialize services
	identityService := service.NewIdentityService(userIdentityMappingRepo)
	orgService := service.NewOrganizationService(
		orgRepo,
		projectRepo,
		appRepo,
		apiRepo,
		gatewayRepo,
		llmProviderRepo,
		llmProxyRepo,
		mcpProxyRepo,
		llmTemplateSeeder,
		auditRepo,
		userOrgMappingRepo,
		identityService,
		cfg,
		slogger,
	)
	projectService := service.NewProjectService(projectRepo, orgRepo, apiRepo, mcpProxyRepo, appRepo, auditRepo, identityService, slogger)
	gatewayEventsService := service.NewGatewayEventsService(eventHub, identityService, slogger)
	appService := service.NewApplicationService(appRepo, projectRepo, orgRepo, apiRepo, gatewayEventsService, auditRepo, identityService, slogger)
	apiService := service.NewAPIService(apiRepo, projectRepo, orgRepo, gatewayRepo, deploymentRepo,
		subscriptionPlanRepo, customPolicyRepo, gatewayEventsService, apiUtil, slogger, auditRepo, identityService)
	gatewayService := service.NewGatewayService(gatewayRepo, orgRepo, apiRepo, customPolicyRepo, gatewayEventsService, slogger, cfg.Gateway.EnableVersionVerification, cfg.Gateway.EnableFunctionalityTypeVerification, auditRepo, identityService)
	subscriptionService := service.NewSubscriptionService(apiRepo, artifactRepo, subscriptionRepo, subscriptionPlanRepo, orgRepo, gatewayEventsService, auditRepo, slogger)
	subscriptionPlanService := service.NewSubscriptionPlanService(subscriptionPlanRepo, gatewayRepo, orgRepo, gatewayEventsService, auditRepo, slogger)
	internalGatewayService := service.NewGatewayInternalAPIService(apiRepo, subscriptionRepo, subscriptionPlanRepo, llmProviderRepo, llmProxyRepo, mcpProxyRepo, deploymentRepo, gatewayRepo, orgRepo, projectRepo, apiKeyRepo, artifactRepo, secretRepo, cfg, slogger)
	apiKeyService := service.NewAPIKeyService(apiRepo, artifactRepo, apiKeyRepo, gatewayEventsService, auditRepo, cfg.Security.APIKey.HashingAlgorithms, slogger)
	deploymentService := service.NewDeploymentService(apiRepo, artifactRepo, deploymentRepo, gatewayRepo, orgRepo, apiKeyRepo, gatewayEventsService, auditRepo, apiUtil, cfg, slogger)
	llmTemplateService := service.NewLLMProviderTemplateService(llmTemplateRepo, auditRepo, identityService)
	llmProviderService := service.NewLLMProviderService(llmProviderRepo, llmTemplateRepo, orgRepo, llmTemplateSeeder, deploymentRepo, gatewayRepo, gatewayEventsService, slogger, auditRepo, cfg, identityService)
	llmProxyService := service.NewLLMProxyService(llmProxyRepo, llmProviderRepo, projectRepo, deploymentRepo, gatewayRepo, gatewayEventsService, slogger, auditRepo, cfg, identityService)
	mcpProxyService := service.NewMCPProxyService(mcpProxyRepo, projectRepo, deploymentRepo, gatewayRepo, gatewayEventsService, slogger, auditRepo, cfg, identityService)

	// The single configured encryption key (APIP_CP_ENCRYPTION_KEY) is used for all encrypted DB
	// columns (secrets, subscription tokens, WebSub HMAC secrets)
	dbEncryptionKey := cfg.Security.EncryptionKey
	llmProviderDeploymentService := service.NewLLMProviderDeploymentService(
		llmProviderRepo,
		llmTemplateRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
		apiKeyRepo,
		gatewayEventsService,
		cfg,
		slogger,
	)
	llmProviderAPIKeyService := service.NewLLMProviderAPIKeyService(llmProviderRepo, apiRepo, apiKeyRepo, gatewayEventsService, identityService, slogger)
	llmProxyAPIKeyService := service.NewLLMProxyAPIKeyService(llmProxyRepo, apiRepo, apiKeyRepo, gatewayEventsService, identityService, slogger)
	apiKeyUserService := service.NewAPIKeyUserService(apiKeyRepo, identityService, slogger)
	llmProxyDeploymentService := service.NewLLMProxyDeploymentService(
		llmProxyRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
		apiKeyRepo,
		gatewayEventsService,
		cfg,
		slogger,
	)
	mcpDeploymentService := service.NewMCPDeploymentService(
		mcpProxyRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
		artifactRepo,
		apiKeyRepo,
		gatewayEventsService,
		cfg,
		slogger,
	)
	artifactImportService := service.NewArtifactImportService(
		apiRepo,
		llmProviderRepo,
		llmTemplateRepo,
		llmProxyRepo,
		mcpProxyRepo,
		artifactRepo,
		deploymentRepo,
		gatewayRepo,
		projectRepo,
		cfg,
		slogger,
		mcpProxyService,
	)

	// Initialize secret vault and service using the single configured encryption key.
	secretKey, keyErr := utils.DeriveEncryptionKey(cfg.Security.EncryptionKey)
	if keyErr != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", keyErr)
	}
	secretVault, vaultErr := internalvault.NewInHouseVault(secretKey)
	if vaultErr != nil {
		return nil, fmt.Errorf("failed to initialize secret vault: %w", vaultErr)
	}
	secretService := service.NewSecretService(secretRepo, secretVault, identityService)

	// Initialize handlers
	orgHandler := handler.NewOrganizationHandler(orgService, identityService, cfg.Auth.IDP.ValidationMode, slogger)
	projectHandler := handler.NewProjectHandler(projectService, identityService, slogger)
	apiHandler := handler.NewAPIHandler(apiService, identityService, slogger)
	gatewayHandler := handler.NewGatewayHandler(gatewayService, identityService, slogger)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService, subscriptionPlanService, identityService, slogger)
	subscriptionPlanHandler := handler.NewSubscriptionPlanHandler(subscriptionPlanService, identityService, slogger)
	appHandler := handler.NewApplicationHandler(appService, identityService, slogger)
	wsHandler := handler.NewWebSocketHandler(wsManager, gatewayService, deploymentService, cfg.Listeners.WebSocket.RateLimitPerMin, slogger)
	internalGatewayHandler := handler.NewGatewayInternalAPIHandler(gatewayService, internalGatewayService, artifactImportService, secretService, slogger)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService, identityService, slogger)
	deploymentHandler := handler.NewDeploymentHandler(deploymentService, identityService, slogger)
	llmHandler := handler.NewLLMHandler(llmTemplateService, llmProviderService, llmProxyService, identityService, slogger)
	llmDeploymentHandler := handler.NewLLMProviderDeploymentHandler(llmProviderDeploymentService, identityService, slogger)
	llmProviderAPIKeyHandler := handler.NewLLMProviderAPIKeyHandler(llmProviderAPIKeyService, identityService, slogger)
	llmProxyAPIKeyHandler := handler.NewLLMProxyAPIKeyHandler(llmProxyAPIKeyService, identityService, slogger)
	apiKeyUserHandler := handler.NewAPIKeyUserHandler(apiKeyUserService, identityService, slogger)
	llmProxyDeploymentHandler := handler.NewLLMProxyDeploymentHandler(llmProxyDeploymentService, identityService, slogger)
	mcpProxyHandler := handler.NewMCPProxyHandler(mcpProxyService, identityService, slogger)
	mcpProxyDeploymentHandler := handler.NewMCPProxyDeploymentHandler(mcpDeploymentService, identityService, slogger)
	// Wire secret placeholder validation into dependent services
	llmProviderService.SetSecretService(secretService)
	llmProxyService.SetSecretService(secretService)
	mcpProxyService.WithSecretService(secretService)
	apiService.SetSecretService(secretService)
	secretHandler := handler.NewSecretHandler(secretService, identityService, slogger)
	// Start deployment timeout background job
	timeoutConfig := service.DeploymentTimeoutConfig{
		Enabled:  cfg.Deployments.TimeoutEnabled,
		Interval: time.Duration(cfg.Deployments.TimeoutInterval) * time.Second,
		Timeout:  time.Duration(cfg.Deployments.TimeoutDuration) * time.Second,
	}
	timeoutService := service.NewDeploymentTimeoutService(deploymentRepo, timeoutConfig, slogger)

	slogger.Info("Initialized all services and handlers successfully")

	// Setup mux and register all routes.
	mux := http.NewServeMux()

	// Load the OpenAPI scope registry — source of truth for required scopes per route.
	scopeRegistry, err := middleware.LoadScopeRegistry(cfg.OpenAPISpecPath)
	if err != nil {
		slogger.Error("Failed to load OpenAPI scope registry", "path", cfg.OpenAPISpecPath, "error", err)
		return nil, fmt.Errorf("failed to load OpenAPI scope registry: %w", err)
	}
	slogger.Info("Loaded OpenAPI scope registry", "path", cfg.OpenAPISpecPath)

	// Load and validate the role-to-scope map when roles.yaml is configured.
	roleScopeMap, err := loadRoleScopeMap(cfg, scopeRegistry, slogger)
	if err != nil {
		return nil, err
	}

	if !cfg.Auth.ScopeValidation {
		slogger.Warn("scope validation is disabled — all authenticated requests will be allowed regardless of scope")
	}

	// Register all routes on the mux. Public routes (login) are accessible
	// because the auth middleware uses cfg.Auth.SkipPaths to bypass them.
	handler.NewAuthLoginHandler(cfg).RegisterPublicRoutes(mux)
	orgHandler.RegisterRoutes(mux)
	projectHandler.RegisterRoutes(mux)
	appHandler.RegisterRoutes(mux)
	apiHandler.RegisterRoutes(mux)
	gatewayHandler.RegisterRoutes(mux)
	subscriptionHandler.RegisterRoutes(mux)
	subscriptionPlanHandler.RegisterRoutes(mux)
	wsHandler.RegisterRoutes(mux)
	internalGatewayHandler.RegisterRoutes(mux)
	apiKeyHandler.RegisterRoutes(mux)
	deploymentHandler.RegisterRoutes(mux)
	llmHandler.RegisterRoutes(mux)
	llmDeploymentHandler.RegisterRoutes(mux)
	llmProviderAPIKeyHandler.RegisterRoutes(mux)
	llmProxyAPIKeyHandler.RegisterRoutes(mux)
	apiKeyUserHandler.RegisterRoutes(mux)
	llmProxyDeploymentHandler.RegisterRoutes(mux)
	mcpProxyHandler.RegisterRoutes(mux)
	mcpProxyDeploymentHandler.RegisterRoutes(mux)
	secretHandler.RegisterRoutes(mux)

	// Initialize plugins and register their routes.
	// Plugins contribute routes, DB schema, and OpenAPI scopes at startup.
	pluginDeps := &plugin.Deps{
		DB:                    db,
		Config:                cfg,
		Logger:                slogger,
		EventHub:              eventHub,
		ArtifactTableRegistry: artifactTableRegistry,
		GatewayRepo:           gatewayRepo,
		OrgRepo:               orgRepo,
		ProjectRepo:           projectRepo,
		ArtifactRepo:          artifactRepo,
		DeploymentRepo:        deploymentRepo,
		APIKeyRepo:            apiKeyRepo,
		AuditRepo:             auditRepo,
		SecretRepo:            secretRepo,
		APIRepo:               apiRepo,
		GatewayEventsService:  gatewayEventsService,
		APIKeyService:         apiKeyService,
		IdentityService:       identityService,
		DBEncryptionKey:       dbEncryptionKey,
	}
	for _, p := range plugin.All() {
		if err := p.Init(pluginDeps); err != nil {
			return nil, fmt.Errorf("plugin %q failed to initialize: %w", p.Name(), err)
		}
		// Merge plugin-contributed scopes into the main registry.
		if spec := p.OpenAPISpec(); len(spec) > 0 {
			pluginRegistry, regErr := middleware.LoadScopeRegistryFromBytes(spec)
			if regErr != nil {
				slogger.Warn("plugin scope registry load failed", "plugin", p.Name(), "error", regErr)
			} else {
				scopeRegistry.Merge(pluginRegistry)
			}
		}
		p.RegisterRoutes(mux)
		slogger.Info("Plugin initialized", "name", p.Name())
		// Wire plugin-owned repos/services into core services.
		if ep, ok := p.(plugin.EventArtifactPlugin); ok {
			internalGatewayService.SetEventArtifactRepos(ep.GetWebSubAPIRepo(), ep.GetWebBrokerAPIRepo())
			internalGatewayHandler.SetHmacSecretService(ep.GetHmacSecretService())
		}
		if guard, ok := p.(service.ProjectDeletionGuard); ok {
			projectService.RegisterDeletionGuard(guard)
		}
	}
	slogger.Info("Registered API routes successfully")

	// Register the control-plane webhook receiver (Developer Portal -> Platform API) when enabled.
	// Authenticity is established by HMAC signature; the route is excluded from JWT/IDP auth via
	// cfg.Auth.SkipPaths (see config defaults).
	if cfg.Webhook.Enabled {
		webhookDecryptor, err := webhook.NewDecryptor(cfg.Webhook.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize webhook decryptor: %w", err)
		}
		webhookReceiver := webhook.NewReceiver(
			cfg.Webhook,
			webhookDecryptor,
			apiKeyService,
			subscriptionService,
			appService,
			orgRepo,
			slogger,
		)
		webhookReceiver.RegisterRoutes(mux)
		slogger.Info("Webhook receiver enabled", "path", webhook.RoutePath)
	}

	slogger.Info("Registered API routes successfully")

	// Build the middleware chain that wraps the mux.
	// Order: CORS → auth → scope enforcer → mux
	var chain []func(http.Handler) http.Handler

	// Cross-origin access is disabled by default (empty AllowedOrigins fails closed in the
	// CORS middleware); operators must opt in explicitly via CORS.AllowedOrigins in config.
	corsOrigins := cfg.Listeners.CORS.AllowedOrigins
	if len(corsOrigins) == 0 {
		slogger.Warn("cors.allowed_origins not set in config — cross-origin requests are disabled")
	} else if slices.Contains(corsOrigins, "*") {
		slogger.Warn("cors.allowed_origins contains \"*\" — allowing all origins without credentials")
	}
	chain = append(chain, gohttpkit.CORSMiddleware(gohttpkit.CORSOptions{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	if cfg.Auth.Mode == config.AuthModeFile {
		slogger.Info("Auth mode: file (local users, HMAC-signed JWT)")
		slogger.Warn("file-based authentication is enabled — this is not recommended for production; please configure an IDP of your choice")
		chain = append(chain, middleware.LocalJWTAuthMiddleware(middleware.AuthConfig{
			SecretKey:      cfg.Auth.JWT.SecretKey,
			TokenIssuer:    cfg.Auth.JWT.Issuer,
			SkipPaths:      cfg.Auth.SkipPaths,
			SkipValidation: false,
			ClaimMappings:  buildClaimMappings(cfg.Auth.ClaimMappings, roleScopeMap),
		}))
	} else {
		authenticator, err := buildAuthenticator(cfg, slogger, roleScopeMap)
		if err != nil {
			return nil, err
		}
		chain = append(chain, authenticator.Middleware()...)
	}

	// Resolve the organization claim (the platform UUID in file-based mode, or
	// the IDP's organization id in IDP mode) into the platform organization UUID
	// so downstream handlers scope their queries by the correct value.
	chain = append(chain, middleware.OrganizationResolverMiddleware(
		func(orgClaim string) (string, bool) {
			if org, err := orgRepo.GetOrganizationByIdpOrgRefUUID(orgClaim); err == nil && org != nil {
				return org.ID, true
			}
			if org, err := orgRepo.GetOrganizationByUUID(orgClaim); err == nil && org != nil {
				return org.ID, true
			}
			return "", false
		},
	))

	// Apply the OpenAPI-driven scope enforcer after authentication so identity
	// values are already in the context when scope checks run.
	chain = append(chain, middleware.ScopeEnforcer(scopeRegistry, middleware.ScopeEnforcerConfig{
		ValidationMode: cfg.Auth.IDP.ValidationMode,
		Enabled:        cfg.Auth.ScopeValidation,
	}))

	slogger.Info("WebSocket manager initialized",
		slog.Int("maxConnections", cfg.Listeners.WebSocket.MaxConnections),
		slog.Int("heartbeatTimeout", cfg.Listeners.WebSocket.ConnectionTimeout),
		slog.Int("rateLimitPerMin", cfg.Listeners.WebSocket.RateLimitPerMin),
	)

	return &Server{
		mux:            mux,
		handler:        gohttpkit.Chain(chain...)(mux),
		orgRepo:        orgRepo,
		projRepo:       projectRepo,
		apiRepo:        apiRepo,
		gatewayRepo:    gatewayRepo,
		wsManager:      wsManager,
		timeoutService: timeoutService,
		dispatcher:     dispatcher,
		eventHub:       eventHub,
		logger:         slogger,
	}, nil
}

// buildClaimMappings adapts the config-level claim name mapping (shared by all
// three auth modes) into the middleware package's ClaimMappings, attaching the
// resolved IDP-role-to-scope table alongside it.
func buildClaimMappings(cm config.ClaimMappings, roleScopeMap map[string][]string) middleware.ClaimMappings {
	return middleware.ClaimMappings{
		OrganizationClaim: cm.Organization,
		OrgNameClaim:      cm.OrgName,
		OrgHandleClaim:    cm.OrgHandle,
		UserIDClaim:       cm.UserID,
		UsernameClaim:     cm.Username,
		EmailClaim:        cm.Email,
		ScopeClaim:        cm.Scope,
		RolesClaimPath:    cm.Roles,
		RoleScopeMap:      roleScopeMap,
	}
}

// buildAuthenticator constructs an Authenticator from the server configuration.
// Only called when the auth mode is "external_token" or "idp" (file mode wires
// its own local-JWT middleware).
func buildAuthenticator(cfg *config.Server, slogger *slog.Logger, roleScopeMap map[string][]string) (middleware.Authenticator, error) {
	if cfg.Auth.Mode != config.AuthModeIDP {
		slogger.Info("Auth mode: jwt (HMAC signature validation enabled)")
		return middleware.NewJWTAuthenticator(
			middleware.LocalJWTAuthMiddleware(middleware.AuthConfig{
				SecretKey:      cfg.Auth.JWT.SecretKey,
				TokenIssuer:    cfg.Auth.JWT.Issuer,
				SkipPaths:      cfg.Auth.SkipPaths,
				SkipValidation: false,
				ClaimMappings:  buildClaimMappings(cfg.Auth.ClaimMappings, roleScopeMap),
			}),
		), nil
	}

	// IDP mode — same config fields for all providers (Thunder, Keycloak, Asgardeo, etc.)
	issuerURL := ""
	if len(cfg.Auth.IDP.Issuer) > 0 {
		issuerURL = cfg.Auth.IDP.Issuer[0]
	}
	idpCfg := commonmodels.IDPConfig{
		Enabled:    true,
		IssuerURL:  issuerURL,
		JWKSUrl:    cfg.Auth.IDP.JWKSUrl,
		ScopeClaim: cfg.Auth.ClaimMappings.Scope,
	}
	// Enforce audience validation only when at least one audience is configured.
	if len(cfg.Auth.IDP.Audience) > 0 {
		idpCfg.Audience = &cfg.Auth.IDP.Audience
	}
	authCfg := commonmodels.AuthConfig{
		JWTConfig: &idpCfg,
		SkipPaths: cfg.Auth.SkipPaths,
	}
	authMiddleware, err := authenticators.AuthMiddleware(authCfg, slogger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IDP auth middleware: %w", err)
	}
	claimsMiddleware := middleware.PlatformClaimsMiddleware(buildClaimMappings(cfg.Auth.ClaimMappings, roleScopeMap))

	idpLabel := cfg.Auth.IDP.Name
	if idpLabel == "" {
		idpLabel = "IDP"
	}
	slogger.Info("IDP authentication enabled",
		slog.String("name", idpLabel),
		slog.String("jwksUrl", cfg.Auth.IDP.JWKSUrl),
		slog.Any("issuers", cfg.Auth.IDP.Issuer),
	)
	return middleware.NewJWTAuthenticator(
		authMiddleware,
		claimsMiddleware,
	), nil
}

// loadRoleScopeMap loads the role-to-scope mapping for IDP role mode.
// Returns nil when role mode is not active or no mapping file is configured,
// which causes IDP role names to be used as-is as scope values (passthrough).
func loadRoleScopeMap(cfg *config.Server, registry *middleware.ScopeRegistry, slogger *slog.Logger) (map[string][]string, error) {
	if cfg.Auth.Mode != config.AuthModeIDP || cfg.Auth.IDP.ValidationMode != "role" || cfg.Auth.IDP.RoleMappings == "" {
		return nil, nil
	}

	m, err := middleware.LoadRoleScopeMap(cfg.Auth.IDP.RoleMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to load role mappings file: %w", err)
	}
	if err := middleware.ValidateRoleScopeMap(m, registry); err != nil {
		return nil, fmt.Errorf("invalid roles.yaml: %w", err)
	}
	slogger.Info("Loaded role-to-scope mapping", "path", cfg.Auth.IDP.RoleMappings, "roles", len(m))

	return m, nil
}

// buildTLSConfig resolves the TLS listener configuration. The caller invokes it
// only when the HTTPS listener is enabled. Certificates are always required —
// there is no self-signed fallback; use the quickstart setup script (or your
// own tooling) to generate a pair and mount it.
func (s *Server) buildTLSConfig(httpsCfg config.HTTPSListener) (*tls.Config, error) {
	certFile := httpsCfg.TLS.CertFile
	keyFile := httpsCfg.TLS.KeyFile
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("HTTPS listener enabled but server.https.tls.cert_file / server.https.tls.key_file is not configured")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to load TLS certificates (cert %q / key %q): %w. "+
				"Mount a certificate pair and point server.https.tls.cert_file / key_file at it, "+
				"or set server.https.enabled=false to serve plain HTTP behind a TLS-terminating proxy",
			certFile, keyFile, err,
		)
	}
	s.logger.Info("Using mounted certificates", "certFile", certFile, "keyFile", keyFile)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// Start brings up the enabled listeners. The plain-HTTP and TLS listeners are
// independent: either or both may run, each on its own port. This mirrors the
// gateway router's http/https listener split. At least one listener must be
// enabled.
//
// timeouts bounds connection lifetime on both listeners so a slow or idle peer
// cannot hold one open indefinitely (Slowloris). It is validated at config load.
func (s *Server) Start(listeners config.ServerListeners, timeouts config.Timeouts) error {
	httpCfg := listeners.HTTP
	httpsCfg := listeners.HTTPS
	if !httpCfg.Enabled && !httpsCfg.Enabled {
		s.logger.Error("No listeners enabled")
		return fmt.Errorf("no listeners enabled: set server.http.enabled=true and/or server.https.enabled=true in config")
	}

	// Preflight: validate listener configuration and build the TLS config before
	// starting any listener, background job, or goroutine
	if httpCfg.Enabled && (httpCfg.Port <= 0 || httpCfg.Port > 65535) {
		return fmt.Errorf("HTTP listener enabled but server.http.port is invalid (got %d)", httpCfg.Port)
	}
	var tlsConfig *tls.Config
	if httpsCfg.Enabled {
		if httpsCfg.Port <= 0 || httpsCfg.Port > 65535 {
			return fmt.Errorf("HTTPS listener enabled but server.https.port is invalid (got %d)", httpsCfg.Port)
		}
		var err error
		if tlsConfig, err = s.buildTLSConfig(httpsCfg); err != nil {
			return err
		}
	}

	// Add a health endpoint. Routes added to s.mux after startup are reachable
	// because s.handler wraps s.mux by reference.
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.timeoutService.Start(ctx)

	// errCh is buffered for both listeners so a failing goroutine never blocks,
	// even while the other listener is still being shut down.
	errCh := make(chan error, 2)
	var httpServers []*http.Server

	// Plain-HTTP listener.
	if httpCfg.Enabled {
		// Plain HTTP is only safe when something upstream terminates TLS, or for
		// internal traffic. Say so loudly.
		s.logger.Warn("Plain-HTTP listener is enabled (server.http.enabled=true); " +
			"terminate TLS at an ingress or service-mesh sidecar and never expose this listener " +
			"directly to untrusted networks.")
		httpPort := strconv.Itoa(httpCfg.Port)
		httpServer := &http.Server{
			Addr:              ":" + httpPort,
			Handler:           s.handler,
			ReadHeaderTimeout: timeouts.ReadHeader,
			ReadTimeout:       timeouts.Read,
			WriteTimeout:      timeouts.Write,
			IdleTimeout:       timeouts.Idle,
		}
		httpServers = append(httpServers, httpServer)
		s.logger.Info("Starting HTTP listener", "address", "http://localhost:"+httpPort)
		go func() {
			errCh <- httpServer.ListenAndServe()
		}()
	}

	// TLS listener.
	if httpsCfg.Enabled {
		httpsPort := strconv.Itoa(httpsCfg.Port)
		httpsServer := &http.Server{
			Addr:              ":" + httpsPort,
			Handler:           s.handler,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: timeouts.ReadHeader,
			ReadTimeout:       timeouts.Read,
			WriteTimeout:      timeouts.Write,
			IdleTimeout:       timeouts.Idle,
		}
		httpServers = append(httpServers, httpsServer)
		s.logger.Info("Starting HTTPS listener", "address", "https://localhost:"+httpsPort)
		go func() {
			errCh <- httpsServer.ListenAndServeTLS("", "")
		}()
	}

	s.logger.Info("Platform API started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	teardown := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		for _, srv := range httpServers {
			if err := srv.Shutdown(shutdownCtx); err != nil {
				s.logger.Error("HTTP server shutdown error", "addr", srv.Addr, "error", err)
			}
		}
		for _, p := range plugin.All() {
			if err := p.Shutdown(shutdownCtx); err != nil {
				s.logger.Error("Plugin shutdown error", "plugin", p.Name(), "error", err)
			}
		}
		s.wsManager.Shutdown()
		s.dispatcher.Shutdown()
		if err := s.eventHub.Close(); err != nil {
			s.logger.Error("EventHub close error", "error", err)
		}
	}

	select {
	case err := <-errCh:
		cancel()
		teardown()
		// A graceful Shutdown surfaces http.ErrServerClosed on the listener
		// goroutine; that is a clean exit, not a startup failure.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case sig := <-quit:
		s.logger.Info("Received shutdown signal", "signal", sig)
		cancel()
		teardown()
		return nil
	}
}

// GetMux returns the raw ServeMux for testing purposes.
func (s *Server) GetMux() *http.ServeMux {
	return s.mux
}

// seedFileBasedOrg ensures the file-based auth organization exists in the DB.
// It fetches by the configured handle (Organization.ID) first; only creates the
// org when no matching org is found. The organization's idp_organization_ref_uuid
// is stored back into cfg (Organization.UUID) so the login handler issues tokens
// whose `organization` claim matches the value the organization resolver looks up.
func seedFileBasedOrg(cfg *config.Server, orgRepo repository.OrganizationRepository, slogger *slog.Logger) error {
	ba := &cfg.Auth.File

	existing, err := orgRepo.GetOrganizationByHandle(ba.Organization.ID)
	if err != nil {
		return fmt.Errorf("failed to check file-based organization: %w", err)
	}
	if existing != nil {
		// The `organization` claim is resolved via idp_organization_ref_uuid, so
		// carry that value (not the PK) even though they coincide for file-based orgs.
		ba.Organization.UUID = existing.IdpOrganizationRefUUID
		slogger.Info("File-based organization already exists", "uuid", existing.ID, "handle", existing.Handle)
		return nil
	}

	// Honor an operator-pinned UUID (config/env) so the org — and therefore the
	// `organization` claim — stays stable across restarts and fresh databases;
	// generate one only when none is configured.
	uuid := ba.Organization.UUID
	if uuid == "" {
		uuid, err = utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate file-based organization UUID: %w", err)
		}
	}

	now := time.Now()
	org := &model.Organization{
		ID:     uuid,
		Name:   ba.Organization.DisplayName,
		Handle: ba.Organization.ID,
		Region: ba.Organization.Region,
		// File-based auth has no external IDP, so the organization references
		// itself: the org claim in issued tokens is this UUID, and the resolver
		// matches it via the same idp_organization_ref_uuid path as IDP orgs.
		IdpOrganizationRefUUID: uuid,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := orgRepo.CreateOrganization(org); err != nil {
		return fmt.Errorf("failed to create file-based organization: %w", err)
	}
	ba.Organization.UUID = uuid
	slogger.Info("Seeded file-based organization", "uuid", org.ID, "handle", org.Handle)
	return nil
}
