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

package aiworkspace

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

// ResolveAIWorkspace resolves the AI Workspace to use from either explicit
// flags or the active AI Workspace in the resolved platform.
func ResolveAIWorkspace(cfg *config.Config, selectedName, selectedPlatform string) (*config.AIWorkspace, string, error) {
	selectedName = strings.TrimSpace(selectedName)
	selectedPlatform = strings.TrimSpace(selectedPlatform)

	if selectedName != "" {
		resolvedPlatform := config.DefaultPlatform
		if selectedPlatform != "" {
			resolvedPlatform = cfg.ResolvePlatform(selectedPlatform)
		}
		aiWorkspace, err := cfg.GetAIWorkspaceFromPlatform(resolvedPlatform, selectedName)
		if err != nil {
			return nil, "", err
		}
		return aiWorkspace, resolvedPlatform, nil
	}

	resolvedPlatform := cfg.ResolvePlatform(selectedPlatform)
	aiWorkspace, err := cfg.GetActiveAIWorkspaceFromPlatform(resolvedPlatform)
	if err != nil {
		return nil, "", err
	}
	return aiWorkspace, resolvedPlatform, nil
}

// ProviderPath builds the llm-providers resource path with the organizationId
// query parameter.
func ProviderPath(orgID string) string {
	return withOrg(utils.AIWorkspaceLLMProvidersPath, orgID)
}

// ProxyPath builds the llm-proxies resource path with the organizationId query
// parameter.
func ProxyPath(orgID string) string {
	return withOrg(utils.AIWorkspaceLLMProxiesPath, orgID)
}

// MCPProxyPath builds the mcp-proxies resource path with the organizationId
// query parameter.
func MCPProxyPath(orgID string) string {
	return withOrg(utils.AIWorkspaceMCPProxiesPath, orgID)
}

// ProviderResourcePath builds the llm-providers/{id} resource path with the
// organizationId query parameter.
func ProviderResourcePath(orgID, id string) string {
	return withOrg(utils.AIWorkspaceLLMProvidersPath+"/"+url.PathEscape(id), orgID)
}

// ProxyResourcePath builds the llm-proxies/{id} resource path with the
// organizationId query parameter.
func ProxyResourcePath(orgID, id string) string {
	return withOrg(utils.AIWorkspaceLLMProxiesPath+"/"+url.PathEscape(id), orgID)
}

// MCPProxyResourcePath builds the mcp-proxies/{id} resource path with the
// organizationId query parameter.
func MCPProxyResourcePath(orgID, id string) string {
	return withOrg(utils.AIWorkspaceMCPProxiesPath+"/"+url.PathEscape(id), orgID)
}

func withOrg(path, orgID string) string {
	return fmt.Sprintf("%s?organizationId=%s", path, url.QueryEscape(orgID))
}

// ReadJSONFile reads and validates a JSON payload file from disk.
func ReadJSONFile(filePath string) ([]byte, error) {
	resolvedPath, err := filepath.Abs(strings.TrimSpace(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", resolvedPath)
		}
		return nil, fmt.Errorf("failed to inspect file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("file path must point to a JSON file, got directory: %s", resolvedPath)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if !json.Valid(content) {
		return nil, fmt.Errorf("file is not valid JSON: %s", resolvedPath)
	}
	return content, nil
}

// WrapRequestError formats request failures and adds a hint about --insecure
// when the error is caused by certificate verification.
func WrapRequestError(action string, err error, insecure bool) error {
	if shouldSuggestInsecure(err) && !insecure {
		return fmt.Errorf("failed to %s: %w\nhint: retry with --insecure if you are using a self-signed or local development certificate", action, err)
	}
	return fmt.Errorf("failed to %s: %w", action, err)
}

func shouldSuggestInsecure(err error) bool {
	var unknownAuthorityErr x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var certificateInvalidError x509.CertificateInvalidError
	const certificateInvalidErrorString = "tls: failed to verify certificate: x509"

	return errors.As(err, &unknownAuthorityErr) ||
		errors.As(err, &hostnameError) ||
		errors.As(err, &certificateInvalidError) ||
		strings.Contains(err.Error(), certificateInvalidErrorString)
}

// PrintJSONResponse prints an HTTP response as pretty JSON when possible and
// falls back to the raw trimmed body otherwise.
func PrintJSONResponse(resp *http.Response) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}

	var jsonBody interface{}
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		if pretty, marshalErr := json.MarshalIndent(jsonBody, "", "  "); marshalErr == nil {
			fmt.Println(string(pretty))
			return nil
		}
	}

	fmt.Println(trimmed)
	return nil
}

func ResourceID(displayName, fallback string) string {
	id := strings.Join(strings.Fields(strings.ToLower(displayName)), "-")
	if id == "" {
		return fallback
	}
	return id
}
