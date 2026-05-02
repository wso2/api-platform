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

@template
Feature: Template functions in RestApi spec
  As an API administrator
  I want template expressions ({{ env }}, {{ secret }}, {{ default }}) in
  a RestApi spec to be resolved at runtime, while the API responses and
  the persisted DB row keep the original unrendered template body.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: secret template in set-headers policy value is rendered upstream but unrendered in response and DB
    When I create a secret named "tpl-auth-token" with value "xyz-test-token-123"
    Then the response status should be 201

    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: tpl-secret-api-v1.0
      spec:
        displayName: Tpl-Secret-Api
        version: v1.0
        context: /tpl-secret/$version
        upstream:
          main:
            url: http://echo-backend-multi-arch:8080/anything
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Auth-Token
                        value: 'Bearer {{ secret "tpl-auth-token" }}'
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ secret "tpl-auth-token" }}
      """

    # GET response must also echo the unrendered template body
    Given I authenticate using basic auth as "admin"
    When I get the API "tpl-secret-api-v1.0"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ secret "tpl-auth-token" }}
      """

    # DB must persist the unrendered template body
    And the stored RestApi configuration for "tpl-secret-api-v1.0" should contain:
      """
      {{ secret "tpl-auth-token" }}
      """

    # Runtime traffic must hit upstream with the resolved secret value
    And I wait for the endpoint "http://localhost:8080/tpl-secret/v1.0/probe" to be ready
    When I send a GET request to "http://localhost:8080/tpl-secret/v1.0/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer xyz-test-token-123"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "tpl-secret-api-v1.0"
    Then the response should be successful
    When I delete the secret "tpl-auth-token"
    Then the response status should be 200

  Scenario: env template in upstream URL path resolves at runtime
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: tpl-env-api-v1.0
      spec:
        displayName: Tpl-Env-Api
        version: v1.0
        context: /tpl-env/$version
        upstream:
          main:
            url: 'http://echo-backend-multi-arch:8080{{ env "IT_TEMPLATE_PATH" }}'
        operations:
          - method: GET
            path: /probe
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    Given I authenticate using basic auth as "admin"
    When I get the API "tpl-env-api-v1.0"
    Then the response status code should be 200
    And the response body should contain template literal:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    And the stored RestApi configuration for "tpl-env-api-v1.0" should contain:
      """
      {{ env "IT_TEMPLATE_PATH" }}
      """

    # Runtime: upstream must have been built with /anything (the resolved env value)
    And I wait for the endpoint "http://localhost:8080/tpl-env/v1.0/probe" to be ready
    When I send a GET request to "http://localhost:8080/tpl-env/v1.0/probe"
    Then the response status code should be 200
    And the response body should contain "/anything/probe"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "tpl-env-api-v1.0"
    Then the response should be successful

  Scenario: default function returns fallback when env is missing
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: tpl-default-api-v1.0
      spec:
        displayName: Tpl-Default-Api
        version: v1.0
        context: /tpl-default/$version
        upstream:
          main:
            url: http://echo-backend-multi-arch:8080/anything
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Fallback
                        value: '{{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}'
      """
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """

    And the stored RestApi configuration for "tpl-default-api-v1.0" should contain:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """

    And I wait for the endpoint "http://localhost:8080/tpl-default/v1.0/probe" to be ready
    When I send a GET request to "http://localhost:8080/tpl-default/v1.0/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Fallback" with value "fallback-value"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "tpl-default-api-v1.0"
    Then the response should be successful

  Scenario: missing secret reference fails with 400 at deploy time
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: tpl-bad-secret-api-v1.0
      spec:
        displayName: Tpl-Bad-Secret-Api
        version: v1.0
        context: /tpl-bad-secret/$version
        upstream:
          main:
            url: http://echo-backend-multi-arch:8080/anything
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Bad
                        value: '{{ secret "tpl-no-such-secret-xyz" }}'
      """
    Then the response status code should be 400
