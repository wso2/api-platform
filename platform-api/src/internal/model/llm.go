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

type ExtractionIdentifier struct {
	Location   string `json:"location" db:"-"`
	Identifier string `json:"identifier" db:"-"`
}

type LLMModel struct {
	ID          string `json:"id" db:"-"`
	Name        string `json:"name,omitempty" db:"-"`
	Description string `json:"description,omitempty" db:"-"`
}

type LLMModelProvider struct {
	ID     string     `json:"id" db:"-"`
	Name   string     `json:"name,omitempty" db:"-"`
	Models []LLMModel `json:"models,omitempty" db:"-"`
}

type RouteException struct {
	Path    string   `json:"path" db:"-"`
	Methods []string `json:"methods" db:"-"`
}

type LLMAccessControl struct {
	Mode       string           `json:"mode" db:"-"`
	Exceptions []RouteException `json:"exceptions,omitempty" db:"-"`
}

type LLMPolicyPath struct {
	Path    string                 `json:"path" db:"-"`
	Methods []string               `json:"methods" db:"-"`
	Params  map[string]interface{} `json:"params" db:"-"`
}

type LLMPolicy struct {
	Name    string          `json:"name" db:"-"`
	Version string          `json:"version" db:"-"`
	Paths   []LLMPolicyPath `json:"paths" db:"-"`
}

type RateLimitingLimitConfig struct {
	Request *RequestRateLimit `json:"request,omitempty" db:"-"`
	Token   *TokenRateLimit   `json:"token,omitempty" db:"-"`
	Cost    *CostRateLimit    `json:"cost,omitempty" db:"-"`
}

type RateLimitResetWindow struct {
	Duration int    `json:"duration" db:"-"`
	Unit     string `json:"unit" db:"-"`
}

type RequestRateLimit struct {
	Enabled bool                 `json:"enabled" db:"-"`
	Count   int                  `json:"count" db:"-"`
	Reset   RateLimitResetWindow `json:"reset" db:"-"`
}

type TokenRateLimit struct {
	Enabled bool                 `json:"enabled" db:"-"`
	Count   int                  `json:"count" db:"-"`
	Reset   RateLimitResetWindow `json:"reset" db:"-"`
}

type CostRateLimit struct {
	Enabled bool                 `json:"enabled" db:"-"`
	Amount  float64              `json:"amount" db:"-"`
	Reset   RateLimitResetWindow `json:"reset" db:"-"`
}

type RateLimitingResourceLimit struct {
	Resource string                  `json:"resource" db:"-"`
	Limit    RateLimitingLimitConfig `json:"limit" db:"-"`
}

type ResourceWiseRateLimitingConfig struct {
	Default   RateLimitingLimitConfig     `json:"default" db:"-"`
	Resources []RateLimitingResourceLimit `json:"resources" db:"-"`
}

type RateLimitingScopeConfig struct {
	Global       *RateLimitingLimitConfig        `json:"global,omitempty" db:"-"`
	ResourceWise *ResourceWiseRateLimitingConfig `json:"resourceWise,omitempty" db:"-"`
}

type LLMRateLimitingConfig struct {
	ProviderLevel *RateLimitingScopeConfig `json:"providerLevel,omitempty" db:"-"`
	ConsumerLevel *RateLimitingScopeConfig `json:"consumerLevel,omitempty" db:"-"`
}

type LLMProviderTemplateAuth struct {
	Type        string `json:"type,omitempty" db:"-"`
	Header      string `json:"header,omitempty" db:"-"`
	ValuePrefix string `json:"valuePrefix,omitempty" db:"-"`
}

type LLMProviderTemplateMetadata struct {
	EndpointURL    string                   `json:"endpointUrl,omitempty" db:"-"`
	Auth           *LLMProviderTemplateAuth `json:"auth,omitempty" db:"-"`
	LogoURL        string                   `json:"logoUrl,omitempty" db:"-"`
	OpenapiSpecURL string                   `json:"openapiSpecUrl,omitempty" db:"-"`
}

type LLMProviderTemplateExtractionFields struct {
	PromptTokens     *ExtractionIdentifier `json:"promptTokens,omitempty" db:"-"`
	CompletionTokens *ExtractionIdentifier `json:"completionTokens,omitempty" db:"-"`
	TotalTokens      *ExtractionIdentifier `json:"totalTokens,omitempty" db:"-"`
	RemainingTokens  *ExtractionIdentifier `json:"remainingTokens,omitempty" db:"-"`
	RequestModel     *ExtractionIdentifier `json:"requestModel,omitempty" db:"-"`
	ResponseModel    *ExtractionIdentifier `json:"responseModel,omitempty" db:"-"`
}

type LLMProviderTemplateResourceMapping struct {
	Resource string `json:"resource" db:"-"`
	LLMProviderTemplateExtractionFields
}

type LLMProviderTemplateResourceMappings struct {
	Resources []LLMProviderTemplateResourceMapping `json:"resources,omitempty" db:"-"`
}

