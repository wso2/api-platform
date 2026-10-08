# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

Feature: API Portal workflows

  Background:
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Workflow Label"}
      """
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Workflow View","labels":["${CTX:labelId}"]}
      """

  Scenario: A publisher creates and retrieves an API workflow
    Given I generate a unique resource name from "portal-workflow" and store it as "workflowId"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with body and stored as "workflowId":
      """
      {"id":"${CTX:workflowId}","displayName":"Weather Onboarding","description":"Guides users through onboarding","apiWorkflowDefinition":{"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Weather Onboarding"

  Scenario: A publisher updates an API workflow
    Given I generate a unique resource name from "portal-workflow" and store it as "workflowId"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with body and stored as "workflowId":
      """
      {"id":"${CTX:workflowId}","displayName":"Original Workflow","description":"original","apiWorkflowDefinition":{"arazzo":"1.0.0","info":{"title":"Original","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON body:
      """
      {"displayName":"Updated Workflow"}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Updated Workflow"

  Scenario: A publisher lists API workflows for a view
    Given I generate a unique resource name from "portal-workflow" and store it as "workflowId"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with body and stored as "workflowId":
      """
      {"id":"${CTX:workflowId}","displayName":"Listed Workflow","description":"listed","apiWorkflowDefinition":{"arazzo":"1.0.0","info":{"title":"Listed","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:workflowId}"

  Scenario: A publisher listing includes hidden API workflows
    Given I generate a unique resource name from "portal-workflow-visible" and store it as "visibleWorkflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with body and stored as "visibleWorkflowId":
      """
      {"id":"${CTX:visibleWorkflowId}","displayName":"Visible Workflow","description":"visible","apiWorkflowDefinition":{"arazzo":"1.0.0","info":{"title":"Visible","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]},"agentVisibility":"VISIBLE"}
      """
    Then the response status code should be 201
    Given I generate a unique resource name from "portal-workflow-hidden" and store it as "hiddenWorkflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with body and stored as "hiddenWorkflowId":
      """
      {"id":"${CTX:hiddenWorkflowId}","displayName":"Hidden Workflow","description":"hidden","apiWorkflowDefinition":{"arazzo":"1.0.0","info":{"title":"Hidden","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]},"agentVisibility":"HIDDEN"}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:visibleWorkflowId}"
    And the response body should contain "${CTX:hiddenWorkflowId}"

  Scenario: A publisher deletes an API workflow
    Given I generate a unique resource name from "portal-workflow" and store it as "workflowId"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with body and stored as "workflowId":
      """
      {"id":"${CTX:workflowId}","displayName":"Delete Workflow","description":"delete","apiWorkflowDefinition":{"arazzo":"1.0.0","info":{"title":"Delete","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 404

  Scenario: A publisher generates an API workflow prompt
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows/generate-prompt" as "publisher" with JSON body:
      """
      {"displayName":"Prompt Workflow","description":"Generates a useful prompt"}
      """
    Then the response status code should be 200
    And the JSON response should have field "agentPrompt"

  Scenario: API workflow prompt generation rejects missing description
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows/generate-prompt" as "publisher" with JSON body:
      """
      {"displayName":"Missing Description"}
      """
    Then the response status code should be 400
