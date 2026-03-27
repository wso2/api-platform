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

Feature: Trailing Slash Handling
  As an API consumer
  I want to invoke APIs with or without trailing slashes
  So that the gateway accepts both URL forms and preserves them when forwarding

  Background:
    Given the gateway services are running

  Scenario: Exact path accepts both with and without trailing slash
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: trailing-slash-exact-api
      spec:
        displayName: Trailing-Slash-Exact-API
        version: v1.0
        context: /tslash/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    And I wait for 2 seconds

    # Without trailing slash
    When I send a GET request to "http://localhost:8080/tslash/v1.0/data"
    Then the response should be successful
    And the response body should contain "/api/v2/data"

    # With trailing slash
    When I send a GET request to "http://localhost:8080/tslash/v1.0/data/"
    Then the response should be successful
    And the response body should contain "/api/v2/data/"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "trailing-slash-exact-api"
    Then the response should be successful

  Scenario: Parameterized path accepts both with and without trailing slash
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: trailing-slash-param-api
      spec:
        displayName: Trailing-Slash-Param-API
        version: v1.0
        context: /tslash-param/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /{id}
      """
    Then the response should be successful
    And I wait for 2 seconds

    # Without trailing slash
    When I send a GET request to "http://localhost:8080/tslash-param/v1.0/123"
    Then the response should be successful
    And the response body should contain "/api/v2/123"

    # With trailing slash - should succeed and preserve trailing slash
    When I send a GET request to "http://localhost:8080/tslash-param/v1.0/123/"
    Then the response should be successful
    And the response body should contain "/api/v2/123/"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "trailing-slash-param-api"
    Then the response should be successful
