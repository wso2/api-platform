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

@apip-cloud
Feature: APIP cloud REST API lifecycle
  As an APIP cloud user
  I want to create, deploy, invoke, and delete a REST API
  So that both the control plane and gateway data plane are verified

  @TC_APIP_API_001 @TC_APIP_API_002 @TC_APIP_API_003 @TC_APIP_API_004 @TC_APIP_API_005 @TC_APIP_API_006 @TC_APIP_API_007 @TC_APIP_API_008 @ID_APIP_BASIC_001
  Scenario: A REST API can be deployed and invoked through the gateway
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the synthetic REST API data-plane response should contain post 1
    When I delete the synthetic REST API
    Then the synthetic REST API should eventually be absent

  @TC_APIP_API_009 @ID_APIP_BASIC_002
  Scenario: Creating an API whose name and version are already in use is rejected
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I create a synthetic REST API in the default project
    And I submit a synthetic REST API creation request with:
      | displayName | Synthetic ${CTX:cloudAPIID} |
      | version     | 1.0.0                       |
    Then the cloud API response status code should be 409

  @TC_APIP_API_010 @ID_APIP_BASIC_003
  Scenario Outline: Creating an API with an invalid context or version is rejected
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I submit a synthetic REST API creation request with:
      | <field> | <value> |
    Then the cloud API response status code should be 400

    Examples:
      | field   | value        |
      | context | no-slash     |
      | context | /bad context |
      | version | 1 0          |
      | version | 1/0          |

  @TC_APIP_API_011 @ID_APIP_BASIC_004
  Scenario: An API deployed with security enabled refuses an unauthenticated call
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 401

  @TC_APIP_API_012 @ID_APIP_BASIC_005
  Scenario: An API deployed with security enabled accepts a valid API key
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I create an API key for the synthetic REST API
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" with header "X-API-Key" set to the API key until status 200
    Then the synthetic REST API data-plane response should contain post 1

  @TC_APIP_API_013 @ID_APIP_BASIC_006
  Scenario: A request to a path the API does not expose is answered with not found by the gateway
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/not-exposed" until status 404

  @TC_APIP_API_014 @ID_APIP_BASIC_007
  Scenario: A new version of an API routes independently of the previous version
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | operations | [{"name":"getPostOne","request":{"method":"GET","path":"/posts/1"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I create a new version "2.0.0" of the synthetic REST API with:
      | operations | [{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    And I deploy the new API version to the active gateway
    Then the new API version deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the synthetic REST API data-plane response should contain post 1
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudVersionAPIID}/2.0.0/posts/2" until status 200
    And the synthetic REST API data-plane response should contain post 2
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/2" until status 404

  @TC_APIP_API_015 @ID_APIP_BASIC_008
  Scenario: Undeploying an API stops the gateway serving it
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I undeploy the synthetic REST API
    Then the synthetic REST API deployment should become UNDEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 404

  @TC_APIP_API_016 @ID_APIP_BASIC_009
  Scenario: Redeploying an API serves the updated definition without interrupting traffic
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I update the synthetic REST API with:
      | operations | [{"name":"getPostOne","request":{"method":"GET","path":"/posts/1"}},{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    And I redeploy the synthetic REST API to the active gateway
    Then a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" should keep returning 200 until the redeployment is DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/2" until status 200
    And the synthetic REST API data-plane response should contain post 2

  @TC_APIP_RES_008 @ID_APIP_BASIC_010
  Scenario: A request above the gateway body limit is refused before it reaches the upstream
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | operations | [{"name":"createPost","request":{"method":"POST","path":"/posts"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts" with a body of 1 KiB until status 201
    And a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts" with a body of 2 MiB should be refused as too large

  @TC_APIP_RES_009 @ID_APIP_BASIC_011
  Scenario: A chunked request above the gateway body limit is terminated
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | operations | [{"name":"createPost","request":{"method":"POST","path":"/posts"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts" with a body of 1 KiB until status 201
    And a chunked "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts" with a body of 4 MiB should be refused as too large

  @TC_APIP_RES_012 @ID_APIP_BASIC_012
  Scenario: A request to an upstream that cannot be reached gets the router's failure and no server detail
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an unreachable backend is available for deployment overrides
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                              |
      | gatewayId   | ${CTX:cloudGatewayID}                                       |
      | parameters  | {"productionEndpoint":"${CTX:cloudUnreachableBackendURL}"} |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become DEPLOYED
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until the gateway routes it
    Then the gateway response should be the router's documented failure for an unreachable backend
