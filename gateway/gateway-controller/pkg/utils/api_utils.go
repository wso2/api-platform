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
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
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
	config PlatformAPIConfig
	logger *slog.Logger
}

// NewAPIUtilsService creates a new API utilities service
func NewAPIUtilsService(config PlatformAPIConfig, logger *slog.Logger) *APIUtilsService {
	// Set default timeout if not provided
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &APIUtilsService{
		config: config,
		logger: logger,
	}
}

// FetchAPIDefinition downloads the API definition as a zip file from the control plane
func (s *APIUtilsService) FetchAPIDefinition(apiID string) ([]byte, error) {
	// Construct the API URL by appending the resource path
	apiURL := s.config.BaseURL + "/apis/" + apiID

	s.logger.Info("Fetching API definition",
		slog.String("api_id", apiID),
		slog.String("url", apiURL),
	)

	// Create HTTP client with TLS configuration
	client := &http.Client{
		Timeout: s.config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.config.InsecureSkipVerify,
			},
		},
	}

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Add("api-key", s.config.Token)
	req.Header.Add("Accept", "application/zip")

	// Make request
	resp, err := client.Do(req)
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
func (s *APIUtilsService) CreateAPIFromYAML(yamlData []byte, apiID string, correlationID string,
	deploymentService *APIDeploymentService) (*APIDeploymentResult, error) {
	// Use the deployment service to handle the API configuration deployment
	result, err := deploymentService.DeployAPIConfiguration(APIDeploymentParams{
		Data:          yamlData,
		ContentType:   "application/yaml",
		APIID:         apiID, // Use the API ID from the deployment event
		CorrelationID: correlationID,
		Logger:        s.logger,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to deploy API configuration from YAML: %w", err)
	}

	return result, nil
}

// SaveAPIDefinition saves the API definition zip file to disk
func (s *APIUtilsService) SaveAPIDefinition(apiID string, zipData []byte) error {
	// Create data directory if it doesn't exist
	dataDir := "data/apis"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Save zip file
	filename := filepath.Join(dataDir, fmt.Sprintf("%s.zip", apiID))
	if err := os.WriteFile(filename, zipData, 0644); err != nil {
		return fmt.Errorf("failed to save API definition: %w", err)
	}

	s.logger.Info("Saved API definition to disk",
		slog.String("api_id", apiID),
		slog.String("filename", filename),
	)

	return nil
}

// APIDeploymentNotification represents the request body for notifying control plane about API deployments in the gateway
type APIDeploymentNotification struct {
	ID                string               `json:"id" yaml:"id"`
	Configuration     api.APIConfiguration `json:"configuration" yaml:"configuration"`
	Status            string               `json:"status" yaml:"status"`
	CreatedAt         time.Time            `json:"createdAt" yaml:"createdAt"`
	UpdatedAt         time.Time            `json:"updatedAt" yaml:"updatedAt"`
	DeployedAt        *time.Time           `json:"deployedAt,omitempty" yaml:"deployedAt,omitempty"`
	DeployedVersion   int64                `json:"deployedVersion" yaml:"deployedVersion"`
	ProjectIdentifier string               `json:"projectIdentifier" yaml:"projectIdentifier"`
}

// NotifyAPIDeployment sends a REST API call to platform-api when an API is deployed successfully
func (s *APIUtilsService) NotifyAPIDeployment(apiID string, apiConfig *models.StoredConfig, revisionID string) error {
	// Construct the deployment URL
	deployURL := s.config.BaseURL + "/apis/" + apiID + "/gateway-deployments"
	if revisionID != "" {
		deployURL += "?revisionId=" + revisionID
	}

	// Create request body
	requestBody := APIDeploymentNotification{
		ID:                apiConfig.ID,
		Configuration:     apiConfig.Configuration,
		Status:            string(apiConfig.Status),
		CreatedAt:         apiConfig.CreatedAt,
		UpdatedAt:         apiConfig.UpdatedAt,
		DeployedAt:        apiConfig.DeployedAt,
		DeployedVersion:   apiConfig.DeployedVersion,
		ProjectIdentifier: "default", // Set a default value or fetch from config if needed
	}

	// Marshal request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP client with TLS configuration
	client := &http.Client{
		Timeout: s.config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.config.InsecureSkipVerify,
			},
		},
	}

	// Create POST request
	req, err := http.NewRequest("POST", deployURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("api-key", s.config.Token)

	s.logger.Info("Sending API deployment notification to platform-api",
		slog.String("api_id", apiID),
		slog.String("url", deployURL),
		slog.String("revision_id", revisionID))

	// Make the request
	resp, err := client.Do(req)
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
		s.logger.Error("API deployment notification failed",
			slog.String("api_id", apiID),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)))
		return fmt.Errorf("deployment notification for api %s failed with status %d", apiID, resp.StatusCode)
	}

	s.logger.Info("Successfully sent API deployment notification",
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
