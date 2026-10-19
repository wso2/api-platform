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

package service_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// listingWithDraft publishes a listing that selects the seeded plan and document, then saves a
// draft selecting the same two, so both the live row and the draft hold a mapping to each.
func listingWithDraft(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph) (apiType, apiHandle, portalHandle string) {
	t.Helper()
	apiType, apiHandle, portalHandle = publishedListing(t, tdb, svc, g)
	draft := &model.Publication{DisplayName: "Next", Version: "2.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "editor", draft, []string{planHandleFor(g)}, []string{docHandleFor(g)}); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	return apiType, apiHandle, portalHandle
}

func deletePlanAndDocument(t *testing.T, tdb *schemaDB, g graph) {
	t.Helper()
	tdb.exec(t, `DELETE FROM subscription_plans WHERE uuid = ?`, g.plan)
	tdb.exec(t, `DELETE FROM api_documents WHERE uuid = ?`, g.apiDoc)
}

func selectionByID(selections []model.PublicationSelection, id string) *model.PublicationSelection {
	for i := range selections {
		if selections[i].Handle == id {
			return &selections[i]
		}
	}
	return nil
}

func TestPublicationSelections_SurviveDeletionOfTheSource(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)

	deletePlanAndDocument(t, tdb, g)

	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	draft, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}
	for name, pub := range map[string]*model.Publication{"live": live, "draft": draft} {
		plan := selectionByID(pub.SubscriptionPlans, planHandleFor(g))
		doc := selectionByID(pub.Documents, docHandleFor(g))
		if plan == nil || !plan.Deleted || plan.DisplayName == "" {
			t.Errorf("%s: want the plan kept with its stored name and marked deleted, got %+v", name, pub.SubscriptionPlans)
		}
		if doc == nil || !doc.Deleted || doc.DisplayName == "" {
			t.Errorf("%s: want the document kept with its stored name and marked deleted, got %+v", name, pub.Documents)
		}
		if !reflect.DeepEqual(pub.SubscriptionPlanIds, []string{planHandleFor(g)}) || !reflect.DeepEqual(pub.DocIds, []string{docHandleFor(g)}) {
			t.Errorf("%s: want the id lists to keep the deleted handles, got %v and %v", name, pub.SubscriptionPlanIds, pub.DocIds)
		}
	}
}

func TestPublicationSelections_SurviveThePortalDeletingThePlan(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)
	plans := service.NewSubscriptionPlanService(repository.NewSubscriptionPlanRepo(tdb.db), nil, nil, nil, nil, nil)

	if err := plans.DeletePlanFromWebhook(context.Background(), g.org, planHandleFor(g), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("DeletePlanFromWebhook failed: %v", err)
	}

	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	draft, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}
	for name, pub := range map[string]*model.Publication{"live": live, "draft": draft} {
		plan := selectionByID(pub.SubscriptionPlans, planHandleFor(g))
		if plan == nil || !plan.Deleted || plan.DisplayName == "" {
			t.Errorf("%s: want the plan kept with its stored name and marked deleted, got %+v", name, pub.SubscriptionPlans)
		}
	}
}

func TestPublicationSelections_ExistingSourceIsNotMarkedDeleted(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)

	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if len(live.SubscriptionPlans) != 1 || live.SubscriptionPlans[0].Deleted || len(live.Documents) != 1 || live.Documents[0].Deleted {
		t.Fatalf("want one plan and one document, neither deleted, got %+v and %+v", live.SubscriptionPlans, live.Documents)
	}
	if live.SubscriptionPlans[0].Edited || live.Documents[0].Edited {
		t.Fatalf("want nothing marked edited when the sources are untouched, got %+v and %+v", live.SubscriptionPlans, live.Documents)
	}
}

func TestPublicationDeprecate_KeepsADeletedPlanOnThePortalListing(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &deprecateRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	apiType, apiHandle, portalHandle := publishedListing(t, tdb, svc, g)

	deletePlanAndDocument(t, tdb, g)

	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "deprecator"); err != nil {
		t.Fatalf("Deprecate failed: %v", err)
	}
	if len(portal.pushed) != 1 || !reflect.DeepEqual(portal.pushed[0].SubscriptionPlanIds, []string{planHandleFor(g)}) {
		t.Fatalf("want the deleted plan's handle still sent to the portal, got %+v", portal.pushed)
	}
}

func TestPublish_RefusesADeletedPlan(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "editor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}

	deletePlanAndDocument(t, tdb, g)

	_, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher")
	if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), planHandleFor(g)) {
		t.Fatalf("want a validation error naming the deleted plan, got %v", err)
	}
	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil || live.Version != "1.0" {
		t.Fatalf("want the live listing untouched, got %+v, %v", live, err)
	}
}

