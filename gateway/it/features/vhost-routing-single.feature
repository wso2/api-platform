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

@vhosts-single
Feature: Gateway vhost routing with single-domain defaults
  As an API developer
  I want gateway-level single-domain defaults to apply when API vhosts are omitted
  So that main and sandbox traffic route on predictable host patterns

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Route requests using single-domain gateway defaults
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: vhost-single-defaults-v1.0
      spec:
        displayName: VHost-Single-Defaults
        version: v1.0
        context: /vhost-single-defaults/$version
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
    And I wait for the endpoint "http://localhost:8080/vhost-single-defaults/v1.0/whoami" to be ready with host "api.wso2.com"

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-single-defaults/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "api-sandbox.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-single-defaults/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.other.com"
    And I send a GET request to "http://localhost:8080/vhost-single-defaults/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "vhost-single-defaults-v1.0"
    Then the response should be successful

  Scenario: API vhost override should take precedence over gateway defaults
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: vhost-single-override-v1.0
      spec:
        displayName: VHost-Single-Override
        version: v1.0
        context: /vhost-single-override/$version
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
    And I wait for the endpoint "http://localhost:8080/vhost-single-override/v1.0/whoami" to be ready with host "custom.wso2.com"

    When I clear all headers
    And I set request host to "custom.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-single-override/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "custom-sandbox.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-single-override/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    When I clear all headers
    And I set request host to "api.wso2.com"
    And I send a GET request to "http://localhost:8080/vhost-single-override/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "vhost-single-override-v1.0"
    Then the response should be successful
