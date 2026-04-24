/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	management "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"gopkg.in/yaml.v3"
)

// TokenResponse represents the OAuth2 token response from on-prem APIM
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// GetAccessToken returns the access token
func (tr *TokenResponse) GetAccessToken() string {
	return tr.AccessToken
}

// GetExpiresIn returns the expires_in value
func (tr *TokenResponse) GetExpiresIn() int {
	return tr.ExpiresIn
}

// OnPremAPIMImportResponse represents the response from on-prem APIM API import
type OnPremAPIMImportResponse struct {
	ID       string `json:"id"`       // API ID (remains constant)
	Revision string `json:"revision"` // Revision ID (changes on updates)
}

// APIMHubPolicy represents a policy in APIM format
type APIMHubPolicy struct {
	PolicyName    string                 `json:"policyName" yaml:"policyName"`
	PolicyId      string                 `json:"policyId" yaml:"policyId"`
	PolicyVersion string                 `json:"policyVersion" yaml:"policyVersion"`
	PolicyType    interface{}            `json:"policyType" yaml:"policyType"`
	Parameters    map[string]interface{} `json:"parameters" yaml:"parameters"`
}

// APIMOperation represents an operation in APIM format
type APIMOperation struct {
	Id                        string                 `json:"id" yaml:"id"`
	Target                    string                 `json:"target" yaml:"target"`
	Verb                      string                 `json:"verb" yaml:"verb"`
	AuthType                  string                 `json:"authType" yaml:"authType"`
	ThrottlingPolicy          string                 `json:"throttlingPolicy" yaml:"throttlingPolicy"`
	Scopes                    []interface{}          `json:"scopes" yaml:"scopes"`
	UsedProductIds            []interface{}          `json:"usedProductIds" yaml:"usedProductIds"`
	AmznResourceName          interface{}            `json:"amznResourceName" yaml:"amznResourceName"`
	AmznResourceTimeout       interface{}            `json:"amznResourceTimeout" yaml:"amznResourceTimeout"`
	AmznResourceContentEncode interface{}            `json:"amznResourceContentEncode" yaml:"amznResourceContentEncode"`
	PayloadSchema             interface{}            `json:"payloadSchema" yaml:"payloadSchema"`
	UriMapping                interface{}            `json:"uriMapping" yaml:"uriMapping"`
	OperationPolicies         map[string]interface{} `json:"operationPolicies" yaml:"operationPolicies"`
	OperationHubPolicies      []APIMHubPolicy        `json:"operationHubPolicies" yaml:"operationHubPolicies"`
}

// APIMCompleteStructure represents the complete APIM API structure for import
type APIMCompleteStructure struct {
	Id             string          `json:"id" yaml:"id"`
	Name           string          `json:"name" yaml:"name"`
	DisplayName    string          `json:"displayName" yaml:"displayName"`
	ApiHubPolicies []APIMHubPolicy `json:"apiHubPolicies" yaml:"apiHubPolicies"`
	Operations     []APIMOperation `json:"operations" yaml:"operations"`
}

// APIMConfig holds the configuration for on-prem APIM operations
type APIMConfig struct {
	Host               string        // APIM control plane host (e.g., "localhost:9443")
	TokenURL           string        // OAuth2 token endpoint URL
	ClientID           string        // OAuth2 client ID (for client credentials flow)
	ClientSecret       string        // OAuth2 client secret (for client credentials flow)
	Username           string        // Username (for resource owner password flow)
	Password           string        // Password (for resource owner password flow)
	InsecureSkipVerify bool          // Skip TLS verification (insecure, dev/test only)
	Timeout            time.Duration // HTTP client timeout (default: 30 seconds)
	GatewayName        string        // Name of the gateway for deployment configuration
}

// APIMTokenService manages authentication for on-prem APIM operations
type APIMTokenService struct {
	config      *APIMConfig
	cachedToken string
	tokenExpiry time.Time
	mu          sync.Mutex
}

// NewAPIMTokenService creates a new APIM token service
func NewAPIMTokenService(config APIMConfig) APIMTokenService {
	return APIMTokenService{
		config: &config,
	}
}

