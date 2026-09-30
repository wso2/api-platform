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

@mtls @mtls-pool-references
Feature: Pool entries referenced by APIs, and the levers that revoke a client
  As a gateway administrator and an API developer
  I want removing trust to be safe and immediate
  So that a pool entry an API depends on cannot vanish underneath it, and cutting off one
  client is an edit that applies on the next request

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== WHO REFERENCES WHAT ====================

  Scenario: The listing counts the APIs that name an authority, not the ones that inherit the pool
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "ref-partner-b" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-naming-api
      spec:
        displayName: Ref Naming API
        version: v1.0
        context: /ref-naming/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-inheriting-api
      spec:
        displayName: Ref Inheriting API
        version: v1.0
        context: /ref-inheriting/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "ref-partner-a" should have "referencedByApis" equal to 1
    And the listed certificate "ref-partner-b" should have "referencedByApis" equal to 0

  Scenario: An authority named in an API's accept list cannot be removed while the reference stands
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-naming-api
      spec:
        displayName: Ref Naming API
        version: v1.0
        context: /ref-naming/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-naming/v1.0/anything" to respond with status 401
    When I delete the certificate named "ref-partner-a"
    Then the response status should be 409
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "is named by 1 deployed API"
    And the response should list a validation error for field "spec.policies[0].params.accept[0].ca" containing "ref-naming-api"
    When I send a GET request to "https://localhost:8443/ref-naming/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I delete the API "ref-naming-api"
    And I delete the certificate named "ref-partner-a" once no API references it
    Then the response should be successful

  Scenario: An authority referenced only by inheriting APIs can be removed, and they stop accepting it on the next request
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "ref-partner-b" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-inheriting-api
      spec:
        displayName: Ref Inheriting API
        version: v1.0
        context: /ref-inheriting/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-inheriting/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/ref-inheriting/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 200
    When I delete the certificate named "ref-partner-b"
    Then the response should be successful
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/ref-inheriting/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/ref-inheriting/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: The last client authority cannot be removed while any API attaches mtls-auth
    Given the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "ref-only-authority" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-inheriting-api
      spec:
        displayName: Ref Inheriting API
        version: v1.0
        context: /ref-inheriting/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    When I delete the certificate named "ref-only-authority"
    Then the response status should be 409
    And the JSON response field "message" should contain "cannot remove the last client-CA authority while 1 deployed API"
    And the response should list a validation error for field "spec.policies[0]" containing "ref-inheriting-api"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "ref-only-authority"
    Given the certificate fixture "ca-b" is pooled as "ref-replacement" with usage "downstream"
    When I delete the certificate named "ref-only-authority"
    Then the response should be successful

  Scenario: A relay entry can be removed even when it is the last one, since header mode simply turns off
    Given the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    And I upload the certificate fixture "edge-lb-ca" as "ref-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-naming-api
      spec:
        displayName: Ref Naming API
        version: v1.0
        context: /ref-naming/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    When I delete the certificate named "ref-edge-lb"
    Then the response should be successful

  Scenario: An unreferenced upstream trust certificate can be deleted
    Given the certificate fixture "backend-ca" is pooled as "ref-backend-trust"
    When I delete the certificate named "ref-backend-trust"
    Then the response should be successful

  # ==================== REVOKING ONE CLIENT IS AN EDIT ====================

  Scenario: Removing one SAN from the list cuts off exactly the clients carrying it
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-lever-api
      spec:
        displayName: Ref Lever API
        version: v1.0
        context: /ref-lever/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
                  match:
                    uriSANs: ["urn:partner-a:payments", "urn:partner-a:first"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-lever/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-multi-san"
    Then the response status code should be 200
    When I update the API "ref-lever-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-lever-api
      spec:
        displayName: Ref Lever API
        version: v1.0
        context: /ref-lever/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
                  match:
                    uriSANs: ["urn:partner-a:payments"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-multi-san"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: Removing one partner's entry leaves the other partner untouched
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "ref-partner-b" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-lever-api
      spec:
        displayName: Ref Lever API
        version: v1.0
        context: /ref-lever/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
                - ca: ref-partner-b
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-lever/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 200
    When I update the API "ref-lever-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-lever-api
      spec:
        displayName: Ref Lever API
        version: v1.0
        context: /ref-lever/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: A thumbprint cut-over lists old and new, then drops the old
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    When I deploy this API configuration with fixture values:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-lever-api
      spec:
        displayName: Ref Lever API
        version: v1.0
        context: /ref-lever/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
                  thumbprints: ["{{thumbprint "client-valid"}}", "{{thumbprint "client-renewed"}}"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-lever/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-renewed"
    Then the response status code should be 200
    When I update the API "ref-lever-api" with this configuration with fixture values:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-lever-api
      spec:
        displayName: Ref Lever API
        version: v1.0
        context: /ref-lever/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
                  thumbprints: ["{{thumbprint "client-renewed"}}"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/ref-lever/v1.0/anything" with client certificate "client-renewed"
    Then the response status code should be 200

  # ==================== EDGE CASES IN THE ACCEPT LIST ====================

  Scenario: Listing the same authority twice is accepted and evaluates once
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-edge-api
      spec:
        displayName: Ref Edge API
        version: v1.0
        context: /ref-edge/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a
                  match:
                    uriSANs: ["urn:partner-a:nothing"]
                - ca: ref-partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-edge/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/ref-edge/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: The same authority pooled under two names is usable by either name
    Given the certificate fixture "ca-a" is pooled as "ref-partner-a" with usage "downstream"
    And the certificate fixture "ca-a" is pooled as "ref-partner-a-again" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ref-edge-api
      spec:
        displayName: Ref Edge API
        version: v1.0
        context: /ref-edge/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: ref-partner-a-again
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ref-edge/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/ref-edge/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
