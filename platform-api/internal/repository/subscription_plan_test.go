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

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestSubscriptionPlanRepo_ListAndCountSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{
		ID: "org-1", Handle: "acme", Name: "Acme", Region: "us",
	}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	repo := NewSubscriptionPlanRepo(db)
	for _, p := range []struct{ handle, name string }{
		{"gold", "Gold Plan"}, {"silver", "Silver Plan"}, {"bronze", "Bronze 100%"},
	} {
		if err := repo.Create(&model.SubscriptionPlan{
			Handle: p.handle, Name: p.name, OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive,
		}); err != nil {
			t.Fatalf("Create %s: %v", p.handle, err)
		}
	}

	tests := []struct {
		name      string
		opts      ListOptions
		wantTotal int
		wantPage  int
	}{
		{"no search", ListOptions{Limit: 10}, 3, 3},
		{"page smaller than total", ListOptions{Limit: 2}, 3, 2},
		{"second page", ListOptions{Limit: 2, Offset: 2}, 3, 1},
		{"matches name case-insensitively", ListOptions{Limit: 10, Search: "GOLD"}, 1, 1},
		{"matches handle", ListOptions{Limit: 10, Search: "silv"}, 1, 1},
		{"percent is literal", ListOptions{Limit: 10, Search: "%"}, 1, 1},
		{"no match", ListOptions{Limit: 10, Search: "platinum"}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plans, err := repo.ListByOrganization("org-1", tt.opts)
			if err != nil {
				t.Fatalf("ListByOrganization: %v", err)
			}
			if len(plans) != tt.wantPage {
				t.Errorf("page size = %d, want %d", len(plans), tt.wantPage)
			}
			total, err := repo.CountByOrganization("org-1", tt.opts.Search)
			if err != nil {
				t.Fatalf("CountByOrganization: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}
