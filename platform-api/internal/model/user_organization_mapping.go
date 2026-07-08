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

package model

import "time"

// UserOrganizationMapping records that a user (identified by their internal
// UUID) has onboarded to an organization. Populate-only today: no reader
// depends on this table yet. There is no ON DELETE CASCADE on either FK by
// design — deleting the referenced user or organization must delete the
// matching rows here first, in the same transaction (see
// repository.UserOrganizationMappingRepository).
type UserOrganizationMapping struct {
	UserUUID  string    `json:"userUuid" db:"user_uuid"`
	OrgUUID   string    `json:"orgUuid" db:"org_uuid"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}
