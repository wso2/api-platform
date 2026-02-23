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

Feature: Sandbox Routing
  As an API developer
  I want main and sandbox upstreams to route by host
  So that sandbox traffic can be validated independently

  Background:
    Given the gateway services are running

  Scenario: Route requests to different upstreams using main and sandbox vhosts
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-api-v1.0
      spec:
        displayName: Env-Routing-API
        version: v1.0
        context: /env/$version
        vhosts:
          main: main.local
          sandbox: sandbox.local
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env/v1.0/whoami" to be ready with host "main.local"

    When I clear all headers
    And I set request host to "main.local"
    And I send a GET request to "http://localhost:8080/env/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox.local"
    And I send a GET request to "http://localhost:8080/env/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-api-v1.0"
    Then the response should be successful

  Scenario: Route requests with main via ref and sandbox via url
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-main-ref-v1.0
      spec:
        displayName: Env-Routing-Main-Ref-API
        version: v1.0
        context: /env-main-ref/$version
        vhosts:
          main: main-ref.local
          sandbox: sandbox-ref.local
        upstreamDefinitions:
          - name: main-upstream
            upstreams:
              - urls:
                  - http://sample-backend:9080
        upstream:
          main:
            ref: main-upstream
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-main-ref/v1.0/whoami" to be ready with host "main-ref.local"

    When I clear all headers
    And I set request host to "main-ref.local"
    And I send a GET request to "http://localhost:8080/env-main-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox-ref.local"
    And I send a GET request to "http://localhost:8080/env-main-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-main-ref-v1.0"
    Then the response should be successful

  Scenario: Route requests with main via url and sandbox via ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Ref-API
        version: v1.0
        context: /env-sandbox-ref/$version
        vhosts:
          main: main-sandbox-ref.local
          sandbox: sandbox-sandbox-ref.local
        upstreamDefinitions:
          - name: sandbox-upstream
            upstreams:
              - urls:
                  - http://sample-backend:9080/sandbox
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-ref/v1.0/whoami" to be ready with host "main-sandbox-ref.local"

    When I clear all headers
    And I set request host to "main-sandbox-ref.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox-sandbox-ref.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-ref-v1.0"
    Then the response should be successful

  Scenario: Deploy API with missing main ref definition should fail
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-missing-ref-v1.0
      spec:
        displayName: Env-Routing-Missing-Ref-API
        version: v1.0
        context: /env-missing-ref/$version
        upstream:
          main:
            ref: non-existent-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Referenced upstream definition 'non-existent-upstream' not found"

  Scenario: Deploy API with invalid URL in upstreamDefinitions should fail
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-invalid-upstream-def-v1.0
      spec:
        displayName: Env-Routing-Invalid-Definition-API
        version: v1.0
        context: /env-invalid-def/$version
        upstreamDefinitions:
          - name: invalid-upstream
            upstreams:
              - urls:
                  - ftp://sample-backend:9080
        upstream:
          main:
            ref: invalid-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "URL must use http or https scheme"
