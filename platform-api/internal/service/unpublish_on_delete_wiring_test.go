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

	"github.com/wso2/api-platform/platform-api/internal/model"
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
