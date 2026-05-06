/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// mock-platform-api simulates platform-api for integration tests.
// It accepts WebSocket connections from the gateway-controller and can inject
// subscription.created events (mimicking how platform-api propagates subscriptions).
package main

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

const (
	wsPath           = "/api/internal/v1/ws/gateways/connect"
	injectPath       = "/inject-subscription"
	subscriptionPath = "/api/internal/v1/subscription-plans"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	connMu     sync.Mutex
	activeConn *websocket.Conn
	dbPath     string
	dbType     string
	dbDSN      string
)

func main() {
	dbType = os.Getenv("DB_TYPE")
	if dbType == "postgres" {
		dbDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			getEnv("DB_USER", "gateway"),
			getEnv("DB_PASSWORD", "gateway"),
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_NAME", "gateway_test"),
			getEnv("DB_SSLMODE", "disable"),
		)
		log.Printf("Mock platform-api using Postgres at %s", getEnv("DB_HOST", "localhost"))
	} else {
		dbPath = os.Getenv("GATEWAY_DB_PATH")
		if dbPath == "" {
			dbPath = "/app/data/gateway.db"
		}
		log.Printf("Mock platform-api using SQLite at %s", dbPath)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(wsPath, wsHandler)
	mux.HandleFunc(injectPath, injectHandler)
	mux.HandleFunc(subscriptionPath, syncHandler)
	mux.HandleFunc("/api/internal/v1/apis/", subscriptionsSyncHandler)

	// HTTP server for IT inject endpoint (port 9244)
	go func() {
		log.Printf("Mock platform-api HTTP server (inject) listening on :9244")
		if err := http.ListenAndServe(":9244", mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// HTTPS server for gateway WebSocket and sync (port 9243)
	certFile := os.Getenv("TLS_CERT")
	keyFile := os.Getenv("TLS_KEY")
	if certFile == "" {
		certFile = "/app/certs/server.crt"
	}
	if keyFile == "" {
		keyFile = "/app/certs/server.key"
	}

	server := &http.Server{
		Addr:    ":9243",
		Handler: mux,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},
	}
	log.Printf("Mock platform-api HTTPS server listening on :9243")
	if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
		log.Fatalf("HTTPS server error: %v", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Validate api-key header (gateway sends this)
	if r.Header.Get("api-key") == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	connMu.Lock()
	if activeConn != nil {
		activeConn.Close()
	}
	activeConn = conn
	connMu.Unlock()
	defer func() {
		connMu.Lock()
		if activeConn == conn {
			activeConn = nil
		}
		connMu.Unlock()
	}()

	// Send connection.ack as gateway expects
	ack := map[string]string{
		"type":         "connection.ack",
		"gatewayId":    "platform-gateway-id",
		"connectionId": uuid.New().String(),
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	if err := conn.WriteJSON(ack); err != nil {
		log.Printf("Failed to send connection.ack: %v", err)
		return
	}
	log.Printf("Sent connection.ack to gateway")

	// Keep connection alive (read loop)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}
	}
}

func injectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		APIHandle             string `json:"apiHandle"`
		SubscriptionToken     string `json:"subscriptionToken"`
		SubscriptionPlanID    string `json:"subscriptionPlanId"`
		BillingCustomerID     string `json:"billingCustomerId"`
		BillingSubscriptionID string `json:"billingSubscriptionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if req.APIHandle == "" || req.SubscriptionToken == "" || req.SubscriptionPlanID == "" {
		http.Error(w, "apiHandle, subscriptionToken, subscriptionPlanId required", http.StatusBadRequest)
		return
	}

	apiID, err := getDeploymentUUID(req.APIHandle)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to resolve API: %v", err), http.StatusInternalServerError)
		return
	}

	connMu.Lock()
	conn := activeConn
	connMu.Unlock()
	if conn == nil {
		http.Error(w, "No gateway connection", http.StatusServiceUnavailable)
		return
	}

	payload := map[string]interface{}{
		"apiId":              apiID,
		"subscriptionId":     uuid.New().String(),
		"subscriptionToken":  req.SubscriptionToken,
		"subscriptionPlanId": req.SubscriptionPlanID,
		"status":             "ACTIVE",
	}
	if req.BillingCustomerID != "" {
		payload["billingCustomerId"] = req.BillingCustomerID
	}
	if req.BillingSubscriptionID != "" {
		payload["billingSubscriptionId"] = req.BillingSubscriptionID
	}

	event := map[string]interface{}{
		"type":          "subscription.created",
		"payload":       payload,
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": uuid.New().String(),
	}
	if err := conn.WriteJSON(event); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send event: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("Sent subscription.created for apiHandle=%s apiId=%s", req.APIHandle, apiID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDeploymentUUID(handle string) (string, error) {
	var db *sql.DB
	var err error
	if dbType == "postgres" {
		db, err = sql.Open("pgx", dbDSN)
	} else {
		db, err = sql.Open("sqlite3", dbPath)
	}
	if err != nil {
		return "", fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	var uuid string
	if dbType == "postgres" {
		err = db.QueryRow(
			"SELECT uuid FROM artifacts WHERE handle = $1 AND kind = 'RestApi' LIMIT 1",
			handle,
		).Scan(&uuid)
	} else {
		err = db.QueryRow(
			"SELECT uuid FROM artifacts WHERE handle = ? AND kind = 'RestApi' LIMIT 1",
			handle,
		).Scan(&uuid)
	}
	if err != nil {
		return "", fmt.Errorf("query: %w", err)
	}
	return uuid, nil
}

// syncHandler returns 500 so gateway sync fails and keeps locally-created plans
func syncHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "mock: sync not implemented", http.StatusInternalServerError)
}

// subscriptionsSyncHandler returns 500 for GET /api/internal/v1/apis/{id}/subscriptions
func subscriptionsSyncHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "mock: sync not implemented", http.StatusInternalServerError)
}
