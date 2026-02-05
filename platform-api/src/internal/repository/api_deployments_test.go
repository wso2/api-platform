/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package repository

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a temporary SQLite database for testing
func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open SQLite database
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}

	// Enable foreign keys for SQLite
	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Wrap in database.DB
	db := &database.DB{DB: sqlDB}

	// Create schema
	err = createTestSchema(db)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

// createTestSchema creates the minimal schema required for deployment tests
func createTestSchema(db *database.DB) error {
	schema := `
		-- APIs table
		CREATE TABLE IF NOT EXISTS apis (
			uuid TEXT PRIMARY KEY,
			handle TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			context TEXT NOT NULL,
			version TEXT NOT NULL,
			provider TEXT,
			project_uuid TEXT,
			organization_uuid TEXT NOT NULL,
			lifecycle_status TEXT DEFAULT 'CREATED',
			has_thumbnail BOOLEAN DEFAULT FALSE,
			is_default_version BOOLEAN DEFAULT FALSE,
			type TEXT DEFAULT 'REST',
			transport TEXT,
			security_enabled BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		-- Gateways table
		CREATE TABLE IF NOT EXISTS gateways (
			uuid TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			display_name TEXT NOT NULL,
			description TEXT,
			vhost TEXT NOT NULL,
			organization_uuid TEXT NOT NULL,
			is_critical BOOLEAN DEFAULT FALSE,
			gateway_functionality_type TEXT DEFAULT 'general',
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		-- API deployments table (artifact storage)
		CREATE TABLE IF NOT EXISTS api_deployments (
			deployment_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			api_uuid TEXT NOT NULL,
			organization_uuid TEXT NOT NULL,
			gateway_uuid TEXT NOT NULL,
			base_deployment_id TEXT,
			content BLOB NOT NULL,
			metadata TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
		);

		-- API deployment status table (lifecycle state)
		CREATE TABLE IF NOT EXISTS api_deployment_status (
			api_uuid TEXT NOT NULL,
			organization_uuid TEXT NOT NULL,
			gateway_uuid TEXT NOT NULL,
			deployment_id TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('DEPLOYED', 'UNDEPLOYED')),
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (api_uuid, organization_uuid, gateway_uuid),
			FOREIGN KEY (deployment_id) REFERENCES api_deployments(deployment_id) ON DELETE CASCADE
		);
	`

	_, err := db.Exec(schema)
	return err
}

// createTestAPI creates a test API in the database
func createTestAPI(t *testing.T, db *database.DB, apiUUID, orgUUID string) {
	t.Helper()

	query := `
		INSERT INTO apis (uuid, handle, name, context, version, organization_uuid)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, apiUUID, "test-api", "Test API", "/test", "v1", orgUUID)
	if err != nil {
		t.Fatalf("Failed to create test API: %v", err)
	}
}

// createTestGateway creates a test gateway in the database
func createTestGateway(t *testing.T, db *database.DB, gatewayUUID, orgUUID string) {
	t.Helper()

	query := `
		INSERT INTO gateways (uuid, name, display_name, vhost, organization_uuid)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, gatewayUUID, "test-gateway", "Test Gateway", "api.example.com", orgUUID)
	if err != nil {
		t.Fatalf("Failed to create test gateway: %v", err)
	}
}

// insertDeployment inserts a deployment artifact into the database
func insertDeployment(t *testing.T, db *database.DB, deploymentID, name, apiUUID, orgUUID, gatewayUUID string, createdAt time.Time) {
	t.Helper()

	query := `
		INSERT INTO api_deployments (deployment_id, name, api_uuid, organization_uuid, gateway_uuid, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	metadata := "{}"
	_, err := db.Exec(query, deploymentID, name, apiUUID, orgUUID, gatewayUUID, []byte("test content"), metadata, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert deployment: %v", err)
	}
}

// setDeploymentStatus inserts or updates the deployment status
func setDeploymentStatus(t *testing.T, db *database.DB, apiUUID, orgUUID, gatewayUUID, deploymentID string, status model.DeploymentStatus) {
	t.Helper()

	query := `
		REPLACE INTO api_deployment_status (api_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, apiUUID, orgUUID, gatewayUUID, deploymentID, status, time.Now())
	if err != nil {
		t.Fatalf("Failed to set deployment status: %v", err)
	}
}

// ============================================================================
// GetDeploymentsWithState Tests - Soft Limit and Ranking Logic
// ============================================================================

