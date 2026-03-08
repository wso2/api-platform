/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package eventhub

import "time"

const defaultOrganizationStatePageSize = 200

// EventhubImpl defines the backend interface for pluggable event hub implementations
type EventhubImpl interface {
	// Initialize sets up the backend
	Initialize() error
	// RegisterOrganization registers a new organization for event tracking
	RegisterOrganization(orgID string) error
	// Publish publishes an event for an organization
	Publish(orgID string, event Event) error
	// Subscribe subscribes to events for an organization, returning a channel
	Subscribe(orgID string) (<-chan Event, error)
	// Unsubscribe removes a subscription for an organization
	Unsubscribe(orgID string) error
	// Cleanup removes events older than the retention period
	Cleanup(retentionPeriod time.Duration) error
	// CleanupRange removes events in a time range for an organization
	CleanupRange(orgID string, before time.Time) error
	// Close gracefully shuts down the backend
	Close() error
}

// SQLBackendConfig holds configuration for the SQL backend
type SQLBackendConfig struct {
	PollInterval              time.Duration
	CleanupInterval           time.Duration
	RetentionPeriod           time.Duration
	OrganizationStatePageSize int
}

// DefaultSQLBackendConfig returns a SQLBackendConfig with sensible defaults
func DefaultSQLBackendConfig() SQLBackendConfig {
	return SQLBackendConfig{
		PollInterval:              2 * time.Second,
		CleanupInterval:           5 * time.Minute,
		RetentionPeriod:           1 * time.Hour,
		OrganizationStatePageSize: defaultOrganizationStatePageSize,
	}
}
