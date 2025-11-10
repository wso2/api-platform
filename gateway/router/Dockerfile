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

FROM envoyproxy/envoy:v1.35.3

# Install envsubst (from gettext package)
USER root
RUN apt-get update && \
    apt-get install -y gettext-base && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy bootstrap configuration
COPY config/envoy-bootstrap.yaml /etc/envoy/envoy.yaml

# Copy config override template
COPY config/config-override.yaml /etc/envoy/config-override.yaml

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose HTTP port and admin port
EXPOSE 8080 9901 8081 8082

# Set environment variables with default values
ENV XDS_SERVER_HOST=gateway-controller
ENV XDS_SERVER_PORT=18000

# Use entrypoint script
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