// TestGetDeploymentsWithState_SoftLimit verifies that only the top N deployments per gateway are returned
func TestGetDeploymentsWithState_SoftLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAPIRepo(db)

	// Setup test data
	apiUUID := "api-001"
	orgUUID := "org-001"
	gateway1UUID := "gateway-001"
	gateway2UUID := "gateway-002"

	createTestAPI(t, db, apiUUID, orgUUID)
	createTestGateway(t, db, gateway1UUID, orgUUID)
	createTestGateway(t, db, gateway2UUID, orgUUID)

	baseTime := time.Now().Add(-24 * time.Hour)

	// Gateway 1: Create 10 deployments (8 ARCHIVED, 1 DEPLOYED, 1 UNDEPLOYED)
	for i := 0; i < 8; i++ {
		deploymentID := fmt.Sprintf("deploy-gw1-%02d", i)
		insertDeployment(t, db, deploymentID, fmt.Sprintf("Deployment %d", i), apiUUID, orgUUID, gateway1UUID, baseTime.Add(time.Duration(i)*time.Minute))
	}

	insertDeployment(t, db, "deploy-gw1-deployed", "Currently Deployed", apiUUID, orgUUID, gateway1UUID, baseTime.Add(10*time.Minute))
	setDeploymentStatus(t, db, apiUUID, orgUUID, gateway1UUID, "deploy-gw1-deployed", model.DeploymentStatusDeployed)

	insertDeployment(t, db, "deploy-gw1-undeployed", "Undeployed", apiUUID, orgUUID, gateway1UUID, baseTime.Add(9*time.Minute))
	setDeploymentStatus(t, db, apiUUID, orgUUID, gateway1UUID, "deploy-gw1-undeployed", model.DeploymentStatusUndeployed)

	// Gateway 2: Create 7 deployments (5 ARCHIVED, 1 DEPLOYED, 1 UNDEPLOYED)
	for i := 0; i < 5; i++ {
		deploymentID := fmt.Sprintf("deploy-gw2-%02d", i)
		insertDeployment(t, db, deploymentID, fmt.Sprintf("GW2 Deployment %d", i), apiUUID, orgUUID, gateway2UUID, baseTime.Add(time.Duration(i)*time.Minute))
	}

	insertDeployment(t, db, "deploy-gw2-deployed", "GW2 Deployed", apiUUID, orgUUID, gateway2UUID, baseTime.Add(6*time.Minute))
	setDeploymentStatus(t, db, apiUUID, orgUUID, gateway2UUID, "deploy-gw2-deployed", model.DeploymentStatusDeployed)

	insertDeployment(t, db, "deploy-gw2-undeployed", "GW2 Undeployed", apiUUID, orgUUID, gateway2UUID, baseTime.Add(5*time.Minute))
	setDeploymentStatus(t, db, apiUUID, orgUUID, gateway2UUID, "deploy-gw2-undeployed", model.DeploymentStatusUndeployed)

	t.Run("Soft limit of 5 per gateway", func(t *testing.T) {
		deployments, err := repo.GetDeploymentsWithState(apiUUID, orgUUID, nil, nil, 5)
		if err != nil {
			t.Fatalf("GetDeploymentsWithState failed: %v", err)
		}

		gw1Count := 0
		gw2Count := 0
		for _, d := range deployments {
			if d.GatewayID == gateway1UUID {
				gw1Count++
			} else if d.GatewayID == gateway2UUID {
				gw2Count++
			}
		}

		if gw1Count > 5 {
			t.Errorf("Gateway 1 has %d deployments, expected at most 5", gw1Count)
		}
		if gw2Count > 5 {
			t.Errorf("Gateway 2 has %d deployments, expected at most 5", gw2Count)
		}

		hasGw1Deployed := false
		hasGw1Undeployed := false
		hasGw2Deployed := false
		hasGw2Undeployed := false

		for _, d := range deployments {
			switch d.DeploymentID {
			case "deploy-gw1-deployed":
				hasGw1Deployed = true
			case "deploy-gw1-undeployed":
				hasGw1Undeployed = true
			case "deploy-gw2-deployed":
				hasGw2Deployed = true
			case "deploy-gw2-undeployed":
				hasGw2Undeployed = true
			}
		}

		if !hasGw1Deployed {
			t.Error("Gateway 1 DEPLOYED deployment must be included")
		}
		if !hasGw1Undeployed {
			t.Error("Gateway 1 UNDEPLOYED deployment must be included")
		}
		if !hasGw2Deployed {
			t.Error("Gateway 2 DEPLOYED deployment must be included")
		}
		if !hasGw2Undeployed {
			t.Error("Gateway 2 UNDEPLOYED deployment must be included")
		}
	})

	t.Run("Soft limit of 3 per gateway", func(t *testing.T) {
		deployments, err := repo.GetDeploymentsWithState(apiUUID, orgUUID, nil, nil, 3)
		if err != nil {
			t.Fatalf("GetDeploymentsWithState failed: %v", err)
		}

		gw1Count := 0
		gw2Count := 0
		for _, d := range deployments {
			if d.GatewayID == gateway1UUID {
				gw1Count++
			} else if d.GatewayID == gateway2UUID {
				gw2Count++
			}
		}

		if gw1Count > 3 {
			t.Errorf("Gateway 1 has %d deployments, expected at most 3", gw1Count)
		}
		if gw2Count > 3 {
			t.Errorf("Gateway 2 has %d deployments, expected at most 3", gw2Count)
		}
	})
}

