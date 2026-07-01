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
import { getProjects } from '../apis/projectApis';
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
}

const defaultContextValue: AppShellContextType = {
  userName: null,
  userEmail: null,
  currentOrganization: null,
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
      const projectList = await getProjects();
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

  const initialize = useCallback(async () => {
    try {
      const tokenOrg = userRef.current?.org;

      if (tokenOrg?.id) {
        // Primary path: fetch org by UUID from the token (works for both OIDC and file-based auth).
        let platformOrg = await getOrganizationById(tokenOrg.id);

        if (!platformOrg && tokenOrg.handle) {
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
          platformOrg = await getOrganizationById(tokenOrg.id);
        }

        if (!platformOrg) {
          logger.warn('[AppShellContext] Org not found for id:', tokenOrg.id);
          setError('Organization not found. Please contact your administrator.');
          return;
        }

        setCurrentOrganizationState(toOrganization(platformOrg));
        setIsTokenExchanged(true);
        await fetchProjectsForOrg();
        return;
      }

      // Fallback: no org id in token — use list endpoint.
      const orgs = await getOrganizations();
      if (orgs.length === 0) {
        logger.warn('[AppShellContext] No organization found');
        setError('Organization not found. Please contact your administrator.');
        return;
      }
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
  }, [getOrganizations, fetchProjectsForOrg, setIsTokenExchanged]);

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
