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
	"archive/zip"
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/microsoft/go-mssqldb"
)

const (
	wsPath           = "/api/internal/v1/ws/gateways/connect"
	injectPath       = "/inject-subscription"
	subscriptionPath = "/api/internal/v1/subscription-plans"
	manifestPath     = "POST /api/internal/v1/gateways/{gatewayId}/manifest"

	// importArtifactsPath is the generic DP->CP bulk artifact import endpoint the
	// gateway-controller pushes to (multipart/form-data: an "artifacts" zip whose
	// single "artifacts.json" entry is a JSON array of ImportArtifactRequest, plus a
	// "total" field). Mirrors platform-api's real handler; used by the dp-to-cp IT.
	importArtifactsPath = "POST /api/internal/v1/artifacts/import-gateway-artifacts"

	// gatewayArtifactsZipEntry is the file name, inside the "artifacts" zip, holding
	// the JSON array of pushed artifacts. Must match the gateway-controller constant
	// of the same name (gateway-controller/pkg/utils/api_utils.go).
	gatewayArtifactsZipEntry = "artifacts.json"
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
	} else if dbType == "sqlserver" {
		dbDSN = fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s&encrypt=%s&TrustServerCertificate=true",
			getEnv("DB_USER", "sa"),
			getEnv("DB_PASSWORD", "gateway"),
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "1433"),
			getEnv("DB_NAME", "gateway_test"),
			getEnv("DB_ENCRYPT", "disable"),
		)
		log.Printf("Mock platform-api using SQL Server at %s", getEnv("DB_HOST", "localhost"))
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
	mux.HandleFunc(manifestPath, manifestHandler)

	// DP->CP artifact import (what the gateway pushes) + test-inspection endpoints.
	mux.HandleFunc(importArtifactsPath, importArtifactsHandler)
	mux.HandleFunc("GET /_test/artifacts", testArtifactsHandler)
	mux.HandleFunc("GET /_test/pushes", testPushesHandler)
	mux.HandleFunc("POST /_test/reset", testResetHandler)
	mux.HandleFunc("POST /_test/config", testConfigHandler)

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
	switch dbType {
	case "postgres":
		db, err = sql.Open("pgx", dbDSN)
	case "sqlserver":
		db, err = sql.Open("sqlserver", dbDSN)
	default:
		db, err = sql.Open("sqlite3", dbPath)
	}
	if err != nil {
		return "", fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	var uuid string
	switch dbType {
	case "postgres":
		err = db.QueryRow(
			"SELECT uuid FROM artifacts WHERE handle = $1 AND kind = 'RestApi' LIMIT 1",
			handle,
		).Scan(&uuid)
	case "sqlserver":
		err = db.QueryRow(
			"SELECT TOP 1 uuid FROM artifacts WHERE handle = @p1 AND kind = 'RestApi'",
			handle,
		).Scan(&uuid)
	default:
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

// manifestHandler accepts the gateway manifest push and mirrors the real
// platform-api response: 204 No Content with an empty body.
func manifestHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// -----------------------------------------------------------------------------
// DP->CP artifact import recorder
//
// The gateway-controller pushes every gateway-originated artifact (LLM provider
// template/provider/proxy, MCP proxy, REST API, ...) to the control plane via the
// bulk import endpoint. This mock stands in for that control plane: it records
// what was pushed so the dp-to-cp integration test can assert on it, and mints a
// per-handle CP artifact UUID (returned as `id`) exactly like the real platform-api
// so the gateway records a `cp_artifact_id` / `cp_sync_status=success`.
//
// The recorder is deliberately dependency-free (in-memory) and additive: it does
// not touch the WebSocket/subscription behaviour the other ITs rely on.
// -----------------------------------------------------------------------------

// importArtifactRequest is one entry in the pushed artifacts.json array. It mirrors
// the gateway-controller's ImportArtifactRequest; Configuration is kept as a generic
// map so the mock stays decoupled from each kind's spec schema.
type importArtifactRequest struct {
	DPID          string                 `json:"dpid"`
	Configuration map[string]interface{} `json:"configuration"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	DeployedAt    *time.Time             `json:"deployedAt,omitempty"`
}

// importArtifactResponse is the per-artifact result the gateway parses. The gateway
// reads `id` (stored as cp_artifact_id) and `error` (empty => cp_sync_status=success).
type importArtifactResponse struct {
	ID         string     `json:"id,omitempty"`
	Status     string     `json:"status"`
	Origin     string     `json:"origin"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	DeployedAt *time.Time `json:"deployedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// importArtifactsResponse is the bulk import reply: per-dpid results plus aggregate counts.
type importArtifactsResponse struct {
	Total     int                               `json:"total"`
	Success   int                               `json:"success"`
	Failed    int                               `json:"failed"`
	Artifacts map[string]importArtifactResponse `json:"artifacts"`
}

// recordedArtifact is the mock's current view of a pushed artifact, keyed by
// kind+handle (the org-scoped identity the real CP matches on). It reflects the
// latest push for that artifact.
type recordedArtifact struct {
	DPID          string                 `json:"dpid"`
	CPID          string                 `json:"cpId"`
	Kind          string                 `json:"kind"`
	Handle        string                 `json:"handle"`
	Status        string                 `json:"status"` // latest pushed status (deployed|undeployed|...)
	Origin        string                 `json:"origin"`
	DeployedAt    *time.Time             `json:"deployedAt,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	Configuration map[string]interface{} `json:"configuration"`
	PushCount     int                    `json:"pushCount"`
}

// pushEvent is an append-only log entry: one per received artifact push. Lets the
// test assert ordering and per-push detail (e.g. an undeploy following a deploy).
type pushEvent struct {
	Seq        int        `json:"seq"`
	DPID       string     `json:"dpid"`
	Kind       string     `json:"kind"`
	Handle     string     `json:"handle"`
	Status     string     `json:"status"`
	DeployedAt *time.Time `json:"deployedAt,omitempty"`
}

var (
	recMu         sync.Mutex
	recByKey      = map[string]*recordedArtifact{} // key = kind + "|" + handle
	pushLog       []pushEvent
	pushSeq       int
	rejectImports bool // when true, every import is rejected (per-artifact error)
)

func artifactKey(kind, handle string) string { return kind + "|" + handle }

func cfgString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// cfgHandle extracts metadata.name (the artifact handle) from a configuration map.
func cfgHandle(cfg map[string]interface{}) string {
	md, _ := cfg["metadata"].(map[string]interface{})
	return cfgString(md, "name")
}

// importArtifactsHandler records a DP->CP bulk artifact push and returns a
// platform-api-shaped response (a minted CP UUID per artifact, matched by handle).
func importArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("artifacts")
	if err != nil {
		http.Error(w, "missing 'artifacts' file part: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	zipBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read artifacts zip: "+err.Error(), http.StatusBadRequest)
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		http.Error(w, "invalid artifacts zip: "+err.Error(), http.StatusBadRequest)
		return
	}
	var listJSON []byte
	for _, f := range zr.File {
		if f.Name == gatewayArtifactsZipEntry {
			rc, err := f.Open()
			if err != nil {
				http.Error(w, "failed to open "+gatewayArtifactsZipEntry+": "+err.Error(), http.StatusBadRequest)
				return
			}
			listJSON, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				http.Error(w, "failed to read "+gatewayArtifactsZipEntry+": "+err.Error(), http.StatusBadRequest)
				return
			}
			break
		}
	}
	if listJSON == nil {
		http.Error(w, gatewayArtifactsZipEntry+" not found in artifacts zip", http.StatusBadRequest)
		return
	}
	var reqs []importArtifactRequest
	if err := json.Unmarshal(listJSON, &reqs); err != nil {
		http.Error(w, "invalid "+gatewayArtifactsZipEntry+": "+err.Error(), http.StatusBadRequest)
		return
	}

	resp := importArtifactsResponse{
		Total:     len(reqs),
		Artifacts: make(map[string]importArtifactResponse, len(reqs)),
	}

	recMu.Lock()
	for _, req := range reqs {
		kind := cfgString(req.Configuration, "kind")
		handle := cfgHandle(req.Configuration)

		if rejectImports {
			resp.Artifacts[req.DPID] = importArtifactResponse{
				Status: "failed",
				Origin: "gateway_api",
				Error:  "mock control plane: artifact import rejected",
			}
			resp.Failed++
			continue
		}

		key := artifactKey(kind, handle)
		rec, ok := recByKey[key]
		if !ok {
			rec = &recordedArtifact{CPID: uuid.New().String(), Kind: kind, Handle: handle}
			recByKey[key] = rec
		}
		rec.DPID = req.DPID
		rec.Status = req.Status
		rec.Origin = "gateway_api"
		rec.DeployedAt = req.DeployedAt
		rec.CreatedAt = req.CreatedAt
		rec.UpdatedAt = req.UpdatedAt
		rec.Configuration = req.Configuration
		rec.PushCount++

		pushSeq++
		pushLog = append(pushLog, pushEvent{
			Seq: pushSeq, DPID: req.DPID, Kind: kind, Handle: handle,
			Status: req.Status, DeployedAt: req.DeployedAt,
		})

		resp.Artifacts[req.DPID] = importArtifactResponse{
			ID:         rec.CPID,
			Status:     req.Status,
			Origin:     "gateway_api",
			CreatedAt:  req.CreatedAt,
			UpdatedAt:  req.UpdatedAt,
			DeployedAt: req.DeployedAt,
		}
		resp.Success++
	}
	recMu.Unlock()

	log.Printf("Recorded DP->CP import: total=%d success=%d failed=%d", resp.Total, resp.Success, resp.Failed)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// testArtifactsHandler returns the current per-artifact view (one entry per
// kind+handle), sorted for deterministic output.
func testArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	recMu.Lock()
	out := make([]recordedArtifact, 0, len(recByKey))
	for _, rec := range recByKey {
		out = append(out, *rec)
	}
	recMu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		return artifactKey(out[i].Kind, out[i].Handle) < artifactKey(out[j].Kind, out[j].Handle)
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// testPushesHandler returns the append-only push log (every received push, in order).
func testPushesHandler(w http.ResponseWriter, r *http.Request) {
	recMu.Lock()
	out := make([]pushEvent, len(pushLog))
	copy(out, pushLog)
	recMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// testResetHandler clears the recorder and failure toggle. Called at the start of
// each dp-to-cp scenario so assertions see only that scenario's pushes.
func testResetHandler(w http.ResponseWriter, r *http.Request) {
	recMu.Lock()
	recByKey = map[string]*recordedArtifact{}
	pushLog = nil
	pushSeq = 0
	rejectImports = false
	recMu.Unlock()
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

// testConfigHandler toggles recorder behaviour for negative tests. Body:
// {"rejectImports": true} makes every subsequent import fail per-artifact.
func testConfigHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RejectImports *bool `json:"rejectImports"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	recMu.Lock()
	if body.RejectImports != nil {
		rejectImports = *body.RejectImports
	}
	current := rejectImports
	recMu.Unlock()
	log.Printf("Recorder config updated: rejectImports=%v", current)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"rejectImports": current})
}
