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

@prompt-compressor
Feature: Prompt compressor policy
  As an API developer
  I want to compress prompts sent to LLMs
  So that I can reduce token usage and cost
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deterministic compression produces exact expected output
    Given I generate a unique value from "pc-deterministic" and store it as "apiName"
    And I generate a unique API version from "pc-deterministic" and store it as "apiVersion"
    And I generate a unique API context from "/pc-deterministic" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"Artificial intelligence and machine learning have transformed the technology landscape significantly over the past decade. Deep learning models now power everything from natural language processing to computer vision applications. The advancement of transformer architectures has enabled breakthrough capabilities in text generation and understanding."}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Artificial intelligence machine transformed technology landscape decade. Deep models everything natural language processing computer applications. advancement transformer enabled breakthrough capabilities text generation understanding."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Compress a long prompt using ratio mode
    Given I generate a unique value from "pc-ratio" and store it as "apiName"
    And I generate a unique API version from "pc-ratio" and store it as "apiVersion"
    And I generate a unique API context from "/pc-ratio" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response should be valid JSON
    # Full length is 1013, 0.5 ratio should make it significantly less. Let's say < 600
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100
    And the response body should not contain "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Short prompt remains unchanged in ratio mode when no compression is needed
    Given I generate a unique value from "pc-ratio-short" and store it as "apiName"
    And I generate a unique API version from "pc-ratio-short" and store it as "apiVersion"
    And I generate a unique API context from "/pc-ratio-short" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.95}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"Hi"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Hi"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A ratio value of 1.0 or greater skips compression
    Given I generate a unique value from "pc-ratio-one" and store it as "apiName"
    And I generate a unique API version from "pc-ratio-one" and store it as "apiVersion"
    And I generate a unique API context from "/pc-ratio-one" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":1.0}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should contain "The deployment pipeline for the cloud-native application"
    And the JSON response field "json.messages[0].content" should contain "performance metrics across all environments."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Compress using token mode with a target below the estimated count
    Given I generate a unique value from "pc-token" and store it as "apiName"
    And I generate a unique API version from "pc-token" and store it as "apiVersion"
    And I generate a unique API context from "/pc-token" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"token","value":50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response should be valid JSON
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Token mode with a target at or above the estimated count skips compression
    Given I generate a unique value from "pc-token-skip" and store it as "apiName"
    And I generate a unique API version from "pc-token-skip" and store it as "apiVersion"
    And I generate a unique API context from "/pc-token-skip" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"token","value":9999}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"Short text for skip"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Short text for skip"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A custom jsonPath targets a flat field
    Given I generate a unique value from "pc-jsonpath-flat" and store it as "apiName"
    And I generate a unique API version from "pc-jsonpath-flat" and store it as "apiVersion"
    And I generate a unique API context from "/pc-jsonpath-flat" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.prompt","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments.","model":"gpt-4"}
      """
    Then the response should be valid JSON
    And the JSON response string field "json.prompt" should have length less than 850
    And the JSON response string field "json.prompt" should have length greater than 100
    And the JSON response field "json.model" should be "gpt-4"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A nested jsonPath is compressed while sibling fields are preserved
    Given I generate a unique value from "pc-jsonpath-nested" and store it as "apiName"
    And I generate a unique API version from "pc-jsonpath-nested" and store it as "apiVersion"
    And I generate a unique API context from "/pc-jsonpath-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.request.data.prompt","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"request":{"data":{"prompt":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."},"meta":"keep"}}
      """
    Then the response should be valid JSON
    And the JSON response string field "json.request.data.prompt" should have length less than 850
    And the JSON response string field "json.request.data.prompt" should have length greater than 100
    And the JSON response field "json.request.meta" should be "keep"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A negative array index jsonPath targets the last message
    Given I generate a unique value from "pc-jsonpath-neg" and store it as "apiName"
    And I generate a unique API version from "pc-jsonpath-neg" and store it as "apiVersion"
    And I generate a unique API context from "/pc-jsonpath-neg" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[-1].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"First"},{"content":"Second"},{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response should be valid JSON
    And the JSON response string field "json.messages[2].content" should have length less than 850
    And the JSON response string field "json.messages[2].content" should have length greater than 100
    And the JSON response field "json.messages[0].content" should be "First"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A jsonPath pointing to a non-string value is left untouched
    Given I generate a unique value from "pc-jsonpath-nonstring" and store it as "apiName"
    And I generate a unique API version from "pc-jsonpath-nonstring" and store it as "apiVersion"
    And I generate a unique API context from "/pc-jsonpath-nonstring" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":12345}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be 12345

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A jsonPath pointing to a missing field passes the request through unchanged
    Given I generate a unique value from "pc-jsonpath-missing" and store it as "apiName"
    And I generate a unique API version from "pc-jsonpath-missing" and store it as "apiVersion"
    And I generate a unique API context from "/pc-jsonpath-missing" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.nonexistent.field","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"Hello"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Hello"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Only the tagged region is compressed, untagged text is preserved
    Given I generate a unique value from "pc-tags" and store it as "apiName"
    And I generate a unique API version from "pc-tags" and store it as "apiVersion"
    And I generate a unique API context from "/pc-tags" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Instructions: Answer concisely.\n\n<APIP-COMPRESS>The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments.</APIP-COMPRESS>\n\nQuestion: What changed?"}]}
      """
    Then the response should be valid JSON
    And the response body should contain "Instructions: Answer concisely."
    And the response body should contain "Question: What changed?"
    And the response body should not contain "<APIP-COMPRESS>"
    And the response body should not contain "</APIP-COMPRESS>"
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple tagged regions in one prompt are each compressed
    Given I generate a unique value from "pc-multi-tags" and store it as "apiName"
    And I generate a unique API version from "pc-multi-tags" and store it as "apiVersion"
    And I generate a unique API context from "/pc-multi-tags" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Part 1:\n<APIP-COMPRESS>The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed.</APIP-COMPRESS>\n\nMiddle Preserved\n\nPart 2:\n<APIP-COMPRESS>The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments.</APIP-COMPRESS>"}]}
      """
    Then the response should be valid JSON
    And the response body should contain "Part 1"
    And the response body should contain "Middle Preserved"
    And the response body should contain "Part 2"
    And the response body should not contain "<APIP-COMPRESS>"
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An unpaired closing tag is stripped
    Given I generate a unique value from "pc-orphan-tag" and store it as "apiName"
    And I generate a unique API version from "pc-orphan-tag" and store it as "apiVersion"
    And I generate a unique API context from "/pc-orphan-tag" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"This is a prompt with an orphan tag</APIP-COMPRESS>"}]}
      """
    Then the response body should not contain "</APIP-COMPRESS>"
    And the JSON response field "json.messages[0].content" should be "This is a prompt with an orphan tag"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple rules evaluate and the smallest matching limit applies
    Given I generate a unique value from "pc-multi-rule" and store it as "apiName"
    And I generate a unique API version from "pc-multi-rule" and store it as "apiVersion"
    And I generate a unique API context from "/pc-multi-rule" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":100,"type":"ratio","value":0.90},{"upperTokenLimit":500,"type":"ratio","value":0.50},{"upperTokenLimit":-1,"type":"ratio","value":0.30}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # 100 tokens ~ 400 chars. Send short text to match rule 1 (ratio 0.90).
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"This is a moderately short text that should be under 100 tokens. The policy will evaluate the first rule with upperTokenLimit: 100 and apply a ratio of 0.90. Because 0.90 doesn't cause much compression, it will likely be forwarded unchanged due to NegativeGainError."}]}
      """
    Then the JSON response string field "json.messages[0].content" should have length less than 320
    And the JSON response string field "json.messages[0].content" should have length greater than 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple rules evaluate and the fallback rule applies for long input
    Given I generate a unique value from "pc-multi-rule-fallback" and store it as "apiName"
    And I generate a unique API version from "pc-multi-rule-fallback" and store it as "apiVersion"
    And I generate a unique API context from "/pc-multi-rule-fallback" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":100,"type":"ratio","value":0.90},{"upperTokenLimit":500,"type":"ratio","value":0.50},{"upperTokenLimit":-1,"type":"ratio","value":0.30}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # ~2684 chars (~650 tokens), so this exceeds the 500-token rule and falls back to ratio 0.30.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments. The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments. The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response should be valid JSON
    # 2684 total length, ratio 0.3 should give < 1200
    And the JSON response string field "json.messages[0].content" should have length less than 1200
    And the JSON response string field "json.messages[0].content" should have length greater than 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Invalid rules are dropped while valid rules still apply
    Given I generate a unique value from "pc-invalid-rules" and store it as "apiName"
    And I generate a unique API version from "pc-invalid-rules" and store it as "apiVersion"
    And I generate a unique API context from "/pc-invalid-rules" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-5,"type":"ratio","value":0.50},{"upperTokenLimit":-1,"type":"ratio","value":0},{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response should be valid JSON
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An empty request body passes through unchanged
    Given I generate a unique value from "pc-empty-body" and store it as "apiName"
    And I generate a unique API version from "pc-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/pc-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      """
    Then the JSON response field "data" should be ""

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A non-JSON request body passes through unchanged
    Given I generate a unique value from "pc-non-json" and store it as "apiName"
    And I generate a unique API version from "pc-non-json" and store it as "apiVersion"
    And I generate a unique API context from "/pc-non-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      just some text
      """
    Then the JSON response field "data" should be "just some text"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An invalid JSON request body passes through unchanged
    Given I generate a unique value from "pc-invalid-json" and store it as "apiName"
    And I generate a unique API version from "pc-invalid-json" and store it as "apiVersion"
    And I generate a unique API context from "/pc-invalid-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.50}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {invalid json}
      """
    Then the JSON response field "data" should be "{invalid json}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Lifecycle operations add, update, and remove the policy
    Given I generate a unique value from "pc-lifecycle" and store it as "apiName"
    And I generate a unique API version from "pc-lifecycle" and store it as "apiVersion"
    And I generate a unique API context from "/pc-lifecycle" and store it as "apiContext"
    # Phase 1: deploy without the policy
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat"},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response body should contain "The deployment pipeline for the cloud-native application underwent significant changes"
    And the response body should contain "performance metrics across all environments"

    # Phase 2: add the policy (aggressive ratio 0.30)
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.30}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # A status/snapshot-sync poll alone cannot tell whether THIS route's policy chain has
    # actually refreshed: an update that only changes an existing route's policy content (no
    # new route, no path change) can still answer 200 from the pre-update config, and policy
    # snapshot sync's version numbers were observed to agree before the compression itself was
    # live. Polling on the compressed length is what makes this deterministic instead of racy.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until the JSON field "json.messages[0].content" has length less than 850 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the JSON response string field "json.messages[0].content" should have length greater than 100

    # Phase 3: update the policy (high ratio 0.95 -> skip compression)
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-compressor","version":"v0","params":{"jsonPath":"$.messages[0].content","rules":[{"upperTokenLimit":-1,"type":"ratio","value":0.95}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # Same class of race as Phase 2: poll on length until this route has moved off the
    # Phase-2 compressed (ratio 0.30) config rather than trusting status/snapshot-sync alone.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until the JSON field "json.messages[0].content" has length greater than 900 with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the JSON response field "json.messages[0].content" should contain "deployment pipeline"
    And the JSON response field "json.messages[0].content" should contain "microservices-based approach"
    And the JSON response field "json.messages[0].content" should contain "authentication module"

    # Phase 4: remove the policy entirely
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat"},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # ratio 0.95 can still drop a small word while leaving overall length almost unchanged, so a
    # length-based poll cannot tell "policy truly removed" from "still on the Phase-3 config" -
    # poll on the exact leading phrase instead, which only an unmodified passthrough preserves.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until the response body contains "The deployment pipeline for the cloud-native application underwent significant changes" with body:
      """
      {"messages":[{"content":"The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}]}
      """
    Then the response body should contain "performance metrics across all environments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
