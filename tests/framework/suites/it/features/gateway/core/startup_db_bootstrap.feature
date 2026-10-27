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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@startup-db-bootstrap
Feature: Startup database bootstrap
  As a gateway operator
  I want the gateway-controller to restore persisted configs from the database on startup
  So that control-plane sync failures do not prevent already stored configs from becoming active

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Restarted gateway-controller restores persisted resources from the database
    Given I generate a unique resource name from "startup-db-mcp-v1" and store it as "mcpName"
    And I generate a unique API context from "/startup-db-mcp" and store it as "mcpContext"
    Given I generate a unique resource name from "startup-db-llm-provider" and store it as "resourceName1_1"
    Given I generate a unique API context from "/startup-db-llm" and store it as "resourceContext1_1"
    Given I generate a unique resource name from "startup-db-llm-proxy" and store it as "resourceName1_2"
    Given I generate a unique API context from "/startup-db-proxy" and store it as "resourceContext1_2"
    Given I generate a unique resource name from "startup-db-rest-api" and store it as "resourceName1_3"
    Given I generate a unique API context from "/startup-db-rest" and store it as "resourceContext1_3"
    Given I generate a unique resource name from "startup-db-agent" and store it as "agentName"
    Given I generate a unique API context from "/startup-db-agent" and store it as "agentContext"
    Given I generate a unique value from "startup-db-agent-key" and store it as "agentKeyName"
    Given I generate a unique value from "startup-db-agent-secret" and store it as "agentKey"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:resourceName1_1}           |
      | displayName        | Startup DB LLM Provider           |
      | version            | v1.0                              |
      | template           | openai                            |
      | spec.context        | ${CTX:resourceContext1_1}         |
      | spec.upstream.url  | http://testbench:3008            |
      | spec.upstream.auth | {"type":"api-key","header":"Authorization","value":"Bearer sk-startup-db-test"} |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:resourceName1_2}            |
      | displayName | Startup DB LLM Proxy             |
      | version     | v1.0                              |
      | context     | ${CTX:resourceContext1_2}         |
      | provider.id | ${CTX:resourceName1_1}            |
    Then the response status should be 201

    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion   | ${CTX:gatewaySpecVersion} |
      | name         | ${CTX:mcpName}                   |
      | displayName  | Startup DB MCP                   |
      | version      | v1.0                              |
      | context      | ${CTX:mcpContext}                 |
      | specVersion  | 2025-06-18                        |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion} |
      | name                  | ${CTX:resourceName1_3}           |
      | spec.displayName      | Startup DB Rest API              |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:resourceContext1_3}/$version |
      | spec.upstream.main.url | http://testbench:3000/api/v2     |
      | spec.operations       | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment

    # The Agent carries an auth policy and managed public and protected cards, because restoring
    # its routes is not the same as restoring what hangs off them: a route restored with an empty
    # policy chain still answers, having dropped authentication and the gateway-served cards.
    # "startup_db_protected_skill" is on the stored protected card alone, and "book_trip" on the
    # agent's own extended card alone, so they tell the gateway's copy from the agent's.
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion} |
      | name                               | ${CTX:agentName}          |
      | displayName                        | Startup DB Agent          |
      | context                            | ${CTX:agentContext}       |
      | upstreamUrl                        | http://a2a-trip-planner:9099 |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"managed","content":{"name":"Startup DB Agent","description":"Persisted Agent restored from the database on restart","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}},"protected":{"mode":"managed","content":{"name":"Startup DB Agent","description":"Persisted protected Agent Card restored on restart","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]},{"id":"startup_db_protected_skill","name":"Only on the persisted protected card","description":"Served from the gateway's stored protected card","tags":["travel"]}]}}} |
    Then the response should be successful

    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:agentKeyName}",
        "apiKey": "${CTX:agentKey}"
      }
      """
    Then the response status should be 201

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "warmup"}]
      }
      """
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy warmup"}]
      }
      """
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "before restart"}]
      }
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy before restart"}]
      }
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I clear all headers
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle" until status 200
    Then the response body should contain "/api/v2/us/seattle"

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # --- Agent, before the restart ---
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:agentKey}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "startup_db_protected_skill"
    And the response body should not contain "book_trip"

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401

    # The public card carries no auth policy. Its bytes and ETag are kept to compare after the
    # restart: a changed tag on unchanged bytes would tell every client its cached copy is stale.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Startup DB Agent"
    And the response body should not contain "startup_db_protected_skill"
    And I store the response body as "agentCard"
    And I store the response header "ETag" as "agentCardETag"

    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "after restart warmup"}]
      }
      """
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy after restart warmup"}]
      }
      """
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle" until status 200

    Given I authenticate using basic auth as "admin"
    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:resourceName1_1}"
    And the response body should contain "${CTX:resourceName1_2}"
    And the response body should contain "${CTX:mcpName}"
    And the response body should contain "${CTX:resourceName1_3}"
    And the response body should contain "${CTX:agentName}"
    And the response body should contain "startup-db-llm"
    And the response body should contain "startup-db-proxy"
    And the response body should contain "startup-db-mcp"
    And the response body should contain "startup-db-rest"

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "after restart"}]
      }
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy after restart"}]
      }
      """
    Then the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I clear all headers
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle" until status 200
    Then the response body should contain "/api/v2/us/seattle"

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # --- Agent, after the restart ---
    Given I authenticate using basic auth as "admin"
    When I get the Agent "${CTX:agentName}"
    Then the response status should be 200
    And the response body should contain "${CTX:agentName}"

    # Both transports still route: asserting one would pass on a restore that brought back only
    # one transport's routes.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:agentKey}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    # The protected card came back inside its operation chain: still the gateway's document.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "startup_db_protected_skill"
    And the response body should not contain "book_trip"
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "result.name" should be "Startup DB Agent"
    And the response body should contain "startup_db_protected_skill"

    # The policy chain came back with the routes: a 401, not a 404, means the route exists and
    # its auth policy still enforces.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 401
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "startup_db_protected_skill"

    # The public card is still served from the stored copy, byte for byte, under the same tag.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response header "ETag" should be "${CTX:agentCardETag}"
    And the response body should be:
      """
      ${CTX:agentCard}
      """

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:resourceName1_2}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:resourceName1_3}"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "${CTX:resourceName1_1}"
    Then the response status code should be 200
