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
	"errors"
	"reflect"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const scopeTestOrg = "org1"

// fakeScopeResolver resolves the fixed projects and artifacts below.
//
//	project "payments"  -> proj-pay
//	project "billing"   -> proj-bill
//	RestApi "orders"    -> art-orders  (project payments)
//	LlmProvider "openai"-> art-openai  (no project)
type fakeScopeResolver struct{}

var (
	fakeProjects  = map[string]string{"payments": "proj-pay", "billing": "proj-bill"}
	fakeArtifacts = map[string]struct{ kind, handle, project string }{
		"art-orders": {constants.RestApi, "orders", "proj-pay"},
		"art-openai": {constants.LLMProvider, "openai", ""},
	}
)

func (fakeScopeResolver) ProjectUUID(_, handle string) (string, error) {
	if id, ok := fakeProjects[handle]; ok {
		return id, nil
	}
	return "", apperror.ProjectNotFound.New()
}

func (fakeScopeResolver) ProjectHandle(_, uuid string) (string, error) {
	for h, id := range fakeProjects {
		if id == uuid {
			return h, nil
		}
	}
	return "", apperror.ProjectNotFound.New()
}

func (fakeScopeResolver) ArtifactUUID(_, kind, handle string) (string, error) {
	for id, a := range fakeArtifacts {
		if a.kind == kind && a.handle == handle {
			return id, nil
		}
	}
	return "", errSecretScopeArtifactNotFound
}

func (fakeScopeResolver) ArtifactKindAndHandle(_, uuid string) (string, string, error) {
	if a, ok := fakeArtifacts[uuid]; ok {
		return a.kind, a.handle, nil
	}
	return "", "", errSecretScopeArtifactNotFound
}

func (fakeScopeResolver) ArtifactProjectUUID(_, uuid string) (string, error) {
	return fakeArtifacts[uuid].project, nil
}

func newScopedSecretService(repo *mockSecretRepo) *SecretService {
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	svc.SetScopeResolver(fakeScopeResolver{})
	return svc
}

func orgScope() model.SecretScope {
	return model.SecretScope{Scope: model.SecretScopeTypeOrg, ScopeValue: scopeTestOrg}
}

func projectScope(id string) model.SecretScope {
	return model.SecretScope{Scope: model.SecretScopeTypeProject, ScopeValue: id}
}

func artifactScope(id string) model.SecretScope {
	return model.SecretScope{Scope: model.SecretScopeTypeArtifact, ScopeValue: id}
}

// ---- parseSecretScopes -------------------------------------------------------

func TestParseSecretScopes_Valid(t *testing.T) {
	svc := newScopedSecretService(newMockRepo())
	got, err := svc.parseSecretScopes(scopeTestOrg, []string{
		"project:payments", " artifact:LlmProvider:openai ", "project:payments", "",
	})
	if err != nil {
		t.Fatalf("parseSecretScopes: %v", err)
	}
	want := []model.SecretScope{projectScope("proj-pay"), artifactScope("art-openai")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scopes = %+v, want %+v", got, want)
	}
}

func TestParseSecretScopes_OrgAlone(t *testing.T) {
	svc := newScopedSecretService(newMockRepo())
	got, err := svc.parseSecretScopes(scopeTestOrg, []string{"org", "org"})
	if err != nil {
		t.Fatalf("parseSecretScopes: %v", err)
	}
	if !reflect.DeepEqual(got, []model.SecretScope{orgScope()}) {
		t.Errorf("scopes = %+v, want the org scope", got)
	}
}

func TestParseSecretScopes_Rejects(t *testing.T) {
	cases := map[string][]string{
		"org with project":    {"org", "project:payments"},
		"unknown prefix":      {"team:payments"},
		"empty project":       {"project:"},
		"artifact no kind":    {"artifact:orders"},
		"unsupported kind":    {"artifact:WebSubApi:orders"},
		"unknown project":     {"project:nope"},
		"unknown artifact":    {"artifact:RestApi:nope"},
		"artifact wrong kind": {"artifact:GraphQLApi:orders"},
	}
	svc := newScopedSecretService(newMockRepo())
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.parseSecretScopes(scopeTestOrg, raw)
			if !apperror.ValidationFailed.Is(err) {
				t.Errorf("err = %v, want ValidationFailed", err)
			}
		})
	}
}

