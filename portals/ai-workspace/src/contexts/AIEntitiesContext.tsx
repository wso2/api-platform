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
 *   'agent-proxy'   → AgentProxy
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
  AgentProxy,
  Application,
} from '../utils/types';
import * as llmProviderApis from '../apis/llmProviderApis';
import * as proxyApis from '../apis/proxyApis';
import { mcpProxiesApis } from '../apis/MCP/mcpProxiesApis';
import { getAgentProxy } from '../apis/agent/agentProxiesApis';
import { applicationApis } from '../apis/applicationApis';
import { useAppShell } from './AppShellContext';
import { PLATFORM_API_BASE_URL } from '../paths';
import { logger } from '../utils/logger';

// ============================================================================
// Public types
// ============================================================================

export type AIEntityType =
  | 'llm-provider'
  | 'llm-proxy'
  | 'mcp-server'
  | 'agent-proxy'
  | 'application';

/** Discriminated union — narrow on `entityType` to access the typed `data`. */
export type AIEntityResult =
  | { entityType: 'llm-provider'; data: LLMProvider }
  | { entityType: 'llm-proxy'; data: Proxy }
  | { entityType: 'mcp-server'; data: MCPServer }
  | { entityType: 'agent-proxy'; data: AgentProxy }
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
  entity: LLMProvider | Proxy | MCPServer | AgentProxy | Application | null;
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
): Promise<LLMProvider | Proxy | MCPServer | AgentProxy | Application> {
  switch (type) {
    case 'llm-provider':
      return llmProviderApis.getLLMProvider(id, organizationId, apimBaseUrl);
    case 'llm-proxy':
      return proxyApis.getProxy(id, organizationId, publisherBaseUrl);
    case 'mcp-server':
      return mcpProxiesApis.getMCPServer(id, publisherBaseUrl);
    case 'agent-proxy':
      return getAgentProxy(id, publisherBaseUrl);
    case 'application':
      return applicationApis.getApplication(id, publisherBaseUrl);
    default: {
      const _exhaustive: never = type;
      throw new Error(`Unknown entity type: ${_exhaustive}`);
    }
  }
}

// ============================================================================
// Provider component
// ============================================================================

type PolicyEntityUnion = LLMProvider | Proxy | MCPServer | AgentProxy | Application;

interface AIEntityProviderProps {
  /** The kind of entity to load. */
  type: AIEntityType;
  /** The entity's ID (from route params or props). */
  id: string;
  /**
   * Skip this provider's own fetch and mirror an entity a parent
   * context/page has already loaded for the same id (e.g.
   * `LLMProviderProvider`/`useLLMProvider()`, `ProxyProvider`,
   * `AgentProxyProvider`, or a page's own local fetch). Pass the parent's
   * current value — `null` while it's still loading is fine, the mirrored
   * `entity`/`isLoading` here track it as it resolves. Omit this prop
   * entirely (not `undefined`) to have this provider fetch independently.
   */
  initialEntity?: PolicyEntityUnion | null;
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
 *
 * // Mirroring an entity a sibling context already fetched, instead of a
 * // second independent round-trip for the same resource:
 * const { proxy } = useProxy();
 * <AIEntityProvider type="llm-proxy" id={proxyId} initialEntity={proxy}>
 *   ...
 * </AIEntityProvider>
 */
export function AIEntityProvider({ type, id, initialEntity, children }: AIEntityProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const publisherBaseUrl = PLATFORM_API_BASE_URL;

  const organizationId = currentOrganization?.uuid ?? '';

  // Presence of the prop (not its value) decides the mode: passing `null`
  // while a parent's own fetch is still in flight still means "mirror it,
  // don't fetch independently" — only omitting the prop means "fetch here".
  const useInitialEntity = initialEntity !== undefined;

  // Independent-fetch mode's own state. Left untouched (and unread) while
  // mirroring — `entity`/`isLoading`/`error` below are derived straight from
  // `initialEntity` in that mode instead, on purpose: routing a mirrored prop
  // through useState+useEffect would echo it one render late (the effect
  // fires after the commit that already painted the stale value), which is
  // exactly the kind of gap a deploy-blocking compatibility check can't
  // afford. Computing it inline during render has no such lag.
  const [fetchedEntity, setFetchedEntity] = useState<PolicyEntityUnion | null>(null);
  const [isFetchLoading, setIsFetchLoading] = useState(!useInitialEntity);
  const [fetchError, setFetchError] = useState<Error | null>(null);

  const fetchEntity = useCallback(async () => {
    if (!id || !organizationId) {
      setFetchedEntity(null);
      setIsFetchLoading(false);
      return;
    }

    try {
      setIsFetchLoading(true);
      setFetchError(null);
      const result = await fetchEntityData(type, id, organizationId, apimBaseUrl, publisherBaseUrl);
      setFetchedEntity(result);
    } catch (err) {
      logger.error(`[AIEntity] Failed to fetch ${type} with id ${id}:`, err);
      setFetchError(err instanceof Error ? err : new Error(`Failed to fetch ${type}`));
      setFetchedEntity(null);
    } finally {
      setIsFetchLoading(false);
    }
  }, [type, id, organizationId, apimBaseUrl, publisherBaseUrl]);

  useEffect(() => {
    if (useInitialEntity) return; // mirrored inline during render — nothing to fetch here
    fetchEntity();
  }, [useInitialEntity, fetchEntity]);

  const refetch = useCallback(async () => {
    if (useInitialEntity) {
      // There's no independent fetch to re-run here — the parent
      // context/page owns refetching its own copy. Logged rather than a
      // silent no-op so a future caller wiring this up on a mirroring page
      // gets an immediate signal instead of quietly-stale data.
      logger.warn(
        `[AIEntity] refetch() called on a provider mirroring an externally-loaded ${type} — this is a no-op; refetch via the entity's own context/page instead.`
      );
      return;
    }
    await fetchEntity();
  }, [useInitialEntity, type, fetchEntity]);

  const entity = useInitialEntity ? initialEntity : fetchedEntity;
  const isLoading = useInitialEntity ? initialEntity === null : isFetchLoading;
  const error = useInitialEntity ? null : fetchError;

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
