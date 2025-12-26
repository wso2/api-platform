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

@gateway-lifecycle
Feature: Gateway CR Lifecycle
  As a platform operator
  I want to manage Gateway CRs
  So that I can deploy and manage gateway instances

  Background:
    Given namespace "default" exists
    And the operator is installed in namespace "operator"
    And the gateway configuration ConfigMap "test-gateway-config" exists in namespace "default"

  @create
  Scenario: Create a Gateway and verify it becomes Programmed
    Given I apply the following CR:
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

  @delete
  Scenario: Delete a Gateway and verify resources are cleaned up
    When I delete the "Gateway" "test-gateway" in namespace "default"
    And I wait for 10 seconds
    Then pods with label "app.kubernetes.io/instance=test-gateway-gateway" should not exist in namespace "default"
    And service "test-gateway-gateway-router" should not exist in namespace "default"
