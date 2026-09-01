@route-path-matching
Feature: Route Path Matching
  As an API developer
  I want paths "/" and "/*" to match correctly in Envoy
  So that requests with and without trailing slashes are routed as expected

  # Migrated with its assertions untouched. Only the upstream (the shared testbench reflector)
  # and the relative request paths differ.
  #
  # The per-scenario `I wait for 2 seconds` sleeps are gone — the framework has no such step.
  # Each was waiting for the route to be programmed, which the readiness wait says directly.
  #
  # Request paths are RELATIVE. In "Exact path preserves trailing slash to upstream" the
  # forwarded path is asserted directly via the reflector's `path` field rather than the legacy
  # suite's full-URL `url` string, whose scheme and host were an artifact of the old backend —
  # `path` pins the exact byte (the trailing slash) that scenario exists to check.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Wildcard path /* matches requests with a subpath
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-wildcard-api
      spec:
        displayName: Route-Wildcard-API
        version: v1.0
        context: /route-wildcard/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /*
      """
    Then the response status code should be 201
    And I send a "GET" request to "/route-wildcard/v1.0/data" until status 200

    When I send a "GET" request to "/route-wildcard/v1.0/us/seattle"
    Then the response status code should be 200

    When I send a "GET" request to "/route-wildcard/v1.0/data"
    Then the response status code should be 200

    When I delete the API "route-wildcard-api"
    Then the response status code should be 200

  Scenario: Wildcard path /* enforces HTTP method
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-wildcard-method-api
      spec:
        displayName: Route-Wildcard-Method-API
        version: v1.0
        context: /route-wildcard-method/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /*
      """
    Then the response status code should be 201
    And I send a "GET" request to "/route-wildcard-method/v1.0/data" until status 200

    When I send a "GET" request to "/route-wildcard-method/v1.0/data"
    Then the response status code should be 200

    When I send a "POST" request to "/route-wildcard-method/v1.0/data"
    Then the response status code should be 404

    When I delete the API "route-wildcard-method-api"
    Then the response status code should be 200

  Scenario: Root path / matches request with trailing slash
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-root-api
      spec:
        displayName: Route-Root-API
        version: v1.0
        context: /route-root/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /
      """
    Then the response status code should be 201
    And I send a "GET" request to "/route-root/v1.0/" until status 200

    When I send a "GET" request to "/route-root/v1.0/"
    Then the response status code should be 200

    When I delete the API "route-root-api"
    Then the response status code should be 200

  Scenario: Root path / matches request without trailing slash
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-root-noslash-api
      spec:
        displayName: Route-Root-NoSlash-API
        version: v1.0
        context: /route-root-noslash/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /
      """
    Then the response status code should be 201
    And I send a "GET" request to "/route-root-noslash/v1.0" until status 200

    When I send a "GET" request to "/route-root-noslash/v1.0"
    Then the response status code should be 200

    When I delete the API "route-root-noslash-api"
    Then the response status code should be 200

  Scenario: Wildcard /* does not match a sibling context prefix
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-wildcard-boundary-api
      spec:
        displayName: Route-Wildcard-Boundary-API
        version: v1.0
        context: /route-wc-boundary/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /*
      """
    Then the response status code should be 201
    And I send a "GET" request to "/route-wc-boundary/v1.0/data" until status 200

    # Exact prefix and sub-paths must match
    When I send a "GET" request to "/route-wc-boundary/v1.0/data"
    Then the response status code should be 200

    # Bare prefix (no trailing slash) must also match
    When I send a "GET" request to "/route-wc-boundary/v1.0"
    Then the response status code should be 200

    # Sibling context that shares the prefix must NOT match
    When I send a "GET" request to "/route-wc-boundary/v1.0beta/data"
    Then the response status code should be 404

    When I delete the API "route-wildcard-boundary-api"
    Then the response status code should be 200

  Scenario: Exact path matches both with and without trailing slash
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-exact-slash-api
      spec:
        displayName: Route-Exact-Slash-API
        version: v1.0
        context: /route-exact-slash/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /weather
      """
    Then the response status code should be 201
    And I send a "GET" request to "/route-exact-slash/v1.0/weather" until status 200

    When I send a "GET" request to "/route-exact-slash/v1.0/weather"
    Then the response status code should be 200

    When I send a "GET" request to "/route-exact-slash/v1.0/weather/"
    Then the response status code should be 200

    When I delete the API "route-exact-slash-api"
    Then the response status code should be 200

  Scenario: Exact path preserves trailing slash to upstream
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-exact-upstream-slash-api
      spec:
        displayName: Route-Exact-Upstream-Slash-API
        version: v1.0
        context: /route-exact-upstream/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /weather
      """
    Then the response status code should be 201

    # Without trailing slash — upstream must not receive one
    When I send a "GET" request to "/route-exact-upstream/v1.0/weather" until status 200
    Then the response status code should be 200
    And the JSON response field "path" should be "/weather"

    # With trailing slash — upstream must receive it
    When I send a "GET" request to "/route-exact-upstream/v1.0/weather/"
    Then the response status code should be 200
    And the JSON response field "path" should be "/weather/"

    When I delete the API "route-exact-upstream-slash-api"
    Then the response status code should be 200

  Scenario: Wildcard /foo/* preserves the matched prefix (and method) on the upstream path
    # A wildcard operation path "/foo/*" must forward "/foo" and any sub-path to the upstream,
    # not strip it down to "/". The PUT /put/* operation checks that the matched prefix and method
    # both reach a method-specific endpoint. The sample backend echoes the path/method it received,
    # so each assertion pins the upstream request.
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: route-wildcard-prefix-api
      spec:
        displayName: Route-Wildcard-Prefix-API
        version: v1.0
        context: /route-wc-prefix/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /forecast/*
          - method: PUT
            path: /put/*
      """
    Then the response status code should be 201

    # Bare prefix — upstream must receive /forecast (before the fix it was stripped to /)
    When I send a "GET" request to "/route-wc-prefix/v1.0/forecast" until status 200
    Then the response status code should be 200
    And the JSON response field "path" should be "/forecast"

    # Single sub-path segment — upstream must receive /forecast/today (was /today before the fix)
    When I send a "GET" request to "/route-wc-prefix/v1.0/forecast/today"
    Then the response status code should be 200
    And the JSON response field "path" should be "/forecast/today"

    # Nested sub-paths
    When I send a "GET" request to "/route-wc-prefix/v1.0/forecast/a/b/c"
    Then the response status code should be 200
    And the JSON response field "path" should be "/forecast/a/b/c"

    # Explicit non-GET method — matched prefix and method must both reach the upstream
    When I send a "PUT" request to "/route-wc-prefix/v1.0/put" with body:
      """
      {"hello":"world"}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/put"
    And the JSON response field "method" should be "PUT"

    When I delete the API "route-wildcard-prefix-api"
    Then the response status code should be 200
