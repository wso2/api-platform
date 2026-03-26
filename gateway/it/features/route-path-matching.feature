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

Feature: Route Path Matching
  As an API developer
  I want paths "/" and "/*" to match correctly in Envoy
  So that requests with and without trailing slashes are routed as expected

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Wildcard path /* matches requests with a subpath
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-wildcard-api
      spec:
        displayName: Route-Wildcard-API
        version: v1.0
        context: /route-wildcard/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /*
      """
    Then the response should be successful
    And I wait for 2 seconds

    When I send a GET request to "http://localhost:8080/route-wildcard/v1.0/us/seattle"
    Then the response should be successful

    When I send a GET request to "http://localhost:8080/route-wildcard/v1.0/data"
    Then the response should be successful

    When I delete the API "route-wildcard-api"
    Then the response should be successful

  Scenario: Wildcard path /* enforces HTTP method
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-wildcard-method-api
      spec:
        displayName: Route-Wildcard-Method-API
        version: v1.0
        context: /route-wildcard-method/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /*
      """
    Then the response should be successful
    And I wait for 2 seconds

    When I send a GET request to "http://localhost:8080/route-wildcard-method/v1.0/data"
    Then the response should be successful

    When I send a POST request to "http://localhost:8080/route-wildcard-method/v1.0/data"
    Then the response status code should be 404

    When I delete the API "route-wildcard-method-api"
    Then the response should be successful

  Scenario: Root path / matches request with trailing slash
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-root-api
      spec:
        displayName: Route-Root-API
        version: v1.0
        context: /route-root/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /
      """
    Then the response should be successful
    And I wait for 2 seconds

    When I send a GET request to "http://localhost:8080/route-root/v1.0/"
    Then the response should be successful

    When I delete the API "route-root-api"
    Then the response should be successful

  Scenario: Root path / matches request without trailing slash
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-root-noslash-api
      spec:
        displayName: Route-Root-NoSlash-API
        version: v1.0
        context: /route-root-noslash/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /
      """
    Then the response should be successful
    And I wait for 2 seconds

    When I send a GET request to "http://localhost:8080/route-root-noslash/v1.0"
    Then the response should be successful

    When I delete the API "route-root-noslash-api"
    Then the response should be successful

  Scenario: Wildcard /* does not match a sibling context prefix
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-wildcard-boundary-api
      spec:
        displayName: Route-Wildcard-Boundary-API
        version: v1.0
        context: /route-wc-boundary/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /*
      """
    Then the response should be successful
    And I wait for 2 seconds

    # Exact prefix and sub-paths must match
    When I send a GET request to "http://localhost:8080/route-wc-boundary/v1.0/data"
    Then the response should be successful

    # Bare prefix (no trailing slash) must also match
    When I send a GET request to "http://localhost:8080/route-wc-boundary/v1.0"
    Then the response should be successful

    # Sibling context that shares the prefix must NOT match
    When I send a GET request to "http://localhost:8080/route-wc-boundary/v1.0beta/data"
    Then the response status code should be 404

    When I delete the API "route-wildcard-boundary-api"
    Then the response should be successful

  Scenario: Exact path matches both with and without trailing slash
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-exact-slash-api
      spec:
        displayName: Route-Exact-Slash-API
        version: v1.0
        context: /route-exact-slash/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /weather
      """
    Then the response should be successful
    And I wait for 2 seconds

    When I send a GET request to "http://localhost:8080/route-exact-slash/v1.0/weather"
    Then the response should be successful

    When I send a GET request to "http://localhost:8080/route-exact-slash/v1.0/weather/"
    Then the response should be successful

    When I delete the API "route-exact-slash-api"
    Then the response should be successful

  Scenario: Exact path preserves trailing slash to upstream
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: route-exact-upstream-slash-api
      spec:
        displayName: Route-Exact-Upstream-Slash-API
        version: v1.0
        context: /route-exact-upstream/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /weather
      """
    Then the response should be successful
    And I wait for 2 seconds

    # Without trailing slash — upstream must not receive one
    When I send a GET request to "http://localhost:8080/route-exact-upstream/v1.0/weather"
    Then the response status code should be 200
    And the JSON response field "url" should be "/anything/weather"

    # With trailing slash — upstream must receive it
    When I send a GET request to "http://localhost:8080/route-exact-upstream/v1.0/weather/"
    Then the response status code should be 200
    And the JSON response field "url" should be "/anything/weather/"

    When I delete the API "route-exact-upstream-slash-api"
    Then the response should be successful
