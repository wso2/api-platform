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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// APIFetchConfig contains configuration for fetching API definitions
type APIFetchConfig struct {
	BaseURL            string        // Base URL for API requests
	Token              string        // Authentication token
	InsecureSkipVerify bool          // Skip TLS verification
	Timeout            time.Duration // Request timeout
}

// APIUtilsService provides utilities for API operations
type APIUtilsService struct {
	config APIFetchConfig
	logger *zap.Logger
}

// NewAPIUtilsService creates a new API utilities service
func NewAPIUtilsService(config APIFetchConfig, logger *zap.Logger) *APIUtilsService {
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
		zap.String("api_id", apiID),
		zap.String("url", apiURL),
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
		zap.String("api_id", apiID),
		zap.Int("size_bytes", len(bodyBytes)),
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
				zap.String("filename", file.Name),
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
func (s *APIUtilsService) CreateAPIFromYAML(yamlData []byte, apiID string, correlationID string, deploymentService *APIDeploymentService) error {
	// Use the deployment service to handle the API configuration deployment
	_, err := deploymentService.DeployAPIConfiguration(APIDeploymentParams{
		Data:          yamlData,
		ContentType:   "application/yaml",
		APIID:         apiID, // Use the API ID from the deployment event
		CorrelationID: correlationID,
		Logger:        s.logger,
	})

	if err != nil {
		return fmt.Errorf("failed to deploy API configuration from YAML: %w", err)
	}

	return nil
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
		zap.String("api_id", apiID),
		zap.String("filename", filename),
	)

	return nil
}
