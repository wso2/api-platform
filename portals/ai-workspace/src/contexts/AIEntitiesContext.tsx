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

/**
 * AIEntityContext — unified single-entity context.
 *
 * Mount <AIEntityProvider type="llm-provider" id={providerId}> (or any other
 * entity type) at the parent-layout level. Any child component can then call
 * useAIEntity() to access the currently loaded entity without prop-drilling.
 *
 * Supported entity types:
 *   'llm-provider'  → LLMProvider
 *   'llm-proxy'     → Proxy
 *   'mcp-server'    → MCPServer
 *   'application'   → Application  (Gen AI application)
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
  LLMProvider,
  Proxy,
  MCPServer,
  Application,
} from '../utils/types';
import * as llmProviderApis from '../apis/llmProviderApis';
import * as proxyApis from '../apis/proxyApis';
import { mcpProxiesApis } from '../apis/MCP/mcpProxiesApis';
import { applicationApis } from '../apis/applicationApis';
import { useAppShell } from './AppShellContext';
import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';

// ============================================================================
// Public types
// ============================================================================

export type AIEntityType =
  | 'llm-provider'
  | 'llm-proxy'
  | 'mcp-server'
  | 'application';

/** Discriminated union — narrow on `entityType` to access the typed `data`. */
export type AIEntityResult =
  | { entityType: 'llm-provider'; data: LLMProvider }
  | { entityType: 'llm-proxy'; data: Proxy }
  | { entityType: 'mcp-server'; data: MCPServer }
  | { entityType: 'application'; data: Application };

// ============================================================================
// Context value
// ============================================================================

type AIEntityContextValue = {
  /** Which entity type this context is loaded for. */
  entityType: AIEntityType | null;
  /** The entity ID that was requested. */
  entityId: string | null;
  /**
   * The loaded entity. Narrow on `entityType` to get the specific typed data:
   *
   * @example
   * const { entityType, entity } = useAIEntity();
   * if (entityType === 'llm-provider') {
   *   const provider = entity as LLMProvider;
   * }
   */
  entity: LLMProvider | Proxy | MCPServer | Application | null;
  isLoading: boolean;
  error: Error | null;
  /** Re-fetch the entity from the API. */
  refetch: () => Promise<void>;
};

// ============================================================================
// Context creation with safe defaults
// ============================================================================

const AIEntityContext = createContext<AIEntityContextValue>({
  entityType: null,
  entityId: null,
  entity: null,
  isLoading: false,
  error: null,
  refetch: async () => {
    throw new Error('AIEntityContext not initialized');
  },
});

// ============================================================================
// Internal fetch helper
// ============================================================================

async function fetchEntityData(
  type: AIEntityType,
  id: string,
  organizationId: string,
  apimBaseUrl: string,
  publisherBaseUrl: string
): Promise<LLMProvider | Proxy | MCPServer | Application> {
  switch (type) {
    case 'llm-provider':
      return llmProviderApis.getLLMProvider(id, organizationId, apimBaseUrl);
    case 'llm-proxy':
      return proxyApis.getProxy(id, organizationId, publisherBaseUrl);
    case 'mcp-server':
      return mcpProxiesApis.getMCPServer(id, organizationId, publisherBaseUrl);
    case 'application':
      return applicationApis.getApplication(id, organizationId, publisherBaseUrl);
    default: {
      const _exhaustive: never = type;
      throw new Error(`Unknown entity type: ${_exhaustive}`);
    }
  }
}

// ============================================================================
// Provider component
// ============================================================================

interface AIEntityProviderProps {
  /** The kind of entity to load. */
  type: AIEntityType;
  /** The entity's ID (from route params or props). */
  id: string;
  children: React.ReactNode;
}

/**
 * Mount this provider at the parent layout/overview level for any entity page.
 * Children call `useAIEntity()` to consume the loaded entity.
 *
 * @example
 * // In ServiceProviderOverview wrapper:
 * <AIEntityProvider type="llm-provider" id={providerId}>
 *   <Outlet />
 * </AIEntityProvider>
 *
 * // In LLMProxyOverview wrapper:
 * <AIEntityProvider type="llm-proxy" id={proxyId}>
 *   <Outlet />
 * </AIEntityProvider>
 */
export function AIEntityProvider({ type, id, children }: AIEntityProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const publisherBaseUrl = PLATFORM_API_BASE_URL;

  const organizationId = currentOrganization?.uuid ?? '';

  const [entity, setEntity] = useState<LLMProvider | Proxy | MCPServer | Application | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchEntity = useCallback(async () => {
    if (!id || !organizationId) {
      setEntity(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const result = await fetchEntityData(type, id, organizationId, apimBaseUrl, publisherBaseUrl);
      setEntity(result);
    } catch (err) {
      logger.error(`[AIEntity] Failed to fetch ${type} with id ${id}:`, err);
      setError(err instanceof Error ? err : new Error(`Failed to fetch ${type}`));
      setEntity(null);
    } finally {
      setIsLoading(false);
    }
  }, [type, id, organizationId, apimBaseUrl, publisherBaseUrl]);

  useEffect(() => {
    fetchEntity();
  }, [fetchEntity]);

  const refetch = useCallback(async () => {
    await fetchEntity();
  }, [fetchEntity]);

  const value = useMemo<AIEntityContextValue>(
    () => ({
      entityType: type,
      entityId: id,
      entity,
      isLoading,
      error,
      refetch,
    }),
    [type, id, entity, isLoading, error, refetch]
  );

  return (
    <AIEntityContext.Provider value={value}>
      {children}
    </AIEntityContext.Provider>
  );
}

// ============================================================================
// Consumer hook
// ============================================================================

/**
 * Access the currently active AI entity (LLM Provider, LLM Proxy, MCP Server,
 * or Gen AI Application) from any child of `AIEntityProvider`.
 *
 * Narrow on `entityType` to get the correct TypeScript type:
 *
 * @example
 * const { entityType, entity, isLoading, error } = useAIEntity();
 *
 * if (entityType === 'llm-provider') {
 *   const provider = entity as LLMProvider; // fully typed
 * }
 * if (entityType === 'llm-proxy') {
 *   const proxy = entity as Proxy;
 * }
 */
export function useAIEntity(): AIEntityContextValue {
  return useContext(AIEntityContext);
}
