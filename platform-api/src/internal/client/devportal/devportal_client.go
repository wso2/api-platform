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

package devportal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/client"
	"platform-api/src/internal/client/devportal/dto"
)

// DevPortalError represents an error from developer portal operations
//
// This error type provides structured error information for intelligent
// retry logic and clear error messages to API consumers.
type DevPortalError struct {
	Code       int    // HTTP status code from developer portal
	Message    string // Human-readable error message
	Retryable  bool   // Whether the error should trigger a retry
	Underlying error  // Underlying error if any
}

// Error implements the error interface for DevPortalError
//
// Returns:
//   - string: Formatted error message including status code and message
func (e *DevPortalError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("devportal error (%d): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("devportal error: %s", e.Message)
}

// Unwrap returns the underlying error for error unwrapping
//
// Returns:
//   - error: The underlying error if present, nil otherwise
func (e *DevPortalError) Unwrap() error {
	return e.Underlying
}

// NewDevPortalError creates a new DevPortalError
//
// Parameters:
//   - code: HTTP status code (0 if not applicable)
//   - message: Error message
//   - retryable: Whether this error should trigger a retry
//   - underlying: Underlying error (can be nil)
//
// Returns:
//   - *DevPortalError: A structured error instance
func NewDevPortalError(code int, message string, retryable bool, underlying error) *DevPortalError {
	return &DevPortalError{
		Code:       code,
		Message:    message,
		Retryable:  retryable,
		Underlying: underlying,
	}
}

// IsRetryable checks if the error should trigger a retry
//
// Returns:
//   - bool: True if the error is retryable (5xx errors, network errors)
func (e *DevPortalError) IsRetryable() bool {
	return e.Retryable
}

// DevPortalClient handles HTTP communication with the developer portal
//
// This client provides methods for creating organizations, managing subscription policies,
// and publishing APIs to the developer portal with automatic retry logic.
type DevPortalClient struct {
	httpClient *client.RetryableHTTPClient // HTTP client with retry capabilities
	baseURL    string                      // Developer portal base URL (e.g., "172.17.0.1:3001")
	apiKey     string                      // Authentication API key
	enabled    bool                        // Whether developer portal integration is enabled
}

// NewDevPortalClient creates a new developer portal client from configuration
//
// Parameters:
//   - cfg: Developer portal configuration from config package
//
// Returns:
//   - *DevPortalClient: Configured client instance
//
// The client initializes with:
//   - 3 retry attempts (per spec requirement)
//   - Configured timeout duration (default 15 seconds)
//   - Base URL and API key from configuration
func NewDevPortalClient(cfg config.DevPortal) *DevPortalClient {
	// Convert timeout from seconds to duration
	timeout := time.Duration(cfg.Timeout) * time.Second

	// Create HTTP client with retry logic (max 3 retries per spec)
	httpClient := client.NewRetryableHTTPClient(3, timeout)

	// Log configuration status
	if cfg.Enabled {
		log.Printf("[DevPortal] Developer portal integration enabled. BaseURL: %s, Timeout: %d seconds",
			cfg.BaseURL, cfg.Timeout)
	} else {
		log.Printf("[DevPortal] Developer portal integration disabled")
	}

	return &DevPortalClient{
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		enabled:    cfg.Enabled,
	}
}

// IsEnabled checks if developer portal integration is enabled
//
// Returns:
//   - bool: True if integration is enabled in configuration
func (c *DevPortalClient) IsEnabled() bool {
	return c.enabled
}

// CreateOrganization creates a new organization in the developer portal
//
// This method is called during organization creation in platform-api to synchronize
// organizations to the developer portal. It uses retry logic to handle transient failures.
//
// Parameters:
//   - req: Organization creation request with ID, Name, DisplayName, Description
//
// Returns:
//   - *dto.OrganizationCreateResponse: Response with created organization details
//   - error: DevPortalError if creation fails after retries
func (c *DevPortalClient) CreateOrganization(req *dto.OrganizationCreateRequest) (*dto.OrganizationCreateResponse, error) {
	url := fmt.Sprintf("http://%s/devportal/organizations", c.baseURL)

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to marshal organization request", false, err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create HTTP request", false, err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-wso2-api-key", c.apiKey)

	log.Printf("[DevPortal] Creating organization: %s (ID: %s)", req.OrgName, req.OrgID)

	// Execute request with retry logic
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create organization after retries", true, err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewDevPortalError(resp.StatusCode, "failed to read response body", false, err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("organization creation failed: %s", string(respBody)), resp.StatusCode >= 500, nil)
	}

	// Unmarshal response
	var orgResp dto.OrganizationCreateResponse
	if err := json.Unmarshal(respBody, &orgResp); err != nil {
		return nil, NewDevPortalError(resp.StatusCode, "failed to unmarshal response", false, err)
	}

	log.Printf("[DevPortal] Organization created successfully: %s (ID: %s)", orgResp.OrgName, orgResp.OrgID)
	return &orgResp, nil
}

