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
	"testing"

	"platform-api/src/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// gatewaySupportsSplitPolicies
// ---------------------------------------------------------------------------

func TestMigrateLegacyProviderPoliciesInPlace_NoLegacy(t *testing.T) {
	cfg := &model.LLMProviderConfig{
		GlobalPolicies:    []model.GlobalPolicy{{Name: "existing-global", Version: "v1"}},
		OperationPolicies: []model.OperationPolicy{},
		Policies:          nil,
	}
	migrateLegacyProviderPoliciesInPlace(cfg)
	assert.Len(t, cfg.GlobalPolicies, 1)
	assert.Empty(t, cfg.OperationPolicies)
	assert.Nil(t, cfg.Policies)
}

func TestMigrateLegacyProviderPoliciesInPlace_WildcardGlobal(t *testing.T) {
	cfg := &model.LLMProviderConfig{
		Policies: []model.LLMPolicy{
			{
				Name:    "basic-ratelimit",
				Version: "v1",
				Paths: []model.LLMPolicyPath{
					{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{"requests": 10}},
				},
			},
		},
	}
	migrateLegacyProviderPoliciesInPlace(cfg)
	require.Len(t, cfg.GlobalPolicies, 1)
	assert.Equal(t, "basic-ratelimit", cfg.GlobalPolicies[0].Name)
	assert.Equal(t, map[string]interface{}{"requests": 10}, cfg.GlobalPolicies[0].Params)
	assert.Empty(t, cfg.OperationPolicies)
	assert.Nil(t, cfg.Policies)
}

func TestMigrateLegacyProviderPoliciesInPlace_SpecificPathBecomesOperation(t *testing.T) {
	cfg := &model.LLMProviderConfig{
		Policies: []model.LLMPolicy{
			{
				Name:    "basic-ratelimit",
				Version: "v1",
				Paths: []model.LLMPolicyPath{
					{Path: "/chat/completions", Methods: []string{"GET"}, Params: map[string]interface{}{"requests": 5}},
				},
			},
		},
	}
	migrateLegacyProviderPoliciesInPlace(cfg)
	assert.Empty(t, cfg.GlobalPolicies)
	require.Len(t, cfg.OperationPolicies, 1)
	assert.Equal(t, "basic-ratelimit", cfg.OperationPolicies[0].Name)
	require.Len(t, cfg.OperationPolicies[0].Paths, 1)
	assert.Equal(t, "/chat/completions", cfg.OperationPolicies[0].Paths[0].Path)
	assert.Nil(t, cfg.Policies)
}

func TestMigrateLegacyProviderPoliciesInPlace_MixedPaths(t *testing.T) {
	// One policy with /*+["*"] path AND a specific path: the /* → global, the other → operation.
	cfg := &model.LLMProviderConfig{
		Policies: []model.LLMPolicy{
			{
				Name:    "basic-ratelimit",
				Version: "v1",
				Paths: []model.LLMPolicyPath{
					{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{"requests": 20}},
					{Path: "/chat/completions", Methods: []string{"POST"}, Params: map[string]interface{}{"requests": 5}},
				},
			},
		},
	}
	migrateLegacyProviderPoliciesInPlace(cfg)
	require.Len(t, cfg.GlobalPolicies, 1)
	assert.Equal(t, "basic-ratelimit", cfg.GlobalPolicies[0].Name)
	require.Len(t, cfg.OperationPolicies, 1)
	assert.Equal(t, "basic-ratelimit", cfg.OperationPolicies[0].Name)
	require.Len(t, cfg.OperationPolicies[0].Paths, 1)
	assert.Equal(t, "/chat/completions", cfg.OperationPolicies[0].Paths[0].Path)
}

func TestMigrateLegacyProviderPoliciesInPlace_WildcardMethodSpecificPath_StaysOperation(t *testing.T) {
	// /* with specific methods (not ["*"]) must NOT become global.
	cfg := &model.LLMProviderConfig{
		Policies: []model.LLMPolicy{
			{Name: "basic-ratelimit", Version: "v1", Paths: []model.LLMPolicyPath{
				{Path: "/*", Methods: []string{"POST"}, Params: nil},
			}},
		},
	}
	migrateLegacyProviderPoliciesInPlace(cfg)
	assert.Empty(t, cfg.GlobalPolicies)
	require.Len(t, cfg.OperationPolicies, 1)
}

func TestMigrateLegacyProviderPoliciesInPlace_DedupsGlobalByName(t *testing.T) {
	// If globalPolicies already has a policy of the same name, don't add a duplicate.
	cfg := &model.LLMProviderConfig{
		GlobalPolicies: []model.GlobalPolicy{{Name: "basic-ratelimit", Version: "v1"}},
		Policies: []model.LLMPolicy{
			{Name: "basic-ratelimit", Version: "v1", Paths: []model.LLMPolicyPath{
				{Path: "/*", Methods: []string{"*"}, Params: nil},
			}},
		},
	}
	migrateLegacyProviderPoliciesInPlace(cfg)
	assert.Len(t, cfg.GlobalPolicies, 1) // still just one
}

// ---------------------------------------------------------------------------
// applyVersionAwareProviderPolicyTransform
// ---------------------------------------------------------------------------
