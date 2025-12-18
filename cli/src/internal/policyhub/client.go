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

package policyhub

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wso2/api-platform/cli/utils"
)

// Checksum represents the checksum information
type Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// PolicyData represents a single policy in the response
type PolicyData struct {
	Checksum    Checksum `json:"checksum"`
	DownloadURL string   `json:"download_url"`
	PolicyName  string   `json:"policy_name"`
	Version     string   `json:"version"`
}

// Meta represents metadata in the response
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
	TraceID   string `json:"trace_id"`
}

// ResolveResponse represents the PolicyHub resolve API response
type ResolveResponse struct {
	Data    []PolicyData `json:"data"`
	Error   interface{}  `json:"error"`
	Meta    Meta         `json:"meta"`
	Success bool         `json:"success"`
}

// PolicyHubClient handles PolicyHub API interactions
type PolicyHubClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewPolicyHubClient creates a new PolicyHub client
func NewPolicyHubClient() *PolicyHubClient {
	return &PolicyHubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: utils.PolicyHubBaseURL,
	}
}

// ResolvePolicies calls the PolicyHub resolve API
func (c *PolicyHubClient) ResolvePolicies(policiesJSON []byte) (*ResolveResponse, error) {
	url := c.baseURL + utils.PolicyHubResolvePath

	req, err := http.NewRequest("POST", url, bytes.NewReader(policiesJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call PolicyHub API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PolicyHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response ResolveResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("PolicyHub API returned error: %v", response.Error)
	}

	return &response, nil
}

// GetPoliciesDir returns the path to the policies directory
func GetPoliciesDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, utils.PoliciesPath), nil
}

// EnsurePoliciesDir creates the policies directory if it doesn't exist
func EnsurePoliciesDir() (string, error) {
	policiesDir, err := GetPoliciesDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(policiesDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create policies directory: %w", err)
	}

	return policiesDir, nil
}

// DownloadPolicy downloads a policy zip file from the given URL
func (c *PolicyHubClient) DownloadPolicy(url, destPath string) error {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// GetPolicyZipPath returns the path for a policy zip file
func GetPolicyZipPath(policiesDir, policyName, version string) string {
	return filepath.Join(policiesDir, fmt.Sprintf("%s-%s.zip", policyName, version))
}

// ExtractZipToMemory extracts a zip file and returns its contents in memory
func ExtractZipToMemory(zipPath string) (map[string][]byte, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	contents := make(map[string][]byte)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s in zip: %w", f.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s in zip: %w", f.Name, err)
		}

		contents[f.Name] = data
	}

	return contents, nil
}
