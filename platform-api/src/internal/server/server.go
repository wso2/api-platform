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
	"platform-api/src/internal/middleware"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/client/apiportal"
	"platform-api/src/internal/database"
	"platform-api/src/internal/handler"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
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

	// Initialize schema
	if err := db.InitSchema(cfg.DBSchemaPath); err != nil {
		return nil, err
	}

	// Initialize repositories
	orgRepo := repository.NewOrganizationRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	apiRepo := repository.NewAPIRepo(db)
	gatewayRepo := repository.NewGatewayRepo(db)

	// Initialize WebSocket manager first (needed for GatewayEventsService)
	wsConfig := websocket.ManagerConfig{
		MaxConnections:    cfg.WebSocket.MaxConnections,
		HeartbeatInterval: 20 * time.Second,
		HeartbeatTimeout:  time.Duration(cfg.WebSocket.ConnectionTimeout) * time.Second,
	}
	wsManager := websocket.NewManager(wsConfig)

	// Initialize api portal client
	apiPortalClient := apiportal.NewApiPortalClient(cfg.ApiPortal)

	// Initialize services
	orgService := service.NewOrganizationService(orgRepo, projectRepo, apiPortalClient)
	projectService := service.NewProjectService(projectRepo, orgRepo, apiRepo)
	gatewayEventsService := service.NewGatewayEventsService(wsManager)
	apiService := service.NewAPIService(apiRepo, projectRepo, gatewayRepo, gatewayEventsService, apiPortalClient)
	gatewayService := service.NewGatewayService(gatewayRepo, orgRepo, apiRepo)
	internalGatewayService := service.NewGatewayInternalAPIService(apiRepo, gatewayRepo, orgRepo, projectRepo)

	// Initialize handlers
	orgHandler := handler.NewOrganizationHandler(orgService)
	projectHandler := handler.NewProjectHandler(projectService)
	apiHandler := handler.NewAPIHandler(apiService)
	gatewayHandler := handler.NewGatewayHandler(gatewayService)
	wsHandler := handler.NewWebSocketHandler(wsManager, gatewayService, cfg.WebSocket.RateLimitPerMin)
	internalGatewayHandler := handler.NewGatewayInternalAPIHandler(gatewayService, internalGatewayService)

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
	gatewayHandler.RegisterRoutes(router)
	wsHandler.RegisterRoutes(router)
	internalGatewayHandler.RegisterRoutes(router)

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

// generateSelfSignedCert creates a self-signed certificate for development
func generateSelfSignedCert() (tls.Certificate, error) {
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

	// CreateOrganization TLS certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}

// Start starts the HTTPS server
func (s *Server) Start(port string) error {
	if port == "" {
		port = ":8443"
	}

	var cert tls.Certificate

	// Try to load existing certificates first
	if _, err := os.Stat("cert.pem"); err == nil {
		if _, err := os.Stat("key.pem"); err == nil {
			cert, err = tls.LoadX509KeyPair("cert.pem", "key.pem")
			if err != nil {
				log.Printf("Failed to load certificates: %v", err)
				log.Println("Generating self-signed certificate for development...")
				cert, err = generateSelfSignedCert()
				if err != nil {
					return fmt.Errorf("failed to generate self-signed certificate: %v", err)
				}
			} else {
				log.Println("Using existing certificates: cert.pem and key.pem")
			}
		} else {
			log.Println("Generating self-signed certificate for development...")
			var err error
			cert, err = generateSelfSignedCert()
			if err != nil {
				return fmt.Errorf("failed to generate self-signed certificate: %v", err)
			}
		}
	} else {
		log.Println("Generating self-signed certificate for development...")
		var err error
		cert, err = generateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("failed to generate self-signed certificate: %v", err)
		}
	}

	// Add a health endpoint that works with self-signed certs
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// CreateOrganization TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      port,
		Handler:   s.router,
		TLSConfig: tlsConfig,
	}

	log.Printf("Starting HTTPS server on https://localhost%s", port)
	log.Println("Note: Using self-signed certificate for development. Browsers will show security warnings.")
	return server.ListenAndServeTLS("", "")
}

// GetRouter returns the gin router for testing purposes
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
