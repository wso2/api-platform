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

set -e

CERT_DIR=/etc/api-portal/tls
TLS_ENABLED="${APIP_AP_SERVER_HTTPS_ENABLED:-false}"

# The server now requires an explicit --config (repeatable, last-wins) — there is
# no default path and no silent-defaults fallback. The base config is mounted at
# /app/configs/config.toml (docker-compose bind mount / helm configMap). Override
# the base path with APIP_DP_CONFIG_PATH, and append overlay files by passing
# extra `--config <path>` arguments to the container (forwarded via "$@" below).
CONFIG_PATH="${APIP_DP_CONFIG_PATH:-/app/configs/config.toml}"

# Fail closed: certificates are generated once by ./setup.sh (host-side, into a
# bind-mounted directory), never here. Startup only checks that they exist —
# it never generates a fallback, matching every other required secret.
if [ "$TLS_ENABLED" = "true" ]; then
  if [ ! -f "$CERT_DIR/cert.pem" ] || [ ! -f "$CERT_DIR/key.pem" ]; then
    echo "[entrypoint] ERROR: TLS is enabled (APIP_AP_SERVER_HTTPS_ENABLED=true) but no certificate was found at $CERT_DIR/cert.pem / key.pem. Run ./setup.sh first, or mount your own certificate at $CERT_DIR." >&2
    exit 1
  fi
  echo "[entrypoint] TLS certificate found at $CERT_DIR"
fi

exec node src/server.js --config "$CONFIG_PATH" "$@"
