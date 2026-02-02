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

import (
	"time"
)

// API represents an API entity in the platform
type API struct {
	ID               string              `json:"id,omitempty" yaml:"id,omitempty"`
	Name             string              `json:"name" yaml:"name"`
	Description      string              `json:"description,omitempty" yaml:"description,omitempty"`
	Context          string              `json:"context" yaml:"context"`
	Version          string              `json:"version" yaml:"version"`
	Provider         string              `json:"provider,omitempty" yaml:"provider,omitempty"`
	ProjectID        string              `json:"projectId" yaml:"projectId"`
	OrganizationID   string              `json:"organizationId" yaml:"organizationId"`
	CreatedAt        time.Time           `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt        time.Time           `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	LifeCycleStatus  string              `json:"lifeCycleStatus,omitempty" yaml:"lifeCycleStatus,omitempty"`
	HasThumbnail     bool                `json:"hasThumbnail,omitempty" yaml:"hasThumbnail,omitempty"`
	IsDefaultVersion bool                `json:"isDefaultVersion,omitempty" yaml:"isDefaultVersion,omitempty"`
	Type             string              `json:"type,omitempty" yaml:"type,omitempty"`
	Transport        []string            `json:"transport,omitempty" yaml:"transport,omitempty"`
	MTLS             *MTLSConfig         `json:"mtls,omitempty" yaml:"mtls,omitempty"`
	Security         *SecurityConfig     `json:"security,omitempty" yaml:"security,omitempty"`
	CORS             *CORSConfig         `json:"cors,omitempty" yaml:"cors,omitempty"`
	BackendServices  []BackendService    `json:"backend-services,omitempty" yaml:"backend-services,omitempty"`
	APIRateLimiting  *RateLimitingConfig `json:"api-rate-limiting,omitempty" yaml:"api-rate-limiting,omitempty"`
	Policies         []Policy            `json:"policies,omitempty" yaml:"policies,omitempty"`
	Operations       []Operation         `json:"operations,omitempty" yaml:"operations,omitempty"`
	Channels         []Channel           `json:"channels,omitempty" yaml:"channels,omitempty"`
}

// MTLSConfig represents mutual TLS configuration
type MTLSConfig struct {
	Enabled                    bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	EnforceIfClientCertPresent bool   `json:"enforceIfClientCertPresent,omitempty" yaml:"enforceIfClientCertPresent,omitempty"`
	VerifyClient               bool   `json:"verifyClient,omitempty" yaml:"verifyClient,omitempty"`
	ClientCert                 string `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey                  string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	CACert                     string `json:"caCert,omitempty" yaml:"caCert,omitempty"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	Enabled       bool                   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	APIKey        *APIKeySecurity        `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	OAuth2        *OAuth2Security        `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
	XHubSignature *XHubSignatureSecurity `json:"xHubSignature,omitempty" yaml:"xHubSignature,omitempty"`
}

