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

@graphql-deploy
Feature: GraphQL API CRUD and connectivity
  As a gateway operator
  I want to deploy a GraphQL API configuration against the gateway-controller
  So that I can verify routing, policy enforcement, and CRUD behavior

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy a GraphQL API and invoke it successfully
    Given I generate a unique resource name from "graphql-e2e" and store it as "graphqlName"
    And I generate a unique value from "graphql-e2e" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-e2e" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-e2e" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ countries { code name } }"}
      """

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ countries { code name } }"}
      """
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "{ countries { code name } }"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Update a deployed GraphQL API's upstream, and verify the change takes effect
    Given I generate a unique resource name from "graphql-update" and store it as "graphqlName"
    And I generate a unique value from "graphql-update" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-update" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-update" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    When I update GraphQL API "${CTX:graphqlName}" from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} v2 |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql-v2 |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "spec.displayName" should be "${CTX:graphqlDisplayName} v2"

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ countries { code } }"}
      """

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ countries { code } }"}
      """
    Then the response should be successful
    And the response body should contain "/graphql-v2"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  # There is no separate "mutation support" at the gateway-controller/Envoy layer, and the
  # artifact carries no schema field at all: a mutation is just another POST body sent to
  # the same single route a query uses, since GraphQL always resolves to exactly one route,
  # never a per-operation list like REST's. This proves that pass-through directly, against
  # an artifact byte-for-byte identical in shape to every query-only artifact in this file.
  Scenario: A mutation query is proxied through the same single route as a query, unmodified
    Given I generate a unique resource name from "graphql-mutation" and store it as "graphqlName"
    And I generate a unique value from "graphql-mutation" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-mutation" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-mutation" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"mutation { createPost(input: { title: \"hi\", body: \"hi\" }) { post { id } } }"}
      """

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"mutation { createPost(input: { title: \"hi\", body: \"hi\" }) { post { id } } }"}
      """
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "createPost(input:"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: Deploy a GraphQL API with labels and verify they are stored
    Given I generate a unique resource name from "graphql-labeled" and store it as "graphqlName"
    And I generate a unique value from "graphql-labeled" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-labeled" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-labeled" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion} |
      | name                    | ${CTX:graphqlName}        |
      | spec.displayName        | ${CTX:graphqlDisplayName} |
      | spec.version            | ${CTX:graphqlVersion}     |
      | spec.context            | ${CTX:graphqlContext}     |
      | spec.upstream.main.url  | http://testbench:3000/graphql |
      | metadata.labels         | {"environment":"production","team":"graphql-team"} |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I get the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful
    And the JSON response field "metadata.labels.environment" should be "production"
    And the JSON response field "metadata.labels.team" should be "graphql-team"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: Deploy a GraphQL API with invalid labels (spaces in keys) fails
    Given I generate a unique resource name from "graphql-invalid-labels" and store it as "graphqlName"
    And I generate a unique API version from "graphql-invalid-labels" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-invalid-labels" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion} |
      | name                    | ${CTX:graphqlName}        |
      | spec.displayName        | Invalid Labels GraphQL    |
      | spec.version            | ${CTX:graphqlVersion}     |
      | spec.context            | ${CTX:graphqlContext}     |
      | spec.upstream.main.url  | http://testbench:3000/graphql |
      | metadata.labels         | {"Invalid Key":"value"}   |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"

  # ==================== LIST ====================

  Scenario: List GraphQL APIs when none exist matching a filter
    Given I generate a unique value from "no-such-graphql-api" and store it as "missingDisplayName"
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis?displayName=${CTX:missingDisplayName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0

  Scenario: List GraphQL APIs with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List GraphQL APIs filtered by displayName
    Given I generate a unique resource name from "graphql-filter" and store it as "graphqlName"
    And I generate a unique value from "UniqueGraphQLFilterTest" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-filter" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-filter" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis?displayName=${CTX:graphqlDisplayName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:graphqlDisplayName}"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: List GraphQL APIs filtered by version
    Given I generate a unique resource name from "graphql-version-filter" and store it as "graphqlName"
    And I generate a unique value from "graphql-version-filter" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-version-filter" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-version-filter" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis?version=${CTX:graphqlVersion}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:graphqlDisplayName}"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: List GraphQL APIs filtered by context
    Given I generate a unique resource name from "graphql-context-filter" and store it as "graphqlName"
    And I generate a unique value from "graphql-context-filter" and store it as "graphqlDisplayName"
    And I generate a unique API version from "graphql-context-filter" and store it as "graphqlVersion"
    And I generate a unique API context from "/graphql-context-filter" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | ${CTX:graphqlVersion}     |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis?context=${CTX:graphqlContext}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:graphqlName}"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  # ==================== GET / UPDATE / DELETE ERROR CASES ====================

  Scenario: Get a non-existent GraphQL API returns 404
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/non-existent-graphql-id"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get a GraphQL API with an invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/invalid@graphql#id"
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Update a non-existent GraphQL API returns 404
    Given I generate a unique resource name from "graphql-nonexistent-update" and store it as "graphqlName"
    When I update GraphQL API "${CTX:graphqlName}" from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | Nonexistent GraphQL Update |
      | spec.version           | v1.0                       |
      | spec.context           | /nonexistent-graphql-update |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Update a GraphQL API with a metadata.name that does not match the path id returns 400
    Given I generate a unique resource name from "graphql-mismatch" and store it as "graphqlName"
    And I generate a unique resource name from "graphql-mismatch-other" and store it as "otherGraphqlName"
    And I generate a unique API context from "/graphql-mismatch" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | Mismatch GraphQL          |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    When I update GraphQL API "${CTX:graphqlName}" from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:otherGraphqlName}   |
      | spec.displayName       | Mismatch GraphQL          |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response status code should be 400
    And the response should be valid JSON
    And the response body should contain "does not match path id"

    # A rejected mismatched update must not persist under either handle: the original
    # resource must still exist unchanged under its own path handle...
    When I get the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful
    And the JSON response field "spec.displayName" should be "Mismatch GraphQL"

    # ...and the rejected body's handle must never have been created.
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/${CTX:otherGraphqlName}"
    Then the response status code should be 404

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: Update a GraphQL API with an invalid JSON body returns an error
    When I send a "PUT" request to the "gateway-controller" service at "/graphql-apis/some-graphql" with body:
      """
      { invalid json body
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Delete a non-existent GraphQL API returns 404
    When I delete the GraphQL API "non-existent-graphql-delete"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== CREATE VALIDATION ERROR CASES ====================

  Scenario: Deploy a GraphQL API with missing required fields returns an error
    Given I generate a unique resource name from "graphql-incomplete" and store it as "graphqlName"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion       | ${CTX:gatewaySpecVersion} |
      | name             | ${CTX:graphqlName}        |
      | spec.displayName | Incomplete GraphQL        |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"

  Scenario: Deploy a GraphQL API with a context that does not start with '/' returns 400
    Given I generate a unique resource name from "graphql-bad-context" and store it as "graphqlName"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | Bad Context GraphQL       |
      | spec.version           | v1.0                       |
      | spec.context           | bad-context-no-slash      |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be a client error
    And the response should be valid JSON
    And the response body should contain "context must start with"

  Scenario: Deploy a GraphQL API without an upstream returns 400
    Given I generate a unique resource name from "graphql-missing-upstream" and store it as "graphqlName"
    And I generate a unique API context from "/graphql-missing-upstream" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion       | ${CTX:gatewaySpecVersion} |
      | name             | ${CTX:graphqlName}        |
      | spec.displayName | Missing Upstream GraphQL  |
      | spec.version     | v1.0                       |
      | spec.context     | ${CTX:graphqlContext}     |
    Then the response should be a client error
    And the response should be valid JSON
    And the response body should contain "Upstream URL is required"

  Scenario: Deploy a GraphQL API with an invalid upstream URL scheme returns 400
    Given I generate a unique resource name from "graphql-bad-scheme" and store it as "graphqlName"
    And I generate a unique API context from "/graphql-bad-scheme" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | Bad Scheme GraphQL        |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | ftp://testbench:3000/graphql |
    Then the response should be a client error
    And the response should be valid JSON
    And the response body should contain "must use http or https"

  Scenario: Deploy a GraphQL API with an upstream URL missing a host returns 400
    Given I generate a unique resource name from "graphql-no-host" and store it as "graphqlName"
    And I generate a unique API context from "/graphql-no-host" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | No Host GraphQL           |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http:///graphql           |
    Then the response should be a client error
    And the response should be valid JSON
    And the response body should contain "must include a host"

  Scenario: Deploy a GraphQL API with an unsupported kind value returns an error
    Given I generate a unique resource name from "graphql-wrong-kind" and store it as "graphqlName"
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "NotAGraphQLApi",
        "metadata": { "name": "${CTX:graphqlName}" },
        "spec": {
          "displayName": "Wrong Kind GraphQL",
          "version": "v1.0",
          "context": "/wrong-kind-graphql",
          "upstream": { "main": { "url": "http://testbench:3000/graphql" } }
        }
      }
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Deploy a GraphQL API with an invalid JSON body returns an error
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis" with body:
      """
      { this is not valid json content
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Deploying a duplicate GraphQL API returns a conflict
    Given I generate a unique resource name from "graphql-duplicate" and store it as "graphqlName"
    And I generate a unique value from "graphql-duplicate" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-duplicate" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response status code should be 409
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  # ==================== ROUTING CORRECTNESS: SINGLE POST ROUTE ONLY ====================

  Scenario: A GraphQL API exposes exactly one POST route - other methods to the same context are not routed
    Given I generate a unique resource name from "graphql-single-route" and store it as "graphqlName"
    And I generate a unique value from "graphql-single-route" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-single-route" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ ping }"}
      """

    When I send a "GET" request to "${CTX:graphqlContext}"
    Then the response status code should be 404

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response should be successful

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  # ==================== POLICY ENFORCEMENT ====================
  # jwt-auth against the "mock-jwks" issuer is covered separately in
  # graphql_policies.feature, which runs in the mcp-policies block where the
  # mcp-jwt-auth.toml overlay registers that issuer. This block's platform-gateway
  # has no such overlay, so a jwt-auth scenario here would 500 on an unknown issuer.

  Scenario: A GraphQL API with set-headers correctly mutates the proxied response
    Given I generate a unique resource name from "graphql-set-headers" and store it as "graphqlName"
    And I generate a unique value from "graphql-set-headers" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-set-headers" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
      | spec.policies          | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-GraphQL-Test-Marker","value":"graphql-policy-works"}]}}}] |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ ping }"}
      """

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response should be successful
    And the response header "X-GraphQL-Test-Marker" should be "graphql-policy-works"

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  # CONFIRMED via this test (not assumed): a GraphQL API resolves to exactly one POST route
  # with an Exact path/method match, so an OPTIONS preflight never matches that route at
  # all - Envoy 404s before the cors policy, or any policy, ever runs. REST's cors preflight
  # support (which relies on an explicit "- method: OPTIONS" entry in operations[]) does not
  # carry over to GraphQL; there is no operations[] to add one to. This is a genuine, current
  # limitation, not yet supported.
  Scenario: A GraphQL API with cors does not handle a preflight request - confirmed limitation
    Given I generate a unique resource name from "graphql-cors" and store it as "graphqlName"
    And I generate a unique value from "graphql-cors" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-cors" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["POST"],"allowedHeaders":["Content-Type"]}}] |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ ping }"}
      """

    When I clear all headers
    And I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I send a "OPTIONS" request to "${CTX:graphqlContext}"
    Then the response status code should be 404

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  Scenario: A GraphQL API with basic-ratelimit enforces its configured limit
    Given I generate a unique resource name from "graphql-ratelimit" and store it as "graphqlName"
    And I generate a unique value from "graphql-ratelimit" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-ratelimit" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:graphqlName}        |
      | spec.displayName       | ${CTX:graphqlDisplayName} |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:graphqlContext}     |
      | spec.upstream.main.url | http://testbench:3000/graphql |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}] |
    Then the response should be successful

    # The readiness wait below itself counts as the 1st request against the 3-request
    # limit - only 2 more successful requests remain before the limit trips.
    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ ping }"}
      """

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response should be successful
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response should be successful
    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response status code should be 429

    When I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful

  # ==================== SANDBOX ROUTING ====================
  # GraphQLAPIConfigData has no vhosts override field (unlike RestApi) - the transformer
  # always resolves sandbox routing against the gateway's own default main/sandbox vhosts.
  # The gateway's built-in default sandbox vhost is the wildcard pattern "sandbox-*", not a
  # fixed literal like REST's per-API "sandbox.local" example, so the Host header used below
  # must actually match "sandbox-*" (start with "sandbox-").
  Scenario: A GraphQL API with a sandbox upstream routes sandbox-host traffic to the sandbox cluster
    Given I generate a unique resource name from "graphql-sandbox" and store it as "graphqlName"
    And I generate a unique value from "graphql-sandbox" and store it as "graphqlDisplayName"
    And I generate a unique API context from "/graphql-sandbox" and store it as "graphqlContext"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion                | ${CTX:gatewaySpecVersion} |
      | name                      | ${CTX:graphqlName}        |
      | spec.displayName          | ${CTX:graphqlDisplayName} |
      | spec.version              | v1.0                       |
      | spec.context              | ${CTX:graphqlContext}     |
      | spec.upstream.main.url    | http://testbench:3000/graphql |
      | spec.upstream.sandbox.url | http://testbench:3000/sandbox/graphql |
    Then the response should be successful

    And I send a "POST" request to "${CTX:graphqlContext}" until status 200 with body:
      """
      {"query":"{ ping }"}
      """

    When I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response should be successful
    And the JSON response field "path" should be "/graphql"

    When I clear all headers
    And I set request host to "sandbox-graphql-e2e"
    And I send a "POST" request to "${CTX:graphqlContext}" with body:
      """
      {"query":"{ ping }"}
      """
    Then the response should be successful
    And the JSON response field "path" should be "/sandbox/graphql"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the GraphQL API "${CTX:graphqlName}"
    Then the response should be successful
