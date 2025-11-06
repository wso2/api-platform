/*
 *  Copyright (c) 2025, WSO2 LLC. (www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  www.apache.org/licenses/LICENSE-2.0
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
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"platform-api/src/internal/client"
	"platform-api/src/internal/client/devportal/dto"
)

// API Path Constants
const (
	// Base paths
	BasePath          = "/devportal"
	OrganizationsPath = BasePath + "/organizations"
	APIsPath          = BasePath + "/apis"

	// Path templates (use with fmt.Sprintf)
	OrganizationPath         = OrganizationsPath + "/%s"
	SubscriptionPoliciesPath = OrganizationPath + "/subscription-policies"
	OrganizationAPIsPath     = OrganizationPath + "/apis"
	OrganizationAPIPath      = OrganizationPath + "/apis/%s"
	APIPath                  = APIsPath + "/%s"

	// Default values
	DefaultHeaderKeyName = "x-wso2-api-key"

	// Configuration constants
	DefaultDevPortalTimeoutSeconds = 10
	DefaultMaxRetryAttempts        = 3
)

// DevPortalError represents an error from DevPortal operations
//
// This error type provides structured error information for intelligent
// retry logic and clear error messages to API consumers.
type DevPortalError struct {
	Code       int    // HTTP status code from DevPortal
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

// DevPortalOperationError represents a DevPortal operation error with enhanced context
//
// This error type provides detailed information about DevPortal operations
// for better error handling and debugging.
type DevPortalOperationError struct {
	Operation   string // Operation that failed (e.g., "CreateOrganization", "PublishAPI")
	DevPortalID string // DevPortal identifier
	APIID       string // API identifier (if applicable)
	OrgID       string // Organization identifier
	Message     string // Human-readable error message
	Underlying  error  // Underlying error
}

// Error implements the error interface for DevPortalOperationError
func (e *DevPortalOperationError) Error() string {
	if e.APIID != "" {
		return fmt.Sprintf("DevPortal operation '%s' failed for API %s on DevPortal %s (org %s): %s",
			e.Operation, e.APIID, e.DevPortalID, e.OrgID, e.Message)
	}
	return fmt.Sprintf("DevPortal operation '%s' failed on DevPortal %s (org %s): %s",
		e.Operation, e.DevPortalID, e.OrgID, e.Message)
}

// Unwrap returns the underlying error
func (e *DevPortalOperationError) Unwrap() error {
	return e.Underlying
}

// NewDevPortalOperationError creates a new DevPortalOperationError
func NewDevPortalOperationError(operation, devPortalID, apiID, orgID, message string, underlying error) *DevPortalOperationError {
	return &DevPortalOperationError{
		Operation:   operation,
		DevPortalID: devPortalID,
		APIID:       apiID,
		OrgID:       orgID,
		Message:     message,
		Underlying:  underlying,
	}
}

// DevPortalClient handles HTTP communication with the DevPortal
//
// This client provides methods for creating organizations, managing subscription policies,
// and publishing APIs to the DevPortal with automatic retry logic.
type DevPortalClient struct {
	httpClient    *client.RetryableHTTPClient // HTTP client with retry capabilities
	baseURL       string                      // DevPortal base URL (e.g., "172.17.0.1:3001")
	apiKey        string                      // Authentication API key
	headerKeyName string                      // Header name for API key (always uses header mode)
}

// requestConfig holds configuration for HTTP requests
type requestConfig struct {
	method      string
	url         string
	body        []byte
	contentType string
	headers     map[string]string
}

// responseConfig holds configuration for response handling
type responseConfig struct {
	expectedStatuses []int
	logOperation     string
}

// NewDevPortalClient creates a new DevPortal client with per-DevPortal configuration
//
// Parameters:
//   - baseURL: DevPortal API base URL
//   - apiKey: Authentication API key for this DevPortal
//   - headerKeyName: Header name for API key authentication
//   - timeoutSeconds: HTTP timeout in seconds
//
// Returns:
//   - *DevPortalClient: Configured client instance with DevPortal-specific settings
//
// The client initializes with:
//   - 3 retry attempts (per spec requirement)
//   - Configured timeout duration
//   - Header-based API key authentication
func NewDevPortalClient(baseURL, apiKey, headerKeyName string, timeoutSeconds int) *DevPortalClient {
	// Convert timeout from seconds to duration
	timeout := time.Duration(timeoutSeconds) * time.Second

	// Create HTTP client with retry logic (max 3 retries per spec)
	httpClient := client.NewRetryableHTTPClient(DefaultMaxRetryAttempts, timeout)

	// Default header name if not provided
	if headerKeyName == "" {
		headerKeyName = DefaultHeaderKeyName
	}

	return &DevPortalClient{
		httpClient:    httpClient,
		baseURL:       baseURL,
		apiKey:        apiKey,
		headerKeyName: headerKeyName,
	}
}

// buildURL constructs a full URL from the base URL and path
func (c *DevPortalClient) buildURL(path string) string {
	return fmt.Sprintf("%s%s", c.baseURL, path)
}

// executeRequest performs a generic HTTP request with common error handling and logging
func (c *DevPortalClient) executeRequest(config requestConfig, respConfig responseConfig) (*http.Response, []byte, error) {
	// Create HTTP request
	var body io.Reader
	if config.body != nil {
		body = bytes.NewBuffer(config.body)
	}

	httpReq, err := http.NewRequest(config.method, config.url, body)
	if err != nil {
		return nil, nil, NewDevPortalError(0, "failed to create HTTP request", false, err)
	}

	// Set content type if provided
	if config.contentType != "" {
		httpReq.Header.Set("Content-Type", config.contentType)
	} else if config.body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Set additional headers
	for key, value := range config.headers {
		httpReq.Header.Set(key, value)
	}

	// Add authentication
	c.addAuthentication(httpReq)

	// Execute request with retry logic
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, NewDevPortalError(0, fmt.Sprintf("failed to %s after retries", strings.ToLower(respConfig.logOperation)), true, err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, NewDevPortalError(resp.StatusCode, "failed to read response body", false, err)
	}

	// Check if status code is expected
	statusOK := false
	for _, expected := range respConfig.expectedStatuses {
		if resp.StatusCode == expected {
			statusOK = true
			break
		}
	}

	if !statusOK {
		return nil, nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("%s failed: %s", strings.ToLower(respConfig.logOperation), string(respBody)), resp.StatusCode >= 500, nil)
	}

	return resp, respBody, nil
}

// executeJSONRequest performs a JSON request and unmarshals the response
func (c *DevPortalClient) executeJSONRequest(config requestConfig, respConfig responseConfig, result interface{}) error {
	_, respBody, err := c.executeRequest(config, respConfig)
	if err != nil {
		return err
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return NewDevPortalError(0, "failed to unmarshal response", false, err)
		}
	}

	return nil
}

// executeExistenceCheck performs a GET request and returns existence based on status codes
func (c *DevPortalClient) executeExistenceCheck(url, operation string) (bool, error) {
	config := requestConfig{
		method: "GET",
		url:    url,
		headers: map[string]string{
			"Accept": "application/json",
		},
	}

	respConfig := responseConfig{
		expectedStatuses: []int{http.StatusOK, http.StatusNotFound},
		logOperation:     operation,
	}

	resp, _, err := c.executeRequest(config, respConfig)
	if err != nil {
		return false, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}

	// This shouldn't happen due to expectedStatuses, but just in case
	return false, NewDevPortalError(resp.StatusCode, fmt.Sprintf("%s check failed", strings.ToLower(operation)), resp.StatusCode >= 500, nil)
}

// addAuthentication adds API key authentication to the HTTP request
//
// This method always uses header-based authentication since API key transmission
// mode is always "header".
//
// Parameters:
//   - req: HTTP request to add authentication to
func (c *DevPortalClient) addAuthentication(req *http.Request) {
	req.Header.Set(c.headerKeyName, c.apiKey)
}

// CreateOrganization creates a new organization in the DevPortal
//
// This method first checks if the organization already exists. If it exists, it returns
// a response indicating the organization already exists. If not, it creates the organization.
//
// Parameters:
//   - req: Organization creation request with ID, Name, DisplayName, Description
//
// Returns:
//   - *dto.OrganizationCreateResponse: Response with created organization details
//   - error: DevPortalError if creation fails after retries
func (c *DevPortalClient) CreateOrganization(req *dto.OrganizationCreateRequest) (*dto.OrganizationCreateResponse, error) {
	// First check if organization already exists
	if trimmedOrgID := strings.TrimSpace(req.OrgID); trimmedOrgID != "" {
		exists, err := c.CheckOrganizationExists(trimmedOrgID)
		if err != nil {
			return nil, fmt.Errorf("failed to check organization existence: %w", err)
		}

		if exists {
			// Return a response indicating the organization already exists
			return &dto.OrganizationCreateResponse{
				OrgID:   trimmedOrgID,
				OrgName: req.OrgName,
			}, nil
		}
	}

	// Organization doesn't exist, proceed with creation
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to marshal organization request", false, err)
	}

	var orgResp dto.OrganizationCreateResponse
	err = c.executeJSONRequest(
		requestConfig{
			method:      "POST",
			url:         c.buildURL(OrganizationsPath),
			body:        body,
			contentType: "application/json",
		},
		responseConfig{
			expectedStatuses: []int{http.StatusCreated, http.StatusOK},
			logOperation:     fmt.Sprintf("Creating organization: %s (ID: %s)", req.OrgName, req.OrgID),
		},
		&orgResp,
	)
	if err != nil {
		return nil, err
	}

	return &orgResp, nil
}

// CheckOrganizationExists checks if an organization exists in the DevPortal
//
// This method queries the DevPortal to check if an organization with the given ID exists.
// It returns true if the organization exists (200 OK), false if not found (404), and an error
// for any other status codes or network issues.
//
// Parameters:
//   - orgID: Organization UUID to check
//
// Returns:
//   - bool: True if organization exists (200), false if not found (404)
//   - error: DevPortalError if check fails (non-404, non-200 responses)
func (c *DevPortalClient) CheckOrganizationExists(orgID string) (bool, error) {
	url := c.buildURL(fmt.Sprintf(OrganizationPath, orgID))
	return c.executeExistenceCheck(url, fmt.Sprintf("Checking if organization exists: %s", orgID))
}

// CreateSubscriptionPolicy creates a subscription policy for an organization in the api portal
//
// This method is used to create the default "unlimited" subscription policy for new organizations.
//
// Parameters:
//   - orgID: Organization UUID
//   - req: Subscription policy creation request
//
// Returns:
//   - *dto.SubscriptionPolicyCreateResponse: Response with created policy details
//   - error: ApiPortalError if creation fails after retries
func (c *DevPortalClient) CreateSubscriptionPolicy(orgID string, req *dto.SubscriptionPolicyCreateRequest) (*dto.SubscriptionPolicyCreateResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to marshal subscription policy request", false, err)
	}

	var policyResp dto.SubscriptionPolicyCreateResponse
	err = c.executeJSONRequest(
		requestConfig{
			method:      "POST",
			url:         c.buildURL(fmt.Sprintf(SubscriptionPoliciesPath, orgID)),
			body:        body,
			contentType: "application/json",
		},
		responseConfig{
			expectedStatuses: []int{http.StatusCreated, http.StatusOK},
			logOperation:     fmt.Sprintf("Creating subscription policy '%s' for organization: %s", req.PolicyName, orgID),
		},
		&policyResp,
	)
	if err != nil {
		return nil, err
	}

	return &policyResp, nil
}

// createMultipartRequest creates a multipart/form-data request with API metadata and definition
//
// This helper constructs the multipart request required by the api portal API publishing endpoint.
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

// PublishAPI publishes an API to the api portal
//
// This method creates a new API in the api portal with metadata and OpenAPI definition.
// It uses multipart/form-data to send both the API metadata (JSON) and the OpenAPI definition file.
//
// Parameters:
//   - orgID: Organization UUID
//   - req: API publish request with metadata
//   - apiDefinition: OpenAPI definition content (JSON bytes)
//
// Returns:
//   - *dto.APIPublishResponse: Response with created API details
//   - error: ApiPortalError if publishing fails after retries
func (c *DevPortalClient) PublishAPI(orgID string, req *dto.APIPublishRequest, apiDefinition []byte) (*dto.APIPublishResponse, error) {
	// Create multipart request body
	body, contentType, err := c.createMultipartRequest(req, apiDefinition)
	if err != nil {
		return nil, NewDevPortalError(0, "failed to create multipart request", false, err)
	}

	var apiResp dto.APIPublishResponse
	err = c.executeJSONRequest(
		requestConfig{
			method:      "POST",
			url:         c.buildURL(fmt.Sprintf(OrganizationAPIsPath, orgID)),
			body:        body.Bytes(),
			contentType: contentType,
		},
		responseConfig{
			expectedStatuses: []int{http.StatusCreated, http.StatusOK},
			logOperation:     fmt.Sprintf("Publishing API: %s (Organization: %s, ReferenceID: %s)", req.APIInfo.APIName, orgID, req.APIInfo.ReferenceID),
		},
		&apiResp,
	)
	if err != nil {
		return nil, err
	}

	return &apiResp, nil
}

// CheckAPIExists checks if an API exists in the api portal
//
// This method queries the api portal to check if an API with the given ID exists.
// It returns true if the API exists (200 OK), false if not found (404), and an error
// for any other status codes or network issues.
//
// Parameters:
//   - orgID: Organization UUID
//   - apiID: API UUID to check (platform-api API UUID used as referenceID in devportal)
//
// Returns:
//   - bool: True if API exists (200), false if not found (404)
//   - error: ApiPortalError if check fails (non-404, non-200 responses)
func (c *DevPortalClient) CheckAPIExists(orgID string, apiID string) (bool, error) {
	url := c.buildURL(fmt.Sprintf(OrganizationAPIPath, orgID, apiID)) + "?view=default"
	return c.executeExistenceCheck(url, fmt.Sprintf("Checking if API exists: %s (Organization: %s)", apiID, orgID))
}

// UnpublishAPI unpublishes an API from the api portal
//
// This method deletes an API from the api portal by its API ID.
// It uses retry logic to handle transient failures.
//
// Parameters:
//   - orgID: Organization UUID
//   - apiID: api portal API UUID (not platform-api API UUID)
//
// Returns:
//   - error: ApiPortalError if unpublishing fails after retries, nil on success
func (c *DevPortalClient) UnpublishAPI(orgID string, apiID string) error {
	return c.executeJSONRequest(
		requestConfig{
			method: "DELETE",
			url:    c.buildURL(fmt.Sprintf(APIPath, apiID)),
			headers: map[string]string{
				"Accept": "application/json",
			},
		},
		responseConfig{
			expectedStatuses: []int{http.StatusOK, http.StatusNoContent},
			logOperation:     fmt.Sprintf("Unpublishing API: %s (Organization: %s)", apiID, orgID),
		},
		nil, // No response body expected for DELETE
	)
}
