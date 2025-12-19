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
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	HealthCmdLiteral = "health"
	HealthCmdExample = `# Check health of the current gateway
ap gateway health`
)

var healthCmd = &cobra.Command{
	Use:     HealthCmdLiteral,
	Short:   "Check the health status of the current gateway",
	Long:    "Returns the health status of the currently active gateway by calling its /health endpoint.",
	Example: HealthCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runHealthCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runHealthCommand() error {
	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	// Call the health endpoint
	resp, err := client.Get(utils.GatewayHealthPath)
	if err != nil {
		return fmt.Errorf("failed to call health endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if the response is successful
	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse the response as JSON to extract relevant fields
	var healthData map[string]interface{}
	if err := json.Unmarshal(body, &healthData); err != nil {
		// If not JSON, print raw response
		fmt.Println("Gateway Status: unknown")
		fmt.Printf("Response: %s\n", string(body))
		return nil
	}

	// Extract and display relevant information
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

	// Display the health information
	fmt.Printf("Gateway Status: %s\n", status)
	if timestamp != "" {
		fmt.Printf("Timestamp: %s\n", timestamp)
	}
	if message != "" {
		fmt.Printf("%s\n", message)
	} else {
		// Default message based on status
		if status == "healthy" || status == "up" {
			fmt.Println("Gateway is healthy.")
		} else if status == "unhealthy" || status == "down" {
			fmt.Println("Gateway is unhealthy.")
		} else {
			fmt.Println("Gateway health status is unclear.")
		}
	}

	return nil
}
