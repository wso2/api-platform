/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// fakeUnpublisher records the artifact it was asked to unpublish and can fail.
type fakeUnpublisher struct {
	calls                       int
	apiType, handle, uuid, user string
	err                         error
}

func (f *fakeUnpublisher) UnpublishEverywhere(_ context.Context, apiType, apiHandle, artifactUUID, _, actor string) error {
	f.calls++
	f.apiType, f.handle, f.uuid, f.user = apiType, apiHandle, artifactUUID, actor
	return f.err
}

func newGraphQLDeleteTestService(repo *mockGraphQLAPIRepo) *GraphQLAPIService {
	return NewGraphQLAPIService(repo, &mockGraphQLProjectRepo{}, &noopAuditRepo{}, nil,
		&stubOrgGatewaysRepo{}, &mockOrganizationRepo{}, nil, newTestIdentityService(), slog.Default())
}

func storedGraphQL() *mockGraphQLAPIRepo {
	stored := &model.GraphQLAPI{ID: "graphql-uuid", Handle: "countries", OrganizationID: "org-1", Origin: "control_plane"}
	return &mockGraphQLAPIRepo{
		getByHandleFunc: func(string, string) (*model.GraphQLAPI, error) { return stored, nil },
	}
}

// Delete unpublishes the API from its portals, naming it as a GraphQL API, before
// it deletes the row.
func TestGraphQLDelete_UnpublishesFromPortalsFirst(t *testing.T) {
	repo := storedGraphQL()
	u := &fakeUnpublisher{}
	svc := newGraphQLDeleteTestService(repo)
	svc.SetArtifactUnpublisher(u)

	if err := svc.Delete("org-1", "countries", "deleter-uuid"); err != nil {
		t.Fatalf("Delete() = %v, want success", err)
	}
	if u.calls != 1 || u.apiType != "graphql-api" || u.handle != "countries" || u.uuid != "graphql-uuid" || u.user != "deleter-uuid" {
		t.Fatalf("unexpected unpublish call: %+v", u)
	}
	if !repo.deleted {
		t.Fatal("expected the repository Delete to be called")
	}
}

// A failed unpublish stops the delete, so the API is still there to retry.
func TestGraphQLDelete_KeepsTheAPIWhenUnpublishFails(t *testing.T) {
	repo := storedGraphQL()
	want := errors.New("portal unavailable")
	svc := newGraphQLDeleteTestService(repo)
	svc.SetArtifactUnpublisher(&fakeUnpublisher{err: want})

	if err := svc.Delete("org-1", "countries", "deleter-uuid"); !errors.Is(err, want) {
		t.Fatalf("Delete() = %v, want %v", err, want)
	}
	if repo.deleted {
		t.Fatal("expected the repository Delete never to be called")
	}
}

// Without an unpublisher (a build without API Portals) Delete works as before.
func TestGraphQLDelete_NoUnpublisher(t *testing.T) {
	repo := storedGraphQL()
	if err := newGraphQLDeleteTestService(repo).Delete("org-1", "countries", "deleter-uuid"); err != nil {
		t.Fatalf("Delete() = %v, want success", err)
	}
	if !repo.deleted {
		t.Fatal("expected the repository Delete to be called")
	}
}

// deleteTestAPIRepo serves one REST API and records whether it was deleted.
type deleteTestAPIRepo struct {
	repository.APIRepository
	api     *model.API
	deleted bool
}

func (r *deleteTestAPIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	return r.api, nil
}

func (r *deleteTestAPIRepo) DeleteAPI(apiUUID, orgUUID string) error {
	r.deleted = true
	return nil
}

func newRESTDeleteTestService(repo *deleteTestAPIRepo, u ArtifactUnpublisher) *APIService {
	svc := &APIService{apiRepo: repo, auditRepo: &noopAuditRepo{}, identity: newTestIdentityService(), slogger: newTestLogger()}
	svc.SetArtifactUnpublisher(u)
	return svc
}

