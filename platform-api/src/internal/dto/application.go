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

package dto

import "time"

type CreateApplicationRequest struct {
	Id          string  `json:"id,omitempty"`
	Name        string  `json:"name"`
	ProjectId   string  `json:"projectId,omitempty"`
	Type        string  `json:"type"`
	Description *string `json:"description,omitempty"`
	CreatedBy   *string `json:"createdBy,omitempty"`
}

type UpdateApplicationRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
}

type ApplicationResponse struct {
	Id          string     `json:"id"`
	Name        string     `json:"name"`
	ProjectId   string     `json:"projectId,omitempty"`
	Type        string     `json:"type"`
	Description *string    `json:"description,omitempty"`
	CreatedBy   *string    `json:"createdBy,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

type ApplicationListResponse struct {
	Count      int                    `json:"count"`
	List       []*ApplicationResponse `json:"list"`
	Pagination Pagination             `json:"pagination"`
}

type ReplaceApplicationAPIKeysRequest struct {
	ApiKeyIds []string `json:"apiKeyIds"`
}

type AddApplicationAPIKeysRequest struct {
	ApiKeyIds []string `json:"apiKeyIds"`
}

type AssociatedEntityResponse struct {
	Handle string `json:"handle"`
	Kind   string `json:"kind"`
}

type MappedAPIKeyResponse struct {
	KeyId            string                   `json:"keyId"`
	AssociatedEntity AssociatedEntityResponse `json:"associated_entity"`
	ApiKeyUuid       string                   `json:"-"`
	ArtifactId       string                   `json:"-"`
	Status           *string                  `json:"status,omitempty"`
	UserId           *string                  `json:"userId,omitempty"`
	CreatedAt        *time.Time               `json:"createdAt,omitempty"`
	UpdatedAt        *time.Time               `json:"updatedAt,omitempty"`
	ExpiresAt        *time.Time               `json:"expiresAt,omitempty"`
}

type MappedAPIKeyListResponse struct {
	Count      int                     `json:"count"`
	List       []*MappedAPIKeyResponse `json:"list"`
	Pagination Pagination              `json:"pagination"`
}
