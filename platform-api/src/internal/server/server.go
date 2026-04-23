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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	"platform-api/src/internal/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router         *gin.Engine
	orgRepo        repository.OrganizationRepository
	projRepo       repository.ProjectRepository
	apiRepo        repository.APIRepository
	gatewayRepo    repository.GatewayRepository
	wsManager      *websocket.Manager // WebSocket connection manager
	timeoutService *service.DeploymentTimeoutService
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
	apiKeyRepo := repository.NewAPIKeyRepo(db)

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
	gatewayEventsService := service.NewGatewayEventsService(wsManager, slogger)
	appService := service.NewApplicationService(appRepo, projectRepo, orgRepo, apiRepo, gatewayEventsService, slogger)
	apiService := service.NewAPIService(apiRepo, projectRepo, orgRepo, gatewayRepo, deploymentRepo, devPortalRepo, publicationRepo,
		subscriptionPlanRepo, customPolicyRepo, gatewayEventsService, devPortalService, apiUtil, slogger)
	gatewayService := service.NewGatewayService(gatewayRepo, orgRepo, apiRepo, customPolicyRepo, gatewayEventsService, slogger)
	subscriptionService := service.NewSubscriptionService(apiRepo, subscriptionRepo, gatewayEventsService, slogger)
	subscriptionPlanService := service.NewSubscriptionPlanService(subscriptionPlanRepo, gatewayRepo, gatewayEventsService, slogger)
	internalGatewayService := service.NewGatewayInternalAPIService(apiRepo, subscriptionRepo, subscriptionPlanRepo, llmProviderRepo, llmProxyRepo, mcpProxyRepo, websubAPIRepo, deploymentRepo, gatewayRepo, orgRepo, projectRepo, apiKeyRepo, artifactRepo, cfg, slogger)
	apiKeyService := service.NewAPIKeyService(apiRepo, apiKeyRepo, gatewayEventsService, cfg.APIKey.HashingAlgorithms, slogger)
	gitService := service.NewGitService()
	deploymentService := service.NewDeploymentService(apiRepo, artifactRepo, deploymentRepo, gatewayRepo, orgRepo, gatewayEventsService, apiUtil, cfg, slogger)
	llmTemplateService := service.NewLLMProviderTemplateService(llmTemplateRepo)
	llmProviderService := service.NewLLMProviderService(llmProviderRepo, llmTemplateRepo, orgRepo, llmTemplateSeeder, deploymentRepo, gatewayRepo, gatewayEventsService, slogger)
	llmProxyService := service.NewLLMProxyService(llmProxyRepo, llmProviderRepo, projectRepo, deploymentRepo, gatewayRepo, gatewayEventsService, slogger)
	mcpProxyService := service.NewMCPProxyService(mcpProxyRepo, projectRepo, deploymentRepo, gatewayRepo, gatewayEventsService, slogger)
	websubAPIService := service.NewWebSubAPIService(websubAPIRepo, projectRepo, gatewayRepo, devPortalService, gatewayEventsService, apiUtil, slogger)
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

	// Initialize handlers
	orgHandler := handler.NewOrganizationHandler(orgService, slogger)
	projectHandler := handler.NewProjectHandler(projectService, slogger)
	apiHandler := handler.NewAPIHandler(apiService, slogger)
	devPortalHandler := handler.NewDevPortalHandler(devPortalService, slogger)
	gatewayHandler := handler.NewGatewayHandler(gatewayService, slogger)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService, subscriptionPlanService, slogger)
	subscriptionPlanHandler := handler.NewSubscriptionPlanHandler(subscriptionPlanService, slogger)
	appHandler := handler.NewApplicationHandler(appService, slogger)
	wsHandler := handler.NewWebSocketHandler(wsManager, gatewayService, deploymentService, cfg.WebSocket.RateLimitPerMin, slogger)
	internalGatewayHandler := handler.NewGatewayInternalAPIHandler(gatewayService, internalGatewayService, slogger)
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
	websubAPIHandler := handler.NewWebSubAPIHandler(websubAPIService, apiKeyService, slogger)
	websubAPIDeploymentHandler := handler.NewWebSubAPIDeploymentHandler(websubAPIDeploymentService, slogger)
	// Start deployment timeout background job
	timeoutConfig := service.DeploymentTimeoutConfig{
		Enabled:  cfg.Deployments.TimeoutEnabled,
		Interval: time.Duration(cfg.Deployments.TimeoutInterval) * time.Second,
		Timeout:  time.Duration(cfg.Deployments.TimeoutDuration) * time.Second,
	}
	timeoutService := service.NewDeploymentTimeoutService(deploymentRepo, timeoutConfig, slogger)

	slogger.Info("Initialized all services and handlers successfully")

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

	// Configure and apply JWT authentication middleware
	authConfig := middleware.AuthConfig{
		SecretKey:      cfg.JWT.SecretKey,
		TokenIssuer:    cfg.JWT.Issuer,
		SkipPaths:      cfg.JWT.SkipPaths,
		SkipValidation: cfg.JWT.SkipValidation,
	}
	router.Use(middleware.AuthMiddleware(authConfig))

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
	websubAPIDeploymentHandler.RegisterRoutes(router)
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
		logger:         slogger,
	}, nil
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		s.logger.Info("Received shutdown signal", "signal", sig)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// GetRouter returns the gin router for testing purposes
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
