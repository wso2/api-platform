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

@http-method-case
Feature: Operation HTTP methods are normalized to uppercase
  As an API developer
  I want a lower- or mixed-case operation method to be accepted and routed
  So that a method-case typo does not silently deploy and then 404 at runtime

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A lowercase method is accepted and routes as uppercase
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: method-lower-v1.0
      spec:
        displayName: Method-Lower
        version: v1.0
        context: /method-lower/$version
        vhosts:
          main: method-case.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: get
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/method-lower/v1.0/whoami" to be ready with host "method-case.local"

    When I clear all headers
    And I set request host to "method-case.local"
    And I send a GET request to "http://localhost:8080/method-lower/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

  Scenario: A mixed-case method is accepted and routes as uppercase
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: method-mixed-v1.0
      spec:
        displayName: Method-Mixed
        version: v1.0
        context: /method-mixed/$version
        vhosts:
          main: method-case.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: gEt
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/method-mixed/v1.0/whoami" to be ready with host "method-case.local"

    When I clear all headers
    And I set request host to "method-case.local"
    And I send a GET request to "http://localhost:8080/method-mixed/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"

  Scenario: Two methods on one path, mixed case, each routes
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: method-multi-v1.0
      spec:
        displayName: Method-Multi
        version: v1.0
        context: /method-multi/$version
        vhosts:
          main: method-case.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: get
            path: /resource
          - method: Post
            path: /resource
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/method-multi/v1.0/resource" to be ready with host "method-case.local"

    When I clear all headers
    And I set request host to "method-case.local"
    And I send a GET request to "http://localhost:8080/method-multi/v1.0/resource"
    Then the response should be successful
    And the JSON response field "path" should be "/resource"

    When I clear all headers
    And I set request host to "method-case.local"
    And I send a POST request to "http://localhost:8080/method-multi/v1.0/resource"
    Then the response should be successful
    And the JSON response field "path" should be "/resource"
