/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package service

import (
	"testing"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// ─── Mock Repository ─────────────────────────────────────────────────────────

type mockWebSubAPIRepository struct {
	repository.WebSubAPIRepository
	store  map[string]*model.WebSubAPI
	exists bool
	count  int
}

func newMockWebSubAPIRepository() *mockWebSubAPIRepository {
	return &mockWebSubAPIRepository{store: make(map[string]*model.WebSubAPI)}
}

func (m *mockWebSubAPIRepository) Create(a *model.WebSubAPI) error {
	a.UUID = "test-uuid"
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	m.store[a.Handle] = a
	return nil
}
func (m *mockWebSubAPIRepository) GetByHandle(handle, _ string) (*model.WebSubAPI, error) {
	a, ok := m.store[handle]
	if !ok {
		return nil, nil
	}
	return a, nil
}
func (m *mockWebSubAPIRepository) Update(a *model.WebSubAPI) error {
	a.UpdatedAt = time.Now()
	m.store[a.Handle] = a
	return nil
}
func (m *mockWebSubAPIRepository) Delete(handle, _ string) error {
	delete(m.store, handle)
	return nil
}
func (m *mockWebSubAPIRepository) Exists(handle, _ string) (bool, error) { return m.exists, nil }
func (m *mockWebSubAPIRepository) Count(_ string) (int, error)           { return m.count, nil }
func (m *mockWebSubAPIRepository) CountByProject(_, _ string) (int, error) {
	return m.count, nil
}
func (m *mockWebSubAPIRepository) List(_, _ string, _, _ int) ([]*model.WebSubAPI, error) {
	result := make([]*model.WebSubAPI, 0, len(m.store))
	for _, v := range m.store {
		result = append(result, v)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func policySlice(name, version string) *[]api.Policy {
	return &[]api.Policy{{Name: name, Version: version}}
}

func buildCreateRequest() *api.WebSubAPI {
	handle := "repo-watcher-v1-0"
	ctx := "/repos"
	return &api.WebSubAPI{
		Id:        &handle,
		Name:      "repo-watcher",
		Version:   "v1.0",
		ProjectId: "project-uuid",
		Context:   &ctx,
		Upstream:  api.Upstream{},
		Policies: &api.WebSubChannelPolicies{
			OnSubscription:    policySlice("api-key-auth", "v1"),
			OnUnsubscription:  &[]api.Policy{},
			OnMessageReceived: policySlice("websub-hmac-auth", "v1"),
			OnMessageDelivery: &[]api.Policy{},
		},
		Channels: &map[string]api.WebSubChannel{
			"issues": {
				Policies: &api.WebSubChannelPolicies{
					OnSubscription:    &[]api.Policy{},
					OnUnsubscription:  &[]api.Policy{},
					OnMessageReceived: &[]api.Policy{},
					OnMessageDelivery: &[]api.Policy{},
				},
			},
		},
	}
}

func buildService(repo *mockWebSubAPIRepository) *WebSubAPIService {
	return &WebSubAPIService{
		repo:    repo,
		apiUtil: &utils.APIUtil{},
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestWebSubAPI_PoliciesStoredAsAllChannels verifies that the flat "policies"
// sent by the client are converted to AllChannels in the stored model.
func TestWebSubAPI_PoliciesStoredAsAllChannels(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["repo-watcher-v1-0"]
	if stored == nil {
		t.Fatal("API not found in store after Create")
	}

	if stored.Configuration.AllChannels == nil {
		t.Fatal("AllChannels should not be nil after storing flat policies")
	}

	ac := stored.Configuration.AllChannels
	if ac.OnSubscription == nil || len(ac.OnSubscription.Policies) != 1 {
		t.Errorf("expected 1 OnSubscription policy, got %v", ac.OnSubscription)
	}
	if ac.OnSubscription.Policies[0].Name != "api-key-auth" {
		t.Errorf("expected policy name 'api-key-auth', got %q", ac.OnSubscription.Policies[0].Name)
	}
	if ac.OnMessageReceived == nil || len(ac.OnMessageReceived.Policies) != 1 {
		t.Errorf("expected 1 OnMessageReceived policy, got %v", ac.OnMessageReceived)
	}
	if ac.OnMessageReceived.Policies[0].Name != "websub-hmac-auth" {
		t.Errorf("expected policy name 'websub-hmac-auth', got %q", ac.OnMessageReceived.Policies[0].Name)
	}
}

// TestWebSubAPI_GetReturnsFlatPolicies verifies that the stored AllChannels
// is returned as flat "policies" in the API response.
func TestWebSubAPI_GetReturnsFlatPolicies(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "repo-watcher-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.Policies == nil {
		t.Fatal("Get response should have Policies (not AllChannels)")
	}

	p := resp.Policies
	if p.OnSubscription == nil || len(*p.OnSubscription) != 1 {
		t.Errorf("expected 1 OnSubscription policy in response, got %v", p.OnSubscription)
	}
	if (*p.OnSubscription)[0].Name != "api-key-auth" {
		t.Errorf("expected policy name 'api-key-auth', got %q", (*p.OnSubscription)[0].Name)
	}
	if p.OnMessageReceived == nil || len(*p.OnMessageReceived) != 1 {
		t.Errorf("expected 1 OnMessageReceived policy in response, got %v", p.OnMessageReceived)
	}
	if (*p.OnMessageReceived)[0].Name != "websub-hmac-auth" {
		t.Errorf("expected policy name 'websub-hmac-auth', got %q", (*p.OnMessageReceived)[0].Name)
	}
}

// TestWebSubAPI_ChannelPoliciesStoredAndReturned verifies channel-level policies
// are stored and returned correctly with the new structure.
func TestWebSubAPI_ChannelPoliciesStoredAndReturned(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify stored model channel structure
	stored := repo.store["repo-watcher-v1-0"]
	if len(stored.Configuration.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(stored.Configuration.Channels))
	}
	ch, ok := stored.Configuration.Channels["issues"]
	if !ok {
		t.Fatal("'issues' channel not found in stored model")
	}
	// Empty slices (&[]api.Policy{}) are non-nil, so policySlicePtrToEventPolicies
	// returns a non-nil *WebSubEventPolicies with an empty Policies slice.
	if ch.OnSubscription == nil || len(ch.OnSubscription.Policies) != 0 {
		t.Errorf("expected ch.OnSubscription non-nil with empty policies, got %v", ch.OnSubscription)
	}
	if ch.OnUnsubscription == nil || len(ch.OnUnsubscription.Policies) != 0 {
		t.Errorf("expected ch.OnUnsubscription non-nil with empty policies, got %v", ch.OnUnsubscription)
	}
	if ch.OnMessageReceived == nil || len(ch.OnMessageReceived.Policies) != 0 {
		t.Errorf("expected ch.OnMessageReceived non-nil with empty policies, got %v", ch.OnMessageReceived)
	}
	if ch.OnMessageDelivery == nil || len(ch.OnMessageDelivery.Policies) != 0 {
		t.Errorf("expected ch.OnMessageDelivery non-nil with empty policies, got %v", ch.OnMessageDelivery)
	}

	// Verify API response channel structure
	resp, err := svc.Get("org-uuid", "repo-watcher-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.Channels == nil {
		t.Fatal("response Channels should not be nil")
	}
	_, ok = (*resp.Channels)["issues"]
	if !ok {
		t.Fatal("'issues' channel not found in response")
	}
}

// TestWebSubAPI_UpdatePolicies verifies that updating the API replaces AllChannels
// with new values from the incoming flat policies.
func TestWebSubAPI_UpdatePolicies(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update with different policies
	updateReq := buildCreateRequest()
	updateReq.Policies = &api.WebSubChannelPolicies{
		OnSubscription:    policySlice("jwt-auth", "v1"),
		OnUnsubscription:  nil,
		OnMessageReceived: nil,
		OnMessageDelivery: nil,
	}

	_, err = svc.Update("org-uuid", "repo-watcher-v1-0", updateReq)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "repo-watcher-v1-0")
	if err != nil {
		t.Fatalf("Get after Update failed: %v", err)
	}
	if resp.Policies == nil {
		t.Fatal("Policies should not be nil after update")
	}
	if resp.Policies.OnSubscription == nil || len(*resp.Policies.OnSubscription) != 1 {
		t.Errorf("expected 1 OnSubscription policy after update, got %v", resp.Policies.OnSubscription)
	}
	if (*resp.Policies.OnSubscription)[0].Name != "jwt-auth" {
		t.Errorf("expected updated policy name 'jwt-auth', got %q", (*resp.Policies.OnSubscription)[0].Name)
	}
	if resp.Policies.OnMessageReceived != nil {
		t.Errorf("expected OnMessageReceived to be nil after update, got %v", resp.Policies.OnMessageReceived)
	}
}

// TestWebSubAPI_Delete verifies that Delete removes the API.
func TestWebSubAPI_Delete(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = svc.Delete("org-uuid", "repo-watcher-v1-0")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, ok := repo.store["repo-watcher-v1-0"]; ok {
		t.Error("API should not exist in store after Delete")
	}
}

// TestWebSubAPI_List verifies that List returns the correct number of items.
func TestWebSubAPI_List(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	listResp, err := svc.List("org-uuid", "project-uuid", 20, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if listResp.Count != 1 {
		t.Errorf("expected 1 item in list, got %d", listResp.Count)
	}
}

// TestWebSubAPI_MapPoliciesAPIToAllChannels tests the low-level mapping helper.
func TestWebSubAPI_MapPoliciesAPIToAllChannels(t *testing.T) {
	in := &api.WebSubChannelPolicies{
		OnSubscription:    policySlice("api-key-auth", "v1"),
		OnMessageReceived: policySlice("websub-hmac-auth", "v1"),
	}
	out := mapWebSubPoliciesAPIToAllChannels(in)
	if out == nil {
		t.Fatal("expected non-nil AllChannels")
	}
	if out.OnSubscription == nil || len(out.OnSubscription.Policies) != 1 {
		t.Errorf("OnSubscription: expected 1 policy, got %v", out.OnSubscription)
	}
	if out.OnSubscription.Policies[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", out.OnSubscription.Policies[0].Name)
	}
	if out.OnUnsubscription != nil {
		t.Errorf("expected nil OnUnsubscription, got %v", out.OnUnsubscription)
	}
}

// TestWebSubAPI_MapAllChannelsModelToWebSubPolicies tests the reverse mapping helper.
func TestWebSubAPI_MapAllChannelsModelToWebSubPolicies(t *testing.T) {
	in := &model.WebSubAllChannelPolicies{
		OnSubscription: &model.WebSubEventPolicies{
			Policies: []model.Policy{{Name: "api-key-auth", Version: "v1"}},
		},
		OnMessageReceived: &model.WebSubEventPolicies{
			Policies: []model.Policy{{Name: "websub-hmac-auth", Version: "v1"}},
		},
	}
	out := mapAllChannelsModelToWebSubPolicies(in)
	if out == nil {
		t.Fatal("expected non-nil WebSubChannelPolicies")
	}
	if out.OnSubscription == nil || len(*out.OnSubscription) != 1 {
		t.Errorf("expected 1 OnSubscription policy, got %v", out.OnSubscription)
	}
	if (*out.OnSubscription)[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", (*out.OnSubscription)[0].Name)
	}
	if out.OnMessageReceived == nil || len(*out.OnMessageReceived) != 1 {
		t.Errorf("expected 1 OnMessageReceived policy, got %v", out.OnMessageReceived)
	}
	if out.OnUnsubscription != nil {
		t.Errorf("expected nil OnUnsubscription, got %v", out.OnUnsubscription)
	}
}

// TestWebSubAPI_NilPoliciesHandled ensures nil policies are handled gracefully.
func TestWebSubAPI_NilPoliciesHandled(t *testing.T) {
	if got := mapWebSubPoliciesAPIToAllChannels(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := mapAllChannelsModelToWebSubPolicies(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}
