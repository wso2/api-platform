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

package graphqlapi

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
	GetCmdExample = `# Get GraphQL API by ID
ap gateway graphql-api get --id countries-graphql-api --format yaml

# Get GraphQL API by display name and version
ap gateway graphql-api get --display-name "Countries GraphQL API" --version v1.0 --format json`
)

var (
	getAPIID      string
	getAPIName    string
	getAPIVersion string
	getAPIFormat  string
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a specific GraphQL API from the gateway",
	Long:    "Retrieves a specific GraphQL API by ID or by display name and version, with optional output formatting.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(getCmd)
	utils.AddStringFlag(getCmd, utils.FlagID, &getAPIID, "", "GraphQL API ID (handle)")
	utils.AddStringFlag(getCmd, utils.FlagName, &getAPIName, "", "GraphQL API display name")
	utils.AddStringFlag(getCmd, utils.FlagVersion, &getAPIVersion, "", "GraphQL API version")
	utils.AddStringFlag(getCmd, utils.FlagFormat, &getAPIFormat, "yaml", "Output format (json or yaml)")
}

// APIGetResponse represents the response from GET /graphql-apis/{id}.
//
// Under the current management API the response body is the k8s-shaped resource
// itself: {apiVersion, kind, metadata, spec, status}. We keep this around as a
// convenience alias so callers can reason about the resource body shape.
type APIGetResponse map[string]interface{}

func runGetCommand(cmd *cobra.Command) error {
	// Validate flags
	if getAPIID == "" && getAPIName == "" {
		return fmt.Errorf("either --id or --display-name (with --version) must be specified")
	}

	if getAPIID != "" && getAPIName != "" {
		return fmt.Errorf("cannot specify both --id and --display-name")
	}

	if getAPIName != "" && getAPIVersion == "" {
		return fmt.Errorf("--version is required when using --display-name")
	}

	// Validate format
	getAPIFormat = strings.ToLower(getAPIFormat)
	if getAPIFormat != "json" && getAPIFormat != "yaml" {
		return fmt.Errorf("invalid format: %s (must be 'json' or 'yaml')", getAPIFormat)
	}

	// Create a client for the selected (or active) gateway
	client, err := gateway.NewClientFromCommand(cmd)
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
		// Get by display name and version
		apiConfig, err = getAPIByNameAndVersion(client, getAPIName, getAPIVersion)
		if err != nil {
			return err
		}
	}

	// Format and display the output
	return displayAPI(apiConfig, getAPIFormat)
}

func getAPIByID(client *gateway.Client, id string) (map[string]interface{}, error) {
	resp, err := client.Get(fmt.Sprintf(utils.GatewayGraphQLAPIByIDPath, url.PathEscape(id)))
	if err != nil {
		return nil, fmt.Errorf("failed to call %s endpoint: %w", fmt.Sprintf(utils.GatewayGraphQLAPIByIDPath, id), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("GraphQL API with ID '%s' not found", id)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get GraphQL API (status %d): %s", resp.StatusCode, string(body))
	}

	var getResp APIGetResponse
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// The response is the resource body itself. Drop the server-managed status
	// block so the display matches the declarative source the user applied.
	delete(getResp, "status")
	return getResp, nil
}

func getAPIByNameAndVersion(client *gateway.Client, name, version string) (map[string]interface{}, error) {
	// Build query string. The list endpoint filters on displayName/version.
	query := url.Values{}
	query.Set("displayName", name)
	query.Set("version", version)

	resp, err := client.Get(utils.GatewayGraphQLAPIsPath + "?" + query.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to call %s endpoint: %w", utils.GatewayGraphQLAPIsPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get GraphQL API (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp APIListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if listResp.Count == 0 {
		return nil, fmt.Errorf("GraphQL API with display name '%s' and version '%s' not found", name, version)
	}

	if listResp.Count > 1 {
		return nil, fmt.Errorf("multiple GraphQL APIs found with display name '%s' and version '%s' (found %d)", name, version, listResp.Count)
	}

	// Get the full API configuration using the ID
	return getAPIByID(client, listResp.GraphQLAPIs[0].ID())
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
