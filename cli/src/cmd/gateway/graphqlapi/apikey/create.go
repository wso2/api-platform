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

package apikey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	// kindApiKey is the CR kind accepted by the create command.
	kindApiKey = "ApiKey"
	// parentKindGraphQLApi is the only parentRef.kind supported by this command,
	// which targets the /graphql-apis/{id}/api-keys management endpoint.
	parentKindGraphQLApi = "GraphQLApi"
)

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Generate an API key from a CR file
ap gateway graphql-api api-key create --file api-key.yaml
ap gateway graphql-api api-key create -f api-key.json

# The file is an ApiKey custom resource, e.g.:
#   apiVersion: gateway.api-platform.wso2.com/v1
#   kind: ApiKey
#   metadata:
#     name: countries-key-acme
#   spec:
#     parentRef:
#       kind: GraphQLApi
#       name: countries-graphql-api
#     expiresIn:
#       duration: 30
#       unit: days`
)

var createFilePath string

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Generate an API key for a GraphQL API",
	Long:    "Generates a new API key from an ApiKey custom resource file (YAML or JSON). The parent GraphQL API is taken from spec.parentRef.name and the key name from metadata.name. The plaintext key is returned once in the response.",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(createCmd)
	utils.AddStringFlag(createCmd, utils.FlagFile, &createFilePath, "", "Path to the ApiKey CR file (YAML or JSON)")
	createCmd.MarkFlagRequired(utils.FlagFile)
}

func runCreateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(createFilePath) == "" {
		return fmt.Errorf("--%s is required", utils.FlagFile)
	}

	cr, err := gateway.ParseResourceCR(createFilePath, kindApiKey)
	if err != nil {
		return err
	}

	// The parent GraphQL API id comes from spec.parentRef.name; the key name from
	// metadata.name. Everything else in the spec is forwarded as the request body.
	apiID, err := graphQLAPIParentName(cr)
	if err != nil {
		return err
	}

	body := map[string]interface{}{}
	for k, v := range cr.Spec {
		if k == "parentRef" {
			continue
		}
		body[k] = v
	}
	body["name"] = cr.Metadata.Name

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to build API key payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	// Client.Post already treats any non-2xx status as an error (via
	// formatHTTPError) and returns a nil *http.Response in that case, so there
	// is no status code left to branch on once err is nil.
	endpoint := fmt.Sprintf(utils.GatewayGraphQLAPIKeysPath, url.PathEscape(apiID))
	resp, err := client.Post(endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	fmt.Printf("API key %q generated successfully.\n", cr.Metadata.Name)
	return gateway.PrintJSONResponse(resp)
}

// graphQLAPIParentName extracts and validates spec.parentRef.name, requiring the
// parent kind to be GraphQLApi (or unset) since this command targets the GraphQL
// API api-key endpoint.
func graphQLAPIParentName(cr *gateway.ResourceCR) (string, error) {
	parentRef, ok := cr.Spec["parentRef"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid %s: spec.parentRef is required", kindApiKey)
	}

	if kind, ok := parentRef["kind"].(string); ok && strings.TrimSpace(kind) != "" && kind != parentKindGraphQLApi {
		return "", fmt.Errorf("unsupported spec.parentRef.kind %q: 'ap gateway graphql-api api-key' only supports %s", kind, parentKindGraphQLApi)
	}

	name, ok := parentRef["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("invalid %s: spec.parentRef.name is required", kindApiKey)
	}

	return strings.TrimSpace(name), nil
}
