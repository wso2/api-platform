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

@template
Feature: Template functions in RestApi spec
  As an API administrator
  I want template expressions ({{ env }}, {{ secret }}, {{ default }}) in
  a RestApi spec to be resolved at runtime, while the API responses and
  the persisted DB row keep the original unrendered template body.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: secret template in set-headers policy value is rendered upstream but unrendered in response and DB
    When I create a secret named "tpl-auth-token" with value "xyz-test-token-123"
    Then the response status should be 201

    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: tpl-secret-api-v1.0
      spec:
        displayName: Tpl-Secret-Api
        version: v1.0
        context: /tpl-secret/$version
        upstream:
          main:
            url: http://echo-backend-multi-arch:8080/anything
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Auth-Token
                        value: 'Bearer {{ secret "tpl-auth-token" }}'
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ secret "tpl-auth-token" }}
      """

    # GET response must also echo the unrendered template body
    Given I authenticate using basic auth as "admin"
    When I get the API "tpl-secret-api-v1.0"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ secret "tpl-auth-token" }}
      """

    # DB must persist the unrendered template body
    And the stored RestApi configuration for "tpl-secret-api-v1.0" should contain:
      """
      {{ secret "tpl-auth-token" }}
      """

    # Runtime traffic must hit upstream with the resolved secret value
    And I wait for the endpoint "http://localhost:8080/tpl-secret/v1.0/probe" to be ready
    When I send a GET request to "http://localhost:8080/tpl-secret/v1.0/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer xyz-test-token-123"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "tpl-secret-api-v1.0"
    Then the response should be successful
    When I delete the secret "tpl-auth-token"
    Then the response status should be 200

  Scenario: env template in upstream URL path resolves at runtime
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: tpl-env-api-v1.0
      spec:
        displayName: Tpl-Env-Api
        version: v1.0
        context: /tpl-env/$version
        upstream:
          main:
            url: 'http://echo-backend-multi-arch:8080{{ env "IT_TEMPLATE_PATH" }}'
        operations:
          - method: GET
            path: /probe
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    Given I authenticate using basic auth as "admin"
    When I get the API "tpl-env-api-v1.0"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    And the stored RestApi configuration for "tpl-env-api-v1.0" should contain:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    # Runtime: upstream must have been built with /anything (the resolved env value)
    And I wait for the endpoint "http://localhost:8080/tpl-env/v1.0/probe" to be ready
    When I send a GET request to "http://localhost:8080/tpl-env/v1.0/probe"
    Then the response status code should be 200
    And the response body should contain "/anything/probe"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "tpl-env-api-v1.0"
    Then the response should be successful

  Scenario: default function returns fallback when env is missing
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: tpl-default-api-v1.0
      spec:
        displayName: Tpl-Default-Api
        version: v1.0
        context: /tpl-default/$version
        upstream:
          main:
            url: http://echo-backend-multi-arch:8080/anything
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Fallback
                        value: '{{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}'
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """

    And the stored RestApi configuration for "tpl-default-api-v1.0" should contain:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """

    And I wait for the endpoint "http://localhost:8080/tpl-default/v1.0/probe" to be ready
    When I send a GET request to "http://localhost:8080/tpl-default/v1.0/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Fallback" with value "fallback-value"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "tpl-default-api-v1.0"
    Then the response should be successful

  Scenario: secret template in LlmProvider upstream auth value is rendered upstream but unrendered in response and DB
    When I create a secret named "tpl-llm-provider-token" with value "llm-prov-secret-789"
    Then the response status should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProvider
      metadata:
        name: tpl-llm-provider
      spec:
        displayName: Tpl-Llm-Provider
        version: v1.0
        template: openai
        context: /tpl-llm-provider
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: 'Bearer {{ secret "tpl-llm-provider-token" }}'
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ secret "tpl-llm-provider-token" }}
      """

    # GET response must echo the unrendered template body
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "tpl-llm-provider"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ secret "tpl-llm-provider-token" }}
      """

    # DB must persist the unrendered template body
    And the stored LlmProvider configuration for "tpl-llm-provider" should contain:
      """
      {{ secret "tpl-llm-provider-token" }}
      """

    # Runtime: upstream must receive the resolved Authorization header value
    And I wait for the endpoint "http://localhost:8080/tpl-llm-provider/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}'
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tpl-llm-provider/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}]
      }
      """
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with value "Bearer llm-prov-secret-789"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "tpl-llm-provider"
    Then the response status code should be 200
    When I delete the secret "tpl-llm-provider-token"
    Then the response status should be 200

  Scenario: secret template in LlmProxy set-headers policy is rendered upstream but unrendered in response and DB
    When I create a secret named "tpl-llm-proxy-token" with value "llm-proxy-secret-456"
    Then the response status should be 201

    # Plain (un-templated) provider used as the proxy upstream
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProvider
      metadata:
        name: tpl-llm-proxy-provider
      spec:
        displayName: Tpl-Llm-Proxy-Provider
        version: v1.0
        template: openai
        vhost: api.my-llm-provider.local
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProxy
      metadata:
        name: tpl-llm-proxy
      spec:
        displayName: Tpl-Llm-Proxy
        version: v1.0
        context: /tpl-llm-proxy
        provider:
          id: tpl-llm-proxy-provider
        policies:
          - name: set-headers
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  request:
                    headers:
                      - name: X-Auth-Token
                        value: 'Bearer {{ secret "tpl-llm-proxy-token" }}'
      """
    Then the response status should be 201
    And the response body should contain template literal:
      """
      {{ secret "tpl-llm-proxy-token" }}
      """

    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/tpl-llm-proxy"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ secret "tpl-llm-proxy-token" }}
      """

    And the stored LlmProxy configuration for "tpl-llm-proxy" should contain:
      """
      {{ secret "tpl-llm-proxy-token" }}
      """

    # Runtime: upstream must receive the resolved X-Auth-Token header value
    And I wait for the endpoint "http://localhost:8080/tpl-llm-proxy/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}'
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tpl-llm-proxy/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}]
      }
      """
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer llm-proxy-secret-456"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/tpl-llm-proxy"
    Then the response should be successful
    When I delete the LLM provider "tpl-llm-proxy-provider"
    Then the response status code should be 200
    When I delete the secret "tpl-llm-proxy-token"
    Then the response status should be 200

  Scenario: env template in McpProxy upstream URL resolves at runtime but is unrendered in response and DB
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: Mcp
      metadata:
        name: tpl-mcp-v1.0
      spec:
        displayName: Tpl-Mcp
        version: v1.0
        context: /tpl-mcp
        specVersion: "2025-06-18"
        upstream:
          url: 'http://mcp-server-backend:3001{{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}'
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}
      """

    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/mcp-proxies/tpl-mcp-v1.0"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}
      """

    And the stored Mcp configuration for "tpl-mcp-v1.0" should contain:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}
      """

    # Runtime: upstream URL must have resolved to the bare mcp-server-backend host
    And I wait for 2 seconds
    When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/tpl-mcp/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/tpl-mcp/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "tpl-mcp-v1.0"
    Then the response should be successful

  Scenario: missing secret reference fails with 400 at deploy time
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: tpl-bad-secret-api-v1.0
      spec:
        displayName: Tpl-Bad-Secret-Api
        version: v1.0
        context: /tpl-bad-secret/$version
        upstream:
          main:
            url: http://echo-backend-multi-arch:8080/anything
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Bad
                        value: '{{ secret "tpl-no-such-secret-xyz" }}'
      """
    Then the response status code should be 400
