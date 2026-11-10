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

@APIPORTAL @apiportal-rest-api
Feature: REST APIs are managed through the API Portal REST API
  As an API publisher
  I want to create, find, update and delete the REST APIs the portal lists
  So that developers see exactly the APIs I publish, and nobody else can change them

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level:
  # the /api-portal/api/v0.9/apis endpoints that the portal's admin UI and GitOps flows call.
  # Every API is rendered from resources/templates/api-portal-api.yaml; a value of null leaves
  # that field out of the request. Every refusal is paired with a request that succeeds, and every
  # denied write is followed by a read that shows nothing changed. Public pages, Markdown and
  # agent routes are read from the default view, which serves every API that carries no other
  # label. Catalogue search runs on this block's PostgreSQL database (whole-word full-text search
  # over metadata and definitions); SQLite and SQL Server search differently and are not covered here.

  @APIP-001 @type:smoke
  Scenario: A publisher lists, edits, deprecates and deletes a REST API, and every portal surface follows
    # A publisher manages a REST API from creation to deletion
    Given I generate a unique resource name from "portal-rest-api" and store it as "apiId"
    And I generate a unique resource name from "portal-rest-decoy" and store it as "decoyId"
    And I generate a unique resource name from "portal-rest-tag" and store it as "tag"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId}       |
      | name    | ${CTX:apiId}       |
      | version | v2.0               |
      | tags    | ["${CTX:tag}"]     |
    Then the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:decoyId}     |
      | name    | ${CTX:apiId}       |
      | version | v1.0               |
      | tags    | ["${CTX:tag}"]     |
    And the response status code should be 201
    And I generate a unique resource name from "portal-rest-version-decoy" and store it as "versionDecoyId"
    And I generate a unique resource name from "portal-rest-other-tag" and store it as "otherTag"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:versionDecoyId} |
      | name    | ${CTX:versionDecoyId} |
      | version | v2.0                  |
      | tags    | ["${CTX:otherTag}"]   |
    And the response status code should be 201

    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "RestApi"
    And the JSON response field "name" should be "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=definition.json" as "publisher"
    Then the response status code should be 200
    And the JSON response field "openapi" should be "3.0.3"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=missing.json" as "publisher"
    Then the response status code should be 404

    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:apiId}&version=v2.0&tags=${CTX:tag}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    And the response body should not contain "${CTX:decoyId}"
    And the response body should not contain "${CTX:versionDecoyId}"
    When I send an authenticated API Portal "GET" request to "/apis?query=${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    # The query is ' OR '1'='1, URL-encoded: search treats it as text, never as SQL.
    When I send an authenticated API Portal "GET" request to "/apis?query=%27%20OR%20%271%27%3D%271" as "publisher"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiId}"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId}         |
      | name    | ${CTX:apiId}-renamed |
      | version | v2.0                 |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:apiId}-renamed"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

    # The API listing pages through its results
    Given I generate a unique resource name from "portal-rest-page" and store it as "apiA"
    And I generate a unique resource name from "portal-rest-page" and store it as "apiB"
    And I generate a unique resource name from "portal-rest-page" and store it as "apiC"
    And I generate a unique resource name from "portal-rest-page-tag" and store it as "tag"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiA}    |
      | name | ${CTX:apiA}    |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiB}    |
      | name | ${CTX:apiB}    |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiC}    |
      | name | ${CTX:apiC}    |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis?tags=${CTX:tag}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "pagination.total" should be 3
    And the JSON response array field "list" should have 3 items
    When I send an authenticated API Portal "GET" request to "/apis?tags=${CTX:tag}&limit=2&offset=0" as "publisher"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response field "pagination.total" should be 3
    And the JSON response field "pagination.limit" should be 2
    And I store the JSON response field "list[0].id" as "firstPageA"
    And I store the JSON response field "list[1].id" as "firstPageB"
    When I send an authenticated API Portal "GET" request to "/apis?tags=${CTX:tag}&limit=2&offset=2" as "publisher"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.offset" should be 2
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:firstPageA}"
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:firstPageB}"

    # An updated API's page and specification show its new endpoints
    Given I generate a unique resource name from "portal-rest-moved" and store it as "apiId"
    And I generate a unique resource name from "portal-rest-moved-tag" and store it as "tag"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    And the response body should contain "https://backend.example.invalid"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                      | ${CTX:apiId}                               |
      | name                    | ${CTX:apiId}                               |
      | description             | Moved to its new backend ${CTX:apiId}      |
      | tags                    | ["${CTX:tag}"]                             |
      | endPoints.productionURL | https://moved.example.invalid/${CTX:apiId} |
      | endPoints.sandboxURL    | https://moved-sandbox.example.invalid      |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "description" should be "Moved to its new backend ${CTX:apiId}"
    And the JSON response field "tags[0]" should be "${CTX:tag}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    And the JSON response field "servers[0].url" should be "https://moved.example.invalid/${CTX:apiId}"
    And the JSON response field "servers[1].url" should be "https://moved-sandbox.example.invalid"
    And the response body should not contain "https://backend.example.invalid"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "Moved to its new backend ${CTX:apiId}"
    And the response body should contain "${CTX:tag}"

    # An API's owners and tags are stored, shown and found by search
    Given I generate a unique resource name from "portal-rest-owned" and store it as "apiId"
    And I generate a unique resource name from "portal-rest-owned-tag" and store it as "tag"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}                                                                                                                                                        |
      | name   | ${CTX:apiId}                                                                                                                                                        |
      | tags   | ["${CTX:tag}"]                                                                                                                                                      |
      | owners | {"technicalOwner": "Tech Owner", "technicalOwnerEmail": "tech@example.invalid", "businessOwner": "Business Owner", "businessOwnerEmail": "business@example.invalid"} |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "owners.technicalOwner" should be "Tech Owner"
    And the JSON response field "owners.technicalOwnerEmail" should be "tech@example.invalid"
    And the JSON response field "owners.businessOwner" should be "Business Owner"
    And the JSON response field "owners.businessOwnerEmail" should be "business@example.invalid"
    And the JSON response field "tags[0]" should be "${CTX:tag}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "${CTX:tag}"
    When I send an authenticated API Portal "GET" request to "/apis?query=${CTX:tag}" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis?tags=${CTX:tag}" as "developer"
    Then the JSON response array field "list" should have 1 item

    # A deprecated API stays listed and subscribed, and can be published again
    Given a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should not contain "dp-badge--deprecated"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId} |
      | name   | ${CTX:apiId} |
      | status | DEPRECATED   |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "status" should be "DEPRECATED"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "dp-badge--deprecated"
    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:apiId}" as "publisher"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "status" should be "ACTIVE"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId} |
      | name   | ${CTX:apiId} |
      | status | PUBLISHED    |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should not contain "dp-badge--deprecated"

    # A deleted API is gone from its page, its Markdown and the agent listings
    Given I generate a unique resource name from "portal-rest-deleted" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response body should contain "${CTX:apiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/llms.txt"
    Then the response body should contain "${CTX:apiId}"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=definition.json" as "publisher"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 404
    And the response body should contain "Page not found"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiId}"

  @APIP-002
  Scenario: An API's labels and offered plans follow every update
    # An API's labels follow every update
    Given I generate a unique resource name from "portal-rest-label-a" and store it as "labelA"
    And I generate a unique resource name from "portal-rest-label-b" and store it as "labelB"
    And I generate a unique resource name from "portal-rest-labelled" and store it as "apiId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelA":
      | id          | ${CTX:labelA} |
      | displayName | Label A       |
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelB":
      | id          | ${CTX:labelB} |
      | displayName | Label B       |
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}        |
      | name   | ${CTX:apiId}        |
      | labels | ["${CTX:labelA}"]   |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:labelA}"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}        |
      | name   | ${CTX:apiId}        |
      | labels | ["${CTX:labelB}"]   |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response body should contain "${CTX:labelB}"
    And the response body should not contain "${CTX:labelA}"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId}      |
      | name    | ${CTX:apiId}      |
      | version | v1.1              |
      | labels  | ["${CTX:labelB}"] |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:labelB}"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId} |
      | name   | ${CTX:apiId} |
      | labels | []           |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response array field "labels" should have 0 items

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}      |
      | name   | ${CTX:apiId}      |
      | labels | ["${CTX:labelA}"] |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until the response body contains "${CTX:labelA}"
    Then the JSON response field "labels[0]" should be "${CTX:labelA}"

    # The plans an API offers follow every update
    Given I generate a unique resource name from "portal-rest-plans" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                | ${CTX:apiId}     |
      | name              | ${CTX:apiId}     |
      | subscriptionPlans | [{"id": "Gold"}] |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "developer"
    Then the JSON response array field "subscriptionPlans" should have 1 item
    And the JSON response field "subscriptionPlans[0].id" should be "Gold"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response body should contain "aov-plans-wrap"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                | ${CTX:apiId}                       |
      | name              | ${CTX:apiId}                       |
      | subscriptionPlans | [{"id": "Gold"}, {"id": "Silver"}] |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "developer"
    Then the JSON response array field "subscriptionPlans" should have 2 items
    And the JSON response array "subscriptionPlans" should contain an item with "id" equal to "Silver"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                | ${CTX:apiId} |
      | name              | ${CTX:apiId} |
      | subscriptionPlans | []           |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "developer"
    Then the response body should not contain "Silver"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should not contain "aov-plans-wrap"
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON values:
      | artifactId         | ${CTX:apiId} |
      | subscriptionPlanId | Gold         |
    Then the response status code should be 400
    And the response body should contain "This API does not support subscriptions"
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:apiId}" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items

  @APIP-007
  Scenario: Catalogue search matches whole words in an API's metadata and definition
    # Catalogue search matches an API's description, tags and definition by whole word
    Given I generate a unique resource name from "portal-rest-searched" and store it as "apiId"
    And I generate a unique resource name from "portal-rest-searched-tag" and store it as "tag"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest titled Pomegranate ledger ${CTX:apiId}" with values:
      | id          | ${CTX:apiId}                             |
      | name        | ${CTX:apiId}                             |
      | description | Kumquatorchard settlement ${CTX:apiId}   |
      | tags        | ["${CTX:tag}"]                           |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis?query=kumquatorchard" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis?query=${CTX:tag}" as "developer"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis?query=pomegranate" as "developer"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis?query=kumquatorch" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:apiId}"

    # A search that matches nothing is empty, and clearing the search lists the catalogue again
    Given I generate a unique resource name from "portal-rest-unmatched" and store it as "missingTerm"
    And a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/apis?query=${CTX:missingTerm}" as "developer"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    And the JSON response array field "list" should have 0 items
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis?query=${CTX:missingTerm}"
    Then the response status code should be 200
    And the response body should contain "No APIs available yet"
    And the response body should not contain "apilist-results-count"
    When I send an authenticated API Portal "GET" request to "/apis?query=" as "developer"
    Then the response status code should be 400
    And the response body should contain "query"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis?query="
    Then the response status code should be 200
    And the response body should contain "apilist-results-count"
    And the response body should contain "apiCard-${CTX:apiId}"

  @APIP-003 @type:negative
  Scenario: Invalid, duplicate and unknown REST API requests are refused and store nothing
    # Creating a REST API with an invalid OpenAPI definition is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-invalid" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "not an OpenAPI document" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | REST       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201

    # Creating a REST API with no type is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-invalid" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | null       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201

    # Creating a REST API with the MCP type is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-invalid" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | MCP       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201

    # A REST API without a name is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-nameless" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | null         |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201

    # A REST API with an invalid status is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-bad-field" and store it as "apiId"
    And I generate a unique resource name from "portal-missing-plan" and store it as "missingPlanId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId} |
      | name    | ${CTX:apiId} |
      | status | BOGUS      |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

    # A REST API with an invalid agentVisibility is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-bad-field" and store it as "apiId"
    And I generate a unique resource name from "portal-missing-plan" and store it as "missingPlanId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId} |
      | name    | ${CTX:apiId} |
      | agentVisibility | BOGUS      |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

    # A REST API with an invalid subscriptionPlans is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-bad-field" and store it as "apiId"
    And I generate a unique resource name from "portal-missing-plan" and store it as "missingPlanId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId} |
      | name    | ${CTX:apiId} |
      | subscriptionPlans | [{"id": "${CTX:missingPlanId}"}]      |
    Then the response status code should be 404
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

    # A REST API with an invalid labels is refused and stores nothing
    Given I generate a unique resource name from "portal-rest-bad-field" and store it as "apiId"
    And I generate a unique resource name from "portal-missing-plan" and store it as "missingPlanId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:apiId} |
      | name    | ${CTX:apiId} |
      | labels | ["${CTX:missingPlanId}"]      |
    Then the response status code should be 404
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

    # Updating a REST API with no type is refused and the API keeps its type
    Given I generate a unique resource name from "portal-rest-retype" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    And the response status code should be 201
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-changed |
      | type | null               |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "type" should be "RestApi"
    And the JSON response field "name" should be "${CTX:apiId}"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-changed |
      | type | REST                 |
    Then the response status code should be 200

    # Updating a REST API with a different type is refused and the API keeps its type
    Given I generate a unique resource name from "portal-rest-retype" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    And the response status code should be 201
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-changed |
      | type | SOAP               |
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "type" should be "RestApi"
    And the JSON response field "name" should be "${CTX:apiId}"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-changed |
      | type | REST                 |
    Then the response status code should be 200

    # A second API with an existing id is refused
    Given I generate a unique resource name from "portal-rest-duplicate" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId}        |
      | name | ${CTX:apiId}-second |
    Then the response status code should be 409
    And the response body should not contain "constraint"
    And the response body should not contain "SQL"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "name" should be "${CTX:apiId}"

    # A second API with the same name and version is refused
    Given I generate a unique resource name from "portal-rest-namesake" and store it as "apiId"
    And I generate a unique resource name from "portal-rest-namesake-copy" and store it as "copyId"
    And I generate a unique resource name from "portal-rest-namesake-next" and store it as "nextId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:copyId} |
      | name | ${CTX:apiId}  |
    Then the response status code should be 409
    And the response body should not contain "constraint"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:copyId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:apiId}" as "publisher"
    Then the JSON response array field "list" should have 1 item
    And the JSON response field "list[0].id" should be "${CTX:apiId}"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id      | ${CTX:nextId} |
      | name    | ${CTX:apiId}  |
      | version | v2.0          |
    Then the response status code should be 201

    # Requests for an API that does not exist return not found
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-rest-missing" and store it as "missingId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:missingId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:missingId}/assets?type=API_DEFINITION&fileName=definition.json" as "publisher"
    Then the response status code should be 404
    When I update the API Portal API "${CTX:missingId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:missingId} |
      | name | ${CTX:missingId} |
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:missingId}" as "publisher"
    Then the response status code should be 404

    # An API listing with a limit of zero is refused
    When I send an authenticated API Portal "GET" request to "/apis?limit=0" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis?limit=100&offset=0" as "publisher"
    Then the response status code should be 200

    # An API listing with a limit over 100 is refused
    When I send an authenticated API Portal "GET" request to "/apis?limit=101" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis?limit=100&offset=0" as "publisher"
    Then the response status code should be 200

    # An API listing with a non-numeric limit is refused
    When I send an authenticated API Portal "GET" request to "/apis?limit=abc" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis?limit=100&offset=0" as "publisher"
    Then the response status code should be 200

    # An API listing with a negative offset is refused
    When I send an authenticated API Portal "GET" request to "/apis?offset=-1" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis?limit=100&offset=0" as "publisher"
    Then the response status code should be 200

    # An API listing with an unknown parameter is refused
    When I send an authenticated API Portal "GET" request to "/apis?colour=red" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis?limit=100&offset=0" as "publisher"
    Then the response status code should be 200

  @APIP-003 @type:negative @type:security
  Scenario: A REST API in use cannot be deleted, and only a publisher can change one
    # An API with a subscription cannot be deleted until the subscription ends
    Given a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200

    # An API with an active key cannot be deleted until the key is revoked
    Given a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200

    # Anonymous callers cannot list or create REST APIs
    When I send an unauthenticated API Portal "GET" request to "/apis"
    Then the response status code should be 401
    When I send an unauthenticated API Portal "POST" request to "/apis"
    Then the response status code should be 401
    When I send an authenticated API Portal "GET" request to "/apis" as "publisher"
    Then the response status code should be 200

    # A developer can read REST APIs but cannot create, update or delete them
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-rest-denied" and store it as "deniedId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "developer"
    Then the response status code should be 200
    And I store the API Portal response body as "originalApi"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "developer" with values:
      | id   | ${CTX:deniedId} |
      | name | ${CTX:deniedId} |
    Then the response status code should be 403
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "developer" with values:
      | id   | ${CTX:apiId}    |
      | name | ${CTX:deniedId} |
    Then the response status code should be 403
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "developer"
    Then the response status code should be 200
    And the API Portal response body should equal stored "originalApi"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:deniedId}" as "publisher"
    Then the response status code should be 404
