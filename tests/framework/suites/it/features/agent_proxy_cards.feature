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

@agent-proxy @agent-proxy-cards
Feature: Agent Cards are previewed through the control plane and served by the gateway
  As an API publisher
  I want the control plane to preview an agent's Agent Card and the gateway to serve the card I
  configured
  So that the card page reflects the upstream without hammering it, and deployed cards are the
  ones I authored

  # The Agent Card fixture counts every card request each scenario scope receives, which is the
  # evidence for "the upstream was (not) contacted". The block runs its platform-api with a 20s
  # positive and an 8s negative cache TTL (overlays/platform-api-agent-card-cache.toml).
  #
  # Cache restart clearing is disruptive and lives in agent_proxy_recovery.feature. A disabled
  # cache, LRU eviction at the entry and byte ceilings, a fetch racing an invalidation, and
  # organization isolation need configuration or a second tenant a shared block cannot provide;
  # they are covered by the focused platform-api tests named in the Section 14 coverage record.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "agent-card-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @cp14-07 @type:smoke
  Scenario: A direct URL preview fetches the card live, with or without supplied credentials
    Given I generate a unique resource name from "card-direct" and store it as "cardScope"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "auth" as "authUrl"
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "${CTX:okUrl}"}
      """
    Then the response status code should be 200
    And the response header "Content-Type" should be "application/json"
    And the JSON response field "name" should be "Card ${CTX:cardScope}"
    And the JSON response field "skills[0].id" should be "card-fixture"
    And the response header "Age" should not exist
    And the response header "Cache-Control" should not exist
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    # The direct form is an authoring action and is never cached.
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "${CTX:okUrl}"}
      """
    Then the response status code should be 200
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "${CTX:authUrl}", "auth": {"type": "api-key", "header": "X-Card-Key", "value": "card-key-${CTX:cardScope}"}}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Protected source card ${CTX:cardScope}"
    And the response body should not contain "card-key-${CTX:cardScope}"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "${CTX:authUrl}"}
      """
    Then the response status code should be 503
    And the JSON response field "code" should be "AGENT_PROXY_UPSTREAM_UNREACHABLE"
    And the Agent Card fixture should have received 4 requests for scope "${CTX:cardScope}"

  @cp14-07
  Scenario: A stored-handle fetch uses the stored endpoint and stored credentials and persists nothing
    Given I generate a unique resource name from "card-stored" and store it as "cardScope"
    And I generate a unique resource name from "card-stored-agent" and store it as "agentHandle"
    And I generate a unique resource name from "card-stored-secret" and store it as "secretHandle"
    And I generate a unique API context from "/card-stored" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "auth" as "authUrl"
    And I create a secret "${CTX:secretHandle}" with value "card-key-${CTX:cardScope}" via the control plane
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                 | ${CTX:agentHandle}                                                                         |
      | displayName        | Stored Card Agent                                                                          |
      | projectId          | ${CTX:projectHandle}                                                                       |
      | context            | ${CTX:agentContext}                                                                        |
      | upstreamUrl        | ${CTX:authUrl}                                                                             |
      | upstream.main.auth | {"type":"api-key","header":"X-Card-Key","value":"{{ secret \"${CTX:secretHandle}\" }}"}    |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                                           |
    And the response status code should be 201

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Protected source card ${CTX:cardScope}"
    And the response header "Age" should be "0"
    And the response header "Cache-Control" should be "max-age=20"
    And the response body should not contain "${CTX:secretHandle}"
    And the response body should not contain "card-key-${CTX:cardScope}"
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    # An unsaved preview cannot borrow the stored credential, even for the same endpoint.
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "${CTX:authUrl}"}
      """
    Then the response status code should be 503
    And the JSON response field "message" should be "The upstream agent rejected the credentials the control plane presented for its Agent Card."

    # The fetched card is display state only: the Agent proxy still has no stored card.
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 200
    And the JSON response field "a2a.agentCard" should not exist
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments"
    Then the response status code should be 200
    And the JSON response field "count" should be 0

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "no-such-agent-proxy"}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with header "Content-Type" set to "text/plain" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 415
    And the JSON response field "code" should be "UNSUPPORTED_MEDIA_TYPE"
    # The stored fetch and the credential-less preview each reached the upstream; the 404 and the
    # 415 were refused before any outbound request.
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

  @cp14-07 @type:negative
  Scenario Outline: A malformed fetch request is rejected before any credential lookup or outbound request: <case>
    Given I generate a unique resource name from "card-invalid" and store it as "cardScope"
    And I generate a unique resource name from "card-invalid-agent" and store it as "agentHandle"
    And I generate a unique API context from "/card-invalid" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Invalid Fetch Agent                              |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | ${CTX:okUrl}                                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      <body>
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the Agent Card fixture should have received 0 requests for scope "${CTX:cardScope}"

    Examples:
      | case                          | body                                                                                                                            |
      | empty object                  | {}                                                                                                                              |
      | auth alone                    | {"auth": {"type": "api-key", "header": "X-Card-Key", "value": "v"}}                                                             |
      | handle with url               | {"agentProxyId": "${CTX:agentHandle}", "url": "${CTX:okUrl}"}                                                                   |
      | handle with auth              | {"agentProxyId": "${CTX:agentHandle}", "auth": {"type": "api-key", "header": "X-Card-Key", "value": "v"}}                        |
      | all three fields              | {"agentProxyId": "${CTX:agentHandle}", "url": "${CTX:okUrl}", "auth": {"type": "api-key", "header": "X-Card-Key", "value": "v"}} |
      | handle with a null url        | {"agentProxyId": "${CTX:agentHandle}", "url": null}                                                                             |
      | handle with a null auth       | {"agentProxyId": "${CTX:agentHandle}", "auth": null}                                                                            |
      | empty handle                  | {"agentProxyId": ""}                                                                                                            |
      | null handle                   | {"agentProxyId": null}                                                                                                          |
      | empty url                     | {"url": ""}                                                                                                                     |
      | url with an unknown field     | {"url": "${CTX:okUrl}", "cache": false}                                                                                         |
      | handle with an unknown field  | {"agentProxyId": "${CTX:agentHandle}", "refresh": true}                                                                         |
      | url that is not a URL         | {"url": "not a url"}                                                                                                            |

  @cp14-08 @type:negative
  Scenario Outline: An upstream failure is reported as a sterile 503 and changes nothing: <case>
    Given I generate a unique resource name from "card-fail" and store it as "cardScope"
    And I generate a unique resource name from "card-fail-agent" and store it as "agentHandle"
    And I generate a unique API context from "/card-fail" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "<mode>" as "fixtureUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Failing Card Agent                               |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | <upstream>                                       |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 503
    And the JSON response field "code" should be "AGENT_PROXY_UPSTREAM_UNREACHABLE"
    And the JSON response field "message" should be "<message>"
    And the JSON response should have field "trackingId"
    And the response header "Age" should be "0"
    And the response header "Cache-Control" should not exist
    And the response body should not contain "testbench"
    And the response body should not contain "a2a-trip-planner"
    And the response body should not contain "agent-card-upstream.invalid"
    And the response body should not contain "not an agent card"
    And the Agent Card fixture should have received <requests> requests for scope "${CTX:cardScope}"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Failing Card Agent"
    And the JSON response field "a2a.agentCard" should not exist
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments"
    Then the response status code should be 200
    And the JSON response field "count" should be 0

    Examples:
      | case                 | mode      | upstream                              | requests | message                                                                                    |
      | connection refused   | ok        | http://testbench:1                    | 0        | The control plane could not reach the upstream agent to retrieve its Agent Card.           |
      | DNS failure          | ok        | http://agent-card-upstream.invalid    | 0        | The control plane could not reach the upstream agent to retrieve its Agent Card.           |
      | TLS failure          | ok        | https://a2a-trip-planner:9099         | 0        | The control plane could not reach the upstream agent to retrieve its Agent Card.           |
      | timeout              | slow      | ${CTX:fixtureUrl}                     | 1        | The control plane could not reach the upstream agent to retrieve its Agent Card.           |
      | upstream error       | status500 | ${CTX:fixtureUrl}                     | 1        | The control plane could not reach the upstream agent to retrieve its Agent Card.           |
      | credentials rejected | auth      | ${CTX:fixtureUrl}                     | 1        | The upstream agent rejected the credentials the control plane presented for its Agent Card. |
      | not a card           | malformed | ${CTX:fixtureUrl}                     | 1        | The upstream agent did not return a usable Agent Card.                                     |

  @cp14-08
  Scenario: A failed display fetch never blocks update or deployment
    Given I generate a unique resource name from "card-unblocked" and store it as "agentHandle"
    And I generate a unique API context from "/card-unblocked" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Unblocked Agent                                  |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://testbench:1                               |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 503

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | Unblocked Agent Renamed                          |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://testbench:1                               |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200
    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    Then the response status code should be 201
    And the JSON response field "status" should be "DEPLOYING"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    Then the JSON response field "statusReason" should not exist

  @cp14-09
  Scenario: A fetched card is served from cache until it expires, and no-cache refreshes it
    Given I generate a unique resource name from "card-cache" and store it as "cardScope"
    And I generate a unique resource name from "card-cache-agent" and store it as "agentHandle"
    And I generate a unique API context from "/card-cache" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Cached Card Agent                                |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | ${CTX:okUrl}                                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the response header "Age" should be "0"
    And the response header "Cache-Control" should be "max-age=20"
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" until the response header "Age" matches "^[1-9][0-9]*$" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Card ${CTX:cardScope}"
    And the response header "Cache-Control" should be "max-age=20"
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with header "Cache-Control" set to "no-cache" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the response header "Age" should be "0"
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

    # The refresh replaced the entry, so the next ordinary read is a hit again.
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

    When I fetch the Agent Card of Agent proxy "${CTX:agentHandle}" via the control plane until the Agent Card fixture has received 3 requests for scope "${CTX:cardScope}"
    Then the response status code should be 200
    And the response header "Age" should be "0"

  @cp14-09 @type:negative
  Scenario: A fetch failure is cached for the shorter negative TTL
    Given I generate a unique resource name from "card-negative" and store it as "cardScope"
    And I generate a unique resource name from "card-negative-agent" and store it as "agentHandle"
    And I generate a unique API context from "/card-negative" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "status500" as "failingUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Negative Cache Agent                             |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | ${CTX:failingUrl}                                |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 503
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" until the response header "Age" matches "^[1-9][0-9]*$" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 503
    And the JSON response field "code" should be "AGENT_PROXY_UPSTREAM_UNREACHABLE"
    And the response header "Cache-Control" should not exist
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    When I fetch the Agent Card of Agent proxy "${CTX:agentHandle}" via the control plane until the Agent Card fixture has received 2 requests for scope "${CTX:cardScope}"
    Then the response status code should be 503
    And the response header "Age" should be "0"

  @cp14-10
  Scenario: Agent proxies sharing an upstream URL never share a cache entry
    Given I generate a unique resource name from "card-shared" and store it as "cardScope"
    And I generate a unique resource name from "card-shared-good" and store it as "goodHandle"
    And I generate a unique resource name from "card-shared-bad" and store it as "badHandle"
    And I generate a unique API context from "/card-shared-good" and store it as "goodContext"
    And I generate a unique API context from "/card-shared-bad" and store it as "badContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "auth" as "authUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                 | ${CTX:goodHandle}                                                            |
      | displayName        | Shared Upstream Good                                                         |
      | projectId          | ${CTX:projectHandle}                                                         |
      | context            | ${CTX:goodContext}                                                           |
      | upstreamUrl        | ${CTX:authUrl}                                                               |
      | upstream.main.auth | {"type":"api-key","header":"X-Card-Key","value":"card-key-${CTX:cardScope}"} |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                             |
    And the response status code should be 201
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                 | ${CTX:badHandle}                                              |
      | displayName        | Shared Upstream Bad                                           |
      | projectId          | ${CTX:projectHandle}                                          |
      | context            | ${CTX:badContext}                                             |
      | upstreamUrl        | ${CTX:authUrl}                                                |
      | upstream.main.auth | {"type":"api-key","header":"X-Card-Key","value":"wrong-key"}  |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]              |
    And the response status code should be 201

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:goodHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Protected source card ${CTX:cardScope}"

    # Keyed by Agent proxy, never by URL: the cached card is not handed to the other Agent proxy.
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:badHandle}"}
      """
    Then the response status code should be 503
    And the response body should not contain "Protected source card"
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" as "developer" with body:
      """
      {"agentProxyId": "${CTX:goodHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Protected source card ${CTX:cardScope}"
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:badHandle}"}
      """
    Then the response status code should be 503
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

  @cp14-10
  Scenario: Updating or deleting an Agent proxy drops its cached card immediately
    Given I generate a unique resource name from "card-invalidate" and store it as "cardScope"
    And I generate a unique resource name from "card-invalidate-agent" and store it as "agentHandle"
    And I generate a unique API context from "/card-invalidate" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "alternate" as "alternateUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Invalidated Card Agent                           |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | ${CTX:okUrl}                                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Card ${CTX:cardScope}"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | Invalidated Card Agent                           |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | ${CTX:alternateUrl}                              |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Alternate card ${CTX:cardScope}"
    And the response header "Age" should be "0"
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"

    # Switching the card mode is a relevant change too.
    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName   | Invalidated Card Agent                                |
      | projectId     | ${CTX:projectHandle}                                  |
      | context       | ${CTX:agentContext}                                   |
      | upstreamUrl   | ${CTX:alternateUrl}                                   |
      | transports    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]      |
      | a2a.agentCard | {"public":{"mode":"passthrough","rewriteUrls":false}} |
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the Agent Card fixture should have received 3 requests for scope "${CTX:cardScope}"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 204
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"
    And the response header "Age" should not exist
    And the Agent Card fixture should have received 3 requests for scope "${CTX:cardScope}"

  @cp14-18
  Scenario: A managed public card authored in the control plane is served by the gateway
    Given I generate a unique resource name from "card-managed" and store it as "agentHandle"
    And I generate a unique API context from "/card-managed" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id            | ${CTX:agentHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | displayName   | Managed Card Agent                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | projectId     | ${CTX:projectHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | context       | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | upstreamUrl   | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | transports    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Managed Trip Planner","description":"Served by the gateway","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"managed_skill","name":"Only on the managed card"}]}}} |
    And the response status code should be 201
    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json" until status 200
    Then the JSON response field "name" should be "Managed Trip Planner"
    And the JSON response field "skills[0].id" should be "managed_skill"
    And the response body should not contain "plan_trip"

  @cp14-18
  Scenario: A passthrough card keeps its rewrite default, honours the opt-out and a custom path
    Given I generate a unique resource name from "card-rewrite" and store it as "rewriteHandle"
    And I generate a unique resource name from "card-verbatim" and store it as "verbatimHandle"
    And I generate a unique API context from "/card-rewrite" and store it as "rewriteContext"
    And I generate a unique API context from "/card-verbatim" and store it as "verbatimContext"
    # The omitted card block is passthrough at the well-known path with URL rewriting on.
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:rewriteHandle}                                                                                  |
      | displayName | Rewritten Card Agent                                                                                  |
      | projectId   | ${CTX:projectHandle}                                                                                  |
      | context     | ${CTX:rewriteContext}                                                                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                                                                          |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]   |
    And the response status code should be 201
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id            | ${CTX:verbatimHandle}                                                                                |
      | displayName   | Verbatim Card Agent                                                                                  |
      | projectId     | ${CTX:projectHandle}                                                                                 |
      | context       | ${CTX:verbatimContext}                                                                               |
      | upstreamUrl   | http://a2a-trip-planner:9099                                                                         |
      | transports    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]  |
      | a2a.agentCard | {"public":{"mode":"passthrough","path":"/cards/public.json","rewriteUrls":false}}                    |
    And the response status code should be 201
    And the JSON response field "a2a.agentCard.public.rewriteUrls" should be "false"

    When I deploy the Agent proxy "${CTX:rewriteHandle}" to the gateway via the control plane and store the deployment id as "rewriteDeployment"
    And I deploy the Agent proxy "${CTX:verbatimHandle}" to the gateway via the control plane and store the deployment id as "verbatimDeployment"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:rewriteHandle}/deployments/${CTX:rewriteDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:verbatimHandle}/deployments/${CTX:verbatimDeployment}" until the JSON field "status" is "DEPLOYED"

    And I send a "GET" request to "${CTX:rewriteContext}/.well-known/agent-card.json" until status 200
    Then the response body should contain "plan_trip"
    And the response body should not contain "book_trip"
    And the response body should not contain "http://a2a-trip-planner:9099"

    When I send a "GET" request to "${CTX:verbatimContext}/cards/public.json" until status 200
    Then the response body should contain "plan_trip"
    And the response body should contain "http://a2a-trip-planner:9099"
    # A custom path replaces the default route rather than aliasing it.
    When I send a "GET" request to "${CTX:verbatimContext}/.well-known/agent-card.json"
    Then the response status code should be 404

  @cp14-18
  Scenario: A protected card requires authentication on both bindings
    Given I generate a unique resource name from "card-protected" and store it as "agentHandle"
    And I generate a unique API context from "/card-protected" and store it as "agentContext"
    And I generate a unique value from "card-protected-key" and store it as "keyValue"
    And I generate a unique resource name from "card-protected-key-id" and store it as "keyId"
    # No authentication policy is attached, so the protected card must refuse every caller.
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id            | ${CTX:agentHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | displayName   | Protected Card Agent                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | projectId     | ${CTX:projectHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | context       | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | upstreamUrl   | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | transports    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","content":{"name":"Protected Trip Planner","description":"Gateway-managed extended card","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"gateway_managed_skill","name":"Only on the protected card"}]}}} |
    And the response status code should be 201
    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "firstDeployment"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:firstDeployment}" until the JSON field "status" is "DEPLOYED"
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # Authenticating the extended-card operation serves the managed card on both bindings.
    Given I authenticate using basic auth as "admin"
    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName                     | Protected Card Agent                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | projectId                       | ${CTX:projectHandle}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | context                         | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | upstreamUrl                     | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | transports                      | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | a2a.operationConfigs.operations | [{"name":"GetExtendedAgentCard","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | a2a.agentCard                   | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","content":{"name":"Protected Trip Planner","description":"Gateway-managed extended card","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"gateway_managed_skill","name":"Only on the protected card"}]}}} |
    Then the response status code should be 200
    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "secondDeployment"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:secondDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "Protected card key", "apiKey": "${CTX:keyValue}"}
      """
    Then the response status code should be 201

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyValue}"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard" until status 200
    Then the JSON response field "name" should be "Protected Trip Planner"
    And the response body should contain "gateway_managed_skill"
    When I send a "POST" request to "${CTX:agentContext}" until status 200 with body:
      """
      {"jsonrpc": "2.0", "id": 2, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the JSON response field "result.name" should be "Protected Trip Planner"
    And the response body should contain "gateway_managed_skill"

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"
