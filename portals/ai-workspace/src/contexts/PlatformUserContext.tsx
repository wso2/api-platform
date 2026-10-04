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
// PlatformUserContext — Platform API (standalone) version
// ----------------------------------------------------------------------------
// Asgardeo authentication has been removed. All org data comes from the
// Platform API (https://localhost:9243/api/v0.9). exchangeOrgToken calls the
// BFF's org-scoped token exchange (see internal/server/handlers.go
// handleSwitchOrg); the rest of the IDP-specific surface stays a no-op.
// ============================================================================

import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode,
} from 'react';
import { logger } from '../utils/logger';
import { useAppAuth } from './AppAuthContext';
import type { Organization, ValidateUserResponse } from '../utils/types';
import { PLATFORM_API_BASE_URL, BASE_PATH } from '../paths';
import { CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { handleUnauthorizedResponse } from '../auth/logout';

const BFF_SWITCH_ORG_URL = `${BASE_PATH}/api/session/org`;

// ── Types ─────────────────────────────────────────────────────────────────────

export interface PlatformUserContextType {
  isTokenExchanged: boolean;
  setIsTokenExchanged: React.Dispatch<React.SetStateAction<boolean>>;
  isOrgAdmin: boolean;
  setIsOrgAdmin: React.Dispatch<React.SetStateAction<boolean>>;
  /** Always false — no Asgardeo sign-in hook available */
  shouldValidateUser: boolean;
  /** Always '' — no IDP in standalone mode */
  fidp: string;
  /**
   * Re-exchange the login token for the given org. Resolves with why it failed, not
   * just that it did — see OrgSwitchFailure.
   */
  exchangeOrgToken: (orgHandle: string) => Promise<OrgSwitchResult>;
  /** No-op — returns empty orgs; use getOrganizations instead */
  validateUser: () => Promise<ValidateUserResponse>;
  /** Calls GET /organizations on the Platform API */
  getOrganizations: () => Promise<Organization[]>;
  /** Always returns true (admin) in local dev */
  getIsOrgAdmin: (orgHandle: string) => Promise<boolean>;
}

const PlatformUserContext = createContext<PlatformUserContextType | null>(null);

/**
 * Why an org switch failed, when it did.
 *
 * The BFF already tells these apart — 403 `ORG_SWITCH_REJECTED` versus 502
 * `UPSTREAM_UNAVAILABLE` (see `handleSwitchOrg` in internal/server/handlers.go) — and
 * they need different words. `rejected` is a verdict about this user that will not
 * clear on its own; `unavailable` is a platform outage that usually will. Collapsing
 * them tells someone to contact their administrator about a blip that would have
 * fixed itself, and tells someone genuinely lacking access to keep retrying.
 */
export type OrgSwitchFailure = 'rejected' | 'unavailable' | 'unknown';

/** Outcome of an org switch. A failure always carries why. */
export type OrgSwitchResult = { ok: true } | { ok: false; reason: OrgSwitchFailure };

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
        logger.warn('[PlatformUserContext] No organization found — register one at /register-org');
        return [];
      }
      const body = await res.json().catch(() => ({}));
      // Pass the code so only a genuine UNAUTHORIZED tears down the session.
      handleUnauthorizedResponse(res, body?.code);
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

  logger.info('[PlatformUserContext] Loaded organizations:', orgs.map((o) => o.id));
  return orgs;
}

// ── Provider ──────────────────────────────────────────────────────────────────

export const PlatformUserProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [isTokenExchanged, setIsTokenExchanged] = useState(false);
  const [isOrgAdmin, setIsOrgAdmin] = useState(true);
  // PlatformUserProvider is mounted inside BFFAuthProvider (AIWorkspace.tsx →
  // AppGate → App), so the auth context is available here.
  const { refreshSession } = useAppAuth();

  /**
   * Ask the BFF to re-exchange the login token for the given org, so an IDP that
   * mints org-scoped scopes issues a token for the org just selected rather than
   * whatever was last cached (see internal/server/handlers.go handleSwitchOrg).
   * A 400 (ORG_SCOPING_DISABLED) means the feature isn't configured for this
   * deployment — that's success from the caller's point of view, since there is
   * nothing to switch. Any other non-2xx (e.g. not a member of that org) is a
   * real failure: the caller should not proceed with the switch.
   *
   * On success the session is re-read before returning. A successful switch mints a
   * new token for the new org, which changes BOTH what the caller may do and which
   * org the platform considers them to be in — so leaving the auth context on its
   * pre-switch values shows the user one org's name and permissions while every API
   * call is scoped to another. The switch response reports the new scopes but not the
   * new org, so re-reading the session is what actually brings the UI back in step;
   * it is a cache hit on the token this switch just minted, not a second exchange.
   */
  const exchangeOrgToken = useCallback(async (orgHandle: string): Promise<OrgSwitchResult> => {
    try {
      const res = await fetch(BFF_SWITCH_ORG_URL, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
          [CSRF_HEADER]: CSRF_VALUE,
        },
        body: JSON.stringify({ org: orgHandle }),
      });
      if (res.ok) {
        // Awaited, so callers that gate on this promise (AppShellContext sets its own
        // org state and loads that org's projects the moment it resolves) never run
        // against a context still describing the org just switched away from.
        await refreshSession();
        setIsTokenExchanged(true);
        return { ok: true };
      }

      // Read the BFF's error code (the same {status, code, message} shape the Platform
      // API uses) rather than inferring everything from the status: 400 carries two
      // different meanings here and only one of them is a success.
      const body = (await res.json().catch(() => ({}))) as { code?: string };

      if (res.status === 400 && body.code === 'ORG_SCOPING_DISABLED') {
        logger.info('[PlatformUserContext] org-scoped token exchange is not configured — skipping');
        return { ok: true };
      }

      // A 401 here always means the BFF session itself is gone (unlike a proxied
      // Platform API 401, which can carry other causes) — no code to filter on. It is
      // a no-op for every other status, which is why it can sit ahead of them.
      handleUnauthorizedResponse(res);

      // Mapped from the status rather than the code so an unrecognised code still
      // lands in the right class; the codes are named above for traceability.
      const reason: OrgSwitchFailure = res.status === 403
        ? 'rejected'
        : res.status === 502
          ? 'unavailable'
          : 'unknown';
      logger.error('[PlatformUserContext] org token exchange failed for org', orgHandle,
        'status', res.status, 'code', body.code ?? '(none)', 'reason', reason);
      return { ok: false, reason };
    } catch (err) {
      // The BFF itself could not be reached — transient in the same way a 502 is, and
      // told to the user the same way.
      logger.error('[PlatformUserContext] org token exchange error:', err);
      return { ok: false, reason: 'unavailable' };
    }
  }, [setIsTokenExchanged, refreshSession]);

  // Not used in platform mode
  const validateUser = useCallback(async (): Promise<ValidateUserResponse> => {
    logger.info('[PlatformUserContext] validateUser — no-op in platform mode');
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
    <PlatformUserContext.Provider
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
    </PlatformUserContext.Provider>
  );
};

export const usePlatformUser = (): PlatformUserContextType => {
  const ctx = useContext(PlatformUserContext);
  if (!ctx) throw new Error('usePlatformUser must be used within a PlatformUserProvider');
  return ctx;
};

export default PlatformUserContext;
