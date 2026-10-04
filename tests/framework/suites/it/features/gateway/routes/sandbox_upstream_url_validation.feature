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

@sandbox-upstream-url-validation
Feature: Sandbox upstream URL validation
  As an API developer
  I want invalid upstream URL paths to be rejected
  So that sandbox upstream routing remains unambiguous

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy fails when an upstreamDefinitions URL contains a path
    Given I generate a unique value from "sandbox-routing-11" and store it as "apiName11"
    And I generate a unique API context from "/sandbox-routing-11" and store it as "apiContext11"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name | ${CTX:apiName11} |
      | spec | {"displayName":"Env-Routing-Sandbox-Multi-URL-Ref-API","version":"v1.0","context":"${CTX:apiContext11}/$version","upstreamDefinitions":[{"name":"sandbox-multi-upstream","upstreams":[{"url":"http://testbench:3000/first"},{"url":"http://testbench:3000/sandbox"}]}],"upstream":{"main":{"ref":"sandbox-multi-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "must not include a path"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName11}"
    Then the response status code should be 404
