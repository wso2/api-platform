@path-normalization
Feature: Path Normalization
  As an API developer
  I want the gateway's default path normalization behavior to resolve dot-segments
  and merge duplicate slashes before route matching, while leaving escaped slashes
  and literal dots inside a path segment untouched
  So that route matching cannot be bypassed via path tricks without breaking
  legitimate resource identifiers that happen to contain "." or an encoded "/"

  # Migrated unchanged in what it asserts. Two mechanical differences:
  #
  # The three `I wait for 2 seconds` sleeps are gone — the framework has no such step. Each was
  # waiting for the route to be programmed, which is what the readiness wait says directly.
  #
  # Request paths are RELATIVE, and here that is load-bearing rather than convention. The
  # original spelled absolute URLs; reducing one to its path goes through net/url, which decodes
  # %2F into a real slash. That would silently turn the escaped-slash scenario into its own
  # negative case and it would still "pass" — as a 404 for the wrong reason.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Default normalization resolves dot-segments and merges duplicate slashes
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: path-norm-api
      spec:
        displayName: Path-Norm-API
        version: v1.0
        context: /path-norm/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /weather
      """
    Then the response status code should be 201

    # A dot-segment traversal must be resolved to /weather before route matching
    When I send a "GET" request to "/path-norm/v1.0/other/../weather" until status 200
    Then the response status code should be 200
    And the JSON response field "path" should be "/weather"

    # Duplicate slashes must be merged to a single slash before route matching
    When I send a "GET" request to "/path-norm/v1.0//weather"
    Then the response status code should be 200
    And the JSON response field "path" should be "/weather"

    When I delete the API "path-norm-api"
    Then the response status code should be 200

  Scenario: Escaped slash (%2F) in a resource path segment is kept unchanged and is not treated as a path separator
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: path-escaped-slash-api
      spec:
        displayName: Path-Escaped-Slash-API
        version: v1.0
        context: /path-escaped-slash/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /pets/{name}
      """
    Then the response status code should be 201

    # %2F is kept unchanged (default path_with_escaped_slashes_action = KEEP_UNCHANGED), so
    # "pet%2FFarm" is one opaque path segment and matches the single-segment {name} template.
    When I send a "GET" request to "/path-escaped-slash/v1.0/pets/pet%2FFarm" until status 200
    Then the response status code should be 200

    # The literal (unescaped) equivalent has two path segments and must NOT match the
    # single-segment {name} template — this contrast confirms %2F is not silently decoded.
    When I send a "GET" request to "/path-escaped-slash/v1.0/pets/pet/Farm"
    Then the response status code should be 404

    When I delete the API "path-escaped-slash-api"
    Then the response status code should be 200

  Scenario: A path segment containing a literal dot is preserved by normalization
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: path-literal-dot-api
      spec:
        displayName: Path-Literal-Dot-API
        version: v1.0
        context: /path-literal-dot/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /pet.api
      """
    Then the response status code should be 201

    # "pet.api" is a normal path segment containing a dot, not a "." or ".." dot-segment,
    # so normalization must leave it untouched.
    When I send a "GET" request to "/path-literal-dot/v1.0/pet.api" until status 200
    Then the response status code should be 200
    And the JSON response field "path" should be "/pet.api"

    # Combine both: a real "./" dot-segment ahead of the dotted resource name must still be
    # resolved away, while "pet.api" itself remains untouched.
    When I send a "GET" request to "/path-literal-dot/v1.0/./pet.api"
    Then the response status code should be 200
    And the JSON response field "path" should be "/pet.api"

    When I delete the API "path-literal-dot-api"
    Then the response status code should be 200
