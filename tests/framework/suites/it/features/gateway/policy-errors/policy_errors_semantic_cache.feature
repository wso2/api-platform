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

# Each scenario drives one policy into its main rejection and holds the whole response — status,
# contract headers and body — to the expected error response recorded for the gateway version
# under test. The expected responses were recorded from the policies as they were before the
# fault contract, and the policies under test come from the adjacent gateway-controllers
# checkout, so a policy change a client could observe fails here. See the "Policy error
# compatibility" section of tests/framework/README.md.
#
# semantic-cache needs Redis and the embedding mock, so it runs in its own block.
@policy-compat
Feature: semantic-cache error responses
  As a gateway operator
  I want a policy release to leave every error response a client receives unchanged
  So that upgrading policies on an existing gateway cannot break its callers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: semantic-cache rejects a request whose JSONPath does not resolve
    Given I generate a unique value from "compat-sc-jsonpath" and store it as "apiName"
    And I generate a unique API version from "compat-sc-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/compat-sc-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"jsonPath":"$.nonexistent.field"}}]},{"method":"GET","path":"/health"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"message":"This field exists but not the expected path"}
      """
    Then the response status code should be 400
    And the response should match the expected error response "semantic-cache-jsonpath-unresolved"
