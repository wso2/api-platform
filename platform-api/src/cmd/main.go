package main

import (
	"log"
	"platform-api/src/config"
	"platform-api/src/internal/server"
)

func main() {
	cfg := config.GetConfig()

	// Create and start server
	srv, err := server.StartPlatformAPIServer(cfg)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	log.Println("Starting HTTPS server on port 8443...")
	if err := srv.Start(":8443"); err != nil {
		log.Fatal("Failed to start HTTPS server:", err)
	}
}
