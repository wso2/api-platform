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

package it

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// RegisterLLMSteps registers all LLM provider template step definitions
func RegisterLLMSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	ctx.Step(`^I create this LLM provider template:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPOSTToService("gateway-controller", "/llm-provider-templates", body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I retrieve the LLM provider template "([^"]*)"$`, func(templateID string) error {
		return httpSteps.SendGETToService("gateway-controller", "/llm-provider-templates/"+templateID)
	})

	ctx.Step(`^I update the LLM provider template "([^"]*)" with:$`, func(templateID string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPUTToService("gateway-controller", "/llm-provider-templates/"+templateID, body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I delete the LLM provider template "([^"]*)"$`, func(templateID string) error {
		err := httpSteps.SendDELETEToService("gateway-controller", "/llm-provider-templates/"+templateID)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I list all LLM provider templates$`, func() error {
		return httpSteps.SendGETToService("gateway-controller", "/llm-provider-templates")
	})

	ctx.Step(`^I list LLM provider templates with filter "([^"]*)" as "([^"]*)"$`, func(filterKey, filterValue string) error {
		return httpSteps.SendGETToService("gateway-controller", "/llm-provider-templates?"+filterKey+"="+filterValue)
	})

	ctx.Step(`^the response should contain oob-templates$`, func() error {
		// This step verifies that out-of-box templates are present in the list response
		// The actual assertion is done by checking the response body
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body for oob-templates assertion")
		}
		// The actual validation of OOB templates should be done using JSON assertions
		// in the feature file itself, so this step just ensures we got a response
		var response struct {
			Count     int `json:"count"`
			Templates []struct {
				ID string `json:"id"`
			} `json:"templates"`
		}

		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return fmt.Errorf("failed to parse response JSON: %w", err)
		}

		// 1️⃣ Expected OOB template IDs
		expectedIDs := []string{
			"azureai-foundry",
			"anthropic",
			"openai",
			"gemini",
			"azure-openai",
			"mistralai",
			"awsbedrock",
		}

		// 2️⃣ Validate count is at least the expected set
		expectedCount := len(expectedIDs)
		if response.Count < expectedCount {
			return fmt.Errorf(
				"expected template count to be >= %d, but got %d",
				expectedCount,
				response.Count,
			)
		}

		// 3️⃣ Collect actual template IDs
		actualIDs := make(map[string]bool)
		for _, t := range response.Templates {
			actualIDs[t.ID] = true
		}

		// 4️⃣ Validate all expected IDs are present
		for _, expectedID := range expectedIDs {
			if !actualIDs[expectedID] {
				return fmt.Errorf(
					"expected oob-template with id '%s' was not found in response",
					expectedID,
				)
			}
		}

		return nil
	})

	// Lazy resource assertion steps for config_dump
	ctx.Step(`^the JSON response field "([^"]*)" should be greater than (\d+)$`, func(field string, expected int) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		value, err := getJSONFieldValue(body, field)
		if err != nil {
			return err
		}

		// JSON numbers are float64
		switch v := value.(type) {
		case float64:
			if int(v) <= expected {
				return fmt.Errorf("expected JSON field %q to be greater than %d, got %v", field, expected, int(v))
			}
		case int:
			if v <= expected {
				return fmt.Errorf("expected JSON field %q to be greater than %d, got %d", field, expected, v)
			}
		default:
			return fmt.Errorf("expected JSON field %q to be a number, got %T", field, value)
		}
		return nil
	})

	ctx.Step(`^the lazy resources should contain template "([^"]*)" of type "([^"]*)"$`, func(templateID, resourceType string) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		return assertLazyResourceExists(body, templateID, resourceType)
	})

	ctx.Step(`^the lazy resources should not contain template "([^"]*)"$`, func(templateID string) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		return assertLazyResourceNotExists(body, templateID)
	})

	ctx.Step(`^the lazy resource "([^"]*)" should have display name "([^"]*)"$`, func(templateID, expectedDisplayName string) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		return assertLazyResourceDisplayName(body, templateID, expectedDisplayName)
	})

	// LLM Provider CRUD steps
	ctx.Step(`^I create this LLM provider:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPOSTToService("gateway-controller", "/llm-providers", body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I update the LLM provider "([^"]*)" with:$`, func(providerID string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPUTToService("gateway-controller", "/llm-providers/"+providerID, body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I delete the LLM provider "([^"]*)"$`, func(providerID string) error {
		err := httpSteps.SendDELETEToService("gateway-controller", "/llm-providers/"+providerID)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	// Generic lazy resource assertions (for both templates and provider mappings)
	ctx.Step(`^the lazy resources should contain resource "([^"]*)" of type "([^"]*)"$`, func(resourceID, resourceType string) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		return assertLazyResourceExists(body, resourceID, resourceType)
	})

	ctx.Step(`^the lazy resources should not contain resource "([^"]*)"$`, func(resourceID string) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		return assertLazyResourceNotExists(body, resourceID)
	})

	// Provider template mapping assertion
	ctx.Step(`^the provider template mapping "([^"]*)" should map to template "([^"]*)"$`, func(providerName, expectedTemplate string) error {
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return fmt.Errorf("expected non-empty response body")
		}

		return assertProviderTemplateMappingTemplate(body, providerName, expectedTemplate)
	})
}