// CreateSubscriptionPolicy creates a subscription policy for an organization in the developer portal
//
// This method is used to create the default "unlimited" subscription policy for new organizations.
//
// Parameters:
//   - orgID: Organization UUID
//   - req: Subscription policy creation request
//
// Returns:
//   - *dto.SubscriptionPolicyCreateResponse: Response with created policy details
//   - error: DevPortalError if creation fails after retries
func (c *DevPortalClient) CreateSubscriptionPolicy(orgID string, req *dto.SubscriptionPolicyCreateRequest) (*dto.SubscriptionPolicyCreateResponse, error) {
	url := fmt.Sprintf("http://%s/devportal/organizations/%s/subscription-policies", c.baseURL, orgID)

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to marshal subscription policy request", false, err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create HTTP request", false, err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-wso2-api-key", c.apiKey)

	log.Printf("[DevPortal] Creating subscription policy '%s' for organization: %s", req.PolicyName, orgID)

	// Execute request with retry logic
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create subscription policy after retries", true, err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewDevPortalError(resp.StatusCode, "failed to read response body", false, err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("subscription policy creation failed: %s", string(respBody)), resp.StatusCode >= 500, nil)
	}

	// Unmarshal response
	var policyResp dto.SubscriptionPolicyCreateResponse
	if err := json.Unmarshal(respBody, &policyResp); err != nil {
		return nil, NewDevPortalError(resp.StatusCode, "failed to unmarshal response", false, err)
	}

	log.Printf("[DevPortal] Subscription policy created successfully: %s (ID: %s)", policyResp.PolicyName, policyResp.ID)
	return &policyResp, nil
}

// CreateDefaultSubscriptionPolicy constructs the default "unlimited" subscription policy request
//
// Per spec requirements, the unlimited policy has:
//   - Policy name: "unlimited"
//   - Display name: "Unlimited Tier"
//   - Billing plan: "FREE"
//   - Request count: 1000000 per minute
//
// Returns:
//   - *dto.SubscriptionPolicyCreateRequest: Configured unlimited policy request
func (c *DevPortalClient) CreateDefaultSubscriptionPolicy() *dto.SubscriptionPolicyCreateRequest {
	return &dto.SubscriptionPolicyCreateRequest{
		PolicyName:   "unlimited",
		DisplayName:  "Unlimited Tier",
		BillingPlan:  "FREE",
		Description:  "Allows unlimited requests per minute",
		Type:         "requestCount",
		TimeUnit:     60,
		UnitTime:     "min",
		RequestCount: 1000000,
	}
}

// createMultipartRequest creates a multipart/form-data request with API metadata and definition
//
// This helper constructs the multipart request required by the developer portal API publishing endpoint.
// The request contains:
//   - apiMetadata: JSON-serialized API metadata (Content-Type: application/json)
//   - apiDefinition: OpenAPI definition file (must be named "apiDefinition.json")
//
// Parameters:
//   - metadata: API metadata request
//   - apiDefinition: OpenAPI definition content (JSON bytes)
//
// Returns:
//   - *bytes.Buffer: Multipart request body
//   - string: Content-Type header value with boundary
//   - error: Error if multipart creation fails
func (c *DevPortalClient) createMultipartRequest(metadata *dto.APIPublishRequest, apiDefinition []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add apiMetadata field as JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal API metadata: %w", err)
	}

	// Create apiMetadata field with application/json content type
	metadataField, err := writer.CreateFormField("apiMetadata")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create apiMetadata form field: %w", err)
	}
	if _, err := metadataField.Write(metadataJSON); err != nil {
		return nil, "", fmt.Errorf("failed to write apiMetadata: %w", err)
	}

	// Add apiDefinition file field
	// IMPORTANT: File must be named "apiDefinition.json" per devportal API spec
	fileField, err := writer.CreateFormFile("apiDefinition", "apiDefinition.json")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create apiDefinition form file: %w", err)
	}
	if _, err := fileField.Write(apiDefinition); err != nil {
		return nil, "", fmt.Errorf("failed to write apiDefinition: %w", err)
	}

	// Close writer to finalize multipart body
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

