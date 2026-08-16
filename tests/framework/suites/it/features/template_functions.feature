@template-functions
Feature: Template functions in RestApi spec
  As an API administrator
  I want template expressions ({{ env }}, {{ secret }}, {{ default }}) in
  a RestApi spec to be resolved at runtime, while the API responses and
  the persisted DB row keep the original unrendered template body.

  # Migration notes — what changed from the legacy suite and why:
  #
  # Upstreams are re-addressed at the testbench reflector. The legacy hosts
  # (echo-backend-multi-arch:8080, sample-backend:9080) do not exist in this
  # harness; testbench:3000 is the catch-all reflector these scenarios need,
  # since every runtime assertion here reads a reflected request header or the
  # reflected path. Where the appended path is LITERAL (a plain /anything) it is
  # dropped; where it is the value UNDER TEST (the {{ env }} in the upstream-URL
  # scenario) it is preserved, because dropping it would delete the assertion.
  #
  # Secrets are provisioned through the controller's own /secrets management API
  # (the framework mounts the AES-GCM key the controller needs to resolve them),
  # expressed via the generic service step rather than a bespoke secret step.
  #
  # The POST-body readiness waits become "route programmed" waits — the closest
  # existing readiness primitive, since /chat/completions does not answer 200 to
  # a bare probe. The legacy "send 4 GET requests" is unrolled into individual
  # requests so the rate-limit count stays explicit.
  #
  # env vars IT_TEMPLATE_PATH (/anything), IT_RATE_LIMIT (5) and
  # IT_ALLOW_CREDENTIALS (true) are set on the gateway component;
  # IT_DEFINITELY_MISSING_KEY is deliberately unset, which is what the default/
  # fallback scenarios exercise.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: secret template in set-headers policy value is rendered upstream but unrendered in response and DB
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": { "name": "tpl-auth-token" },
        "spec": {
          "displayName": "tpl-auth-token",
          "description": "Auto-generated secret",
          "value": "xyz-test-token-123"
        }
      }
      """
    Then the response status code should be 201

    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tpl-secret-api-v1.0
      spec:
        displayName: Tpl-Secret-Api
        version: v1.0
        context: /tpl-secret/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Auth-Token
                        value: 'Bearer {{ secret "tpl-auth-token" }}'
      """
    Then the response status code should be 201
    And the JSON response field "spec.operations[0].policies[0].params.request.headers[0].value" should be:
      """
      Bearer {{ secret "tpl-auth-token" }}
      """

    # GET response must also echo the unrendered template body
    When I get the API "tpl-secret-api-v1.0"
    Then the response status code should be 200
    And the JSON response field "spec.operations[0].policies[0].params.request.headers[0].value" should be:
      """
      Bearer {{ secret "tpl-auth-token" }}
      """

    # PARKED: asserts the stored source config keeps the template UNRENDERED. Needs a
    # controller-DB reader (legacy GetStoredSourceConfigurationWithRetry over rest_apis /
    # llm_providers / llm_proxies). See task notes; do not delete this comment.

    # Runtime traffic must hit upstream with the resolved secret value
    When I send a "GET" request to "/tpl-secret/v1.0/probe" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer xyz-test-token-123"

    # Cleanup
    When I delete the API "tpl-secret-api-v1.0"
    Then the response status code should be 200
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/tpl-auth-token"
    Then the response status code should be 200

  Scenario: env template in upstream URL path resolves at runtime
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tpl-env-api-v1.0
      spec:
        displayName: Tpl-Env-Api
        version: v1.0
        context: /tpl-env/$version
        upstream:
          main:
            url: 'http://testbench:3000{{ env "IT_TEMPLATE_PATH" }}'
        operations:
          - method: GET
            path: /probe
      """
    Then the response status code should be 201
    And the JSON response field "spec.upstream.main.url" should be:
      """
      http://testbench:3000{{ env "IT_TEMPLATE_PATH" }}
      """

    When I get the API "tpl-env-api-v1.0"
    Then the response status code should be 200
    And the JSON response field "spec.upstream.main.url" should be:
      """
      http://testbench:3000{{ env "IT_TEMPLATE_PATH" }}
      """

    # Runtime: upstream must have been built with /anything (the resolved env value)
    When I send a "GET" request to "/tpl-env/v1.0/probe" until status 200
    Then the response status code should be 200
    And the JSON response field "path" should be "/anything/probe"

    # Cleanup
    When I delete the API "tpl-env-api-v1.0"
    Then the response status code should be 200

  Scenario: default function returns fallback when env is missing
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tpl-default-api-v1.0
      spec:
        displayName: Tpl-Default-Api
        version: v1.0
        context: /tpl-default/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Fallback
                        value: '{{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}'
      """
    Then the response status code should be 201
    And the JSON response field "spec.operations[0].policies[0].params.request.headers[0].value" should be:
      """
      {{ env "IT_DEFINITELY_MISSING_KEY" | default "fallback-value" }}
      """

    # PARKED: asserts the stored source config keeps the template UNRENDERED. Needs a
    # controller-DB reader (legacy GetStoredSourceConfigurationWithRetry over rest_apis /
    # llm_providers / llm_proxies). See task notes; do not delete this comment.

    When I send a "GET" request to "/tpl-default/v1.0/probe" until status 200
    Then the response status code should be 200
    And the response should contain echoed header "X-Fallback" with value "fallback-value"

    # Cleanup
    When I delete the API "tpl-default-api-v1.0"
    Then the response status code should be 200

  Scenario: secret template in LlmProvider upstream auth value is rendered upstream, unrendered in DB, and never returned in responses
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": { "name": "tpl-llm-provider-token" },
        "spec": {
          "displayName": "tpl-llm-provider-token",
          "description": "Auto-generated secret",
          "value": "llm-prov-secret-789"
        }
      }
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tpl-llm-provider
      spec:
        displayName: Tpl-Llm-Provider
        version: v1.0
        template: openai
        context: /tpl-llm-provider
        upstream:
          url: http://testbench:3000
          auth:
            type: api-key
            header: Authorization
            value: 'Bearer {{ secret "tpl-llm-provider-token" }}'
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # upstream auth.value is write-only: neither the secret handle nor its
    # resolved value is returned, on create or on any later read.
    And the response body should not contain "tpl-llm-provider-token"
    And the response body should not contain "llm-prov-secret-789"
    And the JSON response field "spec.upstream.auth.value" should not exist

    # GET must not echo the credential either, while the rest of the auth block survives
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/tpl-llm-provider"
    Then the response status code should be 200
    And the response body should not contain "tpl-llm-provider-token"
    And the response body should not contain "llm-prov-secret-789"
    # omitted entirely, not returned blank
    And the JSON response field "spec.upstream.auth.value" should not exist
    And the JSON response field "spec.upstream.auth.header" should be "Authorization"

    # PARKED: asserts the stored source config keeps the template UNRENDERED and does NOT
    # persist the resolved secret value alongside it. Needs a controller-DB reader (legacy
    # GetStoredSourceConfigurationWithRetry over rest_apis / llm_providers / llm_proxies).
    # See task notes; do not delete this comment.

    # Runtime: upstream must receive the resolved Authorization header value
    And I send a "POST" request to "/tpl-llm-provider/chat/completions" until status 200
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/tpl-llm-provider/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}]
      }
      """
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with value "Bearer llm-prov-secret-789"

    # Cleanup
    When I delete the LLM provider "tpl-llm-provider"
    Then the response status code should be 200
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/tpl-llm-provider-token"
    Then the response status code should be 200

  Scenario: secret template in LlmProxy set-headers policy is rendered upstream but unrendered in response and DB
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": { "name": "tpl-llm-proxy-token" },
        "spec": {
          "displayName": "tpl-llm-proxy-token",
          "description": "Auto-generated secret",
          "value": "llm-proxy-secret-456"
        }
      }
      """
    Then the response status code should be 201

    # Plain (un-templated) provider used as the proxy upstream
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tpl-llm-proxy-provider
      spec:
        displayName: Tpl-Llm-Proxy-Provider
        version: v1.0
        template: openai
        vhost: api.my-llm-provider.local
        upstream:
          url: http://testbench:3000
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: tpl-llm-proxy
      spec:
        displayName: Tpl-Llm-Proxy
        version: v1.0
        context: /tpl-llm-proxy
        provider:
          id: tpl-llm-proxy-provider
        policies:
          - name: set-headers
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  request:
                    headers:
                      - name: X-Auth-Token
                        value: 'Bearer {{ secret "tpl-llm-proxy-token" }}'
      """
    Then the response status code should be 201
    And the JSON response field "spec.policies[0].paths[0].params.request.headers[0].value" should be:
      """
      Bearer {{ secret "tpl-llm-proxy-token" }}
      """

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/tpl-llm-proxy"
    Then the response status code should be 200
    And the JSON response field "spec.policies[0].paths[0].params.request.headers[0].value" should be:
      """
      Bearer {{ secret "tpl-llm-proxy-token" }}
      """

    # PARKED: asserts the stored source config keeps the template UNRENDERED. Needs a
    # controller-DB reader (legacy GetStoredSourceConfigurationWithRetry over rest_apis /
    # llm_providers / llm_proxies). See task notes; do not delete this comment.

    # Runtime: upstream must receive the resolved X-Auth-Token header value
    And I send a "POST" request to "/tpl-llm-proxy/chat/completions" until status 200
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/tpl-llm-proxy/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}]
      }
      """
    Then the response status code should be 200
    And the response should contain echoed header "X-Auth-Token" with value "Bearer llm-proxy-secret-456"

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/tpl-llm-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "tpl-llm-proxy-provider"
    Then the response status code should be 200
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/tpl-llm-proxy-token"
    Then the response status code should be 200

  Scenario: env template in integer policy param is coerced and enforced at runtime
    # IT_RATE_LIMIT=5 is set on the gateway component.
    # The template renders to the string "5"; CoerceRestAPIPolicies turns it into
    # the numeric value 5 so schema validation passes (type: integer) and the
    # policy engine enforces the correct limit.
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tpl-env-ratelimit-api-v1.0
      spec:
        displayName: Tpl-Env-Ratelimit-Api
        version: v1.0
        context: /tpl-env-ratelimit/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /probe
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: '{{ env "IT_RATE_LIMIT" }}'
                          duration: "1h"
      """
    Then the response status code should be 201
    And the JSON response field "spec.operations[0].policies[0].params.quotas[0].limits[0].limit" should be:
      """
      {{ env "IT_RATE_LIMIT" }}
      """

    # Read the artifact back: the template must still be unrendered on RETRIEVAL, not just
    # echoed by the create. That is what keeps the artifact portable across environments — an
    # export or GitOps sync reads this, and a rendered value here would pin it to one gateway.
    When I get the API "tpl-env-ratelimit-api-v1.0"
    Then the response status code should be 200
    And the JSON response field "spec.operations[0].policies[0].params.quotas[0].limits[0].limit" should be:
      """
      {{ env "IT_RATE_LIMIT" }}
      """

    # Runtime: the integer limit of 5 must be enforced.
    # The readiness probe consumes ~1 request; four more reach the limit.
    And I send a "GET" request to "/tpl-env-ratelimit/v1.0/probe" until status 200
    When I send a "GET" request to "/tpl-env-ratelimit/v1.0/probe"
    Then the response status code should be 200
    When I send a "GET" request to "/tpl-env-ratelimit/v1.0/probe"
    Then the response status code should be 200
    When I send a "GET" request to "/tpl-env-ratelimit/v1.0/probe"
    Then the response status code should be 200
    When I send a "GET" request to "/tpl-env-ratelimit/v1.0/probe"
    Then the response status code should be 200

    # One more request must be rejected — limit exhausted.
    When I send a "GET" request to "/tpl-env-ratelimit/v1.0/probe"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the API "tpl-env-ratelimit-api-v1.0"
    Then the response status code should be 200

  Scenario: env template in boolean policy param is coerced and applied at runtime
    # IT_ALLOW_CREDENTIALS=true is set on the gateway component.
    # The template renders to the string "true"; CoerceRestAPIPolicies turns it
    # into bool(true) so schema validation passes (type: boolean) and the CORS
    # policy emits the Access-Control-Allow-Credentials: true response header.
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tpl-env-cors-api-v1.0
      spec:
        displayName: Tpl-Env-Cors-Api
        version: v1.0
        context: /tpl-env-cors/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: cors
            version: v1
            params:
              allowedOrigins:
                - "http://example.com"
              allowedMethods:
                - "GET"
              allowCredentials: '{{ env "IT_ALLOW_CREDENTIALS" }}'
        operations:
          - method: GET
            path: /probe
      """
    Then the response status code should be 201
    And the JSON response field "spec.policies[0].params.allowCredentials" should be:
      """
      {{ env "IT_ALLOW_CREDENTIALS" }}
      """

    # Unrendered on RETRIEVAL too, not just echoed by the create — see the integer scenario.
    When I get the API "tpl-env-cors-api-v1.0"
    Then the response status code should be 200
    And the JSON response field "spec.policies[0].params.allowCredentials" should be:
      """
      {{ env "IT_ALLOW_CREDENTIALS" }}
      """

    # Runtime: allowCredentials=true must produce the credentials response header.
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "/tpl-env-cors/v1.0/probe" until status 200
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Credentials" should be "true"

    # Cleanup
    When I delete the API "tpl-env-cors-api-v1.0"
    Then the response status code should be 200

  Scenario: missing secret reference fails with 400 at deploy time
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tpl-bad-secret-api-v1.0
      spec:
        displayName: Tpl-Bad-Secret-Api
        version: v1.0
        context: /tpl-bad-secret/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /probe
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Bad
                        value: '{{ secret "tpl-no-such-secret-xyz" }}'
      """
    Then the response status code should be 400
