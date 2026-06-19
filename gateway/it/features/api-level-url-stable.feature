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

@api-level-url-stable
Feature: API-Level Upstream URL-Stable Cluster Naming
  As an API developer
  I want API-level main and sandbox cluster names to stay stable across
  upstream URL edits
  So that routes, name-keyed stats, and cluster identity survive URL changes
  and requests keep succeeding during updates

  Background:
    Given the gateway services are running

  Scenario: API-level main upstream URL update (host and path change) routes to new backend (URL-stable cluster naming)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-main-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Main-API
        version: v1.0
        context: /api-level-url-stable-main/$version
        vhosts:
          main: api-level-url-stable-main.local
        upstream:
          main:
            url: http://sample-backend:9080/version-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint" to be ready with host "api-level-url-stable-main.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-main.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/version-a/endpoint"

    # Envoy admin: the API-level cluster must use the identity-derived name
    # (main_<hash>) and there must be no URL-derived (cluster_<scheme>_<host>)
    # cluster. The URL-derived form is what the pre-change naming produced, so
    # this assertion fails on the old naming scheme. The exact name set is
    # captured so the post-update step can prove the NAME survived the update.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And I capture the Envoy cluster names prefixed "main_"

    Given I authenticate using basic auth as "admin"
    When I update the API "api-level-url-stable-main-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-main-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Main-API
        version: v1.0
        context: /api-level-url-stable-main/$version
        vhosts:
          main: api-level-url-stable-main.local
        upstream:
          main:
            # The host changes too (container alias of the same backend), proving
            # the cluster survives a HOST edit, not only a path edit. The old
            # URL-derived naming kept its name across path edits but renamed the
            # cluster on any host or scheme change.
            url: http://it-sample-backend:9080/version-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint" to be ready with host "api-level-url-stable-main.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-main.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/version-b/endpoint"

    # After the HOST change the exact cluster-name set must be UNCHANGED:
    # this proves the same main_<hash> cluster survived the host edit (a
    # rename to a different main_<hash> would fail the unchanged step). The
    # old naming would have minted a new cluster_http_it-sample-backend_9080
    # cluster here and dropped the previous one.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And the Envoy cluster names prefixed "main_" should be unchanged

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-main-api-v1.0"
    Then the response should be successful

  Scenario: API-level sandbox upstream URL update (host and path change) routes to new backend
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-sandbox-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Sandbox-API
        version: v1.0
        context: /api-level-url-stable-sandbox/$version
        vhosts:
          main: api-level-url-stable-sandbox-main.local
          sandbox: api-level-url-stable-sandbox-sb.local
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/sandbox-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint" to be ready with host "api-level-url-stable-sandbox-sb.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-sandbox-sb.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/sandbox-a/endpoint"

    # Capture the sandbox cluster-name set so the post-update step can prove
    # the sandbox_<hash> name survived the URL update.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "sandbox_"
    And the response body should not contain "cluster_http_"
    And I capture the Envoy cluster names prefixed "sandbox_"

    Given I authenticate using basic auth as "admin"
    When I update the API "api-level-url-stable-sandbox-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-sandbox-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Sandbox-API
        version: v1.0
        context: /api-level-url-stable-sandbox/$version
        vhosts:
          main: api-level-url-stable-sandbox-main.local
          sandbox: api-level-url-stable-sandbox-sb.local
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            # The sandbox host changes too (container alias of the same
            # backend), so this update exercises a host edit on the sandbox
            # cluster, not only a path edit.
            url: http://it-sample-backend:9080/sandbox-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint" to be ready with host "api-level-url-stable-sandbox-sb.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-sandbox-sb.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/sandbox-b/endpoint"

    # Envoy admin: the sandbox cluster must use the identity-derived name
    # (sandbox_<hash>); no URL-derived cluster may exist, and the exact name
    # set must be unchanged across the host edit (identity proof). Fails on
    # the old URL-derived naming scheme.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "sandbox_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And the Envoy cluster names prefixed "sandbox_" should be unchanged

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-sandbox-api-v1.0"
    Then the response should be successful

  Scenario: API-level upstream with cluster_header routing (default upstream cluster resolves correctly)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-default-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Default-API
        version: v1.0
        context: /api-level-url-stable-default/$version
        vhosts:
          main: api-level-url-stable-default.local
        upstreamDefinitions:
          - name: backend-default
            basePath: /api-main
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: backend-default
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-default/v1.0/endpoint" to be ready with host "api-level-url-stable-default.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-default.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-default/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/endpoint"

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-default-api-v1.0"
    Then the response should be successful

  Scenario: API-level main and sandbox on the same backend host get separate identity-named clusters
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-collision-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Collision-API
        version: v1.0
        context: /api-level-url-stable-collision/$version
        vhosts:
          main: api-level-url-stable-collision-main.local
          sandbox: api-level-url-stable-collision-sb.local
        upstream:
          main:
            url: http://sample-backend:9080/collision-main
          sandbox:
            url: http://sample-backend:9080/collision-sandbox
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-collision/v1.0/endpoint" to be ready with host "api-level-url-stable-collision-main.local"

    # Main and sandbox share the same backend host:port but must route to their
    # own base paths. The old URL-derived naming keyed the cluster on host and
    # scheme only, so main and sandbox collapsed into one shared cluster here;
    # identity naming gives each its own.
    When I clear all headers
    And I set request host to "api-level-url-stable-collision-main.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-collision/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/collision-main/endpoint"

    When I clear all headers
    And I set request host to "api-level-url-stable-collision-sb.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-collision/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/collision-sandbox/endpoint"

    # Envoy admin: an identity-named main_<hash> and a sandbox_<hash> cluster
    # must both exist (they do not collide), and no URL-derived cluster may
    # exist. Under the old naming both upstreams shared one cluster_<scheme>_<host>
    # cluster, so this assertion fails on the previous scheme.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should contain "sandbox_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-collision-api-v1.0"
    Then the response should be successful

  Scenario: Two APIs sharing the same backend host each route under their own identity-named cluster
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-shared-a-v1.0
      spec:
        displayName: API-Level-URL-Stable-Shared-A
        version: v1.0
        context: /api-level-url-stable-shared-a/$version
        vhosts:
          main: api-level-url-stable-shared-a.local
        upstream:
          main:
            url: http://sample-backend:9080/shared-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-shared-a/v1.0/endpoint" to be ready with host "api-level-url-stable-shared-a.local"

    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-shared-b-v1.0
      spec:
        displayName: API-Level-URL-Stable-Shared-B
        version: v1.0
        context: /api-level-url-stable-shared-b/$version
        vhosts:
          main: api-level-url-stable-shared-b.local
        upstream:
          main:
            url: http://sample-backend:9080/shared-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-shared-b/v1.0/endpoint" to be ready with host "api-level-url-stable-shared-b.local"

    # Two distinct APIs point at the same backend host:port. The old URL-derived
    # naming made them share one cluster_<scheme>_<host> cluster; identity naming
    # keys each cluster on its apiID, so the two APIs route independently to their
    # own base paths under identity-named clusters and no URL-derived cluster exists.
    When I clear all headers
    And I set request host to "api-level-url-stable-shared-a.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-shared-a/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/shared-a/endpoint"

    When I clear all headers
    And I set request host to "api-level-url-stable-shared-b.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-shared-b/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/shared-b/endpoint"

    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"

    # Delete API-B and confirm API-A still routes, proving the two APIs own
    # independent clusters (deleting one does not disturb the other).
    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-shared-b-v1.0"
    Then the response should be successful

    When I clear all headers
    And I set request host to "api-level-url-stable-shared-a.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-shared-a/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/shared-a/endpoint"

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-shared-a-v1.0"
    Then the response should be successful

  Scenario: API-level main upstream scheme and port change keeps the same identity-named cluster
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-scheme-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Scheme-API
        version: v1.0
        context: /api-level-url-stable-scheme/$version
        vhosts:
          main: api-level-url-stable-scheme.local
        upstream:
          main:
            url: http://sample-backend:9080/version-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-scheme/v1.0/endpoint" to be ready with host "api-level-url-stable-scheme.local"

    # Capture the identity-derived cluster name while the upstream is plain http.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And I capture the Envoy cluster names prefixed "main_"

    # Change the upstream scheme (http -> https) AND port (9080 -> 9443) in one
    # edit. The old URL-derived naming embedded scheme and port in the cluster
    # name (cluster_<scheme>_<host>_<port>), so this edit would have minted a new
    # cluster_https_ cluster and dropped the previous one. Identity-based naming
    # must keep the SAME main_<hash> and never produce a cluster_https_. TLS
    # routing itself is not asserted (there is no TLS echo backend); the cluster
    # name is stable independent of upstream reachability.
    Given I authenticate using basic auth as "admin"
    When I update the API "api-level-url-stable-scheme-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: api-level-url-stable-scheme-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Scheme-API
        version: v1.0
        context: /api-level-url-stable-scheme/$version
        vhosts:
          main: api-level-url-stable-scheme.local
        upstream:
          main:
            url: https://sample-backend:9443/version-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful

    # The main_<hash> name set must be UNCHANGED after the scheme/port edit, and
    # no URL-derived cluster_https_ may appear.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And the Envoy cluster names prefixed "main_" should be unchanged

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-scheme-api-v1.0"
    Then the response should be successful
