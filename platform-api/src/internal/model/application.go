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

// Application represents an application entity.
type Application struct {
	UUID             string    `json:"uuid" db:"uuid"`
	Handle           string    `json:"id" db:"handle"`
	ProjectUUID      string    `json:"projectId" db:"project_uuid"`
	OrganizationUUID string    `json:"organizationId" db:"organization_uuid"`
	CreatedBy        string    `json:"createdBy,omitempty" db:"created_by"`
	Name             string    `json:"name" db:"name"`
	Description      string    `json:"description,omitempty" db:"description"`
	Type             string    `json:"type" db:"type"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}

func (Application) TableName() string {
	return "applications"
}

// ApplicationAPIKey represents an API key mapped to an application.
type ApplicationAPIKey struct {
	ID         string     `json:"id" db:"id"`
	APIKeyUUID string     `json:"-" db:"uuid"`
	Name       string     `json:"name" db:"name"`
	ArtifactID string     `json:"artifactId" db:"artifact_uuid"`
	ArtifactHandle string   `json:"-" db:"handle"`
	ArtifactKind   string   `json:"-" db:"kind"`
	Status     string     `json:"status,omitempty" db:"status"`
	CreatedBy  string     `json:"createdBy,omitempty" db:"created_by"`
	CreatedAt  time.Time  `json:"createdAt,omitempty" db:"created_at"`
	UpdatedAt  time.Time  `json:"updatedAt,omitempty" db:"updated_at"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty" db:"expires_at"`
}
