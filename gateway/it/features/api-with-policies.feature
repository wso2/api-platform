# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: API Configuration with Policies
  As an API developer
  I want to deploy APIs with various policy configurations
  So that I can test policy integration and handler coverage

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy API without any policies
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: no-policy-api
      spec:
        displayName: No-Policy-Api
        version: v1.0
        context: /no-policy
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "no-policy-api"
    Then the response should be successful

  Scenario: Deploy API with operation-level policy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: operation-policy-api
      spec:
        displayName: Operation-Policy-Api
        version: v1.0
        context: /op-policy
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: cors
                version: v0
                params:
                  allowedOrigins:
                    - "*"
                  allowedMethods:
                    - GET
                    - POST
                  allowedHeaders:
                    - "*"
      """
    Then the response should be successful
    And the response should be valid JSON
    # Cleanup
    When I delete the API "operation-policy-api"
    Then the response should be successful

  Scenario: Deploy API with API-level policy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-policy-api
      spec:
        displayName: Api-Level-Policy-Api
        version: v1.0
        context: /api-policy
        upstream:
          main:
            url: http://sample-backend:9080
        policies:
          - name: cors
            version: v0
            params:
              allowedOrigins:
                - "http://localhost:3000"
              allowedMethods:
                - GET
              allowedHeaders:
                - Content-Type
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    And the response should be valid JSON
    # Cleanup
    When I delete the API "api-level-policy-api"
    Then the response should be successful

  Scenario: Update API to add policies
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-add-policy-api
      spec:
        displayName: Update-Add-Policy-Api
        version: v1.0
        context: /update-policy
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    When I update the API "update-add-policy-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-add-policy-api
      spec:
        displayName: Update-Add-Policy-Api
        version: v1.0
        context: /update-policy
        upstream:
          main:
            url: http://sample-backend:9080
        policies:
          - name: cors
            version: v0
            params:
              allowedOrigins:
                - "*"
              allowedMethods:
                - GET
              allowedHeaders:
                - "*"
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Cleanup
    When I delete the API "update-add-policy-api"
    Then the response should be successful

  Scenario: Update API to remove policies
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-remove-policy-api
      spec:
        displayName: Update-Remove-Policy-Api
        version: v1.0
        context: /update-remove
        upstream:
          main:
            url: http://sample-backend:9080
        policies:
          - name: cors
            version: v0
            params:
              allowedOrigins:
                - "*"
              allowedMethods:
                - GET
              allowedHeaders:
                - "*"
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    When I update the API "update-remove-policy-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-remove-policy-api
      spec:
        displayName: Update-Remove-Policy-Api
        version: v1.0
        context: /update-remove
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Cleanup
    When I delete the API "update-remove-policy-api"
    Then the response should be successful

  Scenario: Deploy API with different HTTP methods
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: http-methods-api
      spec:
        displayName: Http-Methods-Api
        version: v1.0
        context: /methods
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /resource
          - method: POST
            path: /resource
          - method: PUT
            path: /resource
          - method: DELETE
            path: /resource
          - method: PATCH
            path: /resource
      """
    Then the response should be successful
    And the response should be valid JSON
    # Cleanup
    When I delete the API "http-methods-api"
    Then the response should be successful
