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

# Runs in the mcp-policies block, whose platform-gateway carries the
# mcp-jwt-auth.toml overlay registering the "mock-jwks" issuer that jwt-auth
# validates against. graphql_deploy.feature's own policy scenarios (set-headers,
# cors, basic-ratelimit) need no such overlay and stay in gateway-core.
@graphql-policies
Feature: GraphQL API policy enforcement requiring a registered JWT issuer
  As a gateway operator
  I want to secure a GraphQL API with jwt-auth
  So that only requests carrying a valid token reach the upstream

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A GraphQL API with jwt-auth rejects requests without a token and accepts a valid one
    Given I generate a unique resource name from "graphql-jwt-auth" and store it as "graphqlName"
    And I generate a unique value from "graphql-jwt-auth" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-jwt-auth" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
      | spec.policies          | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 401 with body:
      """
      {"query":"{ ping }"}
      """

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "graphqlToken"
    And I set header "Authorization" to "Bearer ${CTX:graphqlToken}"
    And I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response status code should be 200

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful
