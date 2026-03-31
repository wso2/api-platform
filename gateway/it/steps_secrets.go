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

package it

import (
	"encoding/json"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// secretResponse holds the last secret operation response for state management
type secretResponse struct {
	name   string
	exists bool
}

// SecretSteps provides step definitions for secret management
type SecretSteps struct {
	state      *TestState
	httpSteps  *steps.HTTPSteps
	lastSecret *secretResponse
}

// NewSecretSteps creates a new SecretSteps instance
func NewSecretSteps(state *TestState, httpSteps *steps.HTTPSteps) *SecretSteps {
	return &SecretSteps{
		state:      state,
		httpSteps:  httpSteps,
		lastSecret: &secretResponse{},
	}
}

// Reset clears the secret steps state between scenarios
func (s *SecretSteps) Reset() {
	s.lastSecret = &secretResponse{}
}

// RegisterSecretSteps registers all secret management step definitions
func RegisterSecretSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	secretSteps := NewSecretSteps(state, httpSteps)

	// Create secret steps
	ctx.Step(`^I create a secret with the following configuration:$`, secretSteps.createSecret)
	ctx.Step(`^I create a secret named "([^"]*)" with value "([^"]*)"$`, secretSteps.createSecretWithValue)

	// Get/List secret steps
	ctx.Step(`^I get the secret "([^"]*)"$`, secretSteps.getSecret)
	ctx.Step(`^I list all secrets$`, secretSteps.listSecrets)

	// Update secret steps
	ctx.Step(`^I update the secret "([^"]*)" with the following configuration:$`, secretSteps.updateSecret)
	ctx.Step(`^I update the secret "([^"]*)" with value "([^"]*)"$`, secretSteps.updateSecretWithValue)

	// Delete secret steps
	ctx.Step(`^I delete the secret "([^"]*)"$`, secretSteps.deleteSecret)
}

// createSecret creates a secret with the provided JSON configuration
func (s *SecretSteps) createSecret(body *godog.DocString) error {
	s.httpSteps.SetHeader("Content-Type", "application/json")
	err := s.httpSteps.SendPOSTToService("gateway-controller", "/secrets", body)
	if err != nil {
		return err
	}

	// Extract secret name from response if successful
	if s.httpSteps.LastResponse().StatusCode == 201 {
		var response struct {
			Secret struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
			} `json:"secret"`
		}
		if err := json.Unmarshal(s.httpSteps.LastBody(), &response); err == nil {
			s.lastSecret.name = response.Secret.Metadata.Name
			s.lastSecret.exists = true
		}
	}
	return nil
}

// createSecretWithValue creates a secret with a simple name and value
func (s *SecretSteps) createSecretWithValue(name, value string) error {
	config := fmt.Sprintf(`{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "%s"
  },
  "spec": {
    "displayName": "%s",
    "description": "Auto-generated secret",
    "value": "%s"
  }
}`, name, name, value)

	s.httpSteps.SetHeader("Content-Type", "application/json")
	err := s.httpSteps.SendPOSTToService("gateway-controller", "/secrets", &godog.DocString{Content: config})
	if err != nil {
		return err
	}

	if s.httpSteps.LastResponse().StatusCode == 201 {
		s.lastSecret.name = name
		s.lastSecret.exists = true
	}
	return nil
}

// getSecret retrieves a secret by name
func (s *SecretSteps) getSecret(name string) error {
	err := s.httpSteps.SendGETToService("gateway-controller", "/secrets/"+name)
	if err != nil {
		return err
	}

	if s.httpSteps.LastResponse().StatusCode == 200 {
		s.lastSecret.name = name
		s.lastSecret.exists = true
	}
	return nil
}

// listSecrets retrieves all secrets
func (s *SecretSteps) listSecrets() error {
	return s.httpSteps.SendGETToService("gateway-controller", "/secrets")
}

// updateSecret updates a secret with the provided JSON configuration
func (s *SecretSteps) updateSecret(name string, body *godog.DocString) error {
	s.httpSteps.SetHeader("Content-Type", "application/json")
	return s.httpSteps.SendPUTToService("gateway-controller", "/secrets/"+name, body)
}

// updateSecretWithValue updates a secret with a simple value
func (s *SecretSteps) updateSecretWithValue(name, value string) error {
	config := fmt.Sprintf(`{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "%s"
  },
  "spec": {
    "displayName": "%s",
    "description": "Auto-generated secret",
    "value": "%s"
  }
}`, name, name, value)

	s.httpSteps.SetHeader("Content-Type", "application/json")
	return s.httpSteps.SendPUTToService("gateway-controller", "/secrets/"+name, &godog.DocString{Content: config})
}

// deleteSecret deletes a secret by name
func (s *SecretSteps) deleteSecret(name string) error {
	err := s.httpSteps.SendDELETEToService("gateway-controller", "/secrets/"+name)
	if err != nil {
		return err
	}

	if s.httpSteps.LastResponse().StatusCode == 200 {
		if s.lastSecret.name == name {
			s.lastSecret.exists = false
		}
	}
	return nil
}
