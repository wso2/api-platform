/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// PlatformAPIConfig contains configuration for fetching API definitions
type PlatformAPIConfig struct {
	BaseURL            string        // Base URL for API requests
	Token              string        // Authentication token
	InsecureSkipVerify bool          // Skip TLS verification
	Timeout            time.Duration // Request timeout
}

// APIUtilsService provides utilities for API operations
type APIUtilsService struct {
	mu     sync.RWMutex
	config PlatformAPIConfig
	logger *slog.Logger
	client *http.Client
}

// NewAPIUtilsService creates a new API utilities service
func NewAPIUtilsService(config PlatformAPIConfig, logger *slog.Logger) *APIUtilsService {
	// Set default timeout if not provided
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
			MinVersion:         tls.VersionTLS12,
		},
		// Connection pool tuning
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     10,
		IdleConnTimeout:     30 * time.Second,
	}

	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &APIUtilsService{
		config: config,
		logger: logger,
		client: client,
	}
}

// SetBaseURL updates the base URL used for API requests.
// This is used to update the URL after gateway path discovery.
func (s *APIUtilsService) SetBaseURL(baseURL string) {
	s.mu.Lock()
	s.config.BaseURL = baseURL
	s.mu.Unlock()
	s.logger.Debug("Updated API utils service base URL",
		slog.String("base_url", baseURL),
	)
}

// getBaseURL returns the current base URL in a thread-safe manner.
func (s *APIUtilsService) getBaseURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.BaseURL
}

// FetchAPIDefinition downloads the API definition as a zip file from the control plane
func (s *APIUtilsService) FetchAPIDefinition(apiID string) ([]byte, error) {
	// Construct the API URL by appending the resource path
	apiURL := s.getBaseURL() + "/apis/" + apiID

	s.logger.Info("Fetching API definition",
		slog.String("api_id", apiID),
		slog.String("url", apiURL),
	)

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/zip")

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API definition: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Info("Successfully fetched API definition",
		slog.String("api_id", apiID),
		slog.Int("size_bytes", len(bodyBytes)),
	)

	return bodyBytes, nil
}

// FetchLLMProviderDefinition downloads the LLM provider definition as a zip file from the control plane
func (s *APIUtilsService) FetchLLMProviderDefinition(providerID string) ([]byte, error) {
	// Construct the LLM provider URL by appending the resource path
	providerURL := s.getBaseURL() + "/llm-providers/" + providerID

	s.logger.Info("Fetching LLM provider definition",
		slog.String("provider_id", providerID),
		slog.String("url", providerURL),
	)

	// Create request
	req, err := http.NewRequest("GET", providerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/zip")

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch LLM provider definition: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM provider request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Info("Successfully fetched LLM provider definition",
		slog.String("provider_id", providerID),
		slog.Int("size_bytes", len(bodyBytes)),
	)

	return bodyBytes, nil
}

// FetchLLMProxyDefinition downloads the LLM proxy definition as a zip file from the control plane
func (s *APIUtilsService) FetchLLMProxyDefinition(proxyID string) ([]byte, error) {
	// Construct the LLM proxy URL by appending the resource path
	proxyURL := s.getBaseURL() + "/llm-proxies/" + proxyID

	s.logger.Info("Fetching LLM proxy definition",
		slog.String("proxy_id", proxyID),
		slog.String("url", proxyURL),
	)

	// Create request
	req, err := http.NewRequest("GET", proxyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/zip")

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch LLM proxy definition: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM proxy request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Info("Successfully fetched LLM proxy definition",
		slog.String("proxy_id", proxyID),
		slog.Int("size_bytes", len(bodyBytes)),
	)

	return bodyBytes, nil
}

// FetchSubscriptionsForAPI fetches subscriptions for the given API from the control plane.
func (s *APIUtilsService) FetchSubscriptionsForAPI(apiID string) ([]models.Subscription, error) {
	subURL := s.getBaseURL() + "/apis/" + apiID + "/subscriptions"

	s.logger.Info("Fetching subscriptions for API",
		slog.String("api_id", apiID),
		slog.String("url", subURL),
	)

	client := &http.Client{
		Timeout: s.config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.config.InsecureSkipVerify,
			},
		},
	}

	req, err := http.NewRequest("GET", subURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriptions request: %w", err)
	}
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscriptions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("subscriptions request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var subs []models.Subscription
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return nil, fmt.Errorf("failed to decode subscriptions response: %w", err)
	}

	s.logger.Info("Successfully fetched subscriptions for API",
		slog.String("api_id", apiID),
		slog.Int("count", len(subs)),
	)

	return subs, nil
}

