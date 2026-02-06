Feature: JSON Schema Guardrail Policy
  Test the json-schema-guardrail policy which validates request and response payloads
  against JSON Schema specifications, supports JSONPath extraction, inverted logic,
  and detailed error reporting.

  Background:
    Given I deploy an API with the following configuration:
      """
      name: test-api
      version: 1.0.0
      basePath: /test
      type: REST
      endpointConfig:
        production:
          endpoint: http://sample-backend:9080
      """

  # ====================================================================
  # BASIC REQUEST VALIDATION
  # ====================================================================

#   Scenario: Valid request passes schema validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"name": "John Doe", "age": 30}
#       """
#     Then the response status code should be 200

#   Scenario: Invalid request fails schema validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"name": "John Doe"}
#       """
#     Then the response status code should be 422
#     And the response body should contain "JSON_SCHEMA_GUARDRAIL"

#   Scenario: Missing required field fails validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"username":{"type":"string"},"email":{"type":"string"}},"required":["username","email"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"username": "johndoe"}
#       """
#     Then the response status code should be 422
#     And the response body should contain "GUARDRAIL_INTERVENED"

#   Scenario: Wrong type fails validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"name": "John", "age": "thirty"}
#       """
#     Then the response status code should be 422

  # ====================================================================
  # BASIC RESPONSE VALIDATION
  # ====================================================================

#   Scenario: Valid response passes schema validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: GET
#           path: /echo
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 response:
#                   schema: '{"type":"object","properties":{"method":{"type":"string"},"path":{"type":"string"}},"required":["method","path"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a GET request to "/test/echo"
#     Then the response status code should be 200

#   Scenario: Invalid response fails schema validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: GET
#           path: /echo
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 response:
#                   schema: '{"type":"object","properties":{"nonExistentField":{"type":"string"}},"required":["nonExistentField"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a GET request to "/test/echo"
#     Then the response status code should be 422
#     And the response body should contain "JSON_SCHEMA_GUARDRAIL"

  # ====================================================================
  # BOTH REQUEST AND RESPONSE VALIDATION
  # ====================================================================

#   Scenario: Validate both request and response
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /echo
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}'
#                 response:
#                   schema: '{"type":"object","properties":{"method":{"type":"string"}},"required":["method"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/echo" with body:
#       """
#       {"input": "test data"}
#       """
#     Then the response status code should be 200

  # ====================================================================
  # JSONPATH EXTRACTION
  # ====================================================================

#   Scenario: Validate specific field with JSONPath
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer","minimum":18}},"required":["name","age"]}'
#                   jsonPath: $.user
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"user": {"name": "Alice", "age": 25}, "metadata": "ignored"}
#       """
#     Then the response status code should be 200

#   Scenario: JSONPath extraction with invalid data
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"age":{"type":"integer","minimum":18}},"required":["age"]}'
#                   jsonPath: $.user
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"user": {"age": 15}, "other": "data"}
#       """
#     Then the response status code should be 422

#   Scenario: Validate nested object with JSONPath
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /orders
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"street":{"type":"string"},"city":{"type":"string"},"zipCode":{"type":"string"}},"required":["street","city","zipCode"]}'
#                   jsonPath: $.order.shippingAddress
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/orders" with body:
#       """
#       {"order": {"shippingAddress": {"street": "123 Main St", "city": "Boston", "zipCode": "02101"}}}
#       """
#     Then the response status code should be 200

  # ====================================================================
  # INVERT LOGIC
  # ====================================================================

#   Scenario: Invert logic passes when schema validation fails
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /validate
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"dangerousCommand":{"type":"string","pattern":"^(rm|delete|drop).*"}},"required":["dangerousCommand"]}'
#                   invert: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/validate" with body:
#       """
#       {"safeCommand": "list files"}
#       """
#     Then the response status code should be 200

#   Scenario: Invert logic blocks when schema validation succeeds
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /validate
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"command":{"type":"string","pattern":"^(rm|delete|drop).*"}},"required":["command"]}'
#                   invert: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/validate" with body:
#       """
#       {"command": "rm -rf /"}
#       """
#     Then the response status code should be 422

#   Scenario: Block requests matching malicious pattern with invert
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /sql
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"query":{"type":"string","pattern":".*DROP TABLE.*"}}}'
#                   invert: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/sql" with body:
#       """
#       {"query": "SELECT * FROM users"}
#       """
#     Then the response status code should be 200

  # ====================================================================
  # SHOW ASSESSMENT
  # ====================================================================

#   Scenario: Show detailed assessment on validation failure
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string","minLength":3},"age":{"type":"integer","minimum":18}},"required":["name","age"]}'
#                   showAssessment: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"name": "Jo", "age": 15}
#       """
#     Then the response status code should be 422
#     And the response body should contain "assessments"

#   Scenario: Hide assessment details when showAssessment is false
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}'
#                   showAssessment: false
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"age": 25}
#       """
#     Then the response status code should be 422
#     And the response body should contain "GUARDRAIL_INTERVENED"

  # ====================================================================
  # SCHEMA CONSTRAINTS
  # ====================================================================

