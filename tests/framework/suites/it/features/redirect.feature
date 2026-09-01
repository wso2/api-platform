@redirect
Feature: Redirect Policy Integration Tests
  Test the redirect policy for issuing HTTP redirects (Gateway-API RequestRedirect
  semantics) without calling the backend. Each API carries a plain "/probe" operation
  used only to gate readiness — the redirect route itself returns a 3xx and therefore
  never becomes "ready" via a 200 poll. The test HTTP client does not follow redirects,
  so the 3xx status and the Location header are asserted directly.

  # Migrated with every Location assertion intact, which took one addition: the Background
  # pins the request Host to localhost:8080.
  #
  # The redirect policy builds Location from the REQUEST's authority, and four of these five
  # scenarios assert a Location containing it — ":8080" in three, a bare "localhost" in the
  # scheme-upgrade case. The legacy suite got that for free from a fixed host port; this
  # framework publishes an ephemeral one so two blocks can run at once, so the same assertion
  # would read "http://example.org:33871/..." and differ every run.
  #
  # Pinning the Host header reproduces exactly the condition the original ran under — the
  # connection still goes to the mapped port, only the authority the gateway sees is fixed.
  # The alternative was relaxing the assertions to "should contain example.org", which would
  # have dropped the port and path checks that are most of what these scenarios verify.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I set request host to "localhost:8080"

  # ========================================
  # Component override / preservation
  # ========================================

  Scenario: Redirect to a different host defaults to 302 and preserves scheme, port and path
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: test-redirect-host
      spec:
        displayName: Redirect-Host-Test
        version: v1.0.0
        context: /redirect-host/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /go
            policies:
              - name: redirect
                version: v0
                params:
                  hostname: example.org
      """
    Then the response status code should be 201
    And I send a "GET" request to "/redirect-host/v1.0.0/probe" until status 200
    When I send a "GET" request to "/redirect-host/v1.0.0/go"
    Then the response status code should be 302
    And the response header "Location" should be "http://example.org:8080/redirect-host/v1.0.0/go"

  Scenario: Explicit status code produces a permanent 301 redirect
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: test-redirect-permanent
      spec:
        displayName: Redirect-Permanent-Test
        version: v1.0.0
        context: /redirect-permanent/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /go
            policies:
              - name: redirect
                version: v0
                params:
                  statusCode: 301
                  hostname: example.org
      """
    Then the response status code should be 201
    And I send a "GET" request to "/redirect-permanent/v1.0.0/probe" until status 200
    When I send a "GET" request to "/redirect-permanent/v1.0.0/go"
    Then the response status code should be 301
    And the response header "Location" should be "http://example.org:8080/redirect-permanent/v1.0.0/go"

  Scenario: Scheme upgrade to https drops the default port and preserves host and path
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: test-redirect-scheme
      spec:
        displayName: Redirect-Scheme-Test
        version: v1.0.0
        context: /redirect-scheme/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /go
            policies:
              - name: redirect
                version: v0
                params:
                  scheme: https
      """
    Then the response status code should be 201
    And I send a "GET" request to "/redirect-scheme/v1.0.0/probe" until status 200
    When I send a "GET" request to "/redirect-scheme/v1.0.0/go"
    Then the response status code should be 302
    And the response header "Location" should be "https://localhost/redirect-scheme/v1.0.0/go"

  # ========================================
  # Path rewrite
  # ========================================

  Scenario: Full path replacement rewrites the entire path
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: test-redirect-fullpath
      spec:
        displayName: Redirect-FullPath-Test
        version: v1.0.0
        context: /redirect-fullpath/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /go
            policies:
              - name: redirect
                version: v0
                params:
                  hostname: example.org
                  path:
                    mode: full
                    value: /v2/target
      """
    Then the response status code should be 201
    And I send a "GET" request to "/redirect-fullpath/v1.0.0/probe" until status 200
    When I send a "GET" request to "/redirect-fullpath/v1.0.0/go"
    Then the response status code should be 302
    And the response header "Location" should be "http://example.org:8080/v2/target"

  # ========================================
  # Full override
  # ========================================

  Scenario: Overriding every component builds a fully rewritten Location with a 308 status
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: test-redirect-full-override
      spec:
        displayName: Redirect-Full-Override-Test
        version: v1.0.0
        context: /redirect-override/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /go
            policies:
              - name: redirect
                version: v0
                params:
                  statusCode: 308
                  scheme: https
                  hostname: newhost.example.com
                  port: 8443
                  path:
                    mode: full
                    value: /moved
      """
    Then the response status code should be 201
    And I send a "GET" request to "/redirect-override/v1.0.0/probe" until status 200
    When I send a "GET" request to "/redirect-override/v1.0.0/go"
    Then the response status code should be 308
    And the response header "Location" should be "https://newhost.example.com:8443/moved"