// getAccessToken returns a valid access token (OAuth2 or Basic Auth), using cached token if not expired.
// Priority: clientID+clientSecret > Basic Auth (username+password)
func (s *APIMTokenService) getAccessToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return cached token if still valid
	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.cachedToken, nil
	}

	// Priority 1: Use OAuth2 if ClientID and ClientSecret are provided
	if s.config.ClientID != "" && s.config.ClientSecret != "" {
		token, expiresIn, err := s.generateOAuth2Token()
		if err != nil {
			return "", fmt.Errorf("OAuth2 token generation failed: %w", err)
		}

		// Cache the token with expiry (use 90% of expiry time as buffer)
		s.cachedToken = token
		if expiresIn > 0 {
			s.tokenExpiry = time.Now().Add(time.Duration(float64(expiresIn)*0.9) * time.Second)
		} else {
			s.tokenExpiry = time.Now().Add(1 * time.Hour) // Default 1 hour if no expiry provided
		}

		return token, nil
	}

	// Priority 2: Use basic auth with username/password if available
	if s.config.Username != "" && s.config.Password != "" {
		credentials := s.config.Username + ":" + s.config.Password
		basicAuth := "Basic " + encodeBase64(credentials)
		// Cache basic auth (no expiry, valid until changed)
		s.cachedToken = basicAuth
		s.tokenExpiry = time.Now().Add(1 * time.Hour) // Cache for 1 hour
		return basicAuth, nil
	}

	// No authentication method configured
	return "", fmt.Errorf("no authentication method configured: provide either clientID+clientSecret or username+password")
}

// generateOAuth2Token generates a new OAuth2 access token.
// Supports both client credentials (clientID + clientSecret) and
// resource owner password (username + password) flows.
// For on-prem APIM, includes scope=apim:api_import_export for API import operations.
func (s *APIMTokenService) generateOAuth2Token() (string, int, error) {
	var body string
	var authHeader string
	scopes := "apim:api_import_export apim:api_view"
	// Determine which OAuth2 flow to use
	if s.config.ClientID != "" && s.config.ClientSecret != "" {
		// Client Credentials Flow with scope for API import/export
		body = fmt.Sprintf("grant_type=client_credentials&scope=%s",
			url.QueryEscape(scopes))
		// Base64 encode clientID:clientSecret for Basic Auth
		credentials := s.config.ClientID + ":" + s.config.ClientSecret
		encoded := "Basic " + encodeBase64(credentials)
		authHeader = encoded
	} else if s.config.Username != "" && s.config.Password != "" {
		// Resource Owner Password Credentials Flow with scope for API import/export
		body = fmt.Sprintf("grant_type=password&username=%s&password=%s&scope=%s",
			url.QueryEscape(s.config.Username),
			url.QueryEscape(s.config.Password),
			url.QueryEscape(scopes))
	} else {
		return "", 0, fmt.Errorf("OAuth2 credentials not configured: provide either (clientID + clientSecret) or (username + password)")
	}

	// Create request
	req, err := http.NewRequest("POST", s.config.TokenURL, strings.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	// Create HTTP client with timeout and TLS configuration from config
	timeout := s.config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.config.InsecureSkipVerify,
			},
		},
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse token response: %w (response body: %s)", err, string(bodyBytes))
	}

	accessToken := tokenResp.GetAccessToken()
	if accessToken == "" {
		// Log the full response for debugging
		return "", 0, fmt.Errorf("access token not found in response: %s", string(bodyBytes))
	}

	return accessToken, tokenResp.GetExpiresIn(), nil
}