// A REST API's delete unpublishes it as a rest-api before deleting it, and keeps it
// when the unpublish fails.
func TestRESTDelete_UnpublishesFromPortalsFirst(t *testing.T) {
	stored := &model.API{ID: "rest-uuid", Handle: "orders", OrganizationID: "org-1", Origin: "control_plane"}

	repo := &deleteTestAPIRepo{api: stored}
	u := &fakeUnpublisher{}
	if err := newRESTDeleteTestService(repo, u).DeleteAPI("rest-uuid", "org-1", "deleter-uuid"); err != nil {
		t.Fatalf("DeleteAPI() = %v, want success", err)
	}
	if u.calls != 1 || u.apiType != "rest-api" || u.handle != "orders" || u.uuid != "rest-uuid" || u.user != "deleter-uuid" {
		t.Fatalf("unexpected unpublish call: %+v", u)
	}
	if !repo.deleted {
		t.Fatal("expected the API to be deleted")
	}

	repo = &deleteTestAPIRepo{api: stored}
	want := errors.New("portal unavailable")
	if err := newRESTDeleteTestService(repo, &fakeUnpublisher{err: want}).DeleteAPI("rest-uuid", "org-1", "deleter-uuid"); !errors.Is(err, want) {
		t.Fatalf("DeleteAPI() = %v, want %v", err, want)
	}
	if repo.deleted {
		t.Fatal("expected the API to be kept when the unpublish fails")
	}
}

// deleteTestMCPRepo serves one MCP proxy and records whether it was deleted.
type deleteTestMCPRepo struct {
	repository.MCPProxyRepository
	proxy   *model.MCPProxy
	deleted bool
}

func (r *deleteTestMCPRepo) GetByHandle(handle, orgUUID string) (*model.MCPProxy, error) {
	return r.proxy, nil
}

func (r *deleteTestMCPRepo) Delete(handle, orgUUID string) error {
	r.deleted = true
	return nil
}

