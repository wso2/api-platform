Feature: Host Rewrite Policy Integration Tests
  Validate host-rewrite policy that sets the Host/:authority header on upstream requests

  Background:
    Given the gateway services are running

  Scenario: Host rewrite sets the Host header on upstream request
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: host-rewrite-basic
      spec:
        displayName: Host Rewrite Basic
        version: v1.0
        context: /host-rewrite-basic/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
            hostRewrite: manual
        operations:
          - method: GET
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: example-updated.com
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/host-rewrite-basic/v1.0/test" to be ready
    When I send a GET request to "http://localhost:8080/host-rewrite-basic/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "example-updated.com"

  Scenario: Host rewrite with port number
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: host-rewrite-with-port
      spec:
        displayName: Host Rewrite With Port
        version: v1.0
        context: /host-rewrite-port/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
            hostRewrite: manual
        operations:
          - method: GET
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: backend.example.com:8080
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/host-rewrite-port/v1.0/test" to be ready
    When I send a GET request to "http://localhost:8080/host-rewrite-port/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "backend.example.com:8080"

  Scenario: Host rewrite at API level applies to all operations
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: host-rewrite-api-level
      spec:
        displayName: Host Rewrite API Level
        version: v1.0
        context: /host-rewrite-api/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
            hostRewrite: manual
        policies:
          - name: host-rewrite
            version: v1
            params:
              host: api-level.example.com
        operations:
          - method: GET
            path: /test1
          - method: POST
            path: /test2
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/host-rewrite-api/v1.0/test1" to be ready
    When I send a GET request to "http://localhost:8080/host-rewrite-api/v1.0/test1"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "api-level.example.com"
    When I send a POST request to "http://localhost:8080/host-rewrite-api/v1.0/test2"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "api-level.example.com"

  Scenario: Host rewrite without hostRewrite manual should not work
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: host-rewrite-no-manual
      spec:
        displayName: Host Rewrite No Manual
        version: v1.0
        context: /host-rewrite-no-manual/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: should-not-be-used.com
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/host-rewrite-no-manual/v1.0/test" to be ready
    When I send a GET request to "http://localhost:8080/host-rewrite-no-manual/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should contain "echo-backend"

  Scenario: Operation-level policy overrides API-level policy
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: host-rewrite-override
      spec:
        displayName: Host Rewrite Override
        version: v1.0
        context: /host-rewrite-override/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
            hostRewrite: manual
        policies:
          - name: host-rewrite
            version: v1
            params:
              host: api-level.example.com
        operations:
          - method: GET
            path: /default
          - method: GET
            path: /override
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: operation-level.example.com
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/host-rewrite-override/v1.0/default" to be ready
    When I send a GET request to "http://localhost:8080/host-rewrite-override/v1.0/default"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "api-level.example.com"
    When I send a GET request to "http://localhost:8080/host-rewrite-override/v1.0/override"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "operation-level.example.com"

  Scenario: Host rewrite works with different HTTP methods
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: host-rewrite-methods
      spec:
        displayName: Host Rewrite HTTP Methods
        version: v1.0
        context: /host-rewrite-methods/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
            hostRewrite: manual
        operations:
          - method: GET
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: get.example.com
          - method: POST
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: post.example.com
          - method: PUT
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: put.example.com
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/host-rewrite-methods/v1.0/test" to be ready
    When I send a GET request to "http://localhost:8080/host-rewrite-methods/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "get.example.com"
    When I send a POST request to "http://localhost:8080/host-rewrite-methods/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "post.example.com"
    When I send a PUT request to "http://localhost:8080/host-rewrite-methods/v1.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "put.example.com"
