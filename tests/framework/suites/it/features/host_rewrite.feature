@host-rewrite
Feature: Host Rewrite Policy Integration Tests
  Validate host-rewrite policy that sets the Host/:authority header on upstream requests

  # Migrated with its assertions preserved. Mechanical differences from the legacy feature:
  #
  # The upstream is the shared testbench reflector (http://testbench:3000). The legacy
  # `http://echo-backend:80/anything` pointed at a go-httpbin whose /anything was its reflector
  # path; the testbench reflects on any path via its catch-all, so the /anything suffix is
  # dropped rather than prepended as a meaningless segment.
  #
  # Every scenario asserts on the Host the upstream received. That assertion is only expressible
  # because the testbench reflector now clones r.Header and sets Host from r.Host before
  # reflecting — Go's net/http otherwise promotes Host out of the header map, so the field would
  # be absent. Each `the JSON response field "headers.Host" should be ...` is re-expressed as the
  # canonical `the response should contain echoed header "Host" with value ...`, which reads the
  # same reflected header robustly across the bare-string and array-of-values shapes.
  #
  # The one strengthening: the no-manual scenario originally asserted the Host merely *contains*
  # "echo-backend" (the old upstream host). With this upstream the forwarded Host is exactly
  # "testbench:3000", so it is written as an exact match — stronger than the original substring
  # check, never weaker.
  #
  # The repeated per-scenario `Given I authenticate using basic auth as "admin"` is gone; the
  # single Background authentication covers every scenario, as the management credential persists
  # across scenarios and is not cleared by request-shaping resets.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Host rewrite sets the Host header on upstream request
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: host-rewrite-basic
      spec:
        displayName: Host Rewrite Basic
        version: v1.0
        context: /host-rewrite-basic/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    When I send a "GET" request to "/host-rewrite-basic/v1.0/test" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "example-updated.com"

  Scenario: Host rewrite with port number
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: host-rewrite-with-port
      spec:
        displayName: Host Rewrite With Port
        version: v1.0
        context: /host-rewrite-port/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    When I send a "GET" request to "/host-rewrite-port/v1.0/test" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "backend.example.com:8080"

  Scenario: Host rewrite at API level applies to all operations
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: host-rewrite-api-level
      spec:
        displayName: Host Rewrite API Level
        version: v1.0
        context: /host-rewrite-api/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    When I send a "GET" request to "/host-rewrite-api/v1.0/test1" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "api-level.example.com"
    When I send a "POST" request to "/host-rewrite-api/v1.0/test2"
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "api-level.example.com"

  Scenario: Host rewrite without hostRewrite manual should not work
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: host-rewrite-no-manual
      spec:
        displayName: Host Rewrite No Manual
        version: v1.0
        context: /host-rewrite-no-manual/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
            policies:
              - name: host-rewrite
                version: v1
                params:
                  host: should-not-be-used.com
      """
    Then the response status code should be 201
    When I send a "GET" request to "/host-rewrite-no-manual/v1.0/test" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "testbench"

  Scenario: Operation-level policy overrides API-level policy
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: host-rewrite-override
      spec:
        displayName: Host Rewrite Override
        version: v1.0
        context: /host-rewrite-override/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    When I send a "GET" request to "/host-rewrite-override/v1.0/default" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "api-level.example.com"
    When I send a "GET" request to "/host-rewrite-override/v1.0/override"
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "operation-level.example.com"

  Scenario: Host rewrite works with different HTTP methods
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: host-rewrite-methods
      spec:
        displayName: Host Rewrite HTTP Methods
        version: v1.0
        context: /host-rewrite-methods/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    When I send a "GET" request to "/host-rewrite-methods/v1.0/test" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "get.example.com"
    When I send a "POST" request to "/host-rewrite-methods/v1.0/test"
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "post.example.com"
    When I send a "PUT" request to "/host-rewrite-methods/v1.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And the response should contain echoed header "Host" with value "put.example.com"
