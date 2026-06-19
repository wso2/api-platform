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

Feature: Header-based route selection (normal RestApi path)
  As an API developer
  I want operations on the same path to be selected by request-header matches
  So that header-based routing works on the custom RestApi path, not only via Gateway API

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # Several operations share the path /pick and differ only by matchHeaders. Each carries a
  # distinct directResponse status so the selected route is unambiguous. This exercises exact
  # header matching, RegularExpression header matching, the more-specific-route-wins precedence
  # over a header-less catch-all, and the operation-level directResponse field — all on the
  # normal management-API path.
  Scenario: Requests are routed to the operation whose header match they satisfy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: header-routing-api
      spec:
        displayName: Header-Routing-API
        version: v1.0
        context: /header-routing/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /ready
          - method: GET
            path: /pick
            matchHeaders:
              - name: x-variant
                value: alpha
            directResponse:
              statusCode: 201
          - method: GET
            path: /pick
            matchHeaders:
              - name: x-variant
                value: beta
            directResponse:
              statusCode: 202
          - method: GET
            path: /pick
            matchHeaders:
              - name: x-variant
                type: RegularExpression
                value: "^v[0-9]+$"
            directResponse:
              statusCode: 203
          - method: GET
            path: /pick
            directResponse:
              statusCode: 200
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/header-routing/v1.0/ready" to be ready

    # Exact header match -> alpha route (201)
    When I clear all headers
    And I set header "x-variant" to "alpha"
    And I send a GET request to "http://localhost:8080/header-routing/v1.0/pick"
    Then the response status code should be 201

    # Exact header match -> beta route (202)
    When I clear all headers
    And I set header "x-variant" to "beta"
    And I send a GET request to "http://localhost:8080/header-routing/v1.0/pick"
    Then the response status code should be 202

    # RegularExpression header match -> regex route (203)
    When I clear all headers
    And I set header "x-variant" to "v12"
    And I send a GET request to "http://localhost:8080/header-routing/v1.0/pick"
    Then the response status code should be 203

    # A header value matching no specific route falls through to the header-less catch-all (200),
    # proving the header routes are not greedily matched.
    When I clear all headers
    And I set header "x-variant" to "does-not-match"
    And I send a GET request to "http://localhost:8080/header-routing/v1.0/pick"
    Then the response status code should be 200

    # No header at all -> catch-all (200)
    When I clear all headers
    And I send a GET request to "http://localhost:8080/header-routing/v1.0/pick"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "header-routing-api"
    Then the response should be successful
