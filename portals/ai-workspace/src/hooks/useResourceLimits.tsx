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

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

/** The component types a deployment may cap the creation of. */
export type LimitedComponent =
  | 'llmProviders'
  | 'llmProxies'
  | 'mcpProxies'
  | 'gateways';

/** One component's entitlement. `max` of -1 means unlimited. */
export interface ResourceLimit {
  max: number;
  used: number;
  canCreate: boolean;
}

/** What a supplier hands in. A component it omits is simply not capped. */
export type ResourceLimitSet = Partial<Record<LimitedComponent, ResourceLimit>>;

/** Plural nouns for the message a user reads when a create action is disabled. */
const COMPONENT_LABELS: Record<LimitedComponent, string> = {
  llmProviders: 'LLM providers',
  llmProxies: 'LLM proxies',
  mcpProxies: 'MCP proxies',
  gateways: 'gateways',
};

export interface ResourceLimitsContextType {
  /**
   * Whether another component of this type may be created.
   *
   * Answers `true` unless something has supplied a limit saying otherwise — so
   * no supplier, a supplier that has not answered yet, and a component nobody
   * caps all leave creation enabled. A wrong `true` costs a failed submit; a
   * wrong `false` would lock a user out of their own product.
   */
  canCreate: (component: LimitedComponent) => boolean;
  /** Tooltip explaining why a create action is disabled; '' when it is not. */
  limitMessage: (component: LimitedComponent) => string;
  /**
   * Supply the limits. Called by whatever knows them — in this product that is a
   * cloud plugin, which reaches it through the host Port; nothing in the portal
   * itself calls this, and nothing here knows where the numbers come from.
   * Passing `null` clears them, which restores "everything allowed".
   */
  setResourceLimits: (limits: ResourceLimitSet | null) => void;
}

const NOOP_SET = () => {
  /* no supplier mounted */
};

const NO_LIMITS: ResourceLimitsContextType = {
  canCreate: () => true,
  limitMessage: () => '',
  setResourceLimits: NOOP_SET,
};

const ResourceLimitsContext =
  createContext<ResourceLimitsContextType>(NO_LIMITS);

/**
 * Holds whatever component creation limits apply to the current organization,
 * for the pages that gate a create action on them.
 *
 * Deliberately knows nothing about where limits come from. Entitlements are a
 * property of a particular deployment's commercial model — subscription tiers,
 * billing services, trial state — none of which the product itself has an
 * opinion about, so this owns only the shape and the default. A deployment that
 * caps components supplies them through the host Port's `resourceLimits.set`
 * (see `hostPort.tsx`); a deployment that does not supplies nothing and every
 * create action stays enabled exactly as it was before this existed.
 */
export function ResourceLimitsProvider({ children }: { children: ReactNode }) {
  const [limits, setLimits] = useState<ResourceLimitSet | null>(null);

  const setResourceLimits = useCallback(
    (next: ResourceLimitSet | null) => setLimits(next),
    []
  );

  const value = useMemo<ResourceLimitsContextType>(() => {
    const limitOf = (component: LimitedComponent) => limits?.[component];

    return {
      canCreate: (component) => limitOf(component)?.canCreate ?? true,
      limitMessage: (component) => {
        const limit = limitOf(component);
        if (!limit || limit.canCreate) return '';
        return (
          `You cannot create more ${COMPONENT_LABELS[component]} because your ` +
          `organization has reached the maximum of ${limit.max}.`
        );
      },
      setResourceLimits,
    };
  }, [limits, setResourceLimits]);

  return (
    <ResourceLimitsContext.Provider value={value}>
      {children}
    </ResourceLimitsContext.Provider>
  );
}

/**
 * The current organization's creation limits.
 *
 * Safe to call outside the provider — it falls back to "no limits", so a page
 * rendered off the app shell (or a test) behaves as it did before limits existed
 * rather than throwing.
 */
export function useResourceLimits(): ResourceLimitsContextType {
  return useContext(ResourceLimitsContext);
}
