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

package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	GetCmdLiteral = "get"
	GetCmdExample = `# Get MCP by ID
apipctl gateway mcp get --id sample-1 --format yaml

# Get MCP by name and version
apipctl gateway mcp get --name my-mcp --version v1.0 --format json`
)

var (
	getMCPID      string
	getMCPName    string
	getMCPVersion string
	getMCPFormat  string
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a specific MCP from the gateway",
	Long:    "Retrieves a specific MCP by ID or by name and version, with optional output formatting.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagID, &getMCPID, "", "MCP ID (handle)")
	utils.AddStringFlag(getCmd, utils.FlagName, &getMCPName, "", "MCP name")
	utils.AddStringFlag(getCmd, utils.FlagVersion, &getMCPVersion, "", "MCP version")
	utils.AddStringFlag(getCmd, utils.FlagFormat, &getMCPFormat, "yaml", "Output format (json or yaml)")
}

// MCPGetResponse represents the response from GET /mcp-proxies/{id}
type MCPGetResponse struct {
	Status string `json:"status"`
	MCP    struct {
		ID            string                 `json:"id"`
		Configuration map[string]interface{} `json:"configuration"`
		Metadata      map[string]interface{} `json:"metadata"`
	} `json:"mcp"`
}

func runGetCommand() error {
	// Validate flags
	if getMCPID == "" && getMCPName == "" {
		return fmt.Errorf("either --id or --name (with --version) must be specified")
	}

	if getMCPID != "" && getMCPName != "" {
		return fmt.Errorf("cannot specify both --id and --name")
	}

	if getMCPName != "" && getMCPVersion == "" {
		return fmt.Errorf("--version is required when using --name")
	}

	// Validate format
	getMCPFormat = strings.ToLower(getMCPFormat)
	if getMCPFormat != "json" && getMCPFormat != "yaml" {
		return fmt.Errorf("invalid format: %s (must be 'json' or 'yaml')", getMCPFormat)
	}

	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	var mcpConfig map[string]interface{}

	if getMCPID != "" {
		// Get by ID
		mcpConfig, err = getMCPByID(client, getMCPID)
		if err != nil {
			return err
		}
	} else {
		// Get by name and version
		mcpConfig, err = getMCPByNameAndVersion(client, getMCPName, getMCPVersion)
		if err != nil {
			return err
		}
	}

	// Format and display the output
	return displayMCP(mcpConfig, getMCPFormat)
}

func getMCPByID(client *gateway.Client, id string) (map[string]interface{}, error) {
	resp, err := client.Get("/mcp-proxies/" + url.PathEscape(id))
	if err != nil {
		return nil, fmt.Errorf("failed to call /mcp-proxies/%s endpoint: %w", id, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("MCP with ID '%s' not found", id)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get MCP (status %d): %s", resp.StatusCode, string(body))
	}

	var getResp MCPGetResponse
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return getResp.MCP.Configuration, nil
}

func getMCPByNameAndVersion(client *gateway.Client, name, version string) (map[string]interface{}, error) {
	// Build query string
	query := url.Values{}
	query.Set("name", name)
	query.Set("version", version)

	resp, err := client.Get("/mcp-proxies?" + query.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to call /mcp-proxies endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get MCP (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp MCPListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if listResp.Count == 0 {
		return nil, fmt.Errorf("MCP with name '%s' and version '%s' not found", name, version)
	}

	if listResp.Count > 1 {
		return nil, fmt.Errorf("multiple MCPs found with name '%s' and version '%s' (found %d)", name, version, listResp.Count)
	}

	// Get the full MCP configuration using the ID
	return getMCPByID(client, listResp.MCPProxies[0].ID)
}

func displayMCP(mcpConfig map[string]interface{}, format string) error {
	var output []byte
	var err error

	switch format {
	case "json":
		output, err = json.MarshalIndent(mcpConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format as JSON: %w", err)
		}
	case "yaml":
		output, err = yaml.Marshal(mcpConfig)
		if err != nil {
			return fmt.Errorf("failed to format as YAML: %w", err)
		}
	}

	fmt.Println(string(output))
	return nil
}
