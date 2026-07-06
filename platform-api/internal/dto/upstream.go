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

// UpstreamConfig represents the upstream configuration with main and sandbox endpoints
type UpstreamConfig struct {
	Main    *UpstreamEndpoint `json:"main,omitempty" yaml:"main,omitempty" binding:"required"`
	Sandbox *UpstreamEndpoint `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
}

// UpstreamEndpoint represents an upstream endpoint configuration
type UpstreamEndpoint struct {
	URL  string        `json:"url,omitempty" yaml:"url,omitempty"`
	Ref  string        `json:"ref,omitempty" yaml:"ref,omitempty"`
	Auth *UpstreamAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
}

// UpstreamAuth represents authentication configuration for upstream
type UpstreamAuth struct {
	Type   string `json:"type" yaml:"type" binding:"required"`
	Header string `json:"header,omitempty" yaml:"header,omitempty"`
	Value  string `json:"value,omitempty" yaml:"value,omitempty"`
}
