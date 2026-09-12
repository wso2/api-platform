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

@template-functions
Feature: Template functions in resource specs
  As an API administrator
  I want template expressions ({{ env }}, {{ secret }}, {{ default }}) in a
  resource spec to be resolved at runtime, while API responses and the
  persisted configuration keep the original, unrendered template body.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: secret template in set-headers policy value is rendered upstream but unrendered in response and stored configuration
    Given I generate a unique value from "tpl-auth-token" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Template Function Secret",
          "value": "xyz-test-token-123"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    Given I generate a unique resource name from "tpl-secret-api" and store it as "apiName"
    And I generate a unique API version from "tpl-secret-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-secret" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Secret-Api                     |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3002               |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Auth-Token","value":"Bearer {{ secret \"${CTX:secretName}\" }}"}]}}}]}] |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ secret "${CTX:secretName}" }}
      """

    # GET response must also echo the unrendered template body
    When I get the API "${CTX:apiName}"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ secret "${CTX:secretName}" }}
      """

    # Stored configuration must persist the unrendered template body
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ secret "${CTX:secretName}" }}
      """

    # Runtime traffic must hit upstream with the resolved secret value
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer xyz-test-token-123"

  Scenario: env template in upstream URL path resolves at runtime
    Given I generate a unique resource name from "tpl-env-api" and store it as "apiName"
    And I generate a unique API version from "tpl-env-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1               |
      | name                   | ${CTX:apiName}                                   |
      | spec.displayName       | Tpl-Env-Api                                      |
      | spec.version           | ${CTX:apiVersion}                                |
      | spec.context           | ${CTX:apiContext}/$version                       |
      | spec.upstream.main.url | http://testbench:3002{{ env "IT_TEMPLATE_PATH" }} |
      | spec.operations        | [{"method":"GET","path":"/probe"}]                |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    When I get the API "${CTX:apiName}"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    # Runtime: upstream must have been built with the resolved env path
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response body should contain "/anything/probe"

  Scenario: default function returns fallback when env is missing
    Given I generate a unique resource name from "tpl-default-api" and store it as "apiName"
    And I generate a unique API version from "tpl-default-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-default" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Default-Api                    |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3002               |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Fallback","value":"{{ env \"IT_DEFINITELY_MISSING_KEY\" \| default \"fallback-value\" }}"}]}}}]}] |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Fallback" with value "fallback-value"

  Scenario: secret template in LlmProvider upstream auth value is rendered upstream, unrendered in stored configuration, and never returned in responses
    Given I generate a unique value from "tpl-llm-provider-token" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Template Function LLM Provider Secret",
          "value": "llm-prov-secret-789"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    Given I generate a unique resource name from "tpl-llm-provider" and store it as "providerName"
    And I generate a unique value from "tpl-llm-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tpl-llm-provider" and store it as "providerVersion"
    And I generate a unique API context from "/tpl-llm-provider" and store it as "providerContext"
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
    # upstream auth.value is write-only: neither the secret handle nor its
    # resolved value is returned, on create or on any later read.
    And the response body should not contain "${CTX:secretName}"
    And the response body should not contain "llm-prov-secret-789"
    And the JSON response field "spec.upstream.auth.value" should not exist

    When I get the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:secretName}"
    And the response body should not contain "llm-prov-secret-789"
    And the JSON response field "spec.upstream.auth.value" should not exist
    And the JSON response field "spec.upstream.auth.header" should be "Authorization"

    And the stored LlmProvider configuration for "${CTX:providerName}" should contain:
      """
      {{ secret "${CTX:secretName}" }}
      """
    And the stored LlmProvider configuration for "${CTX:providerName}" should not contain:
      """
      llm-prov-secret-789
      """

    # Runtime: upstream must receive the resolved Authorization header value
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with value "Bearer llm-prov-secret-789"

  Scenario: secret template in LlmProxy set-headers policy is rendered upstream but unrendered in response and stored configuration
    Given I generate a unique value from "tpl-llm-proxy-token" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Template Function LLM Proxy Secret",
          "value": "llm-proxy-secret-456"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    Given I generate a unique resource name from "tpl-llm-proxy-prov" and store it as "providerName"
    And I generate a unique value from "tpl-llm-proxy-prov" and store it as "providerDisplayName"
    And I generate a unique API version from "tpl-llm-proxy-prov" and store it as "providerVersion"
    And I generate a unique API context from "/tpl-llm-proxy-prov" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002               |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    Given I generate a unique resource name from "tpl-llm-proxy" and store it as "proxyName"
    And I generate a unique value from "tpl-llm-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "tpl-llm-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/tpl-llm-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:proxyName}                   |
      | displayName            | ${CTX:proxyDisplayName}            |
      | version                | ${CTX:proxyVersion}                |
      | context                | ${CTX:proxyContext}                |
      | provider.id            | ${CTX:providerName}                 |
      | spec.operationPolicies | [{"name":"set-headers","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"request":{"headers":[{"name":"X-Auth-Token","value":"Bearer {{ secret \"${CTX:secretName}\" }}"}]}}}]}] |
    Then the response status should be 201
    And the response body should contain template literal:
      """
      {{ secret "${CTX:secretName}" }}
      """

    When I get the LLM proxy "${CTX:proxyName}"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ secret "${CTX:secretName}" }}
      """
    And the stored LlmProxy configuration for "${CTX:proxyName}" should contain:
      """
      {{ secret "${CTX:secretName}" }}
      """

    # Runtime: upstream must receive the resolved X-Auth-Token header value
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}
      """
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer llm-proxy-secret-456"

  Scenario: env template in McpProxy upstream URL resolves at runtime but is unrendered in response and stored configuration
    Given I generate a unique resource name from "tpl-mcp" and store it as "mcpName"
    And I generate a unique value from "tpl-mcp" and store it as "mcpDisplayName"
    And I generate a unique API version from "tpl-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/tpl-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1                                            |
      | name              | ${CTX:mcpName}                                                                |
      | displayName       | ${CTX:mcpDisplayName}                                                         |
      | version           | ${CTX:mcpVersion}                                                             |
      | context           | ${CTX:mcpContext}                                                             |
      | specVersion       | 2025-06-18                                                                      |
      | spec.upstream.url | http://testbench:3009/mcp{{ env "IT_DEFINITELY_MISSING_KEY" \| default "" }} |
    Then the response should be successful
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}
      """

    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/${CTX:mcpName}"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}
      """
    And the stored Mcp configuration for "${CTX:mcpName}" should contain:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "" }}
      """

    # Runtime: upstream URL must have resolved to the bare mcp-server backend
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

  Scenario: env template in integer policy param is coerced and enforced at runtime
    Given I generate a unique resource name from "tpl-env-ratelimit-api" and store it as "apiName"
    And I generate a unique API version from "tpl-env-ratelimit-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-ratelimit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Env-Ratelimit-Api               |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":"{{ env \"IT_RATE_LIMIT\" }}","duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_RATE_LIMIT" }}
      """
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ env "IT_RATE_LIMIT" }}
      """

    # The readiness probe uses ~1 request; send 4 more to reach the limit of 5.
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I send 4 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200

    # One more request must be rejected — limit exhausted.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  Scenario: env template in boolean policy param is coerced and applied at runtime
    Given I generate a unique resource name from "tpl-env-cors-api" and store it as "apiName"
    And I generate a unique API version from "tpl-env-cors-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-cors" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Env-Cors-Api                    |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET"],"allowCredentials":"{{ env \"IT_ALLOW_CREDENTIALS\" }}"}}] |
      | spec.operations        | [{"method":"GET","path":"/probe"}]  |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_ALLOW_CREDENTIALS" }}
      """
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ env "IT_ALLOW_CREDENTIALS" }}
      """

    # Runtime: allowCredentials=true must produce the credentials response header.
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Credentials" should be "true"

  Scenario: missing secret reference fails with 400 at deploy time
    Given I generate a unique resource name from "tpl-bad-secret-api" and store it as "apiName"
    And I generate a unique API version from "tpl-bad-secret-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-bad-secret" and store it as "apiContext"
    And I generate a unique value from "tpl-no-such-secret" and store it as "missingSecretName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Bad-Secret-Api                  |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3002               |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Bad","value":"{{ secret \"${CTX:missingSecretName}\" }}"}]}}}]}] |
    Then the response status code should be 400
