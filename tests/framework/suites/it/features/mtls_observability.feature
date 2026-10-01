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

  Every scenario starts from an empty client authority pool, and every HTTPS request waits until
  the gateway has applied the pool the scenario built. Access log lines are JSON. Certificate
  gauges and policy counters are read from the controller and the policy engine.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "obs" and store it as "apiName"
    And I generate a unique API version from "obs" and store it as "apiVersion"
    And I generate a unique API context from "/obs" and store it as "apiContext"
    And I generate a unique resource name from "obs-partner-a" and store it as "caA"
    And the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                         |
      | name                   | ${CTX:apiName}                                                                    |
      | spec.displayName       | Observability API                                                                 |
      | spec.version           | ${CTX:apiVersion}                                                                 |
      | spec.context           | ${CTX:apiContext}/$version                                                        |
      | spec.upstream.main.url | http://testbench:3002                                                             |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                             |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401

  Scenario: An accepted certificate is named in the access log and attributed in analytics
    Given I wait for the analytics collector to settle
    And I reset the analytics collector
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the "gateway-runtime" access log since the latest request should show a line for "${CTX:apiContext}/${CTX:apiVersion}/anything" with:
      | "peerSubj":"CN=client-valid" |
      | thumbprint of "client-valid" |
      | "tlsVer":"TLSv1.3"           |
      | "sni":"localhost"            |
    And the analytics collector should have received at least 1 event
    And I wait for the analytics collector to settle
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/anything" should have response status 200
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/anything" should have the user id of fixture "client-valid"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/anything" should not have metadata field "applicationId"

  Scenario: A rejected certificate is an HTTP outcome with the certificate named in the log
    Given I wait for the analytics collector to settle
    And I reset the analytics collector
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for "${CTX:apiContext}/${CTX:apiVersion}/anything" with:
      | "peerSubj":"CN=client-wrong-ca" |
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-expired"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for "${CTX:apiContext}/${CTX:apiVersion}/anything" with:
      | thumbprint of "client-expired" |
    And the analytics collector should have received at least 2 events
    And I wait for the analytics collector to settle
    And the latest analytics event for API context "${CTX:apiContext}/${CTX:apiVersion}" should have response status 401

  Scenario: A request without a certificate logs the certificate fields empty
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for "${CTX:apiContext}/${CTX:apiVersion}/anything" with:
      | "peerSubj":null |
      | "tlsVer":null   |

  Scenario: A backend TLS failure is logged with its reason while the caller sees the sterile body
    # The upstream is the TLS backend on port 8445. Its certificate is issued by the authority this API trusts,
    # for another hostname, so the gateway refuses it while verifying the name.
    Given I generate a unique resource name from "obs-identity-a" and store it as "identity"
    And I generate a unique resource name from "obs-backend-ca" and store it as "backendCA"
    And I generate a unique value from "obs-partner" and store it as "partnerName"
    And I generate a unique API version from "obs-partner" and store it as "partnerVersion"
    And I generate a unique API context from "/obs-partner" and store it as "partnerContext"
    And the gateway identity fixture "gw-identity-a" is stored as "${CTX:identity}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}" with usage "upstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                  | ${CTX:gatewaySpecVersion}                                                                                                                                                          |
      | name                        | ${CTX:partnerName}                                                                                                                                                                 |
      | spec.displayName            | Observability Partner API                                                                                                                                                          |
      | spec.version                | ${CTX:partnerVersion}                                                                                                                                                              |
      | spec.context                | ${CTX:partnerContext}/$version                                                                                                                                                     |
      | spec.upstreamDefinitions    | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8445"}],"tls":{"identity":"${CTX:identity}","trustedCAs":["${CTX:backendCA}"]}}]                                      |
      | spec.upstream.main.ref      | partner-a                                                                                                                                                                          |
      | spec.operations             | [{"method":"GET","path":"/anything"}]                                                                                                                                              |
    Then the response should be successful
    And I send a "GET" request to "${CTX:partnerContext}/${CTX:partnerVersion}/anything" until the route answers 503
    When I send a "GET" request to "${CTX:partnerContext}/${CTX:partnerVersion}/anything"
    Then the response status code should be 503
    And the response body should contain "upstream connect error"
    And the "gateway-runtime" log since the latest request should contain "\"upTlsFail\":\"TLS_error:|268435581:SSL_routines:OPENSSL_internal:CERTIFICATE_VERIFY_FAILED:verify_cert_failed:_SAN_matcher,_certificate_SANs_are_[not-this-host.test]:TLS_error_end\""

  Scenario: Certificate gauges follow the pool and expiry is warned on every channel
    Given I generate a unique resource name from "obs-expiring" and store it as "expiring"
    When I upload the certificate fixture "ca-expires-soon" as "${CTX:expiring}" with usage "downstream"
    Then the response status should be 201
    And the response should include a warning with code "CERT_EXPIRES_SOON"
    And the "gateway-controller" log since the latest request should contain "CERT_EXPIRES_SOON"
    When I send a "GET" request to the "controller-metrics" service at "/metrics"
    Then the response should contain metric "gateway_controller_certificates_total" with labels {usage="downstream"} and value 2
    And the response should contain metric "gateway_controller_certificate_expiry_seconds" with labels {cert_name="${CTX:expiring}"}
    And the response should contain metric "gateway_controller_certificate_expiry_seconds" with labels {cert_name="${CTX:caA}"}
    When I delete the certificate named "${CTX:expiring}"
    And I send a "GET" request to the "controller-metrics" service at "/metrics"
    Then the response should contain metric "gateway_controller_certificates_total" with labels {usage="downstream"} and value 1
    And the response should not contain metric "gateway_controller_certificate_expiry_seconds" with labels {cert_name="${CTX:expiring}"}
    And the response should contain metric "gateway_controller_certificate_expiry_seconds" with labels {cert_name="${CTX:caA}"}

  Scenario: A policy deny is counted like any other policy deny and nothing more
    Given I note the policy engine counter "policy_executions_total" for series labelled {policy_name="mtls-auth",status="denied"}
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    And the policy engine counter "policy_executions_total" for series labelled {policy_name="mtls-auth",status="denied"} should have grown by at least 1
    When I send a "GET" request to the "policy-engine-metrics" service at "/metrics"
    Then the response should not contain metric "mtls_auth_"
