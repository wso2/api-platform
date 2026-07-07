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

// Package aiworkspace holds the HTTP client used by the `ap ai-workspace` commands to
// talk to an AI Workspace server (LLM proxies / providers).
package aiworkspace

import (
	"bytes"
	"context"
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

type Client struct {
	aiWorkspace *config.AIWorkspace
	httpClient  *http.Client
}

type credCtxKey struct{}

func NewClient(aiWorkspace *config.AIWorkspace) *Client {
	return NewClientWithOptions(aiWorkspace, false)
}

func NewClientWithOptions(aiWorkspace *config.AIWorkspace, insecure bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: insecure,
		},
	}

	return &Client{
		aiWorkspace: aiWorkspace,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// PutJSON sends a JSON body with the PUT method.
func (c *Client) PutJSON(path string, body []byte) (*http.Response, error) {
	return c.sendJSON(http.MethodPut, path, body)
}

// PostJSON sends a JSON body with the POST method.
func (c *Client) PostJSON(path string, body []byte) (*http.Response, error) {
	return c.sendJSON(http.MethodPost, path, body)
}

// Get sends a GET request and returns the response when it is a 2xx. A 404 (and
// any other non-2xx) is surfaced as an error, so callers that expect a resource
// to be present are never handed a not-found body. Use Exists for a probe that
// treats a 404 as a normal "absent" answer instead.
func (c *Client) Get(path string) (*http.Response, error) {
	return c.sendNoBody(http.MethodGet, path)
}

// Delete sends a DELETE request and returns the response when it is a 2xx.
func (c *Client) Delete(path string) (*http.Response, error) {
	return c.sendNoBody(http.MethodDelete, path)
}

// Exists reports whether a resource exists at path by issuing a GET and mapping
// the status code: a 2xx means it exists, a 404 means it does not, and any other
// status is returned as an error. This is the create-or-update probe used by
// `ap ai-workspace apply`, mirroring gateway.Client.Get's behaviour. It is kept
// separate from Get so that the get/list commands keep their strict
// "404 is a failure" contract while apply can branch on presence.
func (c *Client) Exists(path string) (bool, error) {
	resp, err := c.send(http.MethodGet, path, nil, "")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, c.formatHTTPError(fmt.Sprintf("GET %s", path), resp)
}

// send builds and executes a request to path with the given method, optional
// body and content type, returning the raw response without interpreting its
// status code. Callers layer their own status handling on top.
func (c *Client) send(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	baseURL := strings.TrimSuffix(c.aiWorkspace.URL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if baseURL != "" {
		req.Header.Set("Origin", baseURL)
	}

	return c.Do(req)
}

func (c *Client) sendNoBody(method, path string) (*http.Response, error) {
	resp, err := c.send(method, path, nil, "")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("%s %s", method, path), resp)
}

func (c *Client) sendJSON(method, path string, body []byte) (*http.Response, error) {
	resp, err := c.send(method, path, bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	return nil, c.formatHTTPError(fmt.Sprintf("%s %s", method, path), resp)
}

// Do attaches credentials based on the configured auth type (with environment
// variable overrides) and sends the request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	authType := c.aiWorkspace.Auth.Type
	var credSource utils.CredentialSource

	switch authType {
	case utils.AuthTypeBasic:
		envUsername := os.Getenv(utils.EnvAIWorkspaceUsername)
		envPassword := os.Getenv(utils.EnvAIWorkspacePassword)
		if envUsername != "" && envPassword != "" {
			req.SetBasicAuth(envUsername, envPassword)
			credSource = utils.CredSourceEnv
		} else {
			if c.aiWorkspace.Auth.Username == "" || c.aiWorkspace.Auth.Password == "" {
				return nil, c.missingCredsError(authType, utils.EnvAIWorkspaceUsername+" and "+utils.EnvAIWorkspacePassword)
			}
			req.SetBasicAuth(c.aiWorkspace.Auth.Username, c.aiWorkspace.Auth.Password)
			credSource = utils.CredSourceConfig
		}

	case utils.AuthTypeOAuth:
		envToken := os.Getenv(utils.EnvAIWorkspaceToken)
		if envToken != "" {
			req.Header.Set("Authorization", "Bearer "+envToken)
			credSource = utils.CredSourceEnv
		} else {
			if c.aiWorkspace.Auth.Token == "" {
				return nil, c.missingCredsError(authType, utils.EnvAIWorkspaceToken)
			}
			req.Header.Set("Authorization", "Bearer "+c.aiWorkspace.Auth.Token)
			credSource = utils.CredSourceConfig
		}

	case utils.AuthTypeAPIKey:
		envAPIKey := os.Getenv(utils.EnvAIWorkspaceAPIKey)
		if envAPIKey != "" {
			req.Header.Set(utils.AIWorkspaceAPIHeader, envAPIKey)
			credSource = utils.CredSourceEnv
		} else {
			if c.aiWorkspace.Auth.APIKey == "" {
				return nil, c.missingCredsError(authType, utils.EnvAIWorkspaceAPIKey)
			}
			req.Header.Set(utils.AIWorkspaceAPIHeader, c.aiWorkspace.Auth.APIKey)
			credSource = utils.CredSourceConfig
		}

	default:
		return nil, fmt.Errorf("unsupported auth type '%s' for ai-workspace '%s'", authType, c.aiWorkspace.Name)
	}

	req = req.WithContext(context.WithValue(req.Context(), credCtxKey{}, credSource))
	return c.httpClient.Do(req)
}

func (c *Client) missingCredsError(authType, envVars string) error {
	return fmt.Errorf("authentication credentials not found for ai-workspace '%s' (auth type: %s).\nPlease either:\n  - Re-add ai-workspace: ap ai-workspace add --display-name %s --server <server_url> --auth %s\n  - Or export: %s",
		c.aiWorkspace.Name, authType, c.aiWorkspace.Name, authType, envVars)
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
	return utils.FormatHTTPErrorWithCredSource(operation, resp, "AI Workspace", c.aiWorkspace.Auth.Type, credSource, c.aiWorkspace.Name)
}
