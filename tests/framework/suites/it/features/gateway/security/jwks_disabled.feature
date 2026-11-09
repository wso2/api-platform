# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
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

# The block runs the gateway with [signing] jwks_endpoint_enabled = false and one runner, so the
# API below may use the fixed /.well-known context without colliding with another runner.
@gateway-jwks-disabled
Feature: Gateway JWKS endpoint disabled
  As an operator whose API already serves /.well-known/jwks
  I want to turn the gateway JWKS endpoint off
  So that my API keeps answering on that path

  Background:
    Given the gateway services are running

  Scenario: The gateway serves no JWK Set when the endpoint is disabled
    When I send a "GET" request to "/.well-known/jwks"
    Then the response status code should be 404

  Scenario: An API can serve the JWKS path when the endpoint is disabled
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "jwks-owned-api" and store it as "apiName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}             |
      | name                   | ${CTX:apiName}                        |
      | spec.displayName       | JWKS-Owned-API                        |
      | spec.version           | v1.0                                  |
      | spec.context           | /.well-known                          |
      | spec.upstream.main.url | http://testbench:3000/api/v2          |
      | spec.operations        | [{"method":"GET","path":"/jwks"}]     |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    When I clear all headers
    And I send a "GET" request to "/.well-known/jwks" until status 200
    Then the response header "Content-Type" should not contain "jwk-set"
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
