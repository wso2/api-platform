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

@vhost-routing-multi
Feature: Multi-domain vhost routing
  As an API user
  I want gateway routing behavior to follow the configured rules
  So that requests reach the intended endpoint and response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Route requests across all configured main and sandbox domains
    Given I generate a unique value from "vhost-routing-multi-1" and store it as "apiName1"
    And I generate a unique API context from "/vhost-routing-multi-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName1}                  |
      | spec.displayName            | VHost-Multi-Domains               |
      | spec.version                | v1.0                              |
      | spec.context                | ${CTX:apiContext1}/$version       |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.upstream.sandbox.url   | http://testbench:3000/sandbox     |
      | spec.operations             | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api.foo.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.foo.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.other.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response status code should be 404
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName1}"
    Then the response should be successful


  Scenario: API vhost override should bypass multi-domain gateway defaults
    Given I generate a unique value from "vhost-routing-multi-2" and store it as "apiName2"
    And I generate a unique API context from "/vhost-routing-multi-2" and store it as "apiContext2"
    And I generate a unique resource name from "mainHost2-vhost-routing-multi-2" and store it as "mainHost2"
    And I generate a unique resource name from "sandboxHost2-vhost-routing-multi-2" and store it as "sandboxHost2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName2}                  |
      | spec.displayName            | VHost-Multi-Override              |
      | spec.version                | v1.0                              |
      | spec.context                | ${CTX:apiContext2}/$version       |
      | spec.vhosts                 | {"main":"${CTX:mainHost2}","sandbox":"${CTX:sandboxHost2}"} |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.upstream.sandbox.url   | http://testbench:3000/sandbox     |
      | spec.operations             | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    And I set request host to "${CTX:mainHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "api.foo.com"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "api-sandbox.foo.com"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response status code should be 404
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName2}"
    Then the response should be successful


  Scenario: Sentinel vhost resolves to all configured gateway domains
    Given I generate a unique value from "vhost-routing-multi-3" and store it as "apiName3"
    And I generate a unique API context from "/vhost-routing-multi-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName3}                  |
      | spec.displayName            | VHost-Multi-Sentinel               |
      | spec.version                | v1.0                              |
      | spec.context                | ${CTX:apiContext3}/$version       |
      | spec.vhosts                 | {"main":"_gateway_default_","sandbox":"_gateway_default_"} |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.upstream.sandbox.url   | http://testbench:3000/sandbox     |
      | spec.operations             | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api.foo.com"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.foo.com"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    # The sentinel string itself must NOT be used as a hostname — proves resolution occurred
    When I clear all headers
    And I set request host to "_gateway_default_"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response status code should be 404
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName3}"
    Then the response should be successful


  Scenario: Semicolon-separated vhosts.main routes every listed production host to the main upstream
    Given I generate a unique value from "vhost-routing-multi-4" and store it as "apiName4"
    And I generate a unique API context from "/vhost-routing-multi-4" and store it as "apiContext4"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName4}                  |
      | spec.displayName            | VHost-Multi-List                  |
      | spec.version                | v1.0                              |
      | spec.context                | ${CTX:apiContext4}/$version       |
      | spec.vhosts                 | {"main":"alpha.example.com;beta.example.com;*.wild.example.com","sandbox":"sandbox.example.com"} |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.upstream.sandbox.url   | http://testbench:3000/sandbox     |
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
