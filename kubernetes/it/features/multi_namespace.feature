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

@multi-namespace
Feature: Multi-Namespace API Support
  As a platform operator
  I want the gateway to watch APIs across multiple namespaces
  So that APIs in different namespaces are exposed through a single gateway

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

  Scenario: API in different namespace is routed through the gateway
    Given I create namespace "test-ns"
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: multi-ns-api
        namespace: test-ns
      spec:
        displayName: multi-ns-api
        version: v1.0
        context: /multi-ns
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        operations:
          - method: GET
            path: /get
      """
    Then the "RestApi" "multi-ns-api" in namespace "test-ns" should have condition "Programmed" within 120 seconds
    And I send a GET request to "http://localhost:8080/multi-ns/get" expecting 200 not accepting 500 with 10 retries

  Scenario: Cleanup multi-namespace API
    When I delete the "RestApi" "multi-ns-api" in namespace "test-ns"
    And I delete namespace "test-ns"
