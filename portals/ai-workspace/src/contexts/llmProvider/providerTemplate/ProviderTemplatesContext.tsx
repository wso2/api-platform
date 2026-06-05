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

import React, { createContext, useContext, useEffect, useMemo, useState, useCallback } from 'react';
import type { 
  ProviderTemplate, 
  CreateProviderTemplateRequest, 
  UpdateProviderTemplateRequest,
  ProviderTemplatesResponse 
} from '../../../utils/types';
import * as providerTemplateApis from '../../../apis/providerTemplateApis';
import { useAppShell } from '../../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../../config.env';
import { logger } from '../../../utils/logger';

// ============================================================================
// Provider Templates List Context - For managing the list of all templates
// ============================================================================

// Default empty response matching API format
const EMPTY_TEMPLATES_RESPONSE: ProviderTemplatesResponse = {
  count: 0,
  list: [],
  pagination: { total: 0, offset: 0, limit: 20 },
};

type ProviderTemplatesContextValue = {
  /** Full API response with count, list, and pagination */
  templatesResponse: ProviderTemplatesResponse;
  isLoading: boolean;
  error: Error | null;
  createTemplate: (template: CreateProviderTemplateRequest) => Promise<ProviderTemplate>;
  updateTemplate: (templateId: string, updates: UpdateProviderTemplateRequest) => Promise<ProviderTemplate>;
  deleteTemplate: (templateId: string) => Promise<void>;
  refreshTemplates: () => Promise<void>;
  getTemplateById: (templateId: string) => ProviderTemplate | undefined;
};

const ProviderTemplatesContext = createContext<ProviderTemplatesContextValue>({
  templatesResponse: EMPTY_TEMPLATES_RESPONSE,
  isLoading: false,
  error: null,
  createTemplate: async () => {
    throw new Error('ProviderTemplatesContext not initialized');
  },
  updateTemplate: async () => {
    throw new Error('ProviderTemplatesContext not initialized');
  },
  deleteTemplate: async () => {
    throw new Error('ProviderTemplatesContext not initialized');
  },
  refreshTemplates: async () => {
    throw new Error('ProviderTemplatesContext not initialized');
  },
  getTemplateById: () => undefined,
});

interface ProviderTemplatesProviderProps {
  children: React.ReactNode;
}

export function ProviderTemplatesProvider({ children }: ProviderTemplatesProviderProps) {
  const { currentOrganization } = useAppShell();
  const [templatesResponse, setTemplatesResponse] = useState<ProviderTemplatesResponse>(EMPTY_TEMPLATES_RESPONSE);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch all templates
  const fetchTemplates = useCallback(async () => {
    if (!organizationId) {
      setTemplatesResponse(EMPTY_TEMPLATES_RESPONSE);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const response = await providerTemplateApis.getProviderTemplates(organizationId, PLATFORM_API_BASE_URL);
      // Store the full API response as-is
      setTemplatesResponse(response as ProviderTemplatesResponse);
    } catch (err) {
      logger.error('Failed to fetch provider templates:', err);
      setError(err instanceof Error ? err : new Error('Failed to fetch templates'));
      // Keep existing response on error
    } finally {
      setIsLoading(false);
    }
  }, [organizationId, PLATFORM_API_BASE_URL]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  const createTemplate = useCallback(async (template: CreateProviderTemplateRequest): Promise<ProviderTemplate> => {
    if (!organizationId) {
      throw new Error('Organization ID is missing');
    }
    try {
      const newTemplate = await providerTemplateApis.createProviderTemplate(template, organizationId, PLATFORM_API_BASE_URL);
      setTemplatesResponse((prev) => ({
        ...prev,
        count: prev.count + 1,
        list: [newTemplate, ...prev.list],
        pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
      }));
      return newTemplate;
    } catch (err) {
      logger.error('Failed to create provider template:', err);
      throw err;
    }
  }, [organizationId, PLATFORM_API_BASE_URL]);

  const updateTemplate = useCallback(async (
    templateId: string,
    updates: UpdateProviderTemplateRequest
  ): Promise<ProviderTemplate> => {
    if (!organizationId) {
      throw new Error('Organization ID is missing');
    }
    try {
      const updatedTemplate = await providerTemplateApis.updateProviderTemplate(templateId, updates, organizationId, PLATFORM_API_BASE_URL);
      setTemplatesResponse((prev) => ({
        ...prev,
        list: prev.list.map((template) =>
          template.id === templateId ? updatedTemplate : template
        ),
      }));
      return updatedTemplate;
    } catch (err) {
      logger.error('Failed to update provider template:', err);
      throw err;
    }
  }, [organizationId, PLATFORM_API_BASE_URL]);

  const deleteTemplate = useCallback(async (templateId: string): Promise<void> => {
    if (!organizationId) {
      throw new Error('Organization ID is missing');
    }
    try {
      await providerTemplateApis.deleteProviderTemplate(templateId, organizationId, PLATFORM_API_BASE_URL);
      setTemplatesResponse((prev) => ({
        ...prev,
        count: Math.max(0, prev.count - 1),
        list: prev.list.filter((template) => template.id !== templateId),
        pagination: { ...prev.pagination, total: Math.max(0, prev.pagination.total - 1) },
      }));
    } catch (err) {
      logger.error('Failed to delete provider template:', err);
      throw err;
    }
  }, [organizationId, PLATFORM_API_BASE_URL]);

  const refreshTemplates = useCallback(async (): Promise<void> => {
    await fetchTemplates();
  }, [fetchTemplates]);

  const getTemplateById = useCallback((templateId: string): ProviderTemplate | undefined => {
    return templatesResponse.list.find((template) => template.id === templateId);
  }, [templatesResponse.list]);

  const value = useMemo(
    () => ({
      templatesResponse,
      isLoading,
      error,
      createTemplate,
      updateTemplate,
      deleteTemplate,
      refreshTemplates,
      getTemplateById,
    }),
    [templatesResponse, isLoading, error, createTemplate, updateTemplate, deleteTemplate, refreshTemplates, getTemplateById]
  );

  return (
    <ProviderTemplatesContext.Provider value={value}>
      {children}
    </ProviderTemplatesContext.Provider>
  );
}

export function useProviderTemplates(): ProviderTemplatesContextValue {
  return useContext(ProviderTemplatesContext);
}
