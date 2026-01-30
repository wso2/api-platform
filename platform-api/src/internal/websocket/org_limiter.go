/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package websocket

import (
	"sync"
)

// OrgConnectionStats holds connection statistics for an organization
type OrgConnectionStats struct {
	OrganizationID string `json:"organizationId"`
	CurrentCount   int    `json:"currentCount"`
	MaxAllowed     int    `json:"maxAllowed"`
}

// OrgConnectionLimiter manages per-organization connection limits
type OrgConnectionLimiter struct {
	mu sync.RWMutex

	// connections maps orgID -> map[connectionID]bool
	connections map[string]map[string]bool

	// maxConnectionsPerOrg is the maximum connections allowed per organization
	maxConnectionsPerOrg int
}

// NewOrgConnectionLimiter creates a new OrgConnectionLimiter with the specified limit
func NewOrgConnectionLimiter(maxConnectionsPerOrg int) *OrgConnectionLimiter {
	return &OrgConnectionLimiter{
		connections:          make(map[string]map[string]bool),
		maxConnectionsPerOrg: maxConnectionsPerOrg,
	}
}

// CanAcceptConnection checks if the organization can accept a new connection
func (l *OrgConnectionLimiter) CanAcceptConnection(orgID string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	currentCount := len(l.connections[orgID])
	return currentCount < l.maxConnectionsPerOrg
}

// AddConnection adds a new connection for the organization
// Returns an error if the organization has reached its connection limit
func (l *OrgConnectionLimiter) AddConnection(orgID, connectionID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentCount := len(l.connections[orgID])

	if currentCount >= l.maxConnectionsPerOrg {
		return &OrgConnectionLimitError{
			OrganizationID: orgID,
			CurrentCount:   currentCount,
			MaxAllowed:     l.maxConnectionsPerOrg,
		}
	}

	// Initialize map for this org if needed
	if l.connections[orgID] == nil {
		l.connections[orgID] = make(map[string]bool)
	}

	l.connections[orgID][connectionID] = true
	return nil
}

// RemoveConnection removes a connection from the organization
func (l *OrgConnectionLimiter) RemoveConnection(orgID, connectionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.connections[orgID] != nil {
		delete(l.connections[orgID], connectionID)

		// Clean up empty org maps
		if len(l.connections[orgID]) == 0 {
			delete(l.connections, orgID)
		}
	}
}

// GetOrgConnectionCount returns the current connection count for an organization
func (l *OrgConnectionLimiter) GetOrgConnectionCount(orgID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return len(l.connections[orgID])
}

// GetOrgStats returns connection statistics for an organization
func (l *OrgConnectionLimiter) GetOrgStats(orgID string) OrgConnectionStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return OrgConnectionStats{
		OrganizationID: orgID,
		CurrentCount:   len(l.connections[orgID]),
		MaxAllowed:     l.maxConnectionsPerOrg,
	}
}

// GetAllOrgConnectionCounts returns connection counts for all organizations
func (l *OrgConnectionLimiter) GetAllOrgConnectionCounts() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]int)
	for orgID, conns := range l.connections {
		result[orgID] = len(conns)
	}
	return result
}
