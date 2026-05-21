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
	"fmt"
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
func (m *mockWebSubAPIRepository) List(_, _ string, limit, offset int) ([]*model.WebSubAPI, error) {
	// Collect all items
	all := make([]*model.WebSubAPI, 0, len(m.store))
	for _, v := range m.store {
		all = append(all, v)
	}

	// Apply pagination
	if offset >= len(all) {
		return []*model.WebSubAPI{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func eventPolicies(name, version string) *api.WebSubEventPolicies {
	return &api.WebSubEventPolicies{
		Policies: &[]api.Policy{{Name: name, Version: version}},
	}
}

func emptyEventPolicies() *api.WebSubEventPolicies {
	return &api.WebSubEventPolicies{
		Policies: &[]api.Policy{},
	}
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
		AllChannels: &api.WebSubAllChannelPolicies{
			OnSubscription:    eventPolicies("api-key-auth", "v1"),
			OnUnsubscription:  emptyEventPolicies(),
			OnMessageReceived: eventPolicies("websub-hmac-auth", "v1"),
			OnMessageDelivery: emptyEventPolicies(),
		},
		Channels: map[string]api.WebSubChannel{
			"issues": {
				OnSubscription:    emptyEventPolicies(),
				OnUnsubscription:  emptyEventPolicies(),
				OnMessageReceived: emptyEventPolicies(),
				OnMessageDelivery: emptyEventPolicies(),
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

// TestWebSubAPI_AllChannelsStoredCorrectly verifies that AllChannels
// sent by the client are stored correctly in the model.
func TestWebSubAPI_AllChannelsStoredCorrectly(t *testing.T) {
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
		t.Fatal("AllChannels should not be nil after storing")
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

// TestWebSubAPI_GetReturnsAllChannels verifies that the stored AllChannels
// is returned as AllChannels in the API response.
func TestWebSubAPI_GetReturnsAllChannels(t *testing.T) {
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
	if resp.AllChannels == nil {
		t.Fatal("Get response should have AllChannels")
	}

	ac := resp.AllChannels
	if ac.OnSubscription == nil || ac.OnSubscription.Policies == nil || len(*ac.OnSubscription.Policies) != 1 {
		t.Errorf("expected 1 OnSubscription policy in response, got %v", ac.OnSubscription)
	}
	if (*ac.OnSubscription.Policies)[0].Name != "api-key-auth" {
		t.Errorf("expected policy name 'api-key-auth', got %q", (*ac.OnSubscription.Policies)[0].Name)
	}
	if ac.OnMessageReceived == nil || ac.OnMessageReceived.Policies == nil || len(*ac.OnMessageReceived.Policies) != 1 {
		t.Errorf("expected 1 OnMessageReceived policy in response, got %v", ac.OnMessageReceived)
	}
	if (*ac.OnMessageReceived.Policies)[0].Name != "websub-hmac-auth" {
		t.Errorf("expected policy name 'websub-hmac-auth', got %q", (*ac.OnMessageReceived.Policies)[0].Name)
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
	// Empty policy slices result in non-nil *WebSubEventPolicies with empty Policies slice.
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
	_, ok = resp.Channels["issues"]
	if !ok {
		t.Fatal("'issues' channel not found in response")
	}
}

// TestWebSubAPI_UpdateAllChannels verifies that updating the API replaces AllChannels
// with new values.
func TestWebSubAPI_UpdateAllChannels(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	req := buildCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update with different policies
	updateReq := buildCreateRequest()
	updateReq.AllChannels = &api.WebSubAllChannelPolicies{
		OnSubscription:    eventPolicies("jwt-auth", "v1"),
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
	if resp.AllChannels == nil {
		t.Fatal("AllChannels should not be nil after update")
	}
	ac := resp.AllChannels
	if ac.OnSubscription == nil || ac.OnSubscription.Policies == nil || len(*ac.OnSubscription.Policies) != 1 {
		t.Errorf("expected 1 OnSubscription policy after update, got %v", ac.OnSubscription)
	}
	if (*ac.OnSubscription.Policies)[0].Name != "jwt-auth" {
		t.Errorf("expected updated policy name 'jwt-auth', got %q", (*ac.OnSubscription.Policies)[0].Name)
	}
	if ac.OnMessageReceived != nil {
		t.Errorf("expected OnMessageReceived to be nil after update, got %v", ac.OnMessageReceived)
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

// TestWebSubAPI_ListPagination verifies List pagination with limit and offset
func TestWebSubAPI_ListPagination(t *testing.T) {
	repo := newMockWebSubAPIRepository()
	svc := buildService(repo)

	// Create 5 APIs
	for i := 1; i <= 5; i++ {
		req := buildCreateRequest()
		name := fmt.Sprintf("api-%d", i)
		req.Name = name
		handle := fmt.Sprintf("api-%d-v1-0", i)
		req.Id = &handle
		_, err := svc.Create("org-uuid", "alice", req)
		if err != nil {
			t.Fatalf("Create API %d failed: %v", i, err)
		}
	}

	t.Run("limit smaller than total returns correct count", func(t *testing.T) {
		listResp, err := svc.List("org-uuid", "project-uuid", 2, 0)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if listResp.Count != 2 {
			t.Errorf("expected 2 items with limit=2, got %d", listResp.Count)
		}
		if len(listResp.List) != 2 {
			t.Errorf("expected 2 items in list array, got %d", len(listResp.List))
		}
	})

	t.Run("offset skips items correctly", func(t *testing.T) {
		listResp, err := svc.List("org-uuid", "project-uuid", 2, 2)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if listResp.Count != 2 {
			t.Errorf("expected 2 items with limit=2, offset=2, got %d", listResp.Count)
		}
	})

	t.Run("offset beyond total returns empty", func(t *testing.T) {
		listResp, err := svc.List("org-uuid", "project-uuid", 10, 10)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if listResp.Count != 0 {
			t.Errorf("expected 0 items with offset=10, got %d", listResp.Count)
		}
		if len(listResp.List) != 0 {
			t.Errorf("expected empty list array, got %d items", len(listResp.List))
		}
	})

	t.Run("limit larger than remaining returns all remaining", func(t *testing.T) {
		listResp, err := svc.List("org-uuid", "project-uuid", 10, 3)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if listResp.Count != 2 {
			t.Errorf("expected 2 remaining items (5 total - 3 offset), got %d", listResp.Count)
		}
	})

	t.Run("all items retrieved with correct pagination", func(t *testing.T) {
		// Get first page
		page1, err := svc.List("org-uuid", "project-uuid", 2, 0)
		if err != nil {
			t.Fatalf("List page 1 failed: %v", err)
		}
		// Get second page
		page2, err := svc.List("org-uuid", "project-uuid", 2, 2)
		if err != nil {
			t.Fatalf("List page 2 failed: %v", err)
		}
		// Get third page
		page3, err := svc.List("org-uuid", "project-uuid", 2, 4)
		if err != nil {
			t.Fatalf("List page 3 failed: %v", err)
		}

		total := page1.Count + page2.Count + page3.Count
		if total != 5 {
			t.Errorf("expected 5 total items across all pages, got %d", total)
		}
	})
}

// TestWebSubAPI_MapAllChannelPoliciesAPIToModel tests the low-level mapping helper.
func TestWebSubAPI_MapAllChannelPoliciesAPIToModel(t *testing.T) {
	in := &api.WebSubAllChannelPolicies{
		OnSubscription:    eventPolicies("api-key-auth", "v1"),
		OnMessageReceived: eventPolicies("websub-hmac-auth", "v1"),
	}
	out := mapWebSubAllChannelPoliciesAPIToModel(in)
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

// TestWebSubAPI_MapAllChannelPoliciesModelToAPI tests the reverse mapping helper.
func TestWebSubAPI_MapAllChannelPoliciesModelToAPI(t *testing.T) {
	in := &model.WebSubAllChannelPolicies{
		OnSubscription: &model.WebSubEventPolicies{
			Policies: []model.Policy{{Name: "api-key-auth", Version: "v1"}},
		},
		OnMessageReceived: &model.WebSubEventPolicies{
			Policies: []model.Policy{{Name: "websub-hmac-auth", Version: "v1"}},
		},
	}
	out := mapWebSubAllChannelPoliciesModelToAPI(in)
	if out == nil {
		t.Fatal("expected non-nil WebSubAllChannelPolicies")
	}
	if out.OnSubscription == nil || out.OnSubscription.Policies == nil || len(*out.OnSubscription.Policies) != 1 {
		t.Errorf("expected 1 OnSubscription policy, got %v", out.OnSubscription)
	}
	if (*out.OnSubscription.Policies)[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", (*out.OnSubscription.Policies)[0].Name)
	}
	if out.OnMessageReceived == nil || out.OnMessageReceived.Policies == nil || len(*out.OnMessageReceived.Policies) != 1 {
		t.Errorf("expected 1 OnMessageReceived policy, got %v", out.OnMessageReceived)
	}
	if out.OnUnsubscription != nil {
		t.Errorf("expected nil OnUnsubscription, got %v", out.OnUnsubscription)
	}
}

// TestWebSubAPI_NilPoliciesHandled ensures nil policies are handled gracefully.
func TestWebSubAPI_NilPoliciesHandled(t *testing.T) {
	if got := mapWebSubAllChannelPoliciesAPIToModel(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := mapWebSubAllChannelPoliciesModelToAPI(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

// TestWebSubAPI_MapModelToAPI_EmptyChannelsDoesNotPanic guards the nil-pointer
// dereference reported in #1995. Previously mapWebSubChannelsModelToAPI
// returned nil for empty/nil channel maps and the caller dereferenced that nil
// pointer when assigning to api.WebSubAPI.Channels (a value-type map). The Get,
// List, and Update return paths panicked for any WebSub API stored with an
// empty channels map.
func TestWebSubAPI_MapModelToAPI_EmptyChannelsDoesNotPanic(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]model.WebSubChannel
	}{
		{name: "nil channel map", in: nil},
		{name: "empty channel map", in: map[string]model.WebSubChannel{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("mapWebSubAPIModelToAPI panicked: %v", r)
				}
			}()

			m := &model.WebSubAPI{
				Handle:  "test",
				Name:    "test",
				Version: "v1",
				Configuration: model.WebSubAPIConfiguration{
					Channels: tc.in,
				},
			}

			got := mapWebSubAPIModelToAPI(m, &utils.APIUtil{})
			if got == nil {
				t.Fatal("expected non-nil WebSubAPI result")
			}
			if got.Channels == nil {
				t.Errorf("expected non-nil Channels map, got nil")
			}
			if len(got.Channels) != 0 {
				t.Errorf("expected empty Channels map, got %d entries", len(got.Channels))
			}
		})
	}
}

// TestWebSubAPI_MapChannelsModelToAPI_NeverReturnsNil ensures the helper itself
// always returns a usable (non-nil) map so callers do not need a nil guard
// before assigning the result to a value-type map field.
func TestWebSubAPI_MapChannelsModelToAPI_NeverReturnsNil(t *testing.T) {
	if got := mapWebSubChannelsModelToAPI(nil); got == nil {
		t.Errorf("expected non-nil map for nil input, got nil")
	}
	if got := mapWebSubChannelsModelToAPI(map[string]model.WebSubChannel{}); got == nil {
		t.Errorf("expected non-nil map for empty input, got nil")
	}
}