// controlPlaneAPIKey is the API key response from the control plane REST API.
// The APIKeyHashes field holds a map of hash algorithm → hash value (e.g. {"sha256": "abc123..."}).
type controlPlaneAPIKey struct {
	CorrelationID string            `json:"correlationId"`
	UUID         string            `json:"uuid"`
	Name         string            `json:"name"`
	MaskedAPIKey string            `json:"maskedApiKey"`
	APIKeyHashes map[string]string `json:"apiKeyHashes"`
	ArtifactUUID string            `json:"artifactUuid"`
	Status       string            `json:"status"`
	CreatedAt    time.Time         `json:"createdAt"`
	CreatedBy    string            `json:"createdBy"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	ExpiresAt    *time.Time        `json:"expiresAt"`
	Source       string            `json:"source"`
	ExternalRefId *string          `json:"externalRefId"`
	Issuer       *string           `json:"issuer,omitempty"`
}

// FetchAPIKeysByKind fetches all API keys for the given artifact kind from the control plane.
// Supported kinds: KindLlmProvider, KindLlmProxy, KindRestApi.
// When issuer is non-empty it is appended as a query parameter so the server returns
// only keys matching that issuer; an empty issuer fetches all keys for the kind.
// Only active keys that carry a sha256 hash are returned; others are skipped.
func (s *APIUtilsService) FetchAPIKeysByKind(artifactKind, issuer string) ([]models.APIKey, error) {
	baseURL := s.getBaseURL()
	var path string
	switch artifactKind {
	case models.KindLlmProvider:
		path = "/llm-providers/api-keys"
	case models.KindLlmProxy:
		path = "/llm-proxies/api-keys"
	case models.KindRestApi:
		path = "/apis/api-keys"
	default:
		return nil, fmt.Errorf("unsupported artifact kind for API key fetch: %s", artifactKind)
	}

	endpoint := baseURL + path
	if issuer != "" {
		endpoint += "?issuer=" + issuer
	}

	s.logger.Info("Fetching API keys by kind",
		slog.String("kind", artifactKind),
		slog.Bool("issuer_filtered", issuer != ""),
	)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create API keys request: %w", err)
	}
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API keys request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var cpKeys []controlPlaneAPIKey
	if err := json.NewDecoder(resp.Body).Decode(&cpKeys); err != nil {
		return nil, fmt.Errorf("failed to decode API keys response: %w", err)
	}

	keys := make([]models.APIKey, 0, len(cpKeys))
	for _, ck := range cpKeys {
		if models.APIKeyStatus(ck.Status) != models.APIKeyStatusActive {
			s.logger.Debug("Skipping non-active API key during bulk sync",
				slog.String("kind", artifactKind),
				slog.String("key_name", ck.Name),
				slog.String("status", ck.Status),
			)
			continue
		}
		sha256Hash, ok := ck.APIKeyHashes[constants.HashingAlgorithmSHA256]
		if !ok || sha256Hash == "" {
			s.logger.Warn("Skipping API key without sha256 hash during bulk sync",
				slog.String("kind", artifactKind),
				slog.String("key_name", ck.Name),
			)
			continue
		}
		keys = append(keys, models.APIKey{
			UUID:          ck.UUID,
			Name:          ck.Name,
			APIKey:        sha256Hash,
			MaskedAPIKey:  ck.MaskedAPIKey,
			ArtifactUUID:  ck.ArtifactUUID,
			Status:        models.APIKeyStatus(ck.Status),
			CreatedAt:     ck.CreatedAt,
			CreatedBy:     ck.CreatedBy,
			UpdatedAt:     ck.UpdatedAt,
			ExpiresAt:     ck.ExpiresAt,
			Source:        ck.Source,
			ExternalRefId: ck.ExternalRefId,
			Issuer:        ck.Issuer,
			CorrelationID: ck.CorrelationID,
		})
	}

	s.logger.Info("Successfully fetched API keys by kind",
		slog.String("kind", artifactKind),
		slog.Int("count", len(keys)),
	)

	return keys, nil
}

// FetchSubscriptionPlans fetches all subscription plans from the control plane for the organization.
func (s *APIUtilsService) FetchSubscriptionPlans() ([]models.SubscriptionPlan, error) {
	planURL := s.getBaseURL() + "/subscription-plans"

	s.logger.Info("Fetching subscription plans", slog.String("url", planURL))

	client := &http.Client{
		Timeout: s.config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.config.InsecureSkipVerify,
			},
		},
	}

	req, err := http.NewRequest("GET", planURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription plans request: %w", err)
	}
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription plans: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("subscription plans request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var plans []models.SubscriptionPlan
	if err := json.NewDecoder(resp.Body).Decode(&plans); err != nil {
		return nil, fmt.Errorf("failed to decode subscription plans response: %w", err)
	}

	s.logger.Info("Successfully fetched subscription plans", slog.Int("count", len(plans)))

	return plans, nil
}

// ExtractYAMLFromZip extracts the API definition YAML from the zip file
func (s *APIUtilsService) ExtractYAMLFromZip(zipData []byte) ([]byte, error) {
	// Create a reader from the zip data
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Look for YAML files in the zip
	for _, file := range zipReader.File {
		// Check for common API definition file names
		if filepath.Ext(file.Name) == ".yaml" || filepath.Ext(file.Name) == ".yml" {
			s.logger.Info("Found YAML file in zip",
				slog.String("filename", file.Name),
			)

			// Open the file
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file %s: %w", file.Name, err)
			}
			defer rc.Close()

			// Read the content
			yamlData, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", file.Name, err)
			}

			return yamlData, nil
		}
	}

	return nil, fmt.Errorf("no YAML file found in zip archive")
}

// CreateAPIFromYAML creates an API configuration from YAML data using the deployment service
func (s *APIUtilsService) CreateAPIFromYAML(yamlData []byte, apiID string, deploymentID string,
	deployedAt *time.Time, correlationID string,
	deploymentService *APIDeploymentService) (*APIDeploymentResult, error) {
	if deploymentID == "" || deployedAt == nil || deployedAt.IsZero() {
		return nil, fmt.Errorf("control-plane deployments require non-empty deploymentID and deployedAt")
	}
	// Use the deployment service to handle the API configuration deployment
	result, err := deploymentService.DeployAPIConfiguration(APIDeploymentParams{
		Data:          yamlData,
		ContentType:   "application/yaml",
		APIID:         apiID, // Use the API ID from the deployment event
		DeploymentID:  deploymentID,
		Origin:        models.OriginControlPlane,
		DeployedAt:    deployedAt,
		CorrelationID: correlationID,
		Logger:        s.logger,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to deploy API configuration from YAML: %w", err)
	}

	return result, nil
}

// CreateLLMProviderFromYAML creates an LLM provider configuration from YAML data using the LLM deployment service
func (s *APIUtilsService) CreateLLMProviderFromYAML(yamlData []byte, providerID string, deploymentID string,
	deployedAt *time.Time, correlationID string,
	llmDeploymentService *LLMDeploymentService) (*APIDeploymentResult, error) {
	if deploymentID == "" || deployedAt == nil || deployedAt.IsZero() {
		return nil, fmt.Errorf("control-plane deployments require non-empty deploymentID and deployedAt")
	}
	// Use the LLM deployment service to handle the provider configuration deployment
	result, err := llmDeploymentService.DeployLLMProviderConfiguration(LLMDeploymentParams{
		Data:          yamlData,
		ContentType:   "application/yaml",
		ID:            providerID,
		DeploymentID:  deploymentID,
		Origin:        models.OriginControlPlane,
		DeployedAt:    deployedAt,
		CorrelationID: correlationID,
		Logger:        s.logger,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to deploy LLM provider configuration from YAML: %w", err)
	}

	return result, nil
}

// CreateLLMProxyFromYAML creates an LLM proxy configuration from YAML data using the LLM deployment service
func (s *APIUtilsService) CreateLLMProxyFromYAML(yamlData []byte, proxyID string, deploymentID string,
	deployedAt *time.Time, correlationID string,
	llmDeploymentService *LLMDeploymentService) (*APIDeploymentResult, error) {
	if deploymentID == "" || deployedAt == nil || deployedAt.IsZero() {
		return nil, fmt.Errorf("control-plane deployments require non-empty deploymentID and deployedAt")
	}
	// Use the LLM deployment service to handle the proxy configuration deployment
	result, err := llmDeploymentService.DeployLLMProxyConfiguration(LLMDeploymentParams{
		Data:          yamlData,
		ContentType:   "application/yaml",
		ID:            proxyID,
		DeploymentID:  deploymentID,
		Origin:        models.OriginControlPlane,
		DeployedAt:    deployedAt,
		CorrelationID: correlationID,
		Logger:        s.logger,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to deploy LLM proxy configuration from YAML: %w", err)
	}

	return result, nil
}

// FetchMCPProxyDefinition downloads the MCP proxy definition as a zip file from the control plane
func (s *APIUtilsService) FetchMCPProxyDefinition(proxyID string) ([]byte, error) {
	// Construct the MCP proxy URL by appending the resource path
	proxyURL := s.getBaseURL() + "/mcp-proxies/" + proxyID

	s.logger.Debug("Fetching MCP proxy definition",
		slog.String("proxy_id", proxyID),
		slog.String("url", proxyURL),
	)

	// Create request
	req, err := http.NewRequest("GET", proxyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/zip")

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MCP proxy definition: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MCP proxy request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Debug("Successfully fetched MCP proxy definition",
		slog.String("proxy_id", proxyID),
		slog.Int("size_bytes", len(bodyBytes)),
	)

	return bodyBytes, nil
}

// CreateMCPProxyFromYAML creates an MCP proxy configuration from YAML data using the MCP deployment service
func (s *APIUtilsService) CreateMCPProxyFromYAML(yamlData []byte, proxyID string, deploymentID string,
	deployedAt *time.Time, correlationID string,
	mcpDeploymentService *MCPDeploymentService) (*APIDeploymentResult, error) {
	if deploymentID == "" || deployedAt == nil || deployedAt.IsZero() {
		return nil, fmt.Errorf("control-plane deployments require non-empty deploymentID and deployedAt")
	}
	// Use the MCP deployment service to handle the proxy configuration deployment
	result, err := mcpDeploymentService.DeployMCPConfiguration(MCPDeploymentParams{
		Data:          yamlData,
		ContentType:   "application/yaml",
		ID:            proxyID,
		DeploymentID:  deploymentID,
		Origin:        models.OriginControlPlane,
		DeployedAt:    deployedAt,
		CorrelationID: correlationID,
		Logger:        s.logger,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to deploy MCP proxy configuration from YAML: %w", err)
	}

	return result, nil
}

// SaveAPIDefinition saves the API definition zip file to disk
func (s *APIUtilsService) SaveAPIDefinition(apiID string, zipData []byte) error {
	// Create data directory if it doesn't exist
	dataDir := "data/apis"
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Save zip file
	filename := filepath.Join(dataDir, fmt.Sprintf("%s.zip", apiID))
	if err := os.WriteFile(filename, zipData, 0600); err != nil {
		return fmt.Errorf("failed to save API definition: %w", err)
	}

	s.logger.Info("Saved API definition to disk",
		slog.String("api_id", apiID),
		slog.String("filename", filename),
	)

	return nil
}

// APIDeploymentPush represents the request body for pushing API deployment details to the control plane
type APIDeploymentPush struct {
	ID                string     `json:"id" yaml:"id"`
	Configuration     any        `json:"configuration" yaml:"configuration"`
	Status            string     `json:"status" yaml:"status"`
	CreatedAt         time.Time  `json:"createdAt" yaml:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt" yaml:"updatedAt"`
	DeployedAt        *time.Time `json:"deployedAt,omitempty" yaml:"deployedAt,omitempty"`
	ProjectIdentifier string     `json:"projectIdentifier" yaml:"projectIdentifier"`
}

