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
import type { ProviderTemplate, UpdateProviderTemplateRequest } from '../../../utils/types';
import * as providerTemplateApis from '../../../apis/providerTemplateApis';
import { useAppShell } from '../../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../../config.env';
import { logger } from '../../../utils/logger';

// ============================================================================
// Single Provider Template Context - For managing a single template by ID
// ============================================================================

type ProviderTemplateContextValue = {
  template: ProviderTemplate | null;
  isLoading: boolean;
  error: Error | null;
  updateTemplate: (updates: UpdateProviderTemplateRequest) => Promise<ProviderTemplate>;
  deleteTemplate: () => Promise<void>;
  refetch: () => Promise<void>;
};

const ProviderTemplateContext = createContext<ProviderTemplateContextValue>({
  template: null,
  isLoading: false,
  error: null,
  updateTemplate: async () => {
    throw new Error('ProviderTemplateContext not initialized');
  },
  deleteTemplate: async () => {
    throw new Error('ProviderTemplateContext not initialized');
  },
  refetch: async () => {
    throw new Error('ProviderTemplateContext not initialized');
  },
});

interface ProviderTemplateProviderProps {
  children: React.ReactNode;
  templateId: string;
}

export function ProviderTemplateProvider({ children, templateId }: ProviderTemplateProviderProps) {
  const { currentOrganization } = useAppShell();
  const [template, setTemplate] = useState<ProviderTemplate | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch single template
  const fetchTemplate = useCallback(async () => {
    if (!templateId || !organizationId) {
      setTemplate(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const fetchedTemplate = await providerTemplateApis.getProviderTemplate(templateId, organizationId, PLATFORM_API_BASE_URL);
      setTemplate(fetchedTemplate);
    } catch (err) {
      logger.error(`Failed to fetch provider template ${templateId}:`, err);
      setError(err instanceof Error ? err : new Error('Failed to fetch template'));
      setTemplate(null);
    } finally {
      setIsLoading(false);
    }
  }, [templateId, organizationId, PLATFORM_API_BASE_URL]);

  useEffect(() => {
    fetchTemplate();
  }, [fetchTemplate]);

  const updateTemplate = useCallback(async (updates: UpdateProviderTemplateRequest): Promise<ProviderTemplate> => {
    if (!templateId || !organizationId) {
      throw new Error('Template ID or Organization ID is missing');
    }
    try {
      const updatedTemplate = await providerTemplateApis.updateProviderTemplate(templateId, updates, organizationId, PLATFORM_API_BASE_URL);
      setTemplate(updatedTemplate);
      return updatedTemplate;
    } catch (err) {
      logger.error('Failed to update provider template:', err);
      throw err;
    }
  }, [templateId, organizationId, PLATFORM_API_BASE_URL]);

  const deleteTemplate = useCallback(async (): Promise<void> => {
    if (!templateId || !organizationId) {
      throw new Error('Template ID or Organization ID is missing');
    }
    try {
      await providerTemplateApis.deleteProviderTemplate(templateId, organizationId, PLATFORM_API_BASE_URL);
      setTemplate(null);
    } catch (err) {
      logger.error('Failed to delete provider template:', err);
      throw err;
    }
  }, [templateId, organizationId, PLATFORM_API_BASE_URL]);

  const refetch = useCallback(async (): Promise<void> => {
    await fetchTemplate();
  }, [fetchTemplate]);

  const value = useMemo(
    () => ({
      template,
      isLoading,
      error,
      updateTemplate,
      deleteTemplate,
      refetch,
    }),
    [template, isLoading, error, updateTemplate, deleteTemplate, refetch]
  );

  return (
    <ProviderTemplateContext.Provider value={value}>
      {children}
    </ProviderTemplateContext.Provider>
  );
}

export function useProviderTemplate(): ProviderTemplateContextValue {
  return useContext(ProviderTemplateContext);
}
