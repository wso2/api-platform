#!/bin/bash

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

# Gateway Runtime Health Check Script
# Checks both Router (Envoy) and Policy Engine return HTTP 200.
# Used by Docker Compose healthcheck and Kubernetes liveness/readiness probes.
#
# Exit 0 = healthy, Exit 1 = unhealthy

ROUTER_ADMIN_ENABLED="${ROUTER_ADMIN_ENABLED:-false}"
ROUTER_ADMIN_PORT="${ROUTER_ADMIN_PORT:-9901}"
ROUTER_HTTP_PORT="${ROUTER_HTTP_PORT:-8080}"
POLICY_ENGINE_ADMIN_PORT="${POLICY_ENGINE_ADMIN_PORT:-9002}"

# Check Router (Envoy) readiness.
# The admin interface (/ready) is disabled by default (see docker-entrypoint.sh /
# ROUTER_ADMIN_ENABLED) — fall back to a raw TCP check against the main listener,
# which confirms Envoy is up and accepting connections.
if [ "${ROUTER_ADMIN_ENABLED}" = "true" ]; then
  ROUTER_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${ROUTER_ADMIN_PORT}/ready")
  if [ "$ROUTER_STATUS" != "200" ]; then
    echo "Router not ready (HTTP ${ROUTER_STATUS})"
    exit 1
  fi
else
  if ! (exec 3<>"/dev/tcp/127.0.0.1/${ROUTER_HTTP_PORT}") 2>/dev/null; then
    echo "Router not accepting connections on port ${ROUTER_HTTP_PORT}"
    exit 1
  fi
  exec 3<&- 3>&- 2>/dev/null || true
fi

# Check Policy Engine health — expect HTTP 200
PE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${POLICY_ENGINE_ADMIN_PORT}/health")
if [ "$PE_STATUS" != "200" ]; then
  echo "Policy Engine not healthy (HTTP ${PE_STATUS})"
  exit 1
fi
