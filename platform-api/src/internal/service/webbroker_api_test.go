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

type mockWebBrokerAPIRepository struct {
	repository.WebBrokerAPIRepository
	store  map[string]*model.WebBrokerAPI
	exists bool
	count  int
}

func newMockWebBrokerAPIRepository() *mockWebBrokerAPIRepository {
	return &mockWebBrokerAPIRepository{store: make(map[string]*model.WebBrokerAPI)}
}

func (m *mockWebBrokerAPIRepository) Create(a *model.WebBrokerAPI) error {
	a.UUID = "test-uuid"
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	m.store[a.Handle] = a
	return nil
}

func (m *mockWebBrokerAPIRepository) GetByHandle(handle, _ string) (*model.WebBrokerAPI, error) {
	a, ok := m.store[handle]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (m *mockWebBrokerAPIRepository) Update(a *model.WebBrokerAPI) error {
	a.UpdatedAt = time.Now()
	m.store[a.Handle] = a
	return nil
}

func (m *mockWebBrokerAPIRepository) Delete(handle, _ string) error {
	delete(m.store, handle)
	return nil
}

func (m *mockWebBrokerAPIRepository) Exists(handle, _ string) (bool, error) { return m.exists, nil }

func (m *mockWebBrokerAPIRepository) Count(_ string) (int, error) { return m.count, nil }

func (m *mockWebBrokerAPIRepository) CountByProject(_, _ string) (int, error) {
	return m.count, nil
}

func (m *mockWebBrokerAPIRepository) List(_, _ string, limit, offset int) ([]*model.WebBrokerAPI, error) {
	// Collect all items
	all := make([]*model.WebBrokerAPI, 0, len(m.store))
	for _, v := range m.store {
		all = append(all, v)
	}

	// Apply pagination
	if offset >= len(all) {
		return []*model.WebBrokerAPI{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func brokerEventPolicies(name, version string) *api.WebBrokerEventPolicies {
	return &api.WebBrokerEventPolicies{
		Policies: &[]api.Policy{{Name: name, Version: version}},
	}
}

func emptyBrokerEventPolicies() *api.WebBrokerEventPolicies {
	return &api.WebBrokerEventPolicies{
		Policies: &[]api.Policy{},
	}
}

func buildWebBrokerCreateRequest() *api.WebBrokerAPI {
	handle := "stock-trading-v1-0"
	ctx := "/trading"
	brokerType := api.Kafka
	receiverType := api.Websocket
	kafkaBootstrap := "kafka:9092"

	return &api.WebBrokerAPI{
		Id:        &handle,
		Name:      "stock-trading",
		Version:   "v1.0",
		ProjectId: "project-uuid",
		Context:   &ctx,
		Receiver: struct {
			Name string                       `json:"name" yaml:"name"`
			Type api.WebBrokerAPIReceiverType `json:"type" yaml:"type"`
		}{
			Name: "websocket-receiver",
			Type: receiverType,
		},
		Broker: struct {
			Name       string                     `json:"name" yaml:"name"`
			Properties *map[string]interface{}    `json:"properties,omitempty" yaml:"properties,omitempty"`
			Type       api.WebBrokerAPIBrokerType `json:"type" yaml:"type"`
		}{
			Name: "kafka-driver",
			Type: brokerType,
			Properties: &map[string]interface{}{
				"bootstrap.servers": kafkaBootstrap,
			},
		},
		AllChannels: &api.WebBrokerAllChannelPolicies{
			OnConnectionInit: brokerEventPolicies("api-key-auth", "v1"),
			OnProduce:        emptyBrokerEventPolicies(),
			OnConsume:        brokerEventPolicies("rate-limit", "v1"),
		},
		Channels: map[string]api.WebBrokerChannel{
			"stocks": {
				ProduceTo: &struct {
					Topic *string `json:"topic,omitempty" yaml:"topic,omitempty"`
				}{
					Topic: stringPtr("stock-updates"),
				},
				ConsumeFrom: &struct {
					Topic *string `json:"topic,omitempty" yaml:"topic,omitempty"`
				}{
					Topic: stringPtr("stock-orders"),
				},
				OnConnectionInit: emptyBrokerEventPolicies(),
				OnProduce:        emptyBrokerEventPolicies(),
				OnConsume:        emptyBrokerEventPolicies(),
			},
		},
	}
}

func buildWebBrokerService(repo *mockWebBrokerAPIRepository) *WebBrokerAPIService {
	return &WebBrokerAPIService{
		repo:    repo,
		apiUtil: &utils.APIUtil{},
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestWebBrokerAPI_Create verifies WebBroker API can be created with all required fields
func TestWebBrokerAPI_Create(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if stored == nil {
		t.Fatal("API not found in store after Create")
	}
	if stored.Name != "stock-trading" {
		t.Errorf("expected name 'stock-trading', got %q", stored.Name)
	}
	if stored.Version != "v1.0" {
		t.Errorf("expected version 'v1.0', got %q", stored.Version)
	}
}

// TestWebBrokerAPI_Get verifies Get returns the correct WebBroker API
func TestWebBrokerAPI_Get(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.Name != "stock-trading" {
		t.Errorf("expected name 'stock-trading', got %q", resp.Name)
	}
	if resp.Version != "v1.0" {
		t.Errorf("expected version 'v1.0', got %q", resp.Version)
	}
}

// TestWebBrokerAPI_Update verifies WebBroker API can be updated
func TestWebBrokerAPI_Update(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updateReq := buildWebBrokerCreateRequest()
	updateReq.Name = "updated-trading"
	_, err = svc.Update("org-uuid", "stock-trading-v1-0", updateReq)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get after Update failed: %v", err)
	}
	if resp.Name != "updated-trading" {
		t.Errorf("expected updated name 'updated-trading', got %q", resp.Name)
	}
}

// TestWebBrokerAPI_Delete verifies Delete removes the API
func TestWebBrokerAPI_Delete(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = svc.Delete("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, ok := repo.store["stock-trading-v1-0"]; ok {
		t.Error("API should not exist in store after Delete")
	}
}

// TestWebBrokerAPI_List verifies List returns the correct number of items
func TestWebBrokerAPI_List(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
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

// TestWebBrokerAPI_ListPagination verifies List pagination with limit and offset
func TestWebBrokerAPI_ListPagination(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	// Create 5 APIs
	for i := 1; i <= 5; i++ {
		req := buildWebBrokerCreateRequest()
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

// TestWebBrokerAPI_BrokerConfigStoredCorrectly verifies broker config is stored correctly
func TestWebBrokerAPI_BrokerConfigStoredCorrectly(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if stored.Configuration.Broker.Name != "kafka-driver" {
		t.Errorf("expected broker name 'kafka-driver', got %q", stored.Configuration.Broker.Name)
	}
	if stored.Configuration.Broker.Type != "kafka" {
		t.Errorf("expected broker type 'kafka', got %q", stored.Configuration.Broker.Type)
	}
	if stored.Configuration.Broker.Properties == nil {
		t.Fatal("broker properties should not be nil")
	}
	if bootstrap, ok := stored.Configuration.Broker.Properties["bootstrap.servers"]; !ok || bootstrap != "kafka:9092" {
		t.Errorf("expected bootstrap.servers 'kafka:9092', got %v", bootstrap)
	}
}

// TestWebBrokerAPI_GetReturnsBrokerConfig verifies Get returns complete broker configuration
func TestWebBrokerAPI_GetReturnsBrokerConfig(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.Broker.Name != "kafka-driver" {
		t.Errorf("expected broker name 'kafka-driver', got %q", resp.Broker.Name)
	}
	if resp.Broker.Type != api.Kafka {
		t.Errorf("expected broker type 'kafka', got %v", resp.Broker.Type)
	}
	if resp.Broker.Properties == nil {
		t.Fatal("broker properties should not be nil in response")
	}
}

// TestWebBrokerAPI_UpdateBrokerProperties verifies updating broker properties works
func TestWebBrokerAPI_UpdateBrokerProperties(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updateReq := buildWebBrokerCreateRequest()
	newBootstrap := "new-kafka:9092"
	updateReq.Broker.Properties = &map[string]interface{}{
		"bootstrap.servers": newBootstrap,
		"compression.type":  "gzip",
	}

	_, err = svc.Update("org-uuid", "stock-trading-v1-0", updateReq)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if bootstrap, ok := stored.Configuration.Broker.Properties["bootstrap.servers"]; !ok || bootstrap != newBootstrap {
		t.Errorf("expected updated bootstrap.servers '%s', got %v", newBootstrap, bootstrap)
	}
	if compression, ok := stored.Configuration.Broker.Properties["compression.type"]; !ok || compression != "gzip" {
		t.Errorf("expected compression.type 'gzip', got %v", compression)
	}
}

// TestWebBrokerAPI_ReceiverConfigStoredCorrectly verifies receiver config is stored correctly
func TestWebBrokerAPI_ReceiverConfigStoredCorrectly(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if stored.Configuration.Receiver.Name != "websocket-receiver" {
		t.Errorf("expected receiver name 'websocket-receiver', got %q", stored.Configuration.Receiver.Name)
	}
	if stored.Configuration.Receiver.Type != "websocket" {
		t.Errorf("expected receiver type 'websocket', got %q", stored.Configuration.Receiver.Type)
	}
}

// TestWebBrokerAPI_GetReturnsReceiverConfig verifies Get returns correct receiver configuration
func TestWebBrokerAPI_GetReturnsReceiverConfig(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.Receiver.Name != "websocket-receiver" {
		t.Errorf("expected receiver name 'websocket-receiver', got %q", resp.Receiver.Name)
	}
	if resp.Receiver.Type != api.Websocket {
		t.Errorf("expected receiver type 'websocket', got %v", resp.Receiver.Type)
	}
}

// TestWebBrokerAPI_AllChannelsStoredCorrectly verifies AllChannels policies are stored correctly
func TestWebBrokerAPI_AllChannelsStoredCorrectly(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if stored.Configuration.AllChannels == nil {
		t.Fatal("AllChannels should not be nil after storing")
	}

	ac := stored.Configuration.AllChannels
	if ac.OnConnectionInit == nil || len(ac.OnConnectionInit.Policies) != 1 {
		t.Errorf("expected 1 OnConnectionInit policy, got %v", ac.OnConnectionInit)
	}
	if ac.OnConnectionInit.Policies[0].Name != "api-key-auth" {
		t.Errorf("expected policy name 'api-key-auth', got %q", ac.OnConnectionInit.Policies[0].Name)
	}
	if ac.OnConsume == nil || len(ac.OnConsume.Policies) != 1 {
		t.Errorf("expected 1 OnConsume policy, got %v", ac.OnConsume)
	}
	if ac.OnConsume.Policies[0].Name != "rate-limit" {
		t.Errorf("expected policy name 'rate-limit', got %q", ac.OnConsume.Policies[0].Name)
	}
}

// TestWebBrokerAPI_GetReturnsAllChannels verifies Get returns AllChannels in the response
func TestWebBrokerAPI_GetReturnsAllChannels(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.AllChannels == nil {
		t.Fatal("Get response should have AllChannels")
	}

	ac := resp.AllChannels
	if ac.OnConnectionInit == nil || ac.OnConnectionInit.Policies == nil || len(*ac.OnConnectionInit.Policies) != 1 {
		t.Errorf("expected 1 OnConnectionInit policy in response, got %v", ac.OnConnectionInit)
	}
	if (*ac.OnConnectionInit.Policies)[0].Name != "api-key-auth" {
		t.Errorf("expected policy name 'api-key-auth', got %q", (*ac.OnConnectionInit.Policies)[0].Name)
	}
	if ac.OnConsume == nil || ac.OnConsume.Policies == nil || len(*ac.OnConsume.Policies) != 1 {
		t.Errorf("expected 1 OnConsume policy in response, got %v", ac.OnConsume)
	}
	if (*ac.OnConsume.Policies)[0].Name != "rate-limit" {
		t.Errorf("expected policy name 'rate-limit', got %q", (*ac.OnConsume.Policies)[0].Name)
	}
}

// TestWebBrokerAPI_UpdateAllChannels verifies updating AllChannels replaces with new values
func TestWebBrokerAPI_UpdateAllChannels(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updateReq := buildWebBrokerCreateRequest()
	updateReq.AllChannels = &api.WebBrokerAllChannelPolicies{
		OnConnectionInit: brokerEventPolicies("jwt-auth", "v1"),
		OnProduce:        brokerEventPolicies("throttle", "v1"),
		OnConsume:        nil,
	}

	_, err = svc.Update("org-uuid", "stock-trading-v1-0", updateReq)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get after Update failed: %v", err)
	}
	if resp.AllChannels == nil {
		t.Fatal("AllChannels should not be nil after update")
	}
	ac := resp.AllChannels
	if ac.OnConnectionInit == nil || ac.OnConnectionInit.Policies == nil || len(*ac.OnConnectionInit.Policies) != 1 {
		t.Errorf("expected 1 OnConnectionInit policy after update, got %v", ac.OnConnectionInit)
	}
	if (*ac.OnConnectionInit.Policies)[0].Name != "jwt-auth" {
		t.Errorf("expected updated policy name 'jwt-auth', got %q", (*ac.OnConnectionInit.Policies)[0].Name)
	}
	if ac.OnConsume != nil {
		t.Errorf("expected OnConsume to be nil after update, got %v", ac.OnConsume)
	}
}

// TestWebBrokerAPI_NilAllChannelsHandled verifies nil AllChannels is handled gracefully
func TestWebBrokerAPI_NilAllChannelsHandled(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	req.AllChannels = nil
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create with nil AllChannels failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if stored.Configuration.AllChannels != nil {
		t.Errorf("expected nil AllChannels in stored model, got %v", stored.Configuration.AllChannels)
	}
}

// TestWebBrokerAPI_ChannelPoliciesStoredAndReturned verifies channel-level policies are stored and returned correctly
func TestWebBrokerAPI_ChannelPoliciesStoredAndReturned(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if len(stored.Configuration.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(stored.Configuration.Channels))
	}
	ch, ok := stored.Configuration.Channels["stocks"]
	if !ok {
		t.Fatal("'stocks' channel not found in stored model")
	}
	if ch.OnConnectionInit == nil || len(ch.OnConnectionInit.Policies) != 0 {
		t.Errorf("expected ch.OnConnectionInit non-nil with empty policies, got %v", ch.OnConnectionInit)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if resp.Channels == nil {
		t.Fatal("response Channels should not be nil")
	}
	_, ok = resp.Channels["stocks"]
	if !ok {
		t.Fatal("'stocks' channel not found in response")
	}
}

// TestWebBrokerAPI_MultipleChannels verifies multiple channels can be configured
func TestWebBrokerAPI_MultipleChannels(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	req.Channels["crypto"] = api.WebBrokerChannel{
		ProduceTo: &struct {
			Topic *string `json:"topic,omitempty" yaml:"topic,omitempty"`
		}{
			Topic: stringPtr("crypto-updates"),
		},
		OnConnectionInit: emptyBrokerEventPolicies(),
		OnProduce:        brokerEventPolicies("validate-crypto", "v1"),
		OnConsume:        emptyBrokerEventPolicies(),
	}

	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	if len(stored.Configuration.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(stored.Configuration.Channels))
	}
	if _, ok := stored.Configuration.Channels["stocks"]; !ok {
		t.Error("'stocks' channel not found")
	}
	if _, ok := stored.Configuration.Channels["crypto"]; !ok {
		t.Error("'crypto' channel not found")
	}
}

// TestWebBrokerAPI_ChannelWithTopics verifies channel topic configuration is stored correctly
func TestWebBrokerAPI_ChannelWithTopics(t *testing.T) {
	repo := newMockWebBrokerAPIRepository()
	svc := buildWebBrokerService(repo)

	req := buildWebBrokerCreateRequest()
	_, err := svc.Create("org-uuid", "alice", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stored := repo.store["stock-trading-v1-0"]
	ch := stored.Configuration.Channels["stocks"]
	if ch.ProduceTo == nil || ch.ProduceTo.Topic != "stock-updates" {
		t.Errorf("expected ProduceTo topic 'stock-updates', got %v", ch.ProduceTo)
	}
	if ch.ConsumeFrom == nil || ch.ConsumeFrom.Topic != "stock-orders" {
		t.Errorf("expected ConsumeFrom topic 'stock-orders', got %v", ch.ConsumeFrom)
	}

	resp, err := svc.Get("org-uuid", "stock-trading-v1-0")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	respCh := resp.Channels["stocks"]
	if respCh.ProduceTo == nil || respCh.ProduceTo.Topic == nil || *respCh.ProduceTo.Topic != "stock-updates" {
		t.Errorf("expected ProduceTo topic 'stock-updates' in response, got %v", respCh.ProduceTo)
	}
	if respCh.ConsumeFrom == nil || respCh.ConsumeFrom.Topic == nil || *respCh.ConsumeFrom.Topic != "stock-orders" {
		t.Errorf("expected ConsumeFrom topic 'stock-orders' in response, got %v", respCh.ConsumeFrom)
	}
}

// TestWebBrokerAPI_MapAllChannelPoliciesAPIToModel tests the low-level mapping helper
func TestWebBrokerAPI_MapAllChannelPoliciesAPIToModel(t *testing.T) {
	in := &api.WebBrokerAllChannelPolicies{
		OnConnectionInit: brokerEventPolicies("api-key-auth", "v1"),
		OnConsume:        brokerEventPolicies("rate-limit", "v1"),
	}
	out := mapWebBrokerAllChannelPoliciesAPIToModel(in)
	if out == nil {
		t.Fatal("expected non-nil AllChannels")
	}
	if out.OnConnectionInit == nil || len(out.OnConnectionInit.Policies) != 1 {
		t.Errorf("OnConnectionInit: expected 1 policy, got %v", out.OnConnectionInit)
	}
	if out.OnConnectionInit.Policies[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", out.OnConnectionInit.Policies[0].Name)
	}
	if out.OnProduce != nil {
		t.Errorf("expected nil OnProduce, got %v", out.OnProduce)
	}
}

// TestWebBrokerAPI_MapAllChannelPoliciesModelToAPI tests the reverse mapping helper
func TestWebBrokerAPI_MapAllChannelPoliciesModelToAPI(t *testing.T) {
	in := &model.WebBrokerAllChannelPolicies{
		OnConnectionInit: &model.WebBrokerEventPolicies{
			Policies: []model.Policy{{Name: "api-key-auth", Version: "v1"}},
		},
		OnConsume: &model.WebBrokerEventPolicies{
			Policies: []model.Policy{{Name: "rate-limit", Version: "v1"}},
		},
	}
	out := mapWebBrokerAllChannelPoliciesModelToAPI(in)
	if out == nil {
		t.Fatal("expected non-nil WebBrokerAllChannelPolicies")
	}
	if out.OnConnectionInit == nil || out.OnConnectionInit.Policies == nil || len(*out.OnConnectionInit.Policies) != 1 {
		t.Errorf("expected 1 OnConnectionInit policy, got %v", out.OnConnectionInit)
	}
	if (*out.OnConnectionInit.Policies)[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", (*out.OnConnectionInit.Policies)[0].Name)
	}
	if out.OnConsume == nil || out.OnConsume.Policies == nil || len(*out.OnConsume.Policies) != 1 {
		t.Errorf("expected 1 OnConsume policy, got %v", out.OnConsume)
	}
	if out.OnProduce != nil {
		t.Errorf("expected nil OnProduce, got %v", out.OnProduce)
	}
}

// TestWebBrokerAPI_MapBrokerAPIToModel tests broker configuration mapping
func TestWebBrokerAPI_MapBrokerAPIToModel(t *testing.T) {
	brokerType := api.Kafka
	in := struct {
		Name       string                     `json:"name" yaml:"name"`
		Properties *map[string]interface{}    `json:"properties,omitempty" yaml:"properties,omitempty"`
		Type       api.WebBrokerAPIBrokerType `json:"type" yaml:"type"`
	}{
		Name: "kafka-driver",
		Type: brokerType,
		Properties: &map[string]interface{}{
			"bootstrap.servers": "kafka:9092",
		},
	}

	out := mapWebBrokerBrokerAPIToModel(in)
	if out.Name != "kafka-driver" {
		t.Errorf("expected broker name 'kafka-driver', got %q", out.Name)
	}
	if out.Type != "kafka" {
		t.Errorf("expected broker type 'kafka', got %q", out.Type)
	}
	if out.Properties == nil {
		t.Fatal("expected non-nil properties")
	}
	if bootstrap, ok := out.Properties["bootstrap.servers"]; !ok || bootstrap != "kafka:9092" {
		t.Errorf("expected bootstrap.servers 'kafka:9092', got %v", bootstrap)
	}
}

// TestWebBrokerAPI_MapReceiverAPIToModel tests receiver configuration mapping
func TestWebBrokerAPI_MapReceiverAPIToModel(t *testing.T) {
	receiverType := api.Websocket
	in := struct {
		Name string                       `json:"name" yaml:"name"`
		Type api.WebBrokerAPIReceiverType `json:"type" yaml:"type"`
	}{
		Name: "websocket-receiver",
		Type: receiverType,
	}

	out := mapWebBrokerReceiverAPIToModel(in)
	if out.Name != "websocket-receiver" {
		t.Errorf("expected receiver name 'websocket-receiver', got %q", out.Name)
	}
	if out.Type != "websocket" {
		t.Errorf("expected receiver type 'websocket', got %q", out.Type)
	}
}

// TestWebBrokerAPI_NilPoliciesHandled ensures nil policies are handled gracefully
func TestWebBrokerAPI_NilPoliciesHandled(t *testing.T) {
	if got := mapWebBrokerAllChannelPoliciesAPIToModel(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := mapWebBrokerAllChannelPoliciesModelToAPI(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := mapWebBrokerEventPoliciesAPIToModel(nil); got != nil {
		t.Errorf("expected nil for nil event policies input, got %v", got)
	}
	if got := mapWebBrokerEventPoliciesModelToAPI(nil); got != nil {
		t.Errorf("expected nil for nil event policies input, got %v", got)
	}
}
