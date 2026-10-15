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

  Scenario: Creating an API whose name and version are already in use is rejected
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I create a synthetic REST API in the default project
    And I submit a synthetic REST API creation request with:
      | displayName | Synthetic ${CTX:cloudAPIID} |
      | version     | 1.0.0                       |
    Then the cloud API response status code should be 409

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

  Scenario: An API deployed with security enabled refuses an unauthenticated call
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 401

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

  Scenario: A request to a path the API does not expose is answered with not found by the gateway
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/not-exposed" until status 404

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
