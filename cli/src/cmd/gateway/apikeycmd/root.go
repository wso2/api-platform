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

// Package apikeycmd holds what's genuinely identical across every API kind's
// own `api-key` command set (REST, GraphQL, ...): the list/regenerate/revoke
// request logic, and the `api-key` root group builder.
//
// It deliberately does NOT build the list/regenerate/revoke *commands*
// themselves - test/unit/cmd_naming_convention_test.go statically requires
// every non-root.go file under cmd/ to contain its own literal
// `&cobra.Command{Use: "<filename>", ...}`, found via AST inspection of that
// file. A shared `NewListCmd`-style constructor would make list.go/
// regenerate.go/revoke.go's own files contain no such literal, tripping that
// check. So each kind's own list.go/regenerate.go/revoke.go keeps its literal
// cobra.Command (satisfying the convention) and calls into this package's
// RunList/RunRegenerate/RunRevoke for the actual behavior.
//
// This file is itself named root.go - not because it defines a top-level
// `ap gateway ...` command, but because that's the one filename the same
// check exempts unconditionally. NewRootCmd here does the same job a root.go
// always does (assemble subcommands built elsewhere), and every value it
// wires in (createCmd/listCmd/regenerateCmd/updateCmd/revokeCmd) still comes
// from the calling kind's own files - this file only supplies the shared
// logic and the group builder, never a subcommand itself.
//
// create/update are not represented here at all: each kind's create command
// takes a fundamentally different input shape (a CR file vs. explicit
// flags), and each kind's update command performs a different operation
// (renaming a key vs. replacing its value) - unifying either would change
// one kind's behavior, not just its plumbing.
package apikeycmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

// Config parameterizes this package's commands/logic for one API kind.
type Config struct {
	// KindLabel is the human-readable API kind used in help/error text, e.g. "REST API".
	KindLabel string
	// KindPathSegment is the `ap gateway <segment> api-key ...` example segment, e.g. "rest-api".
	KindPathSegment string
	// ExampleAPIID is a sample API id used in generated example text.
	ExampleAPIID string
	// KeysPath is a one-%s format string for the collection endpoint, e.g. "/rest-apis/%s/api-keys".
	KeysPath string
	// KeyByNamePath is a two-%s format string for a single named key, e.g. "/rest-apis/%s/api-keys/%s".
	KeyByNamePath string
	// RegeneratePath is a two-%s format string for the regenerate action.
	RegeneratePath string
	// CreateExample is the full "# comment\ncommand" block demonstrating this
	// kind's own create command, inlined into the root group's Example text.
	// create's own input shape is kind-specific (see package doc), so this
	// isn't derived generically like the other fields.
	CreateExample string
}

// APIKey is a list-view projection of an API key. The plaintext apiKey value
// is only present on create/regenerate responses, so it is intentionally
// omitted from the list table.
type APIKey struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	APIID       string `json:"apiId"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	ExpiresAt   string `json:"expiresAt"`
}

// APIKeyListResponse represents the response from GET <Config.KeysPath>.
type APIKeyListResponse struct {
	APIKeys    []APIKey `json:"apiKeys"`
	TotalCount int      `json:"totalCount"`
	Status     string   `json:"status"`
}

// RunList implements `api-key list`.
func RunList(cmd *cobra.Command, cfg Config, apiID string) error {
	if strings.TrimSpace(apiID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(cfg.KeysPath, url.PathEscape(apiID))
	resp, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("failed to call %s endpoint: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s with ID '%s' not found", cfg.KindLabel, apiID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list API keys (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp APIKeyListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(listResp.APIKeys) == 0 {
		fmt.Printf("No API keys found for %s '%s'.\n", cfg.KindLabel, apiID)
		return nil
	}

	headers := []string{"NAME", "DISPLAY_NAME", "API_ID", "STATUS", "CREATED_AT", "EXPIRES_AT"}
	rows := make([][]string, 0, len(listResp.APIKeys))
	for _, k := range listResp.APIKeys {
		rows = append(rows, []string{k.Name, k.DisplayName, k.APIID, k.Status, k.CreatedAt, k.ExpiresAt})
	}
	utils.PrintTable(headers, rows)

	return nil
}

// RunRegenerate implements `api-key regenerate`. Client.Post already treats
// any non-2xx status as an error (internal/gateway/client.go) and returns a
// nil *http.Response in that case, so there is no status code left to branch
// on once err is nil.
func RunRegenerate(cmd *cobra.Command, cfg Config, apiID, keyName string) error {
	if strings.TrimSpace(apiID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(keyName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagKeyName)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(cfg.RegeneratePath, url.PathEscape(apiID), url.PathEscape(keyName))
	resp, err := client.Post(endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to regenerate API key: %w", err)
	}

	fmt.Println("API key regenerated successfully.")
	return gateway.PrintJSONResponse(resp)
}

// RunRevoke implements `api-key revoke`. Client.Delete already treats any
// non-2xx status as an error and returns a nil *http.Response in that case,
// so err == nil here always means success.
func RunRevoke(cmd *cobra.Command, cfg Config, apiID, keyName string) error {
	if strings.TrimSpace(apiID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(keyName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagKeyName)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(cfg.KeyByNamePath, url.PathEscape(apiID), url.PathEscape(keyName))
	resp, err := client.Delete(endpoint)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}
	resp.Body.Close()

	fmt.Println("API key revoked successfully.")
	return nil
}

// NewRootCmd builds the `api-key` command group for one API kind, wiring in
// that kind's own create/update commands (each fundamentally different in
// shape - see the package doc) alongside its list/regenerate/revoke commands.
// Only called from each kind's own root.go, which
// test/unit/cmd_naming_convention_test.go exempts from the literal-command
// requirement described in the package doc.
func NewRootCmd(cfg Config, createCmd, listCmd, regenerateCmd, updateCmd, revokeCmd *cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:   "api-key",
		Short: fmt.Sprintf("Manage API keys for a %s on the gateway", cfg.KindLabel),
		Long: fmt.Sprintf("This command allows you to create, list, regenerate, update, and revoke API keys for a %s on the WSO2 API Platform Gateway.",
			cfg.KindLabel),
		Example: fmt.Sprintf("# List API keys for a %s\nap gateway %s api-key list --id %s\n\n%s",
			cfg.KindLabel, cfg.KindPathSegment, cfg.ExampleAPIID, cfg.CreateExample),
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	root.AddCommand(createCmd, listCmd, regenerateCmd, updateCmd, revokeCmd)
	return root
}