#   Scenario: Validate string length constraints
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"username":{"type":"string","minLength":3,"maxLength":20}},"required":["username"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"username": "ab"}
#       """
#     Then the response status code should be 422

#   Scenario: Validate numeric range constraints
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /products
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"price":{"type":"number","minimum":0,"maximum":10000}},"required":["price"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/products" with body:
#       """
#       {"price": -5}
#       """
#     Then the response status code should be 422

#   Scenario: Validate array constraints
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /tags
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"tags":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":5}},"required":["tags"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/tags" with body:
#       """
#       {"tags": []}
#       """
#     Then the response status code should be 422

#   Scenario: Validate enum constraints
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /orders
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"status":{"type":"string","enum":["pending","processing","completed","cancelled"]}},"required":["status"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/orders" with body:
#       """
#       {"status": "invalid-status"}
#       """
#     Then the response status code should be 422

#   Scenario: Validate pattern constraints
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"email":{"type":"string","pattern":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"}},"required":["email"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"email": "invalid-email"}
#       """
#     Then the response status code should be 422

  # ====================================================================
  # COMPLEX SCHEMAS
  # ====================================================================

#   Scenario: Validate nested object schema
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /users
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"name":{"type":"string"},"address":{"type":"object","properties":{"street":{"type":"string"},"city":{"type":"string"}},"required":["street","city"]}},"required":["name","address"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/users" with body:
#       """
#       {"name": "John", "address": {"street": "Main St"}}
#       """
#     Then the response status code should be 422

#   Scenario: Validate array of objects
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /cart
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"productId":{"type":"string"},"quantity":{"type":"integer","minimum":1}},"required":["productId","quantity"]}}},"required":["items"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/cart" with body:
#       """
#       {"items": [{"productId": "123", "quantity": 2}, {"productId": "456", "quantity": 0}]}
#       """
#     Then the response status code should be 422

  # ====================================================================
  # EDGE CASES
  # ====================================================================

#   Scenario: Handle empty request body
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /validate
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"data":{"type":"string"}},"required":["data"]}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/validate" with body:
#       """
#       """
#     Then the response status code should be 422

#   Scenario: Handle invalid JSON
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /validate
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object"}'
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/validate" with body:
#       """
#       not valid json {
#       """
#     Then the response status code should be 422

#   Scenario: Handle invalid JSONPath
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /validate
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object"}'
#                   jsonPath: $.nonexistent.field
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/validate" with body:
#       """
#       {"data": "test"}
#       """
#     Then the response status code should be 422

  # ====================================================================
  # REAL-WORLD SCENARIOS
  # ====================================================================

#   Scenario: User registration with comprehensive validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /register
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"username":{"type":"string","minLength":3,"maxLength":20,"pattern":"^[a-zA-Z0-9_]+$"},"email":{"type":"string","pattern":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"},"password":{"type":"string","minLength":8},"age":{"type":"integer","minimum":13},"termsAccepted":{"type":"boolean","enum":[true]}},"required":["username","email","password","age","termsAccepted"]}'
#                   showAssessment: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/register" with body:
#       """
#       {"username": "john_doe", "email": "john@example.com", "password": "SecurePass123", "age": 25, "termsAccepted": true}
#       """
#     Then the response status code should be 200

#   Scenario: Block SQL injection patterns
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /search
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"query":{"type":"string","pattern":".*((DROP|DELETE|INSERT|UPDATE|SELECT).*(TABLE|FROM|WHERE)).*"}}}'
#                   invert: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/search" with body:
#       """
#       {"query": "normal search term"}
#       """
#     Then the response status code should be 200

#   Scenario: E-commerce order validation
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /orders
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 request:
#                   schema: '{"type":"object","properties":{"customerId":{"type":"string","minLength":1},"items":{"type":"array","items":{"type":"object","properties":{"productId":{"type":"string"},"quantity":{"type":"integer","minimum":1},"price":{"type":"number","minimum":0}},"required":["productId","quantity","price"]},"minItems":1},"shippingAddress":{"type":"object","properties":{"street":{"type":"string"},"city":{"type":"string"},"zipCode":{"type":"string","pattern":"^[0-9]{5}$"}},"required":["street","city","zipCode"]},"paymentMethod":{"type":"string","enum":["credit_card","paypal","bank_transfer"]}},"required":["customerId","items","shippingAddress","paymentMethod"]}'
#                   showAssessment: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/orders" with body:
#       """
#       {"customerId": "C123", "items": [{"productId": "P456", "quantity": 2, "price": 29.99}], "shippingAddress": {"street": "123 Main St", "city": "Boston", "zipCode": "02101"}, "paymentMethod": "credit_card"}
#       """
#     Then the response status code should be 200

#   Scenario: API response contract enforcement
#     Given I deploy an API with the following configuration:
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: GET
#           path: /echo
#           policies:
#             - name: json-schema-guardrail
#               version: v0
#               params:
#                 response:
#                   schema: '{"type":"object","properties":{"method":{"type":"string"},"path":{"type":"string"},"headers":{"type":"object"}},"required":["method","path","headers"]}'
#                   showAssessment: true
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a GET request to "/test/echo"
#     Then the response status code should be 200
