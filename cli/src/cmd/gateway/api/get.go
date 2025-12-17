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

package api

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
	GetCmdExample = `# Get API by ID
apipctl gateway api get --id sample-1 --format yaml

# Get API by name and version
apipctl gateway api get --name "PetStore API" --version v1.0 --format json`
)

var (
	getAPIID      string
	getAPIName    string
	getAPIVersion string
	getAPIFormat  string
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a specific API from the gateway",
	Long:    "Retrieves a specific API by ID or by name and version, with optional output formatting.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagID, &getAPIID, "", "API ID (handle)")
	utils.AddStringFlag(getCmd, utils.FlagName, &getAPIName, "", "API name")
	utils.AddStringFlag(getCmd, utils.FlagVersion, &getAPIVersion, "", "API version")
	utils.AddStringFlag(getCmd, utils.FlagFormat, &getAPIFormat, "yaml", "Output format (json or yaml)")
}

// APIGetResponse represents the response from GET /apis/{id}
type APIGetResponse struct {
	Status string `json:"status"`
	API    struct {
		ID            string                 `json:"id"`
		Configuration map[string]interface{} `json:"configuration"`
		Metadata      map[string]interface{} `json:"metadata"`
	} `json:"api"`
}

func runGetCommand() error {
	// Validate flags
	if getAPIID == "" && getAPIName == "" {
		return fmt.Errorf("either --id or --name (with --version) must be specified")
	}

	if getAPIID != "" && getAPIName != "" {
		return fmt.Errorf("cannot specify both --id and --name")
	}

	if getAPIName != "" && getAPIVersion == "" {
		return fmt.Errorf("--version is required when using --name")
	}

	// Validate format
	getAPIFormat = strings.ToLower(getAPIFormat)
	if getAPIFormat != "json" && getAPIFormat != "yaml" {
		return fmt.Errorf("invalid format: %s (must be 'json' or 'yaml')", getAPIFormat)
	}

	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	var apiConfig map[string]interface{}

	if getAPIID != "" {
		// Get by ID
		apiConfig, err = getAPIByID(client, getAPIID)
		if err != nil {
			return err
		}
	} else {
		// Get by name and version
		apiConfig, err = getAPIByNameAndVersion(client, getAPIName, getAPIVersion)
		if err != nil {
			return err
		}
	}

	// Format and display the output
	return displayAPI(apiConfig, getAPIFormat)
}

func getAPIByID(client *gateway.Client, id string) (map[string]interface{}, error) {
	resp, err := client.Get("/apis/" + url.PathEscape(id))
	if err != nil {
		return nil, fmt.Errorf("failed to call /apis/%s endpoint: %w", id, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("API with ID '%s' not found", id)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get API (status %d): %s", resp.StatusCode, string(body))
	}

	var getResp APIGetResponse
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return getResp.API.Configuration, nil
}

func getAPIByNameAndVersion(client *gateway.Client, name, version string) (map[string]interface{}, error) {
	// Build query string
	query := url.Values{}
	query.Set("name", name)
	query.Set("version", version)

	resp, err := client.Get("/apis?" + query.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to call /apis endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get API (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp APIListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if listResp.Count == 0 {
		return nil, fmt.Errorf("API with name '%s' and version '%s' not found", name, version)
	}

	if listResp.Count > 1 {
		return nil, fmt.Errorf("multiple APIs found with name '%s' and version '%s' (found %d)", name, version, listResp.Count)
	}

	// Get the full API configuration using the ID
	return getAPIByID(client, listResp.APIs[0].ID)
}

func displayAPI(apiConfig map[string]interface{}, format string) error {
	var output []byte
	var err error

	switch format {
	case "json":
		output, err = json.MarshalIndent(apiConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format as JSON: %w", err)
		}
	case "yaml":
		output, err = yaml.Marshal(apiConfig)
		if err != nil {
			return fmt.Errorf("failed to format as YAML: %w", err)
		}
	}

	fmt.Println(string(output))
	return nil
}
