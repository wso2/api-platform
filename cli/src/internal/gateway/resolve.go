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

package gateway

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

// ResolveGateway returns the gateway selected by an optional name within an
// optional platform, falling back to the active gateway and/or the active
// platform when either is empty. It mirrors devportal.ResolveDevPortal so the
// two command families share the same selection semantics. The resolved
// platform name is returned alongside the gateway for display purposes.
func ResolveGateway(cfg *config.Config, selectedName, selectedPlatform string) (*config.Gateway, string, error) {
	resolvedPlatform := cfg.ResolvePlatform(selectedPlatform)
	selectedName = strings.TrimSpace(selectedName)

	if selectedName != "" {
		gw, err := cfg.GetGatewayFromPlatform(resolvedPlatform, selectedName)
		if err != nil {
			return nil, "", err
		}
		return gw, resolvedPlatform, nil
	}

	gw, err := cfg.GetActiveGatewayFromPlatform(resolvedPlatform)
	if err != nil {
		return nil, "", err
	}
	return gw, resolvedPlatform, nil
}

// NewClientForSelection builds a client for the optional platform/gateway
// selection, defaulting to the active platform and active gateway.
func NewClientForSelection(selectedPlatform, selectedName string) (*Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	gw, _, err := ResolveGateway(cfg, selectedName, selectedPlatform)
	if err != nil {
		return nil, err
	}
	return NewClient(gw), nil
}

// AddSelectionFlags registers the standard --platform and --gateway selection
// flags on a command. Resolve them at run time with NewClientFromCommand.
func AddSelectionFlags(cmd *cobra.Command) {
	cmd.Flags().String(utils.FlagPlatform, "", "Platform name (defaults to the active platform)")
	cmd.Flags().String(utils.FlagGateway, "", "Gateway display name (defaults to the active gateway for the platform)")
}

// NewClientFromCommand builds a client from a command's --platform and
// --gateway flags, defaulting to the active platform and active gateway. The
// command must have registered the flags via AddSelectionFlags.
func NewClientFromCommand(cmd *cobra.Command) (*Client, error) {
	platform, _ := cmd.Flags().GetString(utils.FlagPlatform)
	name, _ := cmd.Flags().GetString(utils.FlagGateway)
	return NewClientForSelection(platform, name)
}