// PushAPIDeployment sends API deployment details to the control plane via a REST call
func (s *APIUtilsService) PushAPIDeployment(apiID string, apiConfig *models.StoredConfig, deploymentID string) error {
	// Construct the deployment URL
	deployURL := s.getBaseURL() + "/apis/" + apiID + "/gateway-deployments"
	if deploymentID != "" {
		deployURL += "?deploymentId=" + deploymentID
	}

	// Create request body
	requestBody := APIDeploymentPush{
		ID:                apiConfig.UUID,
		Configuration:     apiConfig.Configuration,
		Status:            string(apiConfig.DesiredState),
		CreatedAt:         apiConfig.CreatedAt,
		UpdatedAt:         apiConfig.UpdatedAt,
		DeployedAt:        apiConfig.DeployedAt,
		ProjectIdentifier: "default", // Set a default value or fetch from config if needed
	}

	// Marshal request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create POST request
	req, err := http.NewRequest("POST", deployURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("api-key", s.config.Token)

	s.logger.Info("Pushing API deployment to control plane",
		slog.String("api_id", apiID),
		slog.String("url", deployURL),
		slog.String("deployment_id", deploymentID))

	// Make the request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send deployment notification: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		s.logger.Error("API deployment push failed",
			slog.String("api_id", apiID),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)))
		return fmt.Errorf("deployment push for api %s failed with status %d", apiID, resp.StatusCode)
	}

	s.logger.Info("Successfully pushed API deployment to control plane",
		slog.String("api_id", apiID),
		slog.Int("status_code", resp.StatusCode),
		slog.String("response", string(bodyBytes)))

	return nil
}

func MapToStruct(data map[string]interface{}, out interface{}) error {
	// Convert map -> JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal map to JSON: %w", err)
	}

	// Unmarshal JSON bytes -> target struct
	if err := json.Unmarshal(jsonBytes, out); err != nil {
		return fmt.Errorf("failed to unmarshal JSON to struct: %w", err)
	}

	return nil
}