// ImportAPIToAPIMWithConfig imports a REST API to on-prem APIM with explicit configuration
// The zipFileBytes should contain the exported API definition as a zip file.
// cpHost is the control plane host (e.g., "localhost:9443")
// Returns ImportResponse with id and revision on success, error on failure (503 or other status codes).
func ImportAPIToAPIMWithConfig(apimConfig APIMConfig, logger *slog.Logger, apiZipName string, zipFileBytes *bytes.Buffer) (*OnPremAPIMImportResponse, error) {
	tokenService := NewAPIMTokenService(apimConfig)

	// Construct the import URL with standard query parameters
	importURL := "https://" + apimConfig.Host + "/api/am/publisher/v4/apis/import?preserveProvider=false&overwrite=true&dryRun=false&rotateRevision=true"

	logger.Info("Importing API to on-prem APIM",
		slog.String("url", importURL),
		slog.String("zip_name", apiZipName),
	)

	// Create a new multipart form
	body := &bytes.Buffer{}
	mpWriter := multipart.NewWriter(body)

	// Add the zip file to the multipart form with field name "file"
	part, err := mpWriter.CreateFormFile("file", apiZipName)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, zipFileBytes); err != nil {
		return nil, fmt.Errorf("failed to write zip file to form: %w", err)
	}

	// Close the multipart writer to finalize the form
	if err := mpWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create POST request
	req, err := http.NewRequest("POST", importURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create import request: %w", err)
	}

	// Get access token for authentication
	accessToken, err := tokenService.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	// Create HTTP client with TLS configuration and timeout from config
	timeout := apimConfig.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{ // #nosec G402 -- Explicit operator-controlled opt-out for dev/test environments.
				InsecureSkipVerify: apimConfig.InsecureSkipVerify,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send import request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode == http.StatusServiceUnavailable {
		logger.Error("On-prem APIM is currently unavailable",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)),
		)
		return nil, fmt.Errorf("on-prem APIM service unavailable (503)")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Error("API import to on-prem APIM failed",
			slog.String("zip_name", apiZipName),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)),
		)
		return nil, fmt.Errorf("API import failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the successful response to extract id and revision
	var importResp OnPremAPIMImportResponse
	if err := json.Unmarshal(bodyBytes, &importResp); err != nil {
		logger.Error("Failed to parse on-prem APIM import response",
			slog.String("zip_name", apiZipName),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to parse import response: %w", err)
	}

	logger.Info("Successfully imported API to on-prem APIM",
		slog.String("zip_name", apiZipName),
		slog.String("api_id", importResp.ID),
		slog.String("revision", importResp.Revision),
		slog.Int("status_code", resp.StatusCode),
	)

	return &importResp, nil
}

// Deprecated getAccessToken - kept for backward compatibility with APIUtilsService
// Use APIMTokenService instead for on-prem APIM operations
// Supports both client credentials and resource owner password credentials flows.
func (s *APIUtilsService) getAccessToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return cached token if still valid
	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.cachedToken, nil
	}

	// If no token URL configured, return error
	if s.TokenURL == "" {
		return "", fmt.Errorf("no token URL or static token configured")
	}

	// Generate new token using OAuth2
	token, expiresIn, err := s.generateOAuth2Token()
	if err != nil {
		return "", err
	}

	// Cache the token with expiry (use 90% of expiry time as buffer)
	s.cachedToken = token
	if expiresIn > 0 {
		s.tokenExpiry = time.Now().Add(time.Duration(float64(expiresIn)*0.9) * time.Second)
	} else {
		s.tokenExpiry = time.Now().Add(1 * time.Hour) // Default 1 hour if no expiry provided
	}

	return token, nil
}

// generateOAuth2Token generates a new OAuth2 access token.
// Supports both client credentials (clientID + clientSecret) and
// resource owner password (username + password) flows.
func (s *APIUtilsService) generateOAuth2Token() (string, int, error) {
	var body string
	var authHeader string

	// Determine which OAuth2 flow to use
	if s.ClientID != "" && s.ClientSecret != "" {
		// Client Credentials Flow
		body = "grant_type=client_credentials"
		// Base64 encode clientID:clientSecret for Basic Auth
		credentials := s.ClientID + ":" + s.ClientSecret
		encoded := "Basic " + encodeBase64(credentials)
		authHeader = encoded
	} else if s.Username != "" && s.Password != "" {
		// Resource Owner Password Credentials Flow
		body = fmt.Sprintf("grant_type=password&username=%s&password=%s",
			url.QueryEscape(s.Username),
			url.QueryEscape(s.Password))
	} else {
		return "", 0, fmt.Errorf("OAuth2 credentials not configured: provide either (clientID + clientSecret) or (username + password)")
	}

	// Create request
	req, err := http.NewRequest("POST", s.TokenURL, strings.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse token response: %w (response body: %s)", err, string(bodyBytes))
	}

	accessToken := tokenResp.GetAccessToken()
	if accessToken == "" {
		// Log the full response for debugging
		return "", 0, fmt.Errorf("access token not found in response: %s", string(bodyBytes))
	}

	return accessToken, tokenResp.GetExpiresIn(), nil
}

