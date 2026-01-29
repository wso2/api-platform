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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"platform-api/src/internal/middleware"
	"strings"
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
	router      *gin.Engine
	orgRepo     repository.OrganizationRepository
	projRepo    repository.ProjectRepository
	apiRepo     repository.APIRepository
	gatewayRepo repository.GatewayRepository
	wsManager   *websocket.Manager // WebSocket connection manager
}

// StartPlatformAPIServer creates a new server instance with all dependencies initialized
func StartPlatformAPIServer(cfg *config.Server) (*Server, error) {
	// Initialize database using configuration
	db, err := database.NewConnection(&cfg.Database)
	if err != nil {
		return nil, err
	}

	// Initialize schema (skip when ExecuteSchemaDDL is false, e.g. deployed Postgres without DDL access)
	if cfg.Database.ExecuteSchemaDDL {
		if err := db.InitSchema(cfg.DBSchemaPath); err != nil {
			return nil, err
		}
	} else {
		log.Printf("Skipping schema DDL execution (DATABASE_EXECUTE_SCHEMA_DDL=false)\n")
	}

	// Initialize repositories
	orgRepo := repository.NewOrganizationRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	apiRepo := repository.NewAPIRepo(db)
	gatewayRepo := repository.NewGatewayRepo(db)
	backendServiceRepo := repository.NewBackendServiceRepo(db)
	devPortalRepo := repository.NewDevPortalRepository(db)
	publicationRepo := repository.NewAPIPublicationRepository(db)
	llmTemplateRepo := repository.NewLLMProviderTemplateRepo(db)
	llmProviderRepo := repository.NewLLMProviderRepo(db)
	llmProxyRepo := repository.NewLLMProxyRepo(db)

	// Seed default LLM provider templates into the DB (per organization)
	cfg.LLMTemplateDefinitionsPath = strings.TrimSpace(cfg.LLMTemplateDefinitionsPath)
	defaultTemplates, err := utils.LoadLLMProviderTemplatesFromDirectory(cfg.LLMTemplateDefinitionsPath)
	if err != nil {
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
				log.Printf("[WARN] Failed to load default LLM provider templates from %s: %v", fallbackPath, fallbackErr)
			}
		}
		if err != nil {
			log.Printf("[WARN] Failed to load default LLM provider templates from %s: %v", cfg.LLMTemplateDefinitionsPath, err)
		}
	}
	llmTemplateSeeder := service.NewLLMTemplateSeeder(llmTemplateRepo, defaultTemplates)
	if len(defaultTemplates) > 0 {
		const pageSize = 200
		offset := 0
		for {
			orgs, listErr := orgRepo.ListOrganizations(pageSize, offset)
			if listErr != nil {
				log.Printf("[WARN] Failed to list organizations for LLM template seeding: %v", listErr)
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
					log.Printf("[WARN] Failed to seed LLM templates for organization %s: %v", org.ID, seedErr)
				}
			}
			offset += pageSize
		}
		log.Printf("[INFO] Seeded default LLM provider templates: count=%d", len(defaultTemplates))
	}

	// Initialize WebSocket manager first (needed for GatewayEventsService)
	wsConfig := websocket.ManagerConfig{
		MaxConnections:    cfg.WebSocket.MaxConnections,
		HeartbeatInterval: 20 * time.Second,
		HeartbeatTimeout:  time.Duration(cfg.WebSocket.ConnectionTimeout) * time.Second,
	}
	wsManager := websocket.NewManager(wsConfig)

	// Initialize utilities
	apiUtil := &utils.APIUtil{}

	// Initialize DevPortal service
	devPortalService := service.NewDevPortalService(devPortalRepo, orgRepo, publicationRepo, apiRepo, apiUtil, cfg)

	// Initialize services
	orgService := service.NewOrganizationService(orgRepo, projectRepo, devPortalService, llmTemplateSeeder, cfg)
	projectService := service.NewProjectService(projectRepo, orgRepo, apiRepo)
	gatewayEventsService := service.NewGatewayEventsService(wsManager)
	upstreamService := service.NewUpstreamService(backendServiceRepo)
	apiService := service.NewAPIService(apiRepo, projectRepo, orgRepo, gatewayRepo, devPortalRepo, publicationRepo,
		backendServiceRepo, upstreamService, gatewayEventsService, devPortalService, apiUtil)
	gatewayService := service.NewGatewayService(gatewayRepo, orgRepo, apiRepo)
	internalGatewayService := service.NewGatewayInternalAPIService(apiRepo, gatewayRepo, orgRepo, projectRepo, upstreamService, cfg)
	apiKeyService := service.NewAPIKeyService(apiRepo, gatewayEventsService)
	gitService := service.NewGitService()
	deploymentService := service.NewDeploymentService(apiRepo, gatewayRepo, backendServiceRepo, orgRepo, gatewayEventsService, apiUtil, cfg)
	llmTemplateService := service.NewLLMProviderTemplateService(llmTemplateRepo)
	llmProviderService := service.NewLLMProviderService(llmProviderRepo, llmTemplateRepo, orgRepo, llmTemplateSeeder)
	llmProxyService := service.NewLLMProxyService(llmProxyRepo, llmProviderRepo, projectRepo)

	// Initialize handlers
	orgHandler := handler.NewOrganizationHandler(orgService)
	projectHandler := handler.NewProjectHandler(projectService)
	apiHandler := handler.NewAPIHandler(apiService)
	devPortalHandler := handler.NewDevPortalHandler(devPortalService)
	gatewayHandler := handler.NewGatewayHandler(gatewayService)
	wsHandler := handler.NewWebSocketHandler(wsManager, gatewayService, cfg.WebSocket.RateLimitPerMin)
	internalGatewayHandler := handler.NewGatewayInternalAPIHandler(gatewayService, internalGatewayService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)
	gitHandler := handler.NewGitHandler(gitService)
	deploymentHandler := handler.NewDeploymentHandler(deploymentService)
	llmHandler := handler.NewLLMHandler(llmTemplateService, llmProviderService, llmProxyService)

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
	apiHandler.RegisterRoutes(router)
	devPortalHandler.RegisterRoutes(router)
	gatewayHandler.RegisterRoutes(router)
	wsHandler.RegisterRoutes(router)
	internalGatewayHandler.RegisterRoutes(router)
	apiKeyHandler.RegisterRoutes(router)
	gitHandler.RegisterRoutes(router)
	deploymentHandler.RegisterRoutes(router)
	llmHandler.RegisterRoutes(router)

	log.Printf("[INFO] WebSocket manager initialized: maxConnections=%d heartbeatTimeout=%ds rateLimitPerMin=%d",
		cfg.WebSocket.MaxConnections, cfg.WebSocket.ConnectionTimeout, cfg.WebSocket.RateLimitPerMin)

	return &Server{
		router:      router,
		orgRepo:     orgRepo,
		projRepo:    projectRepo,
		apiRepo:     apiRepo,
		gatewayRepo: gatewayRepo,
		wsManager:   wsManager,
	}, nil
}

