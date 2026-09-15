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

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  useCallback,
  ReactNode,
} from 'react';
import { logger } from '../utils/logger';
import { getProjects, createDefaultProject } from '../apis/projectApis';
import type { Organization, ProjectBase } from '../utils/types';
import { useChoreoUser } from './ChoreoUserContext';
import { useAppAuth } from './AppAuthContext';
import { registerOrganization, getOrganizationById } from '../apis/platformApis';
import type { PlatformOrganization } from '../apis/platformApis';
import { DEFAULT_ORG_REGION } from '../config.env';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface AppShellContextType {
  userName: string | null;
  userEmail: string | null;
  currentOrganization: Organization | null;
  organizations: Organization[];
  isOrganizationsLoading: boolean;
  currentProject: ProjectBase | null;
  projectsForCurrentOrganization: ProjectBase[];
  isProjectsLoading: boolean;
  isTokenExchanged: boolean;
  isLoading: boolean;
  isProvisioning: boolean;
  provisioningOrgName: string | null;
  error: string | null;
  setCurrentProject: (project: ProjectBase | null) => void;
  refetchProjects: () => Promise<void>;
  switchOrganization: (organization: Organization) => Promise<void>;
}

const defaultContextValue: AppShellContextType = {
  userName: null,
  userEmail: null,
  currentOrganization: null,
  organizations: [],
  isOrganizationsLoading: false,
  currentProject: null,
  projectsForCurrentOrganization: [],
  isProjectsLoading: false,
  isTokenExchanged: true,
  isLoading: true,
  isProvisioning: false,
  provisioningOrgName: null,
  error: null,
  setCurrentProject: () => {},
  refetchProjects: async () => {},
  switchOrganization: async () => {},
};

const AppShellContext = createContext<AppShellContextType>(defaultContextValue);

interface AppShellProviderProps {
  children: ReactNode;
  userName?: string;
  userEmail?: string;
}

