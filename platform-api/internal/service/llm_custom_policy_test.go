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
	"errors"
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
	policies  map[string][]*model.CustomPolicy
	lookupErr error
}

func (r *llmCustomPolicyRepo) GetCustomPoliciesByName(_ string, name string) ([]*model.CustomPolicy, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	return r.policies[name], nil
}

func TestLLMProviderResolveCustomPolicyUUIDs(t *testing.T) {
	repo := &llmCustomPolicyRepo{
		policies: map[string][]*model.CustomPolicy{
			"global-custom":    {{UUID: "global-v1", Version: "v1.2.0"}},
			"operation-custom": {{UUID: "operation-v2", Version: "v2.0.1"}},
			"llm-custom":       {{UUID: "llm-v3", Version: "v3.4.5"}},
		},
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

	got, err := service.resolveCustomPolicyUUIDs("org-uuid", config)
	if err != nil {
		t.Fatalf("resolveCustomPolicyUUIDs() error = %v", err)
	}

	sort.Strings(got)
	want := []string{"global-v1", "llm-v3", "operation-v2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved policy UUIDs = %v, want %v", got, want)
	}
}

func TestLLMProviderResolveCustomPolicyUUIDsLookupFailure(t *testing.T) {
	repo := &llmCustomPolicyRepo{lookupErr: errors.New("database unavailable")}
	service := &LLMProviderService{
		customPolicyRepo: repo,
		slogger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	config := &model.LLMProviderConfig{
		GlobalPolicies: []model.GlobalPolicy{{Name: "Global-Custom", Version: "v1"}},
	}

	got, err := service.resolveCustomPolicyUUIDs("org-uuid", config)
	if err == nil {
		t.Fatal("resolveCustomPolicyUUIDs() expected lookup error")
	}
	if got != nil {
		t.Fatalf("resolved policy UUIDs = %v, want nil on lookup failure", got)
	}
}
