# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

Feature: Lazy Resources xDS Synchronization
  As an API platform operator
  I want lazy resources (LLM provider templates) to be synchronized from gateway controller to policy engine via xDS
  So that the policy engine can access template configurations for analytics and token tracking

  Background:
    Given the gateway services are running

  # ========================================
  # Scenario: Verify lazy resources are synced via xDS
  # ========================================

  Scenario: LLM provider template is synchronized to policy engine via xDS
    # First, create an LLM provider template in the gateway controller
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProviderTemplate
        metadata:
          name: xds-test-template
        spec:
          displayName: xDS Test Template
          promptTokens:
            location: payload
            identifier: $.usage.prompt_tokens
          completionTokens:
            location: payload
            identifier: $.usage.completion_tokens
          totalTokens:
            location: payload
            identifier: $.usage.total_tokens
          remainingTokens:
            location: header
            identifier: x-ratelimit-remaining-tokens
          requestModel:
            location: payload
            identifier: $.model
          responseModel:
            location: payload
            identifier: $.model
        """
    Then the response status code should be 201
    And the JSON response field "id" should be "xds-test-template"

    # Wait for xDS propagation
    When I wait for 3 seconds

    # Query the policy engine config dump to verify lazy resources
    When I send a GET request to "http://localhost:9002/config_dump"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "lazy_resources.total_resources" should be greater than 0
    And the lazy resources should contain template "xds-test-template" of type "LlmProviderTemplate"

    # Cleanup: delete the test template
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "xds-test-template"
    Then the response status code should be 200


  Scenario: OOB templates are available in policy engine lazy resources
    # Verify that out-of-box templates are synchronized to policy engine
    When I send a GET request to "http://localhost:9002/config_dump"
    Then the response status code should be 200
    And the response should be valid JSON
    And the lazy resources should contain template "openai" of type "LlmProviderTemplate"
    And the lazy resources should contain template "anthropic" of type "LlmProviderTemplate"
    And the lazy resources should contain template "gemini" of type "LlmProviderTemplate"


  Scenario: Updated template is reflected in policy engine lazy resources
    # Create a template
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProviderTemplate
        metadata:
          name: update-test-template
        spec:
          displayName: Original Display Name
          promptTokens:
            location: payload
            identifier: $.usage.prompt_tokens
        """
    Then the response status code should be 201

    # Wait for xDS propagation
    When I wait for 3 seconds

    # Verify template exists in policy engine
    When I send a GET request to "http://localhost:9002/config_dump"
    Then the response status code should be 200
    And the lazy resources should contain template "update-test-template" of type "LlmProviderTemplate"
    And the lazy resource "update-test-template" should have display name "Original Display Name"

    # Update the template
    Given I authenticate using basic auth as "admin"
    When I update the LLM provider template "update-test-template" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProviderTemplate
        metadata:
          name: update-test-template
        spec:
          displayName: Updated Display Name
          promptTokens:
            location: payload
            identifier: $.usage.prompt_tokens
        """
    Then the response status code should be 200

    # Wait for xDS propagation
    When I wait for 3 seconds

    # Verify updated template in policy engine
    When I send a GET request to "http://localhost:9002/config_dump"
    Then the response status code should be 200
    And the lazy resource "update-test-template" should have display name "Updated Display Name"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "update-test-template"
    Then the response status code should be 200


  Scenario: Deleted template is removed from policy engine lazy resources
    # Create a template
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProviderTemplate
        metadata:
          name: delete-test-template
        spec:
          displayName: Delete Test Template
        """
    Then the response status code should be 201

    # Wait for xDS propagation
    When I wait for 3 seconds

    # Verify template exists in policy engine
    When I send a GET request to "http://localhost:9002/config_dump"
    Then the response status code should be 200
    And the lazy resources should contain template "delete-test-template" of type "LlmProviderTemplate"

    # Delete the template
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "delete-test-template"
    Then the response status code should be 200

    # Wait for xDS propagation
    When I wait for 3 seconds

    # Verify template is removed from policy engine
    When I send a GET request to "http://localhost:9002/config_dump"
    Then the response status code should be 200
    And the lazy resources should not contain template "delete-test-template"
