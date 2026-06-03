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
// AppShellContext — Platform API (standalone) version
// ----------------------------------------------------------------------------
// Asgardeo authentication has been removed.
// Organization and project data are loaded directly from the Platform API.
// No token exchange — the stored bearer token is used for all authorized calls.
// ============================================================================

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

// ── Types ─────────────────────────────────────────────────────────────────────

export interface AppShellContextType {
  userName: string | null;
  userEmail: string | null;
  currentOrganization: Organization | null;
  organizations: Organization[];
  currentProject: ProjectBase | null;
  projectsForCurrentOrganization: ProjectBase[];
  isProjectsLoading: boolean;
  isTokenExchanged: boolean;
  isLoading: boolean;
  error: string | null;
  setCurrentOrganization: (org: Organization) => Promise<void>;
  setCurrentProject: (project: ProjectBase | null) => void;
  refetchProjects: () => Promise<void>;
}

const defaultContextValue: AppShellContextType = {
  userName: null,
  userEmail: null,
  currentOrganization: null,
  organizations: [],
  currentProject: null,
  projectsForCurrentOrganization: [],
  isProjectsLoading: false,
  isTokenExchanged: true,
  isLoading: true,
  error: null,
  setCurrentOrganization: async () => {},
  setCurrentProject: () => {},
  refetchProjects: async () => {},
};

const AppShellContext = createContext<AppShellContextType>(defaultContextValue);

const getOrgHandleFromUrl = (): string | null => {
  const match = window.location.pathname.match(/^\/organizations\/([^/]+)/);
  return match ? match[1] : null;
};

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
  const { setIsTokenExchanged, getOrganizations, setOrganizations } = useChoreoUser();

  const isInitializedRef = useRef(false);
  const isOrgChangeInProgressRef = useRef(false);

  // In standalone mode there is no Asgardeo session — use props or empty strings
  const userName: string | null = initialUserName || null;
  const userEmail: string | null = initialUserEmail || null;

  const [currentOrganization, setCurrentOrganizationState] = useState<Organization | null>(null);
  const [organizations, setOrganizationsState] = useState<Organization[]>([]);
  const [projectsForCurrentOrganization, setProjectsForCurrentOrganization] = useState<ProjectBase[]>([]);
  const [currentProject, setCurrentProjectState] = useState<ProjectBase | null>(null);
  const [isProjectsLoading, setIsProjectsLoading] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // ── Project fetching ────────────────────────────────────────────────────────

  const fetchProjectsForOrg = useCallback(async (): Promise<ProjectBase[]> => {
    setIsProjectsLoading(true);
    try {
      // Platform API: GET /projects is org-scoped via JWT — no org ID needed
      const projectList = await getProjects();
      setProjectsForCurrentOrganization(projectList);
      setCurrentProjectState(null);
      return projectList;
    } catch (err) {
      logger.error('Failed to fetch projects:', err);
      return [];
    } finally {
      setIsProjectsLoading(false);
    }
  }, []);

  // ── Organization switch ──────────────────────────────────────────────────────

  const setCurrentOrganization = useCallback(
    async (org: Organization) => {
      if (isOrgChangeInProgressRef.current) return;
      if (currentOrganization?.id === org.id) return;

      isOrgChangeInProgressRef.current = true;
      setCurrentOrganizationState(org);
      setCurrentProjectState(null);
      setProjectsForCurrentOrganization([]);
      setError(null);

      try {
        sessionStorage.setItem('currentOrgHandle', org.handle);
        setIsTokenExchanged(true);
        await fetchProjectsForOrg();
      } catch (err) {
        logger.error('Failed to switch organization:', err);
        setError('Failed to switch organization');
      } finally {
        isOrgChangeInProgressRef.current = false;
      }
    },
    [currentOrganization, fetchProjectsForOrg, setIsTokenExchanged],
  );

  const setCurrentProject = useCallback((project: ProjectBase | null) => {
    setCurrentProjectState(project);
  }, []);

  const refetchProjects = useCallback(async () => {
    await fetchProjectsForOrg();
  }, [fetchProjectsForOrg]);

  // ── Initialization ───────────────────────────────────────────────────────────
  // Runs once on mount — no Asgardeo auth required.

  const initialize = useCallback(async () => {
    // When we hard-redirect to /register-org keep the spinner up (page is leaving)
    let keepLoading = false;
    try {
      const storedOrgHandle = sessionStorage.getItem('currentOrgHandle');
      const urlOrgHandle = getOrgHandleFromUrl();

      const orgs = await getOrganizations();
      setOrganizations(orgs);
      setOrganizationsState(orgs);

      if (orgs.length === 0) {
        logger.warn('No organization found. Please register at /register-org');
        if (!window.location.pathname.startsWith('/register-org')) {
          keepLoading = true;
          window.location.href = '/register-org';
        }
        return;
      }

      // If the token carries an org UUID that doesn't match any registered org,
      // the user needs to register a new org for that identity.
      const pendingOrgUuid = sessionStorage.getItem('pending_org_uuid');
      if (pendingOrgUuid && !orgs.some((o) => o.id === pendingOrgUuid || o.uuid === pendingOrgUuid)) {
        logger.warn('Token org UUID does not match any registered org. Redirecting to /register-org');
        if (!window.location.pathname.startsWith('/register-org')) {
          keepLoading = true;
          window.location.href = '/register-org';
        }
        return;
      }

      // Determine which org to display
      let targetOrg = orgs[0];
      if (urlOrgHandle) {
        const found = orgs.find((o) => o.handle === urlOrgHandle);
        if (found) targetOrg = found;
      } else if (storedOrgHandle) {
        const found = orgs.find((o) => o.handle === storedOrgHandle);
        if (found) targetOrg = found;
      }

      setCurrentOrganizationState(targetOrg);
      sessionStorage.setItem('currentOrgHandle', targetOrg.handle);
      setIsTokenExchanged(true);

      await fetchProjectsForOrg();
    } catch (err: any) {
      logger.error('Initialization failed:', err);
      setError(`Failed to initialize: ${err?.message ?? 'Unknown error'}`);
    } finally {
      // keepLoading stays true when we triggered a hard redirect to /register-org
      // so the loading spinner remains visible while the page navigates away.
      if (!keepLoading) setIsLoading(false);
    }
  }, [getOrganizations, setOrganizations, fetchProjectsForOrg, setIsTokenExchanged]);

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
    currentProject,
    projectsForCurrentOrganization,
    isProjectsLoading,
    isTokenExchanged: true,
    isLoading,
    error,
    setCurrentOrganization,
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
