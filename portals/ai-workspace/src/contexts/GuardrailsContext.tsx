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
import type { PolicyHubPolicy, GuardrailsResponse } from '../utils/types';
import { getGuardrails, getPolicyDefinition } from '../apis/policyHubApis';
import { logger } from '../utils/logger';

// ============================================================================
// Guardrails Context - For managing guardrail policies from Policy Hub
// ============================================================================

// Default empty response
const EMPTY_GUARDRAILS_RESPONSE: GuardrailsResponse = {
  data: [],
  count: 0,
};

type GuardrailsContextValue = {
  /** Full response with policies */
  guardrailsResponse: GuardrailsResponse;
  /** Convenience accessor for just the policies list */
  guardrails: PolicyHubPolicy[];
  isLoading: boolean;
  error: Error | null;
  refreshGuardrails: () => Promise<void>;
  getGuardrailById: (id: string) => PolicyHubPolicy | undefined;
  getGuardrailByName: (name: string) => PolicyHubPolicy | undefined;
  getGuardrailDefinition: (name: string, version: string) => Promise<string>;
};

const GuardrailsContext = createContext<GuardrailsContextValue>({
  guardrailsResponse: EMPTY_GUARDRAILS_RESPONSE,
  guardrails: [],
  isLoading: false,
  error: null,
  refreshGuardrails: async () => {
    throw new Error('GuardrailsContext not initialized');
  },
  getGuardrailById: () => undefined,
  getGuardrailByName: () => undefined,
  getGuardrailDefinition: async () => {
    throw new Error('GuardrailsContext not initialized');
  },
});

interface GuardrailsProviderProps {
  children: React.ReactNode;
  /** Categories to fetch (default: 'Guardrails,AI') */
  categories?: string;
}

export function GuardrailsProvider({ children, categories = 'Guardrails,AI' }: GuardrailsProviderProps) {
  const [guardrailsResponse, setGuardrailsResponse] = useState<GuardrailsResponse>(EMPTY_GUARDRAILS_RESPONSE);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  // Fetch guardrails from Policy Hub API
  const fetchGuardrails = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      logger.info('Fetching guardrails with categories:', categories);
      const response = await getGuardrails(categories);
      logger.info('Guardrails response:', response);
      setGuardrailsResponse(response);
    } catch (err) {
      logger.error('Failed to fetch guardrails:', err);
      setError(err instanceof Error ? err : new Error('Failed to fetch guardrails'));
      // Keep existing response on error
    } finally {
      setIsLoading(false);
    }
  }, [categories]);

  useEffect(() => {
    fetchGuardrails();
  }, [fetchGuardrails]);

  const refreshGuardrails = useCallback(async (): Promise<void> => {
    await fetchGuardrails();
  }, [fetchGuardrails]);

  const getGuardrailById = useCallback((id: string): PolicyHubPolicy | undefined => {
    return guardrailsResponse.data.find((policy) => policy.name === id);
  }, [guardrailsResponse.data]);

  const getGuardrailByName = useCallback((name: string): PolicyHubPolicy | undefined => {
    return guardrailsResponse.data.find((policy) => policy.name === name);
  }, [guardrailsResponse.data]);

  const getGuardrailDefinition = useCallback(async (name: string, version: string): Promise<string> => {
    return getPolicyDefinition(name, version);
  }, []);

  const value = useMemo(
    () => ({
      guardrailsResponse,
      guardrails: guardrailsResponse.data,
      isLoading,
      error,
      refreshGuardrails,
      getGuardrailById,
      getGuardrailByName,
      getGuardrailDefinition,
    }),
    [
      guardrailsResponse,
      isLoading,
      error,
      refreshGuardrails,
      getGuardrailById,
      getGuardrailByName,
      getGuardrailDefinition,
    ]
  );

  return (
    <GuardrailsContext.Provider value={value}>
      {children}
    </GuardrailsContext.Provider>
  );
}

export function useGuardrails(): GuardrailsContextValue {
  return useContext(GuardrailsContext);
}
