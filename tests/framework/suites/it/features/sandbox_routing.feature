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

@sandbox-routing
Feature: Sandbox routing
  As an API developer
  I want main and sandbox upstreams to route by host
  So that sandbox traffic can be validated independently

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Route requests to different upstreams using main and sandbox vhosts
    Given I generate a unique value from "sandbox-routing-1" and store it as "apiName1"
    And I generate a unique API context from "/sandbox-routing-1" and store it as "apiContext1"
    And I generate a unique resource name from "sandbox-routing-1-mainHost1" and store it as "mainHost1"
    And I generate a unique resource name from "sandbox-routing-1-sandboxHost1" and store it as "sandboxHost1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName1} |
      | spec | {"displayName":"Env-Routing-API","version":"v1.0","context":"${CTX:apiContext1}/$version","vhosts":{"main":"${CTX:mainHost1}","sandbox":"${CTX:sandboxHost1}"},"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"url":"http://testbench:3000/sandbox"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost1}"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost1}"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost1}"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  Scenario: Route requests with main via ref and sandbox via url
    Given I generate a unique value from "sandbox-routing-2" and store it as "apiName2"
    And I generate a unique API context from "/sandbox-routing-2" and store it as "apiContext2"
    And I generate a unique resource name from "sandbox-routing-2-mainHost2" and store it as "mainHost2"
    And I generate a unique resource name from "sandbox-routing-2-sandboxHost2" and store it as "sandboxHost2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName2} |
      | spec | {"displayName":"Env-Routing-Main-Ref-API","version":"v1.0","context":"${CTX:apiContext2}/$version","vhosts":{"main":"${CTX:mainHost2}","sandbox":"${CTX:sandboxHost2}"},"upstreamDefinitions":[{"name":"main-upstream","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"ref":"main-upstream"},"sandbox":{"url":"http://testbench:3000/sandbox"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost2}"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  Scenario: Route requests with main via url and sandbox via ref
    Given I generate a unique value from "sandbox-routing-3" and store it as "apiName3"
    And I generate a unique API context from "/sandbox-routing-3" and store it as "apiContext3"
    And I generate a unique resource name from "sandbox-routing-3-mainHost3" and store it as "mainHost3"
    And I generate a unique resource name from "sandbox-routing-3-sandboxHost3" and store it as "sandboxHost3"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName3} |
      | spec | {"displayName":"Env-Routing-Sandbox-Ref-API","version":"v1.0","context":"${CTX:apiContext3}/$version","vhosts":{"main":"${CTX:mainHost3}","sandbox":"${CTX:sandboxHost3}"},"upstreamDefinitions":[{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost3}"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost3}"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost3}"
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  Scenario: Deploy API with missing main ref definition should fail
    Given I generate a unique value from "sandbox-routing-4" and store it as "apiName4"
    And I generate a unique API context from "/sandbox-routing-4" and store it as "apiContext4"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName4} |
      | spec | {"displayName":"Env-Routing-Missing-Ref-API","version":"v1.0","context":"${CTX:apiContext4}/$version","upstream":{"main":{"ref":"non-existent-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Referenced upstream definition 'non-existent-upstream' not found"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName4}"
    Then the response status code should be 404

  Scenario: Deploy API with invalid URL in upstreamDefinitions should fail
    Given I generate a unique value from "sandbox-routing-5" and store it as "apiName5"
    And I generate a unique API context from "/sandbox-routing-5" and store it as "apiContext5"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName5} |
      | spec | {"displayName":"Env-Routing-Invalid-Definition-API","version":"v1.0","context":"${CTX:apiContext5}/$version","upstreamDefinitions":[{"name":"invalid-upstream","upstreams":[{"url":"ftp://testbench:3000"}]}],"upstream":{"main":{"ref":"invalid-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "URL must use http or https scheme"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName5}"
    Then the response status code should be 404

  Scenario: Policy effects should apply for sandbox ref routes
    Given I generate a unique value from "sandbox-routing-6" and store it as "apiName6"
    And I generate a unique API context from "/sandbox-routing-6" and store it as "apiContext6"
    And I generate a unique resource name from "sandbox-routing-6-mainHost6" and store it as "mainHost6"
    And I generate a unique resource name from "sandbox-routing-6-sandboxHost6" and store it as "sandboxHost6"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName6} |
      | spec | {"displayName":"Env-Routing-Sandbox-Ref-Policy-API","version":"v1.0","context":"${CTX:apiContext6}/$version","vhosts":{"main":"${CTX:mainHost6}","sandbox":"${CTX:sandboxHost6}"},"upstreamDefinitions":[{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/whoami","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Sandbox-Ref-Policy","value":"applied"}]}}}]}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost6}"
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost6}"
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Sandbox-Ref-Policy" should be "applied"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost6}"
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Sandbox-Ref-Policy" should be "applied"

  Scenario: Route requests with both main and sandbox via ref
    Given I generate a unique value from "sandbox-routing-7" and store it as "apiName7"
    And I generate a unique API context from "/sandbox-routing-7" and store it as "apiContext7"
    And I generate a unique resource name from "sandbox-routing-7-mainHost7" and store it as "mainHost7"
    And I generate a unique resource name from "sandbox-routing-7-sandboxHost7" and store it as "sandboxHost7"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName7} |
      | spec | {"displayName":"Env-Routing-Both-Ref-API","version":"v1.0","context":"${CTX:apiContext7}/$version","vhosts":{"main":"${CTX:mainHost7}","sandbox":"${CTX:sandboxHost7}"},"upstreamDefinitions":[{"name":"main-upstream","upstreams":[{"url":"http://testbench:3000"}]},{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"ref":"main-upstream"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost7}"
    And I send a "GET" request to "${CTX:apiContext7}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost7}"
    And I send a "GET" request to "${CTX:apiContext7}/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost7}"
    And I send a "GET" request to "${CTX:apiContext7}/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/sandbox/whoami"

  Scenario: Sandbox ref with hostRewrite manual should preserve incoming host
    Given I generate a unique value from "sandbox-routing-8" and store it as "apiName8"
    And I generate a unique API context from "/sandbox-routing-8" and store it as "apiContext8"
    And I generate a unique resource name from "sandbox-routing-8-mainHost8" and store it as "mainHost8"
    And I generate a unique resource name from "sandbox-routing-8-sandboxHost8" and store it as "sandboxHost8"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName8} |
      | spec | {"displayName":"Env-Routing-Sandbox-Manual-Host-API","version":"v1.0","context":"${CTX:apiContext8}/$version","vhosts":{"main":"${CTX:mainHost8}","sandbox":"${CTX:sandboxHost8}"},"upstreamDefinitions":[{"name":"sandbox-upstream","basePath":"/anything","upstreams":[{"url":"http://testbench:3002"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-upstream","hostRewrite":"manual"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost8}"
    And I send a "GET" request to "${CTX:apiContext8}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:sandboxHost8}"
    And I send a "GET" request to "${CTX:apiContext8}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "headers.Host" should contain "${CTX:sandboxHost8}"
    And the JSON response field "path" should be "/anything/whoami"

  Scenario: Sandbox ref should honor upstreamDefinitions connect timeout
    Given I generate a unique value from "sandbox-routing-9" and store it as "apiName9"
    And I generate a unique API context from "/sandbox-routing-9" and store it as "apiContext9"
    And I generate a unique resource name from "sandbox-routing-9-mainHost9" and store it as "mainHost9"
    And I generate a unique resource name from "sandbox-routing-9-sandboxHost9" and store it as "sandboxHost9"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName9} |
      | spec | {"displayName":"Env-Routing-Sandbox-Timeout-Ref-API","version":"v1.0","context":"${CTX:apiContext9}/$version","vhosts":{"main":"${CTX:mainHost9}","sandbox":"${CTX:sandboxHost9}"},"upstreamDefinitions":[{"name":"sandbox-timeout-upstream","timeout":{"connect":"6000ms"},"upstreams":[{"url":"http://192.0.2.1:80"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-timeout-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I wait for policy snapshot sync
    And I set request host to "${CTX:mainHost9}"
    And I send a "GET" request to "${CTX:apiContext9}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:sandboxHost9}"
    And I send a "GET" request to "${CTX:apiContext9}/v1.0/whoami"
    Then the response status code should be 503
    And the gateway should have timed out after "6" seconds with status 503

  Scenario: Sandbox ref should work with HTTP upstream
    Given I generate a unique value from "sandbox-routing-10" and store it as "apiName10"
    And I generate a unique API context from "/sandbox-routing-10" and store it as "apiContext10"
    And I generate a unique resource name from "sandbox-routing-10-mainHost10" and store it as "mainHost10"
    And I generate a unique resource name from "sandbox-routing-10-sandboxHost10" and store it as "sandboxHost10"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName10} |
      | spec | {"displayName":"Env-Routing-Sandbox-HTTPS-Ref-API","version":"v1.0","context":"${CTX:apiContext10}/$version","vhosts":{"main":"${CTX:mainHost10}","sandbox":"${CTX:sandboxHost10}"},"upstreamDefinitions":[{"name":"sandbox-http-upstream","basePath":"/openai/v1","upstreams":[{"url":"http://testbench:3008"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-http-upstream"}},"operations":[{"method":"POST","path":"/chat/completions"}]} |
    Then the response should be successful
    And I wait for policy snapshot sync
    And I set header "Content-Type" to "application/json"
    And I set request host to "${CTX:mainHost10}"
    And I send a "POST" request to "${CTX:apiContext10}/v1.0/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "warmup"}]}
      """

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer sk-test-key"
    And I set request host to "${CTX:sandboxHost10}"
    And I send a "POST" request to "${CTX:apiContext10}/v1.0/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [
          {"role": "user", "content": "Hello, how are you?"}
        ]
      }
      """
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "chat.completion"
    And the response body should contain "choices"
    And the JSON response field "object" should be "chat.completion"

  Scenario: Deploy fails when an upstreamDefinitions URL contains a path
    Given I generate a unique value from "sandbox-routing-11" and store it as "apiName11"
    And I generate a unique API context from "/sandbox-routing-11" and store it as "apiContext11"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName11} |
      | spec | {"displayName":"Env-Routing-Sandbox-Multi-URL-Ref-API","version":"v1.0","context":"${CTX:apiContext11}/$version","upstreamDefinitions":[{"name":"sandbox-multi-upstream","upstreams":[{"url":"http://testbench:3000/first"},{"url":"http://testbench:3000/sandbox"}]}],"upstream":{"main":{"ref":"sandbox-multi-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "must not include a path"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName11}"
    Then the response status code should be 404

  Scenario: Path params should route correctly with sandbox ref
    Given I generate a unique value from "sandbox-routing-12" and store it as "apiName12"
    And I generate a unique API context from "/sandbox-routing-12" and store it as "apiContext12"
    And I generate a unique resource name from "sandbox-routing-12-mainHost12" and store it as "mainHost12"
    And I generate a unique resource name from "sandbox-routing-12-sandboxHost12" and store it as "sandboxHost12"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName12} |
      | spec | {"displayName":"Env-Routing-Sandbox-Params-Ref-API","version":"v1.0","context":"${CTX:apiContext12}/$version","vhosts":{"main":"${CTX:mainHost12}","sandbox":"${CTX:sandboxHost12}"},"upstreamDefinitions":[{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/{country}/{city}"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost12}"
    And I send a "GET" request to "${CTX:apiContext12}/v1.0/us/seattle" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost12}"
    And I send a "GET" request to "${CTX:apiContext12}/v1.0/us/seattle"
    Then the response should be successful
    And the JSON response field "path" should be "/us/seattle"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost12}"
    And I send a "GET" request to "${CTX:apiContext12}/v1.0/us/seattle"
    Then the response should be successful
    And the JSON response field "path" should be "/sandbox/us/seattle"

  Scenario: Wildcard route should work with sandbox ref
    Given I generate a unique value from "sandbox-routing-13" and store it as "apiName13"
    And I generate a unique API context from "/sandbox-routing-13" and store it as "apiContext13"
    And I generate a unique resource name from "sandbox-routing-13-mainHost13" and store it as "mainHost13"
    And I generate a unique resource name from "sandbox-routing-13-sandboxHost13" and store it as "sandboxHost13"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName13} |
      | spec | {"displayName":"Env-Routing-Sandbox-Wildcard-Ref-API","version":"v1.0","context":"${CTX:apiContext13}/$version","vhosts":{"main":"${CTX:mainHost13}","sandbox":"${CTX:sandboxHost13}"},"upstreamDefinitions":[{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/assets/*"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost13}"
    And I send a "GET" request to "${CTX:apiContext13}/v1.0/assets/a/b/c" until status 200

    When I clear all headers
    And I set request host to "${CTX:sandboxHost13}"
    And I send a "GET" request to "${CTX:apiContext13}/v1.0/assets/a/b/c"
    Then the response should be successful
    And the response body should contain "/sandbox/"

  Scenario: Deploy should fail when sandbox ref is missing while main is valid
    Given I generate a unique value from "sandbox-routing-14" and store it as "apiName14"
    And I generate a unique API context from "/sandbox-routing-14" and store it as "apiContext14"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName14} |
      | spec | {"displayName":"Env-Routing-Missing-Sandbox-Ref-API","version":"v1.0","context":"${CTX:apiContext14}/$version","upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"missing-sandbox-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Referenced upstream definition 'missing-sandbox-upstream' not found"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName14}"
    Then the response status code should be 404

  Scenario: Deploy should fail for duplicate upstream definition names used by refs
    Given I generate a unique value from "sandbox-routing-15" and store it as "apiName15"
    And I generate a unique API context from "/sandbox-routing-15" and store it as "apiContext15"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName15} |
      | spec | {"displayName":"Env-Routing-Duplicate-Upstream-Def-API","version":"v1.0","context":"${CTX:apiContext15}/$version","upstreamDefinitions":[{"name":"duplicate-upstream","upstreams":[{"url":"http://testbench:3000"}]},{"name":"duplicate-upstream","upstreams":[{"url":"http://testbench:3000/sandbox"}]}],"upstream":{"main":{"ref":"duplicate-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Duplicate upstream definition name 'duplicate-upstream'"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName15}"
    Then the response status code should be 404

  Scenario: Policy effects should apply for main ref and sandbox ref routes
    Given I generate a unique value from "sandbox-routing-16" and store it as "apiName16"
    And I generate a unique API context from "/sandbox-routing-16" and store it as "apiContext16"
    And I generate a unique resource name from "sandbox-routing-16-mainHost16" and store it as "mainHost16"
    And I generate a unique resource name from "sandbox-routing-16-sandboxHost16" and store it as "sandboxHost16"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName16} |
      | spec | {"displayName":"Env-Routing-Both-Ref-Policy-API","version":"v1.0","context":"${CTX:apiContext16}/$version","vhosts":{"main":"${CTX:mainHost16}","sandbox":"${CTX:sandboxHost16}"},"upstreamDefinitions":[{"name":"main-upstream","upstreams":[{"url":"http://testbench:3000"}]},{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"ref":"main-upstream"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/whoami","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Both-Ref-Policy","value":"applied"}]}}}]}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost16}"
    And I send a "GET" request to "${CTX:apiContext16}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost16}"
    And I send a "GET" request to "${CTX:apiContext16}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Both-Ref-Policy" should be "applied"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost16}"
    And I send a "GET" request to "${CTX:apiContext16}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Both-Ref-Policy" should be "applied"

  Scenario: Policy effects should apply for main ref and sandbox url routes
    Given I generate a unique value from "sandbox-routing-17" and store it as "apiName17"
    And I generate a unique API context from "/sandbox-routing-17" and store it as "apiContext17"
    And I generate a unique resource name from "sandbox-routing-17-mainHost17" and store it as "mainHost17"
    And I generate a unique resource name from "sandbox-routing-17-sandboxHost17" and store it as "sandboxHost17"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName17} |
      | spec | {"displayName":"Env-Routing-Main-Ref-Policy-API","version":"v1.0","context":"${CTX:apiContext17}/$version","vhosts":{"main":"${CTX:mainHost17}","sandbox":"${CTX:sandboxHost17}"},"upstreamDefinitions":[{"name":"main-upstream","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"ref":"main-upstream"},"sandbox":{"url":"http://testbench:3000/sandbox"}},"operations":[{"method":"GET","path":"/whoami","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Main-Ref-Policy","value":"applied"}]}}}]}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost17}"
    And I send a "GET" request to "${CTX:apiContext17}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost17}"
    And I send a "GET" request to "${CTX:apiContext17}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Main-Ref-Policy" should be "applied"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost17}"
    And I send a "GET" request to "${CTX:apiContext17}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Main-Ref-Policy" should be "applied"

  Scenario: Requests to sandbox vhost are rejected when API has no sandbox upstream
    Given I generate a unique value from "sandbox-routing-18" and store it as "apiName18"
    And I generate a unique API context from "/sandbox-routing-18" and store it as "apiContext18"
    And I generate a unique resource name from "sandbox-routing-18-mainHost18" and store it as "mainHost18"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName18} |
      | spec | {"displayName":"Env-Routing-No-Sandbox-API","version":"v1.0","context":"${CTX:apiContext18}/$version","vhosts":{"main":"${CTX:mainHost18}"},"upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost18}"
    And I send a "GET" request to "${CTX:apiContext18}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost18}"
    And I send a "GET" request to "${CTX:apiContext18}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "no-sandbox-sandbox.local"
    And I send a "GET" request to "${CTX:apiContext18}/v1.0/whoami"
    Then the response status code should be 404

  Scenario: Deploy API with invalid URL scheme in direct sandbox url should fail
    Given I generate a unique value from "sandbox-routing-19" and store it as "apiName19"
    And I generate a unique API context from "/sandbox-routing-19" and store it as "apiContext19"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName19} |
      | spec | {"displayName":"Env-Routing-Invalid-Sandbox-URL-API","version":"v1.0","context":"${CTX:apiContext19}/$version","upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"url":"ftp://testbench:3000"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Upstream URL must use http or https scheme"
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName19}"
    Then the response status code should be 404

  Scenario: Policy effects should apply for main url and sandbox url routes
    Given I generate a unique value from "sandbox-routing-20" and store it as "apiName20"
    And I generate a unique API context from "/sandbox-routing-20" and store it as "apiContext20"
    And I generate a unique resource name from "sandbox-routing-20-mainHost20" and store it as "mainHost20"
    And I generate a unique resource name from "sandbox-routing-20-sandboxHost20" and store it as "sandboxHost20"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName20} |
      | spec | {"displayName":"Env-Routing-Both-URL-Policy-API","version":"v1.0","context":"${CTX:apiContext20}/$version","vhosts":{"main":"${CTX:mainHost20}","sandbox":"${CTX:sandboxHost20}"},"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"url":"http://testbench:3000/sandbox"}},"operations":[{"method":"GET","path":"/whoami","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Both-URL-Policy","value":"applied"}]}}}]}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost20}"
    And I send a "GET" request to "${CTX:apiContext20}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost20}"
    And I send a "GET" request to "${CTX:apiContext20}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Both-URL-Policy" should be "applied"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost20}"
    And I send a "GET" request to "${CTX:apiContext20}/v1.0/whoami"
    Then the response should be successful
    And the response header "X-Both-URL-Policy" should be "applied"

  Scenario: Sandbox routing becomes available after redeploying API with sandbox upstream added
    Given I generate a unique value from "sandbox-routing-21" and store it as "apiName21"
    And I generate a unique API context from "/sandbox-routing-21" and store it as "apiContext21"
    And I generate a unique resource name from "sandbox-routing-21-mainHost21" and store it as "mainHost21"
    And I generate a unique resource name from "sandbox-routing-21-sandboxHost21" and store it as "sandboxHost21"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName21} |
      | spec | {"displayName":"Env-Routing-Redeploy-Sandbox-API","version":"v1.0","context":"${CTX:apiContext21}/$version","vhosts":{"main":"${CTX:mainHost21}"},"upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost21}"
    And I send a "GET" request to "${CTX:apiContext21}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost21}"
    And I send a "GET" request to "${CTX:apiContext21}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "${CTX:sandboxHost21}"
    And I send a "GET" request to "${CTX:apiContext21}/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I update API "${CTX:apiName21}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName21} |
      | spec | {"displayName":"Env-Routing-Redeploy-Sandbox-API","version":"v1.0","context":"${CTX:apiContext21}/$version","vhosts":{"main":"${CTX:mainHost21}","sandbox":"${CTX:sandboxHost21}"},"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"url":"http://testbench:3000/sandbox"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:sandboxHost21}"
    And I send a "GET" request to "${CTX:apiContext21}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:sandboxHost21}"
    And I send a "GET" request to "${CTX:apiContext21}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  Scenario: Sandbox routes use global default vhost when vhosts.sandbox is omitted
    Given I generate a unique value from "sandbox-routing-22" and store it as "apiName22"
    And I generate a unique API context from "/sandbox-routing-22" and store it as "apiContext22"
    And I generate a unique resource name from "sandbox-routing-22-mainHost22" and store it as "mainHost22"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName22} |
      | spec | {"displayName":"Env-Routing-Sandbox-Vhost-Fallback-API","version":"v1.0","context":"${CTX:apiContext22}/$version","vhosts":{"main":"${CTX:mainHost22}"},"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"url":"http://testbench:3000/sandbox"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost22}"
    And I send a "GET" request to "${CTX:apiContext22}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost22}"
    And I send a "GET" request to "${CTX:apiContext22}/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"
    And I set request host to "sandbox-sb-vhost-fallback.local"
    And I send a "GET" request to "${CTX:apiContext22}/v1.0/whoami" until status 200
    When I clear all headers
    And I set request host to "sandbox-sb-vhost-fallback.local"
    And I send a "GET" request to "${CTX:apiContext22}/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  Scenario: Requests to global default sandbox vhost are rejected when API has no sandbox upstream
    Given I generate a unique value from "sandbox-routing-23" and store it as "apiName23"
    And I generate a unique API context from "/sandbox-routing-23" and store it as "apiContext23"
    And I generate a unique resource name from "sandbox-routing-23-mainHost23" and store it as "mainHost23"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName23} |
      | spec | {"displayName":"Env-Routing-No-Sandbox-Global-Vhost-API","version":"v1.0","context":"${CTX:apiContext23}/$version","vhosts":{"main":"${CTX:mainHost23}"},"upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost23}"
    And I send a "GET" request to "${CTX:apiContext23}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:mainHost23}"
    And I send a "GET" request to "${CTX:apiContext23}/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"
    When I clear all headers
    And I set request host to "sandbox-no-sb-global.local"
    And I send a "GET" request to "${CTX:apiContext23}/v1.0/whoami"
    Then the response status code should be 404