// An MCP proxy's delete unpublishes it as an mcp-proxy before deleting it, and keeps
// it when the unpublish fails.
func TestMCPProxyDelete_UnpublishesFromPortalsFirst(t *testing.T) {
	stored := &model.MCPProxy{UUID: "mcp-uuid", Handle: "weather", Origin: "control_plane"}
	newSvc := func(repo *deleteTestMCPRepo, u ArtifactUnpublisher) *MCPProxyService {
		svc := NewMCPProxyService(repo, nil, nil, nil, nil, newTestLogger(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
		svc.SetArtifactUnpublisher(u)
		return svc
	}

	repo := &deleteTestMCPRepo{proxy: stored}
	u := &fakeUnpublisher{}
	if err := newSvc(repo, u).Delete("org-1", "weather", "deleter-uuid"); err != nil {
		t.Fatalf("Delete() = %v, want success", err)
	}
	if u.calls != 1 || u.apiType != "mcp-proxy" || u.handle != "weather" || u.uuid != "mcp-uuid" || u.user != "deleter-uuid" {
		t.Fatalf("unexpected unpublish call: %+v", u)
	}
	if !repo.deleted {
		t.Fatal("expected the MCP proxy to be deleted")
	}

	repo = &deleteTestMCPRepo{proxy: stored}
	want := errors.New("portal unavailable")
	if err := newSvc(repo, &fakeUnpublisher{err: want}).Delete("org-1", "weather", "deleter-uuid"); !errors.Is(err, want) {
		t.Fatalf("Delete() = %v, want %v", err, want)
	}
	if repo.deleted {
		t.Fatal("expected the MCP proxy to be kept when the unpublish fails")
	}
}

// failingPublicationRepo fails the calls UnpublishEverywhere makes, one at a time.
type failingPublicationRepo struct {
	repository.PublicationRepository
	rows         []*model.PublicationStatusRow
	listErr      error
	unpublishErr error
}

func (r *failingPublicationRepo) ListStatusByArtifact(artifactUUID, orgUUID string) ([]*model.PublicationStatusRow, error) {
	return r.rows, r.listErr
}

func (r *failingPublicationRepo) UnpublishPublication(artifactUUID, apiPortalUUID, orgUUID, actor string) (bool, error) {
	return r.unpublishErr == nil, r.unpublishErr
}

// stubPortalRepo returns one portal, or an error.
type stubPortalRepo struct {
	repository.APIPortalRepository
	portal *model.APIPortal
	err    error
}

func (r *stubPortalRepo) GetByUUID(uuid, orgUUID string) (*model.APIPortal, error) {
	return r.portal, r.err
}

// forceRecorder records which of Unpublish and ForceUnpublish was called.
type forceRecorder struct {
	PortalPublisher
	forced, plain int
}

func (f *forceRecorder) Unpublish(context.Context, *model.APIPortal, string, string) error {
	f.plain++
	return nil
}

func (f *forceRecorder) ForceUnpublish(context.Context, *model.APIPortal, string, string) error {
	f.forced++
	return nil
}

func liveRow() *model.PublicationStatusRow {
	return &model.PublicationStatusRow{APIPortalUUID: "portal-uuid", Status: model.PublicationStatusPublished}
}

func activePortal() *model.APIPortal {
	return &model.APIPortal{ID: "portal-uuid", Handle: "public", Status: "active"}
}

// Each repository failure stops UnpublishEverywhere with an error; a listing whose
// portal no longer exists is skipped.
func TestUnpublishEverywhere_RepositoryFailures(t *testing.T) {
	boom := errors.New("db down")
	cases := []struct {
		name    string
		pubs    *failingPublicationRepo
		portals *stubPortalRepo
		wantErr bool
	}{
		{"listing the publications fails", &failingPublicationRepo{listErr: boom}, &stubPortalRepo{}, true},
		{"resolving the portal fails", &failingPublicationRepo{rows: []*model.PublicationStatusRow{liveRow()}}, &stubPortalRepo{err: boom}, true},
		{"the portal is gone", &failingPublicationRepo{rows: []*model.PublicationStatusRow{liveRow()}}, &stubPortalRepo{}, false},
		{"recording the unpublish fails", &failingPublicationRepo{rows: []*model.PublicationStatusRow{liveRow()}, unpublishErr: boom}, &stubPortalRepo{portal: activePortal()}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewPublicationService(nil, tc.portals, nil, nil, tc.pubs, &forceRecorder{}, time.Minute, 0, newTestLogger())
			err := svc.UnpublishEverywhere(context.Background(), "rest-api", "orders", "rest-uuid", "org-1", "deleter")
			if (err != nil) != tc.wantErr {
				t.Fatalf("UnpublishEverywhere() = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// A publisher that can force removes the listing with its subscriptions and keys; one
// that can't falls back to a plain unpublish.
func TestRemoveFromPortal_ForcesWhenSupported(t *testing.T) {
	rec := &forceRecorder{}
	svc := NewPublicationService(nil, nil, nil, nil, nil, rec, time.Minute, 0, newTestLogger())
	if err := svc.removeFromPortal(context.Background(), activePortal(), "rest-api", "orders"); err != nil {
		t.Fatalf("removeFromPortal: %v", err)
	}
	if rec.forced != 1 || rec.plain != 0 {
		t.Fatalf("want one forced unpublish, got forced=%d plain=%d", rec.forced, rec.plain)
	}

	plain := &unpublishCounter{}
	svc = NewPublicationService(nil, nil, nil, nil, nil, plain, time.Minute, 0, newTestLogger())
	if err := svc.removeFromPortal(context.Background(), activePortal(), "rest-api", "orders"); err != nil {
		t.Fatalf("removeFromPortal: %v", err)
	}
	if plain.calls != 1 {
		t.Fatalf("want one plain unpublish, got %d", plain.calls)
	}
}

// unpublishCounter is a PortalPublisher without ForceUnpublish.
type unpublishCounter struct {
	PortalPublisher
	calls int
}

func (u *unpublishCounter) Unpublish(context.Context, *model.APIPortal, string, string) error {
	u.calls++
	return nil
}
