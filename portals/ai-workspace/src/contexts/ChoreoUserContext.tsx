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
// Platform API (https://localhost:9243/api/v1).
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
import { getOrgToken } from '../clients/choreoApiClient';

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
 * Fetch the current user's organization from the Platform API.
 * Returns a single org wrapped in an array (Platform API returns one org per JWT).
 *
 * TODO: [REMOVE BEFORE PRODUCTION] getStoredToken() falls back to a hardcoded
 *       dev token in choreoApiClient.ts. Remove DEV_FALLBACK_TOKEN once real
 *       auth is wired up.
 */
async function fetchPlatformOrganization(): Promise<Organization[]> {
  // getStoredToken() always returns a string (sessionStorage value or DEV_FALLBACK_TOKEN)
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    Authorization: `Bearer ${getOrgToken()}`,
  };

  const res = await fetch(`${PLATFORM_API_BASE_URL}/organizations`, { headers });

  if (!res.ok) {
    if (res.status === 404) {
      logger.warn('[ChoreoUserContext] No organization found — register one at /register-org');
      return [];
    }
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.message ?? `GET /organizations failed: HTTP ${res.status}`);
  }

  // Platform API returns a single Organization object (not an array)
  const platformOrg = await res.json();
  logger.info('[ChoreoUserContext] Loaded organization:', platformOrg.handle);

  const org: Organization = {
    id: platformOrg.id,      // UUID string
    uuid: platformOrg.id,    // alias kept for backward compat
    handle: platformOrg.handle,
    name: platformOrg.name,
    region: platformOrg.region,
    owner: { id: 0, idpId: '' },
  };
  return [org];
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
