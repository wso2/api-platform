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

@certificates
Feature: Certificate management
  As a platform operator
  I want to list, upload, delete, and reload trust-store certificates
  So that I can manage the gateway's TLS trust material

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: List certificates endpoint works
    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List certificates returns the expected structure
    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "certificates"
    And the JSON response should have field "totalCount"

  Scenario: Upload certificate with invalid PEM format is rejected
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "invalid-cert",
        "certificate": "This is not a valid PEM certificate"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "Invalid certificate"

  Scenario: Upload certificate without a name is rejected
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "certificate": "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate without certificate data is rejected
    Given I generate a unique value from "cert-no-data" and store it as "certName"
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "${CTX:certName}"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate with an empty body is rejected
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {}
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate with malformed PEM data is rejected
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "malformed-cert",
        "certificate": "-----BEGIN CERTIFICATE-----\nINVALIDBASE64DATA!!!\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate with a missing BEGIN marker is rejected
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "no-begin-cert",
        "certificate": "MIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload a valid certificate successfully
    Given I generate a unique value from "cert-valid" and store it as "certName"
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "${CTX:certName}",
        "certificate": "-----BEGIN CERTIFICATE-----\nMIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\nBQAwgYExCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQH\nDA1Nb3VudGFpbiBWaWV3MRowGAYDVQQKDBFUZXN0IE9yZ2FuaXphdGlvbjEQMA4G\nA1UECwwHVGVzdGluZzEXMBUGA1UEAwwOc2VjdXJlLWJhY2tlbmQwHhcNMjUxMTI2\nMDYwNzI2WhcNMjYxMTI2MDYwNzI2WjCBgTELMAkGA1UEBhMCVVMxEzARBgNVBAgM\nCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcxGjAYBgNVBAoMEVRl\nc3QgT3JnYW5pemF0aW9uMRAwDgYDVQQLDAdUZXN0aW5nMRcwFQYDVQQDDA5zZWN1\ncmUtYmFja2VuZDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAK4ffloD\ngwHOZGhV4nJuznQS6P95TdTuQ3gXE2/TXxz9mUivSkr2xYd8QCK7+1sdxskKCdSM\nUYftW2VB9xhMeJUJOg7vWCTNCY30ffMxV/oQSQNZGGjN9hO2qvQKScIghODr/emZ\nf4dVgsoisKXwG1+WXvkF57zpeN62pi1H0rKm03aRNJBsNhmuU7ELiHVlt6/yNOVG\n4DG9mC0ndp0oI/fMfqvX/8dE7wq+IJTEZvXFm/Hb7+0aw9FAKmZLSNQYQZPZYxVH\nu0ag57I5nq0bmHGXKcMtLNFjU1bu0G4tIcvbU6JoSvqNtZFsPMMiEKGKl6sEoAhr\ndsD9P/4/yyLwsmMzlESWouf+OR1W3rCXeh55QBKIuRQU2LaM79UFaTjtD+J7Q9Ww\nHXlZ2m8bYZoQT1hicqTuvCGrk0eTQf4Q6KT9WPo9WHxC4Hn7NO3cBOOEiqrt/B0v\nJwt9sdxodMWoKWyCxbWYTeTzPXxGcoR9fZqZKz0fIDRK5g5qTEcBrseNchqS46XI\np+KcUZXZ1+PHUr7ItFPif0v4q60GuWgpC3lE8nmj7TknWKSRRbyPZf6BFTpqlLUl\nWf98InpeUD+UKeifZTaucrqvB6QA3G0tbFg/AdTmA3QM52gxbAEkBhRUxkVV0oW2\nw0XnPO0AtKFzukwL+WOJQPove1qcrRG92gPXAgMBAAGjgZYwgZMwHQYDVR0OBBYE\nFG0uv/Hg+71KVmKKXoTaLWniAxFZMB8GA1UdIwQYMBaAFG0uv/Hg+71KVmKKXoTa\nLWniAxFZMA8GA1UdEwEB/wQFMAMBAf8wQAYDVR0RBDkwN4IOc2VjdXJlLWJhY2tl\nbmSCFHNlY3VyZS1iYWNrZW5kLmxvY2Fsgglsb2NhbGhvc3SHBH8AAAEwDQYJKoZI\nhvcNAQELBQADggIBAABtg4O9JpWW81ltidIVctocPkTXn+2s6YZfI3mZvlKoFZDA\nX8L0p7oshG+g9OkbYfTzb2yZ6+BIPuMSUMvgi/QDRM8UeNXPt+1YyWLwXxsRjbfL\nCgRtN+5HVBIemsAV3N/sN8FG65eNaIhjvNR4wEa/EeyJyNNWL3VD8uVSMAaMbjZQ\nJkYeHpjnAmZiQqWCtNGsv3srWwsgHiZFSidpDNPU3KeDnCzJs5VK0CPq7/Eb9BT9\nRF6aq/BZE0ld0gnTrnisYTlyW53XSPAJTdWLE+stMUMJafoXYl7bEwT/NgBbScGu\n/rMZiayHbSmgIb5ikY/YycPWWp4alN6Ckb8+Vk9ied0p5p4G2VlUbPVApmmnWJwW\nnnUWil3xKifnGkkEbgdqzNMIuectfCYNpcK3519n5vXkWXWfungdbRXpi83VKf0a\ne7xDP2iV8c/otubk2BpU1q+9JbQuYIS4D0NCl2flPdHgE6VwKXBpqYmAbifUx38R\nPHFFVagSydGgirQ6ZZVRtgJzUmI7BR86sBjQwaevk8+lkl5w/xPqdQu/9ryaL9Zb\nF6WPkiF8LceoSNlinm4rrCiRTRAKbNOrsXSvE1SMIi4JVfHSVp0K+oOpIeJfgZ/b\nJIlH5QcT1SnwdkIHtyScYWnBIpZ4ZZc8kmeKPC0WqHosCVtSeOnGrKc4rwLC\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "name" should be "${CTX:certName}"
    And the JSON response should have field "id"
    And the JSON response should have field "subject"
    And the JSON response should have field "issuer"
    And the JSON response should have field "notAfter"
    And the JSON response field "count" should be 1
    And I store the JSON response field "id" as "certId"
    And I register the "certificate" "${CTX:certId}" for cleanup

    When I send a "DELETE" request to the "gateway-controller" service at "/certificates/${CTX:certId}"
    Then the response should be successful

  Scenario: An uploaded certificate appears in the certificate list
    Given I generate a unique value from "cert-list" and store it as "certName"
    When I send a "POST" request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "${CTX:certName}",
        "certificate": "-----BEGIN CERTIFICATE-----\nMIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\nBQAwgYExCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQH\nDA1Nb3VudGFpbiBWaWV3MRowGAYDVQQKDBFUZXN0IE9yZ2FuaXphdGlvbjEQMA4G\nA1UECwwHVGVzdGluZzEXMBUGA1UEAwwOc2VjdXJlLWJhY2tlbmQwHhcNMjUxMTI2\nMDYwNzI2WhcNMjYxMTI2MDYwNzI2WjCBgTELMAkGA1UEBhMCVVMxEzARBgNVBAgM\nCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcxGjAYBgNVBAoMEVRl\nc3QgT3JnYW5pemF0aW9uMRAwDgYDVQQLDAdUZXN0aW5nMRcwFQYDVQQDDA5zZWN1\ncmUtYmFja2VuZDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAK4ffloD\ngwHOZGhV4nJuznQS6P95TdTuQ3gXE2/TXxz9mUivSkr2xYd8QCK7+1sdxskKCdSM\nUYftW2VB9xhMeJUJOg7vWCTNCY30ffMxV/oQSQNZGGjN9hO2qvQKScIghODr/emZ\nf4dVgsoisKXwG1+WXvkF57zpeN62pi1H0rKm03aRNJBsNhmuU7ELiHVlt6/yNOVG\n4DG9mC0ndp0oI/fMfqvX/8dE7wq+IJTEZvXFm/Hb7+0aw9FAKmZLSNQYQZPZYxVH\nu0ag57I5nq0bmHGXKcMtLNFjU1bu0G4tIcvbU6JoSvqNtZFsPMMiEKGKl6sEoAhr\ndsD9P/4/yyLwsmMzlESWouf+OR1W3rCXeh55QBKIuRQU2LaM79UFaTjtD+J7Q9Ww\nHXlZ2m8bYZoQT1hicqTuvCGrk0eTQf4Q6KT9WPo9WHxC4Hn7NO3cBOOEiqrt/B0v\nJwt9sdxodMWoKWyCxbWYTeTzPXxGcoR9fZqZKz0fIDRK5g5qTEcBrseNchqS46XI\np+KcUZXZ1+PHUr7ItFPif0v4q60GuWgpC3lE8nmj7TknWKSRRbyPZf6BFTpqlLUl\nWf98InpeUD+UKeifZTaucrqvB6QA3G0tbFg/AdTmA3QM52gxbAEkBhRUxkVV0oW2\nw0XnPO0AtKFzukwL+WOJQPove1qcrRG92gPXAgMBAAGjgZYwgZMwHQYDVR0OBBYE\nFG0uv/Hg+71KVmKKXoTaLWniAxFZMB8GA1UdIwQYMBaAFG0uv/Hg+71KVmKKXoTa\nLWniAxFZMA8GA1UdEwEB/wQFMAMBAf8wQAYDVR0RBDkwN4IOc2VjdXJlLWJhY2tl\nbmSCFHNlY3VyZS1iYWNrZW5kLmxvY2Fsgglsb2NhbGhvc3SHBH8AAAEwDQYJKoZI\nhvcNAQELBQADggIBAABtg4O9JpWW81ltidIVctocPkTXn+2s6YZfI3mZvlKoFZDA\nX8L0p7oshG+g9OkbYfTzb2yZ6+BIPuMSUMvgi/QDRM8UeNXPt+1YyWLwXxsRjbfL\nCgRtN+5HVBIemsAV3N/sN8FG65eNaIhjvNR4wEa/EeyJyNNWL3VD8uVSMAaMbjZQ\nJkYeHpjnAmZiQqWCtNGsv3srWwsgHiZFSidpDNPU3KeDnCzJs5VK0CPq7/Eb9BT9\nRF6aq/BZE0ld0gnTrnisYTlyW53XSPAJTdWLE+stMUMJafoXYl7bEwT/NgBbScGu\n/rMZiayHbSmgIb5ikY/YycPWWp4alN6Ckb8+Vk9ied0p5p4G2VlUbPVApmmnWJwW\nnnUWil3xKifnGkkEbgdqzNMIuectfCYNpcK3519n5vXkWXWfungdbRXpi83VKf0a\ne7xDP2iV8c/otubk2BpU1q+9JbQuYIS4D0NCl2flPdHgE6VwKXBpqYmAbifUx38R\nPHFFVagSydGgirQ6ZZVRtgJzUmI7BR86sBjQwaevk8+lkl5w/xPqdQu/9ryaL9Zb\nF6WPkiF8LceoSNlinm4rrCiRTRAKbNOrsXSvE1SMIi4JVfHSVp0K+oOpIeJfgZ/b\nJIlH5QcT1SnwdkIHtyScYWnBIpZ4ZZc8kmeKPC0WqHosCVtSeOnGrKc4rwLC\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And I store the JSON response field "id" as "certId"
    And I register the "certificate" "${CTX:certId}" for cleanup

    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response body should contain "${CTX:certName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/certificates/${CTX:certId}"
    Then the response should be successful

  Scenario: Delete non-existent certificate returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/certificates/non-existent-cert-id"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Reload certificates
    When I send a "POST" request to the "gateway-controller" service at "/certificates/reload" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should contain "reload"
