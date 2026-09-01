@api-with-policies
Feature: API Configuration with Policies
  As an API developer
  I want to deploy APIs with various policy configurations
  So that I can test policy integration and handler coverage

  # Migrated with its assertions untouched. The only mechanical changes:
  #
  # Upstreams move from the retired sample-backend to the shared testbench reflector
  # (http://sample-backend:9080 -> http://testbench:3000), keeping any appended path so
  # the empty-version scenarios still exercise a non-root upstream path.
  #
  # The two absolute readiness/request URLs (http://localhost:8080/...) become relative,
  # since the block, not the feature, owns the host and port.
  #
  # The `Given I authenticate using basic auth as "admin"` that the two empty-version
  # scenarios repeated before their delete is gone — it re-established a credential nothing
  # had cleared, and Background authenticates once for every scenario.
  #
  # Every scenario already deploys and deletes its own API, so each is self-contained; the
  # scenarios that only assert on the deploy response add no readiness wait because they
  # never invoke the data plane (the deploy step itself waits for management-API visibility).

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy API without any policies
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: no-policy-api
      spec:
        displayName: No-Policy-Api
        version: v1.0
        context: /no-policy
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the API "no-policy-api"
    Then the response status code should be 200

  Scenario: Deploy API with operation-level policy
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: operation-policy-api
      spec:
        displayName: Operation-Policy-Api
        version: v1.0
        context: /op-policy
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
            policies:
              - name: cors
                version: v1
                params:
                  allowedOrigins:
                    - "*"
                  allowedMethods:
                    - GET
                    - POST
                  allowedHeaders:
                    - "*"
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Cleanup
    When I delete the API "operation-policy-api"
    Then the response status code should be 200

  Scenario: Deploy API with API-level policy
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-policy-api
      spec:
        displayName: Api-Level-Policy-Api
        version: v1.0
        context: /api-policy
        upstream:
          main:
            url: http://testbench:3000
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://localhost:3000"
              allowedMethods:
                - GET
              allowedHeaders:
                - Content-Type
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Cleanup
    When I delete the API "api-level-policy-api"
    Then the response status code should be 200

  Scenario: Update API to add policies
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-add-policy-api
      spec:
        displayName: Update-Add-Policy-Api
        version: v1.0
        context: /update-policy
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I update the API "update-add-policy-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-add-policy-api
      spec:
        displayName: Update-Add-Policy-Api
        version: v1.0
        context: /update-policy
        upstream:
          main:
            url: http://testbench:3000
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "*"
              allowedMethods:
                - GET
              allowedHeaders:
                - "*"
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 200
    # Cleanup
    When I delete the API "update-add-policy-api"
    Then the response status code should be 200

  Scenario: Update API to remove policies
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-remove-policy-api
      spec:
        displayName: Update-Remove-Policy-Api
        version: v1.0
        context: /update-remove
        upstream:
          main:
            url: http://testbench:3000
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "*"
              allowedMethods:
                - GET
              allowedHeaders:
                - "*"
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I update the API "update-remove-policy-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-remove-policy-api
      spec:
        displayName: Update-Remove-Policy-Api
        version: v1.0
        context: /update-remove
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 200
    # Cleanup
    When I delete the API "update-remove-policy-api"
    Then the response status code should be 200

  Scenario: Deploy API with API-level policy using empty version resolves to latest
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: empty-version-api-level-api
      spec:
        displayName: Empty-Version-Api-Level-Api
        version: v1.0
        context: /empty-version-api/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            params:
              allowedOrigins:
                - "http://example.com"
              allowedMethods:
                - GET
              allowedHeaders:
                - Content-Type
        operations:
          - method: GET
            path: /{country_code}/{city}
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/empty-version-api/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    # Cleanup
    When I delete the API "empty-version-api-level-api"
    Then the response status code should be 200

  Scenario: Deploy API with operation-level policy using empty version resolves to latest
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: empty-version-op-level-api
      spec:
        displayName: Empty-Version-Op-Level-Api
        version: v1.0
        context: /empty-version-op/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /{country_code}/{city}
            policies:
              - name: cors
                params:
                  allowedOrigins:
                    - "http://example.com"
                  allowedMethods:
                    - GET
                  allowedHeaders:
                    - Content-Type
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/empty-version-op/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    # Cleanup
    When I delete the API "empty-version-op-level-api"
    Then the response status code should be 200

  Scenario: Deploy API with different HTTP methods
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: http-methods-api
      spec:
        displayName: Http-Methods-Api
        version: v1.0
        context: /methods
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /resource
          - method: POST
            path: /resource
          - method: PUT
            path: /resource
          - method: DELETE
            path: /resource
          - method: PATCH
            path: /resource
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Cleanup
    When I delete the API "http-methods-api"
    Then the response status code should be 200
