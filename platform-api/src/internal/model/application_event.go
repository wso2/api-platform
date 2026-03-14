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

// ApplicationKeyMapping represents a single application to API key mapping.
type ApplicationKeyMapping struct {
	ApiKeyUuid string `json:"apiKeyUuid"`
}

// ApplicationUpdatedEvent represents the payload for "application.updated" events.
// This event is sent when application API key mappings are changed.
type ApplicationUpdatedEvent struct {
	ApplicationId   string                  `json:"applicationId"`
	ApplicationUuid string                  `json:"applicationUuid"`
	Mappings        []ApplicationKeyMapping `json:"mappings"`
}
