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

package dto

// ViewRequest represents create/update of a view
type ViewRequest struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Labels      []string `json:"labels,omitempty"`
}

// ViewResponse represents a view returned by the DevPortal
type ViewResponse struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Labels      []string `json:"labels"`
}

type ViewUpdateRequest struct {
	DisplayName string   `json:"displayName"`
	Labels      []string `json:"labels,omitempty"`
}
