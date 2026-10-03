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
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@oauth2-auth
Feature: OAuth2 upstream authentication
  As an API developer
  I want the gateway to fetch, cache, and inject OAuth2 credentials on my behalf
  So that my backend can require OAuth2 without the client handling that credential

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I resolve the "oauth2" service URL at "/oauth2/token" and store it as "oauthTokenEndpoint"
    And I send a "POST" request to the "oauth2" service at "/debug/reset" with body:
      """
      {}
      """

  Scenario: Token-endpoint grant injects a Bearer token
    Given I generate a unique resource name from "oauth2-happy" and store it as "apiName"
    And I generate a unique API version from "oauth2-happy" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-happy" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response body should contain "Bearer mock-token-"
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 1
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Password grant injects a Bearer token
    Given I generate a unique resource name from "oauth2-password" and store it as "apiName"
    And I generate a unique API version from "oauth2-password" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-password" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"grantType":"password","tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret","username":"resource-owner","password":"hunter2"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response body should contain "Bearer mock-token-"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: client_secret_post authentication reaches the token endpoint
    Given I generate a unique resource name from "oauth2-post-auth" and store it as "apiName"
    And I generate a unique API version from "oauth2-post-auth" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-post-auth" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"clientAuthMethod":"client_secret_post","tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the response body should match pattern "authStyle.{3}post"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Custom header name and value prefix are applied
    Given I generate a unique resource name from "oauth2-custom-header" and store it as "apiName"
    And I generate a unique API version from "oauth2-custom-header" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-custom-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret","headerName":"X-Upstream-Token","valuePrefix":""}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    Given I clear all headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response body should contain "mock-token-"
    And the response should not contain echoed header "Authorization"
    And I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A static bearer token does not call the token endpoint
    Given I generate a unique resource name from "oauth2-static" and store it as "apiName"
    And I generate a unique API version from "oauth2-static" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-static" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"bearerToken":"static-long-lived-token-xyz"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with value "Bearer static-long-lived-token-xyz"
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 0
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A token is cached across repeated requests
    Given I generate a unique resource name from "oauth2-cache" and store it as "apiName"
    And I generate a unique API version from "oauth2-cache" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-cache" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}?ttl=3600","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 1
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Invalid client credentials return Bad Gateway
    Given I generate a unique resource name from "oauth2-invalid-client" and store it as "apiName"
    And I generate a unique API version from "oauth2-invalid-client" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-invalid-client" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"definitely-the-wrong-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 502
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An unreachable token endpoint returns Bad Gateway
    Given I generate a unique resource name from "oauth2-unreachable" and store it as "apiName"
    And I generate a unique API version from "oauth2-unreachable" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-unreachable" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"http://mock-oauth2-idp-does-not-exist:9601/oauth2/token","clientId":"test-client","clientSecret":"test-secret","tokenRequestTimeout":"3s","tokenRequestMaxRetries":0}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 502
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A malformed token response returns Bad Gateway
    Given I generate a unique resource name from "oauth2-malformed" and store it as "apiName"
    And I generate a unique API version from "oauth2-malformed" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-malformed" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"malformed-client","clientSecret":"any-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 502
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Token request parameters reach the token endpoint
    Given I generate a unique resource name from "oauth2-params" and store it as "apiName"
    And I generate a unique API version from "oauth2-params" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-params" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret","tokenRequestParams":{"scope":"it-suite-scope"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the response body should contain "it-suite-scope"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Token request headers reach the token endpoint
    Given I generate a unique resource name from "oauth2-headers" and store it as "apiName"
    And I generate a unique API version from "oauth2-headers" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret","tokenRequestHeaders":{"X-IT-Suite-Header":"it-suite-value"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the response body should contain "it-suite-value"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An operation without the policy attached is unaffected
    Given I generate a unique resource name from "oauth2-sibling" and store it as "apiName"
    And I generate a unique API version from "oauth2-sibling" and store it as "apiVersion"
    And I generate a unique API context from "/oauth2-sibling" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"oauth2-generator","version":"v0","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200
    Given I clear all headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health"
    Then the response status code should be 200
    And the response should not contain echoed header "Authorization"
    And I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: LLM provider upstream auth uses the OAuth2 generator
    Given I generate a unique resource name from "oauth2-llm-provider" and store it as "providerName"
    And I generate a unique value from "oauth2-llm-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "oauth2-llm-provider" and store it as "providerVersion"
    And I generate a unique API context from "/oauth2-llm-provider" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion                    | ${CTX:gatewaySpecVersion} |
      | name                          | ${CTX:providerName}               |
      | displayName                   | ${CTX:providerDisplayName}        |
      | version                       | ${CTX:providerVersion}            |
      | template                      | openai                            |
      | spec.context                  | ${CTX:providerContext}            |
      | spec.upstream.url             | http://testbench:3002             |
      | spec.upstream.auth             | {"type":"oauth2","policyParams":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}} |
      | accessControl.mode            | allow_all                         |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}
      """
    Then the response status code should be 200
    And the response body should contain "Bearer mock-token-"
    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
