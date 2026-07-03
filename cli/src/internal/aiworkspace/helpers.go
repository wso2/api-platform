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

// ProviderPath returns the llm-providers collection path used to create a
// provider. The organization is derived from the auth token, so no
// organizationId query parameter is added.
func ProviderPath() string {
	return utils.AIWorkspaceLLMProvidersPath
}

// ProxyPath returns the llm-proxies collection path used to create a proxy. The
// organization is derived from the auth token, so no organizationId query
// parameter is added.
func ProxyPath() string {
	return utils.AIWorkspaceLLMProxiesPath
}

// MCPProxyPath returns the mcp-proxies collection path used to create an MCP
// proxy. The organization is derived from the auth token, so no organizationId
// query parameter is added.
func MCPProxyPath() string {
	return utils.AIWorkspaceMCPProxiesPath
}

// ProviderByIDPath builds the llm-providers/{id} path with only the id path
// parameter (no organizationId/projectId query). Used for delete.
func ProviderByIDPath(id string) string {
	return utils.AIWorkspaceLLMProvidersPath + "/" + url.PathEscape(id)
}

// ProxyByIDPath builds the llm-proxies/{id} path. Fetching a single proxy takes
// only the id path parameter (no organizationId/projectId query).
func ProxyByIDPath(id string) string {
	return utils.AIWorkspaceLLMProxiesPath + "/" + url.PathEscape(id)
}

// MCPProxyByIDPath builds the mcp-proxies/{id} path. Fetching a single proxy
// takes only the id path parameter (no organizationId/projectId query).
func MCPProxyByIDPath(id string) string {
	return utils.AIWorkspaceMCPProxiesPath + "/" + url.PathEscape(id)
}

func withProject(path, projectID string) string {
	return fmt.Sprintf("%s?projectId=%s", path, url.QueryEscape(projectID))
}

// ListQuery holds optional pagination parameters for list requests.
type ListQuery struct {
	Limit  string
	Offset string
}

func withProjectListParams(basePath, projectID string, q ListQuery) string {
	return appendPagination(withProject(basePath, projectID), q)
}

func appendPagination(path string, q ListQuery) string {
	// Use "?" for the first query parameter when the path has none yet
	// (provider list), otherwise "&" (proxy/mcp list already carry ?projectId=).
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	addParam := func(key, value string) {
		if v := strings.TrimSpace(value); v != "" {
			path += sep + key + "=" + url.QueryEscape(v)
			sep = "&"
		}
	}
	addParam("limit", q.Limit)
	addParam("offset", q.Offset)
	return path
}

// ProviderListPath builds the llm-providers list path with optional pagination.
// The organization is derived from the auth token, so no organizationId query
// parameter is added.
func ProviderListPath(q ListQuery) string {
	return appendPagination(utils.AIWorkspaceLLMProvidersPath, q)
}

// ProxyListPath builds the llm-proxies list path with the projectId query
// parameter and optional pagination.
func ProxyListPath(projectID string, q ListQuery) string {
	return withProjectListParams(utils.AIWorkspaceLLMProxiesPath, projectID, q)
}

// MCPProxyListPath builds the mcp-proxies list path with the projectId query
// parameter and optional pagination.
func MCPProxyListPath(projectID string, q ListQuery) string {
	return withProjectListParams(utils.AIWorkspaceMCPProxiesPath, projectID, q)
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

// OutputJSON reports whether the requested output format is the full JSON body.
func OutputJSON(format string) bool {
	return strings.EqualFold(strings.TrimSpace(format), "json")
}

// PrintApplyResult prints the result of a create/edit (push/edit) operation in
// the same structured, key-value form as `ap gateway apply`, e.g.:
//
//	Status: success
//	Message: LlmProvider applied successfully
//	ID: wso2-claude-provider
//	Organization: 019f2324-...
//	Project: 019f2324-...
//	Created At: 2026-07-03T06:31:22Z
//	Updated At: 2026-07-03T06:31:22Z
//	State: deployed
//
// kind is the artifact kind ("LlmProvider", "LlmProxy", "Mcp"); action is the
// past-tense verb ("applied" for create, "updated" for edit). Organization /
// Project / Created At / Updated At / State are printed only when known.
// Organization comes from the response (the CLI derives it from the auth token,
// so it only shows when the server echoes organizationId); Project comes from
// the response or, failing that, the locally supplied fallbackProject
// (--project-id). When the output format is "json" the full response body is
// pretty-printed instead, so the command stays scriptable (e.g. `... -o json | jq`).
//
// fallbackID / fallbackProject are the values known locally (from the pushed
// payload / --project-id); they are used when the server response omits them.
func PrintApplyResult(resp *http.Response, outputFormat, kind, action, fallbackID, fallbackProject string) error {
	if OutputJSON(outputFormat) {
		return PrintJSONResponse(resp)
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var m map[string]interface{}
	_ = json.Unmarshal(body, &m)

	id := stringField(m, "id")
	if id == "" {
		id = strings.TrimSpace(fallbackID)
	}
	project := stringField(m, "projectId")
	if project == "" {
		project = strings.TrimSpace(fallbackProject)
	}

	fmt.Println("Status: success")
	fmt.Printf("Message: %s %s successfully\n", kind, action)
	if id != "" {
		fmt.Printf("ID: %s\n", id)
	}
	if v := stringField(m, "organizationId"); v != "" {
		fmt.Printf("Organization: %s\n", v)
	}
	if project != "" {
		fmt.Printf("Project: %s\n", project)
	}
	if v := stringField(m, "createdAt"); v != "" {
		fmt.Printf("Created At: %s\n", v)
	}
	if v := stringField(m, "updatedAt"); v != "" {
		fmt.Printf("Updated At: %s\n", v)
	}
	// The LLM provider/proxy and MCP proxy responses carry the deployment state
	// in the top-level "status" field (pending/deployed/failed).
	if v := stringField(m, "status"); v != "" {
		fmt.Printf("State: %s\n", v)
	}
	return nil
}

// stringField returns the trimmed string value of m[key], or "" when absent or
// not a string.
func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
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
