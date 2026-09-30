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

# Runs against a gateway configured with
#   [router.downstream_tls.client_certificate_header]
#   trust_any = true
# Run with: cd it && make test-mtls-header-bypass

@mtls @mtls-header-bypass
Feature: Believing the relayed certificate from any connection on a trusted network
  As a gateway administrator whose gateway is reachable from nothing but the front proxy
  I want the relayed certificate believed without the proxy authenticating itself
  So that the topology works, while the gateway warns me on every deploy that the header is
  trusted from anyone

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "bypass-partner-a" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "bypass-partner-b" with usage "downstream"

  Scenario: Every mtls-auth deployment warns that the bypass is active
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: bypass-api
      spec:
        displayName: Bypass API
        version: v1.0
        context: /bypass/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: bypass-partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And the response should include a warning with code "HEADER_CERT_BYPASS_ACTIVE"

  Scenario Outline: The header is believed from a connection that presented no certificate or a pooled one, never in place of a rejected one
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: bypass-api
      spec:
        displayName: Bypass API
        version: v1.0
        context: /bypass/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: bypass-partner-a
        operations:
          - method: GET
            path: /anything
      """
    And the response should be successful
    And I wait for the endpoint "http://localhost:8080/bypass/v1.0/anything" to respond with status 401
    When I send a GET request to "<url>" <presenting>
    Then the response status code should be <status>

    Examples:
      | url                                             | presenting                                                                                                              | status |
      | https://localhost:8443/bypass/v1.0/anything     | with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"                    | 200    |
      | https://localhost:8443/bypass/v1.0/anything     | with client certificate "client-wrong-ca" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"     | 200    |
      | https://localhost:8443/bypass/v1.0/anything     | with client certificate "client-expired" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"      | 401    |
      | https://localhost:8443/bypass/v1.0/anything     | with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"                 | 401    |
      | https://localhost:8443/bypass/v1.0/anything     | with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-expired"                  | 401    |
      | https://localhost:8443/bypass/v1.0/anything     | with no client certificate                                                                                              | 401    |
      | http://localhost:8080/bypass/v1.0/anything      | with header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"                                             | 200    |

  Scenario: Under the bypass a believed header reaches the backend as X-Forwarded-Client-Cert
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: bypass-api
      spec:
        displayName: Bypass API
        version: v1.0
        context: /bypass/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: bypass-partner-a
        operations:
          - method: GET
            path: /anything
      """
    And the response should be successful
    And I wait for the endpoint "http://localhost:8080/bypass/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/bypass/v1.0/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"

  Scenario: Under the bypass an API that opts out of the certificate header receives neither header
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: bypass-nofwd-api
      spec:
        displayName: Bypass Opt-out API
        version: v1.0
        context: /bypass-nofwd/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: bypass-partner-a
              forwardCertificate: false
        operations:
          - method: GET
            path: /anything
      """
    And the response should be successful
    And I wait for the endpoint "http://localhost:8080/bypass-nofwd/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/bypass-nofwd/v1.0/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the response should not contain echoed header "x-forwarded-client-cert"