type LLMProviderTemplate struct {
	UUID             string                       `json:"uuid" db:"uuid"`
	OrganizationUUID string                       `json:"organizationId" db:"organization_uuid"`
	ID               string                       `json:"id" db:"handle"`
	Name             string                       `json:"name" db:"name"`
	Description      string                       `json:"description,omitempty" db:"description"`
	CreatedBy        string                       `json:"createdBy,omitempty" db:"created_by"`
	Metadata         *LLMProviderTemplateMetadata `json:"metadata,omitempty" db:"-"`
	PromptTokens     *ExtractionIdentifier        `json:"promptTokens,omitempty" db:"-"`
	CompletionTokens *ExtractionIdentifier        `json:"completionTokens,omitempty" db:"-"`
	TotalTokens      *ExtractionIdentifier        `json:"totalTokens,omitempty" db:"-"`
	RemainingTokens  *ExtractionIdentifier        `json:"remainingTokens,omitempty" db:"-"`
	RequestModel     *ExtractionIdentifier        `json:"requestModel,omitempty" db:"-"`
	ResponseModel    *ExtractionIdentifier        `json:"responseModel,omitempty" db:"-"`
	ResourceMappings *LLMProviderTemplateResourceMappings `json:"resourceMappings,omitempty" db:"-"`
	CreatedAt        time.Time                    `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time                    `json:"updatedAt" db:"updated_at"`
}

// LLMProvider represents an LLM provider entity
type LLMProvider struct {
	UUID             string             `json:"uuid" db:"uuid"`
	OrganizationUUID string             `json:"organizationId" db:"organization_uuid"`
	ID               string             `json:"id" db:"handle"`
	Name             string             `json:"name" db:"name"`
	Description      string             `json:"description,omitempty" db:"description"`
	CreatedBy        string             `json:"createdBy,omitempty" db:"created_by"`
	Version          string             `json:"version" db:"version"`
	TemplateUUID     string             `json:"templateUuid" db:"template_uuid"`
	OpenAPISpec      string             `json:"openapi,omitempty" db:"openapi_spec"`
	ModelProviders   []LLMModelProvider `json:"modelProviders,omitempty" db:"-"`
	Status           string             `json:"status" db:"status"`
	CreatedAt        time.Time          `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time          `json:"updatedAt" db:"updated_at"`
	Configuration    LLMProviderConfig  `json:"configuration" db:"configuration"`
}

type LLMProviderConfig struct {
	Name          string                 `json:"name,omitempty" db:"-"`
	Version       string                 `json:"version,omitempty" db:"-"`
	Context       *string                `json:"context,omitempty" db:"-"`
	VHost         *string                `json:"vhost,omitempty" db:"-"`
	Template      string                 `json:"template,omitempty" db:"-"`
	Upstream      *UpstreamConfig        `json:"upstream,omitempty" db:"-"`
	AccessControl *LLMAccessControl      `json:"accessControl,omitempty" db:"-"`
	RateLimiting  *LLMRateLimitingConfig `json:"rateLimiting,omitempty" db:"-"`
	Policies      []LLMPolicy            `json:"policies,omitempty" db:"-"`
	Security      *SecurityConfig        `json:"security,omitempty" db:"-"`
}

// LLMProxy represents an LLM proxy entity
type LLMProxy struct {
	UUID             string         `json:"uuid" db:"uuid"`
	OrganizationUUID string         `json:"organizationId" db:"organization_uuid"`
	ID               string         `json:"id" db:"handle"`
	Name             string         `json:"name" db:"name"`
	ProjectUUID      string         `json:"projectId" db:"project_uuid"`
	Description      string         `json:"description,omitempty" db:"description"`
	CreatedBy        string         `json:"createdBy,omitempty" db:"created_by"`
	Version          string         `json:"version" db:"version"`
	ProviderUUID     string         `json:"providerUuid" db:"provider_uuid"`
	OpenAPISpec      string         `json:"openapi,omitempty" db:"openapi_spec"`
	Status           string         `json:"status" db:"status"`
	CreatedAt        time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time      `json:"updatedAt" db:"updated_at"`
	Configuration    LLMProxyConfig `json:"configuration" db:"configuration"`
}

type LLMProxyConfig struct {
	Name         string          `json:"name,omitempty" db:"-"`
	Version      string          `json:"version,omitempty" db:"-"`
	Context      *string         `json:"context,omitempty" db:"-"`
	Vhost        *string         `json:"vhost,omitempty" db:"-"`
	Provider     string          `json:"provider,omitempty" db:"-"`
	UpstreamAuth *UpstreamAuth   `json:"upstreamAuth,omitempty" db:"-"`
	Policies     []LLMPolicy     `json:"policies,omitempty" db:"-"`
	Security     *SecurityConfig `json:"security,omitempty" db:"-"`
}

type SecurityConfig struct {
	Enabled *bool           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	APIKey  *APIKeySecurity `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
}

type APIKeySecurity struct {
	Enabled *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Key     string `json:"key,omitempty" yaml:"key,omitempty"`
	In      string `json:"in,omitempty" yaml:"in,omitempty"`
}