// TestGetDeploymentsWithState_PrioritizationLogic verifies DEPLOYED/UNDEPLOYED are prioritized over ARCHIVED
func TestGetDeploymentsWithState_PrioritizationLogic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAPIRepo(db)

	apiUUID := "api-002"
	orgUUID := "org-002"
	gatewayUUID := "gateway-003"

	createTestAPI(t, db, apiUUID, orgUUID)
	createTestGateway(t, db, gatewayUUID, orgUUID)

	baseTime := time.Now().Add(-10 * time.Hour)

	// Create 5 ARCHIVED deployments (oldest first)
	for i := 0; i < 5; i++ {
		deploymentID := fmt.Sprintf("archived-%02d", i)
		insertDeployment(t, db, deploymentID, fmt.Sprintf("Archived %d", i), apiUUID, orgUUID, gatewayUUID, baseTime.Add(time.Duration(i)*time.Minute))
	}

	// Create 1 DEPLOYED (older than some archived deployments)
	// Note: timestamp at 6 minutes, which is older than the newest archived ones
	insertDeployment(t, db, "deployed-current", "Current Deployed", apiUUID, orgUUID, gatewayUUID, baseTime.Add(6*time.Minute))
	setDeploymentStatus(t, db, apiUUID, orgUUID, gatewayUUID, "deployed-current", model.DeploymentStatusDeployed)

	t.Run("DEPLOYED prioritized over ARCHIVED with limit 3", func(t *testing.T) {
		deployments, err := repo.GetDeploymentsWithState(apiUUID, orgUUID, nil, nil, 3)
		if err != nil {
			t.Fatalf("GetDeploymentsWithState failed: %v", err)
		}

		if len(deployments) != 3 {
			t.Errorf("Expected 3 deployments, got %d", len(deployments))
		}

		hasDeployed := false
		archivedCount := 0

		for _, d := range deployments {
			if d.DeploymentID == "deployed-current" {
				hasDeployed = true
				if *d.Status != model.DeploymentStatusDeployed {
					t.Errorf("deployed-current should have status DEPLOYED, got %s", *d.Status)
				}
			} else {
				archivedCount++
				statusStr := "nil"
				if d.Status != nil {
					statusStr = string(*d.Status)
				}
				// ARCHIVED deployments should have Status set to ARCHIVED by the repo layer
				if d.Status == nil || *d.Status != model.DeploymentStatusArchived {
					t.Errorf("Archived deployment should have status ARCHIVED, got %s", statusStr)
				}
			}
		}

		if !hasDeployed {
			t.Error("DEPLOYED deployment must be included despite not being the newest")
		}
		if archivedCount != 2 {
			t.Errorf("Expected 2 archived deployments, got %d", archivedCount)
		}
	})

	t.Run("With limit 1, only DEPLOYED returned", func(t *testing.T) {
		deployments, err := repo.GetDeploymentsWithState(apiUUID, orgUUID, nil, nil, 1)
		if err != nil {
			t.Fatalf("GetDeploymentsWithState failed: %v", err)
		}

		if len(deployments) != 1 {
			t.Errorf("Expected 1 deployment, got %d", len(deployments))
		}

		if deployments[0].DeploymentID != "deployed-current" {
			t.Errorf("Expected deployed-current, got %s", deployments[0].DeploymentID)
		}

		if *deployments[0].Status != model.DeploymentStatusDeployed {
			t.Errorf("Expected DEPLOYED status, got %s", *deployments[0].Status)
		}
	})
}

