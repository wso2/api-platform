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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

func TestConfigStatus_Constants(t *testing.T) {
	assert.Equal(t, ConfigStatus("pending"), StatusPending)
	assert.Equal(t, ConfigStatus("deployed"), StatusDeployed)
	assert.Equal(t, ConfigStatus("failed"), StatusFailed)
}

func TestStoredConfig_GetHandle(t *testing.T) {
	config := &StoredConfig{
		Configuration: api.APIConfiguration{
			Metadata: api.Metadata{
				Name: "test-handle",
			},
		},
	}

	assert.Equal(t, "test-handle", config.GetHandle())
}

func TestStoredConfig_GetHandle_Empty(t *testing.T) {
	config := &StoredConfig{
		Configuration: api.APIConfiguration{
			Metadata: api.Metadata{
				Name: "",
			},
		},
	}

	assert.Equal(t, "", config.GetHandle())
}

func TestStoredConfig_Fields(t *testing.T) {
	now := time.Now()
	deployedAt := now.Add(time.Hour)

	config := &StoredConfig{
		ID:              "test-id-123",
		Kind:            "API",
		Status:          StatusDeployed,
		CreatedAt:       now,
		UpdatedAt:       now,
		DeployedAt:      &deployedAt,
		DeployedVersion: 5,
	}

	assert.Equal(t, "test-id-123", config.ID)
	assert.Equal(t, "API", config.Kind)
	assert.Equal(t, StatusDeployed, config.Status)
	assert.Equal(t, now, config.CreatedAt)
	assert.Equal(t, now, config.UpdatedAt)
	assert.NotNil(t, config.DeployedAt)
	assert.Equal(t, deployedAt, *config.DeployedAt)
	assert.Equal(t, int64(5), config.DeployedVersion)
}

func TestStoredConfig_NilDeployedAt(t *testing.T) {
	config := &StoredConfig{
		ID:         "test-id",
		Status:     StatusPending,
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
		ID:                  "test-id",
		SourceConfiguration: sourceConfig,
	}

	assert.NotNil(t, config.SourceConfiguration)

	// Type assert and verify
	sc, ok := config.SourceConfiguration.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "API", sc["kind"])
}
