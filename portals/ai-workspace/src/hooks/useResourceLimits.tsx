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

/**
 * One component's entitlement, as a supplier reports it.
 *
 * `max` of -1 means unlimited; `used` of -1 means the count could not be read. The
 * verdict is derived from the pair rather than supplied, so there is one rule in one
 * place — see `isAtLimit`.
 */
export interface ResourceLimit {
  max: number;
  used: number;
}

/** Shown when a supplier reports read-only but gives no reason of its own. */
const DEFAULT_READ_ONLY_MESSAGE =
  'Your organization is read-only, so nothing can be created or changed.';

/** No ceiling. */
const UNLIMITED = -1;
/** The supplier could not read the count. */
const COUNT_UNKNOWN = -1;

/**
 * Whether this component has reached its ceiling.
 *
 * An unlimited ceiling and an unreadable count both answer "no": a wrong "no" costs a
 * failed submit, while a wrong "yes" would lock a user out of their own product. The
 * control plane applies the same rule when it refuses a create, so this is not the only
 * thing standing between a user and an over-limit component.
 */
function isAtLimit(limit: ResourceLimit | undefined): boolean {
  if (!limit) return false;
  if (limit.max === UNLIMITED || limit.used === COUNT_UNKNOWN) return false;
  return limit.used >= limit.max;
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
   * True when the organization may not create or update ANYTHING — a different
   * question from whether it has room for one more of something. In this product
   * it means the free trial has ended and no paid plan replaced it; the control
   * plane refuses every create and update while it holds, so this is a mirror of
   * a decision made there, not one made here.
   *
   * `canCreate` already answers `false` for every component while this is true,
   * so a page that gates creation on `canCreate` needs no change. Read this one
   * directly to gate an EDIT or an update, which no count would cover.
   */
  readOnly: boolean;
  /** Why the workspace is read-only; '' when it is not. */
  readOnlyMessage: string;
  /**
   * Supply the read-only verdict. Called by the same extension that supplies the
   * limits — nothing in the portal itself calls it.
   */
  setReadOnly: (readOnly: boolean, reason?: string) => void;
  /**
   * Bumped every time something asks for the limits to be re-read. The supplier
   * watches it; nothing else should.
   */
  refreshSignal: number;
  /**
   * Ask the supplier to re-read the limits. Call it after a create or a delete:
   * the counts behind `canCreate` were read when the supplier last ran, so
   * without this a user who has just created their last allowed component keeps
   * seeing an enabled create button until they navigate.
   *
   * A no-op when no supplier is mounted.
   */
  refresh: () => void;
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
  readOnly: false,
  readOnlyMessage: '',
  setReadOnly: NOOP_SET,
  refreshSignal: 0,
  refresh: NOOP_SET,
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
  const [refreshSignal, setRefreshSignal] = useState(0);
  const [readOnlyState, setReadOnlyState] = useState<{
    readOnly: boolean;
    message: string;
  }>({ readOnly: false, message: '' });

  const setResourceLimits = useCallback(
    (next: ResourceLimitSet | null) => setLimits(next),
    []
  );

  const refresh = useCallback(() => setRefreshSignal((n) => n + 1), []);

  const setReadOnly = useCallback(
    (readOnly: boolean, reason?: string) =>
      setReadOnlyState({
        readOnly,
        message: readOnly ? reason ?? DEFAULT_READ_ONLY_MESSAGE : '',
      }),
    []
  );

  const value = useMemo<ResourceLimitsContextType>(() => {
    const limitOf = (component: LimitedComponent) => limits?.[component];

    return {
      readOnly: readOnlyState.readOnly,
      readOnlyMessage: readOnlyState.message,
      setReadOnly,
      refreshSignal,
      refresh,
      // Read-only outranks every count: an organization that may not write at all
      // cannot create a component however much room its plan leaves. Answering it
      // here is what makes every page that already gates on `canCreate` honour
      // read-only without a change of its own.
      canCreate: (component) =>
        !readOnlyState.readOnly && !isAtLimit(limitOf(component)),
      limitMessage: (component) => {
        if (readOnlyState.readOnly) return readOnlyState.message;
        const limit = limitOf(component);
        if (!isAtLimit(limit) || !limit) return '';
        return (
          `You cannot create more ${COMPONENT_LABELS[component]} because your ` +
          `organization has reached the maximum of ${limit.max}.`
        );
      },
      setResourceLimits,
    };
  }, [
    limits,
    readOnlyState,
    refresh,
    refreshSignal,
    setReadOnly,
    setResourceLimits,
  ]);

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
