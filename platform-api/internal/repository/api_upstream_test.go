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
	"reflect"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// TestAPIRepo_CreateAndRead_PreservesUpstreamDefinitionsAndPerOp verifies the reusable pool
// and per-operation upstream refs survive the configuration JSON blob's database round trip.
func TestAPIRepo_CreateAndRead_PreservesUpstreamDefinitionsAndPerOp(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-upstream-crud-001"
	projectUUID := "project-upstream-crud-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	weight := 80
	api := &model.API{
		Handle:          "upstream-pool-api",
		Name:            "Upstream Pool API",
		Version:         "1.0.0",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:    "Upstream Pool API",
			Version: "1.0.0",
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "http://default-backend:8080"},
			},
			UpstreamDefinitions: []model.ReusableUpstream{
				{
					Name:      "alt-backend",
					BasePath:  "/api/v2",
					Timeout:   &model.UpstreamTimeout{Connect: "5s"},
					Upstreams: []model.UpstreamTarget{{URL: "http://alt:9090", Weight: &weight}},
				},
			},
			Operations: []model.Operation{
				{Request: &model.OperationRequest{
					Method: "GET", Path: "/whoami",
					Upstream: &model.OperationUpstream{Main: &model.OperationUpstreamRef{Ref: "alt-backend"}},
				}},
				{Request: &model.OperationRequest{Method: "GET", Path: "/ping"}},
			},
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	defer func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	}()

	created, err := repo.GetAPIByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if created == nil {
		t.Fatal("GetAPIByUUID returned nil")
	}

	if !reflect.DeepEqual(created.Configuration.UpstreamDefinitions, api.Configuration.UpstreamDefinitions) {
		t.Fatalf("upstreamDefinitions did not survive the round trip:\ngot  %+v\nwant %+v",
			created.Configuration.UpstreamDefinitions, api.Configuration.UpstreamDefinitions)
	}
	if !reflect.DeepEqual(created.Configuration.Operations, api.Configuration.Operations) {
		t.Fatalf("operations (with per-op upstream) did not survive the round trip:\ngot  %+v\nwant %+v",
			created.Configuration.Operations, api.Configuration.Operations)
	}
}
