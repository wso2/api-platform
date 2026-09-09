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

@llm-proxy-management
Feature: LLM proxy management
  As an API administrator
  I want to manage LLM proxies through the management API
  So that I can create, read, update, delete, and list LLM proxies

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: List all LLM proxies
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM proxies with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM proxies with different limit values
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies?limit=5"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: List LLM proxies with offset only
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies?offset=10"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: Get LLM proxy by non-existent ID returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-id-12345"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get LLM proxy with invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/invalid@proxy#id$format"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Delete non-existent LLM proxy returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-delete-123"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Delete LLM proxy with invalid ID format returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/invalid-delete@id"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Update non-existent LLM proxy returns 404
    Given I generate a unique resource name from "lpx-nonexistent-update" and store it as "proxyName"
    When I send a "PUT" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "LlmProxy",
        "metadata": {
          "name": "${CTX:proxyName}"
        },
        "spec": {
          "displayName": "Test",
          "version": "v1.0",
          "context": "/test"
        }
      }
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create LLM proxy with missing required fields returns error
    Given I generate a unique resource name from "lpx-invalid" and store it as "proxyName"
    When I send a "POST" request to the "gateway-controller" service at "/llm-proxies" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "LlmProxy",
        "metadata": {
          "name": "${CTX:proxyName}"
        },
        "spec": {
          "displayName": "Invalid Proxy"
        }
      }
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Complete LLM proxy lifecycle - create, get, update, and delete
    Given I generate a unique resource name from "lpx-lifecycle-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-lifecycle-provider" and store it as "providerName"
    And I generate a unique value from "lpx-lifecycle-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-lifecycle-provider" and store it as "providerVersion"
    And I generate a unique resource name from "lpx-lifecycle-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-lifecycle-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-lifecycle-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-lifecycle-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "${CTX:proxyDisplayName}"

    When I update LLM proxy "${CTX:proxyName}" from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName} Updated   |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response should be successful
    And the response should be valid JSON
    And the API update response should indicate successful deployment

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    And the response body should contain "${CTX:proxyDisplayName} Updated"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    And the JSON response field "status" should be "success"

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response status should be 404

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: List LLM proxies after creating one
    Given I generate a unique resource name from "lpx-list-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-list-provider" and store it as "providerName"
    And I generate a unique value from "lpx-list-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-list-provider" and store it as "providerVersion"
    And I generate a unique resource name from "lpx-list-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-list-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-list-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-list-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:proxyName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Create LLM proxy with invalid JSON body returns error
    When I send a "POST" request to the "gateway-controller" service at "/llm-proxies" with body:
      """
      { invalid json content here
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create LLM proxy referencing non-existent provider
    Given I generate a unique resource name from "lpx-orphan" and store it as "proxyName"
    And I generate a unique value from "lpx-orphan" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-orphan" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-orphan" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | non-existent-provider-12345        |
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create LLM proxy referencing a non-existent policy version is rejected
    Given I generate a unique resource name from "lpx-badver-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-badver-provider" and store it as "providerName"
    And I generate a unique value from "lpx-badver-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-badver-provider" and store it as "providerVersion"
    And I generate a unique resource name from "lpx-badver-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-badver-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-badver-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-badver-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion          | gateway.api-platform.wso2.com/v1 |
      | name                | ${CTX:proxyName}                  |
      | displayName         | ${CTX:proxyDisplayName}           |
      | version             | ${CTX:proxyVersion}               |
      | context             | ${CTX:proxyContext}               |
      | provider.id         | ${CTX:providerName}                |
      | spec.globalPolicies | [{"name":"basic-ratelimit","version":"v999"}] |
    Then the response should be a client error
    And the response should be valid JSON

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Update LLM proxy with invalid JSON body returns error
    When I send a "PUT" request to the "gateway-controller" service at "/llm-proxies/some-proxy" with body:
      """
      { not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: List LLM proxies with displayName filter
    Given I generate a unique resource name from "lpx-filter-name-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-filter-name-provider" and store it as "providerName"
    And I generate a unique value from "lpx-filter-name-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-filter-name-provider" and store it as "providerVersion"
    And I generate a unique resource name from "lpx-filter-name-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-filter-name-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-filter-name-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-filter-name-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies?displayName=${CTX:proxyDisplayName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:proxyDisplayName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: List LLM proxies with version filter
    Given I generate a unique resource name from "lpx-filter-ver-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-filter-ver-provider" and store it as "providerName"
    And I generate a unique value from "lpx-filter-ver-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-filter-ver-provider" and store it as "providerVersion"
    And I generate a unique resource name from "lpx-filter-ver-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-filter-ver-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-filter-ver-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-filter-ver-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies?version=${CTX:proxyVersion}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: List LLM proxies with non-matching filter returns empty
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies?displayName=NonExistentProxyName99999"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0

  Scenario: Invoke LLM proxy chat completions endpoint
    Given I generate a unique resource name from "lpx-invoke-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-invoke-provider" and store it as "providerName"
    And I generate a unique value from "lpx-invoke-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-invoke-provider" and store it as "providerVersion"
    And I generate a unique API context from "/lpx-invoke-provider" and store it as "providerContext"
    And I generate a unique resource name from "lpx-invoke-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-invoke-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-invoke-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-invoke-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello from proxy test!"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"
    And the response body should contain "choices"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Invoke LLM proxy - provider access control allows exception paths
    Given I generate a unique resource name from "lpx-acl-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-acl-provider" and store it as "providerName"
    And I generate a unique value from "lpx-acl-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-acl-provider" and store it as "providerVersion"
    And I generate a unique API context from "/lpx-acl-provider" and store it as "providerContext"
    And I generate a unique resource name from "lpx-acl-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-acl-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-acl-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-acl-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1 |
      | name                          | ${CTX:providerName}               |
      | displayName                   | ${CTX:providerDisplayName}        |
      | version                       | ${CTX:providerVersion}            |
      | template                      | ${CTX:templateName}               |
      | spec.context                  | ${CTX:providerContext}            |
      | spec.upstream.url             | http://testbench:3008/openai/v1   |
      | accessControl.mode            | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["POST"]}] |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Multiple sequential requests through LLM proxy
    Given I generate a unique resource name from "lpx-multi-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-multi-provider" and store it as "providerName"
    And I generate a unique value from "lpx-multi-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-multi-provider" and store it as "providerVersion"
    And I generate a unique API context from "/lpx-multi-provider" and store it as "providerContext"
    And I generate a unique resource name from "lpx-multi-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-multi-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-multi-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-multi-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateName}              |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"First request"}]}
      """
    Then the JSON response field "object" should be "chat.completion"

    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Second request"}]}
      """
    Then the response status code should be 200
    And the JSON response field "object" should be "chat.completion"

    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-3.5-turbo","messages":[{"role":"system","content":"Be concise"},{"role":"user","content":"Third request"}]}
      """
    Then the response status code should be 200
    And the response should be valid JSON

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
