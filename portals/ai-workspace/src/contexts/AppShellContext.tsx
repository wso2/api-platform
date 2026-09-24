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
import { usePlatformUser, type OrgSwitchFailure } from './PlatformUserContext';
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

/**
 * Turns an org-switch failure into user-facing words. One place, so the two causes
 * cannot drift back into a single message: `rejected` is a verdict about this user
 * that retrying will not change, while `unavailable` is a platform outage that
 * usually clears on its own. Telling someone to contact their administrator about a
 * blip — or telling someone genuinely without access to try again — is the failure
 * this function exists to prevent.
 */
function orgSwitchErrorMessage(reason: OrgSwitchFailure, orgName: string): string {
  switch (reason) {
    case 'rejected':
      return `You do not have access to ${orgName}. If you believe this is a mistake, `
        + 'contact your administrator.';
    case 'unavailable':
      return `${orgName} could not be opened because sign-in is temporarily unavailable. `
        + 'Please try again in a moment.';
    default:
      return `Could not open ${orgName}. Please try again.`;
  }
}

export const AppShellProvider: React.FC<AppShellProviderProps> = ({
  children,
  userName: initialUserName,
  userEmail: initialUserEmail,
}) => {
  const { setIsTokenExchanged, getOrganizations, exchangeOrgToken } = usePlatformUser();
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

  // Guards against an earlier, slower fetchProjectsForOrg call (e.g. from a
  // superseded organization switch) overwriting state with stale results
  // after a later call has already resolved.
  const fetchGenerationRef = useRef(0);

  const fetchProjectsForOrg = useCallback(async (): Promise<ProjectBase[]> => {
    const generation = ++fetchGenerationRef.current;
    setIsProjectsLoading(true);
    try {
      let projectList = await getProjects();
      if (projectList.length === 0) {
        await createDefaultProject();
        projectList = await getProjects();
      }
      if (generation === fetchGenerationRef.current) {
        setProjectsForCurrentOrganization(projectList);
        setCurrentProjectState(null);
      }
      return projectList;
    } catch (err) {
      logger.error('Failed to fetch projects:', err);
      if (generation === fetchGenerationRef.current) {
        setProjectsForCurrentOrganization([]);
      }
      return [];
    } finally {
      if (generation === fetchGenerationRef.current) {
        setIsProjectsLoading(false);
      }
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

  const initialize = useCallback(async () => {
    try {
      const tokenOrg = userRef.current?.org;

      const orgsPromise = getOrganizations();

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

        setIsOrganizationsLoading(true);
        try {
          const orgs = await orgsPromise;
          setOrganizations(
            orgs.some((o) => o.handle === resolvedOrg.handle) ? orgs : [...orgs, resolvedOrg]
          );
        } finally {
          setIsOrganizationsLoading(false);
        }

        const resolvedExchange = await exchangeOrgToken(resolvedOrg.handle);
        if (!resolvedExchange.ok) {
          setError(orgSwitchErrorMessage(resolvedExchange.reason, resolvedOrg.name));
          return;
        }
        setIsTokenExchanged(true);
        await fetchProjectsForOrg();
        return;
      }

      setIsOrganizationsLoading(true);
      let orgs: Organization[];
      try {
        orgs = await orgsPromise;
      } finally {
        setIsOrganizationsLoading(false);
      }
      if (orgs.length === 0) {
        logger.warn('[AppShellContext] No organization found');
        setError('Organization not found. Please contact your administrator.');
        return;
      }
      setOrganizations(orgs);
      setCurrentOrganizationState(orgs[0]);
      const firstOrgExchange = await exchangeOrgToken(orgs[0].handle);
      if (!firstOrgExchange.ok) {
        setError(orgSwitchErrorMessage(firstOrgExchange.reason, orgs[0].name));
        return;
      }
      setIsTokenExchanged(true);
      await fetchProjectsForOrg();
    } catch (err: any) {
      logger.error('Initialization failed:', err);
      setIsProvisioning(false);
      setError(`Failed to initialize: ${err?.message ?? 'Unknown error'}`);
    } finally {
      setIsLoading(false);
    }
  }, [getOrganizations, fetchProjectsForOrg, setIsTokenExchanged, exchangeOrgToken]);

  const switchOrganization = useCallback(
    async (organization: Organization) => {
      if (organization.handle === currentOrganization?.handle) {
        return;
      }
      const switched = await exchangeOrgToken(organization.handle);
      if (!switched.ok) {
        setError(orgSwitchErrorMessage(switched.reason, organization.name));
        return;
      }
      setCurrentOrganizationState(organization);
      await fetchProjectsForOrg();
    },
    [currentOrganization?.handle, fetchProjectsForOrg, exchangeOrgToken]
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
