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
