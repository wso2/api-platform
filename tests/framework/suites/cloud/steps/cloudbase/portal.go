/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"time"
)

const (
	// ManagedPortalsCollection is the collection managed API Portals are created in, listed in and
	// deleted from.
	ManagedPortalsCollection = "/managed-api-portals"
	PortalStatusPending      = "pending"
	PortalStatusActive       = "active"
	PortalStatusFailed       = "failed"
	// PortalBuildTimeout bounds the wait for a portal to become active. The platform builds a
	// portal in the background, which takes minutes.
	PortalBuildTimeout = 15 * time.Minute
)

// PortalPollInterval is how often a portal that is being built or torn down is read, and how often
// a refused cleanup delete is repeated.
var PortalPollInterval = 15 * time.Second