func TestParseSecretScopes_NoResolver(t *testing.T) {
	svc := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	if _, err := svc.parseSecretScopes(scopeTestOrg, []string{"org"}); err != nil {
		t.Errorf("org scope without a resolver: %v", err)
	}
	if _, err := svc.parseSecretScopes(scopeTestOrg, []string{"project:payments"}); !apperror.ValidationFailed.Is(err) {
		t.Errorf("project scope without a resolver: err = %v, want ValidationFailed", err)
	}
}

func TestParseSecretScopes_ResolverFailureIsNotValidation(t *testing.T) {
	svc := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	svc.SetScopeResolver(failingResolver{})
	_, err := svc.parseSecretScopes(scopeTestOrg, []string{"project:payments"})
	if err == nil || apperror.ValidationFailed.Is(err) {
		t.Errorf("err = %v, want a non-validation error", err)
	}
}

type failingResolver struct{ fakeScopeResolver }

func (failingResolver) ProjectUUID(_, _ string) (string, error) { return "", errors.New("db down") }

// ---- scopeAdmits -------------------------------------------------------------

func TestScopeAdmits(t *testing.T) {
	ctxProj := SecretRefContext{ProjectUUID: "proj-pay"}
	ctxArt := SecretRefContext{ProjectUUID: "proj-pay", ArtifactUUID: "art-orders"}
	cases := []struct {
		name   string
		scopes []model.SecretScope
		ctx    SecretRefContext
		want   bool
	}{
		{"org, empty context", []model.SecretScope{orgScope()}, SecretRefContext{}, true},
		{"other org", []model.SecretScope{{Scope: model.SecretScopeTypeOrg, ScopeValue: "org2"}}, ctxArt, false},
		{"project match", []model.SecretScope{projectScope("proj-pay")}, ctxProj, true},
		{"project mismatch", []model.SecretScope{projectScope("proj-bill")}, ctxArt, false},
		{"project, empty context", []model.SecretScope{projectScope("proj-pay")}, SecretRefContext{}, false},
		{"artifact match", []model.SecretScope{artifactScope("art-orders")}, ctxArt, true},
		{"artifact, create (no UUID yet)", []model.SecretScope{artifactScope("art-orders")}, ctxProj, false},
		{"artifact, empty value", []model.SecretScope{artifactScope("")}, SecretRefContext{}, false},
		{"one of several", []model.SecretScope{projectScope("proj-bill"), artifactScope("art-orders")}, ctxArt, true},
		{"legacy, no scope rows", nil, SecretRefContext{}, true},
	}
	for _, c := range cases {
		if got := scopeAdmits(scopeTestOrg, c.scopes, c.ctx); got != c.want {
			t.Errorf("%s: scopeAdmits = %v, want %v", c.name, got, c.want)
		}
	}
}

// ---- ValidateSecretRefsFor ---------------------------------------------------

func scopedRepo() *mockSecretRepo {
	repo := newMockRepo()
	repo.secrets["shared"] = &model.Secret{UUID: "s1", Handle: "shared", Status: model.SecretStatusActive,
		Scopes: []model.SecretScope{orgScope()}}
	repo.secrets["pay-key"] = &model.Secret{UUID: "s2", Handle: "pay-key", Status: model.SecretStatusActive,
		Scopes: []model.SecretScope{projectScope("proj-pay")}}
	repo.secrets["orders-key"] = &model.Secret{UUID: "s3", Handle: "orders-key", Status: model.SecretStatusActive,
		Scopes: []model.SecretScope{artifactScope("art-orders")}}
	repo.secrets["old"] = &model.Secret{UUID: "s4", Handle: "old", Status: model.SecretStatusDeprecated,
		Scopes: []model.SecretScope{orgScope()}}
	return repo
}

