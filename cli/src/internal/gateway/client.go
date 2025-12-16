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
	"time"

	"github.com/wso2/api-platform/cli/internal/config"
)

// Client represents an HTTP client configured for a specific gateway
type Client struct {
	gateway    *config.Gateway
	httpClient *http.Client
}

// NewClient creates a new gateway client for the specified gateway
func NewClient(gateway *config.Gateway) *Client {
	// Create HTTP client with appropriate TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: gateway.Insecure,
			MinVersion:         tls.VersionTLS12,
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
	// Add authentication token if present
	if c.gateway.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.gateway.Token)
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

// Get performs a GET request to the specified path
func (c *Client) Get(path string) (*http.Response, error) {
	url := c.gateway.Server + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.Do(req)
}

// Post performs a POST request to the specified path with the given body
func (c *Client) Post(path string, body io.Reader) (*http.Response, error) {
	url := c.gateway.Server + path
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.Do(req)
}

// Put performs a PUT request to the specified path with the given body
func (c *Client) Put(path string, body io.Reader) (*http.Response, error) {
	url := c.gateway.Server + path
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.Do(req)
}

// Delete performs a DELETE request to the specified path
func (c *Client) Delete(path string) (*http.Response, error) {
	url := c.gateway.Server + path
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.Do(req)
}

// GetGateway returns the gateway configuration
func (c *Client) GetGateway() *config.Gateway {
	return c.gateway
}

// GetBaseURL returns the base URL of the gateway
func (c *Client) GetBaseURL() string {
	return c.gateway.Server
}
