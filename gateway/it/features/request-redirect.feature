# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

Feature: Structured request redirect (normal RestApi path)
  As an API developer
  I want operations to issue HTTP redirects with per-component overrides
  So that scheme/host/port/path redirects work on the custom RestApi path, preserving
  any component left unspecified from the original request (Gateway-API semantics)

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # One API carries a 200 readiness operation plus several redirect operations, each overriding a
  # different component. The test client does not follow redirects, so it asserts the 3xx status
  # and the Location header directly. Components the redirect omits must be preserved from the
  # original request (notably the path), which is the core correctness contract.
  Scenario: Redirect operations override only the specified components and preserve the rest
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: redirect-demo-api
      spec:
        displayName: Redirect-Demo-API
        version: v1.0
        context: /redirect-demo/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /ready
          - method: GET
            path: /to-host
            redirect:
              statusCode: 301
              hostname: moved.example.com
          - method: GET
            path: /to-https
            redirect:
              statusCode: 302
              scheme: https
          - method: GET
            path: /rewrite
            redirect:
              statusCode: 302
              path:
                type: ReplaceFullPath
                replaceFullPath: /brand-new
          - method: GET
            path: /keep-all
            redirect:
              statusCode: 308
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/redirect-demo/v1.0/ready" to be ready

    # Hostname override (301): host changes, scheme (http) and the original path are preserved.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/redirect-demo/v1.0/to-host"
    Then the response status code should be 301
    And the response header "Location" should contain "moved.example.com"
    And the response header "Location" should contain "/redirect-demo/v1.0/to-host"

    # Scheme override (302): http upgraded to https, host and path preserved.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/redirect-demo/v1.0/to-https"
    Then the response status code should be 302
    And the response header "Location" should contain "https://"
    And the response header "Location" should contain "/redirect-demo/v1.0/to-https"

    # Full path replacement (302): the path is replaced, scheme and host preserved.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/redirect-demo/v1.0/rewrite"
    Then the response status code should be 302
    And the response header "Location" should contain "/brand-new"

    # Status-only (308): every component (scheme, host, path) is preserved from the request.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/redirect-demo/v1.0/keep-all"
    Then the response status code should be 308
    And the response header "Location" should exist
    And the response header "Location" should contain "/redirect-demo/v1.0/keep-all"

    Given I authenticate using basic auth as "admin"
    When I delete the API "redirect-demo-api"
    Then the response should be successful
