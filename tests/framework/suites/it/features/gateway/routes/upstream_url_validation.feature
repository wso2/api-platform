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

@upstream-url-validation
Feature: Upstream URL validation
  As an API developer
  I want invalid upstream URL components to be rejected
  So that upstream routing configuration remains unambiguous

  Background:
    Given the gateway services are running

  Scenario Outline: Deploying an API with an invalid upstream URL returns its validation error
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "invalid-upstream-api" and store it as "apiName"
    And I generate a unique API context from "/invalid-upstream" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}         |
      | name                     | ${CTX:apiName}                    |
      | spec.displayName         | Invalid-Upstream-API              |
      | spec.version             | v1.0                              |
      | spec.context             | ${CTX:apiContext}/$version        |
      | spec.upstreamDefinitions | [{"name":"backend-default","basePath":"/api-main","upstreams":[{"url":"<upstreamUrl>"}]}] |
      | spec.upstream.main.ref   | backend-default                   |
      | spec.operations          | [{"method":"GET","path":"/endpoint"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "<message>"

    Examples:
      | upstreamUrl                    | message                          |
      | http://testbench:3000?region=eu | must not include a query string |
      | http://testbench:3000?          | must not include a query string |
      | http://testbench:3000#section   | must not include a fragment     |

  Scenario: Deploying an API with query and fragment in its upstream URL reports both errors
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "invalid-upstream-query-fragment-api" and store it as "apiName"
    And I generate a unique API context from "/invalid-upstream-query-fragment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                 |
      | name                     | ${CTX:apiName}                            |
      | spec.displayName         | Invalid-Upstream-Query-Fragment-API      |
      | spec.version             | v1.0                                      |
      | spec.context             | ${CTX:apiContext}/$version                |
      | spec.upstreamDefinitions | [{"name":"backend-default","basePath":"/api-main","upstreams":[{"url":"http://testbench:3000?a=1#top"}]}] |
      | spec.upstream.main.ref   | backend-default                           |
      | spec.operations          | [{"method":"GET","path":"/endpoint"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "must not include a query string"
    And the response body should contain "must not include a fragment"
