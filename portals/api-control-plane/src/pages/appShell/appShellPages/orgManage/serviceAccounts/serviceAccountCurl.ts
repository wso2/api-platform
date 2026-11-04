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

import { runtimeConfig } from '@/config/runtime';

// The browser only knows the BFF's proxy, which a workload cannot use, so the
// examples name the real address as a shell variable.
const apiBase = () => `$PLATFORM_API_URL/api/${runtimeConfig.platformApiVersion}`;

/**
 * The client-credentials exchange as curl. The secret is always a shell
 * variable, so a pasted example never leaks it.
 */
export const tokenRequestCurl = (clientId: string, scopes: readonly string[]): string =>
  [
    `curl -X POST "${apiBase()}/service-accounts/token" \\`,
    '  -d grant_type=client_credentials \\',
    `  -d client_id=${clientId} \\`,
    scopes.length > 0 ? '  -d client_secret="$CLIENT_SECRET" \\' : '  -d client_secret="$CLIENT_SECRET"',
    ...(scopes.length > 0 ? [`  -d scope="${scopes.join(' ')}"`] : []),
  ].join('\n');

/** A harmless read with a token in hand. */
export const tokenUseCurl = (): string =>
  [
    'curl -H "Authorization: Bearer $ACCESS_TOKEN" \\',
    `  "${apiBase()}/projects"`,
  ].join('\n');
