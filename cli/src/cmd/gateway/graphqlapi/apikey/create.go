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

// validExpiresInUnits mirrors gateway-controller's
// APIKeyCreationRequestExpiresInUnit enum (pkg/utils/api_key.go) — the only
// units the server accepts for expiresIn.unit.
var validExpiresInUnits = map[string]bool{
	"seconds": true,
	"minutes": true,
	"hours":   true,
	"days":    true,
	"weeks":   true,
	"months":  true,
}

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Generate an API key with an auto-generated name that never expires
ap gateway graphql-api api-key create --id countries-graphql-api

# Generate a named API key that expires in 30 days
ap gateway graphql-api api-key create --id countries-graphql-api --name my-production-key --expires-in-duration 30 --expires-in-unit days`
)

var (
	createAPIID             string
	createName              string
	createExpiresInDuration int
	createExpiresInUnit     string
)

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Generate an API key for a GraphQL API",
	Long:    "Generates a new API key for a GraphQL API. --name is optional — if omitted, the server generates a unique name. --expires-in-duration and --expires-in-unit must be supplied together to set an expiry; omit both for a key that never expires. The plaintext key is returned once in the response.",
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
	utils.AddStringFlag(createCmd, utils.FlagID, &createAPIID, "", "GraphQL API ID (required)")
	utils.AddStringFlag(createCmd, utils.FlagPropertyName, &createName, "", "Name for the API key. Omit to let the server generate a unique name.")
	utils.AddIntFlag(createCmd, utils.FlagExpiresInDuration, &createExpiresInDuration, 0, "Expiry duration; must be paired with --expires-in-unit. Omit both for a key that never expires.")
	utils.AddStringFlag(createCmd, utils.FlagExpiresInUnit, &createExpiresInUnit, "", "Expiry duration unit: seconds, minutes, hours, days, weeks, or months. Must be paired with --expires-in-duration.")
	createCmd.MarkFlagRequired(utils.FlagID)
}

func runCreateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(createAPIID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	// A duration of 0 / an empty unit both mean "not set" - there is no
	// meaningful key that expires in 0 seconds, so treating either as unset
	// requires the pair to be supplied together rather than one silently
	// defaulting the other.
	durationSet := createExpiresInDuration != 0
	unitSet := strings.TrimSpace(createExpiresInUnit) != ""
	if durationSet != unitSet {
		return fmt.Errorf("--%s and --%s must be provided together", utils.FlagExpiresInDuration, utils.FlagExpiresInUnit)
	}

	body := map[string]interface{}{}
	if name := strings.TrimSpace(createName); name != "" {
		body["name"] = name
	}
	if durationSet {
		unit := strings.ToLower(strings.TrimSpace(createExpiresInUnit))
		if !validExpiresInUnits[unit] {
			return fmt.Errorf("invalid --%s %q: must be one of seconds, minutes, hours, days, weeks, months", utils.FlagExpiresInUnit, createExpiresInUnit)
		}
		body["expiresIn"] = map[string]interface{}{
			"duration": createExpiresInDuration,
			"unit":     unit,
		}
	}

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
	endpoint := fmt.Sprintf(utils.GatewayGraphQLAPIKeysPath, url.PathEscape(createAPIID))
	resp, err := client.Post(endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	fmt.Println("API key generated successfully.")
	return gateway.PrintJSONResponse(resp)
}
