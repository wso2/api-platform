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
  useCallback,
  useMemo,
} from 'react';
import type { ProjectBase } from '../utils/types';
import type { CreateProjectRequest, UpdateProjectRequest } from '../apis/projectApis';
import * as projectApis from '../apis/projectApis';
import { useAppShell } from './AppShellContext';
import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';

// ============================================================================
// Context Types
// ============================================================================

type ProjectsContextValue = {
  createProject: (request: CreateProjectRequest) => Promise<ProjectBase>;
  updateProject: (projectId: string, request: UpdateProjectRequest) => Promise<ProjectBase>;
  deleteProject: (projectId: string) => Promise<void>;
};

const ProjectsContext = createContext<ProjectsContextValue>({
  createProject: async () => { throw new Error('ProjectsContext not initialized'); },
  updateProject: async () => { throw new Error('ProjectsContext not initialized'); },
  deleteProject: async () => { throw new Error('ProjectsContext not initialized'); },
});

// ============================================================================
// Provider
// ============================================================================

interface ProjectsProviderProps {
  children: React.ReactNode;
}

export function ProjectsProvider({ children }: ProjectsProviderProps) {
  const { refetchProjects } = useAppShell();

  const createProject = useCallback(
    async (request: CreateProjectRequest): Promise<ProjectBase> => {
      try {
        const project = await projectApis.createProject(request, PLATFORM_API_BASE_URL);
        await refetchProjects();
        return project;
      } catch (error) {
        logger.error('Failed to create project:', error);
        throw error;
      }
    },
    [refetchProjects]
  );

  const updateProject = useCallback(
    async (projectId: string, request: UpdateProjectRequest): Promise<ProjectBase> => {
      try {
        const project = await projectApis.updateProject(projectId, request, PLATFORM_API_BASE_URL);
        await refetchProjects();
        return project;
      } catch (error) {
        logger.error('Failed to update project:', error);
        throw error;
      }
    },
    [refetchProjects]
  );

  const deleteProject = useCallback(
    async (projectId: string): Promise<void> => {
      try {
        await projectApis.deleteProject(projectId, PLATFORM_API_BASE_URL);
        await refetchProjects();
      } catch (error) {
        logger.error('Failed to delete project:', error);
        throw error;
      }
    },
    [refetchProjects]
  );

  const value = useMemo(
    () => ({ createProject, updateProject, deleteProject }),
    [createProject, updateProject, deleteProject]
  );

  return (
    <ProjectsContext.Provider value={value}>
      {children}
    </ProjectsContext.Provider>
  );
}

export function useProjects(): ProjectsContextValue {
  return useContext(ProjectsContext);
}
