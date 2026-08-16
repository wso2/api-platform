@cors
Feature: CORS Policy
  As an API developer
  I want to configure CORS policies on my API
  So that cross-origin requests are correctly allowed and preflighted

  # Migrated unchanged in what it asserts. Mechanical differences only:
  #
  # Upstream is the shared testbench reflector (http://testbench:3000/api/v1); the /api/v1
  # path the original appended is kept. Request and readiness URLs are relative.
  #
  # `I authenticate using basic auth as "admin"` is consolidated into Background. The original
  # repeated it at the start of every deploying scenario and again before each delete — the
  # latter was re-establishing credentials that `I reset the request` had wiped. In this
  # framework clearing headers touches request-shaping state only, not the management-API
  # credential, and that state resets before every scenario, so one Background line suffices.
  #
  # Every scenario deploys its own API and waits for readiness. The original deployed an API
  # once and let later scenarios reuse it, but here `I create an API with configuration` registers
  # the API for per-scenario cleanup, so it is torn down at the end of the scenario that
  # deployed it — a reusing scenario would run against an API that no longer exists. That is
  # why the two preflight/simple-GET groups each repeat the same deploy docstring; the shared
  # metadata.name cannot collide because each copy is cleaned up before the next scenario runs.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Preflight request allows configured origin, methods, and headers
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-preflight-api
      spec:
        displayName: CORS Preflight API
        version: v1.0
        context: /cors-preflight/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
                - "http://localhost:5000"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - X-Content-Type-Options
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: OPTIONS
            path: /{country_code}/{city}
      """
    Then the response status code should be 201
    And I send a "GET" request to "/cors-preflight/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    And the response header "Access-Control-Allow-Methods" should contain "GET"
    And the response header "Access-Control-Allow-Methods" should contain "POST"
    And the response header "Access-Control-Allow-Headers" should contain "Content-Type"

  Scenario: Preflight request fails for disallowed origin
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-preflight-api
      spec:
        displayName: CORS Preflight API
        version: v1.0
        context: /cors-preflight/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
                - "http://localhost:5000"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - X-Content-Type-Options
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: OPTIONS
            path: /{country_code}/{city}
      """
    Then the response status code should be 201
    And I send a "GET" request to "/cors-preflight/v1.0/test/test" until status 200

    When I set header "Origin" to "http://evil.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist

  Scenario: Preflight request fails for disallowed method
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-preflight-api
      spec:
        displayName: CORS Preflight API
        version: v1.0
        context: /cors-preflight/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
                - "http://localhost:5000"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - X-Content-Type-Options
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: OPTIONS
            path: /{country_code}/{city}
      """
    Then the response status code should be 201
    And I send a "GET" request to "/cors-preflight/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "PUT"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist

  Scenario: Preflight request fails for disallowed header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-preflight-api
      spec:
        displayName: CORS Preflight API
        version: v1.0
        context: /cors-preflight/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
                - "http://localhost:5000"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - X-Content-Type-Options
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: OPTIONS
            path: /{country_code}/{city}
      """
    Then the response status code should be 201
    And I send a "GET" request to "/cors-preflight/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Authorization"
    And I send a "OPTIONS" request to "/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist

  Scenario: Simple GET from allowed origin gets CORS response headers
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-api
      spec:
        displayName: CORS Simple Request API
        version: v1.0
        context: /cors-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - "X-Custom-Header"
              allowCredentials: true
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
      """
    Then the response status code should be 201

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/cors-simple/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"

  Scenario: Simple GET from disallowed origin has upstream CORS headers stripped
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-api
      spec:
        displayName: CORS Simple Request API
        version: v1.0
        context: /cors-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - "X-Custom-Header"
              allowCredentials: true
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
      """
    Then the response status code should be 201

    When I set header "Origin" to "http://evil.com"
    And I send a "GET" request to "/cors-simple/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should not exist

  Scenario: Simple GET without Origin header gets no CORS headers
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-api
      spec:
        displayName: CORS Simple Request API
        version: v1.0
        context: /cors-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - "X-Custom-Header"
              allowCredentials: true
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
      """
    Then the response status code should be 201

    When I send a "GET" request to "/cors-simple/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should not exist

  Scenario: Simple GET from allowed origin gets Vary header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-api
      spec:
        displayName: CORS Simple Request API
        version: v1.0
        context: /cors-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - "X-Custom-Header"
              allowCredentials: true
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
      """
    Then the response status code should be 201

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/cors-simple/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Vary" should be "Origin"

  Scenario: Simple GET from allowed origin gets Allow-Credentials header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-api
      spec:
        displayName: CORS Simple Request API
        version: v1.0
        context: /cors-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - "X-Custom-Header"
              allowCredentials: true
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
      """
    Then the response status code should be 201

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/cors-simple/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Credentials" should be "true"

  Scenario: Simple GET from allowed origin gets Expose-Headers
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-api
      spec:
        displayName: CORS Simple Request API
        version: v1.0
        context: /cors-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
                - "https://*.example.com"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - "X-Custom-Header"
              allowCredentials: true
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
      """
    Then the response status code should be 201

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/cors-simple/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Expose-Headers" should be "X-Custom-Header"

  Scenario: Simple GET with wildcard origin gets CORS headers
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cors-simple-wildcard-api
      spec:
        displayName: CORS Simple Wildcard API
        version: v1.0
        context: /cors-simple-wildcard/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "*"
              allowedMethods:
                - "GET"
                - "POST"
        operations:
          - method: GET
            path: /{country_code}/{city}
      """
    Then the response status code should be 201

    When I set header "Origin" to "http://anysite.com"
    And I send a "GET" request to "/cors-simple-wildcard/v1.0/us/seattle" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "*"
