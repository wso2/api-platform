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

@agent-proxy @agent-proxy-management
Feature: Agent proxies are authored and managed through the control plane
  As an API publisher
  I want to create, read, replace, list and delete Agent proxies in the control plane
  So that an A2A agent is described once, validated against its contract, and ready to deploy

  # Organization isolation, per-scope OAuth alternatives and read-only provenance need a second
  # organization, a narrowed token or a gateway-originated row, none of which the suite's
  # control-plane users can produce. They are covered by the focused platform-api tests named in
  # the Section 14 coverage record rather than by scenarios here.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "agent-mgmt-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @cp14-01 @type:smoke
  Scenario: A minimal Agent proxy is created with a generated handle, read, listed and deleted
    Given I generate a unique value from "Minimal Agent" and store it as "displayName"
    And I generate a unique API context from "/agent-minimal" and store it as "agentContext"
    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | ${CTX:displayName}                                 |
      | projectId   | ${CTX:projectHandle}                               |
      | context     | ${CTX:agentContext}                                |
      | upstreamUrl | http://a2a-trip-planner:9099                       |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]   |
    Then the response status code should be 201
    And I store the JSON response field "id" as "agentHandle"
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}"
    And the JSON response field "kind" should be "AgentProxy"
    And the JSON response field "protocol" should be "a2a"
    And the JSON response field "displayName" should be "${CTX:displayName}"
    And the JSON response field "projectId" should be "${CTX:projectHandle}"
    And the JSON response field "readOnly" should be "false"
    And the JSON response field "createdBy" should be "admin"
    And the JSON response field "a2a.protocolVersion" should be "1.0"
    And the JSON response field "a2a.transports[0].protocolBinding" should be "JSONRPC"
    And the JSON response field "a2a.agentCard" should not exist
    And the JSON response field "a2a.operationConfigs" should not exist
    And the JSON response field "configuration" should not exist

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:agentHandle}"
    And the JSON response field "context" should be "${CTX:agentContext}"
    And the JSON response field "upstream.main.url" should be "http://a2a-trip-planner:9099"

    When I send a "GET" request to the control plane at "/agent-proxies?limit=100"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:agentHandle}" should have "protocol" equal to "a2a"
    And the JSON response array "list" item with "id" equal to "${CTX:agentHandle}" should have "projectId" equal to "${CTX:projectHandle}"
    And the JSON response array "list" item with "id" equal to "${CTX:agentHandle}" should not have field "a2a"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 204
    And the response body should be empty

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"
    And the JSON response field "status" should be "error"

  @cp14-01 @cp14-05
  Scenario: A full Agent proxy keeps its explicit handle, nested card and free-form extensions
    Given I generate a unique resource name from "agent-full" and store it as "agentHandle"
    And I generate a unique API context from "/agent-full" and store it as "agentContext"
    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                    | ${CTX:agentHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | displayName           | Full Agent                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | description           | Plans trips end to end                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | projectId             | ${CTX:projectHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | context               | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | vhost                 | agents.example.com                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | upstreamUrl           | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | upstream.main.auth    | {"type":"api-key","header":"X-Upstream-Key","value":"upstream-credential-never-echoed"}                                                                                                                                                                                                                                                                                                                                                                                                  |
      | resilience            | {"idleTimeout":"5s"}                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | transports            | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                       |
      | a2a.operationConfigs  | {"policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Chain","value":"common"}]},"signing":true}}],"operations":[{"name":"SendMessage","resilience":{"timeout":"30s"}}]}                                                                                                                                                                                                                                                                     |
      | a2a.agentCard         | {"public":{"mode":"managed","path":"/.well-known/agent-card.json","content":{"name":"Full Agent","description":"Plans trips","version":"1.0.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}],"x-vendor-custom":{"kept":true},"signing":{"enabled":true}}},"protected":{"mode":"passthrough","rewriteUrls":false}} |
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}"
    And the JSON response field "id" should be "${CTX:agentHandle}"
    And the JSON response field "upstream.main.auth.header" should be "X-Upstream-Key"
    And the JSON response field "upstream.main.auth.value" should not exist
    And the response body should not contain "upstream-credential-never-echoed"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 200
    And the JSON response field "description" should be "Plans trips end to end"
    And the JSON response field "vhost" should be "agents.example.com"
    And the JSON response field "resilience.idleTimeout" should be "5s"
    And the JSON response array field "a2a.transports" should have 2 items
    And the JSON response field "a2a.operationConfigs.operations[0].name" should be "SendMessage"
    And the JSON response field "a2a.operationConfigs.policies[0].params.signing" should be "true"
    And the JSON response field "a2a.agentCard.public.mode" should be "managed"
    And the JSON response field "a2a.agentCard.public.content.skills[0].id" should be "plan_trip"
    And the JSON response field "a2a.agentCard.public.content.x-vendor-custom.kept" should be "true"
    And the JSON response field "a2a.agentCard.public.content.signing.enabled" should be "true"
    And the JSON response field "a2a.agentCard.public.rewriteUrls" should not exist
    And the JSON response field "a2a.agentCard.protected.mode" should be "passthrough"
    And the JSON response field "a2a.agentCard.protected.rewriteUrls" should be "false"
    And the JSON response field "upstream.main.auth.value" should not exist
    And the response body should not contain "upstream-credential-never-echoed"

  @cp14-02
  Scenario: Agent proxies are listed by protocol with a lightweight projection and pagination
    Given I generate a unique resource name from "agent-list-a" and store it as "firstHandle"
    And I generate a unique resource name from "agent-list-b" and store it as "secondHandle"
    And I generate a unique API context from "/agent-list-a" and store it as "firstContext"
    And I generate a unique API context from "/agent-list-b" and store it as "secondContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:firstHandle}                               |
      | displayName | List Agent A                                     |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:firstContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:secondHandle}                                  |
      | displayName | List Agent B                                         |
      | projectId   | ${CTX:projectHandle}                                 |
      | context     | ${CTX:secondContext}                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                         |
      | transports  | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    And the response status code should be 201

    When I send a "GET" request to the control plane at "/agent-proxies?protocol=a2a&limit=100"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:firstHandle}"
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:secondHandle}"
    And the JSON response array "list" item with "id" equal to "${CTX:secondHandle}" should have "protocol" equal to "a2a"
    And the JSON response array "list" item with "id" equal to "${CTX:secondHandle}" should have "displayName" equal to "List Agent B"
    And the JSON response array "list" item with "id" equal to "${CTX:secondHandle}" should not have field "a2a"
    And the JSON response array "list" item with "id" equal to "${CTX:secondHandle}" should not have field "upstream"
    And the JSON response field "pagination.limit" should be 100
    And the JSON response field "pagination.offset" should be 0
    And the JSON response field "pagination.total" should be greater than 1

    When I send a "GET" request to the control plane at "/agent-proxies?limit=1"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.limit" should be 1
    And the JSON response field "pagination.total" should be greater than 1

    When I send a "GET" request to the control plane at "/agent-proxies?protocol=mcp"
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I send a "GET" request to the control plane at "/agent-proxies?protocol="
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

  @cp14-03
  Scenario: A replacement PUT clears omitted optional configuration and is idempotent
    Given I generate a unique resource name from "agent-put" and store it as "agentHandle"
    And I generate a unique API context from "/agent-put" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                   | ${CTX:agentHandle}                                                                                                                                                                                                                                                                                                                 |
      | displayName          | Put Agent                                                                                                                                                                                                                                                                                                                          |
      | description          | Removed by the replacement                                                                                                                                                                                                                                                                                                         |
      | projectId            | ${CTX:projectHandle}                                                                                                                                                                                                                                                                                                               |
      | context              | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                |
      | vhost                | agents.example.com                                                                                                                                                                                                                                                                                                                 |
      | upstreamUrl          | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                       |
      | resilience           | {"timeout":"30s"}                                                                                                                                                                                                                                                                                                                  |
      | transports           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                                                                                                                                                                                                                                                                                   |
      | a2a.operationConfigs | {"operations":[{"name":"SendMessage","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Op","value":"send"}]}}}]}]}                                                                                                                                                                |
      | a2a.agentCard        | {"public":{"mode":"passthrough","rewriteUrls":false},"protected":{"mode":"passthrough"}}                                                                                                                                                                                                                                           |
      | associatedGateways   | [{"id":"it-gateway"}]                                                                                                                                                                                                                                                                                                              |
    And the response status code should be 201

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | Put Agent Renamed                                |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:agentHandle}"
    And the JSON response field "displayName" should be "Put Agent Renamed"
    And the JSON response field "protocol" should be "a2a"
    And the JSON response field "description" should not exist
    And the JSON response field "vhost" should not exist
    And the JSON response field "resilience" should not exist
    And the JSON response field "a2a.agentCard" should not exist
    And the JSON response field "a2a.operationConfigs" should not exist
    And the JSON response field "associatedGateways" should not exist

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | Put Agent Renamed                                |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:agentHandle}"
    And the JSON response field "displayName" should be "Put Agent Renamed"
    And the JSON response field "description" should not exist
    And the JSON response field "a2a.agentCard" should not exist

    # A replacement changes the authoring document only: nothing is deployed on its behalf.
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments"
    Then the response status code should be 200
    And the JSON response field "count" should be 0

  @cp14-03 @type:negative
  Scenario: A replacement PUT cannot change the protocol or the handle
    Given I generate a unique resource name from "agent-immutable" and store it as "agentHandle"
    And I generate a unique API context from "/agent-immutable" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Immutable Agent                                  |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | Immutable Agent                                  |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
      | protocol    | mcp                                              |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}-renamed                       |
      | displayName | Immutable Agent                                  |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 200
    And the JSON response field "protocol" should be "a2a"
    And the JSON response field "id" should be "${CTX:agentHandle}"

  @cp14-03
  Scenario: A replacement PUT retains an unchanged write-only credential and rejects an incomplete changed one
    Given I generate a unique resource name from "agent-cred" and store it as "agentHandle"
    And I generate a unique resource name from "agent-cred-scope" and store it as "cardScope"
    And I generate a unique API context from "/agent-cred" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "auth" as "cardUpstream"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                 | ${CTX:agentHandle}                                                              |
      | displayName        | Credential Agent                                                                |
      | projectId          | ${CTX:projectHandle}                                                            |
      | context            | ${CTX:agentContext}                                                             |
      | upstreamUrl        | ${CTX:cardUpstream}                                                             |
      | upstream.main.auth | {"type":"api-key","header":"X-Card-Key","value":"card-key-${CTX:cardScope}"}    |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                                |
    And the response status code should be 201

    # The response never carries the credential, so a read-modify-write replacement omits it.
    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName        | Credential Agent Renamed                         |
      | projectId          | ${CTX:projectHandle}                             |
      | context            | ${CTX:agentContext}                              |
      | upstreamUrl        | ${CTX:cardUpstream}                              |
      | upstream.main.auth | {"type":"api-key","header":"X-Card-Key"}         |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200
    And the JSON response field "upstream.main.auth.value" should not exist

    # The upstream only answers with the credential originally supplied, so a card proves it was kept.
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with header "Cache-Control" set to "no-cache" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Protected source card ${CTX:cardScope}"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName        | Credential Agent Renamed                         |
      | projectId          | ${CTX:projectHandle}                             |
      | context            | ${CTX:agentContext}                              |
      | upstreamUrl        | ${CTX:cardUpstream}                              |
      | upstream.main.auth | {"type":"api-key","header":"X-Other-Key"}        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName        | Credential Agent Renamed                         |
      | projectId          | ${CTX:projectHandle}                             |
      | context            | ${CTX:agentContext}                              |
      | upstreamUrl        | ${CTX:cardUpstream}                              |
      | upstream.main.auth | {"type":"none"}                                  |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with header "Cache-Control" set to "no-cache" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 503
    And the JSON response field "code" should be "AGENT_PROXY_UPSTREAM_UNREACHABLE"
    And the JSON response field "message" should be "The upstream agent rejected the credentials the control plane presented for its Agent Card."

  @cp14-04 @type:negative
  Scenario Outline: An Agent proxy payload that breaks the shared contract is rejected: <case>
    When I send a "POST" request to the control plane at "/agent-proxies" with body:
      """
      <body>
      """
    Then the response status code should be <status>
    And the JSON response field "code" should be "<code>"
    And the JSON response field "status" should be "error"

    Examples:
      | case                              | status | code              | body                                                                                                                                                                                                                                    |
      | missing protocol block            | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a"}                                                                              |
      | missing protocol                  | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}                  |
      | second protocol block             | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]},"mcp":{}} |
      | unsupported protocol              | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"grpc","grpc":{}}                                                                  |
      | generic protocolConfig wrapper    | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","protocolConfig":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | legacy top-level transports       | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]},"transports":[{"protocolBinding":"JSONRPC"}]} |
      | legacy top-level protocolVersion  | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","protocolVersion":"1.0","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | root-level agentCard              | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]},"agentCard":{"public":{"mode":"passthrough"}}} |
      | card under operationConfigs       | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"operationConfigs":{"agentCard":{"public":{"mode":"passthrough"}}}}} |
      | top-level policies                | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]},"policies":[{"name":"cors","version":"v1"}]} |
      | explicit null                     | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","vhost":null,"upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | missing displayName               | 400    | VALIDATION_FAILED | {"version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}                          |
      | wrongly typed version             | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":1,"projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}      |
      | reserved handle                   | 400    | VALIDATION_FAILED | {"id":"fetch-agent-card","displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | handle outside the grammar        | 400    | VALIDATION_FAILED | {"id":"Rejected.Agent","displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | context without a leading slash   | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","context":"agent","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | upstream with an unsupported URL  | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"file:///etc/passwd"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}           |
      | credential type with no value     | 400    | VALIDATION_FAILED | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099","auth":{"type":"api-key","header":"X-Key"}}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | empty object                      | 400    | VALIDATION_FAILED | {}                                                                                                                                                                                                                                      |
      | unknown project                   | 404    | PROJECT_NOT_FOUND | {"displayName":"Rejected","version":"v1.0","projectId":"no-such-agent-project","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |
      | unknown associated gateway        | 404    | GATEWAY_NOT_FOUND | {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","associatedGateways":[{"id":"no-such-agent-gateway"}],"upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}} |

  @cp14-04 @type:negative
  Scenario: A handle is unique within the organization and a secret handle is never echoed
    Given I generate a unique resource name from "agent-dup" and store it as "agentHandle"
    And I generate a unique API context from "/agent-dup" and store it as "agentContext"
    And I generate a unique resource name from "agent-missing-secret" and store it as "missingSecret"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Duplicate Agent                                  |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201

    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Another Agent                                    |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 409
    And the JSON response field "code" should be "AGENT_PROXY_EXISTS"

    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName        | Secret Agent                                                                             |
      | projectId          | ${CTX:projectHandle}                                                                     |
      | context            | ${CTX:agentContext}                                                                      |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                             |
      | upstream.main.auth | {"type":"api-key","header":"X-Upstream-Key","value":"{{ secret \"${CTX:missingSecret}\" }}"} |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                                         |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the response body should not contain "${CTX:missingSecret}"

    When I send a "PUT" request to the control plane at "/agent-proxies/no-such-agent-proxy" with body:
      """
      {"displayName":"Missing","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

    When I send a "DELETE" request to the control plane at "/agent-proxies/no-such-agent-proxy"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

  @cp14-05 @type:negative
  Scenario Outline: A typed A2A configuration that could never deploy is rejected: <case>
    When I send a "POST" request to the control plane at "/agent-proxies" with body:
      """
      {"displayName":"Rejected","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":<a2a>}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    Examples:
      | case                                 | a2a                                                                                                                                                                                                                          |
      | unregistered protocol version        | {"protocolVersion":"9.9","transports":[{"protocolBinding":"JSONRPC"}]}                                                                                                                                                       |
      | missing transports                   | {"protocolVersion":"1.0"}                                                                                                                                                                                                    |
      | empty transports                     | {"protocolVersion":"1.0","transports":[]}                                                                                                                                                                                    |
      | unknown binding                      | {"protocolVersion":"1.0","transports":[{"protocolBinding":"GRPC"}]}                                                                                                                                                          |
      | duplicate binding                    | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC","pathPrefix":"/a"},{"protocolBinding":"JSONRPC","pathPrefix":"/b"}]}                                                                                     |
      | unknown operation                    | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"operationConfigs":{"operations":[{"name":"Teleport"}]}}                                                                                               |
      | duplicate operation                  | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"operationConfigs":{"operations":[{"name":"SendMessage"},{"name":"SendMessage"}]}}                                                                     |
      | unknown typed field                  | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"routing":"sticky"}                                                                                                                                    |
      | card path with a trailing slash      | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"path":"/cards/"}}}                                                                                                            |
      | card path with a query               | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"path":"/card.json?v=1"}}}                                                                                                     |
      | unknown card mode                    | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"cached"}}}                                                                                                             |
      | managed card without content         | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"managed"}}}                                                                                                            |
      | passthrough card with content        | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"passthrough","content":{"name":"x"}}}}                                                                                 |
      | managed card with rewriteUrls        | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"managed","rewriteUrls":true,"content":{"name":"x","description":"x","version":"1.0.0","supportedInterfaces":[],"capabilities":{},"defaultInputModes":[],"defaultOutputModes":[],"skills":[]}}}} |
      | non-boolean rewriteUrls              | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"passthrough","rewriteUrls":"yes"}}}                                                                                    |
      | managed card missing required keys   | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"managed","content":{"name":"Incomplete","description":"No skills or interfaces"}}}}                                    |
      | typed signing on the public card     | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"passthrough","signing":{"enabled":false}}}}                                                                            |
      | typed signing on the protected card  | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"protected":{"mode":"passthrough","signing":{"enabled":false}}}}                                                                         |
      | protected card with a path           | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"protected":{"mode":"passthrough","path":"/extended.json"}}}                                                                             |
      | protected card with policies         | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"protected":{"mode":"passthrough","policies":[{"name":"cors","version":"v1"}]}}}                                                         |
      | protected card without a mode        | {"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"protected":{"rewriteUrls":true}}}                                                                                                      |

  @cp14-06
  Scenario: Agent proxy mutations need a mutating scope and bodies must be JSON
    Given I generate a unique resource name from "agent-authz" and store it as "agentHandle"
    And I generate a unique API context from "/agent-authz" and store it as "agentContext"
    # The publisher holds only ap:agent_proxy:manage for this resource, so every operation below is
    # authorized through the :manage alternative.
    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" as "publisher" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Authz Agent                                      |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 201
    And the JSON response field "createdBy" should be "publisher"

    # The developer holds only ap:agent_proxy:read.
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}" as "developer"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/agent-proxies?limit=100" as "developer"
    Then the response status code should be 200

    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" as "developer" with values:
      | displayName | Denied Agent                                     |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" as "developer" with values:
      | displayName | Denied Agent                                     |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}" as "developer"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments" as "developer" with body:
      """
      {"name": "denied", "base": "current", "gatewayId": "it-gateway"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

    When I send a "POST" request to the control plane at "/agent-proxies" with header "Content-Type" set to "text/plain" with body:
      """
      {"displayName":"Plain","version":"v1.0","projectId":"${CTX:projectHandle}","upstream":{"main":{"url":"http://a2a-trip-planner:9099"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}
      """
    Then the response status code should be 415
    And the JSON response field "code" should be "UNSUPPORTED_MEDIA_TYPE"

    When I send a "POST" request to the control plane at "/agent-proxies" with body:
      """
      {"displayName":
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" as "publisher" with values:
      | displayName | Authz Agent Renamed                              |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200
    And the JSON response field "createdBy" should be "publisher"
    And the JSON response field "updatedBy" should be "publisher"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}" as "publisher"
    Then the response status code should be 204

  @cp14-23
  Scenario: A project cannot be deleted while it owns an Agent proxy
    Given I generate a unique resource name from "agent-owned-project" and store it as "ownedProject"
    And I create a project "${CTX:ownedProject}" on the control plane
    And I generate a unique resource name from "agent-owned" and store it as "agentHandle"
    And I generate a unique API context from "/agent-owned" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Owned Agent                                      |
      | projectId   | ${CTX:ownedProject}                              |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201

    When I send a "DELETE" request to the control plane at "/projects/${CTX:ownedProject}"
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should be "Project has associated Agent proxies"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

    When I send a "DELETE" request to the control plane at "/projects/${CTX:ownedProject}"
    Then the response status code should be 204
