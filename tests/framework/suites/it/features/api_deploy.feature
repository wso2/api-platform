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

@api-deploy
Feature: API deployment and invocation
  As an API developer
  I want to deploy an API configuration and invoke it
  So that I can verify the gateway routes requests correctly

  Background:
    Given the gateway services are running

  Scenario: Deploying a simple HTTP API and invoking it succeeds
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "weather-api" and store it as "apiName"
    And I generate a unique value from "weather-display" and store it as "apiDisplayName"
    And I generate a unique API version from "weather" and store it as "apiVersion"
    And I generate a unique API context from "/weather" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | ${CTX:apiDisplayName}              |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"POST","path":"/alerts/active"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "/api/v2/us/seattle"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Deploying an HTTP API preserves its labels
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "labeled-api" and store it as "apiName"
    And I generate a unique API context from "/labeled" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Labeled-API                        |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}/$version         |
      | metadata.labels        | {"environment":"production","team":"backend","version":"v1"} |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful

    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    When I get the API "${CTX:apiName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "metadata.labels.environment" should be "production"
    And the JSON response field "metadata.labels.team" should be "backend"
    And the JSON response field "metadata.labels.version" should be "v1"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Deploying an API resolves a version placeholder in its context
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "versioned-context-api" and store it as "apiName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Versioned Context API              |
      | spec.version           | v2.0                               |
      | spec.context           | /api/$version                      |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "/api/v2.0/data" until status 200
    When I send a "GET" request to "/api/v2.0/data"
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "/api/v2.0"
    And the response body should not contain "/api/$version"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Deploying an API with invalid label keys returns a validation error
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "invalid-labels-api" and store it as "apiName"
    And I generate a unique API context from "/invalid-labels" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Invalid-Labels-API                 |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}/$version         |
      | metadata.labels        | {"My Label":"value","team":"backend"} |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"

  Scenario Outline: Deploying an API with an invalid upstream URL returns its validation error
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "invalid-upstream-api" and store it as "apiName"
    And I generate a unique API context from "/invalid-upstream" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1 |
      | name                          | ${CTX:apiName}                     |
      | spec.displayName              | Invalid-Upstream-API              |
      | spec.version                  | v1.0                               |
      | spec.context                  | ${CTX:apiContext}/$version         |
      | spec.upstreamDefinitions      | [{"name":"backend-default","basePath":"/api-main","upstreams":[{"url":"<upstreamUrl>"}]}] |
      | spec.upstream.main.ref        | backend-default                    |
      | spec.operations               | [{"method":"GET","path":"/endpoint"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "<message>"

    Examples:
      | upstreamUrl                 | message                         |
      | http://testbench:3000?region=eu | must not include a query string |
      | http://testbench:3000?           | must not include a query string |
      | http://testbench:3000#section    | must not include a fragment      |

  Scenario: Deploying an API with query and fragment in its upstream URL reports both errors
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "invalid-upstream-query-fragment-api" and store it as "apiName"
    And I generate a unique API context from "/invalid-upstream-query-fragment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1 |
      | name                          | ${CTX:apiName}                     |
      | spec.displayName              | Invalid-Upstream-Query-Fragment-API |
      | spec.version                  | v1.0                               |
      | spec.context                  | ${CTX:apiContext}/$version         |
      | spec.upstreamDefinitions      | [{"name":"backend-default","basePath":"/api-main","upstreams":[{"url":"http://testbench:3000?a=1#top"}]}] |
      | spec.upstream.main.ref        | backend-default                    |
      | spec.operations               | [{"method":"GET","path":"/endpoint"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "must not include a query string"
    And the response body should contain "must not include a fragment"
