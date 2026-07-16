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
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// TestGatewayRepo_UpdateGateway_NormalizesUpdatedAtToUTC guards against a prior
// gap where UpdateGateway persisted whatever UpdatedAt the caller passed in
// (typically local-server-time time.Now()) instead of normalizing to UTC like
// Create does, leaving a gateway's created_at (UTC) and updated_at (local)
// in different timezone conventions after any edit.
func TestGatewayRepo_UpdateGateway_NormalizesUpdatedAtToUTC(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-gw-utc"
	projectUUID := "project-gw-utc"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewGatewayRepo(db)
	gateway := &model.Gateway{
		ID:             "gw-update-utc",
		OrganizationID: orgUUID,
		Handle:         "gw-update-utc",
		Name:           "Gateway UTC Update",
	}
	if err := repo.Create(gateway); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if gateway.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected Create to set CreatedAt in UTC, got location %v", gateway.CreatedAt.Location())
	}

	// Deliberately pass a non-UTC UpdatedAt, mimicking a caller that computed
	// time.Now() (server-local time) instead of time.Now().UTC().
	localZone := time.FixedZone("Test/Local", 5*60*60)
	gateway.Name = "Gateway UTC Update (renamed)"
	gateway.UpdatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, localZone)

	if err := repo.UpdateGateway(gateway); err != nil {
		t.Fatalf("UpdateGateway failed: %v", err)
	}

	if gateway.UpdatedAt.Location() != time.UTC {
		t.Fatalf("expected UpdateGateway to normalize UpdatedAt to UTC, got location %v", gateway.UpdatedAt.Location())
	}
	if time.Since(gateway.UpdatedAt) > time.Minute {
		t.Fatalf("expected UpdatedAt to be normalized to roughly now, got %v", gateway.UpdatedAt)
	}

	updated, err := repo.GetByHandleAndOrgID(gateway.Handle, orgUUID)
	if err != nil {
		t.Fatalf("GetByHandleAndOrgID failed: %v", err)
	}
	if updated == nil {
		t.Fatal("GetByHandleAndOrgID returned nil")
	}
	if updated.Name != "Gateway UTC Update (renamed)" {
		t.Fatalf("expected updated name to persist, got %q", updated.Name)
	}
	if updated.UpdatedAt.Year() == 2020 {
		t.Fatalf("expected the persisted UpdatedAt to be the normalized value, not the caller-supplied 2020 date: %v", updated.UpdatedAt)
	}
}
