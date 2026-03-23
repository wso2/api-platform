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

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDesiredState_Constants(t *testing.T) {
	assert.Equal(t, DesiredState("deployed"), StateDeployed)
	assert.Equal(t, DesiredState("undeployed"), StateUndeployed)
}

func TestStoredConfig_Handle(t *testing.T) {
	config := &StoredConfig{
		Handle: "0000-test-handle-0000-000000000000",
	}

	assert.Equal(t, "0000-test-handle-0000-000000000000", config.Handle)
}

func TestStoredConfig_Handle_Empty(t *testing.T) {
	config := &StoredConfig{
		Handle: "",
	}

	assert.Equal(t, "", config.Handle)
}

func TestStoredConfig_Fields(t *testing.T) {
	now := time.Now()
	deployedAt := now.Add(time.Hour)

	config := &StoredConfig{
		UUID:       "0000-test-id-123-0000-000000000000",
		Kind:       "API",
		DesiredState: StateDeployed,
		CreatedAt:  now,
		UpdatedAt:  now,
		DeployedAt: &deployedAt,
	}

	assert.Equal(t, "0000-test-id-123-0000-000000000000", config.UUID)
	assert.Equal(t, "API", config.Kind)
	assert.Equal(t, StateDeployed, config.DesiredState)
	assert.Equal(t, now, config.CreatedAt)
	assert.Equal(t, now, config.UpdatedAt)
	assert.NotNil(t, config.DeployedAt)
	assert.Equal(t, deployedAt, *config.DeployedAt)
}

func TestStoredConfig_NilDeployedAt(t *testing.T) {
	config := &StoredConfig{
		UUID:       "0000-test-id-0000-000000000000",
		DesiredState: StateDeployed,
		DeployedAt: nil,
	}

	assert.Nil(t, config.DeployedAt)
}

func TestStoredConfig_SourceConfiguration(t *testing.T) {
	sourceConfig := map[string]interface{}{
		"kind": "API",
		"spec": map[string]interface{}{
			"displayName": "Test API",
		},
	}

	config := &StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		SourceConfiguration: sourceConfig,
	}

	assert.NotNil(t, config.SourceConfiguration)

	// Type assert and verify
	sc, ok := config.SourceConfiguration.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "API", sc["kind"])
}
