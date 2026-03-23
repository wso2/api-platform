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
	"strings"
	"sync"
	"time"

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

// FetchControlPlaneDeployments retrieves the list of deployments that should be active on this gateway
// from the platform-API. If since is non-nil, only deployments updated after that timestamp are returned
// (incremental sync). Pass nil for a full sync.
func (s *APIUtilsService) FetchControlPlaneDeployments(since *time.Time) ([]models.ControlPlaneDeployment, error) {
	deploymentsURL := s.getBaseURL() + "/deployments"
	if since != nil {
		deploymentsURL += "?since=" + since.Format(time.RFC3339)
	}

	s.logger.Info("Fetching control plane deployments",
		slog.String("url", deploymentsURL),
	)

	req, err := http.NewRequest("GET", deploymentsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch control plane deployments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("control plane deployments request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response models.ControlPlaneDeploymentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode control plane deployments response: %w", err)
	}

	s.logger.Info("Successfully fetched control plane deployments",
		slog.Int("count", len(response.Deployments)),
	)

	return response.Deployments, nil
}

// BatchFetchDeployments fetches multiple deployment artifacts in a single request.
// It returns the raw zip data containing deployment directories, each named by deployment ID
// and containing the artifact YAML file. Returns an error if the request fails.
func (s *APIUtilsService) BatchFetchDeployments(deploymentIDs []string) ([]byte, error) {
	batchURL := s.getBaseURL() + "/deployments/fetch-batch"

	s.logger.Info("Batch fetching deployments from platform-API",
		slog.String("url", batchURL),
		slog.Int("count", len(deploymentIDs)),
	)

	requestBody := models.BatchFetchRequest{
		DeploymentIDs: deploymentIDs,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch fetch request: %w", err)
	}

	req, err := http.NewRequest("POST", batchURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/zip")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch deployments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("batch fetch request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch fetch response body: %w", err)
	}

	s.logger.Info("Successfully batch fetched deployments",
		slog.Int("count", len(deploymentIDs)),
		slog.Int("size_bytes", len(bodyBytes)),
	)

	return bodyBytes, nil
}

// ExtractDeploymentsFromBatchZip processes a batch zip response and extracts YAML content
// for each deployment. The zip structure has top-level directories named by deployment ID,
// each containing a YAML file. Returns a map of deployment ID to YAML content bytes.
func (s *APIUtilsService) ExtractDeploymentsFromBatchZip(zipData []byte) (map[string][]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	deployments := make(map[string][]byte)
	for _, file := range zipReader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		cleanPath := filepath.Clean(file.Name)
		if strings.Contains(cleanPath, "..") {
			s.logger.Warn("Skipping zip entry with path traversal",
				slog.String("path", file.Name),
			)
			continue
		}

		// Extract deployment ID from directory name (first path component)
		dir := filepath.Dir(cleanPath)
		deploymentID := filepath.Base(dir)
		if deploymentID == "." || deploymentID == "" {
			s.logger.Warn("Skipping file with unexpected path in batch zip",
				slog.String("path", file.Name),
			)
			continue
		}

		// Only process YAML files
		ext := filepath.Ext(cleanPath)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			s.logger.Warn("Failed to open file in batch zip",
				slog.String("path", file.Name),
				slog.Any("error", err),
			)
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			s.logger.Warn("Failed to read file in batch zip",
				slog.String("path", file.Name),
				slog.Any("error", err),
			)
			continue
		}

		deployments[deploymentID] = content
	}

	s.logger.Info("Extracted deployments from batch zip",
		slog.Int("count", len(deployments)),
	)

	return deployments, nil
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
