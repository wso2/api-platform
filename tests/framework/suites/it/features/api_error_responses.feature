@api-error-responses
Feature: API Error Responses
  As an API administrator
  I want clear validation and parse error responses
  So that I can correct invalid API payloads

  # Migrated with every assertion untouched. This feature is entirely management-plane: each
  # scenario deploys/updates/deletes a configuration and asserts on the gateway-controller's
  # own validation/parse error response. Nothing here routes a request THROUGH the gateway to
  # the upstream, so the testbench reflector is never involved and no assertion depends on it —
  # the upstream `url` swap from sample-backend to the shared testbench is cosmetic, kept only
  # so the config parses the same way it did before. The validation errors asserted (context,
  # operations, policy schema/lookup, metadata name, version format, JSON parse) are produced
  # by the controller regardless of which host the upstream points at.
  #
  # There are no data-plane requests and no `I wait for N seconds` sleeps, so — unlike the other
  # migrated gateway features — no readiness wait is added: `I deploy`/`I update` return the
  # management response synchronously and the next step reads it directly. The PUT in the parse
  # scenario is a service-relative management call (`/rest-apis/...`), not an absolute URL, so
  # it is left exactly as written.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Create API returns validation errors with field details
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: create-error-response-api
      spec:
        displayName: Create-Error-Response-Api
        version: v1.0
        upstream:
          main:
            url: http://testbench:3000
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].field" should be "spec.context"
    And the JSON response field "errors[0].message" should be "Context is required"
    And the JSON response field "errors[1].field" should be "spec.operations"
    And the JSON response field "errors[1].message" should be "At least one operation is required"

  Scenario: Create API returns policy schema validation errors
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: policy-schema-error-api
      spec:
        displayName: Policy-Schema-Error-Api
        version: v1.0
        context: /policy-schema-error
        upstream:
          main:
            url: http://testbench:3000
        policies:
          - name: respond
            version: v1
            params:
              statusCode: "not-a-number"
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].field" should be "spec.policies[0].params.statusCode"
    And the JSON response field "errors[0].message" should be "Invalid type. Expected: integer, given: string"

  Scenario: Create API returns policy not found errors
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: policy-not-found-error-api
      spec:
        displayName: Policy-Not-Found-Error-Api
        version: v1.0
        context: /policy-not-found-error
        upstream:
          main:
            url: http://testbench:3000
        policies:
          - name: policy-does-not-exist
            version: v1
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].field" should be "spec.policies[0].version"
    And the JSON response field "errors[0].message" should be "policy 'policy-does-not-exist' major version 'v1' not found in loaded policy definitions"
    And the JSON response field "errors[0].message" should be "policy 'policy-does-not-exist' major version 'v1' not found in loaded policy definitions"

  Scenario: Create API returns metadata name validation errors
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata: {}
      spec:
        displayName: Missing-Metadata-Name-Api
        version: v1.0
        context: /missing-metadata-name
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].field" should be "metadata.name"
    And the JSON response field "errors[0].message" should be "Metadata name is required"

  Scenario: Create API returns version format validation errors
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-version-format-api
      spec:
        displayName: Invalid-Version-Format-Api
        version: v1.0.0-beta
        context: /invalid-version-format
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].field" should be "spec.version"
    And the JSON response field "errors[0].message" should be "API version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)"

  Scenario: Update API parse errors include detailed message
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-parse-error-api
      spec:
        displayName: Update-Parse-Error-Api
        version: v1.0
        context: /update-parse-error
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And I set header "Content-Type" to "application/json"
    When I send a "PUT" request to the "gateway-controller" service at "/rest-apis/update-parse-error-api" with body:
      """
      { this is not valid json
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "Failed to parse configuration:"
    And the JSON response field "message" should contain "failed to parse JSON"
    # Cleanup
    When I delete the API "update-parse-error-api"
    Then the response status code should be 200

  Scenario: Update API returns validation errors with field details
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-validation-error-api
      spec:
        displayName: Update-Validation-Error-Api
        version: v1.0
        context: /update-validation-error
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I update the API "update-validation-error-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-validation-error-api
      spec:
        displayName: Update-Validation-Error-Api
        version: v1.0
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].field" should be "spec.context"
    And the JSON response field "errors[0].message" should be "Context is required"
    # Cleanup
    When I delete the API "update-validation-error-api"
    Then the response status code should be 200
