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
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.etcd.io/bbolt"
)

var (
	// Bucket names
	bucketAPIs     = []byte("apis")
	bucketAudit    = []byte("audit")
	bucketMetadata = []byte("metadata")
)

// BBoltStorage implements the Storage interface using bbolt
type BBoltStorage struct {
	db *bbolt.DB
}

// NewBBoltStorage creates a new bbolt storage instance
func NewBBoltStorage(dbPath string) (*BBoltStorage, error) {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets if they don't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketAPIs); err != nil {
			return fmt.Errorf("failed to create apis bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(bucketAudit); err != nil {
			return fmt.Errorf("failed to create audit bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(bucketMetadata); err != nil {
			return fmt.Errorf("failed to create metadata bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BBoltStorage{db: db}, nil
}

// SaveConfig persists a new API configuration
func (s *BBoltStorage) SaveConfig(cfg *models.StoredAPIConfig) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			return fmt.Errorf("apis bucket not found")
		}

		data, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		return bucket.Put([]byte(cfg.ID), data)
	})
}

// UpdateConfig updates an existing API configuration
func (s *BBoltStorage) UpdateConfig(cfg *models.StoredAPIConfig) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			return fmt.Errorf("apis bucket not found")
		}

		// Check if config exists
		existing := bucket.Get([]byte(cfg.ID))
		if existing == nil {
			return fmt.Errorf("configuration with ID '%s' not found", cfg.ID)
		}

		data, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		return bucket.Put([]byte(cfg.ID), data)
	})
}

// DeleteConfig removes an API configuration by ID
func (s *BBoltStorage) DeleteConfig(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			return fmt.Errorf("apis bucket not found")
		}

		return bucket.Delete([]byte(id))
	})
}

// GetConfig retrieves an API configuration by ID
func (s *BBoltStorage) GetConfig(id string) (*models.StoredAPIConfig, error) {
	var cfg *models.StoredAPIConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			return fmt.Errorf("apis bucket not found")
		}

		data := bucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("configuration with ID '%s' not found", id)
		}

		cfg = &models.StoredAPIConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("failed to unmarshal config: %w", err)
		}

		return nil
	})

	return cfg, err
}

// GetConfigByNameVersion retrieves an API configuration by name and version
func (s *BBoltStorage) GetConfigByNameVersion(name, version string) (*models.StoredAPIConfig, error) {
	var cfg *models.StoredAPIConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			return fmt.Errorf("apis bucket not found")
		}

		// Iterate through all configs to find the one matching name and version
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var tempCfg models.StoredAPIConfig
			if err := json.Unmarshal(v, &tempCfg); err != nil {
				continue // Skip malformed entries
			}

			if tempCfg.Configuration.Data.Name == name && tempCfg.Configuration.Data.Version == version {
				cfg = &tempCfg
				return nil
			}
		}

		return fmt.Errorf("configuration with name '%s' and version '%s' not found", name, version)
	})

	return cfg, err
}

// GetAllConfigs retrieves all API configurations
func (s *BBoltStorage) GetAllConfigs() ([]*models.StoredAPIConfig, error) {
	var configs []*models.StoredAPIConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			return fmt.Errorf("apis bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var cfg models.StoredAPIConfig
			if err := json.Unmarshal(v, &cfg); err != nil {
				return fmt.Errorf("failed to unmarshal config: %w", err)
			}
			configs = append(configs, &cfg)
			return nil
		})
	})

	return configs, err
}

// Close closes the database connection
func (s *BBoltStorage) Close() error {
	return s.db.Close()
}

// GetDB returns the underlying bbolt database (for loading data)
func (s *BBoltStorage) GetDB() *bbolt.DB {
	return s.db
}

// LogEvent logs an audit event
func (s *BBoltStorage) LogEvent(event *AuditEvent) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAudit)
		if bucket == nil {
			return fmt.Errorf("audit bucket not found")
		}

		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal audit event: %w", err)
		}

		// Use timestamp + UUID as key for ordering
		key := fmt.Sprintf("%s_%s", event.Timestamp, event.ID)
		return bucket.Put([]byte(key), data)
	})
}

// GetEvents retrieves audit events (limited to last N entries)
func (s *BBoltStorage) GetEvents(limit int) ([]*AuditEvent, error) {
	var events []*AuditEvent

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAudit)
		if bucket == nil {
			return fmt.Errorf("audit bucket not found")
		}

		cursor := bucket.Cursor()
		count := 0

		// Iterate in reverse order to get latest events
		for k, v := cursor.Last(); k != nil && count < limit; k, v = cursor.Prev() {
			var event AuditEvent
			if err := json.Unmarshal(v, &event); err != nil {
				return fmt.Errorf("failed to unmarshal audit event: %w", err)
			}
			events = append(events, &event)
			count++
		}

		return nil
	})

	return events, err
}

// LoadFromDatabase loads all configurations from bbolt into the in-memory store
func LoadFromDatabase(db *bbolt.DB, store *ConfigStore) error {
	return db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPIs)
		if bucket == nil {
			// No configurations stored yet
			return nil
		}

		var maxVersion int64 = 0

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var cfg models.StoredAPIConfig
			if err := json.Unmarshal(v, &cfg); err != nil {
				return fmt.Errorf("failed to unmarshal config %s: %w", k, err)
			}

			// Reset status to pending on startup to ensure re-deployment to clean Router
			// This fixes the cold-start issue where previously deployed configs would be
			// skipped by the translator even though Envoy has no configuration yet
			cfg.Status = models.StatusPending

			// Add to in-memory store (bypassing locking since we're in startup)
			if err := store.Add(&cfg); err != nil {
				return fmt.Errorf("failed to add config to memory store: %w", err)
			}

			// Track highest deployed version for snapshot versioning
			if cfg.DeployedVersion > maxVersion {
				maxVersion = cfg.DeployedVersion
			}
		}

		// Set the snapshot version to the highest deployed version
		store.SetSnapshotVersion(maxVersion)

		return nil
	})
}

// CreateAuditEvent creates a new audit event with generated ID and timestamp
func CreateAuditEvent(operation AuditOperation, configID, configName, configVersion, status, errorMsg string) *AuditEvent {
	return &AuditEvent{
		ID:            uuid.New().String(),
		Timestamp:     time.Now().Format(time.RFC3339),
		Operation:     operation,
		ConfigID:      configID,
		ConfigName:    configName,
		ConfigVersion: configVersion,
		Status:        status,
		ErrorMessage:  errorMsg,
		Details:       make(map[string]interface{}),
	}
}
