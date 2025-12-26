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

@full-lifecycle
Feature: Complete Operator Lifecycle
  As a platform operator
  I want to verify the full operator lifecycle
  Including installation, Gateway/API management, and uninstallation

  # This feature demonstrates self-contained tests that manage their own operator instances
  # Tests install the operator, perform operations, and clean up when done

  @setup
  Scenario: Install prerequisites
    Given cert-manager is installed
    And httpbin is deployed in namespace "default"
    And mock-jwks is deployed in namespace "default"
    And the gateway configuration ConfigMap "test-gateway-config" exists in namespace "default"

  @install-operator
  Scenario: Install the operator in cluster-wide mode
    Given I install the operator in namespace "operator"
    Then the operator pod is ready in namespace "operator"

  @gateway-create
  Scenario: Create a Gateway
    Given the operator is installed in namespace "operator"
    When I apply the following CR:
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
    Then Gateway "test-gateway" should be Programmed within 180 seconds
    And pods with label "app.kubernetes.io/instance=test-gateway-gateway" should be running in namespace "default"
    And service "test-gateway-gateway-router" should exist in namespace "default"

  @api-create
  Scenario: Create and invoke a RestApi
    Given Gateway "test-gateway" is Programmed in namespace "default"
    And I port-forward service "test-gateway-gateway-router" in namespace "default" to local port 8080
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
    When I wait for 5 seconds
    And I send a GET request to "http://localhost:8080/test/get"
    Then the response status code should be 200

  @api-update
  Scenario: Update a RestApi and verify new operation
    Given I port-forward service "test-gateway-gateway-router" in namespace "default" to local port 8080
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
          - method: PUT
            path: /put
      """
    And RestApi "test-api" should be Programmed within 120 seconds
    And I wait for 5 seconds
    Then I send a PUT request to "http://localhost:8080/test/put"
    And the response status code should be 200

  @api-delete
  Scenario: Delete a RestApi and verify route is removed
    Given I port-forward service "test-gateway-gateway-router" in namespace "default" to local port 8080
    When I delete the "RestApi" "test-api" in namespace "default"
    And I wait for 5 seconds
    And I send a GET request to "http://localhost:8080/test/get"
    Then the response status code should be 404

  @gateway-delete
  Scenario: Delete a Gateway and verify cleanup
    When I delete the "Gateway" "test-gateway" in namespace "default"
    And I wait for 10 seconds
    Then pods with label "app.kubernetes.io/instance=test-gateway-gateway" should not exist in namespace "default"

  @uninstall-operator
  Scenario: Uninstall and reinstall the operator
    When I uninstall the operator from namespace "operator"
    And I wait for 10 seconds
    And I install the operator in namespace "operator"
    Then the operator pod is ready in namespace "operator"

  @full-cycle
  Scenario: Complete create-invoke-delete cycle after reinstall
    Given the operator is installed in namespace "operator"
    # Create Gateway
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Gateway
      metadata:
        name: cycle-gateway
        namespace: default
      spec:
        gatewayClassName: "test"
        apiSelector:
          scope: Cluster
        configRef:
          name: test-gateway-config
      """
    Then Gateway "cycle-gateway" should be Programmed within 180 seconds
    
    # Create RestApi
    And I port-forward service "cycle-gateway-gateway-router" in namespace "default" to local port 8090
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: cycle-api
        namespace: default
      spec:
        displayName: cycle-api
        version: v1.0
        context: /cycle
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        operations:
          - method: GET
            path: /get
      """
    Then RestApi "cycle-api" should be Programmed within 120 seconds
    
    # Invoke API
    When I wait for 5 seconds
    And I send a GET request to "http://localhost:8090/cycle/get"
    Then the response status code should be 200
    
    # Cleanup
    When I delete the "RestApi" "cycle-api" in namespace "default"
    And I delete the "Gateway" "cycle-gateway" in namespace "default"
    And I wait for 10 seconds
    Then pods with label "app.kubernetes.io/instance=cycle-gateway-gateway" should not exist in namespace "default"
