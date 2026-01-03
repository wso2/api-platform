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

@jwt-authentication
Feature: JWT Authentication Policy
  As an API developer
  I want to protect APIs with JWT authentication
  So that only authenticated requests are allowed

  Background:
    Given namespace "default" exists
    Given cert-manager is installed
    And httpbin is deployed in namespace "default"
    And mock-jwks is deployed in namespace "default"
    And the gateway configuration ConfigMap "test-gateway-config" exists in namespace "default"
    Given I install the operator in namespace "operator"
    And the operator is installed in namespace "operator"
    And the gateway configuration ConfigMap "test-gateway-config" exists in namespace "default"
    And httpbin is deployed in namespace "default"
    And mock-jwks is deployed in namespace "default"
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
    And I port-forward service "mock-jwks" in namespace "default" to local port 8081

  @jwt-reject-unauthenticated
  Scenario: JWT protected API rejects unauthenticated requests
    Given I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: jwt-api
        namespace: default
      spec:
        displayName: jwt-api
        version: v1.0
        context: /secure
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        policies:
          - name: JwtAuthentication
            version: v0.1.0
            params:
              issuers:
                - MockKeyManager
        operations:
          - method: GET
            path: /get
      """
    And RestApi "jwt-api" should be Programmed within 120 seconds
    And I send a GET request to "http://localhost:8080/secure/get" expecting 401 not accepting 500 with 10 retries

  @jwt-accept-authenticated
  Scenario: JWT protected API accepts valid tokens
    Given I obtain a token from "http://localhost:8081/token"
    And I send a GET request with the token to "http://localhost:8080/secure/get" expecting 200 not accepting 500 with 10 retries

  @jwt-issuer-update
  Scenario: Updating JWT issuer rejects previously valid tokens
    # First get a token from MockKeyManager
    Given I obtain a token from "http://localhost:8081/token"
    And I send a GET request with the token to "http://localhost:8080/secure/get" expecting 200 not accepting 500 with 10 retries
    
    # Update API to use a different (non-existent) issuer
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: jwt-api
        namespace: default
      spec:
        displayName: jwt-api
        version: v1.0
        context: /secure
        upstream:
          main:
            url: http://httpbin.default.svc.cluster.local:80
        policies:
          - name: JwtAuthentication
            version: v0.1.0
            params:
              issuers:
                - DummyKeyManager
        operations:
          - method: GET
            path: /get
      """
    And RestApi "jwt-api" should be Programmed within 120 seconds

    # Same token should now be rejected
    And I send a GET request with the token to "http://localhost:8080/secure/get" expecting 401 not accepting 500 with 10 retries
