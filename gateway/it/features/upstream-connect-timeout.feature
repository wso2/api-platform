# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@backend-timeout @timeouts
Feature: Timeouts
  As an API developer
  I want upstream (connect) and HTTP Connection Manager timeouts to be enforced by the gateway
  So that requests to slow or unreachable backends, and slow downstream clients,
  fail within the configured timeout

  # request_timeout, stream_idle_timeout and idle_timeout are not exercised here:
  # small values would affect the whole shared suite

  Background:
    Given the gateway services are running

  # Tests cluster connect_timeout: upstream does not accept TCP connection in time.
  # Uses unreachable IP (192.0.2.1 per RFC 5737) so connect attempt hangs until connect_timeout.
  Scenario: RestApi backend timeout using upstreamDefinitions
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: timeout-api-v1.0
      spec:
        displayName: Timeout-API
        version: v1.0
        context: /timeout-api/$version
        upstreamDefinitions:
          - name: my-timeout-upstream
            timeout:
              connect: 6000ms
            upstreams:
              - url: http://192.0.2.1:8080
        upstream:
          main:
            ref: my-timeout-upstream
        operations:
          - method: GET
            path: / 
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for policy snapshot sync
    And I record the current time as "request_start"
    When I send a GET request to "http://localhost:8080/timeout-api/v1.0/"
    Then the response status code should be 503
    And the request should have taken at least "6" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the API "timeout-api-v1.0"
    Then the response should be successful

  # Global-default scenario: route timeout comes from it config (6s); elapsed-time assertion verifies configured global timeout.
  Scenario: RestApi without upstream timeout uses global defaults
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: timeout-api-global-v1.0
      spec:
        displayName: Timeout-API-Global
        version: v1.0
        context: /timeout-global/$version
        upstreamDefinitions:
          - name: my-timeout-upstream-global
            upstreams:
              - url: http://192.0.2.1:8080
        upstream:
          main:
            ref: my-timeout-upstream-global
        operations:
          - method: GET
            path: /
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for policy snapshot sync
    And I record the current time as "request_start"
    When I send a GET request to "http://localhost:8080/timeout-global/v1.0/"
    Then the response status code should be 503
    And the request should have taken at least "5" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the API "timeout-api-global-v1.0"
    Then the response should be successful

  # Tests HCM request_headers_timeout (set to "5s" in it/test-config.toml).
  # A raw client sends a partial request and never terminates the headers; the gateway
  # must close the stream with 408 once request_headers_timeout elapses.
  Scenario: HCM request_headers_timeout terminates a slow-header request
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: headers-timeout-api-v1.0
      spec:
        displayName: Headers-Timeout-API
        version: v1.0
        context: /headers-timeout/$version
        upstreamDefinitions:
          - name: headers-timeout-upstream
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: headers-timeout-upstream
        operations:
          - method: GET
            path: /
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I record the current time as "request_start"
    When I open a raw connection to "localhost:8080" and send incomplete request headers for path "/headers-timeout/v1.0/"
    Then the raw response status code should be "408"
    And the request should have taken at least "4" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the API "headers-timeout-api-v1.0"
    Then the response should be successful

  # LLM Provider connect_timeout via upstreamDefinitions ref: the provider's upstream references a
  # definition whose only target is unreachable (192.0.2.1). The connect attempt hangs until the
  # per-upstream connect timeout (6s), then the gateway returns 503. Proves the ref -> upstreamDefinition
  # connect timeout works for LLM providers exactly as it does for RestApi.
  Scenario: LLM provider backend connect timeout using upstreamDefinitions ref
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-connect-timeout-provider
      spec:
        displayName: LLM Connect Timeout Provider
        version: v1.0
        template: openai
        context: /llm-connect-timeout
        upstreamDefinitions:
          - name: llm-unreachable-upstream
            timeout:
              connect: 6000ms
            upstreams:
              - url: http://192.0.2.1:8080
        upstream:
          ref: llm-unreachable-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    And I record the current time as "request_start"
    When I send a GET request to "http://localhost:8080/llm-connect-timeout/get"
    Then the response status code should be 503
    And the request should have taken at least "6" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-connect-timeout-provider"
    Then the response should be successful

  # MCP connect_timeout via upstreamDefinitions ref: the MCP backend reference is unreachable, so the
  # synthesized /mcp route's connect attempt hangs until the per-upstream connect timeout (6s) -> 503.
  Scenario: MCP backend connect timeout using upstreamDefinitions ref
    Given I authenticate using basic auth as "admin"
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: mcp-connect-timeout
      spec:
        displayName: MCP Connect Timeout
        version: v1.0
        context: /mcp-connect-timeout
        specVersion: "2025-06-18"
        upstreamDefinitions:
          - name: mcp-unreachable-upstream
            timeout:
              connect: 6000ms
            upstreams:
              - url: http://192.0.2.1:3001
        upstream:
          ref: mcp-unreachable-upstream
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    And I record the current time as "request_start"
    When I send a GET request to "http://localhost:8080/mcp-connect-timeout/mcp"
    Then the response status code should be 503
    And the request should have taken at least "6" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "mcp-connect-timeout"
    Then the response should be successful