func TestValidateSecretRefsFor(t *testing.T) {
	svc := newScopedSecretService(scopedRepo())
	orders := SecretRefContext{ProjectUUID: "proj-pay", ArtifactUUID: "art-orders"}
	billing := SecretRefContext{ProjectUUID: "proj-bill", ArtifactUUID: "art-other"}
	newInPay := SecretRefContext{ProjectUUID: "proj-pay"}
	cases := []struct {
		name   string
		handle string
		ctx    SecretRefContext
		ok     bool
	}{
		{"org-wide, any artifact", "shared", billing, true},
		{"org-wide, no context", "shared", SecretRefContext{}, true},
		{"project secret, same project", "pay-key", orders, true},
		{"project secret, new artifact in project", "pay-key", newInPay, true},
		{"project secret, other project", "pay-key", billing, false},
		{"project secret, no context", "pay-key", SecretRefContext{}, false},
		{"artifact secret, that artifact", "orders-key", orders, true},
		{"artifact secret, same project other artifact", "orders-key", newInPay, false},
		{"deprecated secret", "old", orders, false},
		{"missing secret", "nope", orders, false},
	}
	for _, c := range cases {
		err := svc.ValidateSecretRefsFor(scopeTestOrg, c.ctx, `{{ secret "`+c.handle+`" }}`)
		if c.ok && err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
		if !c.ok && !apperror.ValidationFailed.Is(err) {
			t.Errorf("%s: err = %v, want ValidationFailed", c.name, err)
		}
	}
}

// A secret that exists but is out of scope must be reported exactly as one
// that does not exist, so a caller cannot probe other projects' secrets.
func TestValidateSecretRefsFor_DoesNotRevealExistence(t *testing.T) {
	repo := scopedRepo()
	repo.secrets["hidden"] = &model.Secret{UUID: "s5", Handle: "hidden", Status: model.SecretStatusActive,
		Scopes: []model.SecretScope{projectScope("proj-bill")}}
	svc := newScopedSecretService(repo)
	ctx := SecretRefContext{ProjectUUID: "proj-pay"}

	outOfScope := svc.ValidateSecretRefsFor(scopeTestOrg, ctx, `{{ secret "hidden" }}`)
	delete(repo.secrets, "hidden")
	missing := svc.ValidateSecretRefsFor(scopeTestOrg, ctx, `{{ secret "hidden" }}`)
	if outOfScope == nil || missing == nil {
		t.Fatalf("expected both to fail: outOfScope=%v missing=%v", outOfScope, missing)
	}
	if outOfScope.Error() != missing.Error() {
		t.Errorf("out-of-scope error %q differs from missing error %q", outOfScope, missing)
	}
}

func TestValidateSecretRefs_LegacyEntryPointIsOrgOnly(t *testing.T) {
	svc := newScopedSecretService(scopedRepo())
	if err := svc.ValidateSecretRefs(scopeTestOrg, `{{ secret "shared" }}`); err != nil {
		t.Errorf("org-wide secret: %v", err)
	}
	if err := svc.ValidateSecretRefs(scopeTestOrg, `{{ secret "pay-key" }}`); !apperror.ValidationFailed.Is(err) {
		t.Errorf("project secret without context: err = %v, want ValidationFailed", err)
	}
}

// ---- Create / Update ---------------------------------------------------------