// PublishAPI publishes an API to the developer portal
//
// This method creates a new API in the developer portal with metadata and OpenAPI definition.
// It uses multipart/form-data to send both the API metadata (JSON) and the OpenAPI definition file.
//
// Parameters:
//   - orgID: Organization UUID
//   - req: API publish request with metadata
//   - apiDefinition: OpenAPI definition content (JSON bytes)
//
// Returns:
//   - *dto.APIPublishResponse: Response with created API details
//   - error: DevPortalError if publishing fails after retries
func (c *DevPortalClient) PublishAPI(orgID string, req *dto.APIPublishRequest, apiDefinition []byte) (*dto.APIPublishResponse, error) {
	url := fmt.Sprintf("http://%s/devportal/apis", c.baseURL)

	// Create multipart request body
	body, contentType, err := c.createMultipartRequest(req, apiDefinition)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create multipart request", false, err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create HTTP request", false, err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("x-wso2-api-key", c.apiKey)
	httpReq.Header.Set("organization", orgID)

	log.Printf("[DevPortal] Publishing API: %s (Organization: %s, ReferenceID: %s)",
		req.APIInfo.APIName, orgID, req.APIInfo.ReferenceID)

	// Execute request with retry logic
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to publish API after retries", true, err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewDevPortalError(resp.StatusCode, "failed to read response body", false, err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("API publishing failed: %s", string(respBody)), resp.StatusCode >= 500, nil)
	}

	// Unmarshal response
	var apiResp dto.APIPublishResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, NewDevPortalError(resp.StatusCode, "failed to unmarshal response", false, err)
	}

	log.Printf("[DevPortal] API published successfully: %s (ID: %s, ReferenceID: %s)",
		apiResp.APIHandle, apiResp.APIID, apiResp.ReferenceID)
	return &apiResp, nil
}

// UnpublishAPI unpublishes an API from the developer portal
//
// This method deletes an API from the developer portal by its API ID.
// It uses retry logic to handle transient failures.
//
// Parameters:
//   - orgID: Organization UUID
//   - apiID: Developer portal API UUID (not platform-api API UUID)
//
// Returns:
//   - error: DevPortalError if unpublishing fails after retries, nil on success
func (c *DevPortalClient) UnpublishAPI(orgID string, apiID string) error {
	url := fmt.Sprintf("http://%s/devportal/apis/%s", c.baseURL, apiID)

	// Create HTTP request
	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return NewDevPortalError(0, "failed to create HTTP request", false, err)
	}

	// Set headers
	httpReq.Header.Set("x-wso2-api-key", c.apiKey)
	httpReq.Header.Set("organization", orgID)
	httpReq.Header.Set("Accept", "application/json")

	log.Printf("[DevPortal] Unpublishing API: %s (Organization: %s)", apiID, orgID)

	// Execute request with retry logic
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return NewDevPortalError(0, "failed to unpublish API after retries", true, err)
	}
	defer resp.Body.Close()

	// Read response body for error messages
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewDevPortalError(resp.StatusCode, "failed to read response body", false, err)
	}

	// Check status code - DELETE should return 200 or 204 on success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return NewDevPortalError(resp.StatusCode, fmt.Sprintf("API unpublishing failed: %s", string(respBody)), resp.StatusCode >= 500, nil)
	}

	log.Printf("[DevPortal] API unpublished successfully: %s", apiID)
	return nil
}