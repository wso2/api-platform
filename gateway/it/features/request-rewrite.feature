Feature: Request Transformation Policy Integration Tests
  Validate request-rewrite policy for path, query, and method rewrites

  Background:
    Given the gateway services are running

  Scenario: ReplacePrefixMatch rewrites the path prefix
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-prefix
      spec:
        displayName: Request Transformation Prefix
        version: v1.0
        context: /req-transform-prefix/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /api/v1
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplacePrefixMatch
                    replacePrefixMatch: "/api/v2"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-prefix/v1.0/api/v1" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-prefix/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"

  Scenario: ReplaceFullPath rewrites the entire path
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-full
      spec:
        displayName: Request Transformation Full Path
        version: v1.0
        context: /req-transform-full/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /api/v1
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplaceFullPath
                    replaceFullPath: "/fixed/path"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-full/v1.0/api/v1" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-full/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/fixed/path"

  Scenario: ReplaceRegexMatch rewrites using regex substitution
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-regex
      spec:
        displayName: Request Transformation Regex
        version: v1.0
        context: /req-transform-regex/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /api/v1
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplaceRegexMatch
                    replaceRegexMatch:
                      pattern: "^/api/v1$"
                      substitution: "/api/v2"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-regex/v1.0/api/v1" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-regex/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"

  Scenario: ReplaceRegexMatch reorders captured segments
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-regex-capture
      spec:
        displayName: Request Transformation Regex Capture
        version: v1.0
        context: /req-transform-regex-capture/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /*
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplaceRegexMatch
                    replaceRegexMatch:
                      pattern: "^/service/([^/]+)(/.*)$"
                      substitution: "\\2/instance/\\1"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-regex-capture/v1.0/service/foo/v1/api" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-regex-capture/v1.0/service/foo/v1/api"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/v1/api/instance/foo"

  Scenario: ReplaceRegexMatch is case-insensitive
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-regex-ci
      spec:
        displayName: Request Transformation Regex Case Insensitive
        version: v1.0
        context: /req-transform-regex-ci/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /*
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplaceRegexMatch
                    replaceRegexMatch:
                      pattern: "(?i)/xxx/"
                      substitution: "/yyy/"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-regex-ci/v1.0/aaa/XxX/bbb" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-regex-ci/v1.0/aaa/XxX/bbb"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/aaa/yyy/bbb"

  Scenario: ReplaceRegexMatch replaces all matches
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-regex-multi
      spec:
        displayName: Request Transformation Regex Replace All
        version: v1.0
        context: /req-transform-regex-multi/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /*
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplaceRegexMatch
                    replaceRegexMatch:
                      pattern: "one"
                      substitution: "two"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-regex-multi/v1.0/xxx/one/yyy/one/zzz" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-regex-multi/v1.0/xxx/one/yyy/one/zzz"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/xxx/two/yyy/two/zzz"

  Scenario: Query rewrite adds, replaces, and removes parameters
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-query
      spec:
        displayName: Request Transformation Query
        version: v1.0
        context: /req-transform-query/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /search
            policies:
              - name: request-rewrite
                version: v0
                params:
                  queryRewrite:
                    rules:
                      - action: Add
                        name: source
                        value: legacy
                      - action: Replace
                        name: q
                        value: new-value
                      - action: Remove
                        name: debug
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-query/v1.0/search?q=old-value&debug=true" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-query/v1.0/search?q=old-value&debug=true"
    Then the response status code should be 200
    And the JSON response field "args.source" should be "legacy"
    And the JSON response field "args.q" should be "new-value"

  Scenario: Method rewrite changes the request method
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-method
      spec:
        displayName: Request Transformation Method
        version: v1.0
        context: /req-transform-method/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /test/*
            policies:
              - name: request-rewrite
                version: v0
                params:
                  methodRewrite: POST
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-method/v1.0/test/hello" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-method/v1.0/test/hello"
    Then the response status code should be 200
    And the JSON response field "method" should be "POST"

  Scenario: API-level policy rewrites the path prefix
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-api-level-prefix
      spec:
        displayName: Request Transformation API Level Prefix
        version: v1.0
        context: /req-transform-api-prefix/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        policies:
          - name: request-rewrite
            version: v0
            params:
              pathRewrite:
                type: ReplacePrefixMatch
                replacePrefixMatch: "/api/v2"
        operations:
          - method: GET
            path: /api/v1
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-api-prefix/v1.0/api/v1" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-api-prefix/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"

  Scenario: API-level policy rewrites the method
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-api-level-method
      spec:
        displayName: Request Transformation API Level Method
        version: v1.0
        context: /req-transform-api-method/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        policies:
          - name: request-rewrite
            version: v0
            params:
              methodRewrite: POST
        operations:
          - method: GET
            path: /test/*
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-api-method/v1.0/test/hello" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-api-method/v1.0/test/hello"
    Then the response status code should be 200
    And the JSON response field "method" should be "POST"

  Scenario: Match conditions gate transformations
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-match
      spec:
        displayName: Request Transformation Match
        version: v1.0
        context: /req-transform-match/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /api/v1
            policies:
              - name: request-rewrite
                version: v0
                params:
                  match:
                    headers:
                      - name: x-client-id
                        type: Exact
                        value: client-123
                  pathRewrite:
                    type: ReplacePrefixMatch
                    replacePrefixMatch: "/api/v2"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-match/v1.0/api/v1" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-match/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v1"
    And I set header "x-client-id" to "client-123"
    When I send a GET request to "http://localhost:8080/req-transform-match/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"

  Scenario: Operation-level dynamic upstream rewrite routes to configured host
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-upstream-operation
      spec:
        displayName: Request Transformation Upstream Operation
        version: v1.0
        context: /req-transform-upstream-op/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /dynamic/*
            policies:
              - name: request-rewrite
                version: v0
                params:
                  upstreamRewrite:
                    host: echo-backend-multi-arch
                    port: 8080
                    scheme: http
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-upstream-op/v1.0/dynamic/health" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-upstream-op/v1.0/dynamic/hello"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should contain "echo-backend-multi-arch:8080"

  Scenario: API-level dynamic upstream rewrite routes all operations
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-upstream-api
      spec:
        displayName: Request Transformation Upstream API Level
        version: v1.0
        context: /req-transform-upstream-api/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        policies:
          - name: request-rewrite
            version: v0
            params:
              upstreamRewrite:
                host: echo-backend-multi-arch
                port: 8080
                scheme: http
        operations:
          - method: GET
            path: /api/*
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-upstream-api/v1.0/api/health" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-upstream-api/v1.0/api/value"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should contain "echo-backend-multi-arch:8080"

  Scenario: Dynamic upstream rewrite works with path rewrite
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-upstream-with-path
      spec:
        displayName: Request Transformation Upstream With Path Rewrite
        version: v1.0
        context: /req-transform-upstream-path/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /users/*
            policies:
              - name: request-rewrite
                version: v0
                params:
                  pathRewrite:
                    type: ReplacePrefixMatch
                    replacePrefixMatch: "/users/anything/accounts"
                  upstreamRewrite:
                    host: echo-backend-multi-arch
                    port: 8080
                    scheme: http
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-upstream-path/v1.0/users/health" to be ready
    And I set header "Content-Type" to "application/json"
    When I send a GET request to "http://localhost:8080/req-transform-upstream-path/v1.0/users/123"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should contain "echo-backend-multi-arch:8080"
    And the JSON response field "url" should contain "/anything/anything/accounts/123"

  Scenario: Client-supplied dynamic upstream header is ignored without upstream rewrite metadata
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: request-rewrite-client-header-ignored
      spec:
        displayName: Request Transformation Client Header Ignored
        version: v1.0
        context: /req-transform-client-header/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /api/*
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/req-transform-client-header/v1.0/api/health" to be ready
    And I set header "Content-Type" to "application/json"
    And I set header "x-ap-platform-dynamic-upstream-scheme" to "http"
    When I send a GET request to "http://localhost:8080/req-transform-client-header/v1.0/api/value"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should contain "echo-backend"
    And the JSON response field "url" should contain "/anything/api/value"
