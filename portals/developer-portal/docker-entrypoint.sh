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

CERT_DIR=/etc/devportal/tls
TLS_ENABLED="${APIP_DP_SERVER_HTTPS_ENABLED:-false}"

# Fail closed: certificates are generated once by ./setup.sh (host-side, into a
# bind-mounted directory), never here. Startup only checks that they exist —
# it never generates a fallback, matching every other required secret.
if [ "$TLS_ENABLED" = "true" ]; then
  if [ ! -f "$CERT_DIR/cert.pem" ] || [ ! -f "$CERT_DIR/key.pem" ]; then
    echo "[entrypoint] ERROR: TLS is enabled (APIP_DP_SERVER_HTTPS_ENABLED=true) but no certificate was found at $CERT_DIR/cert.pem / key.pem. Run ./setup.sh first, or mount your own certificate at $CERT_DIR." >&2
    exit 1
  fi
  echo "[entrypoint] TLS certificate found at $CERT_DIR"
fi

exec node src/server.js
