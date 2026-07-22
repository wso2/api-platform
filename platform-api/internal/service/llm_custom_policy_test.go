/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package service

import (
	"io"
	"log/slog"
	"reflect"
	"sort"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type llmCustomPolicyRepo struct {
	repository.CustomPolicyRepository
	policies map[string][]*model.CustomPolicy
	current  []string
	inserted []string
	deleted  []string
}

func (r *llmCustomPolicyRepo) GetCustomPoliciesByName(_ string, name string) ([]*model.CustomPolicy, error) {
	return r.policies[name], nil
}

func (r *llmCustomPolicyRepo) GetCustomPolicyUsagesByAPIUUID(_ string) ([]string, error) {
	return r.current, nil
}

func (r *llmCustomPolicyRepo) InsertCustomPolicyUsage(policyUUID, _ string) error {
	r.inserted = append(r.inserted, policyUUID)
	return nil
}

func (r *llmCustomPolicyRepo) DeleteCustomPolicyUsage(policyUUID, _ string) error {
	r.deleted = append(r.deleted, policyUUID)
	return nil
}

func TestLLMProviderRefreshCustomPolicyUsages(t *testing.T) {
	repo := &llmCustomPolicyRepo{
		policies: map[string][]*model.CustomPolicy{
			"global-custom":    {{UUID: "global-v1", Version: "v1.2.0"}},
			"operation-custom": {{UUID: "operation-v2", Version: "v2.0.1"}},
			"llm-custom":       {{UUID: "llm-v3", Version: "v3.4.5"}},
		},
		current: []string{"global-v1", "removed-policy"},
	}
	service := &LLMProviderService{
		customPolicyRepo: repo,
		slogger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	config := &model.LLMProviderConfig{
		GlobalPolicies:    []model.GlobalPolicy{{Name: "Global-Custom", Version: "v1"}},
		OperationPolicies: []model.OperationPolicy{{Name: "Operation-Custom", Version: "v2.8.0"}},
		Policies:          []model.LLMPolicy{{Name: "LLM-Custom", Version: "v3"}},
	}

	service.refreshCustomPolicyUsages("provider-uuid", "org-uuid", config)

	sort.Strings(repo.inserted)
	wantInserted := []string{"llm-v3", "operation-v2"}
	if !reflect.DeepEqual(repo.inserted, wantInserted) {
		t.Fatalf("inserted usages = %v, want %v", repo.inserted, wantInserted)
	}
	if !reflect.DeepEqual(repo.deleted, []string{"removed-policy"}) {
		t.Fatalf("deleted usages = %v, want [removed-policy]", repo.deleted)
	}
}