func TestPublish_DropsADeletedDocument(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "editor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}

	tdb.exec(t, `DELETE FROM api_documents WHERE uuid = ?`, g.apiDoc)

	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if len(published.Documents) != 0 || len(published.DocIds) != 0 {
		t.Fatalf("want the deleted document dropped from the published listing, got %+v", published.Documents)
	}
	if got := tdb.count(t, "api_publication_doc_mappings", "organization_uuid", g.org); got != 0 {
		t.Fatalf("want no document mapping rows left, got %d", got)
	}
	if len(published.SubscriptionPlans) != 1 {
		t.Fatalf("want the plan kept, got %+v", published.SubscriptionPlans)
	}
}

func TestPublish_RefreshesTheStoredCopies(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "editor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}

	tdb.exec(t, `UPDATE subscription_plans SET display_name = 'Renamed plan' WHERE uuid = ?`, g.plan)
	tdb.exec(t, `UPDATE api_documents SET display_name = 'Renamed doc' WHERE uuid = ?`, g.apiDoc)

	before, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if before.SubscriptionPlans[0].DisplayName == "Renamed plan" || before.Documents[0].DisplayName == "Renamed doc" {
		t.Fatalf("want the live listing to keep the names it was published with, got %+v and %+v", before.SubscriptionPlans, before.Documents)
	}

	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if published.SubscriptionPlans[0].DisplayName != "Renamed plan" || published.Documents[0].DisplayName != "Renamed doc" {
		t.Fatalf("want the published response to carry the refreshed names, got %+v and %+v", published.SubscriptionPlans, published.Documents)
	}
	after, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if after.SubscriptionPlans[0].DisplayName != "Renamed plan" || after.Documents[0].DisplayName != "Renamed doc" {
		t.Fatalf("want the live listing to hold the refreshed names, got %+v and %+v", after.SubscriptionPlans, after.Documents)
	}
}

// touchPlanAndDocument records an edit to the seeded plan and document, as Settings and the
// document editor do, by moving their updated_at forward.
func touchPlanAndDocument(t *testing.T, tdb *schemaDB, g graph) {
	t.Helper()
	tdb.exec(t, `UPDATE subscription_plans SET updated_at = ? WHERE uuid = ?`, time.Now().UTC(), g.plan)
	tdb.exec(t, `UPDATE api_documents SET updated_at = ? WHERE uuid = ?`, time.Now().UTC(), g.apiDoc)
}

func editedFlags(pub *model.Publication) (plan, doc bool) {
	return pub.SubscriptionPlans[0].Edited, pub.Documents[0].Edited
}

func TestPublicationSelections_EditedAfterTheSelectionWasStored(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "editor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}

	touchPlanAndDocument(t, tdb, g)

	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	draft, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}
	for name, pub := range map[string]*model.Publication{"live": live, "draft": draft} {
		if plan, doc := editedFlags(pub); !plan || !doc {
			t.Errorf("%s: want the plan and document marked edited, got %+v and %+v", name, pub.SubscriptionPlans, pub.Documents)
		}
	}

	// Saving the draft takes a fresh snapshot of the draft only; the live listing still holds the old one.
	next := &model.Publication{DisplayName: "Next", Version: "2.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "editor", next, []string{planHandleFor(g)}, []string{docHandleFor(g)}); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	draft, err = svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}
	if plan, doc := editedFlags(draft); plan || doc {
		t.Errorf("draft: want the flags cleared by saving the draft, got %+v and %+v", draft.SubscriptionPlans, draft.Documents)
	}
	live, err = svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if plan, doc := editedFlags(live); !plan || !doc {
		t.Errorf("live: want the flags kept until the next publish, got %+v and %+v", live.SubscriptionPlans, live.Documents)
	}

	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if plan, doc := editedFlags(published); plan || doc {
		t.Errorf("published response: want nothing edited, got %+v and %+v", published.SubscriptionPlans, published.Documents)
	}
	live, err = svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if plan, doc := editedFlags(live); plan || doc {
		t.Errorf("live: want the flags cleared by publishing, got %+v and %+v", live.SubscriptionPlans, live.Documents)
	}
}

func TestPublish_ReadsAnEditMadeBeforeItAsPublished(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	apiType, apiHandle, portalHandle := listingWithDraft(t, tdb, svc, g)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "editor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}

	// Edited after the draft was saved but before it is published: the publish sends the new
	// version, so the live listing must not read as edited.
	touchPlanAndDocument(t, tdb, g)
	if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if plan, doc := editedFlags(live); plan || doc {
		t.Fatalf("want nothing edited after publishing the edited plan and document, got %+v and %+v", live.SubscriptionPlans, live.Documents)
	}
}
