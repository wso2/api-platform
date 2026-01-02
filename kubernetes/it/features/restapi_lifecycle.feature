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

@restapi-lifecycle
Feature: RestApi CR Lifecycle
  As an API developer
  I want to manage RestApi CRs
  So that I can expose APIs through the gateway

  Background:
    Given namespace "default" exists
    And the operator is installed in namespace "operator"
    And the gateway configuration ConfigMap "test-gateway-config" exists in namespace "default"
    And httpbin is deployed in namespace "default"
    And I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Gateway
      metadata:
        name: test-gateway
        namespace: default
      spec:
        gatewayClassName: "test"
        apiSelector:
          scope: Cluster
        configRef:
          name: test-gateway-config
      """
    And Gateway "test-gateway" is Programmed in namespace "default"
    And I port-forward service "test-gateway-gateway-router" in namespace "default" to local port 8080

  @create
  Scenario: Create a RestApi and invoke it
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-api
        namespace: default
      spec:
        displayName: test-api
        version: v1.0
        context: /test
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /post
      """
    Then RestApi "test-api" should be Programmed within 120 seconds
    And I send a GET request to "http://localhost:8080/test/get" expecting 200 not accepting 500 with 10 retries

  @update
  Scenario: Update a RestApi and verify new operation
    Given I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-api
        namespace: default
      spec:
        displayName: test-api
        version: v1.0
        context: /test
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /post
          - method: PUT
            path: /put
      """
    And RestApi "test-api" should be Programmed within 120 seconds
    And I send a PUT request to "http://localhost:8080/test/put" expecting 200 not accepting 500 with 10 retries

  @delete
  Scenario: Delete a RestApi and verify route is removed
    When I delete the "RestApi" "test-api" in namespace "default"
    And I send a GET request to "http://localhost:8080/test/get" expecting 404 not accepting 500 with 10 retries
