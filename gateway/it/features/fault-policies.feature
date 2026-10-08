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

@fault-policies
Feature: Per-API fault policies
  As an API developer
  I want a list of policies that runs only when a request fails
  So that I can act on failures without touching the success path

  # A fault policy implements the SDK's OnFault contract. Nothing in the shipped
  # catalogue does yet, so these scenarios use the two dev policies the demo build
  # registers:
  #
  #   fault-declarer   PRODUCES a rejection and states whether it is a failure
  #   fault-notifier   HANDLES one, stamping a marker header so the effect is
  #                    observable without reading logs. The marker's VALUE is the
  #                    response status.
  #
  # These previously used set-headers as a stand-in. That stopped working the moment
  # fault policies got their own contract: set-headers implements a response hook and
  # not OnFault, so the chain builder drops it — the assertions could never pass, and
  # the one scenario that asserted the marker was ABSENT passed for that reason rather
  # than because the fault flow declined to run. The last scenario here now pins that
  # dropping behaviour deliberately instead.
  #
  # Every negative scenario is paired with a positive control on the same policy
  # configuration. A fault-policy test that only asserts absence is indistinguishable
  # from one where the feature is switched off entirely.
  #
  # This feature is not in the default suite (see getFeaturePaths). Run it with
  # IT_FEATURE_PATHS=features/fault-policies.feature, against a gateway build whose manifest
  # registers fault-declarer, fault-notifier and error-response-formatter.
  #
  # The scenarios tagged @handle-upstream-faults expect an upstream or router error to reach
  # the fault policies, which happens only with
  #   [policy_engine.fault_policies] handle_upstream_faults = true
  # in test-config.toml. It is off there because it moves every upstream error off the
  # response policies, which the default suite's round-robin and interceptor features rely
  # on. Set it for a run of this feature; with it off, those scenarios fail by design.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ── what reaches the fault chain ─────────────────────────────────────────────

  Scenario: A policy that declares its rejection reaches the fault policies
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-declared-api-v1.0
      spec:
        displayName: Fault-Declared
        version: v1.0
        context: /fault-declared/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 422
              isFault: true
              fault:
                code: "906000"
                type: "guardrail"
                message: "Payload rejected by policy"
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-handled
      """
    Then the response should be successful

    When I send a GET request to "http://localhost:8080/fault-declared/v1.0/get"
    Then the response status code should be 422
    And the response header "x-fault-handled" should be "422"

    When I delete the API "fault-declared-api-v1.0"
    Then the response should be successful

  Scenario: A described error at an error status reaches the fault policies
    # The status is what routes a rejection into the fault flow: 400 and above always does,
    # whether or not the policy set IsFault. Describing the failure decides what the chain
    # and analytics are told, not whether the chain runs.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-described-only-api-v1.0
      spec:
        displayName: Fault-Described-Only
        version: v1.0
        context: /fault-described-only/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 404
              fault:
                code: "961000"
                type: "not_found"
                message: "No such widget"
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-handled
      """
    Then the response should be successful

    When I send a GET request to "http://localhost:8080/fault-described-only/v1.0/get"
    Then the response status code should be 404
    # Not rendered: RestApi is not one of the kinds the gateway formats, so a policy that
    # describes its failure and writes no body sends no body. The description still reaches
    # the fault chain, the logs and analytics — it is simply not the client's copy.
    And the response body should be empty
    # Reported: the 404 reached the chain, so the notifier ran and set its marker.
    And the response header "x-fault-handled" should be "404"

    When I delete the API "fault-described-only-api-v1.0"
    Then the response should be successful

  Scenario: A policy that declares nothing still reaches the fault policies by its status
    # api-key-auth here declares neither isFault nor a fault description — the shape of every
    # policy released before the fault contract. Its 401 reaches the chain all the same,
    # because the status is enough, so a fault policy attached to an existing API sees the
    # rejections of policies that were never migrated.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-undeclared-api-v1.0
      spec:
        displayName: Fault-Undeclared
        version: v1.0
        context: /fault-undeclared/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: api-key-auth
            version: v1
            params:
              key: X-API-Key
              in: header
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-handled
      """
    Then the response should be successful

    When I send a GET request to "http://localhost:8080/fault-undeclared/v1.0/get"
    Then the response status code should be 401
    And the response header "x-fault-handled" should be "401"

    When I delete the API "fault-undeclared-api-v1.0"
    Then the response should be successful

  @handle-upstream-faults
  Scenario: Both a router failure and a backend error reach the fault policies
    # Both are a 5xx leaving the gateway, and no status rule can tell them apart — which one
    # it was comes from Envoy's response.code_details. They used to behave oppositely, the
    # backend's own error being dropped; now both are reported and the distinction is carried
    # to the handler as ErrorContext.Source instead of being spent on a gate. Narrowing back
    # down is an execution condition, covered by its own scenario below.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-router-api-v1.0
      spec:
        displayName: Fault-Router
        version: v1.0
        context: /fault-router/$version
        upstream:
          main:
            url: http://127.0.0.1:39999
        operations:
          - method: GET
            path: /get
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-handled
      """
    Then the response should be successful

    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-backend-api-v1.0
      spec:
        displayName: Fault-Backend
        version: v1.0
        context: /fault-backend/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /status/500
          - method: GET
            path: /get
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-handled
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/fault-backend/v1.0/get" to be ready

    # The ROUTER produced this one — the upstream never answered.
    When I send a GET request to "http://localhost:8080/fault-router/v1.0/get"
    Then the response status code should be 503
    And the response header "x-fault-handled" should be "503"

    # The BACKEND produced this one. Identical fault configuration, and now the same outcome:
    # the operator running a gateway in front of this API is the one who needs to hear it is
    # failing.
    When I send a GET request to "http://localhost:8080/fault-backend/v1.0/status/500"
    Then the response status code should be 500
    And the response header "x-fault-handled" should be "500"

    # What must NOT happen is the backend's own error being rewritten. The backend said it, so
    # it stands — the notifier annotates and does not author. httpbin's /status/500 sends no
    # body, so the client must receive none: nothing synthesized one in its place.
    And the response body should be empty

    # And a healthy response fires nothing, on the same configuration again.
    When I send a GET request to "http://localhost:8080/fault-backend/v1.0/get"
    Then the response status code should be 200
    And the response header "x-fault-handled" should not exist

    When I delete the API "fault-router-api-v1.0"
    Then the response should be successful
    When I delete the API "fault-backend-api-v1.0"
    Then the response should be successful

  # ── the chain itself ─────────────────────────────────────────────────────────

  Scenario: Fault entries execute in the order declared
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-order-api-v1.0
      spec:
        displayName: Fault-Order
        version: v1.0
        context: /fault-order/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 403
              isFault: true
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-first
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-second
      """
    Then the response should be successful

    # Both fire. The fault list is authored top to bottom and executed in that order,
    # unlike the response chain which unwinds back to front.
    When I send a GET request to "http://localhost:8080/fault-order/v1.0/get"
    Then the response status code should be 403
    And the response header "x-fault-first" should be "403"
    And the response header "x-fault-second" should be "403"

    When I delete the API "fault-order-api-v1.0"
    Then the response should be successful

  Scenario: An execution condition narrows which failures an entry sees
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-cond-api-v1.0
      spec:
        displayName: Fault-Cond
        version: v1.0
        context: /fault-cond/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 422
              isFault: true
        faultPolicies:
          - name: fault-notifier
            version: v1
            executionCondition: "response.ResponseStatus == 503"
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-only-503
          - name: fault-notifier
            version: v1
            executionCondition: "response.ResponseStatus == 422"
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-only-422
      """
    Then the response should be successful

    # The 422 entry runs; the 503 entry is skipped. Both are in the same chain, so this
    # distinguishes "the condition was evaluated" from "the chain did not run".
    When I send a GET request to "http://localhost:8080/fault-cond/v1.0/get"
    Then the response status code should be 422
    And the response header "x-fault-only-422" should be "422"
    And the response header "x-fault-only-503" should not exist

    When I delete the API "fault-cond-api-v1.0"
    Then the response should be successful

  @handle-upstream-faults
  Scenario: An execution condition on the source separates gateway failures from the backend's
    # The narrowing that replaced the provenance gate. Both failures are now delivered to the
    # chain, so the condition — not the engine — is what decides which one an entry acts on.
    # One API, one chain, two entries: whichever way each fires, the chain itself ran.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-source-api-v1.0
      spec:
        displayName: Fault-Source
        version: v1.0
        context: /fault-source/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /status/500
          - method: GET
            path: /reject
            policies:
              - name: fault-declarer
                version: v1
                params:
                  statusCode: 422
                  isFault: true
        faultPolicies:
          - name: fault-notifier
            version: v1
            executionCondition: 'fault.Source == "gateway"'
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-gateway-only
          - name: fault-notifier
            version: v1
            executionCondition: 'fault.Source == "backend"'
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-backend-only
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/fault-source/v1.0/get" to be ready

    # A policy rejection: the gateway built this response.
    When I send a GET request to "http://localhost:8080/fault-source/v1.0/reject"
    Then the response status code should be 422
    And the response header "x-fault-gateway-only" should be "422"
    And the response header "x-fault-backend-only" should not exist

    # The backend's own 500. Same chain, opposite entry — which is the whole point: the
    # failure reached the chain, and the condition is what told the two apart.
    When I send a GET request to "http://localhost:8080/fault-source/v1.0/status/500"
    Then the response status code should be 500
    And the response header "x-fault-backend-only" should be "500"
    And the response header "x-fault-gateway-only" should not exist

    When I delete the API "fault-source-api-v1.0"
    Then the response should be successful

  Scenario: Operation-level and API-level fault policies both run
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-levels-api-v1.0
      spec:
        displayName: Fault-Levels
        version: v1.0
        context: /fault-levels/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /scoped
            faultPolicies:
              - name: fault-notifier
                version: v1
                params:
                  url: http://echo-backend:80/
                  markerHeader: x-fault-op
          - method: GET
            path: /plain
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 422
              isFault: true
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-api
      """
    Then the response should be successful

    # Both levels run on the scoped operation — an operation entry ADDS to the API's
    # rather than overriding it, because a fault entry is a handler and not a setting.
    When I send a GET request to "http://localhost:8080/fault-levels/v1.0/scoped"
    Then the response status code should be 422
    And the response header "x-fault-op" should be "422"
    And the response header "x-fault-api" should be "422"

    # The other operation gets the API-level entry only.
    When I send a GET request to "http://localhost:8080/fault-levels/v1.0/plain"
    Then the response status code should be 422
    And the response header "x-fault-api" should be "422"
    And the response header "x-fault-op" should not exist

    When I delete the API "fault-levels-api-v1.0"
    Then the response should be successful

  @handle-upstream-faults
  Scenario: An API whose only configuration is fault policies still gets them
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-only-api-v1.0
      spec:
        displayName: Fault-Only
        version: v1.0
        context: /fault-only/$version
        upstream:
          main:
            url: http://127.0.0.1:39999
        operations:
          - method: GET
            path: /get
        faultPolicies:
          - name: fault-notifier
            version: v1
            params:
              url: http://echo-backend:80/
              markerHeader: x-fault-handled
      """
    Then the response should be successful

    # No policies block at all. Attaching fault policies has to force the response-body
    # phase on by itself, or the chain would never be reached on an API that needs no
    # body for its own reasons.
    When I send a GET request to "http://localhost:8080/fault-only/v1.0/get"
    Then the response status code should be 503
    And the response header "x-fault-handled" should be "503"

    When I delete the API "fault-only-api-v1.0"
    Then the response should be successful

  # ── protocol formatting: reserved for kinds whose callers cannot read anything else ──
  #
  # errorformat.supportedKinds is empty, so nothing below renders. The scenarios are kept
  # pointed at that fact rather than deleted: "the gateway did not invent a body" is the
  # behaviour every shipping client depends on, and it needs a test as much as rendering
  # would.

  Scenario: A declared error is not rendered for a kind the gateway does not format
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-format-api-v1.0
      spec:
        displayName: Fault-Format
        version: v1.0
        context: /fault-format/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 401
              fault:
                code: "900902"
                type: "authentication"
                message: "Valid credentials required"
                description: "THE-DETAIL-MUST-NOT-BE-FORWARDED"
      """
    Then the response should be successful

    # No fault policies at all, and RestApi does not format, so nothing synthesizes a body.
    # The caller gets the status the policy chose and nothing else.
    When I send a GET request to "http://localhost:8080/fault-format/v1.0/get"
    Then the response status code should be 401
    And the response body should be empty

    # Description is the detail a guardrail blocked. It reaches a fault policy and the logs;
    # it must never reach the client — and with nothing rendering, nothing can leak it.
    And the response body should not contain "THE-DETAIL-MUST-NOT-BE-FORWARDED"

    # Asking for XML changes nothing. Content negotiation chooses BETWEEN rendered shapes,
    # and there is no rendering here to choose for.
    When I send a GET request to "http://localhost:8080/fault-format/v1.0/get" with header "Accept" value "application/xml"
    Then the response status code should be 401
    And the response body should be empty

    When I delete the API "fault-format-api-v1.0"
    Then the response should be successful

  Scenario: A producer's body accompanying a described error is forwarded unchanged
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-fallback-api-v1.0
      spec:
        displayName: Fault-Fallback
        version: v1.0
        context: /fault-fallback/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 422
              isFault: true
              body: '{"legacy":"only an older gateway sends this"}'
              fault:
                code: "906000"
                type: "guardrail"
                message: "Request blocked by a guardrail"
                description: "the blocked content itself"
                guardrail:
                  interveningGuardrail: "word-count-guardrail"
                  actionReason: "Violation of applied word count constraints detected"
                  assessments: "Expected word count to be between 10 and 500 words."
      """
    Then the response should be successful

    # The policy set both. Since this kind does not format, the body the policy wrote IS the
    # client's copy — byte for byte what the same policy sent before the fault flow existed.
    # The description shapes nothing on the wire; it exists for the fault chain, the logs and
    # analytics.
    When I send a GET request to "http://localhost:8080/fault-fallback/v1.0/get"
    Then the response status code should be 422
    And the response body should contain "legacy"
    And the response body should not contain "906000"

    # Nothing renders, so neither the guardrail assessment nor the withheld detail can reach
    # the client by that route.
    And the response body should not contain "word-count-guardrail"
    And the response body should not contain "the blocked content itself"

    When I delete the API "fault-fallback-api-v1.0"
    Then the response should be successful

  Scenario: A body authored by a fault policy is left alone
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-authored-api-v1.0
      spec:
        displayName: Fault-Authored
        version: v1.0
        context: /fault-authored/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 401
              isFault: true
              fault:
                code: "900902"
                type: "authentication"
                message: "Valid credentials required"
        faultPolicies:
          - name: error-response-formatter
            version: v1
            params:
              template:
                oops: ${Message}
                ref: ${Code}
      """
    Then the response should be successful

    # The operator's envelope, even asking for XML. This is the supported way to shape an
    # error body for a kind the gateway does not format: a fault entry writes it, and what a
    # fault entry writes is what the client receives.
    When I send a GET request to "http://localhost:8080/fault-authored/v1.0/get" with header "Accept" value "application/xml"
    Then the response status code should be 401
    And the response body should contain "oops"
    And the response body should not contain "<error>"

    When I delete the API "fault-authored-api-v1.0"
    Then the response should be successful

  # ── the OnFault contract ─────────────────────────────────────────────────────

  Scenario: A policy without OnFault is dropped from the fault chain
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-nohook-api-v1.0
      spec:
        displayName: Fault-NoHook
        version: v1.0
        context: /fault-nohook/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        policies:
          - name: fault-declarer
            version: v1
            params:
              statusCode: 422
              isFault: true
        faultPolicies:
          - name: set-headers
            version: v1
            params:
              response:
                headers:
                  - name: x-should-never-appear
                    value: "1"
      """
    # Deployment SUCCEEDS: the policy exists and its parameters are valid. The entry is
    # dropped when the route's chain is built, because set-headers implements a
    # response-phase hook and not OnFault. Before the contract existed it would have been
    # dispatched through that hook, which is the confusion the contract removes.
    Then the response should be successful

    When I send a GET request to "http://localhost:8080/fault-nohook/v1.0/get"
    Then the response status code should be 422
    And the response header "x-should-never-appear" should not exist

    When I delete the API "fault-nohook-api-v1.0"
    Then the response should be successful

  Scenario: A fault entry naming an unknown policy is rejected at deploy time
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: fault-invalid-api-v1.0
      spec:
        displayName: Fault-Invalid
        version: v1.0
        context: /fault-invalid/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
        faultPolicies:
          - name: no-such-policy-exists
            version: v1
      """
    # Validated exactly like any other policy reference: a fault entry is an ordinary
    # policy invoked on a different path, not a special kind.
    Then the response should be a client error
