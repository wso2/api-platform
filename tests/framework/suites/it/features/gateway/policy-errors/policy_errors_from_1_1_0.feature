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
# Policies every supported gateway release ships, one scenario each.
@policy-compat
Feature: Policy error responses on Gateway 1.1.0 and later
  As a gateway operator
  I want a policy release to leave every error response a client receives unchanged
  So that upgrading policies on an existing gateway cannot break its callers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: jwt-auth rejects a request with no authorization header
    Given I generate a unique value from "compat-jwt-none" and store it as "apiName"
    And I generate a unique API version from "compat-jwt-none" and store it as "apiVersion"
    And I generate a unique API context from "/compat-jwt-none" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response should match the expected error response "jwt-auth-missing-token"

  Scenario: api-key-auth rejects a request with no API key
    Given I generate a unique value from "compat-apikey" and store it as "apiName"
    And I generate a unique API version from "compat-apikey" and store it as "apiVersion"
    And I generate a unique API context from "/compat-apikey" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/protected","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response should match the expected error response "api-key-auth-missing-key"

  Scenario: basic-auth rejects a request with no authorization header
    Given I generate a unique value from "compat-basic-none" and store it as "apiName"
    And I generate a unique API version from "compat-basic-none" and store it as "apiVersion"
    And I generate a unique API context from "/compat-basic-none" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/protected","policies":[{"name":"basic-auth","version":"v1","params":{"username":"compat-user","password":"compat-pass"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response should match the expected error response "basic-auth-missing-header"

  Scenario: subscription-validation rejects a request with no subscription key
    Given I generate a unique value from "compat-sub-none" and store it as "apiName"
    And I generate a unique API version from "compat-sub-none" and store it as "apiVersion"
    And I generate a unique API context from "/compat-sub-none" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/protected","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 403
    And the response should match the expected error response "subscription-validation-missing-key"

  Scenario: advanced-ratelimit rejects a request over its quota with the default response
    Given I generate a unique value from "compat-arl-default" and store it as "apiName"
    And I generate a unique API version from "compat-arl-default" and store it as "apiVersion"
    And I generate a unique API context from "/compat-arl-default" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/limited","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":2,"duration":"1h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 2 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 429
    And the response should match the expected error response "advanced-ratelimit-default-exceeded"

  Scenario: basic-ratelimit rejects a request over the limit
    Given I generate a unique value from "compat-brl" and store it as "apiName"
    And I generate a unique API version from "compat-brl" and store it as "apiVersion"
    And I generate a unique API context from "/compat-brl" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/limited","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":1,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 429
    And the response should match the expected error response "basic-ratelimit-exceeded"

  Scenario: word-count-guardrail rejects a request over the maximum
    Given I generate a unique value from "compat-wcg" and store it as "apiName"
    And I generate a unique API version from "compat-wcg" and store it as "apiVersion"
    And I generate a unique API context from "/compat-wcg" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":3,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      this request has far too many words
      """
    Then the response status code should be 422
    And the response should match the expected error response "word-count-guardrail-request-max"

  Scenario: sentence-count-guardrail rejects a request over the maximum
    Given I generate a unique value from "compat-scg-max" and store it as "apiName"
    And I generate a unique API version from "compat-scg-max" and store it as "apiVersion"
    And I generate a unique API context from "/compat-scg-max" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":2,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four.
      """
    Then the response status code should be 422
    And the response should match the expected error response "sentence-count-guardrail-request-max"

  Scenario: content-length-guardrail rejects a request over the maximum
    Given I generate a unique value from "compat-clg-max" and store it as "apiName"
    And I generate a unique API version from "compat-clg-max" and store it as "apiVersion"
    And I generate a unique API context from "/compat-clg-max" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      this request body is longer than ten bytes
      """
    Then the response status code should be 422
    And the response should match the expected error response "content-length-guardrail-request-max"

  Scenario: regex-guardrail rejects a request that does not match the pattern
    Given I generate a unique value from "compat-rg-miss" and store it as "apiName"
    And I generate a unique API version from "compat-rg-miss" and store it as "apiVersion"
    And I generate a unique API context from "/compat-rg-miss" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[0-9]+$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      abc123
      """
    Then the response status code should be 422
    And the response should match the expected error response "regex-guardrail-request-mismatch"

  Scenario: url-guardrail rejects a request containing an unreachable URL
    Given I generate a unique value from "compat-ug-bad" and store it as "apiName"
    And I generate a unique API version from "compat-ug-bad" and store it as "apiVersion"
    And I generate a unique API context from "/compat-ug-bad" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://nonexistent-host-12345.invalid/test
      """
    Then the response status code should be 422
    And the response should match the expected error response "url-guardrail-request-invalid-url"

  Scenario: json-schema-guardrail rejects a request missing a required field
    Given I generate a unique value from "compat-jsg-invalid" and store it as "apiName"
    And I generate a unique API version from "compat-jsg-invalid" and store it as "apiVersion"
    And I generate a unique API context from "/compat-jsg-invalid" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"},\"age\":{\"type\":\"integer\"}},\"required\":[\"name\",\"age\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"name": "John Doe"}
      """
    Then the response status code should be 422
    And the response should match the expected error response "json-schema-guardrail-request-invalid"

  Scenario: pii-masking-regex rejects a request whose JSONPath does not resolve
    Given I generate a unique value from "compat-pii-path" and store it as "apiName"
    And I generate a unique API version from "compat-pii-path" and store it as "apiVersion"
    And I generate a unique API context from "/compat-pii-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+[.][a-zA-Z]{2,}"}],"jsonPath":"$.nonexistent.field","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    # The request is rejected before the upstream, so the backend stands in for the capture service.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "test@example.com"}
      """
    Then the response status code should be 500
    And the response should match the expected error response "pii-masking-regex-request-jsonpath-missing"

  Scenario: semantic-prompt-guard rejects a prompt matching a denied phrase
    Given I generate a unique value from "compat-spg-deny" and store it as "apiName"
    And I generate a unique API version from "compat-spg-deny" and store it as "apiVersion"
    And I generate a unique API context from "/compat-spg-deny" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["hack the system","bypass security"],"denySimilarityThreshold":0.9}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack the system"}
      """
    Then the response status code should be 422
    And the response should match the expected error response "semantic-prompt-guard-denied"

  Scenario: aws-bedrock-guardrail rejects a request with violating content
    Given I generate a unique value from "compat-abg-block" and store it as "apiName"
    And I generate a unique API version from "compat-abg-block" and store it as "apiVersion"
    And I generate a unique API context from "/compat-abg-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"","showAssessment":false}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This content contains violence and illegal activities"}
      """
    Then the response status code should be 422
    And the response should match the expected error response "aws-bedrock-guardrail-request-violation"

  Scenario: azure-content-safety-content-moderation rejects a request with hate speech
    Given I generate a unique value from "compat-acs-hate" and store it as "apiName"
    And I generate a unique API version from "compat-acs-hate" and store it as "apiVersion"
    And I generate a unique API context from "/compat-acs-hate" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","hateSeverityThreshold":4}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This content contains hate speech"}
      """
    Then the response status code should be 422
    And the response should match the expected error response "azure-content-safety-content-moderation-request-violation"

  Scenario: mcp-auth rejects a request with no token
    Given I generate a unique resource name from "compat-mcp-auth-none" and store it as "mcpName"
    And I generate a unique value from "compat-mcp-auth-none" and store it as "mcpDisplayName"
    And I generate a unique API version from "compat-mcp-auth-none" and store it as "mcpVersion"
    And I generate a unique API context from "/compat-mcp-auth-none" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName} |
      | displayName       | ${CTX:mcpDisplayName} |
      | version           | ${CTX:mcpVersion} |
      | context           | ${CTX:mcpContext} |
      | specVersion       | 2025-06-18 |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the resource creation response should indicate successful deployment

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I send a "POST" request to "${CTX:mcpContext}/mcp" with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"add","arguments":{"a":40,"b":60}}}
      """
    Then the response status code should be 401
    And the response should match the expected error response "mcp-auth-missing-token"

  Scenario: mcp-authz rejects a governed tool call with no authenticated identity
    Given I generate a unique resource name from "compat-mcp-authz-anon" and store it as "mcpName"
    And I generate a unique value from "compat-mcp-authz-anon" and store it as "mcpDisplayName"
    And I generate a unique API version from "compat-mcp-authz-anon" and store it as "mcpVersion"
    And I generate a unique API context from "/compat-mcp-authz-anon" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName} |
      | displayName       | ${CTX:mcpDisplayName} |
      | version           | ${CTX:mcpVersion} |
      | context           | ${CTX:mcpContext} |
      | specVersion       | 2025-06-18 |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["compat-add-scope"]}]}}] |
    Then the resource creation response should indicate successful deployment

    When I clear all headers
    And I set request host to "compat-mcp.example.com"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I send a "POST" request to "${CTX:mcpContext}/mcp" with body:
      """
      {"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"add","arguments":{"a":40,"b":60}}}
      """
    Then the response status code should be 401
    And the response should match the expected error response "mcp-authz-unauthenticated"

  Scenario: mcp-acl-list rejects a tool the list denies
    Given I generate a unique resource name from "compat-mcp-acl-deny" and store it as "mcpName"
    And I generate a unique value from "compat-mcp-acl-deny" and store it as "mcpDisplayName"
    And I generate a unique API version from "compat-mcp-acl-deny" and store it as "mcpVersion"
    And I generate a unique API context from "/compat-mcp-acl-deny" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName} |
      | displayName       | ${CTX:mcpDisplayName} |
      | version           | ${CTX:mcpVersion} |
      | context           | ${CTX:mcpContext} |
      | specVersion       | 2025-06-18 |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-acl-list","version":"v1","params":{"tools":{"mode":"deny","exceptions":["add"]}}}] |
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
      {"jsonrpc":"2.0","id":21,"method":"tools/call","params":{"name":"echo","arguments":{"message":"compat"}}}
      """
    Then the response status code should be 400
    And the response should match the expected error response "mcp-acl-list-tool-denied"

  Scenario: mcp-rewrite rejects a tool the rewrite list does not expose
    Given I generate a unique resource name from "compat-mcp-rw-deny" and store it as "mcpName"
    And I generate a unique value from "compat-mcp-rw-deny" and store it as "mcpDisplayName"
    And I generate a unique API version from "compat-mcp-rw-deny" and store it as "mcpVersion"
    And I generate a unique API context from "/compat-mcp-rw-deny" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName} |
      | displayName       | ${CTX:mcpDisplayName} |
      | version           | ${CTX:mcpVersion} |
      | context           | ${CTX:mcpContext} |
      | specVersion       | 2025-06-18 |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-rewrite","version":"v1","params":{"tools":[{"name":"sum","description":"Take the sum of two numbers","target":"add","inputSchema":"{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"additionalProperties\":false,\"properties\":{\"a\":{\"description\":\"First number\",\"type\":\"number\"},\"b\":{\"description\":\"Second number\",\"type\":\"number\"}},\"required\":[\"a\",\"b\"],\"type\":\"object\"}"}]}}] |
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
      {"jsonrpc":"2.0","id":31,"method":"tools/call","params":{"name":"echo","arguments":{"message":"compat"}}}
      """
    Then the response status code should be 403
    And the response should match the expected error response "mcp-rewrite-tool-not-allowed"

  Scenario: model-round-robin rejects a request when every model is suspended
    Given I generate a unique value from "compat-mrr-suspended" and store it as "apiName"
    And I generate a unique API version from "compat-mrr-suspended" and store it as "apiVersion"
    And I generate a unique API context from "/compat-mrr-suspended" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"model-1"},{"model":"model-2"}],"suspendDuration":60,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 503
    And the response should match the expected error response "model-round-robin-all-models-unavailable"

  Scenario: model-weighted-round-robin rejects a request when every model is suspended
    Given I generate a unique value from "compat-wrr-suspended" and store it as "apiName"
    And I generate a unique API version from "compat-wrr-suspended" and store it as "apiVersion"
    And I generate a unique API context from "/compat-wrr-suspended" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"model-1","weight":1},{"model":"model-2","weight":1}],"suspendDuration":60,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 503
    And the response should match the expected error response "model-weighted-round-robin-all-models-unavailable"

  Scenario: json-xml-mediator rejects a request whose content type does not match the downstream format
    Given I generate a unique value from "compat-jxm-ctype" and store it as "apiName"
    And I generate a unique API version from "compat-jxm-ctype" and store it as "apiVersion"
    And I generate a unique API context from "/compat-jxm-ctype" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/convert","policies":[{"name":"json-xml-mediator","version":"v1","params":{"downsteamPayloadFormat":"json","upstreamPayloadFormat":"xml"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/convert" with body:
      """
      plain text, not JSON
      """
    Then the response status code should be 500
    And the response should match the expected error response "json-xml-mediator-request-content-type"

  Scenario: interceptor-service refuses a request when the request-phase interceptor call fails
    Given I generate a unique value from "compat-is-req" and store it as "apiName"
    And I generate a unique API version from "compat-is-req" and store it as "apiVersion"
    And I generate a unique API context from "/compat-is-req" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/intercepted","policies":[{"name":"interceptor-service","version":"v1","params":{"endpoint":"http://testbench:3002/status","request":{"includeRequestHeaders":true,"includeRequestBody":true,"passthroughOnError":false}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/intercepted" with body:
      """
      {"client":"payload"}
      """
    Then the response status code should be 500
    And the response should match the expected error response "interceptor-service-request-call-failed"

  Scenario: request-rewrite refuses a request when a query rewrite pattern cannot be compiled
    Given I generate a unique value from "compat-rrw-regex" and store it as "apiName"
    And I generate a unique API version from "compat-rrw-regex" and store it as "apiVersion"
    And I generate a unique API context from "/compat-rrw-regex" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/search","policies":[{"name":"request-rewrite","version":"v1","params":{"queryRewrite":{"rules":[{"action":"ReplaceRegexMatch","name":"q","pattern":"(","substitution":"x"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/search?q=value"
    Then the response status code should be 500
    And the response should match the expected error response "request-rewrite-invalid-query-regex"

  Scenario: respond returns its configured maintenance response unchanged
    Given I generate a unique value from "compat-respond-503" and store it as "apiName"
    And I generate a unique API version from "compat-respond-503" and store it as "apiVersion"
    And I generate a unique API context from "/compat-respond-503" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/maintenance","policies":[{"name":"respond","version":"v1","params":{"statusCode":503,"body":"{\"error\":\"Service Unavailable\",\"message\":\"System under maintenance.\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Retry-After","value":"3600"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/maintenance"
    Then the response status code should be 503
    And the response should match the expected error response "respond-configured-503"

  Scenario: prompt-decorator rejects an empty request body
    Given I generate a unique value from "compat-pd-empty" and store it as "apiName"
    And I generate a unique API version from "compat-pd-empty" and store it as "apiVersion"
    And I generate a unique API context from "/compat-pd-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"Test"}]},"jsonPath":"$.messages","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      """
    Then the response status code should be 500
    And the response should match the expected error response "prompt-decorator-empty-body"

  Scenario: prompt-template rejects a reference to an unknown template
    Given I generate a unique value from "compat-pt-notfound" and store it as "apiName"
    And I generate a unique API version from "compat-pt-notfound" and store it as "apiVersion"
    And I generate a unique API context from "/compat-pt-notfound" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"existing","template":"This exists"}]}}]},{"method":"GET","path":"/health"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" with body:
      """
      {"prompt":"template://nonexistent?param=value"}
      """
    Then the response status code should be 500
    And the response should match the expected error response "prompt-template-template-not-found"

  Scenario: log-message leaves a logged request and response unchanged
    Given I generate a unique value from "compat-logmsg" and store it as "apiName"
    And I generate a unique API version from "compat-logmsg" and store it as "apiVersion"
    And I generate a unique API context from "/compat-logmsg" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/json","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true,"headers":true},"response":{"payload":true,"headers":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/json"
    Then the response status code should be 200
    And the response should match the expected error response "log-message-request-response-flow"
