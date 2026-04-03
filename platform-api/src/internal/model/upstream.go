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

// UpstreamConfig represents the upstream configuration with main and sandbox endpoints
type UpstreamConfig struct {
	Main    *UpstreamEndpoint `json:"main,omitempty" db:"-"`
	Sandbox *UpstreamEndpoint `json:"sandbox,omitempty" db:"-"`
}

// UpstreamEndpoint represents an upstream endpoint configuration
type UpstreamEndpoint struct {
	URL  string        `json:"url,omitempty" db:"-"`
	Ref  string        `json:"ref,omitempty" db:"-"`
	Auth *UpstreamAuth `json:"auth,omitempty" db:"-"`
}

type UpstreamAuth struct {
	Type   string `json:"type" db:"-"`
	Header string `json:"header,omitempty" db:"-"`
	Value  string `json:"value,omitempty" db:"-"`
}
