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
  useEffect,
  useMemo,
  useState,
  useCallback,
} from 'react';
import type {
  Application,
  ApplicationListResponse,
  APIKeyMappingListQueryParams,
  CreateApplicationRequest,
  UpdateApplicationRequest,
  MappedAPIKeyListResponse,
  RemoveApplicationAPIKeyOptions,
  AddApplicationAPIKeysRequest,
} from '../utils/types';
export type { Application } from '../utils/types';
import { applicationApis } from '../apis/applicationApis';
import { useAppShell } from './AppShellContext';
import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';

// ============================================================================
// Applications List Context
// ============================================================================

const EMPTY_APPLICATIONS_RESPONSE: ApplicationListResponse = {
  count: 0,
  list: [],
  pagination: { total: 0, offset: 0, limit: 20 },
};

type ApplicationsContextValue = {
  /** Convenience accessor for the applications list */
  applications: Application[];
  /** Full API response with count, list, and pagination */
  applicationsResponse: ApplicationListResponse;
  isLoading: boolean;
  error: Error | null;
  /** Add an application locally (backward-compatible) */
  addApplication: (application: Application) => void;
  createApplication: (
    application: CreateApplicationRequest
  ) => Promise<Application>;
  updateApplication: (
    appId: string,
    updates: UpdateApplicationRequest
  ) => Promise<Application>;
  deleteApplication: (appId: string) => Promise<void>;
  refreshApplications: () => Promise<void>;
  getApplicationById: (appId: string) => Application | undefined;
  // API Key mapping operations
  getApplicationAPIKeys: (
    appId: string,
    options?: APIKeyMappingListQueryParams
  ) => Promise<MappedAPIKeyListResponse>;
  addApplicationAPIKeys: (
    appId: string,
    request: AddApplicationAPIKeysRequest
  ) => Promise<MappedAPIKeyListResponse>;
  removeApplicationAPIKey: (
    appId: string,
    mappedKeyId: string,
    options?: RemoveApplicationAPIKeyOptions
  ) => Promise<void>;
};

const ApplicationsContext = createContext<ApplicationsContextValue>({
  applications: [],
  applicationsResponse: EMPTY_APPLICATIONS_RESPONSE,
  isLoading: false,
  error: null,
  addApplication: () => {},
  createApplication: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
  updateApplication: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
  deleteApplication: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
  refreshApplications: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
  getApplicationById: () => undefined,
  getApplicationAPIKeys: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
  addApplicationAPIKeys: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
  removeApplicationAPIKey: async () => {
    throw new Error('ApplicationsContext not initialized');
  },
});

interface ApplicationsProviderProps {
  children: React.ReactNode;
}

