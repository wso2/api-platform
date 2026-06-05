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

  const isInitializedRef = useRef(false);

  const userName: string | null = initialUserName || null;
  const userEmail: string | null = initialUserEmail || null;

  const [currentOrganization, setCurrentOrganizationState] = useState<Organization | null>(null);
  const [projectsForCurrentOrganization, setProjectsForCurrentOrganization] = useState<ProjectBase[]>([]);
  const [currentProject, setCurrentProjectState] = useState<ProjectBase | null>(null);
  const [isProjectsLoading, setIsProjectsLoading] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
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

  const initialize = useCallback(async () => {
    let keepLoading = false;
    try {
      const orgs = await getOrganizations();

      if (orgs.length === 0) {
        logger.warn('No organization found. Please register at /register-org');
        if (!window.location.pathname.startsWith('/register-org')) {
          keepLoading = true;
          window.location.href = '/register-org';
        }
        return;
      }

      const org = orgs[0];
      setCurrentOrganizationState(org);
      setIsTokenExchanged(true);

      await fetchProjectsForOrg();
    } catch (err: any) {
      logger.error('Initialization failed:', err);
      setError(`Failed to initialize: ${err?.message ?? 'Unknown error'}`);
    } finally {
      if (!keepLoading) setIsLoading(false);
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
