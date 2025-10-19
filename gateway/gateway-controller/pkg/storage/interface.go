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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// Storage is the interface for persisting API configurations
type Storage interface {
	// SaveConfig persists a new API configuration
	SaveConfig(cfg *models.StoredAPIConfig) error

	// UpdateConfig updates an existing API configuration
	UpdateConfig(cfg *models.StoredAPIConfig) error

	// DeleteConfig removes an API configuration by ID
	DeleteConfig(id string) error

	// GetConfig retrieves an API configuration by ID
	GetConfig(id string) (*models.StoredAPIConfig, error)

	// GetConfigByNameVersion retrieves an API configuration by name and version
	GetConfigByNameVersion(name, version string) (*models.StoredAPIConfig, error)

	// GetAllConfigs retrieves all API configurations
	GetAllConfigs() ([]*models.StoredAPIConfig, error)

	// Close closes the storage connection
	Close() error
}