// encodeBase64 encodes a string to base64
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// ImportAPIToAPIM imports a REST API to on-prem APIM via the publisher's /apis/import endpoint.
// The zipFileBytes should contain the exported API definition as a zip file.
// queryParams include: preserveProvider=false, overwrite=true/false, dryRun=false, rotateRevision=true
// Returns ImportResponse with id and revision on success, error on failure (503 or other status codes).
func (s *APIUtilsService) ImportAPIToAPIM(apiZipName string, zipFileBytes *bytes.Buffer, queryParams string) (*OnPremAPIMImportResponse, error) {
	// Construct the import URL with query parameters
	importURL := s.getBaseURL() + "/apis/import"
	if queryParams != "" {
		importURL += "?" + queryParams
	}

	s.logger.Info("Importing API to on-prem APIM", slog.String("url", importURL), slog.String("zip_name", apiZipName))

	// Create a new multipart form
	body := &bytes.Buffer{}
	mpWriter := multipart.NewWriter(body)

	// Add the zip file to the multipart form with field name "file"
	part, err := mpWriter.CreateFormFile("file", apiZipName)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, zipFileBytes); err != nil {
		return nil, fmt.Errorf("failed to write zip file to form: %w", err)
	}

	// Close the multipart writer to finalize the form
	if err := mpWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create POST request
	req, err := http.NewRequest("POST", importURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create import request: %w", err)
	}

	// Get access token for authentication
	accessToken, err := s.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	// Make the request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send import request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode == http.StatusServiceUnavailable {
		s.logger.Error("On-prem APIM is currently unavailable",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)),
		)
		return nil, fmt.Errorf("on-prem APIM service unavailable (503)")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		s.logger.Error("API import to on-prem APIM failed",
			slog.String("zip_name", apiZipName),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)),
		)
		return nil, fmt.Errorf("API import failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the successful response to extract id and revision
	var importResp OnPremAPIMImportResponse
	if err := json.Unmarshal(bodyBytes, &importResp); err != nil {
		s.logger.Error("Failed to parse on-prem APIM import response",
			slog.String("zip_name", apiZipName),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to parse import response: %w", err)
	}

	s.logger.Info("Successfully imported API to on-prem APIM",
		slog.String("zip_name", apiZipName),
		slog.String("api_id", importResp.ID),
		slog.String("revision", importResp.Revision),
		slog.Int("status_code", resp.StatusCode),
	)

	return &importResp, nil
}

// ZipFile represents onprem APIM's import API operation's request payload
type ZipFile struct {
	Path    string
	Content string
}

// UndeployRevisionFromAPIM undeploys a specific API revision from a gateway in on-prem APIM.
// Calls POST /api/am/publisher/v4/apis/{apiId}/undeploy-revision?revisionId={revisionId}
// with the gateway name as the deployment environment.
func UndeployRevisionFromAPIM(apimConfig APIMConfig, apiID string, revisionID string, logger *slog.Logger) error {
	tokenService := NewAPIMTokenService(apimConfig)

	token, err := tokenService.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	undeployURL := fmt.Sprintf("https://%s/api/am/publisher/v4/apis/%s/undeploy-revision?revisionId=%s",
		apimConfig.Host, apiID, revisionID)
	logger.Info("Undeploying API revision from APIM", slog.String("url", undeployURL))

	payload := []map[string]interface{}{
		{
			"name":               apimConfig.GatewayName,
			"displayOnDevportal": false,
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal undeploy payload: %w", err)
	}

	req, err := http.NewRequest("POST", undeployURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create undeploy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	timeout := apimConfig.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: apimConfig.InsecureSkipVerify,
			},
		},
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send undeploy request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("undeploy-revision returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// FetchSwaggerFromAPIM fetches the OpenAPI/Swagger definition of an existing API from APIM.
// Used during bottom-up sync updates to retrieve the current swagger instead of generating it locally.
func FetchSwaggerFromAPIM(apimConfig APIMConfig, apiID string, logger *slog.Logger) (string, error) {
	tokenService := NewAPIMTokenService(apimConfig)

	token, err := tokenService.getAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	swaggerURL := fmt.Sprintf("https://%s/api/am/publisher/v4/apis/%s/swagger", apimConfig.Host, apiID)
	logger.Info("Fetching swagger from APIM", slog.String("url", swaggerURL))

	req, err := http.NewRequest("GET", swaggerURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create swagger request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	timeout := apimConfig.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: apimConfig.InsecureSkipVerify,
			},
		},
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send swagger request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("swagger endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read swagger response: %w", err)
	}
	logger.Info("Successfully fetched swagger from APIM", slog.String("url",string(bodyBytes)), slog.Int("response_size", len(bodyBytes)))
	return string(bodyBytes), nil
}

// ExportAPIAsZip exports a StoredConfig (REST API) as a zip file suitable for APIM import.
// The zip structure matches APIM format:
//
//	{ApiName-Version}/
//	├── api.yaml (APIM metadata + definition)
//	├── deployment_environments.yaml (deployment config)
//	└── Definitions/swagger.yaml
//
// swaggerOverride, if non-empty, is used as the OpenAPI definition instead of generating it locally.
// Returns a bytes.Buffer containing the zip file.
func ExportAPIAsZip(api *models.StoredConfig, gatewayName string, swaggerOverride string) (*bytes.Buffer, error) {
	// Extract API metadata from configuration
	apiName, apiVersion, err := extractAPIMetadata(api.Configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to extract API metadata: %w", err)
	}

	// Generate the APIM-formatted api.yaml
	apiYaml := generateAPIYaml(api, apiName, apiVersion)

	// Generate deployment_environments.yaml
	deploymentYaml := generateDeploymentEnvironmentsYaml(gatewayName)

	// Use fetched swagger for updates, generate locally for new APIs
	var openAPIDefinition string
	if swaggerOverride != "" {
		openAPIDefinition = swaggerOverride
	} else {
		openAPIDefinition = extractOpenAPIDefinition(api.Configuration, apiName, apiVersion)
	}

	// Build zip files list using APIM structure
	zipFiles := []ZipFile{
		{
			Path:    fmt.Sprintf("%s-%s/api.yaml", apiName, apiVersion),
			Content: apiYaml,
		},
		{
			Path:    fmt.Sprintf("%s-%s/deployment_environments.yaml", apiName, apiVersion),
			Content: deploymentYaml,
		},
		{
			Path:    fmt.Sprintf("%s-%s/Definitions/swagger.yaml", apiName, apiVersion),
			Content: openAPIDefinition,
		},
	}

	// Create the zip file
	buf := &bytes.Buffer{}
	if err := createZipFile(buf, zipFiles); err != nil {
		return nil, fmt.Errorf("failed to create zip file for %s-%s: %w", apiName, apiVersion, err)
	}

	return buf, nil
}

// createZipFile creates a zip archive from a list of ZipFile entries
// Follows the same pattern as APK agent (pkg/utils/zip_utils.go)
func createZipFile(writer *bytes.Buffer, zipFiles []ZipFile) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	for _, zipFile := range zipFiles {
		fileWriter, err := zipWriter.Create(zipFile.Path)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", zipFile.Path, err)
		}

		if _, err := fileWriter.Write([]byte(zipFile.Content)); err != nil {
			return fmt.Errorf("failed to write content to zip entry %s: %w", zipFile.Path, err)
		}
	}

	return nil
}

