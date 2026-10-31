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

@AIW-LLM @llm-provider-inspect
Feature: LLM providers are read back faithfully and shared across projects
  As an AI Workspace administrator
  I want a stored provider to read back exactly as it was created, without its credential
  So that what I configured is what runs, and secrets never leave the platform

  # Sheet AIW-LLM, sub-features "List" (AIW-PRV-007), "Overview" (AIW-PRV-008) and
  # "Isolation" (AIW-PRV-031), at the API level. The list API has no search: finding a
  # provider is paging, which AIW-PRV-007 checks. Providers belong to the organization:
  # AIW-PRV-031 checks that one provider serves proxies in two different projects. The browser journey is
  # suites/ui/features/aiw_llm/provider_inspection.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @AIW-PRV-008
  Scenario: A provider reads back what it was created with and never returns its credential
    Given I generate a unique value from "PRV-008 Provider" and store it as "displayName"
    And I generate a unique resource name from "prv008-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" with value "sk-aiw-prv-008-upstream" via the control plane
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                       |
      | description        | Provider read back by AIW-PRV-008                                                        |
      | template           | azure-openai                                                                             |
      | upstreamUrl        | http://testbench:3008/openai/v1                                                          |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:secretHandle}\" }}"} |
    Then the response status code should be 201
    And I store the JSON response field "id" as "providerId"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:providerId}"
    And the JSON response field "displayName" should be "${CTX:displayName}"
    And the JSON response field "description" should be "Provider read back by AIW-PRV-008"
    And the JSON response field "version" should be "v1.0"
    And the JSON response field "template" should be "azure-openai"
    And the JSON response field "upstream.main.url" should be "http://testbench:3008/openai/v1"
    And the JSON response field "upstream.main.auth.type" should be "api-key"
    And the JSON response field "upstream.main.auth.header" should be "api-key"
    And the JSON response field "createdBy" should be "admin"
    And the response body should not contain "sk-aiw-prv-008-upstream"

    When I send a "GET" request to the control plane at "/llm-providers?limit=100"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:providerId}"
    And the response body should not contain "sk-aiw-prv-008-upstream"

  @AIW-PRV-031
  Scenario: One organization-level provider serves proxies in two different projects
    Given I generate a unique resource name from "prv031-a" and store it as "projectA"
    And I generate a unique resource name from "prv031-b" and store it as "projectB"
    And I create a project "${CTX:projectA}" on the control plane
    And I create a project "${CTX:projectB}" on the control plane
    And I generate a unique resource name from "prv031-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" with value "sk-aiw-prv-031-upstream" via the control plane
    And I generate a unique value from "PRV-031 Provider" and store it as "displayName"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:displayName}              |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    And I generate a unique resource name from "prv031-proxy-a" and store it as "proxyA"
    And I generate a unique resource name from "prv031-proxy-b" and store it as "proxyB"
    And I generate a unique API context from "/prv031-a" and store it as "contextA"
    And I generate a unique API context from "/prv031-b" and store it as "contextB"
    When I create an LLM proxy "${CTX:proxyA}" via the control plane in project "${CTX:projectA}" with context "${CTX:contextA}" referencing provider "${CTX:providerId}" and secret "${CTX:secretHandle}"
    And I create an LLM proxy "${CTX:proxyB}" via the control plane in project "${CTX:projectB}" with context "${CTX:contextB}" referencing provider "${CTX:providerId}" and secret "${CTX:secretHandle}"

    When I send a "GET" request to the control plane at "/llm-proxies/${CTX:proxyA}"
    Then the response status code should be 200
    And the JSON response field "projectId" should be "${CTX:projectA}"
    And the JSON response field "provider.id" should be "${CTX:providerId}"
    When I send a "GET" request to the control plane at "/llm-proxies/${CTX:proxyB}"
    Then the response status code should be 200
    And the JSON response field "projectId" should be "${CTX:projectB}"
    And the JSON response field "provider.id" should be "${CTX:providerId}"

  # Other runners create providers concurrently, so order is asserted within one response
  # and totals as lower bounds; nothing depends on the org's exact contents.
  @AIW-PRV-007
  Scenario: The provider list pages newest first and keeps its page size within bounds
    Given I generate a unique value from "PRV-007 First" and store it as "firstName"
    And I generate a unique value from "PRV-007 Second" and store it as "secondName"
    And I generate a unique value from "PRV-007 Third" and store it as "thirdName"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:firstName}                |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    And I store the JSON response field "id" as "firstId"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:secondName}               |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    And I store the JSON response field "id" as "secondId"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:thirdName}                |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    And I store the JSON response field "id" as "thirdId"

    When I send a "GET" request to the control plane at "/llm-providers?limit=100"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:thirdId}" should come before the item with "id" equal to "${CTX:secondId}"
    And the JSON response array "list" item with "id" equal to "${CTX:secondId}" should come before the item with "id" equal to "${CTX:firstId}"
    And the JSON response field "pagination.limit" should be 100
    And the JSON response field "pagination.offset" should be 0
    And the JSON response field "pagination.total" should be greater than 2

    When I send a "GET" request to the control plane at "/llm-providers?limit=1"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.limit" should be 1

    When I send a "GET" request to the control plane at "/llm-providers?limit=500"
    Then the response status code should be 200
    And the JSON response field "pagination.limit" should be 100

    When I send a "GET" request to the control plane at "/llm-providers?limit=0"
    Then the response status code should be 200
    And the JSON response field "pagination.limit" should be 1

    When I send a "GET" request to the control plane at "/llm-providers?limit=100&offset=1000000"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items
    And the JSON response field "pagination.offset" should be 1000000