// ConfigDumpResponse represents the policy engine config dump response structure
type ConfigDumpResponse struct {
	LazyResources LazyResourcesDump `json:"lazy_resources"`
}

// LazyResourcesDump represents the lazy resources section of config dump
type LazyResourcesDump struct {
	TotalResources  int                           `json:"total_resources"`
	ResourcesByType map[string][]LazyResourceInfo `json:"resources_by_type"`
}

// LazyResourceInfo represents a single lazy resource
type LazyResourceInfo struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

// getJSONFieldValue extracts a field from JSON body (supports dot notation)
func getJSONFieldValue(body []byte, field string) (interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	parts := strings.Split(field, ".")
	current := interface{}(data)

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map at %q but got %T", part, current)
		}
		v, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("key %q does not exist in JSON", part)
		}
		current = v
	}

	return current, nil
}

// assertLazyResourceExists checks if a lazy resource with the given ID and type exists in the config dump
func assertLazyResourceExists(body []byte, templateID, resourceType string) error {
	var response ConfigDumpResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse config dump JSON: %w", err)
	}

	resources, exists := response.LazyResources.ResourcesByType[resourceType]
	if !exists {
		return fmt.Errorf("resource type %q not found in lazy resources. Available types: %v",
			resourceType, getResourceTypes(response.LazyResources.ResourcesByType))
	}

	for _, resource := range resources {
		if resource.ID == templateID {
			return nil
		}
	}

	// Collect all resource IDs for better error message
	var resourceIDs []string
	for _, r := range resources {
		resourceIDs = append(resourceIDs, r.ID)
	}

	return fmt.Errorf("template %q not found in lazy resources of type %q. Available resources: %v",
		templateID, resourceType, resourceIDs)
}

// assertLazyResourceNotExists checks that a lazy resource with the given ID does not exist
func assertLazyResourceNotExists(body []byte, templateID string) error {
	var response ConfigDumpResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse config dump JSON: %w", err)
	}

	for resourceType, resources := range response.LazyResources.ResourcesByType {
		for _, resource := range resources {
			if resource.ID == templateID {
				return fmt.Errorf("template %q should not exist but was found in lazy resources of type %q",
					templateID, resourceType)
			}
		}
	}

	return nil
}

// assertLazyResourceDisplayName checks that a lazy resource has the expected display name
func assertLazyResourceDisplayName(body []byte, templateID, expectedDisplayName string) error {
	var response ConfigDumpResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse config dump JSON: %w", err)
	}

	for _, resources := range response.LazyResources.ResourcesByType {
		for _, resource := range resources {
			if resource.ID == templateID {
				// The resource.Resource contains the template spec
				// Navigate to spec.displayName
				spec, ok := resource.Resource["spec"].(map[string]interface{})
				if !ok {
					return fmt.Errorf("resource %q does not have a valid spec field", templateID)
				}
				displayName, ok := spec["displayName"].(string)
				if !ok {
					return fmt.Errorf("resource %q does not have a valid displayName field in spec", templateID)
				}
				if displayName != expectedDisplayName {
					return fmt.Errorf("expected display name %q for resource %q, got %q",
						expectedDisplayName, templateID, displayName)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("template %q not found in lazy resources", templateID)
}

// getResourceTypes returns a slice of resource type names from the map
func getResourceTypes(resourcesByType map[string][]LazyResourceInfo) []string {
	types := make([]string, 0, len(resourcesByType))
	for t := range resourcesByType {
		types = append(types, t)
	}
	return types
}

// assertProviderTemplateMappingTemplate checks that a ProviderTemplateMapping resource maps to the expected template
func assertProviderTemplateMappingTemplate(body []byte, providerName, expectedTemplate string) error {
	var response ConfigDumpResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse config dump JSON: %w", err)
	}

	resources, exists := response.LazyResources.ResourcesByType["ProviderTemplateMapping"]
	if !exists {
		return fmt.Errorf("ProviderTemplateMapping resource type not found in lazy resources. Available types: %v",
			getResourceTypes(response.LazyResources.ResourcesByType))
	}

	for _, resource := range resources {
		if resource.ID == providerName {
			// The resource.Resource contains provider_name and template_handle
			templateHandle, ok := resource.Resource["template_handle"].(string)
			if !ok {
				return fmt.Errorf("resource %q does not have a valid template_handle field", providerName)
			}
			if templateHandle != expectedTemplate {
				return fmt.Errorf("expected provider %q to map to template %q, but got %q",
					providerName, expectedTemplate, templateHandle)
			}
			return nil
		}
	}

	// Collect all provider names for better error message
	var providerNames []string
	for _, r := range resources {
		providerNames = append(providerNames, r.ID)
	}

	return fmt.Errorf("provider %q not found in ProviderTemplateMapping resources. Available providers: %v",
		providerName, providerNames)
}
