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

@vhost-routing-multiple-production-hosts
Feature: Multiple production vhosts in API routing
  As an API user
  I want multiple production vhosts configured on an API to route to the main upstream
  So that every configured production hostname serves the API

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Semicolon-separated main vhosts route listed production hosts to the main upstream
    Given I generate a unique value from "vhost-routing-multi-4" and store it as "apiName4"
    And I generate a unique API context from "/vhost-routing-multi-4" and store it as "apiContext4"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | ${CTX:gatewaySpecVersion} |
      | name                       | ${CTX:apiName4} |
      | spec.displayName            | VHost-Multi-List |
      | spec.version                | v1.0 |
      | spec.context                | ${CTX:apiContext4}/$version |
      | spec.vhosts                 | {"main":"alpha.example.com;beta.example.com;*.wild.example.com","sandbox":"sandbox.example.com"} |
      | spec.upstream.main.url      | http://testbench:3000 |
      | spec.upstream.sandbox.url   | http://testbench:3000/sandbox |
      | spec.operations             | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    And I set request host to "alpha.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "alpha.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "beta.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "node1.wild.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "wild.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName4}"
    Then the response should be successful
    And I clear all headers
    And I set request host to "alpha.example.com"
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/whoami" until status 404
    Then the response status code should be 404
