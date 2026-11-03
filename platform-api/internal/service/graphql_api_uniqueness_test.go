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
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// GraphQL APIs are unique in an organization by id (handle) and by display
// name + version, exactly like REST APIs (validateCreateAPIRequest /
// validateUpdateAPIRequest) and like the gateway's own deploy-time check.

func graphQLConflictMessage(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperror.Error, got %T: %v", err, err)
	}
	if appErr.Code != apperror.CodeGraphQLAPIExists {
		t.Fatalf("expected %s, got %s", apperror.CodeGraphQLAPIExists, appErr.Code)
	}
	return appErr.Message
}

func newUniquenessCreateRequest() *api.CreateGraphQLAPIRequest {
	return &api.CreateGraphQLAPIRequest{
		Id:          graphQLStrPtr("countries-graphql-api"),
		DisplayName: "Countries GraphQL API",
		Context:     graphQLStrPtr("/countries"),
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}
}

func TestGraphQLCreate_DuplicateNameAndVersion_Conflict(t *testing.T) {
	repo := &mockGraphQLAPIRepo{nameVersionExists: true}
	svc := newGraphQLTestService(repo, &model.Project{ID: "project-uuid", OrganizationID: "org-1"})

	_, err := svc.Create("org-1", "creator-uuid", newUniquenessCreateRequest())

	if err == nil {
		t.Fatal("expected a conflict for a display name + version already in use")
	}
	if got, want := graphQLConflictMessage(t, err), graphQLAPINameVersionExistsMessage; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if repo.created != nil {
		t.Error("the API must not be created")
	}
	if len(repo.nameVersionChecks) != 1 {
		t.Fatalf("expected one name/version check, got %d", len(repo.nameVersionChecks))
	}
	if got := repo.nameVersionChecks[0]; got != (nameVersionCheck{"Countries GraphQL API", "v1.0", ""}) {
		t.Errorf("checked %+v, want the requested name/version with nothing excluded", got)
	}
}

func TestGraphQLCreate_DuplicateHandle_KeepsItsOwnMessage(t *testing.T) {
	// GraphQLAPIExists now carries a per-conflict message; an id clash must
	// still say it is the id, and is reported before name/version is checked.
	repo := &mockGraphQLAPIRepo{existsResult: true, nameVersionExists: true}
	svc := newGraphQLTestService(repo, &model.Project{ID: "project-uuid", OrganizationID: "org-1"})

	_, err := svc.Create("org-1", "creator-uuid", newUniquenessCreateRequest())

	if got, want := graphQLConflictMessage(t, err), graphQLAPIIDExistsMessage; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestGraphQLCreate_AvailableNameAndVersion_Creates(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	svc := newGraphQLTestService(repo, &model.Project{ID: "project-uuid", OrganizationID: "org-1"})

	if _, err := svc.Create("org-1", "creator-uuid", newUniquenessCreateRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected the API to be created")
	}
}

func newUniquenessUpdateFixture(nameVersionExists bool) (*mockGraphQLAPIRepo, *api.GraphQLAPI) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "cities-graphql-api",
		Name:           "Cities GraphQL API",
		Version:        "v1.0",
		OrganizationID: "org-1",
		ProjectID:      "project-uuid",
		Origin:         "control_plane",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL, IntrospectionMode: "SDL"},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc:   func(string, string) (*model.GraphQLAPI, error) { return stored, nil },
		nameVersionExists: nameVersionExists,
	}
	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     graphQLStrPtr("/cities"),
		Version:     "v2.0",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}
	return repo, req
}

func TestGraphQLUpdate_OntoAnotherAPIsNameAndVersion_Conflict(t *testing.T) {
	repo, req := newUniquenessUpdateFixture(true)
	svc := newGraphQLTestService(repo, nil)

	_, err := svc.Update("org-1", "cities-graphql-api", "updater-uuid", req)

	if err == nil {
		t.Fatal("expected a conflict for a display name + version another API uses")
	}
	if got, want := graphQLConflictMessage(t, err), graphQLAPINameVersionExistsMessage; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if repo.updated != nil {
		t.Error("the API must not be updated")
	}
	// The requested pair is checked — a GraphQL update may change the version
	// too — and the API being updated is excluded from it.
	if got := repo.nameVersionChecks[0]; got != (nameVersionCheck{"Countries GraphQL API", "v2.0", "cities-graphql-api"}) {
		t.Errorf("checked %+v, want the requested name/version excluding the API itself", got)
	}
}

func TestGraphQLUpdate_AvailableNameAndVersion_Updates(t *testing.T) {
	repo, req := newUniquenessUpdateFixture(false)
	svc := newGraphQLTestService(repo, nil)

	if _, err := svc.Update("org-1", "cities-graphql-api", "updater-uuid", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected the API to be updated")
	}
}
