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

@scoped-mode
Feature: Scoped Operator Mode
  As a platform operator
  I want to run the operator in scoped mode
  So that it only watches specific namespaces and uses namespace-scoped RBAC

  # This feature tests scoped operator mode - installing with watchNamespaces
  # and verifying it only processes resources in watched namespaces

  Background:
    Given cert-manager is installed
    And httpbin is deployed in namespace "default"

  @scoped-install
  Scenario: Install operator in scoped mode
    # Uninstall any existing cluster-wide operator first to avoid conflicts
    Given I uninstall the operator from namespace "operator"
    And I uninstall the operator from namespace "scoped-test"
    When I install the operator in namespace "scoped-test" with watchNamespaces "scoped-test"
    Then the operator pod is ready in namespace "scoped-test"

  @scoped-rbac
  Scenario: Operator in scoped mode creates Role/RoleBinding instead of ClusterRole
    Given the operator is installed in namespace "scoped-test"
    Then Role "gateway-operator-manager-role" should exist in namespace "scoped-test"
    And RoleBinding "gateway-operator-manager-rolebinding" should exist in namespace "scoped-test"
    And ClusterRole "gateway-operator-manager-role" should not exist

  @scoped-positive
  Scenario: Scoped operator processes resources in watched namespace
    Given namespace "scoped-test" exists
    And the gateway configuration ConfigMap "test-gateway-config" exists in namespace "scoped-test"
    And I use namespace "scoped-test"
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Gateway
      metadata:
        name: scoped-gateway
        namespace: scoped-test
      spec:
        gatewayClassName: "test"
        apiSelector:
          scope: Cluster
        configRef:
          name: test-gateway-config
      """
    Then Gateway "scoped-gateway" should be Programmed within 180 seconds
    And pods with label "app.kubernetes.io/instance=scoped-gateway-gateway" should be running in namespace "scoped-test"

  @scoped-api
  Scenario: API in scoped namespace is accessible
    Given Gateway "scoped-gateway" is Programmed in namespace "scoped-test"
    And I port-forward service "scoped-gateway-gateway-router" in namespace "scoped-test" to local port 9090
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: scoped-api
        namespace: scoped-test
      spec:
        displayName: scoped-api
        version: v1.0
        context: /scoped
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        operations:
          - method: GET
            path: /get
      """
    Then the "RestApi" "scoped-api" in namespace "scoped-test" should have condition "Programmed" within 120 seconds
    And I send a GET request to "http://localhost:9090/scoped/get" expecting 200 not accepting 500 with 10 retries

  @scoped-negative
  Scenario: Scoped operator ignores resources in non-watched namespace
    Given I create namespace "ignored-ns"
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ignored-api
        namespace: ignored-ns
      spec:
        displayName: ignored-api
        version: v1.0
        context: /ignored
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        operations:
          - method: GET
            path: /get
      """
    And I wait for 10 seconds
    Then the "RestApi" "ignored-api" in namespace "ignored-ns" status should be empty

  @scoped-cleanup
  Scenario: Cleanup scoped mode resources
    When I delete the "Gateway" "scoped-gateway" in namespace "scoped-test"
    And I delete the "RestApi" "scoped-api" in namespace "scoped-test"
    And I delete the "RestApi" "ignored-api" in namespace "ignored-ns"
    And I wait for 30 seconds
    And I uninstall the operator from namespace "scoped-test"
    And I delete namespace "scoped-test"
    And I delete namespace "ignored-ns"
