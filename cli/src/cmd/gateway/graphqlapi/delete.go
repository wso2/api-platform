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
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete a GraphQL API by ID
ap gateway graphql-api delete --id countries-graphql-api`
)

var (
	deleteAPIID string
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete a GraphQL API from the gateway",
	Long:    "Deletes a specific GraphQL API from the gateway by ID.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(deleteCmd)
	utils.AddStringFlag(deleteCmd, utils.FlagID, &deleteAPIID, "", "GraphQL API ID (handle) to delete")
	deleteCmd.MarkFlagRequired(utils.FlagID)
}

func runDeleteCommand(cmd *cobra.Command) error {
	// Proceed with deletion (no confirm flag required)

	// Create a client for the active gateway
	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	// Call the DELETE endpoint. Client.Delete already treats any non-2xx status
	// as an error (formatted via formatHTTPError, including the status code and
	// response body) and returns a nil *http.Response in that case - so there is
	// no status code left to branch on below; a 404 surfaces through err here.
	resp, err := client.Delete(fmt.Sprintf(utils.GatewayGraphQLAPIByIDPath, url.PathEscape(deleteAPIID)))
	if err != nil {
		return fmt.Errorf("failed to delete GraphQL API: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("GraphQL API deleted successfully.")
	return nil
}
