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

# testbench's graphql-backend service echoes the request body back verbatim as the response
# body. This is essential here: the generic reflect-style backends (services/backend,
# services/echo) always wrap every response in their own fixed envelope and can never
# produce a literal top-level "errors" array the way a real GraphQL server does, so they
# can't exercise the response-phase error enrichment under test below.
@graphql-analytics
Feature: GraphQL API analytics - operation identity and error enrichment
  As a platform administrator
  I want GraphQL requests published to analytics with operation-level identity,
  request-complexity signals, and response-level error/partial-success info, since a
  GraphQL API's single POST route otherwise carries no such signal in the request path
  or method
  So that I can monitor GraphQL query/mutation usage and failures the same way I can for REST

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: A named GraphQL mutation is published to analytics with its operation identity
    Given I generate a unique resource name from "graphql-analytics-mutation" and store it as "graphqlName"
    And I generate a unique value from "graphql-analytics-mutation" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-analytics-mutation" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3013     |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"operationName":"CreatePost","query":"mutation CreatePost($title: String!) { createPost(title: $title) { id } }"}
      """

    Given I reset the analytics collector
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"operationName":"CreatePost","query":"mutation CreatePost($title: String!) { createPost(title: $title) { id } }","variables":{"title":"hi"}}
      """
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "apiType" with value "GraphQLApi"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.operationName" with value "CreatePost"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.operationType" with value "mutation"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.requestType" with value "operation"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.transport" with value "HTTP"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.variableCount" with value "1"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.variableNames" with value "[title]"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isBatched" with value "false"
    And I wait for the analytics collector to settle

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: An anonymous GraphQL query is published to analytics as operation type "query"
    Given I generate a unique resource name from "graphql-analytics-query" and store it as "graphqlName"
    And I generate a unique value from "graphql-analytics-query" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-analytics-query" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3013     |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ countries { code } }"}
      """

    Given I reset the analytics collector
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ countries { code } }"}
      """
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.operationType" with value "query"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.requestType" with value "operation"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.variableCount" with value "0"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isBatched" with value "false"
    And I wait for the analytics collector to settle

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: An introspection query is published to analytics with requestType "introspection"
    Given I generate a unique resource name from "graphql-analytics-introspection" and store it as "graphqlName"
    And I generate a unique value from "graphql-analytics-introspection" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-analytics-introspection" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3013     |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ __schema { queryType { name } } }"}
      """

    Given I reset the analytics collector
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ __schema { queryType { name } } }"}
      """
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.requestType" with value "introspection"
    And I wait for the analytics collector to settle

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: A batched GraphQL request is published to analytics with isBatched true
    Given I generate a unique resource name from "graphql-analytics-batch" and store it as "graphqlName"
    And I generate a unique value from "graphql-analytics-batch" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-analytics-batch" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3013     |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      [{"query":"{ countries { code } }"}]
      """

    Given I reset the analytics collector
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      [{"operationName":"A","query":"query A { countries { code } }"},{"operationName":"B","query":"query B { countries { name } }"}]
      """
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isBatched" with value "true"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.transport" with value "HTTP"
    And I wait for the analytics collector to settle

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: A GraphQL response carrying a top-level errors array is published to analytics as a failure
    Given I generate a unique resource name from "graphql-analytics-error" and store it as "graphqlName"
    And I generate a unique value from "graphql-analytics-error" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-analytics-error" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3013     |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"query GetSecret { secret }"}
      """

    Given I reset the analytics collector
    # The "errors" field below is not something a real GraphQL client would ever send in a
    # request - it is here purely because testbench's graphql-backend echoes the request body
    # back verbatim, so putting it in the request is how this test makes it appear in the
    # response, which is the only place the gateway's response-phase enrichment inspects it.
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"operationName":"GetSecret","query":"query GetSecret { secret }","data":null,"errors":[{"message":"Not authorized","extensions":{"code":"FORBIDDEN"}}]}
      """
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isError" with value "true"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.errorCount" with value "1"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.errorCode" with value "FORBIDDEN"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isPartialSuccess" with value "false"
    And I wait for the analytics collector to settle

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: A GraphQL response carrying both data and errors is published to analytics as a partial success
    Given I generate a unique resource name from "graphql-analytics-partial" and store it as "graphqlName"
    And I generate a unique value from "graphql-analytics-partial" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-analytics-partial" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3013     |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"query GetCountries { countries { code } }"}
      """

    Given I reset the analytics collector
    # As with the error scenario above, "data"/"errors" are placed directly in the request only
    # because testbench's graphql-backend echoes it back verbatim as the response body.
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"operationName":"GetCountries","query":"query GetCountries { countries { code } }","data":{"countries":[{"code":"US"}]},"errors":[{"message":"One field failed to resolve"}]}
      """
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isError" with value "true"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.errorCount" with value "1"
    And the latest analytics event for path "${CTX:graphqlContext}" should have metadata field "graphqlAnalytics.isPartialSuccess" with value "true"
    And I wait for the analytics collector to settle

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful
