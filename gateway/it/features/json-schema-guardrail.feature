Feature: JSON Schema Guardrail Policy
  Test the json-schema-guardrail policy which validates request and response payloads
  against JSON Schema specifications, supports JSONPath extraction, inverted logic,
  and detailed error reporting.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # BASIC REQUEST VALIDATION
  # ====================================================================

  Scenario: Valid request passes schema validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-valid-request
      spec:
        displayName: JSG Valid Request
        version: v1.0.0
        context: /jsg-valid-req/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-valid-req/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-valid-req/v1.0.0/check" with body:
      """
      {"name": "John Doe", "age": 30}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-valid-request"
    Then the response should be successful

  Scenario: Invalid request fails schema validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-invalid-request
      spec:
        displayName: JSG Invalid Request
        version: v1.0.0
        context: /jsg-invalid-req/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-invalid-req/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-invalid-req/v1.0.0/check" with body:
      """
      {"name": "John Doe"}
      """
    Then the response status code should be 422
    And the response body should contain "JSON_SCHEMA_GUARDRAIL"
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-invalid-request"
    Then the response should be successful

  Scenario: Missing required field fails validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-missing-field
      spec:
        displayName: JSG Missing Field
        version: v1.0.0
        context: /jsg-missing-field/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"username":{"type":"string"},"email":{"type":"string"}},"required":["username","email"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-missing-field/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-missing-field/v1.0.0/check" with body:
      """
      {"username": "johndoe"}
      """
    Then the response status code should be 422
    And the response body should contain "GUARDRAIL_INTERVENED"
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-missing-field"
    Then the response should be successful

  Scenario: Wrong type fails validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-wrong-type
      spec:
        displayName: JSG Wrong Type
        version: v1.0.0
        context: /jsg-wrong-type/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-wrong-type/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-wrong-type/v1.0.0/check" with body:
      """
      {"name": "John", "age": "thirty"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-wrong-type"
    Then the response should be successful

  # ====================================================================
  # BASIC RESPONSE VALIDATION
  # ====================================================================

  Scenario: Valid response passes schema validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-valid-response
      spec:
        displayName: JSG Valid Response
        version: v1.0.0
        context: /jsg-valid-resp/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: GET
            path: /echo
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  response:
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"method":{"type":"string"},"path":{"type":"string"}},"required":["method","path"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-valid-resp/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/jsg-valid-resp/v1.0.0/echo"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-valid-response"
    Then the response should be successful

  # ====================================================================
  # BOTH REQUEST AND RESPONSE VALIDATION
  # ====================================================================

  Scenario: Validate both request and response
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-both-validation
      spec:
        displayName: JSG Both Validation
        version: v1.0.0
        context: /jsg-both-valid/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}'
                  response:
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"method":{"type":"string"}},"required":["method"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-both-valid/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-both-valid/v1.0.0/check" with body:
      """
      {"input": "test data"}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-both-validation"
    Then the response should be successful

  # ====================================================================
  # JSONPATH EXTRACTION
  # ====================================================================

  Scenario: Validate specific field with JSONPath
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-jsonpath
      spec:
        displayName: JSG JSONPath
        version: v1.0.0
        context: /jsg-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer","minimum":18}},"required":["name","age"]}'
                    jsonPath: $.user
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-jsonpath/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-jsonpath/v1.0.0/check" with body:
      """
      {"user": {"name": "Alice", "age": 25}, "metadata": "ignored"}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-jsonpath"
    Then the response should be successful

  Scenario: JSONPath extraction with invalid data
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-jsonpath-invalid
      spec:
        displayName: JSG JSONPath Invalid
        version: v1.0.0
        context: /jsg-jsonpath-inv/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    schema: '{"type":"object","properties":{"age":{"type":"integer","minimum":18}},"required":["age"]}'
                    jsonPath: $.user
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-jsonpath-inv/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-jsonpath-inv/v1.0.0/check" with body:
      """
      {"user": {"age": 15}, "other": "data"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-jsonpath-invalid"
    Then the response should be successful

  Scenario: Validate nested object with JSONPath
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-nested-jsonpath
      spec:
        displayName: JSG Nested JSONPath
        version: v1.0.0
        context: /jsg-nested-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    schema: '{"type":"object","properties":{"street":{"type":"string"},"city":{"type":"string"},"zipCode":{"type":"string"}},"required":["street","city","zipCode"]}'
                    jsonPath: $.order.shippingAddress
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-nested-jsonpath/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-nested-jsonpath/v1.0.0/check" with body:
      """
      {"order": {"shippingAddress": {"street": "123 Main St", "city": "Boston", "zipCode": "02101"}}}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-nested-jsonpath"
    Then the response should be successful

  # ====================================================================
  # INVERT LOGIC
  # ====================================================================

  Scenario: Invert logic passes when schema validation fails
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-invert-pass
      spec:
        displayName: JSG Invert Pass
        version: v1.0.0
        context: /jsg-invert-pass/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"dangerousCommand":{"type":"string","pattern":"^(rm|delete|drop).*"}},"required":["dangerousCommand"]}'
                    invert: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-invert-pass/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-invert-pass/v1.0.0/check" with body:
      """
      {"safeCommand": "list files"}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-invert-pass"
    Then the response should be successful

  Scenario: Invert logic blocks when schema validation succeeds
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-invert-block
      spec:
        displayName: JSG Invert Block
        version: v1.0.0
        context: /jsg-invert-block/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"command":{"type":"string","pattern":"^(rm|delete|drop).*"}},"required":["command"]}'
                    invert: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-invert-block/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-invert-block/v1.0.0/check" with body:
      """
      {"command": "rm -rf /"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-invert-block"
    Then the response should be successful

  Scenario: Block requests matching malicious pattern with invert
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-block-malicious
      spec:
        displayName: JSG Block Malicious
        version: v1.0.0
        context: /jsg-block-mal/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"query":{"type":"string","pattern":".*DROP TABLE.*"}}}'
                    invert: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-block-mal/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-block-mal/v1.0.0/check" with body:
      """
      {"query": "SELECT * FROM users"}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-block-malicious"
    Then the response should be successful

  # ====================================================================
  # SHOW ASSESSMENT
  # ====================================================================

  Scenario: Show detailed assessment on validation failure
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-show-assessment
      spec:
        displayName: JSG Show Assessment
        version: v1.0.0
        context: /jsg-show-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"name":{"type":"string","minLength":3},"age":{"type":"integer","minimum":18}},"required":["name","age"]}'
                    showAssessment: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-show-assessment/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-show-assessment/v1.0.0/check" with body:
      """
      {"name": "Jo", "age": 15}
      """
    Then the response status code should be 422
    And the response body should contain "assessments"
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-show-assessment"
    Then the response should be successful

  Scenario: Hide assessment details when showAssessment is false
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-hide-assessment
      spec:
        displayName: JSG Hide Assessment
        version: v1.0.0
        context: /jsg-hide-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}'
                    showAssessment: false
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-hide-assessment/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-hide-assessment/v1.0.0/check" with body:
      """
      {"age": 25}
      """
    Then the response status code should be 422
    And the response body should contain "GUARDRAIL_INTERVENED"
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-hide-assessment"
    Then the response should be successful

  # ====================================================================
  # SCHEMA CONSTRAINTS
  # ====================================================================

  Scenario: Validate string length constraints
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-string-length
      spec:
        displayName: JSG String Length
        version: v1.0.0
        context: /jsg-string-length/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"username":{"type":"string","minLength":3,"maxLength":20}},"required":["username"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-string-length/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-string-length/v1.0.0/check" with body:
      """
      {"username": "ab"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-string-length"
    Then the response should be successful

  Scenario: Validate numeric range constraints
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-numeric-range
      spec:
        displayName: JSG Numeric Range
        version: v1.0.0
        context: /jsg-numeric-range/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"price":{"type":"number","minimum":0,"maximum":10000}},"required":["price"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-numeric-range/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-numeric-range/v1.0.0/check" with body:
      """
      {"price": -5}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-numeric-range"
    Then the response should be successful

  Scenario: Validate array constraints
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-array-constraints
      spec:
        displayName: JSG Array Constraints
        version: v1.0.0
        context: /jsg-array-const/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"tags":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":5}},"required":["tags"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-array-const/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-array-const/v1.0.0/check" with body:
      """
      {"tags": []}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-array-constraints"
    Then the response should be successful

  Scenario: Validate enum constraints
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-enum-constraints
      spec:
        displayName: JSG Enum Constraints
        version: v1.0.0
        context: /jsg-enum-const/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"status":{"type":"string","enum":["pending","processing","completed","cancelled"]}},"required":["status"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-enum-const/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-enum-const/v1.0.0/check" with body:
      """
      {"status": "invalid-status"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-enum-constraints"
    Then the response should be successful

  Scenario: Validate pattern constraints
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-pattern-constraints
      spec:
        displayName: JSG Pattern Constraints
        version: v1.0.0
        context: /jsg-pattern-const/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"email":{"type":"string","pattern":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"}},"required":["email"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-pattern-const/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-pattern-const/v1.0.0/check" with body:
      """
      {"email": "invalid-email"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-pattern-constraints"
    Then the response should be successful

  # ====================================================================
  # COMPLEX SCHEMAS
  # ====================================================================

  Scenario: Validate nested object schema
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-nested-object
      spec:
        displayName: JSG Nested Object
        version: v1.0.0
        context: /jsg-nested-obj/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"name":{"type":"string"},"address":{"type":"object","properties":{"street":{"type":"string"},"city":{"type":"string"}},"required":["street","city"]}},"required":["name","address"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-nested-obj/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-nested-obj/v1.0.0/check" with body:
      """
      {"name": "John", "address": {"street": "Main St"}}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-nested-object"
    Then the response should be successful

  Scenario: Validate array of objects
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-array-objects
      spec:
        displayName: JSG Array Objects
        version: v1.0.0
        context: /jsg-array-obj/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"productId":{"type":"string"},"quantity":{"type":"integer","minimum":1}},"required":["productId","quantity"]}}},"required":["items"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-array-obj/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-array-obj/v1.0.0/check" with body:
      """
      {"items": [{"productId": "123", "quantity": 2}, {"productId": "456", "quantity": 0}]}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-array-objects"
    Then the response should be successful

  # ====================================================================
  # EDGE CASES
  # ====================================================================

  Scenario: Handle empty request body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-empty-body
      spec:
        displayName: JSG Empty Body
        version: v1.0.0
        context: /jsg-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"data":{"type":"string"}},"required":["data"]}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-empty-body/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-empty-body/v1.0.0/check" with body:
      """
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-empty-body"
    Then the response should be successful

  Scenario: Handle invalid JSON
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-invalid-json
      spec:
        displayName: JSG Invalid JSON
        version: v1.0.0
        context: /jsg-invalid-json/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object"}'
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-invalid-json/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-invalid-json/v1.0.0/check" with body:
      """
      not valid json {
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-invalid-json"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-invalid-jsonpath
      spec:
        displayName: JSG Invalid JSONPath
        version: v1.0.0
        context: /jsg-invalid-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    schema: '{"type":"object"}'
                    jsonPath: $.nonexistent.field
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-invalid-jsonpath/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-invalid-jsonpath/v1.0.0/check" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 422
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-invalid-jsonpath"
    Then the response should be successful

  # ====================================================================
  # REAL-WORLD SCENARIOS
  # ====================================================================

  Scenario: User registration with comprehensive validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-registration
      spec:
        displayName: JSG Registration
        version: v1.0.0
        context: /jsg-registration/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"username":{"type":"string","minLength":3,"maxLength":20,"pattern":"^[a-zA-Z0-9_]+$"},"email":{"type":"string","pattern":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"},"password":{"type":"string","minLength":8},"age":{"type":"integer","minimum":13},"termsAccepted":{"type":"boolean","enum":[true]}},"required":["username","email","password","age","termsAccepted"]}'
                    showAssessment: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-registration/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-registration/v1.0.0/check" with body:
      """
      {"username": "john_doe", "email": "john@example.com", "password": "SecurePass123", "age": 25, "termsAccepted": true}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-registration"
    Then the response should be successful

  Scenario: Block SQL injection patterns
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-sql-injection
      spec:
        displayName: JSG SQL Injection
        version: v1.0.0
        context: /jsg-sql-inject/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"query":{"type":"string","pattern":".*((DROP|DELETE|INSERT|UPDATE|SELECT).*(TABLE|FROM|WHERE)).*"}}}'
                    invert: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-sql-inject/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-sql-inject/v1.0.0/check" with body:
      """
      {"query": "normal search term"}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-sql-injection"
    Then the response should be successful

  Scenario: E-commerce order validation
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-ecommerce-order
      spec:
        displayName: JSG E-commerce Order
        version: v1.0.0
        context: /jsg-ecommerce/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /check
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"customerId":{"type":"string","minLength":1},"items":{"type":"array","items":{"type":"object","properties":{"productId":{"type":"string"},"quantity":{"type":"integer","minimum":1},"price":{"type":"number","minimum":0}},"required":["productId","quantity","price"]},"minItems":1},"shippingAddress":{"type":"object","properties":{"street":{"type":"string"},"city":{"type":"string"},"zipCode":{"type":"string","pattern":"^[0-9]{5}$"}},"required":["street","city","zipCode"]},"paymentMethod":{"type":"string","enum":["credit_card","paypal","bank_transfer"]}},"required":["customerId","items","shippingAddress","paymentMethod"]}'
                    showAssessment: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-ecommerce/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/jsg-ecommerce/v1.0.0/check" with body:
      """
      {"customerId": "C123", "items": [{"productId": "P456", "quantity": 2, "price": 29.99}], "shippingAddress": {"street": "123 Main St", "city": "Boston", "zipCode": "02101"}, "paymentMethod": "credit_card"}
      """
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-ecommerce-order"
    Then the response should be successful

  Scenario: API response contract enforcement
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-jsg-response-contract
      spec:
        displayName: JSG Response Contract
        version: v1.0.0
        context: /jsg-resp-contract/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: GET
            path: /echo
            policies:
              - name: json-schema-guardrail
                version: v0
                params:
                  response:
                    jsonPath: ""
                    schema: '{"type":"object","properties":{"method":{"type":"string"},"path":{"type":"string"},"headers":{"type":"object"}},"required":["method","path","headers"]}'
                    showAssessment: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jsg-resp-contract/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/jsg-resp-contract/v1.0.0/echo"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "test-jsg-response-contract"
    Then the response should be successful
