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

# Each scenario drives one policy into its main rejection and holds the whole response — status,
# contract headers and body — to the expected error response recorded for the gateway version
# under test. The expected responses were recorded from the policies as they were before the
# fault contract, and the policies under test come from the adjacent gateway-controllers
# checkout, so a policy change a client could observe fails here. See the "Policy error
# compatibility" section of tests/framework/README.md.
#
# Policies, and a streamed rejection, that a gateway ships from 1.2.0.
@policy-compat
Feature: Policy error responses on Gateway 1.2.0 and later
  As a gateway operator
  I want a policy release to leave every error response a client receives unchanged
  So that upgrading policies on an existing gateway cannot break its callers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: opaque-token-auth rejects a request with no authorization header
    Given I generate a unique value from "compat-opaque-none" and store it as "apiName"
    And I generate a unique API version from "compat-opaque-none" and store it as "apiVersion"
    And I generate a unique API context from "/compat-opaque-none" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/protected","policies":[{"name":"opaque-token-auth","version":"v1","params":{"issuers":["compat-inactive"]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response should match the expected error response "opaque-token-auth-missing-header"

  Scenario: backend-jwt refuses a request when no signing key is configured
    Given I generate a unique value from "compat-backend-jwt" and store it as "apiName"
    And I generate a unique API version from "compat-backend-jwt" and store it as "apiVersion"
    And I generate a unique API context from "/compat-backend-jwt" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/protected","policies":[{"name":"backend-jwt","version":"v1","params":{}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 500
    And the response should match the expected error response "backend-jwt-missing-signing-key"

  Scenario: mcp-ratelimit rejects a tool call over the limit
    Given I generate a unique resource name from "compat-mcp-rl" and store it as "mcpName"
    And I generate a unique value from "compat-mcp-rl" and store it as "mcpDisplayName"
    And I generate a unique API version from "compat-mcp-rl" and store it as "mcpVersion"
    And I generate a unique API context from "/compat-mcp-rl" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName} |
      | displayName       | ${CTX:mcpDisplayName} |
      | version           | ${CTX:mcpVersion} |
      | context           | ${CTX:mcpContext} |
      | specVersion       | 2025-06-18 |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-ratelimit","version":"v1","params":{"tools":[{"name":"add","limits":[{"limit":1,"duration":"1h"}]}]}}] |
    Then the resource creation response should indicate successful deployment

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I send a "POST" request to "${CTX:mcpContext}/mcp" with body:
      """
      {"jsonrpc":"2.0","id":41,"method":"tools/call","params":{"name":"add","arguments":{"a":40,"b":60}}}
      """
    And I send a "POST" request to "${CTX:mcpContext}/mcp" with body:
      """
      {"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"add","arguments":{"a":40,"b":60}}}
      """
    Then the response status code should be 429
    And the response should match the expected error response "mcp-ratelimit-exceeded"

  Scenario: redirect returns its configured permanent redirect unchanged
    Given I generate a unique value from "compat-redirect" and store it as "apiName"
    And I generate a unique API version from "compat-redirect" and store it as "apiVersion"
    And I generate a unique API context from "/compat-redirect" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/old","policies":[{"name":"redirect","version":"v0","params":{"statusCode":301,"scheme":"https","hostname":"example.org","port":8443,"path":{"mode":"full","value":"/moved"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/old"
    Then the response status code should be 301
    And the response should match the expected error response "redirect-configured-301"

  Scenario: openai-to-anthropic-transformer rejects an empty request body
    Given I generate a unique value from "compat-oa-empty" and store it as "apiName"
    And I generate a unique API version from "compat-oa-empty" and store it as "apiVersion"
    And I generate a unique API context from "/compat-oa-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/chat/completions","policies":[{"name":"openai-to-anthropic-transformer","version":"v0","params":{"model":"claude-3-5-haiku-20241022"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      """
    Then the response status code should be 400
    And the response should match the expected error response "openai-to-anthropic-transformer-empty-body"

  Scenario: openai-to-gemini-transformer rejects an empty request body
    Given I generate a unique value from "compat-og-empty" and store it as "apiName"
    And I generate a unique API version from "compat-og-empty" and store it as "apiVersion"
    And I generate a unique API context from "/compat-og-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/chat/completions","policies":[{"name":"openai-to-gemini-transformer","version":"v0","params":{"model":"gemini-1.5-flash-002"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      """
    Then the response status code should be 400
    And the response should match the expected error response "openai-to-gemini-transformer-empty-body"

  Scenario: openai-to-mistral-transformer rejects an empty request body
    Given I generate a unique value from "compat-om-empty" and store it as "apiName"
    And I generate a unique API version from "compat-om-empty" and store it as "apiVersion"
    And I generate a unique API context from "/compat-om-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/chat/completions","policies":[{"name":"openai-to-mistral-transformer","version":"v0","params":{"model":"mistral-small-latest"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      """
    Then the response status code should be 400
    And the response should match the expected error response "openai-to-mistral-transformer-empty-body"

  Scenario: openai-to-bedrock-transformer rejects an empty request body
    Given I generate a unique value from "compat-ob-empty" and store it as "apiName"
    And I generate a unique API version from "compat-ob-empty" and store it as "apiVersion"
    And I generate a unique API context from "/compat-ob-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/chat/completions","policies":[{"name":"openai-to-bedrock-transformer","version":"v0","params":{"model":"anthropic.claude-3-5-sonnet-20240620-v1:0"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      """
    Then the response status code should be 400
    And the response should match the expected error response "openai-to-bedrock-transformer-empty-body"

  Scenario: openai-to-azure-openai-transformer rejects a request that names no deployment anywhere
    Given I generate a unique value from "compat-oz-nomodel" and store it as "apiName"
    And I generate a unique API version from "compat-oz-nomodel" and store it as "apiVersion"
    And I generate a unique API context from "/compat-oz-nomodel" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/chat/completions","policies":[{"name":"openai-to-azure-openai-transformer","version":"v0","params":{"apiVersion":"2024-10-21"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 400
    And the response should match the expected error response "openai-to-azure-openai-transformer-model-missing"

  Scenario: word-count-guardrail ends a streamed reply over the maximum
    Given I generate a unique resource name from "compat-stream-wcg" and store it as "providerName"
    And I generate a unique value from "compat-stream-wcg" and store it as "providerDisplayName"
    And I generate a unique API version from "compat-stream-wcg" and store it as "providerVersion"
    And I generate a unique API context from "/compat-stream-wcg" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerDisplayName} |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3008      |
      | accessControl.mode | allow_all                  |
      | spec.policies      | [{"name":"word-count-guardrail","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"response":{"enabled":true,"min":1,"max":3}}}]}] |
    Then the response status code should be 201

    # The streamed reply has 9 words; the guardrail allows at most 3.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","stream":true,"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response should match the expected error response "word-count-guardrail-stream-max"
