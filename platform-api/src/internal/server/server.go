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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"platform-api/src/internal/middleware"
	"strings"
	"syscall"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/database"
	"platform-api/src/internal/handler"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	"platform-api/src/internal/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
)

type Server struct {
	router         *gin.Engine
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

// StartPlatformAPIServer creates a new server instance with all dependencies initialized
func StartPlatformAPIServer(cfg *config.Server, slogger *slog.Logger) (*Server, error) {
	// Initialize database using configuration
	db, err := database.NewConnection(&cfg.Database, slogger)
	if err != nil {
		slogger.Error("Failed to connect to database", "error", err)
		return nil, err
	}

	// Initialize schema (skip when ExecuteSchemaDDL is false, e.g. deployed Postgres without DDL access)
	if cfg.Database.ExecuteSchemaDDL {
		if err := db.InitSchema(cfg.DBSchemaPath, slogger); err != nil {
			slogger.Error("Failed to initialize database schema", "error", err)
			return nil, err
		}
	} else {
		slogger.Debug("Skipping schema DDL execution (DATABASE_EXECUTE_SCHEMA_DDL=false)")
	}

	// Initialize repositories
	orgRepo := repository.NewOrganizationRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	apiRepo := repository.NewAPIRepo(db)
	appRepo := repository.NewApplicationRepo(db)
	gatewayRepo := repository.NewGatewayRepo(db)
	customPolicyRepo := repository.NewCustomPolicyRepo(db)
	artifactRepo := repository.NewArtifactRepo(db)
	devPortalRepo := repository.NewDevPortalRepository(db)
	publicationRepo := repository.NewAPIPublicationRepository(db)
	deploymentRepo := repository.NewDeploymentRepo(db)
	subscriptionRepo := repository.NewSubscriptionRepo(db)
	subscriptionPlanRepo := repository.NewSubscriptionPlanRepo(db)
	llmTemplateRepo := repository.NewLLMProviderTemplateRepo(db)
	llmProviderRepo := repository.NewLLMProviderRepo(db)
	llmProxyRepo := repository.NewLLMProxyRepo(db)
	mcpProxyRepo := repository.NewMCPProxyRepo(db)
	websubAPIRepo := repository.NewWebSubAPIRepo(db)
	webbrokerAPIRepo := repository.NewWebBrokerAPIRepo(db)
	apiKeyRepo := repository.NewAPIKeyRepo(db)

	// Seed the file-based organization on startup if file-based auth mode is enabled.
	if cfg.Auth.FileBased.Enabled {
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
		MaxConnections:       cfg.WebSocket.MaxConnections,
		HeartbeatInterval:    20 * time.Second,
		HeartbeatTimeout:     time.Duration(cfg.WebSocket.ConnectionTimeout) * time.Second,
		MaxConnectionsPerOrg: cfg.WebSocket.MaxConnectionsPerOrg,
		MetricsLogEnabled:    cfg.WebSocket.MetricsLogEnabled,
		MetricsLogInterval:   time.Duration(cfg.WebSocket.MetricsLogInterval) * time.Second,
	}
	wsManager := websocket.NewManager(wsConfig, gatewayRepo, slogger)

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

	// Initialize DevPortal service
	devPortalService := service.NewDevPortalService(devPortalRepo, orgRepo, publicationRepo, apiRepo, apiUtil, cfg, slogger)

	// Initialize services
	orgService := service.NewOrganizationService(
		orgRepo,
		projectRepo,
		appRepo,
		apiRepo,
		gatewayRepo,
		llmProviderRepo,
		llmProxyRepo,
		mcpProxyRepo,
		websubAPIRepo,
		devPortalService,
		llmTemplateSeeder,
		cfg,
		slogger,
	)
	projectService := service.NewProjectService(projectRepo, orgRepo, apiRepo, mcpProxyRepo, websubAPIRepo, slogger)
	gatewayEventsService := service.NewGatewayEventsService(eventHub, slogger)
	appService := service.NewApplicationService(appRepo, projectRepo, orgRepo, apiRepo, gatewayEventsService, slogger)
	apiService := service.NewAPIService(apiRepo, projectRepo, orgRepo, gatewayRepo, deploymentRepo, devPortalRepo, publicationRepo,
		subscriptionPlanRepo, customPolicyRepo, gatewayEventsService, devPortalService, apiUtil, slogger)
	gatewayService := service.NewGatewayService(gatewayRepo, orgRepo, apiRepo, customPolicyRepo, gatewayEventsService, slogger, cfg.Gateway.EnableVersionVerification, cfg.Gateway.EnableFunctionalityTypeVerification)
	subscriptionService := service.NewSubscriptionService(apiRepo, subscriptionRepo, gatewayEventsService, slogger)
	subscriptionPlanService := service.NewSubscriptionPlanService(subscriptionPlanRepo, gatewayRepo, gatewayEventsService, slogger)
	internalGatewayService := service.NewGatewayInternalAPIService(apiRepo, subscriptionRepo, subscriptionPlanRepo, llmProviderRepo, llmProxyRepo, mcpProxyRepo, websubAPIRepo, webbrokerAPIRepo, deploymentRepo, gatewayRepo, orgRepo, projectRepo, apiKeyRepo, artifactRepo, cfg, slogger)
	apiKeyService := service.NewAPIKeyService(apiRepo, apiKeyRepo, gatewayEventsService, cfg.APIKey.HashingAlgorithms, slogger)
	gitService := service.NewGitService()
	deploymentService := service.NewDeploymentService(apiRepo, artifactRepo, deploymentRepo, gatewayRepo, orgRepo, gatewayEventsService, apiUtil, cfg, slogger)
	llmTemplateService := service.NewLLMProviderTemplateService(llmTemplateRepo)
	llmProviderService := service.NewLLMProviderService(llmProviderRepo, llmTemplateRepo, orgRepo, llmTemplateSeeder, deploymentRepo, gatewayRepo, gatewayEventsService, slogger)
	llmProxyService := service.NewLLMProxyService(llmProxyRepo, llmProviderRepo, projectRepo, deploymentRepo, gatewayRepo, gatewayEventsService, slogger)
	mcpProxyService := service.NewMCPProxyService(mcpProxyRepo, projectRepo, deploymentRepo, gatewayRepo, gatewayEventsService, slogger)
	websubAPIService := service.NewWebSubAPIService(websubAPIRepo, projectRepo, gatewayRepo, devPortalService, gatewayEventsService, apiUtil, slogger)
	webbrokerAPIService := service.NewWebBrokerAPIService(webbrokerAPIRepo, projectRepo, gatewayRepo, devPortalService, gatewayEventsService, apiUtil, slogger)

	// Initialize the shared database encryption key used for all encrypted DB columns.
	// DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY is accepted as a legacy alias.
	// DeriveEncryptionKey requires 64-char hex or base64-to-32-bytes; when falling back to
	// the raw JWT secret (arbitrary length), hash it to a valid 64-char hex key.
	dbEncryptionKey := cfg.Database.EncryptionKey
	if dbEncryptionKey == "" && cfg.Auth.JWT.SecretKey != "" {
		h := sha256.Sum256([]byte(cfg.Auth.JWT.SecretKey))
		dbEncryptionKey = hex.EncodeToString(h[:])
	}
	hmacSecretRepo := repository.NewWebSubAPIHmacSecretRepo(db)
	hmacSecretService, hmacErr := service.NewWebSubAPIHmacSecretService(
		hmacSecretRepo, websubAPIRepo, gatewayEventsService, gatewayRepo,
		dbEncryptionKey, slogger,
	)
	if hmacErr != nil {
		slogger.Warn("WebSub HMAC secret service disabled — no valid encryption key configured", "error", hmacErr)
	}
	llmProviderDeploymentService := service.NewLLMProviderDeploymentService(
		llmProviderRepo,
		llmTemplateRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
		gatewayEventsService,
		cfg,
		slogger,
	)
	llmProviderAPIKeyService := service.NewLLMProviderAPIKeyService(llmProviderRepo, gatewayRepo, apiKeyRepo, gatewayEventsService, slogger)
	llmProxyAPIKeyService := service.NewLLMProxyAPIKeyService(llmProxyRepo, gatewayRepo, apiKeyRepo, gatewayEventsService, slogger)
	apiKeyUserService := service.NewAPIKeyUserService(apiKeyRepo, slogger)
	llmProxyDeploymentService := service.NewLLMProxyDeploymentService(
		llmProxyRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
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
		gatewayEventsService,
		cfg,
		slogger,
	)
	websubAPIDeploymentService := service.NewWebSubAPIDeploymentService(
		websubAPIRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
		artifactRepo,
		apiRepo,
		gatewayEventsService,
		cfg,
		slogger,
	)
	webbrokerAPIDeploymentService := service.NewWebBrokerAPIDeploymentService(
		webbrokerAPIRepo,
		deploymentRepo,
		gatewayRepo,
		orgRepo,
		artifactRepo,
		apiRepo,
		gatewayEventsService,
		cfg,
		slogger,
	)

	// Initialize handlers
	orgHandler := handler.NewOrganizationHandler(orgService, cfg.OrgCreationRequiresAuth, slogger)
	projectHandler := handler.NewProjectHandler(projectService, slogger)
	apiHandler := handler.NewAPIHandler(apiService, slogger)
	devPortalHandler := handler.NewDevPortalHandler(devPortalService, slogger)
	gatewayHandler := handler.NewGatewayHandler(gatewayService, slogger)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService, subscriptionPlanService, slogger)
	subscriptionPlanHandler := handler.NewSubscriptionPlanHandler(subscriptionPlanService, slogger)
	appHandler := handler.NewApplicationHandler(appService, slogger)
	wsHandler := handler.NewWebSocketHandler(wsManager, gatewayService, deploymentService, cfg.WebSocket.RateLimitPerMin, slogger)
	internalGatewayHandler := handler.NewGatewayInternalAPIHandler(gatewayService, internalGatewayService, hmacSecretService, slogger)
	hmacSecretHandler := handler.NewWebSubAPIHmacSecretHandler(hmacSecretService, slogger)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService, slogger)
	gitHandler := handler.NewGitHandler(gitService, slogger)
	deploymentHandler := handler.NewDeploymentHandler(deploymentService, slogger)
	llmHandler := handler.NewLLMHandler(llmTemplateService, llmProviderService, llmProxyService, slogger)
	llmDeploymentHandler := handler.NewLLMProviderDeploymentHandler(llmProviderDeploymentService, slogger)
	llmProviderAPIKeyHandler := handler.NewLLMProviderAPIKeyHandler(llmProviderAPIKeyService, slogger)
	llmProxyAPIKeyHandler := handler.NewLLMProxyAPIKeyHandler(llmProxyAPIKeyService, slogger)
	apiKeyUserHandler := handler.NewAPIKeyUserHandler(apiKeyUserService, slogger)
	llmProxyDeploymentHandler := handler.NewLLMProxyDeploymentHandler(llmProxyDeploymentService, slogger)
	mcpProxyHandler := handler.NewMCPProxyHandler(mcpProxyService, slogger)
	mcpProxyDeploymentHandler := handler.NewMCPProxyDeploymentHandler(mcpDeploymentService, slogger)
	websubAPIHandler := handler.NewWebSubAPIHandler(websubAPIService, slogger)
	websubAPIKeyHandler := handler.NewWebSubAPIKeyHandler(websubAPIService, apiKeyService, slogger)
	websubAPIDeploymentHandler := handler.NewWebSubAPIDeploymentHandler(websubAPIDeploymentService, slogger)
	webbrokerAPIHandler := handler.NewWebBrokerAPIHandler(webbrokerAPIService, slogger)
	webbrokerAPIKeyHandler := handler.NewWebBrokerAPIKeyHandler(webbrokerAPIService, apiKeyService, slogger)
	webbrokerAPIDeploymentHandler := handler.NewWebBrokerAPIDeploymentHandler(webbrokerAPIDeploymentService, slogger)
	// Start deployment timeout background job
	timeoutConfig := service.DeploymentTimeoutConfig{
		Enabled:  cfg.Deployments.TimeoutEnabled,
		Interval: time.Duration(cfg.Deployments.TimeoutInterval) * time.Second,
		Timeout:  time.Duration(cfg.Deployments.TimeoutDuration) * time.Second,
	}
	timeoutService := service.NewDeploymentTimeoutService(deploymentRepo, timeoutConfig, slogger)

	slogger.Info("Initialized all services and handlers successfully")
	slogger.Info("Platform API configuration", slog.Bool("demoMode", demoMode()))

	if strings.ToLower(cfg.LogLevel) == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Setup router
	router := gin.Default()

	// Configure and apply CORS middleware first (before auth middleware)
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

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

	if !cfg.EnableScopeValidation {
		slogger.Warn("scope validation is disabled — all authenticated requests will be allowed regardless of scope")
	}

	if !cfg.OrgCreationRequiresAuth && !demoMode() {
		slogger.Warn("WARNING: organization creation endpoint is public — any unauthenticated caller can create organizations; set ORG_CREATION_REQUIRES_AUTH=true for production")
	}

	// Register public routes before auth middleware so they bypass authentication.
	orgHandler.RegisterPublicRoutes(router)
	handler.NewAuthLoginHandler(cfg).RegisterPublicRoutes(router)

	// Build and apply the authenticator middleware.
	if cfg.Auth.FileBased.Enabled {
		slogger.Info("Auth mode: file-based (HMAC-signed JWT)")
		if !demoMode() {
			slogger.Warn("file-based authentication is enabled — this is not recommended for production; please configure an IDP of your choice")
		}
		router.Use(middleware.LocalJWTAuthMiddleware(middleware.AuthConfig{
			SecretKey:      cfg.Auth.JWT.SecretKey,
			TokenIssuer:    cfg.Auth.JWT.Issuer,
			SkipPaths:      cfg.Auth.SkipPaths,
			SkipValidation: false,
		}))
	} else {
		authenticator, err := buildAuthenticator(cfg, slogger, roleScopeMap)
		if err != nil {
			return nil, err
		}
		for _, mw := range authenticator.Middleware() {
			router.Use(mw)
		}
	}

	// Apply the OpenAPI-driven scope enforcer after authentication so identity
	// values are already in the context when scope checks run.
	router.Use(middleware.ScopeEnforcer(scopeRegistry, middleware.ScopeEnforcerConfig{
		ValidationMode: cfg.Auth.IDP.ValidationMode,
		Enabled:        cfg.EnableScopeValidation,
	}))

	// Register routes
	orgHandler.RegisterRoutes(router)
	projectHandler.RegisterRoutes(router)
	appHandler.RegisterRoutes(router)
	apiHandler.RegisterRoutes(router)
	devPortalHandler.RegisterRoutes(router)
	gatewayHandler.RegisterRoutes(router)
	subscriptionHandler.RegisterRoutes(router)
	subscriptionPlanHandler.RegisterRoutes(router)
	wsHandler.RegisterRoutes(router)
	internalGatewayHandler.RegisterRoutes(router)
	apiKeyHandler.RegisterRoutes(router)
	gitHandler.RegisterRoutes(router)
	deploymentHandler.RegisterRoutes(router)
	llmHandler.RegisterRoutes(router)
	llmDeploymentHandler.RegisterRoutes(router)
	llmProviderAPIKeyHandler.RegisterRoutes(router)
	llmProxyAPIKeyHandler.RegisterRoutes(router)
	apiKeyUserHandler.RegisterRoutes(router)
	llmProxyDeploymentHandler.RegisterRoutes(router)
	mcpProxyHandler.RegisterRoutes(router)
	mcpProxyDeploymentHandler.RegisterRoutes(router)
	websubAPIHandler.RegisterRoutes(router)
	websubAPIKeyHandler.RegisterRoutes(router)
	websubAPIDeploymentHandler.RegisterRoutes(router)
	hmacSecretHandler.RegisterRoutes(router)
	webbrokerAPIHandler.RegisterRoutes(router)
	webbrokerAPIKeyHandler.RegisterRoutes(router)
	webbrokerAPIDeploymentHandler.RegisterRoutes(router)
	slogger.Info("Registered API routes successfully")

	slogger.Info("WebSocket manager initialized",
		slog.Int("maxConnections", cfg.WebSocket.MaxConnections),
		slog.Int("heartbeatTimeout", cfg.WebSocket.ConnectionTimeout),
		slog.Int("rateLimitPerMin", cfg.WebSocket.RateLimitPerMin),
		slog.Int("maxConnectionsPerOrg", cfg.WebSocket.MaxConnectionsPerOrg),
	)

	return &Server{
		router:         router,
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

// demoMode reports whether APIP_DEMO_MODE is enabled.
// Defaults to true when the variable is unset.
func demoMode() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("APIP_DEMO_MODE")))
	if v == "" {
		return true
	}
	return v == "true" || v == "1"
}

