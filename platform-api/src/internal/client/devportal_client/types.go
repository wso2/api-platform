/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package devportal_client

import dto "platform-api/src/internal/client/devportal_client/dto"

// Re-export commonly used DTO types so callers can import only this package.
// These are type aliases and incur no runtime cost.
type (
	// Organizations
	OrganizationCreateRequest = dto.OrganizationCreateRequest
	OrganizationResponse      = dto.OrganizationResponse
	OrganizationUpdateRequest = dto.OrganizationUpdateRequest
	OrganizationListResponse  = dto.OrganizationListResponse

	// Views
	ViewRequest  = dto.ViewRequest
	ViewResponse = dto.ViewResponse

	// Labels
	Label     = dto.Label
	LabelList = dto.LabelList

	// Subscription policies
	SubscriptionPolicy = dto.SubscriptionPolicy

	// APIs
	Owners             = dto.Owners
	APIInfo            = dto.APIInfo
	EndPoints          = dto.EndPoints
	APIMetadataRequest = dto.APIMetadataRequest
	APIResponse        = dto.APIResponse

	// Re-export enum types from dto so callers only import devportal_client.
	SubscriptionType = dto.SubscriptionType
	APIVisibility       = dto.APIVisibility
	APIStatus        = dto.APIStatus
	APIType          = dto.APIType
)
