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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { GlobalPolicy } from './types';

export interface ApiKeyAuthDisplay {
  headerName: string;
  location: 'header' | 'query';
  valuePrefix: string;
}

interface ApiKeySecurityLike {
  enabled?: boolean;
  key?: string;
  in?: 'header' | 'query';
  valuePrefix?: string;
}

interface SecurityConfigLike {
  enabled?: boolean;
  apiKey?: ApiKeySecurityLike;
}

const API_KEY_AUTH_POLICY = 'api-key-auth';
const DEFAULT_HEADER_NAME = 'X-API-Key';

/**
 * Resolve the API key auth settings that are actually in effect for a
 * provider/proxy, mirroring the deployment precedence: the first-class
 * security config wins; otherwise an attached api-key-auth global policy;
 * otherwise the platform defaults. Used by the key-generation dialogs and
 * try-it-out curl snippets so they reflect the deployed configuration.
 */
export function resolveApiKeyAuthDisplay(
  security?: SecurityConfigLike | null,
  globalPolicies?: GlobalPolicy[] | null
): ApiKeyAuthDisplay {
  const apiKey = security?.apiKey;
  if (security?.enabled !== false && apiKey && apiKey.enabled !== false && apiKey.key?.trim()) {
    return {
      headerName: apiKey.key.trim(),
      location: apiKey.in ?? 'header',
      valuePrefix: apiKey.valuePrefix ?? '',
    };
  }

  const policy = globalPolicies?.find((p) => p.name === API_KEY_AUTH_POLICY);
  if (policy?.params) {
    const key = typeof policy.params.key === 'string' ? policy.params.key.trim() : '';
    const location = policy.params.in === 'query' ? 'query' : 'header';
    const valuePrefix =
      typeof policy.params.valuePrefix === 'string' ? policy.params.valuePrefix : '';
    return {
      headerName: key || DEFAULT_HEADER_NAME,
      location,
      valuePrefix,
    };
  }

  // Fall back to security config even if disabled flags are ambiguous
  if (apiKey?.key?.trim()) {
    return {
      headerName: apiKey.key.trim(),
      location: apiKey.in ?? 'header',
      valuePrefix: apiKey.valuePrefix ?? '',
    };
  }

  return { headerName: DEFAULT_HEADER_NAME, location: 'header', valuePrefix: '' };
}

/**
 * Build the header/query value a client must send: the configured prefix
 * (single-space separated, e.g. "Bearer <key>") followed by the key.
 */
export function formatPrefixedKey(valuePrefix: string, apiKey: string): string {
  const prefix = valuePrefix.trimEnd();
  return prefix ? `${prefix} ${apiKey}` : apiKey;
}