// validateStringField validates that a field in the config map is a non-empty string
func validateStringField(configMap map[string]interface{}, fieldName string) (string, error) {
	value, ok := configMap[fieldName].(string)
	if !ok {
		return "", fmt.Errorf("API %s is missing or not a string in configuration", fieldName)
	}
	if value == "" {
		return "", fmt.Errorf("API %s cannot be empty", fieldName)
	}
	return value, nil
}

// extractAPIMetadata extracts API name and version from configuration
func extractAPIMetadata(config interface{}) (string, string, error) {
	// Handle map[string]interface{} type
	if configMap, ok := config.(map[string]interface{}); ok {
		name, err := validateStringField(configMap, "name")
		if err != nil {
			return "", "", err
		}

		version, err := validateStringField(configMap, "version")
		if err != nil {
			return "", "", err
		}

		return name, version, nil
	}
	if restAPI, ok := config.(api.RestAPI); ok {
		return restAPI.Spec.DisplayName, restAPI.Spec.Version, nil
	}
	return "", "", fmt.Errorf("configuration is not a map or RestAPI struct: got %T", config)
}

// extractUpstreamURL extracts the upstream URL from API configuration.
// Handles both map-based and RestAPI struct-based configurations.
func extractUpstreamURL(config interface{}) string {
	// Handle map-based configuration
	if configMap, ok := config.(map[string]interface{}); ok {
		if upstream, exists := configMap["upstream"].(map[string]interface{}); exists {
			if main, exists := upstream["main"].(map[string]interface{}); exists {
				if url, exists := main["url"].(string); exists && url != "" {
					return url
				}
			}
		}
	}

	// Handle RestAPI value type
	if restAPI, ok := config.(management.RestAPI); ok && restAPI.Spec.Upstream.Main.Url != nil && *restAPI.Spec.Upstream.Main.Url != "" {
		return *restAPI.Spec.Upstream.Main.Url
	}

	// Handle RestAPI pointer configuration
	if restAPI, ok := config.(*management.RestAPI); ok && restAPI != nil && restAPI.Spec.Upstream.Main.Url != nil && *restAPI.Spec.Upstream.Main.Url != "" {
		return *restAPI.Spec.Upstream.Main.Url
	}
	// ========= check correct one above and remove this if not needed =========

	return ""
}

