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

package devportal

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

type Client struct {
	devPortal  *config.DevPortal
	httpClient *http.Client
}

type credCtxKey struct{}

func NewClient(devPortal *config.DevPortal) *Client {
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
		devPortal:  devPortal,
		httpClient: httpClient,
	}
}

func NewClientByName(devPortalName string) (*Client, error) {
	return NewClientByNamePlatform("", devPortalName)
}

func NewClientByNamePlatform(platformName, devPortalName string) (*Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, err := cfg.GetDevPortalFromPlatform(cfg.ResolvePlatform(platformName), devPortalName)
	if err != nil {
		return nil, err
	}

	return NewClient(devPortal), nil
}

func NewClientForActive() (*Client, error) {
	return NewClientForActivePlatform("")
}

func NewClientForActivePlatform(platformName string) (*Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, err := cfg.GetActiveDevPortalFromPlatform(cfg.ResolvePlatform(platformName))
	if err != nil {
		return nil, err
	}

	return NewClient(devPortal), nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	authType := c.devPortal.Auth.Type
	var credSource utils.CredentialSource

	switch authType {
	case utils.AuthTypeBasic:
		envUsername := os.Getenv(utils.EnvDevPortalUsername)
		envPassword := os.Getenv(utils.EnvDevPortalPassword)

		if envUsername != "" && envPassword != "" {
			req.SetBasicAuth(envUsername, envPassword)
			credSource = utils.CredSourceEnv
		} else {
			if c.devPortal.Auth.Username == "" || c.devPortal.Auth.Password == "" {
				return nil, fmt.Errorf("authentication credentials not found for devportal '%s' (auth type: %s).\nPlease either:\n  - Re-add devportal: ap devportal add --display-name %s --server <server_url> --auth %s\n  - Or export: %s and %s",
					c.devPortal.Name, authType, c.devPortal.Name, authType, utils.EnvDevPortalUsername, utils.EnvDevPortalPassword)
			}
			req.SetBasicAuth(c.devPortal.Auth.Username, c.devPortal.Auth.Password)
			credSource = utils.CredSourceConfig
		}

	case utils.AuthTypeOAuth:
		envToken := os.Getenv(utils.EnvDevPortalToken)
		if envToken != "" {
			req.Header.Set("Authorization", "Bearer "+envToken)
			credSource = utils.CredSourceEnv
		} else {
			if c.devPortal.Auth.Token == "" {
				return nil, fmt.Errorf("authentication credentials not found for devportal '%s' (auth type: %s).\nPlease either:\n  - Re-add devportal: ap devportal add --display-name %s --server <server_url> --auth %s\n  - Or export: %s",
					c.devPortal.Name, authType, c.devPortal.Name, authType, utils.EnvDevPortalToken)
			}
			req.Header.Set("Authorization", "Bearer "+c.devPortal.Auth.Token)
			credSource = utils.CredSourceConfig
		}

	case utils.AuthTypeAPIKey:
		envAPIKey := os.Getenv(utils.EnvDevPortalAPIKey)
		if envAPIKey != "" {
			req.Header.Set(utils.DevPortalAPIHeader, envAPIKey)
			credSource = utils.CredSourceEnv
		} else {
			if c.devPortal.Auth.APIKey == "" {
				return nil, fmt.Errorf("authentication credentials not found for devportal '%s' (auth type: %s).\nPlease either:\n  - Re-add devportal: ap devportal add --display-name %s --server <server_url> --auth %s\n  - Or export: %s",
					c.devPortal.Name, authType, c.devPortal.Name, authType, utils.EnvDevPortalAPIKey)
			}
			req.Header.Set(utils.DevPortalAPIHeader, c.devPortal.Auth.APIKey)
			credSource = utils.CredSourceConfig
		}

	default:
		return nil, fmt.Errorf("unsupported auth type '%s' for devportal '%s'", authType, c.devPortal.Name)
	}

	req = req.WithContext(context.WithValue(req.Context(), credCtxKey{}, credSource))

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	return c.httpClient.Do(req)
}

func (c *Client) Get(path string) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.devPortal.URL, "/")
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

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return resp, nil
	}

	return nil, c.formatHTTPError(fmt.Sprintf("GET %s", path), resp)
}

func (c *Client) formatHTTPError(operation string, resp *http.Response) error {
	var credSource utils.CredentialSource
	if resp != nil && resp.Request != nil {
		if v := resp.Request.Context().Value(credCtxKey{}); v != nil {
			if cs, ok := v.(utils.CredentialSource); ok {
				credSource = cs
			}
		}
	}

	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		switch credSource {
		case utils.CredSourceEnv:
			switch c.devPortal.Auth.Type {
			case utils.AuthTypeBasic:
				return fmt.Errorf("%s failed (status %d) from DevPortal: unauthorized.\n\nCredentials were sourced from environment variables.\nPlease verify and re-export:\n  export %s=<username>\n  export %s=<password>",
					operation, resp.StatusCode, utils.EnvDevPortalUsername, utils.EnvDevPortalPassword)
			case utils.AuthTypeOAuth:
				return fmt.Errorf("%s failed (status %d) from DevPortal: unauthorized.\n\nCredentials were sourced from environment variables.\nPlease verify and re-export:\n  export %s=<token>",
					operation, resp.StatusCode, utils.EnvDevPortalToken)
			case utils.AuthTypeAPIKey:
				return fmt.Errorf("%s failed (status %d) from DevPortal: unauthorized.\n\nCredentials were sourced from environment variables.\nPlease verify and re-export:\n  export %s=<api-key>",
					operation, resp.StatusCode, utils.EnvDevPortalAPIKey)
			}
		case utils.CredSourceConfig:
			switch c.devPortal.Auth.Type {
			case utils.AuthTypeBasic:
				return fmt.Errorf("%s failed (status %d) from DevPortal: unauthorized.\n\nCredentials were sourced from the configuration file.\nPlease either:\n  1. Re-add the devportal with correct credentials:\n     ap devportal add --display-name %s --server <server_url> --auth %s\n  2. Or export environment variables to override:\n     export %s=<username>\n     export %s=<password>",
					operation, resp.StatusCode, c.devPortal.Name, utils.AuthTypeBasic, utils.EnvDevPortalUsername, utils.EnvDevPortalPassword)
			case utils.AuthTypeOAuth:
				return fmt.Errorf("%s failed (status %d) from DevPortal: unauthorized.\n\nCredentials were sourced from the configuration file.\nPlease either:\n  1. Re-add the devportal with correct credentials:\n     ap devportal add --display-name %s --server <server_url> --auth %s\n  2. Or export environment variables to override:\n     export %s=<token>",
					operation, resp.StatusCode, c.devPortal.Name, utils.AuthTypeOAuth, utils.EnvDevPortalToken)
			case utils.AuthTypeAPIKey:
				return fmt.Errorf("%s failed (status %d) from DevPortal: unauthorized.\n\nCredentials were sourced from the configuration file.\nPlease either:\n  1. Re-add the devportal with correct credentials:\n     ap devportal add --display-name %s --server <server_url> --auth %s\n  2. Or export environment variables to override:\n     export %s=<api-key>",
					operation, resp.StatusCode, c.devPortal.Name, utils.AuthTypeAPIKey, utils.EnvDevPortalAPIKey)
			}
		}
	}

	return utils.FormatHTTPError(operation, resp, "DevPortal")
}
