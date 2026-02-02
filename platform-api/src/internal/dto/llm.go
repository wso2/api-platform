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

type ExtractionIdentifier struct {
	Location   string `json:"location" yaml:"location" binding:"required"`
	Identifier string `json:"identifier" yaml:"identifier" binding:"required"`
}

type RouteException struct {
	Path    string   `json:"path" yaml:"path" binding:"required"`
	Methods []string `json:"methods" yaml:"methods" binding:"required"`
}

type LLMAccessControl struct {
	Mode       string           `json:"mode" yaml:"mode" binding:"required"`
	Exceptions []RouteException `json:"exceptions,omitempty" yaml:"exceptions,omitempty"`
}

type LLMPolicyPath struct {
	Path    string                 `json:"path" yaml:"path" binding:"required"`
	Methods []string               `json:"methods" yaml:"methods" binding:"required"`
	Params  map[string]interface{} `json:"params" yaml:"params" binding:"required"`
}

type LLMPolicy struct {
	Name    string          `json:"name" yaml:"name" binding:"required"`
	Version string          `json:"version" yaml:"version" binding:"required"`
	Paths   []LLMPolicyPath `json:"paths" yaml:"paths" binding:"required"`
}

type LLMUpstreamAuth struct {
	Type   string `json:"type" yaml:"type" binding:"required"`
	Header string `json:"header,omitempty" yaml:"header,omitempty"`
	Value  string `json:"value,omitempty" yaml:"value,omitempty"`
}

type LLMUpstream struct {
	URL  string           `json:"url" yaml:"url" binding:"required"`
	Auth *LLMUpstreamAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
}

type LLMProviderTemplateAuth struct {
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Header      string `json:"header,omitempty" yaml:"header,omitempty"`
	ValuePrefix string `json:"valuePrefix,omitempty" yaml:"valuePrefix,omitempty"`
}

type LLMProviderTemplateMetadata struct {
	EndpointURL string                   `json:"endpointUrl,omitempty" yaml:"endpointUrl,omitempty"`
	Auth        *LLMProviderTemplateAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
	LogoURL     string                   `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
}

type LLMProviderTemplate struct {
	ID               string                `json:"id" yaml:"id" binding:"required"`
	Name             string                `json:"name" yaml:"name" binding:"required"`
	Description      string                `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedBy        string                `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Metadata         *LLMProviderTemplateMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	PromptTokens     *ExtractionIdentifier `json:"promptTokens,omitempty" yaml:"promptTokens,omitempty"`
	CompletionTokens *ExtractionIdentifier `json:"completionTokens,omitempty" yaml:"completionTokens,omitempty"`
	TotalTokens      *ExtractionIdentifier `json:"totalTokens,omitempty" yaml:"totalTokens,omitempty"`
	RemainingTokens  *ExtractionIdentifier `json:"remainingTokens,omitempty" yaml:"remainingTokens,omitempty"`
	RequestModel     *ExtractionIdentifier `json:"requestModel,omitempty" yaml:"requestModel,omitempty"`
	ResponseModel    *ExtractionIdentifier `json:"responseModel,omitempty" yaml:"responseModel,omitempty"`
	CreatedAt        time.Time             `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt        time.Time             `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

type LLMProviderTemplateListItem struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedBy   string    `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
}

type LLMProviderTemplateListResponse struct {
	Count      int                           `json:"count" yaml:"count"`
	List       []LLMProviderTemplateListItem `json:"list" yaml:"list"`
	Pagination Pagination                   `json:"pagination" yaml:"pagination"`
}

type LLMProvider struct {
	ID            string           `json:"id" yaml:"id" binding:"required"`
	Name          string           `json:"name" yaml:"name" binding:"required"`
	Description   string           `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedBy     string           `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Version       string           `json:"version" yaml:"version" binding:"required"`
	Context       string           `json:"context,omitempty" yaml:"context,omitempty"`
	VHost         string           `json:"vhost,omitempty" yaml:"vhost,omitempty"`
	Template      string           `json:"template" yaml:"template" binding:"required"`
	Upstream      LLMUpstream      `json:"upstream" yaml:"upstream" binding:"required"`
	OpenAPI       string           `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	AccessControl LLMAccessControl `json:"accessControl" yaml:"accessControl" binding:"required"`
	Policies      []LLMPolicy      `json:"policies,omitempty" yaml:"policies,omitempty"`
	Security      *SecurityConfig  `json:"security,omitempty" yaml:"security,omitempty"`
	CreatedAt     time.Time        `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt     time.Time        `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

type LLMProviderListItem struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedBy   string    `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Version     string    `json:"version" yaml:"version"`
	Template    string    `json:"template" yaml:"template"`
	Status      string    `json:"status" yaml:"status"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
}

type LLMProviderListResponse struct {
	Count      int                   `json:"count" yaml:"count"`
	List       []LLMProviderListItem `json:"list" yaml:"list"`
	Pagination Pagination            `json:"pagination" yaml:"pagination"`
}

type LLMProxy struct {
	ID            string            `json:"id" yaml:"id" binding:"required"`
	Name          string            `json:"name" yaml:"name" binding:"required"`
	Description   string            `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedBy     string            `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Version       string            `json:"version" yaml:"version" binding:"required"`
	ProjectID     string            `json:"projectId" yaml:"projectId"`
	Context       string            `json:"context,omitempty" yaml:"context,omitempty"`
	VHost         string            `json:"vhost,omitempty" yaml:"vhost,omitempty"`
	Provider      string            `json:"provider" yaml:"provider" binding:"required"`
	OpenAPI       string            `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	AccessControl *LLMAccessControl `json:"accessControl,omitempty" yaml:"accessControl,omitempty"`
	Policies      []LLMPolicy       `json:"policies,omitempty" yaml:"policies,omitempty"`
	Security      *SecurityConfig   `json:"security,omitempty" yaml:"security,omitempty"`
	CreatedAt     time.Time         `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt     time.Time         `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

type LLMProxyListItem struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedBy   string    `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Version     string    `json:"version" yaml:"version"`
	ProjectID   string    `json:"projectId" yaml:"projectId"`
	Provider    string    `json:"provider" yaml:"provider"`
	Status      string    `json:"status" yaml:"status"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
}

type LLMProxyListResponse struct {
	Count      int                `json:"count" yaml:"count"`
	List       []LLMProxyListItem `json:"list" yaml:"list"`
	Pagination Pagination         `json:"pagination" yaml:"pagination"`
}