export const AppShellProvider: React.FC<AppShellProviderProps> = ({
  children,
  userName: initialUserName,
  userEmail: initialUserEmail,
}) => {
  const { setIsTokenExchanged, getOrganizations } = useChoreoUser();
  const { user } = useAppAuth();

  const isInitializedRef = useRef(false);
  // Keep a ref to avoid stale closure in initialize callback
  const userRef = useRef(user);
  useEffect(() => { userRef.current = user; }, [user]);

  const userName: string | null = initialUserName || null;
  const userEmail: string | null = initialUserEmail || null;

  const [currentOrganization, setCurrentOrganizationState] = useState<Organization | null>(null);
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [isOrganizationsLoading, setIsOrganizationsLoading] = useState(false);
  const [projectsForCurrentOrganization, setProjectsForCurrentOrganization] = useState<ProjectBase[]>([]);
  const [currentProject, setCurrentProjectState] = useState<ProjectBase | null>(null);
  const [isProjectsLoading, setIsProjectsLoading] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isProvisioning, setIsProvisioning] = useState(false);
  const [provisioningOrgName, setProvisioningOrgName] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // ── Project fetching ────────────────────────────────────────────────────────

  const fetchProjectsForOrg = useCallback(async (): Promise<ProjectBase[]> => {
    setIsProjectsLoading(true);
    try {
      let projectList = await getProjects();
      if (projectList.length === 0) {
        await createDefaultProject();
        projectList = await getProjects();
      }
      setProjectsForCurrentOrganization(projectList);
      setCurrentProjectState(null);
      return projectList;
    } catch (err) {
      logger.error('Failed to fetch projects:', err);
      setProjectsForCurrentOrganization([]);
      return [];
    } finally {
      setIsProjectsLoading(false);
    }
  }, []);

  const setCurrentProject = useCallback((project: ProjectBase | null) => {
    setCurrentProjectState(project);
  }, []);

  const refetchProjects = useCallback(async () => {
    await fetchProjectsForOrg();
  }, [fetchProjectsForOrg]);

  // ── Initialization ───────────────────────────────────────────────────────────

  const toOrganization = (p: PlatformOrganization): Organization => ({
    id: p.id,
    uuid: p.id,
    handle: p.id,
    name: p.displayName,
    region: p.region,
    owner: { id: 0, idpId: '' },
  });

  // Resolves the user's organization ids (from the token's "organizations" claim)
  // into full Organization objects by looking each one up individually — the
  // claim carries ids only, never display names, and only ever the calling
  // user's own memberships, so each id is resolved one at a time rather than
  // ever falling back to the unscoped /organizations list endpoint, which (for
  // a caller holding ap:organization:manage) returns every organization on the
  // platform, not just the caller's own — see initialize() for the no-claim case.
  const loadOrganizationsByIds = useCallback(async (orgIds: string[]): Promise<Organization[]> => {
    if (orgIds.length === 0) return [];
    setIsOrganizationsLoading(true);
    try {
      const resolved = await Promise.all(
        orgIds.map(async (id) => {
          try {
            const platformOrg = await getOrganizationById(id);
            return platformOrg ? toOrganization(platformOrg) : null;
          } catch (err) {
            logger.error(`[AppShellContext] Failed to resolve organization ${id}:`, err);
            return null;
          }
        })
      );
      return resolved.filter((org): org is Organization => org !== null);
    } finally {
      setIsOrganizationsLoading(false);
    }
  }, []);

  const initialize = useCallback(async () => {
    try {
      const tokenOrg = userRef.current?.org;
      const orgIds = userRef.current?.organizations ?? [];

      // Kicked off in parallel with resolving the "current" org below — only
      // used when the token actually carries an "organizations" claim.
      const claimOrgsPromise = loadOrganizationsByIds(orgIds);

      if (tokenOrg?.handle) {
        // Primary path: fetch org by handle from the token (works for both OIDC and file-based auth).
        let platformOrg = await getOrganizationById(tokenOrg.handle);

        if (!platformOrg) {
          // Org not registered yet — provision it from token claims.
          const displayName = tokenOrg.name || tokenOrg.handle;
          logger.info('[AppShellContext] Auto-provisioning organization:', tokenOrg.handle);
          setIsProvisioning(true);
          setProvisioningOrgName(displayName);
          try {
            await registerOrganization({
              id: tokenOrg.handle,
              displayName,
              region: DEFAULT_ORG_REGION,
            });
          } catch (provisionErr: any) {
            // 409 = already exists (race), safe to continue
            if (!provisionErr?.message?.includes('already exists')) {
              throw provisionErr;
            }
          }
          setIsProvisioning(false);
          platformOrg = await getOrganizationById(tokenOrg.handle);
        }

        if (!platformOrg) {
          logger.warn('[AppShellContext] Org not found for handle:', tokenOrg.handle);
          setError('Organization not found. Please contact your administrator.');
          return;
        }

        const resolvedOrg = toOrganization(platformOrg);
        setCurrentOrganizationState(resolvedOrg);

        if (orgIds.length > 0) {
          // Token carries an "organizations" claim — the switcher offers exactly
          // those memberships. A just-provisioned org may not yet appear in that
          // resolved set (e.g. a race with the claim being minted), so make sure
          // the switcher still offers it.
          const claimOrgs = await claimOrgsPromise;
          setOrganizations(
            claimOrgs.some((o) => o.handle === resolvedOrg.handle)
              ? claimOrgs
              : [...claimOrgs, resolvedOrg]
          );
        } else {
          // No "organizations" claim on the token — show only the single
          // current organization, never the unscoped /organizations list.
          setOrganizations([resolvedOrg]);
        }
        setIsTokenExchanged(true);
        await fetchProjectsForOrg();
        return;
      }

      // Fallback: no "org" claim in the token at all — nothing to resolve a
      // "current" org from, so bootstrap off whatever org list is available:
      // the claim-resolved set if the token had one, otherwise the unscoped
      // list endpoint (there's no narrower option when neither claim exists).
      const orgs = orgIds.length > 0 ? await claimOrgsPromise : await getOrganizations();
      if (orgs.length === 0) {
        logger.warn('[AppShellContext] No organization found');
        setError('Organization not found. Please contact your administrator.');
        return;
      }
      setOrganizations(orgs);
      setCurrentOrganizationState(orgs[0]);
      setIsTokenExchanged(true);
      await fetchProjectsForOrg();
    } catch (err: any) {
      logger.error('Initialization failed:', err);
      setIsProvisioning(false);
      setError(`Failed to initialize: ${err?.message ?? 'Unknown error'}`);
    } finally {
      setIsLoading(false);
    }
  }, [loadOrganizationsByIds, getOrganizations, fetchProjectsForOrg, setIsTokenExchanged]);

  const switchOrganization = useCallback(
    async (organization: Organization) => {
      if (organization.handle === currentOrganization?.handle) {
        return;
      }
      setCurrentOrganizationState(organization);
      await fetchProjectsForOrg();
    },
    [currentOrganization?.handle, fetchProjectsForOrg]
  );

  useEffect(() => {
    if (isInitializedRef.current) return;
    isInitializedRef.current = true;
    initialize();
  }, [initialize]);

  // ── Context value ─────────────────────────────────────────────────────────────

  const contextValue: AppShellContextType = {
    userName,
    userEmail,
    currentOrganization,
    organizations,
    isOrganizationsLoading,
    currentProject,
    projectsForCurrentOrganization,
    isProjectsLoading,
    isTokenExchanged: true,
    isLoading,
    isProvisioning,
    provisioningOrgName,
    error,
    setCurrentProject,
    refetchProjects,
    switchOrganization,
  };

  return (
    <AppShellContext.Provider value={contextValue}>
      {children}
    </AppShellContext.Provider>
  );
};

export const useAppShell = (): AppShellContextType => {
  const context = useContext(AppShellContext);
  if (!context) throw new Error('useAppShell must be used within an AppShellProvider');
  return context;
};

export default AppShellContext;
