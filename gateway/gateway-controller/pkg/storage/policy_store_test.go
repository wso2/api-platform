/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func createTestPolicy(id, apiName, version, context string) *models.StoredPolicyConfig {
	return &models.StoredPolicyConfig{
		ID: id,
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: apiName,
				Version: version,
				Context: context,
			},
		},
		Version: 1,
	}
}

func TestNewPolicyStore(t *testing.T) {
	store := NewPolicyStore()
	assert.NotNil(t, store)
	assert.Equal(t, 0, store.Count())
	assert.Equal(t, int64(0), store.GetResourceVersion())
}

func TestPolicyStore_Set(t *testing.T) {
	store := NewPolicyStore()
	policy := createTestPolicy("policy-1", "test-api", "v1", "/test")

	err := store.Set(policy)
	assert.NoError(t, err)
	assert.Equal(t, 1, store.Count())
}

func TestPolicyStore_Set_Update(t *testing.T) {
	store := NewPolicyStore()
	policy := createTestPolicy("policy-1", "test-api", "v1", "/test")

	err := store.Set(policy)
	assert.NoError(t, err)

	// Update the same policy with new metadata
	policy.Configuration.Metadata.APIName = "updated-api"
	err = store.Set(policy)
	assert.NoError(t, err)
	assert.Equal(t, 1, store.Count())

	retrieved, exists := store.Get("policy-1")
	assert.True(t, exists)
	assert.Equal(t, "updated-api", retrieved.APIName())
}

func TestPolicyStore_Set_DuplicateCompositeKey(t *testing.T) {
	store := NewPolicyStore()

	policy1 := createTestPolicy("policy-1", "test-api", "v1", "/test")
	err := store.Set(policy1)
	assert.NoError(t, err)

	// Try to add different policy with same composite key
	policy2 := createTestPolicy("policy-2", "test-api", "v1", "/test")
	err = store.Set(policy2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestPolicyStore_Get(t *testing.T) {
	store := NewPolicyStore()
	policy := createTestPolicy("policy-1", "test-api", "v1", "/test")
	store.Set(policy)

	retrieved, exists := store.Get("policy-1")
	assert.True(t, exists)
	assert.Equal(t, policy.ID, retrieved.ID)
	assert.Equal(t, policy.APIName(), retrieved.APIName())
}

func TestPolicyStore_Get_NotFound(t *testing.T) {
	store := NewPolicyStore()

	retrieved, exists := store.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestPolicyStore_GetByCompositeKey(t *testing.T) {
	store := NewPolicyStore()
	policy := createTestPolicy("policy-1", "test-api", "v1", "/test")
	store.Set(policy)

	retrieved, exists := store.GetByCompositeKey("test-api", "v1", "/test")
	assert.True(t, exists)
	assert.Equal(t, policy.ID, retrieved.ID)
}

func TestPolicyStore_GetByCompositeKey_NotFound(t *testing.T) {
	store := NewPolicyStore()

	retrieved, exists := store.GetByCompositeKey("nonexistent", "v1", "/path")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestPolicyStore_GetAll(t *testing.T) {
	store := NewPolicyStore()
	store.Set(createTestPolicy("policy-1", "api-1", "v1", "/path1"))
	store.Set(createTestPolicy("policy-2", "api-2", "v1", "/path2"))
	store.Set(createTestPolicy("policy-3", "api-3", "v1", "/path3"))

	all := store.GetAll()
	assert.Len(t, all, 3)
}

func TestPolicyStore_GetAll_Empty(t *testing.T) {
	store := NewPolicyStore()

	all := store.GetAll()
	assert.NotNil(t, all)
	assert.Len(t, all, 0)
}

func TestPolicyStore_Delete(t *testing.T) {
	store := NewPolicyStore()
	policy := createTestPolicy("policy-1", "test-api", "v1", "/test")
	store.Set(policy)

	err := store.Delete("policy-1")
	assert.NoError(t, err)
	assert.Equal(t, 0, store.Count())

	_, exists := store.Get("policy-1")
	assert.False(t, exists)
}

func TestPolicyStore_Delete_NotFound(t *testing.T) {
	store := NewPolicyStore()

	err := store.Delete("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPolicyStore_Count(t *testing.T) {
	store := NewPolicyStore()
	assert.Equal(t, 0, store.Count())

	store.Set(createTestPolicy("policy-1", "api-1", "v1", "/path1"))
	assert.Equal(t, 1, store.Count())

	store.Set(createTestPolicy("policy-2", "api-2", "v1", "/path2"))
	assert.Equal(t, 2, store.Count())

	store.Delete("policy-1")
	assert.Equal(t, 1, store.Count())
}

func TestPolicyStore_Clear(t *testing.T) {
	store := NewPolicyStore()
	store.Set(createTestPolicy("policy-1", "api-1", "v1", "/path1"))
	store.Set(createTestPolicy("policy-2", "api-2", "v1", "/path2"))

	store.Clear()

	assert.Equal(t, 0, store.Count())
	all := store.GetAll()
	assert.Len(t, all, 0)
}

func TestPolicyStore_IncrementResourceVersion(t *testing.T) {
	store := NewPolicyStore()
	assert.Equal(t, int64(0), store.GetResourceVersion())

	v1 := store.IncrementResourceVersion()
	assert.Equal(t, int64(1), v1)
	assert.Equal(t, int64(1), store.GetResourceVersion())

	v2 := store.IncrementResourceVersion()
	assert.Equal(t, int64(2), v2)
	assert.Equal(t, int64(2), store.GetResourceVersion())
}

func TestPolicyStore_GetResourceVersion(t *testing.T) {
	store := NewPolicyStore()
	assert.Equal(t, int64(0), store.GetResourceVersion())

	store.IncrementResourceVersion()
	store.IncrementResourceVersion()
	store.IncrementResourceVersion()

	assert.Equal(t, int64(3), store.GetResourceVersion())
}

func TestPolicyStore_UpdateCompositeKey(t *testing.T) {
	store := NewPolicyStore()

	// Add policy
	policy := createTestPolicy("policy-1", "api-1", "v1", "/path1")
	store.Set(policy)

	// Create updated version with new composite key (same ID)
	updatedPolicy := &models.StoredPolicyConfig{
		ID: "policy-1",
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: "api-updated",
				Version: "v1",
				Context: "/path-updated",
			},
		},
		Version: 1,
	}
	store.Set(updatedPolicy)

	// Old key should not exist
	_, exists := store.GetByCompositeKey("api-1", "v1", "/path1")
	assert.False(t, exists)

	// New key should exist
	retrieved, exists := store.GetByCompositeKey("api-updated", "v1", "/path-updated")
	assert.True(t, exists)
	assert.Equal(t, "policy-1", retrieved.ID)
}
