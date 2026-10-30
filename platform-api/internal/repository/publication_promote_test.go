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
)

// TestPromoteDraft_KeepsSelectionsRecreatedDuringPublish covers a plan and a document that were
// deleted and recreated under the same handle while the listing was pushed: they resolve to new
// UUIDs, and the promoted listing must keep a mapping for each rather than pruning the old rows
// and leaving none.
func TestPromoteDraft_KeepsSelectionsRecreatedDuringPublish(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	repo := NewPublicationRepo(tdb.db)

	oldPlans := []model.PublicationSelection{{UUID: g.plan, Handle: "plan", DisplayName: "Plan"}}
	oldDocs := []model.PublicationSelection{{UUID: g.apiDoc, Handle: "doc", DisplayName: "Doc"}}
	draft := &model.Publication{
		OrganizationUUID: g.org,
		ArtifactUUID:     g.apiArtifact,
		APIPortalUUID:    g.apiPortal,
		DisplayName:      "Listing",
		Version:          "1.0.0",
		AgentVisibility:  "VISIBLE",
	}
	saved, err := repo.SaveDraftDetails(draft, oldPlans, oldDocs, "actor")
	if err != nil {
		t.Fatalf("saving draft: %v", err)
	}

	newPlan, newDoc := id(), id()
	newPlans := []model.PublicationSelection{{UUID: newPlan, Handle: "plan", DisplayName: "Plan renamed"}}
	newDocs := []model.PublicationSelection{{UUID: newDoc, Handle: "doc", DisplayName: "Doc renamed"}}
	if _, _, err := repo.PromoteDraftToPublication(g.apiArtifact, g.apiPortal, g.org, "actor", saved.UpdatedAt, time.Now().UTC(), newPlans, newDocs); err != nil {
		t.Fatalf("promoting draft: %v", err)
	}

	for table, want := range map[string]struct{ column, kept, dropped string }{
		"api_publication_plan_mappings": {"subscription_plan_uuid", newPlan, g.plan},
		"api_publication_doc_mappings":  {"doc_uuid", newDoc, g.apiDoc},
	} {
		if got := tdb.count(t, table, want.column, want.kept); got != 1 {
			t.Errorf("%s rows for the recreated selection = %d, want 1", table, got)
		}
		if got := tdb.count(t, table, want.column, want.dropped); got != 0 {
			t.Errorf("%s rows for the replaced selection = %d, want 0", table, got)
		}
	}

	_, plans, docs, err := repo.GetPublication(g.apiArtifact, g.apiPortal, g.org)
	if err != nil {
		t.Fatalf("reading publication: %v", err)
	}
	if len(plans) != 1 || plans[0].DisplayName != "Plan renamed" {
		t.Errorf("plan selections = %+v, want the recreated plan", plans)
	}
	if len(docs) != 1 || docs[0].DisplayName != "Doc renamed" {
		t.Errorf("document selections = %+v, want the recreated document", docs)
	}
}