// convertPolicyVersion converts policy version format (v1 → 1.0)
func convertPolicyVersion(version string) string {
	if version == "" {
		return "1.0"
	}

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// If no dot, append .0
	if !strings.Contains(version, ".") {
		version = version + ".0"
	}

	return version
}

// convertAPILevelPolicies converts RestAPI spec policies to APIM apiHubPolicies
func convertAPILevelPolicies(policies *[]api.Policy) []APIMHubPolicy {
	if policies == nil || len(*policies) == 0 {
		return []APIMHubPolicy{}
	}

	var apimPolicies []APIMHubPolicy

	for _, policy := range *policies {
		convertedVersion := convertPolicyVersion(policy.Version)
		apimPolicy := APIMHubPolicy{
			PolicyName:    policy.Name,
			PolicyVersion: convertedVersion,
			PolicyId:      generateRandomPolicyUUID(policy.Name, convertedVersion),
			PolicyType:    nil,
			Parameters:    convertPolicyParams(policy.Params),
		}
		apimPolicies = append(apimPolicies, apimPolicy)
	}

	return apimPolicies
}

// convertOperationPolicies converts operation policies to APIM operationHubPolicies
func convertOperationPolicies(policies *[]api.Policy) []APIMHubPolicy {
	if policies == nil || len(*policies) == 0 {
		return []APIMHubPolicy{}
	}

	var apimPolicies []APIMHubPolicy

	for _, policy := range *policies {
		convertedVersion := convertPolicyVersion(policy.Version)
		apimPolicy := APIMHubPolicy{
			PolicyName:    policy.Name,
			PolicyVersion: convertedVersion,
			PolicyId:      generateRandomPolicyUUID(policy.Name, convertedVersion),
			PolicyType:    nil,
			Parameters:    convertPolicyParams(policy.Params),
		}
		apimPolicies = append(apimPolicies, apimPolicy)
	}

	return apimPolicies
}

// convertPolicyParams converts policy parameters to APIM format
func convertPolicyParams(params *map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{}
	}

	// If params is a pointer to a map, dereference it and return
	return *params
}

// generateRandomPolicyUUID generates a deterministic UUID for a policy
// Same policy name+version always produces the same UUID (idempotent)
// Different policies get different UUIDs
func generateRandomPolicyUUID(policyName string, policyVersion string) string {
	// Create a deterministic UUID v5 based on policy name and version
	// Using DNS namespace for consistency
	namespace := uuid.NameSpaceDNS
	data := fmt.Sprintf("apim:policy:%s:%s", policyName, policyVersion)
	return uuid.NewSHA1(namespace, []byte(data)).String()
}

// buildAdditionalProperties builds the additionalProperties array for the APIM api.yaml.
func buildAdditionalProperties(deploymentID string) []interface{} {
	if deploymentID == "" {
		return []interface{}{}
	}
	return []interface{}{
		map[string]interface{}{
			"name":    "deployment_id",
			"value":   deploymentID,
			"display": false,
		},
	}
}

