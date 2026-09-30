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

@mtls @mtls-pool
Feature: Client certificate authority pool
  As a gateway administrator
  I want to curate the set of authorities whose client certificates the gateway trusts
  So that API developers can select from them when protecting an API with mutual TLS

  The pool is managed through the /certificates endpoint. A certificate uploaded with
  usage "downstream" is a client authority; one uploaded without usage (or with usage
  "upstream") is backend trust. Certificate fixtures are generated
  into resources/mtls-pki before the suite runs.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== UPLOADING A CLIENT AUTHORITY ====================

  Scenario: A CA certificate uploaded with usage downstream becomes a pooled client authority
    When I upload the certificate fixture "ca-a" as "pool-partner-a" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "status" should be "success"
    And the JSON response field "name" should be "pool-partner-a"
    And the JSON response field "usage" should be "downstream"
    And the JSON response field "role" should be "client"
    And the JSON response field "isLeaf" should be false
    And the JSON response field "count" should be 1
    And the JSON response should have field "id"
    And the JSON response should have field "subject"
    And the JSON response should have field "notAfter"
    And the JSON response field "warnings" should not exist
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the response status should be 200
    And the listed certificate "pool-partner-a" should have "usage" equal to "downstream"
    And the listed certificate "pool-partner-a" should have "role" equal to "client"
    And the listed certificate "pool-partner-a" should have "referencedByApis" equal to 0
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=upstream"
    Then the certificate list should not contain "pool-partner-a"

  Scenario: A certificate uploaded without usage is backend trust
    When I upload the certificate fixture "ca-a" as "pool-backend-ca"
    Then the response status should be 201
    And the JSON response field "usage" should be "upstream"
    And the JSON response field "role" should not exist
    And the JSON response field "count" should be 1
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=upstream"
    Then the listed certificate "pool-backend-ca" should have "usage" equal to "upstream"
    And the listed certificate "pool-backend-ca" should not have field "referencedByApis"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "pool-backend-ca"
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the certificate list should contain "pool-backend-ca"

  Scenario: A client authority can be marked as a relay for certificates carried in a header
    When I upload the certificate fixture "ca-a" as "pool-edge-lb-ca" with usage "downstream" and role "relay"
    Then the response status should be 201
    And the JSON response field "usage" should be "downstream"
    And the JSON response field "role" should be "relay"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "pool-edge-lb-ca" should have "role" equal to "relay"

  Scenario: An issuing CA uploaded together with its root is one entry identified by the issuing CA
    When I upload the certificate fixtures "ca-a-intermediate,ca-a" as "pool-partner-a-chain" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "count" should be 2
    And the JSON response field "isLeaf" should be false
    And the JSON response field "subject" should be the subject of fixture "ca-a-intermediate"
    And the JSON response field "warnings" should not exist

  Scenario: A leaf certificate is accepted as a one-member authority and flagged
    When I upload the certificate fixture "client-selfsigned" as "pool-acme-selfsigned" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "isLeaf" should be true
    And the JSON response field "warnings[0].code" should be "CLIENT_CA_IS_LEAF"
    And the JSON response field "warnings[0].field" should be "certificate"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "pool-acme-selfsigned" should have "isLeaf" equal to true

  Scenario: A not-yet-valid authority is accepted with a warning
    When I upload the certificate fixture "ca-not-yet-valid" as "pool-future-ca" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "warnings[0].code" should be "CLIENT_CA_NOT_YET_VALID"
    And the JSON response field "warnings[0].field" should be "certificate"

  Scenario: The same certificate may be pooled under a second name
    Given the certificate fixture "ca-b" is pooled as "pool-partner-b" with usage "downstream"
    When I upload the certificate fixture "ca-b" as "pool-partner-b-again" with usage "downstream"
    Then the response status should be 201
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "pool-partner-b"
    And the certificate list should contain "pool-partner-b-again"

  Scenario: A second authority with the same subject DN as a pooled one is accepted
    Given the certificate fixture "ca-a" is pooled as "pool-partner-a" with usage "downstream"
    When I upload the certificate fixture "ca-b-same-dn" as "pool-lookalike-ca" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "subject" should be the subject of fixture "ca-a"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "pool-partner-a"
    And the certificate list should contain "pool-lookalike-ca"

  Scenario: An authority expiring within thirty days is listed with an expiry warning
    Given the certificate fixture "ca-expires-soon" is pooled as "pool-expiring-ca" with usage "downstream"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "pool-expiring-ca" should have a warning with code "CERT_EXPIRES_SOON"
    And the listed certificate "pool-expiring-ca" should have a warning with field "notAfter"

  Scenario: An authority with more than thirty days left is listed without an expiry warning
    Given the certificate fixture "ca-a" is pooled as "pool-partner-a" with usage "downstream"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "pool-partner-a" should have no warnings

  # ==================== REJECTED UPLOADS ====================

  Scenario: An expired certificate is rejected because nothing it signed can validate
    When I upload the certificate fixture "ca-expired" as "pool-expired-ca" with usage "downstream"
    Then the response status should be 400
    And the JSON response field "status" should be "error"
    And the response should list a validation error for field "certificate" containing "the certificate expired on"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "pool-expired-ca"

  Scenario: Two unrelated authorities in one PEM are rejected
    When I upload the certificate fixtures "ca-a,ca-b" as "pool-two-roots" with usage "downstream"
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "this PEM contains more than one unrelated authority; upload each as its own entry"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "pool-two-roots"

  Scenario: A PEM that carries a private key is rejected and nothing is stored
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool-leaky-ca",
        "usage": "downstream",
        "certificate": "{{pem "ca-a"}}\n{{key "ca-a"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "the upload contains a private key; a client-CA entry accepts certificates only"
    And the response body should not contain "PRIVATE KEY"
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the certificate list should not contain "pool-leaky-ca"

  Scenario: A backend trust certificate that carries a private key is rejected and nothing is stored
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool-leaky-backend-ca",
        "usage": "upstream",
        "certificate": "{{pem "backend-ca"}}\n{{key "backend-ca"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "the certificate field takes certificates only; the private key belongs in privateKey"
    And the response body should not contain "PRIVATE KEY"
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the certificate list should not contain "pool-leaky-backend-ca"

  Scenario Outline: A relay upload with a narrowing the gateway does not understand is rejected
    When I upload to the certificates endpoint the body:
      """
      <body>
      """
    Then the response status should be 400
    And the response should list a validation error for field "<field>" with message "<message>"
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the certificate list should not contain "pool-relay-bad"

    Examples:
      | body                                                                                                                | field         | message                                                          |
      | {"name":"pool-relay-bad","usage":"downstream","role":"relay","certificate":"{{pem "edge-lb-ca"}}","match":{"dnsSAN":["lb.example"]}} | match.dnsSAN  | unknown field dnsSAN; match takes uriSANs and dnsSANs            |
      | {"name":"pool-relay-bad","usage":"downstream","role":"relay","certificate":"{{pem "edge-lb-ca"}}","match":{}}                        | match         | match must list uriSANs or dnsSANs, or be omitted                |
      | {"name":"pool-relay-bad","usage":"downstream","role":"relay","certificate":"{{pem "edge-lb-ca"}}","match":{"dnsSANs":"lb.example"}}  | match.dnsSANs | dnsSANs must be a list                                           |
      | {"name":"pool-relay-bad","usage":"downstream","roles":"relay","certificate":"{{pem "edge-lb-ca"}}"}                                  | roles         | unknown field roles                                              |

  Scenario: A value that is not a PEM certificate is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool-garbage",
        "usage": "downstream",
        "certificate": "this is not a certificate"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "the value is not a PEM-encoded certificate"

  Scenario: An unknown usage is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool-bad-usage",
        "usage": "backend",
        "certificate": "{{pem "ca-a"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "usage" with message "usage must be upstream, downstream or identity"

  Scenario: An unknown role is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool-bad-role",
        "usage": "downstream",
        "role": "proxy",
        "certificate": "{{pem "ca-a"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "role" with message "role must be client or relay"

  Scenario: A role given on an upstream certificate is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool-role-on-upstream",
        "usage": "upstream",
        "role": "relay",
        "certificate": "{{pem "ca-a"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "role" with message "role applies only to usage: downstream certificates"

  Scenario: A name with characters outside letters, digits, dot, underscore and hyphen is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool/partner a",
        "usage": "downstream",
        "certificate": "{{pem "ca-a"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "name" with message "name may contain only letters, digits, ., _ and -"

  Scenario: Several problems in one upload are all reported at once
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool/bad name",
        "usage": "upstream",
        "role": "relay",
        "certificate": "{{pem "ca-a"}}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "name"
    And the response should list a validation error for field "role"

  Scenario: A body over the size limit is rejected without stating the limit
    When I upload a certificate body a tenth over the upload limit as "pool-too-big" with usage "downstream"
    Then the response status should be 413
    And the response body should not contain "1048576"
    And the response body should not contain "MiB"
    And the response body should not contain "bytes"

  # ==================== ONE NAME SPACE ====================

  Scenario: A client authority may not reuse the name of an upstream certificate
    Given the certificate fixture "ca-a" is pooled as "pool-shared-name"
    When I upload the certificate fixture "ca-b" as "pool-shared-name" with usage "downstream"
    Then the response status should be 409
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "already exists"

  Scenario: A duplicate client authority name is a conflict
    Given the certificate fixture "ca-a" is pooled as "pool-partner-a" with usage "downstream"
    When I upload the certificate fixture "ca-b" as "pool-partner-a" with usage "downstream"
    Then the response status should be 409
    And the JSON response field "message" should be "a client-CA authority named pool-partner-a already exists"

  # ==================== REMOVING AN AUTHORITY ====================

  Scenario: A client authority that no API references can be removed
    Given the certificate fixture "ca-a" is pooled as "pool-partner-a" with usage "downstream"
    When I delete the certificate named "pool-partner-a"
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "pool-partner-a"

  Scenario: Removing an unknown certificate is not found
    When I send a DELETE request to the "gateway-controller" service at "/certificates/no-such-certificate-id"
    Then the response status should be 404
    And the JSON response field "status" should be "error"

  # ==================== ROLES ====================

  Scenario: A developer can read the pool
    Given the certificate fixture "ca-a" is pooled as "pool-partner-a" with usage "downstream"
    And I authenticate using basic auth as "developer"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the response status should be 200
    And the certificate list should contain "pool-partner-a"

  Scenario: A developer cannot add to the pool
    Given I authenticate using basic auth as "developer"
    When I upload the certificate fixture "ca-a" as "pool-dev-attempt" with usage "downstream"
    Then the response status should be 403
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "pool-dev-attempt"

  Scenario: A developer cannot remove from the pool
    Given the certificate fixture "ca-a" is pooled as "pool-partner-a" with usage "downstream"
    And I authenticate using basic auth as "developer"
    When I delete the certificate named "pool-partner-a"
    Then the response status should be 403
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "pool-partner-a"

  Scenario: An unauthenticated caller cannot add to the pool
    Given I clear all headers
    When I upload the certificate fixture "ca-a" as "pool-anon-attempt" with usage "downstream"
    Then the response status should be 401
