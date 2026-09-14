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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@path-normalization
Feature: Path normalization
  As an API developer
  I want gateway route matching to handle path variants correctly
  So that valid requests are routed and invalid variants are rejected

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Default normalization resolves dot-segments and merges duplicate slashes
    Given I generate a unique value from "path-normalization-1" and store it as "apiName1"
    And I generate a unique API context from "/path-normalization-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:apiName1}                  |
      | spec.displayName      | Path-Norm-API                     |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext1}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/weather"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/other/../weather" until status 200

    When I send a "GET" request to "${CTX:apiContext1}/v1.0/other/../weather"
    Then the response status code should be 200
    And the JSON response field "path" should be "/weather"

    When I send a "GET" request to "${CTX:apiContext1}/v1.0//weather"
    Then the response status code should be 200
    And the JSON response field "path" should be "/weather"

    When I delete the API "${CTX:apiName1}"
    Then the response should be successful


  Scenario: Escaped slash (%2F) in a resource path segment is kept unchanged and is not treated as a path separator
    Given I generate a unique value from "path-normalization-2" and store it as "apiName2"
    And I generate a unique API context from "/path-normalization-2" and store it as "apiContext2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:apiName2}                  |
      | spec.displayName      | Path-Escaped-Slash-API            |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext2}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/pets/{name}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/pets/pet%2FFarm" until status 200

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/pets/pet%2FFarm"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/pets/pet/Farm"
    Then the response status code should be 404

    When I delete the API "${CTX:apiName2}"
    Then the response should be successful


  Scenario: A path segment containing a literal dot is preserved by normalization
    Given I generate a unique value from "path-normalization-3" and store it as "apiName3"
    And I generate a unique API context from "/path-normalization-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:apiName3}                  |
      | spec.displayName      | Path-Literal-Dot-API              |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext3}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/pet.api"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/pet.api" until status 200

    When I send a "GET" request to "${CTX:apiContext3}/v1.0/pet.api"
    Then the response status code should be 200
    And the JSON response field "path" should be "/pet.api"

    When I send a "GET" request to "${CTX:apiContext3}/v1.0/./pet.api"
    Then the response status code should be 200
    And the JSON response field "path" should be "/pet.api"

    When I delete the API "${CTX:apiName3}"
    Then the response should be successful
