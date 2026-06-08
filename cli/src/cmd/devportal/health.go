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

package devportal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	HealthCmdLiteral = "health"
	HealthCmdExample = `# Check health of the current devportal
ap devportal health`
)

var healthCmd = &cobra.Command{
	Use:     HealthCmdLiteral,
	Short:   "Check the health status of the current devportal",
	Long:    "Returns the health status of the currently active devportal by calling its /health endpoint.",
	Example: HealthCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runHealthCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var healthPlatform string

func init() {
	utils.AddStringFlag(healthCmd, utils.FlagPlatform, &healthPlatform, "", "Platform name")
}

func runHealthCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	resolvedPlatform := cfg.ResolvePlatform(healthPlatform)

	client, err := internaldevportal.NewClientForActivePlatform(resolvedPlatform)
	if err != nil {
		return err
	}

	resp, err := client.Get(utils.DevPortalHealthPath)
	if err != nil {
		return fmt.Errorf("failed to call health endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	var healthData map[string]interface{}
	if err := json.Unmarshal(body, &healthData); err != nil {
		return fmt.Errorf("failed to parse health response: %w\nResponse: %s", err, string(body))
	}

	status := "unknown"
	timestamp := ""
	message := ""

	if val, ok := healthData["status"].(string); ok {
		status = val
	}
	if val, ok := healthData["timestamp"].(string); ok {
		timestamp = val
	}
	if val, ok := healthData["message"].(string); ok {
		message = val
	}

	fmt.Printf("DevPortal Status: %s\n", status)
	if timestamp != "" {
		fmt.Printf("Timestamp: %s\n", timestamp)
	}
	if message != "" {
		fmt.Printf("%s\n", message)
	} else {
		if status == "healthy" || status == "up" {
			fmt.Println("DevPortal is healthy.")
		} else if status == "unhealthy" || status == "down" {
			fmt.Println("DevPortal is unhealthy.")
		} else {
			fmt.Println("DevPortal health status is unclear.")
		}
	}

	return nil
}
