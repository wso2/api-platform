/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { Gateway } from '../apis/gatewayTypes';

/**
 * The client-facing MCP endpoint a gateway serves the proxy on, including the
 * `/mcp` path. Empty when the gateway has no host to build it from.
 */
export function buildMcpInvokeUrl(gateway: Gateway | undefined, context?: string): string {
  const vhost = (gateway?.endpoints?.[0] || gateway?.vhost)?.trim();
  if (!vhost) return '';
  const base = /^https?:\/\//i.test(vhost) ? vhost.replace(/\/+$/, '') : `https://${vhost.replace(/\/+$/, '')}`;
  const trimmed = (context ?? '/').trim();
  const withSlash = trimmed ? (trimmed.startsWith('/') ? trimmed : `/${trimmed}`) : '/';
  return `${base}${withSlash.replace(/\/+$/, '')}/mcp`;
}
