@secrets
Feature: Secret Management Operations
  As an API administrator
  I want to manage secrets for APIs and providers
  So that I can securely store sensitive configuration data

  # Migration notes — what changed from the legacy suite and why:
  #
  # The bespoke secret steps (I create/get/update/delete a secret) don't exist in
  # this framework. They were only thin wrappers over the controller's /secrets
  # management API, so every one is expressed here via the generic service step —
  # exactly as template_functions.feature provisions its secrets. Secret resolution
  # works because the framework mounts the AES-GCM key the controller needs.
  #
  # The "create/update a secret named X with value Y" convenience steps become full
  # JSON bodies (displayName=name, description "Auto-generated secret"), matching
  # byte-for-byte what those legacy steps constructed.
  #
  # "the response status should be N" is not a step here; every one becomes
  # "the response status code should be N", which is the same assertion.
  #
  # Upstreams are re-addressed at the testbench reflector: the legacy hosts
  # (echo-backend-multi-arch:8080, mock-openapi:4010) don't exist in this harness.
  # Their appended paths (/anything, /openai/v1) are LITERAL, not under test, so
  # they are dropped. The POST-body readiness "wait 3 seconds" becomes a
  # "route programmed" wait, since /chat/completions doesn't answer 200 to a probe.
  # The mid-scenario re-auth before cleanup is dropped (the Background auth holds).

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== CREATE SECRET - SUCCESS CASES ====================

  Scenario: Create a new secret successfully
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "test-secret-1"
        },
        "spec": {
          "displayName": "Test Secret 1",
          "description": "A test secret for validation",
          "value": "my-secret-value-123"
        }
      }
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "test-secret-1"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/test-secret-1"
    Then the response status code should be 200

  Scenario: Create a secret with simple name and value
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "simple-secret"
        },
        "spec": {
          "displayName": "simple-secret",
          "description": "Auto-generated secret",
          "value": "simple-value-123"
        }
      }
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "simple-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/simple-secret"
    Then the response status code should be 200

  Scenario: Create secret with special characters in value
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "special-secret"
        },
        "spec": {
          "displayName": "Special Secret",
          "description": "Secret with special characters",
          "value": "!@#$%^&*()_+-=[]{}|;':\",./<>?"
        }
      }
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "special-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/special-secret"
    Then the response status code should be 200

  Scenario: Create secret with long value
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "long-secret"
        },
        "spec": {
          "displayName": "Long Secret",
          "description": "Secret with a very long value",
          "value": "this-is-a-very-long-secret-value-with-many-characters-to-test-that-the-system-can-handle-secrets-of-reasonable-length"
        }
      }
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "long-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/long-secret"
    Then the response status code should be 200

  # ==================== CREATE SECRET - ERROR CASES ====================

  Scenario: Create secret without name returns error
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "spec": {
          "displayName": "No Name Secret",
          "description": "Secret without a name",
          "value": "my-secret-value"
        }
      }
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create secret without value returns error
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "no-value-secret"
        },
        "spec": {
          "displayName": "No Value Secret",
          "description": "Secret without a value"
        }
      }
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create duplicate secret returns conflict error
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "duplicate-secret"
        },
        "spec": {
          "displayName": "Duplicate Secret",
          "description": "Original secret",
          "value": "original-value"
        }
      }
      """
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "duplicate-secret"
        },
        "spec": {
          "displayName": "Duplicate Secret",
          "description": "Duplicate secret",
          "value": "duplicate-value"
        }
      }
      """
    Then the response status code should be 409
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/duplicate-secret"
    Then the response status code should be 200

  # ==================== GET SECRET - SUCCESS CASES ====================

  Scenario: Get secret by name returns secret details
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "get-test-secret"
        },
        "spec": {
          "displayName": "Get Test Secret",
          "description": "Secret for get testing",
          "value": "retrievable-secret-value"
        }
      }
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/secrets/get-test-secret"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "kind" should be "Secret"
    And the JSON response field "metadata.name" should be "get-test-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/get-test-secret"
    Then the response status code should be 200

  Scenario: Get secret list contains created secret
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "list-test-secret"
        },
        "spec": {
          "displayName": "List Test Secret",
          "description": "Secret for list testing",
          "value": "listable-secret-value"
        }
      }
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/secrets"
    Then the response status code should be 200
    And the response should be valid JSON
    # Order-independent: the element index in a shared listing is not stable across runs.
    And the response body should contain "list-test-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/list-test-secret"
    Then the response status code should be 200

  # ==================== GET SECRET - ERROR CASES ====================

  Scenario: Get non-existent secret returns 404
    When I send a "GET" request to the "gateway-controller" service at "/secrets/non-existent-secret-12345"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== UPDATE SECRET - SUCCESS CASES ====================

  Scenario: Update secret value successfully
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "update-test-secret"
        },
        "spec": {
          "displayName": "Update Test Secret",
          "description": "Original secret description",
          "value": "original-value"
        }
      }
      """
    Then the response status code should be 201
    When I send a "PUT" request to the "gateway-controller" service at "/secrets/update-test-secret" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "update-test-secret"
        },
        "spec": {
          "displayName": "Updated Secret Name",
          "description": "Updated secret description",
          "value": "updated-value-123"
        }
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "update-test-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/update-test-secret"
    Then the response status code should be 200

  Scenario: Update secret with simple value
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "simple-update-secret"
        },
        "spec": {
          "displayName": "simple-update-secret",
          "description": "Auto-generated secret",
          "value": "original-simple-value"
        }
      }
      """
    Then the response status code should be 201
    When I send a "PUT" request to the "gateway-controller" service at "/secrets/simple-update-secret" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "simple-update-secret"
        },
        "spec": {
          "displayName": "simple-update-secret",
          "description": "Auto-generated secret",
          "value": "updated-simple-value"
        }
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "simple-update-secret"
    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/simple-update-secret"
    Then the response status code should be 200

  # ==================== UPDATE SECRET - ERROR CASES ====================

  Scenario: Update non-existent secret returns 404
    When I send a "PUT" request to the "gateway-controller" service at "/secrets/non-existent-secret-12345" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "non-existent-secret-12345"
        },
        "spec": {
          "displayName": "Non-existent Secret",
          "description": "This secret does not exist",
          "value": "new-value"
        }
      }
      """
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== DELETE SECRET - SUCCESS CASES ====================

  Scenario: Delete secret successfully
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "delete-test-secret"
        },
        "spec": {
          "displayName": "Delete Test Secret",
          "description": "Secret for deletion testing",
          "value": "deletable-secret-value"
        }
      }
      """
    Then the response status code should be 201
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/delete-test-secret"
    Then the response status code should be 200
    # Verify deletion
    When I send a "GET" request to the "gateway-controller" service at "/secrets/delete-test-secret"
    Then the response status code should be 404

  Scenario: Delete secret is idempotent - deleting non-existent secret returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/non-existent-secret-99999"
    Then the response status code should be 404

  # ==================== SECRET RESOLUTION AT RUNTIME ====================

  Scenario: Invoke an LLM Provider that uses a secret for configuration
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "upstream-secret"
        },
        "spec": {
          "displayName": "upstream-secret",
          "description": "Auto-generated secret",
          "value": "ssk-test-auth-key-12345"
        }
      }
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "upstream-secret"

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: invoke-auth-provider-secret
      spec:
        displayName: Invoke Auth Provider Secret
        version: v1.0
        template: openai
        context: /llm-auth-secret
        upstream:
          url: http://testbench:3000
          auth:
            type: api-key
            header: Authorization
            value: 'Bearer {{ secret "upstream-secret" }}'
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I send a "POST" request to "/llm-auth-secret/chat/completions" until status 200

    # Verify the secret value is correctly injected into the Authorization header sent upstream
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/llm-auth-secret/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Test auth"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response should contain echoed header "Authorization" with value "Bearer ssk-test-auth-key-12345"

    # Cleanup
    When I delete the LLM provider "invoke-auth-provider-secret"
    Then the response status code should be 200
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/upstream-secret"
    Then the response status code should be 200

  Scenario: Invoke an LLM Provider where the secret value contains JSON special characters (backslash and quote)
    # Secret value contains \ and " which must be JSON-escaped as \\ and \" when submitted
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "upstream-secret-special"
        },
        "spec": {
          "displayName": "Special Chars Secret",
          "description": "Secret whose value contains backslash and quote characters",
          "value": "ssk-test\\auth-\"key\""
        }
      }
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "upstream-secret-special"

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: invoke-auth-provider-special-secret
      spec:
        displayName: Invoke Auth Provider Special Secret
        version: v1.0
        template: openai
        context: /llm-auth-special-secret
        upstream:
          url: http://testbench:3000
          auth:
            type: api-key
            header: Authorization
            value: 'Bearer {{ secret "upstream-secret-special" }}'
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I send a "POST" request to "/llm-auth-special-secret/chat/completions" until status 200

    # Verify the secret value (with special chars) is correctly injected into the Authorization header sent upstream.
    # Use the docstring variant so the expected value can contain embedded double-quote characters.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/llm-auth-special-secret/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Test special char auth"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response should contain echoed header "Authorization" with exact value:
      """
      Bearer ssk-test\auth-"key"
      """

    # Cleanup
    When I delete the LLM provider "invoke-auth-provider-special-secret"
    Then the response status code should be 200
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/upstream-secret-special"
    Then the response status code should be 200

  Scenario: Create LLM Provider with a secret that does not exist
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: invalid-secret-llm-provider
      spec:
        displayName: Invalid Secret LLM Provider
        version: v1.0
        template: openai
        context: /invalid-secret-test
        upstream:
          url: http://testbench:3000
          auth:
            type: api-key
            header: Authorization
            value: 'Bearer {{ secret "non-existent-secret-abcde" }}'
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 400
