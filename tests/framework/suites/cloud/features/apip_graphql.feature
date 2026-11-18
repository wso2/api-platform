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

@apip-cloud @GRAPHQL
Feature: APIP cloud GraphQL API lifecycle
  As an APIP cloud user
  I want to create, deploy, change and secure a GraphQL API
  So that the gateway serves it the way its definition and its builds say

  @ID_APIP_GRAPHQL_001
  Scenario: A GraphQL API created with its schema and deployed answers a query with the backend's data
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic GraphQL API in the default project
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until status 200
    And the GraphQL response should come from the GraphQL backend

  @ID_APIP_GRAPHQL_002
  Scenario: A mutation goes through the same route as a query and reaches the backend unchanged
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic GraphQL API in the default project
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until status 200
    And I send the GraphQL request "mutation { deletePost(id: 1) }" to the synthetic GraphQL API until status 200
    And the GraphQL response should come from the GraphQL backend

  @ID_APIP_GRAPHQL_003
  Scenario: After its upstream changes and it is redeployed, the gateway sends a GraphQL API's requests to the new upstream
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic GraphQL API in the default project
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until the GraphQL backend answers
    When I update the synthetic GraphQL API with:
      | upstream | ${CTX:cloudEchoBackendURL} |
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until the echo backend answers

  @ID_APIP_GRAPHQL_004
  Scenario: Replacing a GraphQL API's schema stores it, and the API returns the new schema
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I create a synthetic GraphQL API in the default project
    Then the synthetic GraphQL API should have the schema "type Query { hello: String }"
    When I update the synthetic GraphQL API with:
      | sdl | type Query { hello: String ping: String } |
    Then the synthetic GraphQL API should have the schema "type Query { hello: String ping: String }"

  @ID_APIP_GRAPHQL_005
  Scenario: Schema validation resolves a valid schema and reports an invalid one as unresolved with its errors
    Given I obtain an APIP cloud console token
    When I validate the GraphQL schema "type Query { hello: String }"
    Then the GraphQL schema should be reported as resolved
    When I validate the GraphQL schema "type Query {"
    Then the GraphQL schema should be reported as not resolved

  @ID_APIP_GRAPHQL_006 @type:negative
  Scenario: A GraphQL API whose schema does not match its schema source or does not parse is refused and not created, and one whose schema URL cannot be fetched is created with no schema
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And an echo backend is available for deployment overrides
    When I submit a synthetic GraphQL API creation request with:
      | schemaSource | inline                     |
      | sdlUrl       | ${CTX:cloudEchoBackendURL} |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "VALIDATION_FAILED"
    And the submitted GraphQL API should not exist
    When I submit a synthetic GraphQL API creation request with:
      | sdl | type Query { |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "VALIDATION_FAILED"
    And the cloud API response body should contain "could not be parsed"
    And the submitted GraphQL API should not exist
    When I submit a synthetic GraphQL API creation request with:
      | schemaSource | url                                   |
      | sdlUrl       | ${CTX:cloudEchoBackendURL}/status/404 |
    Then the cloud API response status code should be 201
    And the submitted GraphQL API should exist
    And the submitted GraphQL API should have an empty schema

  @ID_APIP_GRAPHQL_007
  Scenario: Deploying a GraphQL API's build serves it, deploying an earlier build serves that one again, and undeploying stops it
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic GraphQL API in the default project
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until the GraphQL backend answers
    When I update the synthetic GraphQL API with:
      | upstream | ${CTX:cloudEchoBackendURL} |
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until the echo backend answers
    When I deploy build 1 of the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until the GraphQL backend answers
    When I undeploy the synthetic GraphQL API
    Then the synthetic GraphQL API deployment should become UNDEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until status 404

  @ID_APIP_GRAPHQL_008
  Scenario: A GraphQL API with API key security refuses a request without a key, accepts its keys, and refuses a revoked key while another key still works
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic GraphQL API in the default project with:
      | policies | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    And I deploy the synthetic GraphQL API to the active gateway
    Then the synthetic GraphQL API deployment should become DEPLOYED
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API until status 401
    When I create an API key for the synthetic GraphQL API
    And I create another API key for the synthetic GraphQL API
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API with header "X-API-Key" set to the API key until status 200
    Then the GraphQL response should come from the GraphQL backend
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API with header "X-API-Key" set to the other API key until status 200
    When I revoke the API key of the synthetic GraphQL API
    Then I send the GraphQL request "{ hello }" to the synthetic GraphQL API with header "X-API-Key" set to the API key until status 401
    And I send the GraphQL request "{ hello }" to the synthetic GraphQL API with header "X-API-Key" set to the other API key until status 200
    And the GraphQL response should come from the GraphQL backend
