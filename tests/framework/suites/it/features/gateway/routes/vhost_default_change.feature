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

@SVR @vhost-default-change
Feature: An API keeps the default vhost it was deployed with until it is redeployed
  As a gateway operator
  I want an API's default vhosts fixed when it is deployed
  So that changing the gateway's defaults does not move APIs that are already deployed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @SVR-013
  Scenario: An API deployed without vhosts answers on the old default after the default changes, and on the new one once redeployed
    Given I generate a unique value from "vhost-default-change" and store it as "apiName"
    And I generate a unique API context from "/vhost-default-change" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | VHost-Default-Change                |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/whoami" until status 200
    When I clear all headers
    And I set request host to "api.example.org"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I change "router.vhosts.main.default" from "*.wso2.com" to "*.example.org" in the gateway's config.toml
    And I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    # Not redeployed: the API keeps the default it was deployed with.
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}"
    Then the response status code should be 200
    And the JSON response field "spec.vhosts.main" should be "*.wso2.com"
    When I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/whoami" until status 200
    And I clear all headers
    And I set request host to "api.example.org"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/whoami"
    Then the response status code should be 404

    # Redeployed (deleted and created again): the API takes the new default.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | VHost-Default-Change                |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/whoami"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}"
    Then the JSON response field "spec.vhosts.main" should be "*.example.org"
    When I set request host to "api.example.org"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/whoami" until status 200
    Then the JSON response field "path" should be "/whoami"
    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/whoami"
    Then the response status code should be 404