// generateSelfSignedCert creates a self-signed certificate for development and saves it to disk
func generateSelfSignedCert(certPath, keyPath string) (tls.Certificate, error) {
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
	log.Printf("Saved certificate to %s and key to %s", certPath, keyPath)

	// CreateOrganization TLS certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}

// Start starts the HTTPS server
func (s *Server) Start(port string, certDir string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Build certificate paths
	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")

	var cert tls.Certificate

	// Try to load existing certificates first
	if _, certErr := os.Stat(certPath); certErr == nil {
		if _, keyErr := os.Stat(keyPath); keyErr == nil {
			loadedCert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				log.Printf("Failed to load certificates: %v", err)
			} else {
				log.Printf("Using existing certificates from %s", certDir)
				cert = loadedCert
			}
		}
	}

	// Generate new certificate if not loaded
	if cert.Certificate == nil {
		log.Println("Generating self-signed certificate for development...")
		// Ensure cert directory exists
		if err := os.MkdirAll(certDir, 0755); err != nil {
			return fmt.Errorf("failed to create cert directory: %v", err)
		}
		generatedCert, err := generateSelfSignedCert(certPath, keyPath)
		if err != nil {
			return fmt.Errorf("failed to generate self-signed certificate: %v", err)
		}
		cert = generatedCert
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
	server := &http.Server{
		Addr:      address,
		Handler:   s.router,
		TLSConfig: tlsConfig,
	}

	log.Printf("Starting HTTPS server on https://localhost:%s", port)
	log.Println("Note: Using self-signed certificate for development. Browsers will show security warnings.")
	return server.ListenAndServeTLS("", "")
}

// GetRouter returns the gin router for testing purposes
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
