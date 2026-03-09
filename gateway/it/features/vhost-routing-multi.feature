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

@vhosts-multi
Feature: Gateway vhost routing with multi-domain defaults
  As an API developer
  I want gateway-level multi-domain vhost lists to be honored
  So that APIs route across all configured main and sandbox domains

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Route requests across all configured main and sandbox domains
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: vhost-multi-domains-v1.0
      spec:
        displayName: VHost-Multi-Domains
        version: v1.0
        context: /vhost-multi-domains/$version
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/vhost-multi-domains/v1.0/whoami" to be ready with host "api.wso2.com"

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-domains/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api.foo.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-domains/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-domains/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.foo.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-domains/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.other.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-domains/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "vhost-multi-domains-v1.0"
    Then the response should be successful

  Scenario: API vhost override should bypass multi-domain gateway defaults
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: vhost-multi-override-v1.0
      spec:
        displayName: VHost-Multi-Override
        version: v1.0
        context: /vhost-multi-override/$version
        vhosts:
          main: custom.wso2.com
          sandbox: custom-sandbox.wso2.com
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/vhost-multi-override/v1.0/whoami" to be ready with host "custom.wso2.com"

    When I clear all headers
    And I set request host to "custom.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-override/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "custom-sandbox.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-override/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-override/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "api.foo.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-override/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-override/v1.0/whoami"
    Then the response status code should be 404

    When I clear all headers
    And I set request host to "api-sandbox.foo.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-override/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "vhost-multi-override-v1.0"
    Then the response should be successful

  Scenario: Sentinel vhost resolves to all configured gateway domains
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: vhost-multi-sentinel-v1.0
      spec:
        displayName: VHost-Multi-Sentinel
        version: v1.0
        context: /vhost-multi-sentinel/$version
        vhosts:
          main: _gateway_default_
          sandbox: _gateway_default_
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/vhost-multi-sentinel/v1.0/whoami" to be ready with host "api.wso2.com"

    # Sentinel resolves to *.wso2.com (default) — translator expands to all main domains
    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-sentinel/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api.foo.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-sentinel/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    # Sandbox sentinel resolves to *-sandbox.wso2.com (default) — translator expands to all sandbox domains
    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-sentinel/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.foo.com"
    And I send a GET request to "http://localhost:8080/vhost-multi-sentinel/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    # The sentinel string itself must NOT be used as a hostname — proves resolution occurred
    When I clear all headers
    And I set request host to "_gateway_default_"
    And I send a GET request to "http://localhost:8080/vhost-multi-sentinel/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "vhost-multi-sentinel-v1.0"
    Then the response should be successful