// TestGetDeploymentsWithState_MultipleGateways verifies ranking across multiple gateways
func TestGetDeploymentsWithState_MultipleGateways(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAPIRepo(db)

	apiUUID := "api-003"
	orgUUID := "org-003"
	gw1UUID := "gateway-004"
	gw2UUID := "gateway-005"
	gw3UUID := "gateway-006"

	createTestAPI(t, db, apiUUID, orgUUID)
	createTestGateway(t, db, gw1UUID, orgUUID)
	createTestGateway(t, db, gw2UUID, orgUUID)
	createTestGateway(t, db, gw3UUID, orgUUID)

	baseTime := time.Now().Add(-5 * time.Hour)

	// Gateway 1: 1 DEPLOYED, 3 ARCHIVED
	insertDeployment(t, db, "gw1-deployed", "GW1 Deployed", apiUUID, orgUUID, gw1UUID, baseTime)
	setDeploymentStatus(t, db, apiUUID, orgUUID, gw1UUID, "gw1-deployed", model.DeploymentStatusDeployed)
	for i := 0; i < 3; i++ {
		insertDeployment(t, db, fmt.Sprintf("gw1-arch-%d", i), fmt.Sprintf("GW1 Archived %d", i), apiUUID, orgUUID, gw1UUID, baseTime.Add(time.Duration(i+1)*time.Minute))
	}

	// Gateway 2: 1 UNDEPLOYED, 2 ARCHIVED
	insertDeployment(t, db, "gw2-undeployed", "GW2 Undeployed", apiUUID, orgUUID, gw2UUID, baseTime)
	setDeploymentStatus(t, db, apiUUID, orgUUID, gw2UUID, "gw2-undeployed", model.DeploymentStatusUndeployed)
	for i := 0; i < 2; i++ {
		insertDeployment(t, db, fmt.Sprintf("gw2-arch-%d", i), fmt.Sprintf("GW2 Archived %d", i), apiUUID, orgUUID, gw2UUID, baseTime.Add(time.Duration(i+1)*time.Minute))
	}

	// Gateway 3: 5 ARCHIVED (no active status)
	for i := 0; i < 5; i++ {
		insertDeployment(t, db, fmt.Sprintf("gw3-arch-%d", i), fmt.Sprintf("GW3 Archived %d", i), apiUUID, orgUUID, gw3UUID, baseTime.Add(time.Duration(i+1)*time.Minute))
	}

	t.Run("Ranking with limit 2 per gateway", func(t *testing.T) {
		deployments, err := repo.GetDeploymentsWithState(apiUUID, orgUUID, nil, nil, 2)
		if err != nil {
			t.Fatalf("GetDeploymentsWithState failed: %v", err)
		}

		gw1Count := 0
		gw2Count := 0
		gw3Count := 0

		for _, d := range deployments {
			switch d.GatewayID {
			case gw1UUID:
				gw1Count++
			case gw2UUID:
				gw2Count++
			case gw3UUID:
				gw3Count++
			}
		}

		if gw1Count > 2 {
			t.Errorf("Gateway 1 has %d deployments, expected at most 2", gw1Count)
		}
		if gw2Count > 2 {
			t.Errorf("Gateway 2 has %d deployments, expected at most 2", gw2Count)
		}
		if gw3Count > 2 {
			t.Errorf("Gateway 3 has %d deployments, expected at most 2", gw3Count)
		}

		// Verify GW1 has DEPLOYED + 1 ARCHIVED
		gw1HasDeployed := false
		gw1ArchivedCount := 0
		for _, d := range deployments {
			if d.GatewayID == gw1UUID {
				if *d.Status == model.DeploymentStatusDeployed {
					gw1HasDeployed = true
				} else if *d.Status == model.DeploymentStatusArchived {
					gw1ArchivedCount++
				}
			}
		}
		if !gw1HasDeployed {
			t.Error("GW1 must include DEPLOYED")
		}
		if gw1ArchivedCount != 1 {
			t.Errorf("GW1 should have 1 ARCHIVED, got %d", gw1ArchivedCount)
		}

		// Verify GW2 has UNDEPLOYED + 1 ARCHIVED
		gw2HasUndeployed := false
		gw2ArchivedCount := 0
		for _, d := range deployments {
			if d.GatewayID == gw2UUID {
				if *d.Status == model.DeploymentStatusUndeployed {
					gw2HasUndeployed = true
				} else if *d.Status == model.DeploymentStatusArchived {
					gw2ArchivedCount++
				}
			}
		}
		if !gw2HasUndeployed {
			t.Error("GW2 must include UNDEPLOYED")
		}
		if gw2ArchivedCount != 1 {
			t.Errorf("GW2 should have 1 ARCHIVED, got %d", gw2ArchivedCount)
		}

		// Verify GW3 has exactly 2 ARCHIVED
		if gw3Count != 2 {
			t.Errorf("GW3 should have exactly 2 ARCHIVED, got %d", gw3Count)
		}
	})

	t.Run("Filter by specific gateway", func(t *testing.T) {
		deployments, err := repo.GetDeploymentsWithState(apiUUID, orgUUID, &gw1UUID, nil, 10)
		if err != nil {
			t.Fatalf("GetDeploymentsWithState failed: %v", err)
		}

		for _, d := range deployments {
			if d.GatewayID != gw1UUID {
				t.Errorf("All deployments should be from gateway %s, got %s", gw1UUID, d.GatewayID)
			}
		}

		if len(deployments) > 4 {
			t.Errorf("Should have at most 4 deployments (1 DEPLOYED + 3 ARCHIVED), got %d", len(deployments))
		}
	})
}

func TestMain(m *testing.M) {
	// Setup
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Run tests
	code := m.Run()

	// Teardown
	os.Exit(code)
}
