#!/bin/sh

# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

# Default XDS server configuration if not provided
export XDS_SERVER_HOST="${XDS_SERVER_HOST:-gateway-controller}"
export XDS_SERVER_PORT="${XDS_SERVER_PORT:-18000}"
export LOG_LEVEL="${LOG_LEVEL:-info}"

echo "Starting Envoy with xDS server at ${XDS_SERVER_HOST}:${XDS_SERVER_PORT}"
echo "Log level: ${LOG_LEVEL}"

# Generate config override by substituting environment variables
CONFIG_OVERRIDE=$(envsubst < /etc/envoy/config-override.yaml)

# Use --config-yaml to override the xDS cluster address
exec /usr/local/bin/envoy \
  -c /etc/envoy/envoy.yaml \
  --config-yaml "${CONFIG_OVERRIDE}" \
  --log-level "${LOG_LEVEL}" \
  "$@"
