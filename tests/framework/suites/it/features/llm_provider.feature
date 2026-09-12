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

@llm-provider-management
Feature: LLM provider management
  As an API administrator
  I want to manage LLM providers through the management API
  So that I can configure and control access to LLM services

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Complete LLM provider lifecycle - create, retrieve, update, and delete
    Given I generate a unique resource name from "lpm-lifecycle" and store it as "providerName"
    And I generate a unique value from "lpm-lifecycle" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-lifecycle" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-lifecycle" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And the JSON response field "status.id" should be "${CTX:providerName}"
    And the JSON response field "metadata.name" should be "${CTX:providerName}"

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "metadata.name" should be "${CTX:providerName}"
    And the JSON response field "spec.displayName" should be "${CTX:providerDisplayName}"
    And the JSON response field "spec.version" should be "${CTX:providerVersion}"
    And the JSON response field "spec.template" should be "openai"
    And the JSON response field "spec.accessControl.mode" should be "allow_all"

    When I update LLM provider "${CTX:providerName}" from "resources/templates/llm-provider.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1 |
      | name                          | ${CTX:providerName}               |
      | displayName                   | ${CTX:providerDisplayName} Updated |
      | version                       | ${CTX:providerVersion}            |
      | template                      | openai                              |
      | spec.context                  | ${CTX:providerContext}            |
      | spec.upstream.url             | http://testbench:3008/openai/v1   |
      | accessControl.mode            | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["POST"]}] |
    Then the response status code should be 200
    And the response should be valid JSON
    And the API update response should indicate successful deployment
    And the JSON response field "status.id" should be "${CTX:providerName}"
    And the JSON response field "metadata.name" should be "${CTX:providerName}"

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "${CTX:providerDisplayName} Updated"
    And the JSON response field "spec.accessControl.mode" should be "deny_all"
    And the JSON response field "spec.accessControl.exceptions[0].path" should be "/chat/completions"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should be "LLM provider deleted successfully"

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List all LLM providers after deploying some
    Given I generate a unique resource name from "lpm-list-1" and store it as "providerName1"
    And I generate a unique value from "lpm-list-1" and store it as "providerDisplayName1"
    And I generate a unique API version from "lpm-list-1" and store it as "providerVersion1"
    And I generate a unique API context from "/lpm-list-1" and store it as "providerContext1"
    And I generate a unique resource name from "lpm-list-2" and store it as "providerName2"
    And I generate a unique value from "lpm-list-2" and store it as "providerDisplayName2"
    And I generate a unique API version from "lpm-list-2" and store it as "providerVersion2"
    And I generate a unique API context from "/lpm-list-2" and store it as "providerContext2"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName1}              |
      | displayName        | ${CTX:providerDisplayName1}       |
      | version            | ${CTX:providerVersion1}           |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext1}           |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName2}              |
      | displayName        | ${CTX:providerDisplayName2}       |
      | version            | ${CTX:providerVersion2}           |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext2}           |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | deny_all                            |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be greater than 1

    When I delete the LLM provider "${CTX:providerName1}"
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName2}"
    Then the response status code should be 200

  Scenario: List all LLM providers
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM providers with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Get LLM provider with invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/invalid@provider#id"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Filter LLM providers by displayName
    Given I generate a unique resource name from "lpm-filter-name" and store it as "providerName"
    And I generate a unique value from "lpm-filter-name" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-filter-name" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-filter-name" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers?displayName=${CTX:providerDisplayName}"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "providers[0].spec.displayName" should be "${CTX:providerDisplayName}"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Filter LLM providers by version
    Given I generate a unique resource name from "lpm-filter-version" and store it as "providerName"
    And I generate a unique value from "lpm-filter-version" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-filter-version" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-filter-version" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers?version=${CTX:providerVersion}"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "providers[0].spec.version" should be "${CTX:providerVersion}"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: LLM provider with vhost configuration
    Given I generate a unique resource name from "lpm-vhost" and store it as "providerName"
    And I generate a unique value from "lpm-vhost" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-vhost" and store it as "providerVersion"
    And I generate a unique resource name from "lpm-vhost-host" and store it as "vhostName"
    And I generate a unique API context from "/lpm-vhost" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.vhost         | ${CTX:vhostName}.local             |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 200
    And the JSON response field "spec.vhost" should be "${CTX:vhostName}.local"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: LLM provider using openai template
    Given I generate a unique resource name from "lpm-template" and store it as "providerName"
    And I generate a unique value from "lpm-template" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-template" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-template" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 200
    And the JSON response field "spec.template" should be "openai"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: LLM provider with attached policies
    Given I generate a unique resource name from "lpm-policy" and store it as "providerName"
    And I generate a unique value from "lpm-policy" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-policy" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-policy" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | openai                              |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3008/openai/v1   |
      | accessControl.mode     | allow_all                          |
      | spec.operationPolicies | [{"name":"set-headers","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"request":{"headers":[{"name":"x-custom-header","value":"test-value"}]}}}]}] |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 200
    And the JSON response field "spec.operationPolicies[0].name" should be "set-headers"
    And the JSON response field "spec.operationPolicies[0].version" should be "v1"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  # The canonical template requires version/template/accessControl.mode as placeholders, so
  # this narrows to specifically the missing-upstream case rather than an entirely bare
  # document - the template renderer itself (not the product) would reject the latter before
  # any request is even sent.
  Scenario: Create LLM provider with invalid configuration - missing required fields
    Given I generate a unique resource name from "lpm-invalid" and store it as "providerName"
    And I generate a unique API version from "lpm-invalid" and store it as "providerVersion"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | Invalid Provider                    |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider referencing a non-existent policy is rejected
    Given I generate a unique resource name from "lpm-bad-policy" and store it as "providerName"
    And I generate a unique value from "lpm-bad-policy" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-bad-policy" and store it as "providerVersion"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | gateway.api-platform.wso2.com/v1 |
      | name                | ${CTX:providerName}               |
      | displayName         | ${CTX:providerDisplayName}        |
      | version             | ${CTX:providerVersion}            |
      | template            | openai                              |
      | spec.upstream.url   | http://testbench:3008/openai/v1   |
      | accessControl.mode  | allow_all                          |
      | spec.globalPolicies | [{"name":"this-policy-does-not-exist","version":"v1"}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider referencing a non-existent policy version is rejected
    Given I generate a unique resource name from "lpm-bad-version" and store it as "providerName"
    And I generate a unique value from "lpm-bad-version" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-bad-version" and store it as "providerVersion"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | gateway.api-platform.wso2.com/v1 |
      | name                | ${CTX:providerName}               |
      | displayName         | ${CTX:providerDisplayName}        |
      | version             | ${CTX:providerVersion}            |
      | template            | openai                              |
      | spec.upstream.url   | http://testbench:3008/openai/v1   |
      | accessControl.mode  | allow_all                          |
      | spec.globalPolicies | [{"name":"basic-ratelimit","version":"v999"}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # An empty policy version is a valid input: it resolves to the latest loaded version.
  Scenario: Create LLM provider with an empty policy version resolves to the latest
    Given I generate a unique resource name from "lpm-empty-version" and store it as "providerName"
    And I generate a unique value from "lpm-empty-version" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-empty-version" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-empty-version" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | gateway.api-platform.wso2.com/v1 |
      | name                | ${CTX:providerName}               |
      | displayName         | ${CTX:providerDisplayName}        |
      | version             | ${CTX:providerVersion}            |
      | template            | openai                              |
      | spec.context        | ${CTX:providerContext}            |
      | spec.upstream.url   | http://testbench:3008/openai/v1   |
      | accessControl.mode  | allow_all                          |
      | spec.globalPolicies | [{"name":"basic-ratelimit","version":"","params":{"limits":[{"requests":10,"duration":"1h"}]}}] |
    Then the response status code should be 201
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Retrieve non-existent LLM provider
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/non-existent-provider"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Delete non-existent LLM provider
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-providers/non-existent-delete"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider with invalid JSON body returns error
    When I send a "POST" request to the "gateway-controller" service at "/llm-providers" with body:
      """
      { this is not valid json content
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Update LLM provider with invalid JSON body returns error
    When I send a "PUT" request to the "gateway-controller" service at "/llm-providers/some-provider" with body:
      """
      { invalid json
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create LLM provider with minimal required fields
    Given I generate a unique resource name from "lpm-minimal" and store it as "providerName"
    And I generate a unique value from "lpm-minimal" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-minimal" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-minimal" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerName}"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "${CTX:providerDisplayName}"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Invoke LLM provider chat completions endpoint via context path
    Given I generate a unique resource name from "lpm-invoke-context" and store it as "providerName"
    And I generate a unique value from "lpm-invoke-context" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-invoke-context" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-invoke-context" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello, how are you?"}]}
      """
    Then the response should be valid JSON
    And the response body should contain "chat.completion"
    And the response body should contain "choices"
    And the JSON response field "object" should be "chat.completion"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Invoke LLM provider - access control deny_all allows exception paths
    Given I generate a unique resource name from "lpm-invoke-acl" and store it as "providerName"
    And I generate a unique value from "lpm-invoke-acl" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-invoke-acl" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-invoke-acl" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1 |
      | name                          | ${CTX:providerName}               |
      | displayName                   | ${CTX:providerDisplayName}        |
      | version                       | ${CTX:providerVersion}            |
      | template                      | openai                              |
      | spec.context                  | ${CTX:providerContext}            |
      | spec.upstream.url             | http://testbench:3008/openai/v1   |
      | accessControl.mode            | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["POST"]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  Scenario: Invoke LLM provider - verify upstream auth header is added
    Given I generate a unique resource name from "lpm-invoke-auth" and store it as "providerName"
    And I generate a unique value from "lpm-invoke-auth" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-invoke-auth" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-invoke-auth" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Test auth"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200

  # A basic secret-in-auth-header round trip (create secret, reference it from
  # spec.upstream.auth.value, invoke, assert the resolved value reaches upstream) is already
  # covered, more thoroughly, by template_functions.feature - including that the value is
  # never returned by any response and is never persisted resolved. The two scenarios below
  # cover what that feature does not: a secret whose value itself needs JSON escaping, and
  # this resource kind's own contract for a secret reference that cannot be resolved at all.
  Scenario: Creating an LLM provider with a nonexistent secret reference fails with 400
    Given I generate a unique resource name from "lpm-bad-secret" and store it as "providerName"
    And I generate a unique value from "lpm-bad-secret" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-bad-secret" and store it as "providerVersion"
    And I generate a unique value from "lpm-bad-secret-missing" and store it as "missingSecretName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | spec.upstream.auth | {"type":"api-key","header":"Authorization","value":"Bearer {{ secret \"${CTX:missingSecretName}\" }}"} |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 400

  Scenario: A secret value containing JSON special characters is resolved correctly in the upstream auth header
    Given I generate a unique value from "lpm-special-secret" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Special Characters Secret",
          "value": "ssk-test\\auth-\"key\""
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    Given I generate a unique resource name from "lpm-special-secret-prov" and store it as "providerName"
    And I generate a unique value from "lpm-special-secret-prov" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-special-secret-prov" and store it as "providerVersion"
    And I generate a unique API context from "/lpm-special-secret-prov" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002               |
      | spec.upstream.auth | {"type":"api-key","header":"Authorization","value":"Bearer {{ secret \"${CTX:secretName}\" }}"} |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Test special char auth"}]}
      """
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with exact value:
      """
      Bearer ssk-test\auth-"key"
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
