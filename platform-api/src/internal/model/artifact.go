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

type Artifact struct {
	UUID             string `db:"uuid"`
	Type             string `db:"type"`
	OrganizationUUID string `db:"organization_uuid"`
	// Supplemental fields: populated by UNION queries across kind-specific tables, not stored in artifacts table.
	Handle    string    `db:"handle"`
	Name      string    `db:"display_name"`
	Version   string    `db:"version"`
	Kind      string    `db:"kind"`
	Origin    string    `db:"origin"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