// buildAuthenticator constructs an Authenticator from the server configuration.
// Only called when file-based auth is disabled.
func buildAuthenticator(cfg *config.Server, slogger *slog.Logger, roleScopeMap map[string][]string) (middleware.Authenticator, error) {
	if !cfg.Auth.IDP.Enabled {
		if cfg.Auth.JWT.SkipValidation {
			if !demoMode() {
				slogger.Warn("WARNING: JWT signature validation is DISABLED (AUTH_JWT_SKIP_VALIDATION=true) but APIP_DEMO_MODE=false. " +
					"Tokens are NOT verified — any bearer value will be accepted. " +
					"Set APIP_DEMO_MODE=true to suppress this warning, or set AUTH_JWT_SKIP_VALIDATION=false for production.")
			} else {
				slogger.Warn("JWT mode: signature validation disabled (AUTH_JWT_SKIP_VALIDATION=true) [APIP_DEMO_MODE=true]")
			}
		} else {
			slogger.Info("JWT mode: HMAC signature validation enabled")
		}
		return middleware.NewJWTAuthenticator(
			middleware.LocalJWTAuthMiddleware(middleware.AuthConfig{
				SecretKey:      cfg.Auth.JWT.SecretKey,
				TokenIssuer:    cfg.Auth.JWT.Issuer,
				SkipPaths:      cfg.Auth.SkipPaths,
				SkipValidation: cfg.Auth.JWT.SkipValidation,
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
		ScopeClaim: cfg.Auth.IDP.ClaimMappings.ScopeClaimName,
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
	claimsMiddleware := middleware.PlatformClaimsMiddleware(middleware.PlatformClaimNames{
		OrganizationClaim: cfg.Auth.IDP.ClaimMappings.OrganizationClaimName,
		OrgNameClaim:      cfg.Auth.IDP.ClaimMappings.OrgNameClaimName,
		OrgHandleClaim:    cfg.Auth.IDP.ClaimMappings.OrgHandleClaimName,
		UserIDClaim:       cfg.Auth.IDP.ClaimMappings.UserIDClaimName,
		UsernameClaim:     cfg.Auth.IDP.ClaimMappings.UsernameClaimName,
		EmailClaim:        cfg.Auth.IDP.ClaimMappings.EmailClaimName,
		ScopeClaim:        cfg.Auth.IDP.ClaimMappings.ScopeClaimName,
		RolesClaimPath:    cfg.Auth.IDP.ClaimMappings.RolesClaimPath,
		RoleScopeMap:      roleScopeMap,
	})

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
	if !cfg.Auth.IDP.Enabled || cfg.Auth.IDP.ValidationMode != "role" || cfg.Auth.IDP.RoleMappingsFile == "" {
		return nil, nil
	}

	m, err := middleware.LoadRoleScopeMap(cfg.Auth.IDP.RoleMappingsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load role mappings file: %w", err)
	}
	if err := middleware.ValidateRoleScopeMap(m, registry); err != nil {
		return nil, fmt.Errorf("invalid roles.yaml: %w", err)
	}
	slogger.Info("Loaded role-to-scope mapping", "path", cfg.Auth.IDP.RoleMappingsFile, "roles", len(m))

	return m, nil
}

// generateSelfSignedCert creates a self-signed certificate for development and saves it to disk
func generateSelfSignedCert(certPath, keyPath string, logger *slog.Logger) (tls.Certificate, error) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// CreateOrganization certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Platform API Dev"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	// CreateOrganization certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// CreateOrganization PEM blocks
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Save certificate and key to disk for persistence
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to save certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to save private key: %v", err)
	}
	logger.Info("Saved certificate", "certPath", certPath, "keyPath", keyPath)

	// CreateOrganization TLS certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		logger.Error("Failed to create TLS certificate", "error", err)
		return tls.Certificate{}, err
	}

	return cert, nil
}

// Start starts the HTTPS server
func (s *Server) Start(port string, certDir string) error {
	if port == "" {
		s.logger.Error("Port cannot be empty")
		return fmt.Errorf("port cannot be empty")
	}

	// Build certificate paths
	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")

	var cert tls.Certificate
	certGenerated := false

	// Try to load existing certificates first
	if _, certErr := os.Stat(certPath); certErr == nil {
		if _, keyErr := os.Stat(keyPath); keyErr == nil {
			loadedCert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				s.logger.Warn("Failed to load certificates", "error", err)
			} else {
				s.logger.Info("Using existing certificates", "certDir", certDir)
				cert = loadedCert
			}
		}
	}

	// Generate new certificate if not loaded
	if cert.Certificate == nil {
		s.logger.Info("Generating self-signed certificate for development...")
		// Ensure cert directory exists
		if err := os.MkdirAll(certDir, 0755); err != nil {
			s.logger.Error("Failed to create cert directory", "error", err)
			return fmt.Errorf("failed to create cert directory: %v", err)
		}
		generatedCert, err := generateSelfSignedCert(certPath, keyPath, s.logger)
		if err != nil {
			s.logger.Error("Failed to generate self-signed certificate", "error", err)
			return fmt.Errorf("failed to generate self-signed certificate: %v", err)
		}
		cert = generatedCert
		certGenerated = true
	}

	// Add a health endpoint that works with self-signed certs
	s.router.GET("/health", func(c *gin.Context) {
		c.Status(200)
		c.JSON(200, gin.H{"status": "ok"})
	})

	// CreateOrganization TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	address := fmt.Sprintf(":%s", port)
	httpServer := &http.Server{
		Addr:      address,
		Handler:   s.router,
		TLSConfig: tlsConfig,
	}

	s.logger.Info("Starting HTTPS server", "address", "https://localhost:"+port)
	if certGenerated {
		s.logger.Warn("Note: Using self-signed certificate for development. Browsers will show security warnings.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.timeoutService.Start(ctx)

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServeTLS("", "")
	}()

	mode := "Production"
	if demoMode() {
		mode = "Demo"
	}
	const termWidth = 80
	msg := fmt.Sprintf("=== Platform API started [%s] ===", mode)
	pad := (termWidth - len(msg)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("\n%*s%s\n\n", pad, "", msg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	teardown := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("HTTP server shutdown error", "error", err)
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
		return err
	case sig := <-quit:
		s.logger.Info("Received shutdown signal", "signal", sig)
		cancel()
		teardown()
		return nil
	}
}

// GetRouter returns the gin router for testing purposes
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// seedFileBasedOrg ensures the file-based auth organization exists in the DB.
// It fetches by the configured handle first; only creates the org when no
// matching org is found. The org ID in cfg is updated to the persisted value
// so the login handler issues tokens with the correct org ID.
func seedFileBasedOrg(cfg *config.Server, orgRepo repository.OrganizationRepository, slogger *slog.Logger) error {
	ba := &cfg.Auth.FileBased

	existing, err := orgRepo.GetOrganizationByHandle(ba.Organization.Handle)
	if err != nil {
		return fmt.Errorf("failed to check file-based organization: %w", err)
	}
	if existing != nil {
		ba.Organization.ID = existing.ID
		slogger.Info("File-based organization already exists", "id", existing.ID, "handle", existing.Handle)
		return nil
	}

	now := time.Now()
	org := &model.Organization{
		ID:        ba.Organization.ID,
		Name:      ba.Organization.Name,
		Handle:    ba.Organization.Handle,
		Region:    ba.Organization.Region,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := orgRepo.CreateOrganization(org); err != nil {
		return fmt.Errorf("failed to create file-based organization: %w", err)
	}
	slogger.Info("Seeded file-based organization", "id", org.ID, "handle", org.Handle)
	return nil
}
