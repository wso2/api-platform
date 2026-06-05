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

import { PLATFORM_API_BASE_URL } from '../config.env';

export class IDPNotConfiguredError extends Error {
  constructor(message = 'IDP not configured for this organization') {
    super(message);
    this.name = 'IDPNotConfiguredError';
  }
}

export interface OrgAuthConfig {
  idp_type?: string;
  issuer: string;
  client_id: string;
  authorization_endpoint: string;
  token_endpoint: string;
  logout_url?: string;
  scopes?: string[];
  pkce_required: boolean;
  response_type: string;
}

// The auth discovery endpoint lives under /portal/api/v1/, separate from the
// regular /api/v1/ base used by authenticated endpoints.
const PORTAL_API_ROOT = PLATFORM_API_BASE_URL.replace(/\/api\/v\d+$/, '');

export async function fetchOrgAuthConfig(orgHandle: string): Promise<OrgAuthConfig> {
  const res = await fetch(
    `${PORTAL_API_ROOT}/portal/api/v1/organizations/${encodeURIComponent(orgHandle)}/auth`
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    if (res.status === 400) {
      throw new IDPNotConfiguredError((body as any)?.message);
    }
    throw new Error(
      (body as any)?.message ?? `Failed to fetch auth config: HTTP ${res.status}`
    );
  }
  return res.json() as Promise<OrgAuthConfig>;
}
