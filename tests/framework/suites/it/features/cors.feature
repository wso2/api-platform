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

@cors
Feature: CORS policy behavior
  As an API developer
  I want CORS policy decisions to control cross-origin requests
  So that allowed requests receive the correct headers and disallowed requests do not

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Preflight request allows configured origin, methods, and headers
    Given I generate a unique value from "cors-1" and store it as "apiName1"
    And I generate a unique API context from "/cors-1" and store it as "apiContext1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName1}                  |
      | spec.displayName       | CORS Preflight API                |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext1}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com","http://localhost:5000"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Content-Type-Options"]}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"OPTIONS","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:apiContext1}/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    And the response header "Access-Control-Allow-Methods" should contain "GET"
    And the response header "Access-Control-Allow-Methods" should contain "POST"
    And the response header "Access-Control-Allow-Headers" should contain "Content-Type"
    When I delete the API "${CTX:apiName1}"
    Then the response should be successful


  Scenario: Preflight request fails for disallowed origin
    Given I generate a unique value from "cors-2" and store it as "apiName2"
    And I generate a unique API context from "/cors-2" and store it as "apiContext2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName2}                  |
      | spec.displayName       | CORS Preflight API                |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext2}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com","http://localhost:5000"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Content-Type-Options"]}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"OPTIONS","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://evil.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:apiContext2}/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist
    When I delete the API "${CTX:apiName2}"
    Then the response should be successful


  Scenario: Preflight request fails for disallowed method
    Given I generate a unique value from "cors-3" and store it as "apiName3"
    And I generate a unique API context from "/cors-3" and store it as "apiContext3"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName3}                  |
      | spec.displayName       | CORS Preflight API                |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext3}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com","http://localhost:5000"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Content-Type-Options"]}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"OPTIONS","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "PUT"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:apiContext3}/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist
    When I delete the API "${CTX:apiName3}"
    Then the response should be successful


  Scenario: Preflight request fails for disallowed header
    Given I generate a unique value from "cors-4" and store it as "apiName4"
    And I generate a unique API context from "/cors-4" and store it as "apiContext4"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName4}                  |
      | spec.displayName       | CORS Preflight API                |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext4}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com","http://localhost:5000"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Content-Type-Options"]}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"OPTIONS","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Authorization"
    And I send a "OPTIONS" request to "${CTX:apiContext4}/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist
    When I delete the API "${CTX:apiName4}"
    Then the response should be successful


  Scenario: Simple GET from allowed origin gets CORS response headers
    Given I generate a unique value from "cors-5" and store it as "apiName5"
    And I generate a unique API context from "/cors-5" and store it as "apiContext5"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName5}                  |
      | spec.displayName       | CORS Simple Request API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext5}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"],"allowCredentials":true}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:apiContext5}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    When I delete the API "${CTX:apiName5}"
    Then the response should be successful


  Scenario: Simple GET from disallowed origin has upstream CORS headers stripped
    Given I generate a unique value from "cors-6" and store it as "apiName6"
    And I generate a unique API context from "/cors-6" and store it as "apiContext6"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName6}                  |
      | spec.displayName       | CORS Simple Request API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext6}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"],"allowCredentials":true}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://evil.com"
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should not exist
    When I delete the API "${CTX:apiName6}"
    Then the response should be successful


  Scenario: Simple GET without Origin header gets no CORS headers
    Given I generate a unique value from "cors-7" and store it as "apiName7"
    And I generate a unique API context from "/cors-7" and store it as "apiContext7"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName7}                  |
      | spec.displayName       | CORS Simple Request API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext7}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"],"allowCredentials":true}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext7}/v1.0/test/test" until status 200

    When I send a "GET" request to "${CTX:apiContext7}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should not exist
    When I delete the API "${CTX:apiName7}"
    Then the response should be successful


  Scenario: Simple GET from allowed origin gets Vary header
    Given I generate a unique value from "cors-8" and store it as "apiName8"
    And I generate a unique API context from "/cors-8" and store it as "apiContext8"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName8}                  |
      | spec.displayName       | CORS Simple Request API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext8}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"],"allowCredentials":true}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext8}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:apiContext8}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Vary" should be "Origin"
    When I delete the API "${CTX:apiName8}"
    Then the response should be successful


  Scenario: Simple GET from allowed origin gets Allow-Credentials header
    Given I generate a unique value from "cors-9" and store it as "apiName9"
    And I generate a unique API context from "/cors-9" and store it as "apiContext9"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName9}                  |
      | spec.displayName       | CORS Simple Request API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext9}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"],"allowCredentials":true}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext9}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:apiContext9}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Credentials" should be "true"
    When I delete the API "${CTX:apiName9}"
    Then the response should be successful


  Scenario: Simple GET from allowed origin gets Expose-Headers
    Given I generate a unique value from "cors-10" and store it as "apiName10"
    And I generate a unique API context from "/cors-10" and store it as "apiContext10"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName10}                 |
      | spec.displayName       | CORS Simple Request API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext10}/$version     |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com","https://*.example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"],"allowCredentials":true}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext10}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:apiContext10}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Expose-Headers" should be "X-Custom-Header"
    When I delete the API "${CTX:apiName10}"
    Then the response should be successful


  Scenario: Simple GET with wildcard origin gets CORS headers
    Given I generate a unique value from "cors-11" and store it as "apiName11"
    And I generate a unique API context from "/cors-11" and store it as "apiContext11"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName11}                 |
      | spec.displayName       | CORS Simple Wildcard API          |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext11}/$version     |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["*"],"allowedMethods":["GET","POST"]}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext11}/v1.0/test/test" until status 200

    When I set header "Origin" to "http://anysite.com"
    And I send a "GET" request to "${CTX:apiContext11}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "*"
    When I delete the API "${CTX:apiName11}"
    Then the response should be successful
