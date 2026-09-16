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

Feature: GraphQL API Analytics - Operation Identity and Error Enrichment
  As a platform administrator
  I want GraphQL requests to be published to analytics with operation-level identity,
  request-complexity signals, and response-level error/partial-success info, since a
  GraphQL API's single POST route otherwise carries no such signal in
  request.path/request.method
  So that I can monitor GraphQL query/mutation usage and failures the same way I can for REST

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  # mock-graphql-backend echoes the request body back verbatim as the response body. This is
  # essential here: the shared sample-service fixture always wraps every response in a fixed
  # {method,path,query,headers,body} envelope and can never produce a literal top-level
  # "errors" array the way a real GraphQL server does, so it can't exercise the response-phase
  # error enrichment under test below.

  Scenario: A named GraphQL mutation is published to analytics with its operation identity
    Given I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-analytics-mutation-e2e-v1
      spec:
        displayName: GraphQL Analytics Mutation E2E
        version: v1
        context: /graphql-analytics-mutation-e2e
        upstream:
          main:
            url: http://mock-graphql-backend:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/graphql-analytics-mutation-e2e" to be ready with method "POST" and body '{"operationName":"CreatePost","query":"mutation CreatePost($title: String!) { createPost(title: $title) { id } }"}'

    Given I reset the analytics collector
    When I send a POST request to "http://localhost:8080/graphql-analytics-mutation-e2e" with body:
      """
      {"operationName":"CreatePost","query":"mutation CreatePost($title: String!) { createPost(title: $title) { id } }","variables":{"title":"hi"}}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have request URI "/graphql-analytics-mutation-e2e"
    And the latest analytics event should have metadata field "apiType" with value "GraphQLApi"
    And the latest analytics event should have metadata field "graphqlAnalytics.operationName" with value "CreatePost"
    And the latest analytics event should have metadata field "graphqlAnalytics.operationType" with value "mutation"
    And the latest analytics event should have metadata field "graphqlAnalytics.requestType" with value "operation"
    And the latest analytics event should have metadata field "graphqlAnalytics.transport" with value "HTTP"
    And the latest analytics event should have metadata field "graphqlAnalytics.variableCount" with value "1"
    And the latest analytics event should have metadata field "graphqlAnalytics.variableNames" with value "[title]"
    And the latest analytics event should have metadata field "graphqlAnalytics.isBatched" with value "false"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the GraphQL API "graphql-analytics-mutation-e2e-v1"
    Then the response should be successful

  Scenario: An anonymous GraphQL query is published to analytics as operation type "query"
    Given I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-analytics-query-e2e-v1
      spec:
        displayName: GraphQL Analytics Query E2E
        version: v1
        context: /graphql-analytics-query-e2e
        upstream:
          main:
            url: http://mock-graphql-backend:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/graphql-analytics-query-e2e" to be ready with method "POST" and body '{"query":"{ countries { code } }"}'

    Given I reset the analytics collector
    When I send a POST request to "http://localhost:8080/graphql-analytics-query-e2e" with body:
      """
      {"query":"{ countries { code } }"}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have metadata field "graphqlAnalytics.operationType" with value "query"
    And the latest analytics event should have metadata field "graphqlAnalytics.requestType" with value "operation"
    And the latest analytics event should have metadata field "graphqlAnalytics.variableCount" with value "0"
    And the latest analytics event should have metadata field "graphqlAnalytics.isBatched" with value "false"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the GraphQL API "graphql-analytics-query-e2e-v1"
    Then the response should be successful

  Scenario: An introspection query is published to analytics with requestType "introspection"
    Given I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-analytics-introspection-e2e-v1
      spec:
        displayName: GraphQL Analytics Introspection E2E
        version: v1
        context: /graphql-analytics-introspection-e2e
        upstream:
          main:
            url: http://mock-graphql-backend:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/graphql-analytics-introspection-e2e" to be ready with method "POST" and body '{"query":"{ __schema { queryType { name } } }"}'

    Given I reset the analytics collector
    When I send a POST request to "http://localhost:8080/graphql-analytics-introspection-e2e" with body:
      """
      {"query":"{ __schema { queryType { name } } }"}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have metadata field "graphqlAnalytics.requestType" with value "introspection"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the GraphQL API "graphql-analytics-introspection-e2e-v1"
    Then the response should be successful

  Scenario: A batched GraphQL request is published to analytics with isBatched true
    Given I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-analytics-batch-e2e-v1
      spec:
        displayName: GraphQL Analytics Batch E2E
        version: v1
        context: /graphql-analytics-batch-e2e
        upstream:
          main:
            url: http://mock-graphql-backend:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/graphql-analytics-batch-e2e" to be ready with method "POST" and body '[{"query":"{ countries { code } }"}]'

    Given I reset the analytics collector
    When I send a POST request to "http://localhost:8080/graphql-analytics-batch-e2e" with body:
      """
      [{"operationName":"A","query":"query A { countries { code } }"},{"operationName":"B","query":"query B { countries { name } }"}]
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have metadata field "graphqlAnalytics.isBatched" with value "true"
    And the latest analytics event should have metadata field "graphqlAnalytics.transport" with value "HTTP"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the GraphQL API "graphql-analytics-batch-e2e-v1"
    Then the response should be successful

  Scenario: A GraphQL response carrying a top-level errors array is published to analytics as a failure
    Given I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-analytics-error-e2e-v1
      spec:
        displayName: GraphQL Analytics Error E2E
        version: v1
        context: /graphql-analytics-error-e2e
        upstream:
          main:
            url: http://mock-graphql-backend:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/graphql-analytics-error-e2e" to be ready with method "POST" and body '{"query":"query GetSecret { secret }"}'

    Given I reset the analytics collector
    # The "errors" field below is not something a real GraphQL client would ever send in a
    # request — it is here purely because mock-graphql-backend echoes the request body back
    # verbatim, so putting it in the request is how this test makes it appear in the response,
    # which is the only place the gateway's response-phase enrichment actually inspects it.
    When I send a POST request to "http://localhost:8080/graphql-analytics-error-e2e" with body:
      """
      {"operationName":"GetSecret","query":"query GetSecret { secret }","data":null,"errors":[{"message":"Not authorized","extensions":{"code":"FORBIDDEN"}}]}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have metadata field "graphqlAnalytics.isError" with value "true"
    And the latest analytics event should have metadata field "graphqlAnalytics.errorCount" with value "1"
    And the latest analytics event should have metadata field "graphqlAnalytics.errorCode" with value "FORBIDDEN"
    And the latest analytics event should have metadata field "graphqlAnalytics.isPartialSuccess" with value "false"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the GraphQL API "graphql-analytics-error-e2e-v1"
    Then the response should be successful

  Scenario: A GraphQL response carrying both data and errors is published to analytics as a partial success
    Given I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-analytics-partial-e2e-v1
      spec:
        displayName: GraphQL Analytics Partial Success E2E
        version: v1
        context: /graphql-analytics-partial-e2e
        upstream:
          main:
            url: http://mock-graphql-backend:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/graphql-analytics-partial-e2e" to be ready with method "POST" and body '{"query":"query GetCountries { countries { code } }"}'

    Given I reset the analytics collector
    # As with the error scenario above, "data"/"errors" are placed directly in the request only
    # because mock-graphql-backend echoes it back verbatim as the response body.
    When I send a POST request to "http://localhost:8080/graphql-analytics-partial-e2e" with body:
      """
      {"operationName":"GetCountries","query":"query GetCountries { countries { code } }","data":{"countries":[{"code":"US"}]},"errors":[{"message":"One field failed to resolve"}]}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have metadata field "graphqlAnalytics.isError" with value "true"
    And the latest analytics event should have metadata field "graphqlAnalytics.errorCount" with value "1"
    And the latest analytics event should have metadata field "graphqlAnalytics.isPartialSuccess" with value "true"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the GraphQL API "graphql-analytics-partial-e2e-v1"
    Then the response should be successful