// generateAPIYaml generates APIM-formatted api.yaml content
func generateAPIYaml(api *models.StoredConfig, apiName, apiVersion string) string {
	contextValue, err := api.GetContext()
	if err != nil {
		contextValue = ""
	}

	// Extract upstream URL from API configuration
	upstreamURL := extractUpstreamURL(api.Configuration)
	if upstreamURL == "" {
		upstreamURL = "http://localhost:8080" // Default fallback
	}

	// Build endpoint configuration
	endpointConfig := map[string]interface{}{
		"endpoint_type": "http",
		"production_endpoints": map[string]interface{}{
			"url": upstreamURL,
		},
		"sandbox_endpoints": map[string]interface{}{
			"url": upstreamURL,
		},
	}

	// Extract and convert API-level policies
	var apiHubPolicies []APIMHubPolicy
	if restAPIVal, ok := api.Configuration.(management.RestAPI); ok {
		if restAPIVal.Spec.Policies != nil {
			apiHubPolicies = convertAPILevelPolicies(restAPIVal.Spec.Policies)
		}
	} else if restAPIPtr, ok := api.Configuration.(*management.RestAPI); ok {
		if restAPIPtr != nil && restAPIPtr.Spec.Policies != nil {
			apiHubPolicies = convertAPILevelPolicies(restAPIPtr.Spec.Policies)
		}
	}

	// Build APIM-compatible api.yaml structure
	apiData := map[string]interface{}{
		"type":    "api",
		"version": "v4.7.0",
		"data": map[string]interface{}{
			"name":                       apiName,
			"version":                    apiVersion,
			"context":                    contextValue,
			"type":                       "HTTP",
			"transport":                  []string{"http", "https"},
			"provider":                   "admin",
			"tags":                       []string{},
			"policies":                   []string{"Unlimited"},
			"securityScheme":             []string{"oauth_basic_auth_api_key_mandatory"},
			"visibility":                 "PUBLIC",
			"visibleRoles":               []string{},
			"visibleTenants":             []string{},
			"visibleOrganizations":       []string{"none"},
			"mediationPolicies":          []interface{}{},
			"apiHubPolicies":             apiHubPolicies,
			"responseCachingEnabled":     false,
			"cacheTimeout":               300,
			"hasThumbnail":               false,
			"isDefaultVersion":           false,
			"isRevision":                 false,
			"revisionId":                 0,
			"enableSchemaValidation":     false,
			"additionalProperties":       buildAdditionalProperties(api.DeploymentID),
			"additionalPropertiesMap":    map[string]interface{}{},
			"gatewayType":                "APIPlatform",
			"gatewayVendor":              "wso2",
			"endpointConfig":             endpointConfig,
			"endpointImplementationType": "ENDPOINT",
			"initiatedFromGateway":       true,
			"operations":                 buildOperationsWithPolicies(api.Configuration),
		},
	}

	// Merge any additional metadata from the stored config
	if configMap, ok := api.Configuration.(map[string]interface{}); ok {
		if dataMap, ok := apiData["data"].(map[string]interface{}); ok {
			// Copy relevant fields from stored config to apiData
			if context, exists := configMap["context"]; exists {
				dataMap["context"] = context
			}
			if basePath, exists := configMap["basePath"]; exists {
				dataMap["basePath"] = basePath
			}
			if description, exists := configMap["description"]; exists {
				dataMap["description"] = description
			}
		}
	}
	

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(apiData)
	if err != nil {
		return ""
	}

	return string(yamlBytes)
}

// buildOperationsWithPolicies builds APIM operations with policies from RestAPI spec
func buildOperationsWithPolicies(config interface{}) []map[string]interface{} {
	var operations []map[string]interface{}

	// Handle RestAPI value type
	if restAPIVal, ok := config.(management.RestAPI); ok && restAPIVal.Spec.Operations != nil {
		for _, op := range restAPIVal.Spec.Operations {
			operation := buildAPIMOperation(op)
			operations = append(operations, operation)
		}
		return operations
	}

	// Handle RestAPI pointer type
	if restAPIPtr, ok := config.(*management.RestAPI); ok && restAPIPtr != nil && restAPIPtr.Spec.Operations != nil {
		for _, op := range restAPIPtr.Spec.Operations {
			operation := buildAPIMOperation(op)
			operations = append(operations, operation)
		}
		return operations
	}

	return operations
}

// buildAPIMOperation builds a single APIM operation with policies
func buildAPIMOperation(op api.Operation) map[string]interface{} {
	// Convert operation policies
	operationHubPolicies := convertOperationPolicies(op.Policies)

	return map[string]interface{}{
		"id":                        "",
		"target":                    op.Path, // Use operation path
		"verb":                      strings.ToUpper(string(op.Method)),
		"authType":                  "Application & Application User",
		"throttlingPolicy":          "Unlimited",
		"scopes":                    []interface{}{},
		"usedProductIds":            []interface{}{},
		"amznResourceName":          nil,
		"amznResourceTimeout":       nil,
		"amznResourceContentEncode": nil,
		"payloadSchema":             nil,
		"uriMapping":                nil,
		"operationPolicies": map[string]interface{}{
			"request":  []interface{}{},
			"response": []interface{}{},
			"fault":    []interface{}{},
		},
		"operationHubPolicies": operationHubPolicies,
	}
}

// generateDeploymentEnvironmentsYaml generates deployment_environments.yaml content
func generateDeploymentEnvironmentsYaml(gatewayName string) string {
	deploymentData := map[string]interface{}{
		"type":    "deployment_environments",
		"version": "v4.3.0",
		"data": []map[string]interface{}{
			{
				"deploymentEnvironment": gatewayName,
				"displayOnDevportal":    true,
			},
		},
	}

	yamlBytes, err := yaml.Marshal(deploymentData)
	if err != nil {
		return ""
	}

	return string(yamlBytes)
}

// extractOpenAPIDefinition extracts OpenAPI/Swagger definition from configuration.
// Generates paths from the operations in the spec using provided apiName and apiVersion.
func extractOpenAPIDefinition(config interface{}, apiName, apiVersion string) string {
	// Handle map-based configuration
	if configMap, ok := config.(map[string]interface{}); ok {
		paths := buildOpenAPIPaths(configMap)
		return createMinimalOpenAPI(apiName, apiVersion, paths)
	}

	// Handle RestAPI struct configuration
	if restAPI, ok := config.(management.RestAPI); ok && restAPI.Spec.Operations != nil {
		paths := buildOpenAPIPathsFromRestAPI(restAPI.Spec.Operations)
		return createMinimalOpenAPI(apiName, apiVersion, paths)
	}

	// Handle RestAPI pointer configuration
	if restAPI, ok := config.(*management.RestAPI); ok && restAPI != nil && restAPI.Spec.Operations != nil {
		paths := buildOpenAPIPathsFromRestAPI(restAPI.Spec.Operations)
		return createMinimalOpenAPI(apiName, apiVersion, paths)
	}

	return createMinimalOpenAPI(apiName, apiVersion, nil)
}

// buildOpenAPIPaths builds OpenAPI path items from the operations in the configuration.
func buildOpenAPIPaths(configMap map[string]interface{}) map[string]interface{} {
	paths := make(map[string]interface{})
	if operations, exists := configMap["operations"].([]interface{}); exists {
		for _, op := range operations {
			if opMap, ok := op.(map[string]interface{}); ok {
				path, pathOk := opMap["path"].(string)
				method, methodOk := opMap["method"].(string)

				if pathOk && methodOk && path != "" && method != "" {
					if paths[path] == nil {
						paths[path] = make(map[string]interface{})
					}
					pathItem := paths[path].(map[string]interface{})
					methodLower := strings.ToLower(method)
					pathItem[methodLower] = map[string]interface{}{
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Successful response",
							},
						},
					}
				}
			}
		}
	}
	return paths
}

// buildOpenAPIPathsFromRestAPI builds OpenAPI path items from RestAPI operations.
func buildOpenAPIPathsFromRestAPI(operations []api.Operation) map[string]interface{} {
	paths := make(map[string]interface{})

	for _, op := range operations {
		path := op.Path
		method := string(op.Method)

		if path != "" && method != "" {
			if paths[path] == nil {
				paths[path] = make(map[string]interface{})
			}
			pathItem := paths[path].(map[string]interface{})
			methodLower := strings.ToLower(method)

			// Extract path parameters from the path (e.g., {petId} from /pet/{petId})
			parameters := extractPathParameters(path)

			operation := map[string]interface{}{
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Successful response",
					},
				},
			}

			// Add parameters to operation if any exist
			if len(parameters) > 0 {
				operation["parameters"] = parameters
			}

			pathItem[methodLower] = operation
		}
	}
	return paths
}

// extractPathParameters extracts parameter definitions from a path string.
// For example: /pet/{petId} returns [{name: "petId", in: "path", required: true, schema: {type: "string"}}]
func extractPathParameters(path string) []map[string]interface{} {
	var parameters []map[string]interface{}

	// Find all {paramName} patterns in the path
	start := 0
	for {
		openIdx := strings.Index(path[start:], "{")
		if openIdx == -1 {
			break
		}
		openIdx += start

		closeIdx := strings.Index(path[openIdx:], "}")
		if closeIdx == -1 {
			break
		}
		closeIdx += openIdx

		paramName := path[openIdx+1 : closeIdx]
		if paramName != "" {
			parameters = append(parameters, map[string]interface{}{
				"name":     paramName,
				"in":       "path",
				"required": true,
				"schema": map[string]interface{}{
					"type": "string",
				},
			})
		}

		start = closeIdx + 1
	}

	return parameters
}

// createMinimalOpenAPI creates a minimal OpenAPI 3.0.0 definition with the provided paths
func createMinimalOpenAPI(apiName, apiVersion string, paths map[string]interface{}) string {
	if paths == nil {
		paths = make(map[string]interface{})
	}

	openAPISpec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   apiName,
			"version": apiVersion,
		},
		"paths": paths,
	}

	yamlBytes, err := yaml.Marshal(openAPISpec)
	if err != nil {
		return "openapi: 3.0.0\ninfo:\n  title: " + apiName + "\n  version: " + apiVersion + "\npaths: {}"
	}

	return string(yamlBytes)
}
