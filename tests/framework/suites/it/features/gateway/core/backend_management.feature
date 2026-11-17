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

@backend-management @GWBE
Feature: Global backend management
  As a platform administrator managing gateway-wide Backends through the management API
  I want to create, read, update and delete Backends, and have the gateway refuse unsafe changes
  So that APIs reach shared backends by name without being edited or redeployed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @GWBE-001
  Scenario: A Backend is created, read, listed, updated and deleted
    Given I generate a unique resource name from "ep-crud" and store it as "backendName"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                                       |
      | name                 | ${CTX:backendName}                                             |
      | metadata.labels      | {"team":"orders"}                                               |
      | spec.displayName     | Orders backend                                                  |
      | spec.basePath        | /v1                                                             |
      | spec.timeout         | {"connect":"2s"}                                                |
      | spec.endpoints       | [{"url":"http://testbench:3000","weight":80},{"url":"http://testbench:3000","weight":20}] |
    Then the response status code should be 201
    And the JSON response field "status.id" should be "${CTX:backendName}"
    And the JSON response field "spec.basePath" should be "/v1"

    When I get the Backend "${CTX:backendName}"
    Then the response status code should be 200
    And the JSON response field "metadata.labels.team" should be "orders"
    And the JSON response field "spec.timeout.connect" should be "2s"
    And the JSON response array field "spec.endpoints" should have 2 items
    And the JSON response field "spec.endpoints[0].weight" should be 80
    And the JSON response field "spec.endpoints[1].weight" should be 20

    When I send a "GET" request to the "gateway-controller" service at "/backends"
    Then the response status should be 200
    And the JSON response array "backends" should contain an item with "metadata.name" equal to "${CTX:backendName}"

    When I update Backend "${CTX:backendName}" from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                                       |
      | name                 | ${CTX:backendName}                                             |
      | metadata.labels      | {"team":"billing"}                                              |
      | spec.basePath        | /v2                                                             |
      | spec.endpoints       | [{"url":"http://testbench:3000","weight":50}]                   |
    Then the response status code should be 200
    And the JSON response field "spec.basePath" should be "/v2"
    And the JSON response field "metadata.labels.team" should be "billing"

    # The update replaced the endpoints rather than merging them.
    When I get the Backend "${CTX:backendName}"
    Then the response status code should be 200
    And the JSON response array field "spec.endpoints" should have 1 item
    And the JSON response field "spec.endpoints[0].url" should be "http://testbench:3000"
    And the JSON response field "spec.endpoints[0].weight" should be 50

    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 200
    When I get the Backend "${CTX:backendName}"
    Then the response status code should be 404

  @GWBE-002 @type:negative
  Scenario: An invalid Backend is rejected with the field at fault
    Given I generate a unique resource name from "ep-invalid" and store it as "backendName"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:gatewaySpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 400
    And the response body should contain "gateway.choreolabs.com/v1"

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | Not_A_Valid_Name                          |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 400
    And the response body should contain "metadata.name"

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"https://testbench:3443"},{"url":"http://testbench:3000"}] |
    Then the response status code should be 400
    And the response body should contain "spec.endpoints"

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.basePath        | /v1/                                      |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 400
    And the response body should contain "spec.basePath"

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
    Then the response status code should be 400
    And the response body should contain "spec.endpoints"

    When I get the Backend "${CTX:backendName}"
    Then the response status code should be 404

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.basePath        | /v1                                       |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201

  @GWBE-003 @type:negative
  Scenario: A duplicate name is refused, and a Backend cannot be renamed
    Given I generate a unique resource name from "ep-dup" and store it as "backendName"
    And I generate a unique resource name from "ep-other" and store it as "otherName"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 409

    When I update Backend "${CTX:backendName}" from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:otherName}                          |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 400
    And the response body should contain "cannot be renamed"

    When I update Backend "${CTX:backendName}" from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.basePath        | /v2                                       |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 200
    And the JSON response field "spec.basePath" should be "/v2"

    When I update Backend "${CTX:otherName}" from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:otherName}                          |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 404

  @GWBE-004 @type:negative
  Scenario: Developers can read Backends, and only administrators can change them
    Given I generate a unique resource name from "ep-roles" and store it as "backendName"
    And I generate a unique resource name from "ep-roles-dev" and store it as "developerBackend"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201

    When I authenticate using basic auth as "developer"
    And I get the Backend "${CTX:backendName}"
    Then the response status code should be 200

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:developerBackend}                  |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 403

    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 403

    When I authenticate using basic auth as "admin"
    And I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:developerBackend}                  |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201

    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 200

  @GWBE-005 @type:negative
  Scenario: A Backend a deployed API names as its fixed target cannot be deleted until the API is gone
    Given I generate a unique resource name from "ep-pinned" and store it as "backendName"
    And I generate a unique value from "backend-pinned-api" and store it as "apiName"
    And I generate a unique API context from "/backend-pinned" and store it as "apiContext"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.basePath        | /pinned                                   |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/items","policies":[{"name":"dynamic-endpoint","version":"v1","params":{"targetUpstream":"${CTX:backendName}"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items" until status 200
    And the JSON response field "path" should be "/pinned/items"

    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 409
    And the response body should contain "${CTX:apiName}"
    And the response body should contain "spec.operations[0].policies[0].params.targetUpstream"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 200

  @GWBE-006
  Scenario: An API's own upstream definition takes precedence over a Backend of the same name
    Given I generate a unique resource name from "ep-shadow" and store it as "backendName"
    And I generate a unique value from "backend-shadow-api" and store it as "apiName"
    And I generate a unique API context from "/backend-shadow" and store it as "apiContext"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.basePath        | /shared                                   |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}          |
      | name                     | ${CTX:apiName}                      |
      | spec.displayName         | ${CTX:apiName}                      |
      | spec.version             | v1.0                                |
      | spec.context             | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url   | http://testbench:3000               |
      | spec.upstreamDefinitions | [{"name":"${CTX:backendName}","basePath":"/own","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.operations          | [{"method":"GET","path":"/items","policies":[{"name":"dynamic-endpoint","version":"v1","params":{"targetUpstream":"${CTX:backendName}"}}]},{"method":"GET","path":"/control","policies":[{"name":"dynamic-endpoint","version":"v1","params":{"backendExpression":"'${CTX:backendName}'","backendSelector":{}}}]}] |
    Then the response should be successful
    # The control operation names the Backend through backendExpression, which only names
    # Backends, so it reaches the Backend once the Backend is routable.
    And I send a "GET" request to "${CTX:apiContext}/v1.0/control" until status 200
    And the JSON response field "path" should be "/shared/control"
    When I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 200
    And the JSON response field "path" should be "/own/items"

    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 200
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 200
    And the JSON response field "path" should be "/own/items"

  @GWBE-007
  Scenario: An API routes by backendExpression to a Backend added after it was deployed
    Given I generate a unique resource name from "ep-team" and store it as "team"
    And I generate a unique resource name from "ep-late" and store it as "backendName"
    And I generate a unique resource name from "ep-outsider" and store it as "outsiderName"
    And I generate a unique value from "backend-header-api" and store it as "apiName"
    And I generate a unique API context from "/backend-header" and store it as "apiContext"
    And I generate a unique value from "backend-probe-api" and store it as "probeName"
    And I generate a unique API context from "/backend-probe" and store it as "probeContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"dynamic-endpoint","version":"v1","params":{"backendExpression":"request.Headers['x-backend'][0]","backendSelector":{"matchLabels":{"team":"${CTX:team}"}}}},{"name":"remove-headers","version":"v1","params":{"request":{"headers":[{"name":"x-backend"}]}}}] |
      | spec.operations        | [{"method":"GET","path":"/items"}]  |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items" until status 200
    And the JSON response field "path" should be "/items"

    When I set header "x-backend" to "${CTX:backendName}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 400

    # A Backend outside the selector.
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:outsiderName}                      |
      | metadata.labels      | {"team":"outsider-${CTX:team}"}           |
      | spec.basePath        | /outsider                                 |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | metadata.labels      | {"team":"${CTX:team}"}                    |
      | spec.basePath        | /late                                     |
      | spec.endpoints       | [{"url":"http://testbench:3000"}]         |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items" until status 200
    And the JSON response field "path" should be "/late/items"
    And the response should not contain echoed header "x-backend"

    # A probe API whose selector allows every Backend shows when the outsider is routable, so the
    # 400 below comes from the selector rather than from a Backend the policy can't see yet.
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:probeName}                    |
      | spec.displayName       | ${CTX:probeName}                    |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:probeContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"dynamic-endpoint","version":"v1","params":{"backendExpression":"request.Headers['x-backend'][0]","backendSelector":{}}}] |
      | spec.operations        | [{"method":"GET","path":"/items"}]  |
    Then the response should be successful
    When I set header "x-backend" to "${CTX:outsiderName}"
    And I send a "GET" request to "${CTX:probeContext}/v1.0/items" until status 200
    Then the JSON response field "path" should be "/outsider/items"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 400

  @GWBE-008
  Scenario: A templated Backend reaches the host each request's key expands to
    Given I generate a unique resource name from "ep-env" and store it as "backendName"
    And I generate a unique value from "backend-templated-api" and store it as "apiName"
    And I generate a unique API context from "/backend-templated" and store it as "apiContext"

    # A template that breaks a rule is refused with the field and the rule.
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"http://{key}:3000"}]             |
    Then the response status code should be 400
    And the response body should contain "spec.endpoints[0].url"
    And the response body should contain "first label"
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.endpoints       | [{"url":"http://env-{key}.testbench:3000"},{"url":"http://testbench:3000"}] |
    Then the response status code should be 400
    And the response body should contain "only endpoint"

    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                 |
      | name                 | ${CTX:backendName}                       |
      | spec.basePath        | /tmpl                                     |
      | spec.endpoints       | [{"url":"http://env-{key}.testbench:3000"}] |
    Then the response status code should be 201
    And the JSON response field "spec.endpoints[0].url" should be "http://env-{key}.testbench:3000"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"dynamic-endpoint","version":"v1","params":{"targetUpstream":"${CTX:backendName}","backendKeyExpression":"request.Headers['env'][0].lowerAscii()"}}] |
      | spec.operations        | [{"method":"GET","path":"/items"}]  |
    Then the response should be successful

    # Each key reaches its own host, with the Backend's base path, and the host header the
    # gateway uses to choose the host never reaches the backend.
    When I set header "env" to "DEV"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items" until status 200
    Then the JSON response field "path" should be "/tmpl/items"
    And the response should contain echoed header "Host" containing "env-dev.testbench"
    And the response should not contain echoed header "x-target-upstream-host"
    When I set header "env" to "prod"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 200
    And the response should contain echoed header "Host" containing "env-prod.testbench"

    # A host the client names itself is ignored: the key alone chooses it.
    When I set header "x-target-upstream-host" to "env-prod.testbench:3000"
    And I set header "env" to "dev"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 200
    And the response should contain echoed header "Host" containing "env-dev.testbench"

    # A key that names no host fails at DNS; a value that isn't one DNS label, or no key, is
    # refused before routing.
    When I clear all headers
    And I set header "env" to "qa"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 503
    When I set header "env" to "dev.evil.example"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 400
    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items"
    Then the response status code should be 400

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 200
