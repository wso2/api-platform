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

@mtls @mtls-observability
Feature: Seeing what client and backend certificates did
  As a gateway operator
  I want every certificate outcome visible in the access log, analytics and metrics
  So that I can tell which client reached the gateway and why it was accepted or refused

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "obs-partner-a" with usage "downstream"
    And I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: obs-api
      spec:
        displayName: Observability API
        version: v1.0
        context: /obs/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: obs-partner-a
        operations:
          - method: GET
            path: /anything
      """
    And the response should be successful
    And I wait for the endpoint "http://localhost:8080/obs/v1.0/anything" to respond with status 401

  Scenario: An accepted certificate is named in the access log and attributed in analytics
    Given I reset the analytics collector
    When I send a GET request to "https://localhost:8443/obs/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the "gateway-runtime" access log should show within 10 seconds:
      | "peerSubj":"CN=client-valid" |
      | thumbprint of "client-valid" |
      | "tlsVer":"TLSv1.3"           |
      | "sni":"localhost"            |
    And the analytics collector should receive at least 1 event within 10 seconds
    And the latest analytics event should have response status 200
    And the latest analytics event should have the user id of fixture "client-valid"
    And the latest analytics event should have no metadata field "applicationId"

  Scenario: A rejected certificate is an HTTP outcome with the certificate named in the log
    Given I reset the analytics collector
    When I send a GET request to "https://localhost:8443/obs/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    And the "gateway-runtime" access log should show within 10 seconds:
      | "peerSubj":"CN=client-wrong-ca" |
    When I send a GET request to "https://localhost:8443/obs/v1.0/anything" with client certificate "client-expired"
    Then the response status code should be 401
    And the "gateway-runtime" access log should show within 10 seconds:
      | thumbprint of "client-expired" |
    And the analytics collector should receive at least 2 events within 10 seconds
    And the analytics collector should have received at least 2 events
    And the latest analytics event should have response status 401

  Scenario: A request without a certificate logs the certificate fields empty
    When I send a GET request to "http://localhost:8080/obs/v1.0/anything"
    Then the response status code should be 401
    And the "gateway-runtime" access log should show within 10 seconds:
      | "peerSubj":null |
      | "tlsVer":null   |

  Scenario: A backend TLS failure is logged with its reason while the caller sees the sterile body
    Given the gateway identity fixture "gw-identity-a" is stored as "obs-identity-a"
    And the certificate fixture "backend-ca-b" is pooled as "obs-backend-ca-b"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: obs-partner-api
      spec:
        displayName: Observability Partner API
        version: v1.0
        context: /obs-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: obs-identity-a
              trustedCAs: [obs-backend-ca-b]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/obs-partner/v1.0/anything" to respond with status 503
    When I send a GET request to "http://localhost:8080/obs-partner/v1.0/anything"
    Then the response status code should be 503
    And the response body should contain "upstream connect error"
    And the "gateway-runtime" container log should contain "\"upTlsFail\":\"TLS_error" within 10 seconds

  Scenario: Certificate gauges follow the pool and expiry is warned on every channel
    When I upload the certificate fixture "ca-expires-soon" as "obs-expiring" with usage "downstream"
    Then the response status should be 201
    And the response should include a warning with code "CERT_EXPIRES_SOON"
    And the "gateway-controller" container log should contain "CERT_EXPIRES_SOON" within 10 seconds
    When I send a GET request to the gateway controller metrics endpoint
    Then the response should contain metric "certificates_total{usage=\"downstream\"} 2"
    And the response should contain metric "cert_name=\"obs-expiring\""
    And the response should contain metric "cert_name=\"obs-partner-a\""
    When I delete the certificate named "obs-expiring"
    And I send a GET request to the gateway controller metrics endpoint
    Then the response should contain metric "certificates_total{usage=\"downstream\"} 1"
    And the response should not contain metric "cert_name=\"obs-expiring\""
    And the response should contain metric "cert_name=\"obs-partner-a\""

  Scenario: A policy deny is counted like any other policy deny and nothing more
    Given I note the policy engine counter "policy_executions_total" for series labelled "policy_name=\"mtls-auth\",status=\"denied\""
    When I send a GET request to "https://localhost:8443/obs/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    And the policy engine counter "policy_executions_total" for series labelled "policy_name=\"mtls-auth\",status=\"denied\"" should have grown by at least 1 within 5 seconds
    When I send a GET request to the policy engine metrics endpoint
    Then the response should not contain metric "mtls_auth_"
