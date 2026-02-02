Feature: Certificate Management
  Test certificate listing, deletion, and reload operations

  Background:
    Given I authenticate using basic auth as "admin"

  # ==================== LIST CERTIFICATES ====================
  
  Scenario: List certificates endpoint works
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  # ==================== UPLOAD CERTIFICATE ERROR CASES ====================
  
  Scenario: Upload certificate with invalid PEM format
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
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

  Scenario: Upload certificate without name
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "certificate": "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate without certificate data
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "cert-no-data"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== UPLOAD CERTIFICATE SUCCESS CASES ====================

  Scenario: Upload valid certificate successfully
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "test-valid-cert",
        "certificate": "-----BEGIN CERTIFICATE-----\nMIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\nBQAwgYExCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQH\nDA1Nb3VudGFpbiBWaWV3MRowGAYDVQQKDBFUZXN0IE9yZ2FuaXphdGlvbjEQMA4G\nA1UECwwHVGVzdGluZzEXMBUGA1UEAwwOc2VjdXJlLWJhY2tlbmQwHhcNMjUxMTI2\nMDYwNzI2WhcNMjYxMTI2MDYwNzI2WjCBgTELMAkGA1UEBhMCVVMxEzARBgNVBAgM\nCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcxGjAYBgNVBAoMEVRl\nc3QgT3JnYW5pemF0aW9uMRAwDgYDVQQLDAdUZXN0aW5nMRcwFQYDVQQDDA5zZWN1\ncmUtYmFja2VuZDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAK4ffloD\ngwHOZGhV4nJuznQS6P95TdTuQ3gXE2/TXxz9mUivSkr2xYd8QCK7+1sdxskKCdSM\nUYftW2VB9xhMeJUJOg7vWCTNCY30ffMxV/oQSQNZGGjN9hO2qvQKScIghODr/emZ\nf4dVgsoisKXwG1+WXvkF57zpeN62pi1H0rKm03aRNJBsNhmuU7ELiHVlt6/yNOVG\n4DG9mC0ndp0oI/fMfqvX/8dE7wq+IJTEZvXFm/Hb7+0aw9FAKmZLSNQYQZPZYxVH\nu0ag57I5nq0bmHGXKcMtLNFjU1bu0G4tIcvbU6JoSvqNtZFsPMMiEKGKl6sEoAhr\ndsD9P/4/yyLwsmMzlESWouf+OR1W3rCXeh55QBKIuRQU2LaM79UFaTjtD+J7Q9Ww\nHXlZ2m8bYZoQT1hicqTuvCGrk0eTQf4Q6KT9WPo9WHxC4Hn7NO3cBOOEiqrt/B0v\nJwt9sdxodMWoKWyCxbWYTeTzPXxGcoR9fZqZKz0fIDRK5g5qTEcBrseNchqS46XI\np+KcUZXZ1+PHUr7ItFPif0v4q60GuWgpC3lE8nmj7TknWKSRRbyPZf6BFTpqlLUl\nWf98InpeUD+UKeifZTaucrqvB6QA3G0tbFg/AdTmA3QM52gxbAEkBhRUxkVV0oW2\nw0XnPO0AtKFzukwL+WOJQPove1qcrRG92gPXAgMBAAGjgZYwgZMwHQYDVR0OBBYE\nFG0uv/Hg+71KVmKKXoTaLWniAxFZMB8GA1UdIwQYMBaAFG0uv/Hg+71KVmKKXoTa\nLWniAxFZMA8GA1UdEwEB/wQFMAMBAf8wQAYDVR0RBDkwN4IOc2VjdXJlLWJhY2tl\nbmSCFHNlY3VyZS1iYWNrZW5kLmxvY2Fsgglsb2NhbGhvc3SHBH8AAAEwDQYJKoZI\nhvcNAQELBQADggIBAABtg4O9JpWW81ltidIVctocPkTXn+2s6YZfI3mZvlKoFZDA\nX8L0p7oshG+g9OkbYfTzb2yZ6+BIPuMSUMvgi/QDRM8UeNXPt+1YyWLwXxsRjbfL\nCgRtN+5HVBIemsAV3N/sN8FG65eNaIhjvNR4wEa/EeyJyNNWL3VD8uVSMAaMbjZQ\nJkYeHpjnAmZiQqWCtNGsv3srWwsgHiZFSidpDNPU3KeDnCzJs5VK0CPq7/Eb9BT9\nRF6aq/BZE0ld0gnTrnisYTlyW53XSPAJTdWLE+stMUMJafoXYl7bEwT/NgBbScGu\n/rMZiayHbSmgIb5ikY/YycPWWp4alN6Ckb8+Vk9ied0p5p4G2VlUbPVApmmnWJwW\nnnUWil3xKifnGkkEbgdqzNMIuectfCYNpcK3519n5vXkWXWfungdbRXpi83VKf0a\ne7xDP2iV8c/otubk2BpU1q+9JbQuYIS4D0NCl2flPdHgE6VwKXBpqYmAbifUx38R\nPHFFVagSydGgirQ6ZZVRtgJzUmI7BR86sBjQwaevk8+lkl5w/xPqdQu/9ryaL9Zb\nF6WPkiF8LceoSNlinm4rrCiRTRAKbNOrsXSvE1SMIi4JVfHSVp0K+oOpIeJfgZ/b\nJIlH5QcT1SnwdkIHtyScYWnBIpZ4ZZc8kmeKPC0WqHosCVtSeOnGrKc4rwLC\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "name" should be "test-valid-cert"
    And the JSON response should have field "id"
    And the JSON response should have field "subject"
    And the JSON response should have field "issuer"
    And the JSON response should have field "notAfter"
    And the JSON response field "count" should be 1

  Scenario: Upload certificate and verify it appears in list
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "list-test-cert",
        "certificate": "-----BEGIN CERTIFICATE-----\nMIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\nBQAwgYExCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQH\nDA1Nb3VudGFpbiBWaWV3MRowGAYDVQQKDBFUZXN0IE9yZ2FuaXphdGlvbjEQMA4G\nA1UECwwHVGVzdGluZzEXMBUGA1UEAwwOc2VjdXJlLWJhY2tlbmQwHhcNMjUxMTI2\nMDYwNzI2WhcNMjYxMTI2MDYwNzI2WjCBgTELMAkGA1UEBhMCVVMxEzARBgNVBAgM\nCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcxGjAYBgNVBAoMEVRl\nc3QgT3JnYW5pemF0aW9uMRAwDgYDVQQLDAdUZXN0aW5nMRcwFQYDVQQDDA5zZWN1\ncmUtYmFja2VuZDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAK4ffloD\ngwHOZGhV4nJuznQS6P95TdTuQ3gXE2/TXxz9mUivSkr2xYd8QCK7+1sdxskKCdSM\nUYftW2VB9xhMeJUJOg7vWCTNCY30ffMxV/oQSQNZGGjN9hO2qvQKScIghODr/emZ\nf4dVgsoisKXwG1+WXvkF57zpeN62pi1H0rKm03aRNJBsNhmuU7ELiHVlt6/yNOVG\n4DG9mC0ndp0oI/fMfqvX/8dE7wq+IJTEZvXFm/Hb7+0aw9FAKmZLSNQYQZPZYxVH\nu0ag57I5nq0bmHGXKcMtLNFjU1bu0G4tIcvbU6JoSvqNtZFsPMMiEKGKl6sEoAhr\ndsD9P/4/yyLwsmMzlESWouf+OR1W3rCXeh55QBKIuRQU2LaM79UFaTjtD+J7Q9Ww\nHXlZ2m8bYZoQT1hicqTuvCGrk0eTQf4Q6KT9WPo9WHxC4Hn7NO3cBOOEiqrt/B0v\nJwt9sdxodMWoKWyCxbWYTeTzPXxGcoR9fZqZKz0fIDRK5g5qTEcBrseNchqS46XI\np+KcUZXZ1+PHUr7ItFPif0v4q60GuWgpC3lE8nmj7TknWKSRRbyPZf6BFTpqlLUl\nWf98InpeUD+UKeifZTaucrqvB6QA3G0tbFg/AdTmA3QM52gxbAEkBhRUxkVV0oW2\nw0XnPO0AtKFzukwL+WOJQPove1qcrRG92gPXAgMBAAGjgZYwgZMwHQYDVR0OBBYE\nFG0uv/Hg+71KVmKKXoTaLWniAxFZMB8GA1UdIwQYMBaAFG0uv/Hg+71KVmKKXoTa\nLWniAxFZMA8GA1UdEwEB/wQFMAMBAf8wQAYDVR0RBDkwN4IOc2VjdXJlLWJhY2tl\nbmSCFHNlY3VyZS1iYWNrZW5kLmxvY2Fsgglsb2NhbGhvc3SHBH8AAAEwDQYJKoZI\nhvcNAQELBQADggIBAABtg4O9JpWW81ltidIVctocPkTXn+2s6YZfI3mZvlKoFZDA\nX8L0p7oshG+g9OkbYfTzb2yZ6+BIPuMSUMvgi/QDRM8UeNXPt+1YyWLwXxsRjbfL\nCgRtN+5HVBIemsAV3N/sN8FG65eNaIhjvNR4wEa/EeyJyNNWL3VD8uVSMAaMbjZQ\nJkYeHpjnAmZiQqWCtNGsv3srWwsgHiZFSidpDNPU3KeDnCzJs5VK0CPq7/Eb9BT9\nRF6aq/BZE0ld0gnTrnisYTlyW53XSPAJTdWLE+stMUMJafoXYl7bEwT/NgBbScGu\n/rMZiayHbSmgIb5ikY/YycPWWp4alN6Ckb8+Vk9ied0p5p4G2VlUbPVApmmnWJwW\nnnUWil3xKifnGkkEbgdqzNMIuectfCYNpcK3519n5vXkWXWfungdbRXpi83VKf0a\ne7xDP2iV8c/otubk2BpU1q+9JbQuYIS4D0NCl2flPdHgE6VwKXBpqYmAbifUx38R\nPHFFVagSydGgirQ6ZZVRtgJzUmI7BR86sBjQwaevk8+lkl5w/xPqdQu/9ryaL9Zb\nF6WPkiF8LceoSNlinm4rrCiRTRAKbNOrsXSvE1SMIi4JVfHSVp0K+oOpIeJfgZ/b\nJIlH5QcT1SnwdkIHtyScYWnBIpZ4ZZc8kmeKPC0WqHosCVtSeOnGrKc4rwLC\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response body should contain "list-test-cert"

  # ==================== DELETE CERTIFICATE ====================
  
  Scenario: Delete non-existent certificate returns 404
    When I send a DELETE request to the "gateway-controller" service at "/certificates/non-existent-cert-id"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== RELOAD CERTIFICATES ====================
  
  Scenario: Reload certificates
    When I send a POST request to the "gateway-controller" service at "/certificates/reload" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should contain "reload"

  # ==================== ADDITIONAL CERTIFICATE EDGE CASES ====================

  Scenario: Upload certificate with empty body returns error
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {}
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate with malformed PEM returns error
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "malformed-cert",
        "certificate": "-----BEGIN CERTIFICATE-----\nINVALIDBASE64DATA!!!\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Upload certificate with missing BEGIN marker returns error
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "no-begin-cert",
        "certificate": "MIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List certificates returns correct structure
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "certificates"
    And the JSON response should have field "totalCount"

  Scenario: Upload and then delete certificate successfully
    When I send a POST request to the "gateway-controller" service at "/certificates" with body:
      """
      {
        "name": "delete-test-cert",
        "certificate": "-----BEGIN CERTIFICATE-----\nMIIGKTCCBBGgAwIBAgIUU04tbcQ4yTsPtRFlOtBPJVUILUUwDQYJKoZIhvcNAQEL\nBQAwgYExCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQH\nDA1Nb3VudGFpbiBWaWV3MRowGAYDVQQKDBFUZXN0IE9yZ2FuaXphdGlvbjEQMA4G\nA1UECwwHVGVzdGluZzEXMBUGA1UEAwwOc2VjdXJlLWJhY2tlbmQwHhcNMjUxMTI2\nMDYwNzI2WhcNMjYxMTI2MDYwNzI2WjCBgTELMAkGA1UEBhMCVVMxEzARBgNVBAgM\nCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcxGjAYBgNVBAoMEVRl\nc3QgT3JnYW5pemF0aW9uMRAwDgYDVQQLDAdUZXN0aW5nMRcwFQYDVQQDDA5zZWN1\ncmUtYmFja2VuZDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAK4ffloD\ngwHOZGhV4nJuznQS6P95TdTuQ3gXE2/TXxz9mUivSkr2xYd8QCK7+1sdxskKCdSM\nUYftW2VB9xhMeJUJOg7vWCTNCY30ffMxV/oQSQNZGGjN9hO2qvQKScIghODr/emZ\nf4dVgsoisKXwG1+WXvkF57zpeN62pi1H0rKm03aRNJBsNhmuU7ELiHVlt6/yNOVG\n4DG9mC0ndp0oI/fMfqvX/8dE7wq+IJTEZvXFm/Hb7+0aw9FAKmZLSNQYQZPZYxVH\nu0ag57I5nq0bmHGXKcMtLNFjU1bu0G4tIcvbU6JoSvqNtZFsPMMiEKGKl6sEoAhr\ndsD9P/4/yyLwsmMzlESWouf+OR1W3rCXeh55QBKIuRQU2LaM79UFaTjtD+J7Q9Ww\nHXlZ2m8bYZoQT1hicqTuvCGrk0eTQf4Q6KT9WPo9WHxC4Hn7NO3cBOOEiqrt/B0v\nJwt9sdxodMWoKWyCxbWYTeTzPXxGcoR9fZqZKz0fIDRK5g5qTEcBrseNchqS46XI\np+KcUZXZ1+PHUr7ItFPif0v4q60GuWgpC3lE8nmj7TknWKSRRbyPZf6BFTpqlLUl\nWf98InpeUD+UKeifZTaucrqvB6QA3G0tbFg/AdTmA3QM52gxbAEkBhRUxkVV0oW2\nw0XnPO0AtKFzukwL+WOJQPove1qcrRG92gPXAgMBAAGjgZYwgZMwHQYDVR0OBBYE\nFG0uv/Hg+71KVmKKXoTaLWniAxFZMB8GA1UdIwQYMBaAFG0uv/Hg+71KVmKKXoTa\nLWniAxFZMA8GA1UdEwEB/wQFMAMBAf8wQAYDVR0RBDkwN4IOc2VjdXJlLWJhY2tl\nbmSCFHNlY3VyZS1iYWNrZW5kLmxvY2Fsgglsb2NhbGhvc3SHBH8AAAEwDQYJKoZI\nhvcNAQELBQADggIBAABtg4O9JpWW81ltidIVctocPkTXn+2s6YZfI3mZvlKoFZDA\nX8L0p7oshG+g9OkbYfTzb2yZ6+BIPuMSUMvgi/QDRM8UeNXPt+1YyWLwXxsRjbfL\nCgRtN+5HVBIemsAV3N/sN8FG65eNaIhjvNR4wEa/EeyJyNNWL3VD8uVSMAaMbjZQ\nJkYeHpjnAmZiQqWCtNGsv3srWwsgHiZFSidpDNPU3KeDnCzJs5VK0CPq7/Eb9BT9\nRF6aq/BZE0ld0gnTrnisYTlyW53XSPAJTdWLE+stMUMJafoXYl7bEwT/NgBbScGu\n/rMZiayHbSmgIb5ikY/YycPWWp4alN6Ckb8+Vk9ied0p5p4G2VlUbPVApmmnWJwW\nnnUWil3xKifnGkkEbgdqzNMIuectfCYNpcK3519n5vXkWXWfungdbRXpi83VKf0a\ne7xDP2iV8c/otubk2BpU1q+9JbQuYIS4D0NCl2flPdHgE6VwKXBpqYmAbifUx38R\nPHFFVagSydGgirQ6ZZVRtgJzUmI7BR86sBjQwaevk8+lkl5w/xPqdQu/9ryaL9Zb\nF6WPkiF8LceoSNlinm4rrCiRTRAKbNOrsXSvE1SMIi4JVfHSVp0K+oOpIeJfgZ/b\nJIlH5QcT1SnwdkIHtyScYWnBIpZ4ZZc8kmeKPC0WqHosCVtSeOnGrKc4rwLC\n-----END CERTIFICATE-----"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response should have field "id"
    # Store the ID and delete
    When I send a GET request to the "gateway-controller" service at "/certificates"
    Then the response status should be 200
    And the response body should contain "delete-test-cert"
