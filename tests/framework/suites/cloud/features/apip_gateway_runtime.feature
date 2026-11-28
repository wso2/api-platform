# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

# The managed gateway is reached only through its own host: the cloud's edge answers any other
# Host header itself, so the on-premises "on any virtual host" scenarios have no cloud counterpart.
@apip-cloud
Feature: APIP cloud managed gateway runtime
  As an APIP cloud user
  I want the managed gateway to report its health, publish its signing keys and correlate every request
  So that I can monitor it, verify what it signs, and trace a call through it

  # The cloud counterpart of on-premises HEALTH-006.
  @ID_APIP_GATEWAY_RUNTIME_001
  Scenario: Runtime readiness reports the policy engine and its control-plane connection
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/_gateway-health/ready" until status 200
    Then the response header "Content-Type" should contain "application/json"
    And the response header "Cache-Control" should be "no-store"
    And the cloud API response should report:
      | status                       | ready |
      | checks.policy_engine.status  | up    |
      | checks.xds_connection.status | up    |

  # The cloud counterpart of on-premises HEALTH-007.
  @ID_APIP_GATEWAY_RUNTIME_002
  Scenario: Runtime liveness depends on no dependency check
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/_gateway-health/healthy" until status 200
    Then the cloud API response should report:
      | status | healthy |
    And the cloud API response should not report "checks.policy_engine"
    And the cloud API response should not report "checks.xds_connection"

  # The cloud counterpart of on-premises HEALTH-009.
  @ID_APIP_GATEWAY_RUNTIME_003 @type:negative
  Scenario: A path under the runtime health namespace that is not a health route is not answered as one
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/_gateway-health/ready/extra"
    Then the cloud API response status code should be 404
    And the cloud API response body should not contain "policy_engine"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/_gateway-health/ready"
    Then the cloud API response status code should be 200

  # The cloud counterpart of on-premises HEALTH-033.
  @ID_APIP_GATEWAY_RUNTIME_004 @type:negative
  Scenario Outline: Runtime health routes refuse the <method> method on <path>
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "<method>" request to "${CTX:cloudGatewayEndpoint}<path>"
    Then the cloud API response status code should be 405
    And the response header "Allow" should be "GET, HEAD"
    And the cloud API response body should not contain "policy_engine"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}<path>"
    Then the cloud API response status code should be 200

    Examples:
      | method | path                     |
      | POST   | /_gateway-health/ready   |
      | DELETE | /_gateway-health/ready   |
      | POST   | /_gateway-health/healthy |
      | DELETE | /_gateway-health/healthy |

  # The cloud counterpart of on-premises HEALTH-010. The platform accepts the API, since only the
  # gateway knows its reserved namespace, so the refusal is the gateway's: the deployment fails.
  @ID_APIP_GATEWAY_RUNTIME_005 @type:negative
  Scenario Outline: An API cannot claim a route in the reserved health namespace (<case>)
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | context    | <context>                                                                 |
      | operations | [{"name":"probe","request":{"method":"GET","path":"<operation>"}}] |
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become FAILED
    And the synthetic REST API deployment should report a status reason code
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/_gateway-health/ready"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | status | ready |

    Examples:
      | case                                 | context                                   | operation                    |
      | context inside the namespace         | /_gateway-health/${UNIQUE:probe}          | /status                      |
      | operation resolving to a health path | /${CTX:cloudAPIID}/v1                     | /../../_gateway-health/ready |

  # The cloud counterpart of on-premises "A successful response carries the generated request ID and
  # the upstream receives the same ID", without its upstream half: the cloud's testbench sits behind
  # another gateway that gives every request a new ID, so what the upstream receives cannot be seen.
  @ID_APIP_GATEWAY_RUNTIME_006
  Scenario: A successful response carries a request ID the gateway generated
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic REST API in the APIP test project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                      |
      | gatewayId   | ${CTX:cloudGatewayID}                               |
      | parameters  | {"productionEndpoint":"${CTX:cloudEchoBackendURL}"} |
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1"
    Then the cloud API response status code should be 200
    And the response should carry a request ID the gateway generated

  # The cloud counterpart of on-premises "A request ID sent by the client is returned unchanged". The
  # gateway keeps a client's ID, but the cloud's edge in front of the managed gateway replaces it with
  # a new one (see the scenario list entry).
  @ID_APIP_GATEWAY_RUNTIME_007 @known-issue
  Scenario: A request ID sent by the client is returned unchanged
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic REST API in the APIP test project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                      |
      | gatewayId   | ${CTX:cloudGatewayID}                               |
      | parameters  | {"productionEndpoint":"${CTX:cloudEchoBackendURL}"} |
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" with the header "X-Request-Id" set to "client-request-id-123"
    Then the cloud API response status code should be 200
    And the response header "X-Request-Id" should be "client-request-id-123"
    And the echoed request header "X-Request-Id" should be "client-request-id-123"

  # The cloud counterpart of on-premises "A request rejected by the policy engine still carries the
  # request ID". Its client-sent ID half is ID_APIP_GATEWAY_RUNTIME_007's known issue.
  @ID_APIP_GATEWAY_RUNTIME_008 @type:negative
  Scenario: A request rejected by the policy engine still carries the request ID
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | policies | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 401
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1"
    Then the cloud API response status code should be 401
    And the response should carry a request ID the gateway generated

  # The cloud counterpart of on-premises "A request to an unknown route still carries the request ID".
  @ID_APIP_GATEWAY_RUNTIME_009 @type:negative
  Scenario: A request to an unknown route still carries the request ID
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${UNIQUE:reqid-missing}/nothing"
    Then the cloud API response status code should be 404
    And the response should carry a request ID the gateway generated

  # The cloud counterpart of on-premises "A gateway error for an unreachable upstream still carries
  # the request ID".
  @ID_APIP_GATEWAY_RUNTIME_010 @type:negative
  Scenario: A gateway error for an unreachable upstream still carries the request ID
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                              |
      | gatewayId   | ${CTX:cloudGatewayID}                                       |
      | parameters  | {"productionEndpoint":"https://no-such-backend.invalid"} |
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 503
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1"
    Then the cloud API response status code should be 503
    And the response should carry a request ID the gateway generated

  # The cloud counterpart of on-premises "The gateway request ID replaces an X-Request-ID header set
  # by a policy".
  @ID_APIP_GATEWAY_RUNTIME_011
  Scenario: The gateway request ID replaces an X-Request-ID header set by a policy
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"probe","request":{"method":"GET","path":"/probe","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"headers":[{"name":"X-Request-ID","value":"policy-set-value"}]}}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/probe" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/probe"
    Then the cloud API response status code should be 200
    And the response header "X-Request-Id" should not be "policy-set-value"
    And the response should carry a request ID the gateway generated

  # The cloud counterpart of on-premises "A conditional request with the current ETag is answered
  # without a body".
  @ID_APIP_GATEWAY_RUNTIME_012
  Scenario: A conditional request for the JWK Set with the current ETag is answered without a body
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks" until status 200
    Then the response header "Content-Type" should contain "application/jwk-set+json"
    And the response header "Cache-Control" should be "public, max-age=300"
    And the response header "Access-Control-Allow-Origin" should be "*"
    And I store the response header "ETag" as "jwksETag"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks" with the header "If-None-Match" set to "${CTX:jwksETag}"
    Then the cloud API response status code should be 304
    And the response body should be empty
    And the response header "Cache-Control" should be "public, max-age=300"

  # The cloud counterpart of on-premises "A HEAD request returns the JWK Set headers".
  @ID_APIP_GATEWAY_RUNTIME_013
  Scenario: A HEAD request returns the JWK Set headers
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks" until status 200
    And I store the response header "ETag" as "jwksETag"
    And I send a "HEAD" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks"
    Then the cloud API response status code should be 200
    And the response header "ETag" should be "${CTX:jwksETag}"
    And the response header "Content-Type" should contain "application/jwk-set+json"
    And the response body should be empty

  # The cloud counterpart of on-premises "Methods other than GET and HEAD are rejected".
  @ID_APIP_GATEWAY_RUNTIME_014 @type:negative
  Scenario: Methods other than GET and HEAD on the JWK Set are rejected
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks"
    Then the cloud API response status code should be 405
    And the response header "Allow" should be "GET, HEAD"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks"
    Then the cloud API response status code should be 200

  # The cloud counterpart of on-premises "An API cannot claim the JWKS path while the endpoint is
  # enabled". The platform refuses the /.well-known context itself, before the gateway sees it.
  @ID_APIP_GATEWAY_RUNTIME_015 @type:negative
  Scenario: An API cannot claim the JWKS path while the endpoint is enabled
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I submit a synthetic REST API creation request with:
      | context    | /.well-known                                                |
      | operations | [{"name":"jwks","request":{"method":"GET","path":"/jwks"}}] |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "VALIDATION_FAILED"
    And the cloud API response body should contain "context"
    And the submitted synthetic REST API should not exist
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/.well-known/jwks"
    Then the cloud API response status code should be 200
    And the response header "Content-Type" should contain "application/jwk-set+json"

  # The cloud counterpart of on-premises "A buffered request body above the listener buffer limit is
  # rejected". On-premises it is a known issue (wso2/api-platform#4061): whether Envoy answers 413 or
  # the policy engine an empty 500 depends on timing. The cloud's edge passes the body on, so the 413
  # is the managed gateway's. The testbench refuses a body over 10 MiB with its own 413 ("Request
  # Entity Too Large"), so the body tells the gateway's 413 apart.
  @ID_APIP_GATEWAY_RUNTIME_016 @type:negative
  Scenario: A buffered request body above the listener buffer limit is rejected
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getPost","request":{"method":"GET","path":"/posts/1"}},{"name":"upload","request":{"method":"POST","path":"/upload","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":1,"max":33554432,"jsonPath":""}}}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/upload" with a JSON body of 17825792 bytes
    Then the cloud API response status code should be 413
    And the cloud API response body should contain "Payload Too Large"
