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
	"time"

	"github.com/gin-gonic/gin"
	"platform-api/src/config"
	"platform-api/src/internal/database"
	"platform-api/src/internal/handler"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
)

type Server struct {
	router   *gin.Engine
	orgRepo  repository.OrganizationRepository
	projRepo repository.ProjectRepository
}

// StartPlatformAPIServer creates a new server instance with all dependencies initialized
func StartPlatformAPIServer(cfg *config.Server) (*Server, error) {
	// Initialize database
	db, err := database.NewConnection("./data/platform.db")
	if err != nil {
		return nil, err
	}

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		return nil, err
	}

	// Initialize repositories
	orgRepo := repository.NewOrganizationRepo(db)
	projectRepo := repository.NewProjectRepo(db)

	// Initialize services
	orgService := service.NewOrganizationService(orgRepo, projectRepo)
	projectService := service.NewProjectService(projectRepo, orgRepo)

	// Initialize handlers
	orgHandler := handler.NewOrganizationHandler(orgService)
	projectHandler := handler.NewProjectHandler(projectService)

	// Setup router
	router := gin.Default()
	gin.SetMode(gin.ReleaseMode)

	// Register routes
	orgHandler.RegisterRoutes(router)
	projectHandler.RegisterRoutes(router)

	return &Server{
		router:   router,
		orgRepo:  orgRepo,
		projRepo: projectRepo,
	}, nil
}

// generateSelfSignedCert creates a self-signed certificate for development
func generateSelfSignedCert() (tls.Certificate, error) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create certificate template
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

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create PEM blocks
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Create TLS certificate
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

	// Create TLS configuration
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