export function ApplicationsProvider({ children }: ApplicationsProviderProps) {
  const { currentOrganization, currentProject } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [applicationsResponse, setApplicationsResponse] =
    useState<ApplicationListResponse>(EMPTY_APPLICATIONS_RESPONSE);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';
  const projectId = currentProject?.id ?? '';

  // Fetch all applications
  const fetchApplications = useCallback(async () => {
    if (!organizationId) {
      setApplicationsResponse(EMPTY_APPLICATIONS_RESPONSE);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const response = await applicationApis.getApplications(
        organizationId,
        apimBaseUrl,
        {
          projectId: projectId || undefined,
        }
      );
      setApplicationsResponse(response as ApplicationListResponse);
    } catch (err) {
      logger.error('Failed to fetch applications:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch applications')
      );
    } finally {
      setIsLoading(false);
    }
  }, [organizationId, projectId, apimBaseUrl]);

  useEffect(() => {
    fetchApplications();
  }, [fetchApplications]);

  const addApplication = useCallback((application: Application) => {
    setApplicationsResponse((prev) => ({
      ...prev,
      count: prev.count + 1,
      list: [application, ...prev.list],
      pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
    }));
  }, []);

  const createApplication = useCallback(
    async (application: CreateApplicationRequest): Promise<Application> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        const newApplication = await applicationApis.createApplication(
          application,
          organizationId,
          apimBaseUrl
        );
        setApplicationsResponse((prev) => ({
          ...prev,
          count: prev.count + 1,
          list: [newApplication, ...prev.list],
          pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
        }));
        return newApplication;
      } catch (err) {
        logger.error('Failed to create application:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const updateApplication = useCallback(
    async (
      appId: string,
      updates: UpdateApplicationRequest
    ): Promise<Application> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        const updatedApplication = await applicationApis.updateApplication(
          appId,
          updates,
          organizationId,
          apimBaseUrl
        );
        setApplicationsResponse((prev) => ({
          ...prev,
          list: prev.list.map((app) =>
            app.id === appId ? updatedApplication : app
          ),
        }));
        return updatedApplication;
      } catch (err) {
        logger.error('Failed to update application:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const deleteApplication = useCallback(
    async (appId: string): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        await applicationApis.deleteApplication(
          appId,
          organizationId,
          apimBaseUrl
        );
        setApplicationsResponse((prev) => ({
          ...prev,
          count: Math.max(0, prev.count - 1),
          list: prev.list.filter((app) => app.id !== appId),
          pagination: {
            ...prev.pagination,
            total: Math.max(0, prev.pagination.total - 1),
          },
        }));
      } catch (err) {
        logger.error('Failed to delete application:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const refreshApplications = useCallback(async (): Promise<void> => {
    await fetchApplications();
  }, [fetchApplications]);

  const getApplicationById = useCallback(
    (appId: string): Application | undefined => {
      return applicationsResponse.list.find((app) => app.id === appId);
    },
    [applicationsResponse.list]
  );

  // API Key mapping operations
  const getApplicationAPIKeys = useCallback(
    async (
      appId: string,
      options?: APIKeyMappingListQueryParams
    ): Promise<MappedAPIKeyListResponse> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        return await applicationApis.getApplicationAPIKeys(
          appId,
          organizationId,
          apimBaseUrl,
          options
        );
      } catch (err) {
        logger.error(`Failed to fetch API keys for application ${appId}:`, err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const addApplicationAPIKeys = useCallback(
    async (
      appId: string,
      request: AddApplicationAPIKeysRequest
    ): Promise<MappedAPIKeyListResponse> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        return await applicationApis.addApplicationAPIKeys(
          appId,
          request,
          organizationId,
          apimBaseUrl
        );
      } catch (err) {
        logger.error(`Failed to add API keys to application ${appId}:`, err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const removeApplicationAPIKey = useCallback(
    async (
      appId: string,
      mappedKeyId: string,
      options?: RemoveApplicationAPIKeyOptions
    ): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        await applicationApis.removeApplicationAPIKey(
          appId,
          mappedKeyId,
          organizationId,
          apimBaseUrl,
          options
        );
      } catch (err) {
        logger.error(
          `Failed to remove API key ${mappedKeyId} from application ${appId}:`,
          err
        );
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const value = useMemo(
    () => ({
      applications: applicationsResponse.list,
      applicationsResponse,
      isLoading,
      error,
      addApplication,
      createApplication,
      updateApplication,
      deleteApplication,
      refreshApplications,
      getApplicationById,
      getApplicationAPIKeys,
      addApplicationAPIKeys,
      removeApplicationAPIKey,
    }),
    [
      applicationsResponse,
      isLoading,
      error,
      addApplication,
      createApplication,
      updateApplication,
      deleteApplication,
      refreshApplications,
      getApplicationById,
      getApplicationAPIKeys,
      addApplicationAPIKeys,
      removeApplicationAPIKey,
    ]
  );

  return (
    <ApplicationsContext.Provider value={value}>
      {children}
    </ApplicationsContext.Provider>
  );
}

export function useApplications(): ApplicationsContextValue {
  return useContext(ApplicationsContext);
}

export function formatRelativeTime(value?: string): string {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown';

  const now = Date.now();
  const diffMs = now - date.getTime();
  const diffSeconds = Math.abs(diffMs) / 1000;

  if (diffSeconds < 45) return 'Just now';
  if (diffSeconds < 90) return '1 minute ago';

  const diffMinutes = diffSeconds / 60;
  if (diffMinutes < 45) return `${Math.round(diffMinutes)} minutes ago`;
  if (diffMinutes < 90) return '1 hour ago';

  const diffHours = diffMinutes / 60;
  if (diffHours < 22) return `${Math.round(diffHours)} hours ago`;
  if (diffHours < 36) return '1 day ago';

  const diffDays = diffHours / 24;
  if (diffDays < 26) return `${Math.round(diffDays)} days ago`;
  if (diffDays < 45) return '1 month ago';

  const diffMonths = diffDays / 30;
  if (diffMonths < 11) return `${Math.round(diffMonths)} months ago`;
  if (diffMonths < 18) return '1 year ago';

  const diffYears = diffDays / 365;
  return `${Math.round(diffYears)} years ago`;
}

export function buildApplicationId(
  name: string,
  takenIds: Set<string>
): string {
  const base = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');

  if (!takenIds.has(base)) return base;

  let suffix = 2;
  while (takenIds.has(`${base}-${suffix}`)) {
    suffix += 1;
  }
  return `${base}-${suffix}`;
}