func TestSecretService_Create_DefaultsToOrgScope(t *testing.T) {
	repo := newMockRepo()
	svc := newScopedSecretService(repo)
	resp, err := svc.Create(scopeTestOrg, "alice", &dto.CreateSecretRequest{Handle: "k", DisplayName: "K", Value: "v"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !reflect.DeepEqual(repo.secrets["k"].Scopes, []model.SecretScope{orgScope()}) {
		t.Errorf("stored scopes = %+v, want the org scope", repo.secrets["k"].Scopes)
	}
	if !reflect.DeepEqual(resp.Scopes, []string{"org"}) {
		t.Errorf("response scopes = %v, want [org]", resp.Scopes)
	}
}

func TestSecretService_Create_WithScopes(t *testing.T) {
	repo := newMockRepo()
	svc := newScopedSecretService(repo)
	resp, err := svc.Create(scopeTestOrg, "alice", &dto.CreateSecretRequest{
		Handle: "k", DisplayName: "K", Value: "v",
		Scopes: []string{"project:payments", "artifact:RestApi:orders"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := []model.SecretScope{projectScope("proj-pay"), artifactScope("art-orders")}
	if !reflect.DeepEqual(repo.secrets["k"].Scopes, want) {
		t.Errorf("stored scopes = %+v, want %+v", repo.secrets["k"].Scopes, want)
	}
	if !reflect.DeepEqual(resp.Scopes, []string{"artifact:RestApi:orders", "project:payments"}) {
		t.Errorf("response scopes = %v", resp.Scopes)
	}
}

func TestSecretService_Create_InvalidScopeStoresNothing(t *testing.T) {
	repo := newMockRepo()
	svc := newScopedSecretService(repo)
	_, err := svc.Create(scopeTestOrg, "alice", &dto.CreateSecretRequest{
		Handle: "k", DisplayName: "K", Value: "v", Scopes: []string{"project:nope"},
	})
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("err = %v, want ValidationFailed", err)
	}
	if _, ok := repo.secrets["k"]; ok {
		t.Error("secret was stored despite an invalid scope")
	}
}

func TestSecretService_Update_KeepsScopesWhenAbsent(t *testing.T) {
	repo := scopedRepo()
	svc := newScopedSecretService(repo)
	if _, err := svc.Update(scopeTestOrg, "pay-key", "bob", &dto.UpdateSecretRequest{Value: "new"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !reflect.DeepEqual(repo.secrets["pay-key"].Scopes, []model.SecretScope{projectScope("proj-pay")}) {
		t.Errorf("scopes changed to %+v", repo.secrets["pay-key"].Scopes)
	}
}

func TestSecretService_Update_ReplacesScopes(t *testing.T) {
	repo := scopedRepo()
	svc := newScopedSecretService(repo)
	resp, err := svc.Update(scopeTestOrg, "shared", "bob", &dto.UpdateSecretRequest{
		Value: "new", Scopes: []string{"project:billing"}, ScopesSet: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !reflect.DeepEqual(repo.secrets["shared"].Scopes, []model.SecretScope{projectScope("proj-bill")}) {
		t.Errorf("scopes = %+v, want project billing", repo.secrets["shared"].Scopes)
	}
	if !reflect.DeepEqual(resp.Scopes, []string{"project:billing"}) {
		t.Errorf("response scopes = %v", resp.Scopes)
	}
}

func TestSecretService_Update_EmptyScopesMeansOrg(t *testing.T) {
	repo := scopedRepo()
	svc := newScopedSecretService(repo)
	if _, err := svc.Update(scopeTestOrg, "pay-key", "bob", &dto.UpdateSecretRequest{
		Value: "new", ScopesSet: true,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !reflect.DeepEqual(repo.secrets["pay-key"].Scopes, []model.SecretScope{orgScope()}) {
		t.Errorf("scopes = %+v, want the org scope", repo.secrets["pay-key"].Scopes)
	}
}

func TestSecretService_Update_NarrowingGuard(t *testing.T) {
	cases := []struct {
		name   string
		refs   []string
		scopes []string
		ok     bool
	}{
		{"excludes the referencing artifact", []string{"art-orders"}, []string{"project:billing"}, false},
		{"keeps it via its project", []string{"art-orders"}, []string{"project:payments"}, true},
		{"keeps it via the artifact", []string{"art-orders"}, []string{"artifact:RestApi:orders"}, true},
		{"org keeps everything", []string{"art-orders", "art-openai"}, []string{"org"}, true},
		{"a provider has no project", []string{"art-orders", "art-openai"}, []string{"project:payments"}, false},
		{"a provider needs its artifact scope", []string{"art-orders", "art-openai"},
			[]string{"project:payments", "artifact:LlmProvider:openai"}, true},
		{"no references", nil, []string{"project:billing"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := scopedRepo()
			repo.refArtifactsFn = func(_, handle string) ([]string, error) {
				if handle != "shared" {
					t.Fatalf("FindRefArtifactUUIDs handle = %q", handle)
				}
				return c.refs, nil
			}
			svc := newScopedSecretService(repo)
			_, err := svc.Update(scopeTestOrg, "shared", "bob", &dto.UpdateSecretRequest{
				Value: "new", Scopes: c.scopes, ScopesSet: true,
			})
			if c.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !c.ok {
				if !apperror.ValidationFailed.Is(err) {
					t.Fatalf("err = %v, want ValidationFailed", err)
				}
				if !reflect.DeepEqual(repo.secrets["shared"].Scopes, []model.SecretScope{orgScope()}) {
					t.Errorf("scopes changed despite the refusal: %+v", repo.secrets["shared"].Scopes)
				}
			}
		})
	}
}

func TestDescribeSecretScopes_FallsBackToUUID(t *testing.T) {
	svc := newScopedSecretService(newMockRepo())
	got := svc.describeSecretScopes(scopeTestOrg, []model.SecretScope{
		orgScope(), projectScope("proj-pay"), projectScope("proj-gone"), artifactScope("art-openai"), artifactScope("art-gone"),
	})
	want := []string{"artifact:LlmProvider:openai", "artifact:art-gone", "org", "project:payments", "project:proj-gone"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("describeSecretScopes = %v, want %v", got, want)
	}
}