// XHubSignatureSecurity represents X-Hub-Signature security configuration
type XHubSignatureSecurity struct {
	Enabled   bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Header    string `json:"header,omitempty" yaml:"header,omitempty"`
	Secret    string `json:"secret,omitempty" yaml:"secret,omitempty"`
	Algorithm string `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
}

// APIKeySecurity represents API key security configuration
type APIKeySecurity struct {
	Enabled bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Header  string `json:"header,omitempty" yaml:"header,omitempty"`
	Query   string `json:"query,omitempty" yaml:"query,omitempty"`
	Cookie  string `json:"cookie,omitempty" yaml:"cookie,omitempty"`
}

// OAuth2Security represents OAuth2 security configuration
type OAuth2Security struct {
	GrantTypes *OAuth2GrantTypes `json:"grantTypes,omitempty" yaml:"grantTypes,omitempty"`
	Scopes     []string          `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// OAuth2GrantTypes represents OAuth2 grant types configuration
type OAuth2GrantTypes struct {
	AuthorizationCode *AuthorizationCodeGrant `json:"authorizationCode,omitempty" yaml:"authorizationCode,omitempty"`
	Implicit          *ImplicitGrant          `json:"implicit,omitempty" yaml:"implicit,omitempty"`
	Password          *PasswordGrant          `json:"password,omitempty" yaml:"password,omitempty"`
	ClientCredentials *ClientCredentialsGrant `json:"clientCredentials,omitempty" yaml:"clientCredentials,omitempty"`
}

// AuthorizationCodeGrant represents authorization code grant configuration
type AuthorizationCodeGrant struct {
	Enabled     bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	CallbackURL string `json:"callbackUrl,omitempty" yaml:"callbackUrl,omitempty"`
}

// ImplicitGrant represents implicit grant configuration
type ImplicitGrant struct {
	Enabled     bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	CallbackURL string `json:"callbackUrl,omitempty" yaml:"callbackUrl,omitempty"`
}

// PasswordGrant represents password grant configuration
type PasswordGrant struct {
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// ClientCredentialsGrant represents client credentials grant configuration
type ClientCredentialsGrant struct {
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled          bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	AllowOrigins     string `json:"allowOrigins,omitempty" yaml:"allowOrigins,omitempty"`
	AllowMethods     string `json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`
	AllowHeaders     string `json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`
	ExposeHeaders    string `json:"exposeHeaders,omitempty" yaml:"exposeHeaders,omitempty"`
	MaxAge           int    `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	AllowCredentials bool   `json:"allowCredentials,omitempty" yaml:"allowCredentials,omitempty"`
}

// RateLimitingConfig represents rate limiting configuration
type RateLimitingConfig struct {
	Enabled           bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	RateLimitCount    int    `json:"rateLimitCount,omitempty" yaml:"rateLimitCount,omitempty"`
	RateLimitTimeUnit string `json:"rateLimitTimeUnit,omitempty" yaml:"rateLimitTimeUnit,omitempty"`
	StopOnQuotaReach  bool   `json:"stopOnQuotaReach,omitempty" yaml:"stopOnQuotaReach,omitempty"`
}

// Operation represents an API operation
type Operation struct {
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Request     *OperationRequest `json:"request" yaml:"request" binding:"required"`
}

// Channel represents an API channel
type Channel struct {
	Name        string          `json:"name,omitempty" yaml:"name,omitempty"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Request     *ChannelRequest `json:"request" yaml:"request" binding:"required"`
}

// OperationRequest represents operation request details
type OperationRequest struct {
	Method          string                `json:"method" yaml:"method" binding:"required"`
	Path            string                `json:"path" yaml:"path" binding:"required"`
	BackendServices []BackendRouting      `json:"backend-services,omitempty" yaml:"backend-services,omitempty"`
	Authentication  *AuthenticationConfig `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	Policies        []Policy              `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// ChannelRequest represents channel request details
type ChannelRequest struct {
	Method          string                `json:"method" yaml:"method" binding:"required"`
	Name            string                `json:"name" yaml:"name" binding:"required"`
	BackendServices []BackendRouting      `json:"backend-services,omitempty" yaml:"backend-services,omitempty"`
	Authentication  *AuthenticationConfig `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	Policies        []Policy              `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// BackendRouting represents backend routing configuration
type BackendRouting struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Weight int    `json:"weight,omitempty" yaml:"weight,omitempty"`
}

// AuthenticationConfig represents authentication configuration for operations
type AuthenticationConfig struct {
	Required bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Scopes   []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// Policy represents a request or response policy
type Policy struct {
	ExecutionCondition *string                 `json:"executionCondition,omitempty" yaml:"executionCondition,omitempty"`
	Name               string                  `json:"name" yaml:"name"`
	Params             *map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
	Version            string                  `json:"version" yaml:"version"`
}

// DeployAPIRequest represents a request to deploy an API
type DeployAPIRequest struct {
	Name      string                 `json:"name" yaml:"name"`                              // Deployment name
	Base      string                 `json:"base" yaml:"base"`                              // "current" or a deploymentId
	GatewayID string                 `json:"gatewayId" yaml:"gatewayId"`                    // Target gateway ID
	Metadata  map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"` // Flexible key-value metadata
}

// DeploymentResponse represents a deployment artifact
type DeploymentResponse struct {
	DeploymentID     string                 `json:"deploymentId" yaml:"deploymentId"`
	Name             string                 `json:"name" yaml:"name"`
	GatewayID        string                 `json:"gatewayId" yaml:"gatewayId"`
	Status           string                 `json:"status" yaml:"status"` // DEPLOYED, UNDEPLOYED, or ARCHIVED
	BaseDeploymentID *string                `json:"baseDeploymentId,omitempty" yaml:"baseDeploymentId,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"createdAt" yaml:"createdAt"`
	UpdatedAt        *time.Time             `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"` // When status last changed (nil for ARCHIVED)
}

// DeploymentListResponse represents a list of deployments
type DeploymentListResponse struct {
	Count int                   `json:"count" yaml:"count"`
	List  []*DeploymentResponse `json:"list" yaml:"list"`
}

// APIDeploymentYAML represents the API deployment YAML structure
type APIDeploymentYAML struct {
	ApiVersion string                `yaml:"apiVersion" binding:"required"`
	Kind       string                `yaml:"kind" binding:"required"`
	Metadata   APIDeploymentMetadata `yaml:"metadata" binding:"required"`
	Spec       APIYAMLData           `yaml:"spec" binding:"required"`
}

// APIDeploymentMetadata represents the metadata section of the API deployment YAML
type APIDeploymentMetadata struct {
	Name   string            `yaml:"name" binding:"required"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// APIYAMLData represents a basic spec section of the API deployment YAML
type APIYAMLData struct {
	DisplayName string             `yaml:"displayName"`
	Version     string             `yaml:"version"`
	Context     string             `yaml:"context"`
	Upstream    *UpstreamYAML      `yaml:"upstream,omitempty"`
	Policies    []Policy           `yaml:"policies,omitempty"`
	Operations  []OperationRequest `yaml:"operations,omitempty"`
	Channels    []ChannelRequest   `yaml:"channels,omitempty"`
}

// UpstreamYAML represents the upstream configuration for API deployment YAML
type UpstreamYAML struct {
	Main    *UpstreamTarget `yaml:"main,omitempty"`
	Sandbox *UpstreamTarget `yaml:"sandbox,omitempty"`
}

// UpstreamTarget represents a single upstream target (url or ref)
type UpstreamTarget struct {
	URL string `yaml:"url,omitempty"`
	Ref string `yaml:"ref,omitempty"`
}

// APIListResponse represents a paginated list of APIs (constitution-compliant)
type APIListResponse struct {
	Count      int        `json:"count" yaml:"count"`           // Number of items in current response
	List       []*API     `json:"list" yaml:"list"`             // Array of API objects
	Pagination Pagination `json:"pagination" yaml:"pagination"` // Pagination metadata
}

// APIValidationRequest represents the request parameters for API validation
type APIValidationRequest struct {
	Identifier string `form:"identifier"`
	Name       string `form:"name"`
	Version    string `form:"version"`
}

// APIValidationResponse represents the response for API validation
type APIValidationResponse struct {
	Valid bool                `json:"valid"`
	Error *APIValidationError `json:"error"`
}

// APIValidationError represents the error object in the validation response
type APIValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
