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

@SVR @vhost-routing-single
Feature: Single-domain vhost routing
  As an API user
  I want gateway routing behavior to follow the configured rules
  So that requests reach the intended endpoint and response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  @SVR-006
  Scenario: Route requests using single-domain gateway defaults
    Given I generate a unique value from "vhost-routing-single-1" and store it as "apiName1"
    And I generate a unique API context from "/vhost-routing-single-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | ${CTX:gatewaySpecVersion} |
      | name                       | ${CTX:apiName1}                  |
      | spec.displayName            | VHost-Single-Defaults             |
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
    And I set request host to "api-sandbox.wso2.com"
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
    And I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami" until status 404
    Then the response status code should be 404


  @SVR-009
  Scenario: API vhost override should take precedence over gateway defaults
    Given I generate a unique value from "vhost-routing-single-2" and store it as "apiName2"
    And I generate a unique API context from "/vhost-routing-single-2" and store it as "apiContext2"
    And I generate a unique resource name from "mainHost2-vhost-routing-single-2" and store it as "mainHost2"
    And I generate a unique resource name from "sandboxHost2-vhost-routing-single-2" and store it as "sandboxHost2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | ${CTX:gatewaySpecVersion} |
      | name                       | ${CTX:apiName2}                  |
      | spec.displayName            | VHost-Single-Override             |
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
    And I set request host to "api-sandbox.wso2.com"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response status code should be 404
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName2}"
    Then the response should be successful
    And I clear all headers
    And I set request host to "${CTX:mainHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami" until status 404
    Then the response status code should be 404


  @SVR-011
  Scenario: Sentinel vhost resolves to gateway defaults at deploy time
    Given I generate a unique value from "vhost-routing-single-3" and store it as "apiName3"
    And I generate a unique API context from "/vhost-routing-single-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | ${CTX:gatewaySpecVersion} |
      | name                       | ${CTX:apiName3}                  |
      | spec.displayName            | VHost-Single-Sentinel              |
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
    And I set request host to "api-sandbox.wso2.com"
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
    And I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami" until status 404
    Then the response status code should be 404

  @SVR-014
  Scenario: An API created on the gateway without vhosts stores the gateway's default vhosts
    Given I generate a unique value from "vhost-routing-single-14" and store it as "apiName14"
    And I generate a unique API context from "/vhost-routing-single-14" and store it as "apiContext14"
    # No sandbox upstream: the sandbox default is stored all the same.
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName14}                    |
      | spec.displayName       | VHost-Single-Stored-Defaults        |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext14}/$version        |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName14}"
    Then the response status code should be 200
    And the JSON response field "spec.vhosts.main" should be "*.wso2.com"
    And the JSON response field "spec.vhosts.sandbox" should be "*-sandbox.wso2.com"
    When I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext14}/v1.0/whoami" until status 200
    Then the JSON response field "path" should be "/whoami"

  # An update stores _gateway_default_ literally and stores no vhosts when they are left out:
  # https://github.com/wso2/api-platform/issues/4167
  @SVR-029 @known-issue
  Scenario: Updating an API on the gateway with the default placeholder or without vhosts keeps the gateway's default vhosts
    Given I generate a unique value from "vhost-routing-single-29" and store it as "apiName29"
    And I generate a unique API context from "/vhost-routing-single-29" and store it as "apiContext29"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName29}                    |
      | spec.displayName       | VHost-Single-Updated-Defaults       |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext29}/$version        |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName29}"
    Then the JSON response field "spec.vhosts.main" should be "*.wso2.com"

    When I update API "${CTX:apiName29}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName29}                    |
      | spec.displayName       | VHost-Single-Updated-Defaults       |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext29}/$version        |
      | spec.vhosts            | {"main":"_gateway_default_"}        |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName29}"
    Then the JSON response field "spec.vhosts.main" should be "*.wso2.com"
    And the JSON response field "spec.vhosts.sandbox" should be "*-sandbox.wso2.com"
    When I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext29}/v1.0/whoami" until status 200

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I update API "${CTX:apiName29}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName29}                    |
      | spec.displayName       | VHost-Single-Updated-Defaults       |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext29}/$version        |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName29}"
    Then the JSON response field "spec.vhosts.main" should be "*.wso2.com"
    And the JSON response field "spec.vhosts.sandbox" should be "*-sandbox.wso2.com"
    When I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext29}/v1.0/whoami" until status 200
