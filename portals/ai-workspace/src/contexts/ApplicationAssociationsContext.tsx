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
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import type {
  ApplicationAssociation,
  ApplicationAssociationListResponse,
  AddApplicationAssociationsRequest,
  AddApplicationAPIKeysRequest,
  AssociationListQueryParams,
  MappedAPIKeyListResponse,
  RemoveApplicationAPIKeyOptions,
} from '../utils/types';
import { applicationApis } from '../apis/applicationApis';
import { useAppShell } from './AppShellContext';
import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';
import { getErrorMessage } from '../utils/apiError';

const EMPTY_ASSOCIATIONS_RESPONSE: ApplicationAssociationListResponse = {
  count: 0,
  list: [],
};

type ApplicationAssociationsContextValue = {
  associations: ApplicationAssociation[];
  associationsResponse: ApplicationAssociationListResponse;
  isLoading: boolean;
  error: string | null;
  refreshAssociations: () => Promise<void>;
  addAssociations: (
    request: AddApplicationAssociationsRequest
  ) => Promise<ApplicationAssociationListResponse>;
  removeAssociation: (associationId: string) => Promise<void>;
  listAssociationAPIKeys: (
    associationId: string,
    options?: AssociationListQueryParams
  ) => Promise<MappedAPIKeyListResponse>;
  addAPIKeys: (
    request: AddApplicationAPIKeysRequest
  ) => Promise<MappedAPIKeyListResponse>;
  removeAPIKey: (
    mappedKeyId: string,
    options: RemoveApplicationAPIKeyOptions
  ) => Promise<void>;
};

const ApplicationAssociationsContext =
  createContext<ApplicationAssociationsContextValue>({
    associations: [],
    associationsResponse: EMPTY_ASSOCIATIONS_RESPONSE,
    isLoading: false,
    error: null,
    refreshAssociations: async () => {
      throw new Error('ApplicationAssociationsContext not initialized');
    },
    addAssociations: async () => {
      throw new Error('ApplicationAssociationsContext not initialized');
    },
    removeAssociation: async () => {
      throw new Error('ApplicationAssociationsContext not initialized');
    },
    listAssociationAPIKeys: async () => {
      throw new Error('ApplicationAssociationsContext not initialized');
    },
    addAPIKeys: async () => {
      throw new Error('ApplicationAssociationsContext not initialized');
    },
    removeAPIKey: async () => {
      throw new Error('ApplicationAssociationsContext not initialized');
    },
  });

interface ApplicationAssociationsProviderProps {
  applicationId: string;
  children: React.ReactNode;
}

export function ApplicationAssociationsProvider({
  applicationId,
  children,
}: ApplicationAssociationsProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const organizationId = currentOrganization?.uuid ?? '';

  const [associationsResponse, setAssociationsResponse] =
    useState<ApplicationAssociationListResponse>(EMPTY_ASSOCIATIONS_RESPONSE);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchAssociations = useCallback(async () => {
    if (!applicationId || !organizationId) {
      setAssociationsResponse(EMPTY_ASSOCIATIONS_RESPONSE);
      return;
    }
    try {
      setIsLoading(true);
      setError(null);
      const response = await applicationApis.listApplicationAssociations(
        applicationId,
        apimBaseUrl
      );
      const seen = new Set<string>();
      const uniqueList = (response.list ?? []).filter((assoc) => {
        if (seen.has(assoc.id)) return false;
        seen.add(assoc.id);
        return true;
      });
      setAssociationsResponse({ ...response, list: uniqueList });
    } catch (err) {
      logger.error(
        `Failed to fetch associations for application ${applicationId}:`,
        err
      );
      setError(getErrorMessage(err, 'Failed to load associations.'));
    } finally {
      setIsLoading(false);
    }
  }, [applicationId, organizationId, apimBaseUrl]);

  useEffect(() => {
    void fetchAssociations();
  }, [fetchAssociations]);

  const addAssociations = useCallback(
    async (
      request: AddApplicationAssociationsRequest
    ): Promise<ApplicationAssociationListResponse> => {
      if (!organizationId) throw new Error('Organization ID is missing');
      try {
        const response = await applicationApis.addApplicationAssociations(
          applicationId,
          request,
          apimBaseUrl
        );
        await fetchAssociations();
        return response;
      } catch (err) {
        logger.error(
          `Failed to add associations to application ${applicationId}:`,
          err
        );
        throw err;
      }
    },
    [applicationId, organizationId, apimBaseUrl, fetchAssociations]
  );

  const removeAssociation = useCallback(
    async (associationId: string): Promise<void> => {
      if (!organizationId) throw new Error('Organization ID is missing');
      try {
        await applicationApis.removeApplicationAssociation(
          applicationId,
          associationId,
          apimBaseUrl
        );
        await fetchAssociations();
      } catch (err) {
        logger.error(
          `Failed to remove association ${associationId} from application ${applicationId}:`,
          err
        );
        throw err;
      }
    },
    [applicationId, organizationId, apimBaseUrl, fetchAssociations]
  );

  const listAssociationAPIKeys = useCallback(
    async (
      associationId: string,
      options?: AssociationListQueryParams
    ): Promise<MappedAPIKeyListResponse> => {
      if (!organizationId) throw new Error('Organization ID is missing');
      try {
        return await applicationApis.listApplicationAssociationAPIKeys(
          applicationId,
          associationId,
          apimBaseUrl,
          options
        );
      } catch (err) {
        logger.error(
          `Failed to list API keys for association ${associationId}:`,
          err
        );
        throw err;
      }
    },
    [applicationId, organizationId, apimBaseUrl]
  );

  const addAPIKeys = useCallback(
    async (request: AddApplicationAPIKeysRequest): Promise<MappedAPIKeyListResponse> => {
      if (!organizationId) throw new Error('Organization ID is missing');
      try {
        return await applicationApis.addApplicationAPIKeys(
          applicationId,
          request,
          apimBaseUrl
        );
      } catch (err) {
        logger.error(`Failed to add API keys to application ${applicationId}:`, err);
        throw err;
      }
    },
    [applicationId, organizationId, apimBaseUrl]
  );

  const removeAPIKey = useCallback(
    async (
      mappedKeyId: string,
      options: RemoveApplicationAPIKeyOptions
    ): Promise<void> => {
      if (!organizationId) throw new Error('Organization ID is missing');
      try {
        await applicationApis.removeApplicationAPIKey(
          applicationId,
          mappedKeyId,
          apimBaseUrl,
          options
        );
      } catch (err) {
        logger.error(
          `Failed to remove API key ${mappedKeyId} from application ${applicationId}:`,
          err
        );
        throw err;
      }
    },
    [applicationId, organizationId, apimBaseUrl]
  );

  const value = useMemo(
    () => ({
      associations: associationsResponse.list,
      associationsResponse,
      isLoading,
      error,
      refreshAssociations: fetchAssociations,
      addAssociations,
      removeAssociation,
      listAssociationAPIKeys,
      addAPIKeys,
      removeAPIKey,
    }),
    [
      associationsResponse,
      isLoading,
      error,
      fetchAssociations,
      addAssociations,
      removeAssociation,
      listAssociationAPIKeys,
      addAPIKeys,
      removeAPIKey,
    ]
  );

  return (
    <ApplicationAssociationsContext.Provider value={value}>
      {children}
    </ApplicationAssociationsContext.Provider>
  );
}

export function useApplicationAssociations(): ApplicationAssociationsContextValue {
  return useContext(ApplicationAssociationsContext);
}
