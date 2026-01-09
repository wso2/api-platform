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

package gateway

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

// Client represents an HTTP client configured for a specific gateway
type Client struct {
	gateway    *config.Gateway
	httpClient *http.Client
	credSource utils.CredentialSource
}

// NewClient creates a new gateway client for the specified gateway
func NewClient(gateway *config.Gateway) *Client {
	// Create HTTP client with TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &Client{
		gateway:    gateway,
		httpClient: httpClient,
	}
}

// NewClientByName creates a new gateway client for the gateway with the specified name
func NewClientByName(gatewayName string) (*Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	gateway, err := cfg.GetGateway(gatewayName)
	if err != nil {
		return nil, err
	}

	return NewClient(gateway), nil
}

// NewClientForActive creates a new gateway client for the active gateway
func NewClientForActive() (*Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	gateway, err := cfg.GetActiveGateway()
	if err != nil {
		return nil, err
	}

	return NewClient(gateway), nil
}

// Do executes an HTTP request with the gateway's authentication and settings
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Apply authentication based on gateway's auth type
	authType := c.gateway.Auth
	switch authType {
	case utils.AuthTypeNone:
		// No authentication required

	case utils.AuthTypeBasic:
		// Step 1: Check if ALL required environment variables are present
		envUsername := os.Getenv(utils.EnvGatewayUsername)
		envPassword := os.Getenv(utils.EnvGatewayPassword)

		if envUsername != "" && envPassword != "" {
			// Use environment variables (both present)
			req.SetBasicAuth(envUsername, envPassword)
			c.credSource = utils.CredSourceEnv
		} else {
			// Step 2: Fall back to config credentials
			username := c.gateway.Username
			password := c.gateway.Password

			if username == "" || password == "" {
				// Step 3: Neither env nor config has complete credentials
				return nil, fmt.Errorf("%s", utils.FormatCredentialsNotFoundError(c.gateway.Name, authType))
			}

			req.SetBasicAuth(username, password)
			c.credSource = utils.CredSourceConfig
		}

	case utils.AuthTypeBearer:
		// Step 1: Check if environment variable is present
		envToken := os.Getenv(utils.EnvGatewayToken)

		if envToken != "" {
			// Use environment variable
			req.Header.Set("Authorization", "Bearer "+envToken)
			c.credSource = utils.CredSourceEnv
		} else {
			// Step 2: Fall back to config token
			token := c.gateway.Token

			if token == "" {
				// Step 3: Neither env nor config has credentials
				return nil, fmt.Errorf("%s", utils.FormatCredentialsNotFoundError(c.gateway.Name, authType))
			}

			req.Header.Set("Authorization", "Bearer "+token)
			c.credSource = utils.CredSourceConfig
		}

	default:
		return nil, fmt.Errorf("unsupported auth type '%s' for gateway '%s'", authType, c.gateway.Name)
	}

	// Set common headers
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	return c.httpClient.Do(req)
}

// formatHTTPError formats HTTP errors with credential-source-aware messaging
func (c *Client) formatHTTPError(operation string, resp *http.Response) error {
	return utils.FormatHTTPErrorWithCredSource(
		operation,
		resp,
		"Gateway Controller",
		c.gateway.Auth,
		c.credSource,
		c.gateway.Name,
	)
}

// Get performs a GET request to the specified path
func (c *Client) Get(path string) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.gateway.Server, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := baseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	// Treat 2XX as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	// Treat 404 as non-error
	if resp.StatusCode == http.StatusNotFound {
		return resp, nil
	}

	return nil, c.formatHTTPError(fmt.Sprintf("GET %s", path), resp)
}

// Post performs a POST request to the specified path with the given body
func (c *Client) Post(path string, body io.Reader) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.gateway.Server, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := baseURL + path
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	// Treat 2XX as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("POST %s", path), resp)
}

// PostYAML performs a POST request with YAML content
func (c *Client) PostYAML(path string, body io.Reader) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.gateway.Server, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := baseURL + path
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-yaml")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	// Treat 2XX as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("POST %s", path), resp)
}

// Put performs a PUT request to the specified path with the given body
func (c *Client) Put(path string, body io.Reader) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.gateway.Server, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := baseURL + path
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	// Treat 2XX as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("PUT %s", path), resp)
}

// PutYAML performs a PUT request with YAML content
func (c *Client) PutYAML(path string, body io.Reader) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.gateway.Server, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := baseURL + path
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-yaml")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	// Treat 2XX as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("PUT %s", path), resp)
}

// Delete performs a DELETE request to the specified path
func (c *Client) Delete(path string) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.gateway.Server, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := baseURL + path
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	// Treat 2XX as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("DELETE %s", path), resp)
}

// GetGateway returns the gateway configuration
func (c *Client) GetGateway() *config.Gateway {
	return c.gateway
}

// GetBaseURL returns the base URL of the gateway
func (c *Client) GetBaseURL() string {
	return c.gateway.Server
}

// FormatHTTPError is implemented in the utils package; use utils.FormatHTTPError
