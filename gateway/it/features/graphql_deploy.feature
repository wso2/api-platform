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

Feature: Test GraphQL API CRUD and connectivity (gateway-only path)
    As a gateway operator
    I want to deploy a GraphQLApi configuration directly against the gateway-controller
    So that I can verify routing, policy enforcement, and CRUD behavior with no control plane involved

    Background:
        Given the gateway services are running

    # ==================== HAPPY PATH: DEPLOY, INVOKE, UPDATE, DELETE ====================

    Scenario: Deploy a GraphQL API and invoke it successfully
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: countries-graphql-e2e-v1
            spec:
              displayName: Countries E2E
              version: v1
              context: /countries-e2e
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "kind" should be "GraphQLApi"
        And I wait for the endpoint "http://localhost:8080/countries-e2e" to be ready with method "POST" and body '{"query":"{ countries { code name } }"}'

        When I send a POST request to "http://localhost:8080/countries-e2e" with body:
            """
            {"query":"{ countries { code name } }"}
            """
        Then the response should be successful
        And the response should be valid JSON
        And the response body should contain "{ countries { code name } }"

        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "countries-graphql-e2e-v1"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: Update a deployed GraphQL API's upstream, and verify the change takes effect
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: countries-update-e2e-v1
            spec:
              displayName: Countries Update E2E
              version: v1
              context: /countries-update-e2e
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        And I wait for 2 seconds

        Given I authenticate using basic auth as "admin"
        When I update the GraphQL API "countries-update-e2e-v1" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: countries-update-e2e-v1
            spec:
              displayName: Countries Update E2E v2
              version: v1
              context: /countries-update-e2e
              upstream:
                main:
                  url: http://sample-backend:9080/graphql-v2
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "spec.displayName" should be "Countries Update E2E v2"
        And I wait for the endpoint "http://localhost:8080/countries-update-e2e" to be ready with method "POST" and body '{"query":"{ countries { code } }"}'

        When I send a POST request to "http://localhost:8080/countries-update-e2e" with body:
            """
            {"query":"{ countries { code } }"}
            """
        Then the response should be successful
        And the response body should contain "/graphql-v2"

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "countries-update-e2e-v1"
        Then the response should be successful

    # ==================== MUTATIONS ====================
    # There is no separate "mutation support" at the gateway-controller/Envoy
    # layer, and the artifact carries no schema field at all: a mutation is
    # just another POST body sent to the same single route a query uses,
    # since GraphQL always resolves to exactly one route, never a
    # per-operation list like REST's. This scenario proves that
    # pass-through directly by sending a mutation-shaped body against an
    # artifact that is byte-for-byte identical in shape to every query-only
    # artifact in this file.

    Scenario: A mutation query is proxied through the same single route as a query, unmodified
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: blog-mutation-e2e-v1
            spec:
              displayName: Blog Mutation E2E
              version: v1
              context: /blog-mutation-e2e
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        And I wait for the endpoint "http://localhost:8080/blog-mutation-e2e" to be ready with method "POST" and body '{"query":"mutation { createPost(input: { title: \"hi\", body: \"hi\" }) { post { id } } }"}'

        When I send a POST request to "http://localhost:8080/blog-mutation-e2e" with body:
            """
            {"query":"mutation { createPost(input: { title: \"hi\", body: \"hi\" }) { post { id } } }"}
            """
        Then the response should be successful
        And the response should be valid JSON
        And the response body should contain "createPost(input:"

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "blog-mutation-e2e-v1"
        Then the response should be successful

    # ==================== LABELS ====================

    Scenario: Deploy a GraphQL API with labels and verify they are stored
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: labeled-graphql-v1
              labels:
                environment: production
                team: graphql-team
            spec:
              displayName: Labeled GraphQL
              version: v1
              context: /labeled-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        And I wait for 2 seconds

        Given I authenticate using basic auth as "admin"
        When I get the GraphQL API "labeled-graphql-v1"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "metadata.labels.environment" should be "production"
        And the JSON response field "metadata.labels.team" should be "graphql-team"

        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "labeled-graphql-v1"
        Then the response should be successful

    Scenario: Deploy a GraphQL API with invalid labels (spaces in keys) should fail
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: invalid-labels-graphql-v1
              labels:
                "Invalid Key": value
            spec:
              displayName: Invalid Labels GraphQL
              version: v1
              context: /invalid-labels-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        And the response body should contain "Configuration validation failed"

    # ==================== LIST ====================

    Scenario: List GraphQL APIs when none exist
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/graphql-apis?displayName=NoSuchGraphQLAPIDisplayName"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 0

    Scenario: List GraphQL APIs with pagination parameters
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/graphql-apis?limit=10&offset=0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: List GraphQL APIs with displayName filter
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: filter-test-graphql-v1
            spec:
              displayName: UniqueGraphQLFilterTest
              version: v1
              context: /filter-test-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/graphql-apis?displayName=UniqueGraphQLFilterTest"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the response body should contain "UniqueGraphQLFilterTest"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "filter-test-graphql-v1"
        Then the response should be successful

    Scenario: List GraphQL APIs with version filter
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: version-test-graphql-v99
            spec:
              displayName: Version Test GraphQL
              version: v99
              context: /version-test-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/graphql-apis?version=v99"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "version-test-graphql-v99"
        Then the response should be successful

    Scenario: List GraphQL APIs with context filter
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: context-filter-graphql-v1
            spec:
              displayName: Context Filter GraphQL
              version: v1
              context: /context-filter-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/graphql-apis?context=/context-filter-graphql"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the response body should contain "context-filter-graphql-v1"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "context-filter-graphql-v1"
        Then the response should be successful

    # ==================== GET ERROR CASES ====================

    Scenario: Get non-existent GraphQL API returns 404
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/graphql-apis/non-existent-graphql-id"
        Then the response status should be 404
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Get GraphQL API with invalid ID format returns 404
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/graphql-apis/invalid@graphql#id"
        Then the response status should be 404
        And the response should be valid JSON

    # ==================== UPDATE ERROR CASES ====================

    Scenario: Update non-existent GraphQL API returns 404
        Given I authenticate using basic auth as "admin"
        When I update the GraphQL API "non-existent-graphql-update" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: non-existent-graphql-update
            spec:
              displayName: Ghost
              version: v1
              context: /ghost
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response status should be 404
        And the response should be valid JSON

    Scenario: Update GraphQL API with a metadata.name that does not match the path id returns 400
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: mismatch-graphql-v1
            spec:
              displayName: Mismatch GraphQL
              version: v1
              context: /mismatch-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful

        Given I authenticate using basic auth as "admin"
        When I update the GraphQL API "mismatch-graphql-v1" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: a-different-name-v1
            spec:
              displayName: Mismatch GraphQL
              version: v1
              context: /mismatch-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response status should be 400
        And the response should be valid JSON
        And the response body should contain "does not match path id"

        # A rejected mismatched update must not persist under either handle: the
        # original resource must still exist, unchanged, under its own path handle...
        Given I authenticate using basic auth as "admin"
        When I get the GraphQL API "mismatch-graphql-v1"
        Then the response should be successful
        And the JSON response field "spec.displayName" should be "Mismatch GraphQL"

        # ...and the rejected body's handle must never have been created.
        Given I authenticate using basic auth as "admin"
        When I get the GraphQL API "a-different-name-v1"
        Then the response status should be 404

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "mismatch-graphql-v1"
        Then the response should be successful

    Scenario: Update GraphQL API with invalid JSON body returns error
        Given I authenticate using basic auth as "admin"
        When I send a PUT request to the "gateway-controller" service at "/graphql-apis/some-graphql" with body:
            """
            { invalid json body
            """
        Then the response should be a client error
        And the response should be valid JSON

    # ==================== DELETE ERROR CASES ====================

    Scenario: Delete non-existent GraphQL API returns 404
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "non-existent-graphql-delete"
        Then the response status should be 404
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    # ==================== CREATE VALIDATION ERROR CASES ====================

    Scenario: Deploy GraphQL API with missing required fields returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: incomplete-graphql-v1
            spec:
              displayName: Incomplete GraphQL
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        And the response body should contain "Configuration validation failed"

    Scenario: Deploy GraphQL API with a context that does not start with '/' returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: bad-context-graphql-v1
            spec:
              displayName: Bad Context GraphQL
              version: v1
              context: bad-context-no-slash
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the response body should contain "context must start with"

    Scenario: Deploy GraphQL API without an upstream returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: missing-upstream-graphql-v1
            spec:
              displayName: Missing Upstream GraphQL
              version: v1
              context: /missing-upstream-graphql
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the response body should contain "Upstream URL is required"

    Scenario: Deploy GraphQL API with an invalid upstream URL scheme returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: bad-scheme-graphql-v1
            spec:
              displayName: Bad Scheme GraphQL
              version: v1
              context: /bad-scheme-graphql
              upstream:
                main:
                  url: ftp://sample-backend:9080/graphql
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the response body should contain "must use http or https"

    Scenario: Deploy GraphQL API with an upstream URL missing a host returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: no-host-graphql-v1
            spec:
              displayName: No Host GraphQL
              version: v1
              context: /no-host-graphql
              upstream:
                main:
                  url: http:///graphql
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the response body should contain "must include a host"

    Scenario: Deploy GraphQL API with an unsupported kind value returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: NotAGraphQLApi
            metadata:
              name: wrong-kind-graphql-v1
            spec:
              displayName: Wrong Kind GraphQL
              version: v1
              context: /wrong-kind-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be a client error
        And the response should be valid JSON

    Scenario: Deploy GraphQL API with invalid JSON body returns error
        Given I authenticate using basic auth as "admin"
        When I send a POST request to the "gateway-controller" service at "/graphql-apis" with body:
            """
            { this is not valid json content
            """
        Then the response should be a client error
        And the response should be valid JSON

    Scenario: Deploy duplicate GraphQL API returns conflict
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: duplicate-graphql-v1
            spec:
              displayName: Duplicate GraphQL
              version: v1
              context: /duplicate-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful

        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: duplicate-graphql-v1
            spec:
              displayName: Duplicate GraphQL
              version: v1
              context: /duplicate-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response status should be 409
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "duplicate-graphql-v1"
        Then the response should be successful

    # ==================== ROUTING CORRECTNESS: SINGLE POST ROUTE ONLY ====================

    Scenario: A GraphQL API exposes exactly one POST route - other methods to the same context are not routed
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: single-route-graphql-v1
            spec:
              displayName: Single Route GraphQL
              version: v1
              context: /single-route-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
            """
        Then the response should be successful
        And I wait for the endpoint "http://localhost:8080/single-route-graphql" to be ready with method "POST" and body '{"query":"{ ping }"}'

        When I send a GET request to "http://localhost:8080/single-route-graphql"
        Then the response status code should be 404

        # POST still works on the same context
        When I send a POST request to "http://localhost:8080/single-route-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response should be successful

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "single-route-graphql-v1"
        Then the response should be successful

    # ==================== POLICY ENFORCEMENT ====================

    Scenario: GraphQL API with jwt-auth rejects requests without a token and accepts a valid one
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: jwt-auth-graphql-v1
            spec:
              displayName: JWT Auth GraphQL
              version: v1
              context: /jwt-auth-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
              policies:
                - name: jwt-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
            """
        Then the response should be successful
        And I wait for 5 seconds

        And I clear all headers
        When I send a POST request to "http://localhost:8080/jwt-auth-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response status code should be 401

        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I send a POST request to "http://localhost:8080/jwt-auth-graphql" with the JWT token and body:
            """
            {"query":"{ ping }"}
            """
        Then the response status code should be 200

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "jwt-auth-graphql-v1"
        Then the response should be successful

    Scenario: GraphQL API with set-headers correctly mutates the proxied response
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: set-headers-graphql-v1
            spec:
              displayName: Set Headers GraphQL
              version: v1
              context: /set-headers-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
              policies:
                - name: set-headers
                  version: v1
                  params:
                    response:
                      headers:
                        - name: X-GraphQL-Test-Marker
                          value: graphql-policy-works
            """
        Then the response should be successful
        And I wait for the endpoint "http://localhost:8080/set-headers-graphql" to be ready with method "POST" and body '{"query":"{ ping }"}'

        When I send a POST request to "http://localhost:8080/set-headers-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response should be successful
        And the response header "X-GraphQL-Test-Marker" should be "graphql-policy-works"

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "set-headers-graphql-v1"
        Then the response should be successful

    Scenario: GraphQL API with cors does not handle a preflight request - confirmed limitation, not yet supported
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: cors-graphql-v1
            spec:
              displayName: CORS GraphQL
              version: v1
              context: /cors-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
              policies:
                - name: cors
                  version: v1
                  params:
                    allowedOrigins:
                      - "http://example.com"
                    allowedMethods:
                      - "POST"
                    allowedHeaders:
                      - "Content-Type"
            """
        Then the response should be successful
        And I wait for the endpoint "http://localhost:8080/cors-graphql" to be ready with method "POST" and body '{"query":"{ ping }"}'

        # CONFIRMED via this test (not assumed): a GraphQL API resolves to
        # exactly one POST route with an Exact path/method match, so an
        # OPTIONS preflight never matches that route at all — Envoy 404s
        # before the cors policy, or any policy, ever runs. REST's cors
        # preflight support (which relies on an explicit `- method: OPTIONS`
        # entry in operations[]) does NOT carry over to GraphQL; there is no
        # operations[] to add one to. This is a genuine, current limitation —
        # not yet supported.
        Given I clear all headers
        When I set header "Origin" to "http://example.com"
        And I set header "Access-Control-Request-Method" to "POST"
        And I send an OPTIONS request to "http://localhost:8080/cors-graphql"
        Then the response status code should be 404

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "cors-graphql-v1"
        Then the response should be successful

    Scenario: GraphQL API with basic-ratelimit enforces its configured limit
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: ratelimit-graphql-v1
            spec:
              displayName: RateLimit GraphQL
              version: v1
              context: /ratelimit-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
              policies:
                - name: basic-ratelimit
                  version: v1
                  params:
                    limits:
                      - requests: 3
                        duration: "1h"
            """
        Then the response should be successful
        # The readiness wait below itself counts as the 1st request against
        # the 3-request limit — only 2 more successful requests remain before
        # the limit trips, not 3.
        And I wait for the endpoint "http://localhost:8080/ratelimit-graphql" to be ready with method "POST" and body '{"query":"{ ping }"}'

        When I send a POST request to "http://localhost:8080/ratelimit-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response should be successful
        When I send a POST request to "http://localhost:8080/ratelimit-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response should be successful
        When I send a POST request to "http://localhost:8080/ratelimit-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response status code should be 429

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "ratelimit-graphql-v1"
        Then the response should be successful

    # ==================== SANDBOX ROUTING ====================
    # GraphQLAPIConfigData has no vhosts override field (unlike RestApi) — the
    # transformer always resolves sandbox routing against the gateway's own
    # default main/sandbox vhosts (gateway-controller/pkg/transform/graphql.go,
    # t.routerConfig.VHosts.{Main,Sandbox}.Default), proven at the unit level
    # by TestGraphQLAPITransformer_SandboxProducesSecondRoute. This scenario
    # is the missing E2E half: does traffic carrying the sandbox Host header
    # actually land on the sandbox cluster, not just "does the route exist."
    #
    # The gateway's built-in default sandbox vhost is the WILDCARD pattern
    # "sandbox-*" (gateway-controller/pkg/config/config.go), not a fixed
    # literal like REST's per-API "sandbox.local" example — GraphQL has no
    # per-API vhosts override to set a literal, so the Host header used below
    # must actually match "sandbox-*" (start with "sandbox-"), matching what
    # this codebase's own default resolves to.

    Scenario: A GraphQL API with a sandbox upstream routes sandbox-host traffic to the sandbox cluster
        Given I authenticate using basic auth as "admin"
        When I deploy this GraphQL configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: GraphQLApi
            metadata:
              name: sandbox-graphql-v1
            spec:
              displayName: Sandbox GraphQL
              version: v1
              context: /sandbox-graphql
              upstream:
                main:
                  url: http://sample-backend:9080/graphql
                sandbox:
                  url: http://sample-backend:9080/sandbox/graphql
            """
        Then the response should be successful
        And I wait for the endpoint "http://localhost:8080/sandbox-graphql" to be ready with method "POST" and body '{"query":"{ ping }"}'

        When I clear all headers
        And I send a POST request to "http://localhost:8080/sandbox-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response should be successful
        And the JSON response field "path" should be "/graphql"

        When I clear all headers
        And I set request host to "sandbox-graphql-e2e"
        And I send a POST request to "http://localhost:8080/sandbox-graphql" with body:
            """
            {"query":"{ ping }"}
            """
        Then the response should be successful
        And the JSON response field "path" should be "/sandbox/graphql"

        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the GraphQL API "sandbox-graphql-v1"
        Then the response should be successful
