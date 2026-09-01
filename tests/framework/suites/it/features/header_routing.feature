@header-routing
Feature: Header-based route selection (normal RestApi path)
  As an API developer
  I want operations on the same path to be selected by request-header matches
  So that header-based routing works on the custom RestApi path, not only via Gateway API

  # Migrated with its assertions untouched. Only the upstream (the shared testbench reflector)
  # and the relative request paths differ; every step it uses already existed.
  #
  # The repeated `Given I authenticate using basic auth as "admin"` before each delete is gone.
  # It was re-establishing credentials that `I reset the request` had just wiped — in this
  # framework clearing headers touches request-shaping state only, not the management-API
  # credential, and that state is reset before every scenario anyway.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # Several operations share the path /pick and differ only by match.headers. Each carries a
  # respond policy with a distinct status so the selected route is unambiguous. This exercises
  # exact header matching, RegularExpression header matching, the more-specific-route-wins
  # precedence over a header-less catch-all, and native header-match route selection — all on
  # the normal management-API path.
  Scenario: Requests are routed to the operation whose header match they satisfy
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: header-routing-api
      spec:
        displayName: Header-Routing-API
        version: v1.0
        context: /header-routing/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /ready
          - match:
              method: GET
              path:
                value: /pick
              headers:
                - name: x-variant
                  value: alpha
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 201
          - match:
              method: GET
              path:
                value: /pick
              headers:
                - name: x-variant
                  value: beta
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 202
          - match:
              method: GET
              path:
                value: /pick
              headers:
                - name: x-variant
                  type: RegularExpression
                  value: "^v[0-9]+$"
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 203
          - method: GET
            path: /pick
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
      """
    Then the response status code should be 201
    And I send a "GET" request to "/header-routing/v1.0/ready" until status 200

    # Exact header match -> alpha route (201)
    When I reset the request
    And I set header "x-variant" to "alpha"
    And I send a "GET" request to "/header-routing/v1.0/pick"
    Then the response status code should be 201

    # Exact header match -> beta route (202)
    When I reset the request
    And I set header "x-variant" to "beta"
    And I send a "GET" request to "/header-routing/v1.0/pick"
    Then the response status code should be 202

    # RegularExpression header match -> regex route (203)
    When I reset the request
    And I set header "x-variant" to "v12"
    And I send a "GET" request to "/header-routing/v1.0/pick"
    Then the response status code should be 203

    # A header value matching no specific route falls through to the header-less catch-all (200),
    # proving the header routes are not greedily matched.
    When I reset the request
    And I set header "x-variant" to "does-not-match"
    And I send a "GET" request to "/header-routing/v1.0/pick"
    Then the response status code should be 200

    # No header at all -> catch-all (200)
    When I reset the request
    And I send a "GET" request to "/header-routing/v1.0/pick"
    Then the response status code should be 200

    When I delete the API "header-routing-api"
    Then the response status code should be 200

  # The SAME path can be served by a simple (header-less) operation AND a match operation that
  # adds a header condition. They are two separate operations — the header match gives the second
  # a distinct route key, so both coexist. The header-matched operation wins WHEN its header is
  # present; otherwise the request falls through to the header-less operation (more-specific wins).
  Scenario: A simple operation and a match operation on the same path coexist by header precedence
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mixed-form-api
      spec:
        displayName: Mixed-Form-API
        version: v1.0
        context: /mixed-form/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /ready
          - method: GET
            path: /via-match
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
          - match:
              method: GET
              path:
                value: /via-match
              headers:
                - name: x-variant
                  value: alpha
                  type: Exact
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 210
      """
    Then the response status code should be 201
    And I send a "GET" request to "/mixed-form/v1.0/ready" until status 200

    # header present -> the match operation wins
    When I reset the request
    And I set header "x-variant" to "alpha"
    And I send a "GET" request to "/mixed-form/v1.0/via-match"
    Then the response status code should be 210

    # header absent -> falls through to the simple (header-less) operation
    When I reset the request
    And I send a "GET" request to "/mixed-form/v1.0/via-match"
    Then the response status code should be 200

    When I delete the API "mixed-form-api"
    Then the response status code should be 200
