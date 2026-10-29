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

package storage

import "time"

// AuditLogFilter holds the optional predicates for audit-log queries.
// GatewayID is always set by the storage layer from the store's own gatewayId —
// callers must not supply it.
type AuditLogFilter struct {
	StartTime    *time.Time
	EndTime      *time.Time
	Outcome      string
	PerformedBy  string
	Action       string
	ResourceType string
	ResourceUUID string
}
