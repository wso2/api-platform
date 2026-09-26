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

package service

import (
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// modelToAPIPortalListItem must emit updatedAt when the row has a non-zero
// UpdatedAt column, and omit it when zero. This locks the two branches so a
// future translate refactor cannot silently drop the field or emit a bogus
// zero timestamp.
func TestModelToAPIPortalListItem_UpdatedAtBranches(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	withUpdated := &model.APIPortal{
		Handle:    "acme",
		Name:      "Acme",
		URL:       "https://acme.example.com",
		CreatedAt: created,
		UpdatedAt: updated,
	}
	got := modelToAPIPortalListItem(withUpdated)
	if got.UpdatedAt == nil {
		t.Fatal("UpdatedAt must be emitted when the source column is non-zero")
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt round-trip mismatch: want %v got %v", updated, got.UpdatedAt)
	}

	withoutUpdated := &model.APIPortal{
		Handle:    "acme",
		Name:      "Acme",
		URL:       "https://acme.example.com",
		CreatedAt: created,
	}
	if gotZero := modelToAPIPortalListItem(withoutUpdated); gotZero.UpdatedAt != nil {
		t.Errorf("UpdatedAt must be nil when the source column is zero; got %v", gotZero.UpdatedAt)
	}
}
