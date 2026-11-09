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


@APIPORTAL @apiportal-workflow
Feature: API workflows are managed per view and shown to people and agents by their status
  As an API publisher
  I want to author multi-step API workflows in a view, publish or draft them, and hide them from agents
  So that developers and AI agents see exactly the workflows I mean them to

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). Each
  # scenario uses its own view, so its workflows and its agent routes belong to it alone. The
  # gallery page and the agent routes are anonymous. A draft is shown nowhere; a hidden workflow is
  # shown to people but to no agent.

  @APIP-021 @type:smoke
  Scenario: A publisher manages an API workflow, its agent visibility, its Arazzo document and its prompt
    # A publisher manages an API workflow from creation to deletion
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-workflow" and store it as "workflowId"
    And I generate a unique resource name from "portal-hidden-workflow" and store it as "hiddenWorkflowId"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}             |
      | displayName           | Weather Onboarding            |
      | description           | Guides users through onboarding |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Weather Onboarding"

    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | displayName | Updated Workflow |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the JSON response field "displayName" should be "Updated Workflow"

    Given a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "hiddenWorkflowId":
      | id                    | ${CTX:hiddenWorkflowId} |
      | displayName           | Hidden Workflow         |
      | description           | Kept from agents        |
      | agentVisibility       | HIDDEN                  |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:workflowId}"
    And the response body should contain "${CTX:hiddenWorkflowId}"

    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 404

    # A PUBLISHED workflow that is VISIBLE to agents is in the gallery and to agents
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-shown-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | ${CTX:workflowId}  |
      | description           | Shown by its status |
      | status                | PUBLISHED           |
      | agentVisibility       | VISIBLE       |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows"
    Then the response status code should be 200
    And the response body should contain "${CTX:workflowId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:workflowId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/prompt"
    Then the response status code should be 200

    # A PUBLISHED workflow that is HIDDEN to agents is in the gallery but to no agent
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-shown-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | ${CTX:workflowId}  |
      | description           | Shown by its status |
      | status                | PUBLISHED           |
      | agentVisibility       | HIDDEN       |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows"
    Then the response status code should be 200
    And the response body should contain "${CTX:workflowId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows.md"
    Then the response status code should be 200
    And the response body should not contain "${CTX:workflowId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/prompt"
    Then the response status code should be 404

    # A DRAFT workflow that is VISIBLE to agents is nowhere
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-shown-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | ${CTX:workflowId}  |
      | description           | Shown by its status |
      | status                | DRAFT           |
      | agentVisibility       | VISIBLE       |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows"
    Then the response status code should be 200
    And the response body should not contain "${CTX:workflowId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows.md"
    Then the response status code should be 200
    And the response body should not contain "${CTX:workflowId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/prompt"
    Then the response status code should be 404

    # An ARAZZO workflow's Arazzo document is served
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-arazzo-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id          | ${CTX:workflowId}  |
      | displayName | ${CTX:workflowId}  |
      | description | Has a document     |
      | status      | PUBLISHED          |
      | contentType | ARAZZO     |
      | apiWorkflowDefinition      | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}          |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200

    # An MD workflow's Arazzo document is not found
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-arazzo-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id          | ${CTX:workflowId}  |
      | displayName | ${CTX:workflowId}  |
      | description | Has a document     |
      | status      | PUBLISHED          |
      | contentType | MD     |
      | markdownContent      | # Onboarding steps          |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 404

    # A workflow's content switches from an Arazzo document to Markdown
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-switched-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId} |
      | displayName           | Switched Workflow |
      | description           | Changes its format |
      | status                | PUBLISHED         |
      | contentType           | ARAZZO            |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | contentType     | MD                         |
      | markdownContent | # Booking steps for agents |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the JSON response field "contentType" should be "MD"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 200
    And the response body should contain "Booking steps for agents"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 404

    # A publisher generates an agent prompt for a workflow, which needs a description
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows/generate-prompt" as "publisher" with JSON values:
      | displayName | Prompt Workflow           |
      | description | Generates a useful prompt |
    Then the response status code should be 200
    And the JSON response should have field "agentPrompt"
    And the JSON response field "agentPrompt" should contain "API Execution Agent"
    And the JSON response field "agentPrompt" should contain "App Builder Agent"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows/generate-prompt" as "publisher" with JSON values:
      | displayName | Missing Description |
    Then the response status code should be 400

    # A view's llms.txt lists only its published workflows that agents may see
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-listed-workflow" and store it as "workflowId"
    And I generate a unique resource name from "portal-hidden-workflow" and store it as "hiddenWorkflowId"
    And I generate a unique resource name from "portal-draft-workflow" and store it as "draftWorkflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}           |
      | displayName           | Listed Workflow             |
      | description           | Listed ${CTX:workflowId}    |
      | status                | PUBLISHED                   |
      | agentVisibility       | VISIBLE                     |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "hiddenWorkflowId":
      | id                    | ${CTX:hiddenWorkflowId}        |
      | displayName           | Hidden Workflow                |
      | description           | Hidden ${CTX:hiddenWorkflowId} |
      | status                | PUBLISHED                      |
      | agentVisibility       | HIDDEN                         |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "draftWorkflowId":
      | id                    | ${CTX:draftWorkflowId}       |
      | displayName           | Draft Workflow               |
      | description           | Draft ${CTX:draftWorkflowId} |
      | status                | DRAFT                        |
      | agentVisibility       | VISIBLE                      |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "## API Workflows"
    And the response body should contain "Listed ${CTX:workflowId}"
    And the response body should not contain "Hidden ${CTX:hiddenWorkflowId}"
    And the response body should not contain "Draft ${CTX:draftWorkflowId}"

  @APIP-021 @type:negative
  Scenario: Invalid workflow requests are refused and the workflow is unchanged
    # A workflow created with an id holding a space is refused and stores nothing, while a YAML definition is accepted
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-invalid-workflow" and store it as "workflowId"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id                    | bad id ${CTX:workflowId}                  |
      | displayName           | Invalid Workflow      |
      | description           | Refused on create     |
      | status               | PUBLISHED               |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}          |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response body should not contain "Invalid Workflow"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | Valid Workflow      |
      | description           | Accepted on create  |
      | status                | PUBLISHED           |
      | apiWorkflowDefinition | arazzo: 1.0.0       |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200
    And the response body should contain "arazzo"

    # A workflow created with an unknown content type is refused and stores nothing, while a YAML definition is accepted
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-invalid-workflow" and store it as "workflowId"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id                    | ${CTX:workflowId}                  |
      | displayName           | Invalid Workflow      |
      | description           | Refused on create     |
      | contentType               | PDF               |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}          |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response body should not contain "Invalid Workflow"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | Valid Workflow      |
      | description           | Accepted on create  |
      | status                | PUBLISHED           |
      | apiWorkflowDefinition | arazzo: 1.0.0       |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200
    And the response body should contain "arazzo"

    # A workflow created with an unknown status is refused and stores nothing, while a YAML definition is accepted
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-invalid-workflow" and store it as "workflowId"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id                    | ${CTX:workflowId}                  |
      | displayName           | Invalid Workflow      |
      | description           | Refused on create     |
      | status               | ARCHIVED               |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}          |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response body should not contain "Invalid Workflow"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | Valid Workflow      |
      | description           | Accepted on create  |
      | status                | PUBLISHED           |
      | apiWorkflowDefinition | arazzo: 1.0.0       |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200
    And the response body should contain "arazzo"

    # A workflow created with an unknown agent visibility is refused and stores nothing, while a YAML definition is accepted
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-invalid-workflow" and store it as "workflowId"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id                    | ${CTX:workflowId}                  |
      | displayName           | Invalid Workflow      |
      | description           | Refused on create     |
      | agentVisibility               | SECRET               |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]}          |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response body should not contain "Invalid Workflow"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | Valid Workflow      |
      | description           | Accepted on create  |
      | status                | PUBLISHED           |
      | apiWorkflowDefinition | arazzo: 1.0.0       |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200
    And the response body should contain "arazzo"

    # A workflow created with a definition that is not JSON or YAML is refused and stores nothing, while a YAML definition is accepted
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-invalid-workflow" and store it as "workflowId"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id                    | ${CTX:workflowId}                  |
      | displayName           | Invalid Workflow      |
      | description           | Refused on create     |
      | status               | PUBLISHED               |
      | apiWorkflowDefinition | {not: [valid          |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows" as "publisher"
    Then the response body should not contain "Invalid Workflow"
    When a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}  |
      | displayName           | Valid Workflow      |
      | description           | Accepted on create  |
      | status                | PUBLISHED           |
      | apiWorkflowDefinition | arazzo: 1.0.0       |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}/arazzo.json"
    Then the response status code should be 200
    And the response body should contain "arazzo"

    # Workflow requests that repeat an id, name nothing or come from a developer are refused
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-guarded-workflow" and store it as "workflowId"
    And I generate a unique resource name from "portal-missing-view" and store it as "missingViewId"
    And I generate a unique resource name from "portal-missing-workflow" and store it as "missingWorkflowId"
    And I generate a unique resource name from "portal-developer-workflow" and store it as "developerWorkflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId} |
      | displayName           | Guarded Workflow  |
      | description           | Already exists    |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id                    | ${CTX:workflowId} |
      | displayName           | Duplicate         |
      | description           | Same id           |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the JSON response field "displayName" should be "Guarded Workflow"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:missingViewId}/api-workflows" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:missingWorkflowId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "publisher" with JSON values:
      | id | ${CTX:missingWorkflowId} |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/api-workflows" as "developer" with JSON values:
      | id                    | ${CTX:developerWorkflowId} |
      | displayName           | Developer Workflow         |
      | description           | Not allowed                |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:developerWorkflowId}" as "publisher"
    Then the response status code should be 404

    # Workflow updates with invalid values, a taken handle or no target are refused, and the workflow is unchanged
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    Given I generate a unique resource name from "portal-updated-workflow" and store it as "workflowId"
    And I generate a unique resource name from "portal-other-workflow" and store it as "otherWorkflowId"
    And I generate a unique resource name from "portal-absent-workflow" and store it as "missingWorkflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId} |
      | displayName           | Stable Workflow    |
      | description           | Stays as it is     |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "otherWorkflowId":
      | id                    | ${CTX:otherWorkflowId} |
      | displayName           | Other Workflow          |
      | description           | Holds its handle        |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | status | ARCHIVED |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | agentVisibility | SECRET |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | contentType | PDF |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | id | bad id |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | apiWorkflowDefinition | {not: [valid |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher" with JSON values:
      | id | ${CTX:otherWorkflowId} |
    Then the response status code should be 409
    And the response body should not contain "constraint"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Stable Workflow"
    And the JSON response field "status" should be "PUBLISHED"
    And the JSON response field "agentVisibility" should be "VISIBLE"
    And the JSON response field "contentType" should be "ARAZZO"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/api-workflows/${CTX:otherWorkflowId}" as "publisher"
    Then the JSON response field "displayName" should be "Other Workflow"
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}/api-workflows/${CTX:missingWorkflowId}" as "publisher" with JSON values:
      | displayName | Nothing to update |
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}/api-workflows/${CTX:missingWorkflowId}" as "publisher"
    Then the response status code should be 404

  @APIP-021 @known-issue
  Scenario: Each workflow entry in a view's llms.txt shows the workflow's name and links to its page
    # Each workflow entry in a view's llms.txt shows the workflow's name and links to its page
    Given I generate a unique resource name from "portal-workflow-label" and store it as "labelId"
    And I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Workflow Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Workflow View      |
      | labels      | ["${CTX:labelId}"] |
    # Known issue: the llms.txt template reads "name" and "apiWorkflowId", which the workflow has not,
    # so every entry renders with an empty name and a link to ".../api-workflows/.md".
    Given I generate a unique resource name from "portal-linked-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}        |
      | displayName           | Linked ${CTX:workflowId} |
      | description           | Has a working link       |
      | status                | PUBLISHED                |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "[Linked ${CTX:workflowId}]("
    And the response body should contain "/api-workflows/${CTX:workflowId}.md"
    And the response body should not contain "/api-workflows/.md"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api-workflows/${CTX:workflowId}.md"
    Then the response status code should be 200
