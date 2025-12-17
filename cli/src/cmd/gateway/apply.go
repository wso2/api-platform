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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	ApplyCmdLiteral = "apply"
	ApplyCmdExample = `# Apply a resource from a YAML file
apipctl gateway apply --file petstore-api.yaml
apipctl gateway apply -f petstore-api.yaml

# Apply a resource from a JSON file
apipctl gateway apply --file petstore-api.json`
)

var (
	applyFilePath string
)

var applyCmd = &cobra.Command{
	Use:     ApplyCmdLiteral,
	Short:   "Apply a resource to the gateway",
	Long:    "Create or update a gateway resource (API, MCP proxy, etc.) from a YAML or JSON file.",
	Example: ApplyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runApplyCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(applyCmd, utils.FlagFile, &applyFilePath, "", "Path to the resource file")
	applyCmd.MarkFlagRequired(utils.FlagFile)
}

// ResourceMetadata represents the metadata section of a resource
type ResourceMetadata struct {
	Name string `yaml:"name"`
}

// ResourceDefinition represents the basic structure of a resource file
type ResourceDefinition struct {
	Kind     string           `yaml:"kind"`
	Metadata ResourceMetadata `yaml:"metadata"`
}

func runApplyCommand() error {
	// Read the file
	fileContent, err := os.ReadFile(applyFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect if the content is JSON and convert to YAML if needed
	fileContent, err = utils.ConvertJSONToYAMLIfNeeded(fileContent)
	if err != nil {
		return fmt.Errorf("failed to process file content: %w", err)
	}

	// Parse the YAML to extract kind and metadata.name
	var resourceDef ResourceDefinition
	if err := yaml.Unmarshal(fileContent, &resourceDef); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if resourceDef.Kind == "" {
		return fmt.Errorf("'kind' field is required in the resource file")
	}
	if resourceDef.Metadata.Name == "" {
		return fmt.Errorf("'metadata.name' field is required in the resource file")
	}

	// Get the resource handler for this kind
	handler := gateway.GetResourceHandler(resourceDef.Kind)
	if handler == nil {
		return fmt.Errorf("unsupported resource kind: %s", resourceDef.Kind)
	}

	// Create a gateway client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	// Create the resource object
	resource := gateway.Resource{
		Kind:    resourceDef.Kind,
		Handle:  resourceDef.Metadata.Name,
		RawYAML: fileContent,
	}

	// Apply the resource (check if exists, then create or update)
	return applyResource(client, handler, resource)
}

func applyResource(client *gateway.Client, handler gateway.ResourceHandler, resource gateway.Resource) error {
	// Check if the resource already exists
	exists, err := resourceExists(client, handler, resource.Handle)
	if err != nil {
		return fmt.Errorf("failed to check resource existence: %w", err)
	}

	var resp *http.Response
	var operation string

	if exists {
		// Update existing resource
		operation = "update"
		endpoint := handler.UpdateEndpoint(resource.Handle)
		resp, err = client.PutYAML(endpoint, bytes.NewReader(resource.RawYAML))
	} else {
		// Create new resource
		operation = "create"
		endpoint := handler.CreateEndpoint()
		resp, err = client.PostYAML(endpoint, bytes.NewReader(resource.RawYAML))
	}

	if err != nil {
		return fmt.Errorf("failed to %s resource: %w", operation, err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if the operation was successful
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse error message from response
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if msg, ok := errorResp["message"].(string); ok {
				return fmt.Errorf("%s failed (status %d): %s", operation, resp.StatusCode, msg)
			}
			if msg, ok := errorResp["error"].(string); ok {
				return fmt.Errorf("%s failed (status %d): %s", operation, resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("%s failed (status %d): %s", operation, resp.StatusCode, string(body))
	}

	// Parse the success response
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		// If we can't parse JSON, just show success
		fmt.Println("Status: success")
		fmt.Printf("Message: %s applied successfully\n", resource.Kind)
		fmt.Printf("ID: %s\n", resource.Handle)
		return nil
	}

	// Display the response
	fmt.Println("Status: success")

	// Try to extract common fields
	if msg, ok := responseData["message"].(string); ok {
		fmt.Printf("Message: %s\n", msg)
	} else {
		if exists {
			fmt.Printf("Message: %s updated successfully\n", resource.Kind)
		} else {
			fmt.Printf("Message: %s applied successfully\n", resource.Kind)
		}
	}

	if id, ok := responseData["id"].(string); ok {
		fmt.Printf("ID: %s\n", id)
	} else {
		fmt.Printf("ID: %s\n", resource.Handle)
	}

	if createdAt, ok := responseData["createdAt"].(string); ok {
		fmt.Printf("Created At: %s\n", createdAt)
	} else if timestamp, ok := responseData["timestamp"].(string); ok {
		fmt.Printf("Timestamp: %s\n", timestamp)
	}

	return nil
}

func resourceExists(client *gateway.Client, handler gateway.ResourceHandler, handle string) (bool, error) {
	endpoint := handler.GetEndpoint(handle)
	resp, err := client.Get(endpoint)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 200 means it exists, 404 means it doesn't
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	// Any other status code is an error
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
}
