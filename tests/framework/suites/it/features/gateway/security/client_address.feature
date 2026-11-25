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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# CLADR-001 and CLADR-002 run in the gateway-client-address block, which sets
# router.http_listener.xff_num_trusted_hops to 0: the client address is the connection's peer and
# every X-Forwarded-For entry a caller sends is untrusted. CLADR-003 and CLADR-004 (@trusted-hops-unset)
# run in gateway-core, which leaves it unset.
@client-address @CLADR
Feature: Client address
  As an API developer
  I want the gateway to decide the client address itself
  So that a caller cannot choose the address my backend, the logs and IP rate limits see

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @CLADR-001
  Scenario: X-Forwarded-For does not reach the backend
    Given I generate a unique value from "client-address" and store it as "apiName"
    And I generate a unique API context from "/client-address" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:apiName}                 |
      | spec.displayName       | Client-Address-API             |
      | spec.version           | v1.0                           |
      | spec.context           | ${CTX:apiContext}/$version     |
      | spec.upstream.main.url | http://testbench:3002          |
      | spec.operations        | [{"method":"GET","path":"/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe" until status 200

    When I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200
    And the response should not contain echoed header "X-Forwarded-For"

    When I set header "X-Forwarded-For" to "203.0.113.77"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200
    And the response should not contain echoed header "X-Forwarded-For"

    When I set header "X-Forwarded-For" to "203.0.113.77, 198.51.100.9"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200
    And the response should not contain echoed header "X-Forwarded-For"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  @CLADR-002
  Scenario: A per-IP rate limit holds when each request sets a different X-Forwarded-For
    Given I generate a unique value from "client-address-ratelimit" and store it as "apiName"
    And I generate a unique API context from "/client-address-ratelimit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:apiName}                 |
      | spec.displayName       | Client-Address-Ratelimit-API   |
      | spec.version           | v1.0                           |
      | spec.context           | ${CTX:apiContext}/$version     |
      | spec.upstream.main.url | http://testbench:3002          |
      | spec.operations        | [{"method":"GET","path":"/*"}] |
      | spec.policies          | [{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"per-ip","limits":[{"limit":3,"duration":"1h"}],"keyExtraction":[{"type":"ip"}]}]}}] |
    Then the response should be successful
    # The first 200 is the first request the quota counts: earlier attempts get 404 from a
    # gateway that has no route yet.
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe" until status 200

    When I set header "X-Forwarded-For" to "198.51.100.1"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200

    When I set header "X-Forwarded-For" to "198.51.100.2"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200

    When I set header "X-Forwarded-For" to "198.51.100.3"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  @CLADR-003 @trusted-hops-unset
  Scenario: X-Forwarded-For passes through unchanged when no trusted hops are set
    Given I generate a unique value from "client-address-unset" and store it as "apiName"
    And I generate a unique API context from "/client-address-unset" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:apiName}                 |
      | spec.displayName       | Client-Address-Unset-API       |
      | spec.version           | v1.0                           |
      | spec.context           | ${CTX:apiContext}/$version     |
      | spec.upstream.main.url | http://testbench:3002          |
      | spec.operations        | [{"method":"GET","path":"/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe" until status 200

    When I set header "X-Forwarded-For" to "203.0.113.77, 198.51.100.9"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200
    And the response should contain echoed header "X-Forwarded-For" with value "203.0.113.77, 198.51.100.9"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Unset, an ip-keyed quota uses the first X-Forwarded-For entry, which the caller chooses: the
  # documented reason to set xff_num_trusted_hops for IP-based quotas.
  @CLADR-004 @trusted-hops-unset
  Scenario: An ip-keyed quota follows the first X-Forwarded-For entry when no trusted hops are set
    Given I generate a unique value from "client-address-unset-ratelimit" and store it as "apiName"
    And I generate a unique API context from "/client-address-unset-ratelimit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Client-Address-Unset-Ratelimit-API |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/*"}]     |
      | spec.policies          | [{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"per-ip","limits":[{"limit":2,"duration":"1h"}],"keyExtraction":[{"type":"ip"}]}]}}] |
    Then the response should be successful
    # The readiness probe sends no X-Forwarded-For, so it is counted against a different key.
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe" until status 200

    When I set header "X-Forwarded-For" to "198.51.100.41"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200

    When I set header "X-Forwarded-For" to "198.51.100.41"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200

    # Same first entry, different later entry: the same bucket, now exhausted.
    When I set header "X-Forwarded-For" to "198.51.100.41, 203.0.113.5"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 429

    # A different first entry: a new bucket.
    When I set header "X-Forwarded-For" to "198.51.100.42, 198.51.100.41"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/probe"
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
