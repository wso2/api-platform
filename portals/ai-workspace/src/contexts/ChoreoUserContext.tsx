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

// ============================================================================
// ChoreoUserContext — Platform API (standalone) version
// ----------------------------------------------------------------------------
// Asgardeo authentication has been removed. All org data comes from the
// Platform API (https://localhost:9243/api/v0.9).
// Token exchange and IDP-specific logic are no-ops.
// ============================================================================

import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode,
} from 'react';
import { logger } from '../utils/logger';
import type { Organization, ValidateUserResponse } from '../utils/types';
import { PLATFORM_API_BASE_URL } from '../config.env';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface ChoreoUserContextType {
  isTokenExchanged: boolean;
  setIsTokenExchanged: React.Dispatch<React.SetStateAction<boolean>>;
  isOrgAdmin: boolean;
  setIsOrgAdmin: React.Dispatch<React.SetStateAction<boolean>>;
  /** Always false — no Asgardeo sign-in hook available */
  shouldValidateUser: boolean;
  /** Always '' — no IDP in standalone mode */
  fidp: string;
  /** No-op — token exchange not needed for Platform API */
  exchangeOrgToken: (orgHandle: string) => Promise<boolean>;
  /** No-op — returns empty orgs; use getOrganizations instead */
  validateUser: () => Promise<ValidateUserResponse>;
  /** Calls GET /organizations on the Platform API */
  getOrganizations: () => Promise<Organization[]>;
  /** Always returns true (admin) in local dev */
  getIsOrgAdmin: (orgHandle: string) => Promise<boolean>;
}

const ChoreoUserContext = createContext<ChoreoUserContextType | null>(null);

// ── Helpers ───────────────────────────────────────────────────────────────────

/**
 * Fetch the organizations the current user is a member of from the Platform API.
 *
 * Routed same-origin through the BFF: the request carries the HttpOnly
 * `_ai_workspace_session` cookie (credentials: 'include') and the BFF injects the bearer
 * token. The browser holds no token, so no Authorization header is set here.
 */
async function fetchPlatformOrganization(): Promise<Organization[]> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  };

  // The API's `id` field is the organization's handle (a URL-safe slug), not
  // a UUID — there is no separate handle field, and no internal UUID is ever
  // exposed to clients.
  const mapOrg = (platformOrg: { id: string; displayName: string; region?: string }): Organization => ({
    id: platformOrg.id,
    uuid: platformOrg.id,
    handle: platformOrg.id,
    name: platformOrg.displayName,
    region: platformOrg.region,
    owner: { id: 0, idpId: '' },
  });

  // GET /organizations is paginated ({ count, list, pagination }); a caller who
  // belongs to more organizations than one page holds spans several pages, so
  // follow pagination.total and merge every page rather than keeping only the
  // first.
  const orgs: Organization[] = [];
  let offset = 0;
  for (;;) {
    const res = await fetch(`${PLATFORM_API_BASE_URL}/organizations?offset=${offset}`, {
      credentials: 'include',
      headers,
    });

    if (!res.ok) {
      if (res.status === 404) {
        logger.warn('[ChoreoUserContext] No organization found — register one at /register-org');
        return [];
      }
      const body = await res.json().catch(() => ({}));
      throw new Error(body?.message ?? `GET /organizations failed: HTTP ${res.status}`);
    }

    const body = await res.json();
    const page: Array<{ id: string; displayName: string; region?: string }> = Array.isArray(body?.list)
      ? body.list
      : [];
    orgs.push(...page.map(mapOrg));

    const total = typeof body?.pagination?.total === 'number' ? body.pagination.total : orgs.length;
    // Stop once every organization is collected, or defensively if a page comes
    // back empty (so a stale/growing total can never spin this forever).
    if (orgs.length >= total || page.length === 0) {
      break;
    }
    offset = orgs.length;
  }

  logger.info('[ChoreoUserContext] Loaded organizations:', orgs.map((o) => o.id));
  return orgs;
}

// ── Provider ──────────────────────────────────────────────────────────────────

export const ChoreoUserProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [isTokenExchanged, setIsTokenExchanged] = useState(false);
  const [isOrgAdmin, setIsOrgAdmin] = useState(true);

  // No token exchange needed — always return admin:true
  const exchangeOrgToken = useCallback(async (_orgHandle: string): Promise<boolean> => {
    logger.info('[ChoreoUserContext] exchangeOrgToken — no-op in platform mode');
    return true;
  }, []);

  // Not used in platform mode
  const validateUser = useCallback(async (): Promise<ValidateUserResponse> => {
    logger.info('[ChoreoUserContext] validateUser — no-op in platform mode');
    return { organizations: [], idpId: '' };
  }, []);

  // Real call to Platform API
  const getOrganizations = useCallback(async (): Promise<Organization[]> => {
    return fetchPlatformOrganization();
  }, []);

  // Admin check — always true in local dev
  const getIsOrgAdmin = useCallback(async (_orgHandle: string): Promise<boolean> => {
    return true;
  }, []);

  return (
    <ChoreoUserContext.Provider
      value={{
        isTokenExchanged,
        setIsTokenExchanged,
        isOrgAdmin,
        setIsOrgAdmin,
        shouldValidateUser: false,
        fidp: '',
        exchangeOrgToken,
        validateUser,
        getOrganizations,
        getIsOrgAdmin,
      }}
    >
      {children}
    </ChoreoUserContext.Provider>
  );
};

export const useChoreoUser = (): ChoreoUserContextType => {
  const ctx = useContext(ChoreoUserContext);
  if (!ctx) throw new Error('useChoreoUser must be used within a ChoreoUserProvider');
  return ctx;
};

export default ChoreoUserContext;
