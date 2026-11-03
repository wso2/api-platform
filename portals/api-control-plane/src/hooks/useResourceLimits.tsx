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
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  readOnlyDefault: {
    id: 'apiControlPlane.hooks.useResourceLimits.readOnlyDefault',
    defaultMessage: 'Your organization is read-only, so nothing can be created or changed.',
    description:
      'Shown when something reports the organization as read-only but gives no reason of its own.',
  },
  limitReached: {
    id: 'apiControlPlane.hooks.useResourceLimits.limitReached',
    defaultMessage:
      'You cannot create more {component} because your organization has reached the maximum of {max}.',
    description:
      'Tooltip on a create action disabled because a limit is reached. {component} is a plural noun such as "APIs"; {max} is the ceiling.',
  },
  componentRestApis: {
    id: 'apiControlPlane.hooks.useResourceLimits.componentRestApis',
    defaultMessage: 'APIs',
    description: 'Plural noun for APIs, used inside the limit-reached sentence.',
  },
  componentGateways: {
    id: 'apiControlPlane.hooks.useResourceLimits.componentGateways',
    defaultMessage: 'gateways',
    description: 'Plural noun for gateways, used inside the limit-reached sentence.',
  },
  componentPortals: {
    id: 'apiControlPlane.hooks.useResourceLimits.componentPortals',
    defaultMessage: 'portals',
    description: 'Plural noun for developer portals, used inside the limit-reached sentence.',
  },
});

/**
 * The component types a deployment may cap the creation of.
 *
 * Product nouns, deliberately — never the names a billing system gives its
 * meters. Which meters exist, how they combine and what a plan includes are
 * properties of one deployment's commercial model; this portal also ships
 * standalone and on-prem, where none of that exists. A supplier maps whatever
 * it knows onto these names before handing them over.
 */
export type LimitedComponent = 'restApis' | 'gateways' | 'portals';

/**
 * One component's entitlement, as a supplier reports it.
 *
 * `max` of -1 means unlimited; `used` of -1 means the count could not be read.
 * The verdict is derived from the pair rather than supplied, so there is one
 * rule in one place — see `isAtLimit`.
 */
export interface ResourceLimit {
  max: number;
  used: number;
  /**
   * What to tell a user whose create action this limit has disabled.
   *
   * Supplied, because the portal cannot write it: naming the cap that actually
   * binds ("the maximum of 10 Managed Interfaces") means naming a commercial
   * concept this portal has no vocabulary for, and which differs between
   * deployments. Absent, `limitMessage` falls back to a generic sentence built
   * from `max` alone.
   */
  message?: string;
}

/**
 * A link offered alongside the read-only message — "Subscribe to a plan" and
 * wherever that lives. Supplied whole by whatever reports the verdict, so no
 * part of the destination (a billing console, a plan, an organization id) is
 * assembled, configured or known here.
 */
export type ReadOnlyAction = {
  href: string;
  label: string;
};

/** No ceiling. */
const UNLIMITED = -1;
/** The supplier could not read the count. */
const COUNT_UNKNOWN = -1;

/**
 * Whether this component has reached its ceiling.
 *
 * An unlimited ceiling and an unreadable count both answer "no": a wrong "no"
 * costs a failed submit, while a wrong "yes" would lock a user out of their own
 * product. The control plane applies the same rule when it refuses a create, so
 * this is not the only thing standing between a user and an over-limit
 * component.
 */
function isAtLimit(limit: ResourceLimit | undefined): boolean {
  if (!limit) return false;
  if (limit.max === UNLIMITED || limit.used === COUNT_UNKNOWN) return false;
  return limit.used >= limit.max;
}

/** What a supplier hands in. A component it omits is simply not capped. */
export type ResourceLimitSet = Partial<Record<LimitedComponent, ResourceLimit>>;

/** Plural nouns for the message a user reads when a create action is disabled. */
const COMPONENT_LABELS: Record<LimitedComponent, (typeof messages)[keyof typeof messages]> = {
  restApis: messages.componentRestApis,
  gateways: messages.componentGateways,
  portals: messages.componentPortals,
};

export interface ResourceLimitsContextType {
  /**
   * True when the organization may not create or update ANYTHING — a different
   * question from whether it has room for one more of something. The control
   * plane refuses every create and update while it holds, so this is a mirror
   * of a decision made there, not one made here.
   *
   * `canCreate` already answers `false` for every component while this is true,
   * so a page that gates creation on `canCreate` needs no change. Read this one
   * directly to gate an EDIT or an update, which no count would cover.
   */
  readOnly: boolean;
  /** Why the organization is read-only; '' when it is not. */
  readOnlyMessage: string;
  /**
   * The banner's content, when the supplier wants more than a sentence — a
   * phrase in bold, a link to wherever write access is restored. `null` when it
   * gave none, and the plain `readOnlyMessage` is shown instead.
   *
   * A node rather than a string because the whole point is markup the supplier
   * controls: the portal has no opinion about plans, prices or consoles, and
   * assembles no part of what this says or where it points.
   */
  readOnlyBanner: ReactNode;
  /**
   * Supply the read-only verdict. Called by the same extension that supplies
   * the limits — nothing in the portal itself calls it.
   */
  setReadOnly: (readOnly: boolean, reason?: string, banner?: ReactNode) => void;
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
   * Deliberately says only "something changed, re-read" — the caller does not
   * know, and must not learn, whether a limit exists or why a create failed.
   * That keeps every call site identical in a standalone build, where it is a
   * no-op because no supplier is mounted.
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
   * Supply the limits. Called by whatever knows them — in this product that is
   * a cloud plugin, which reaches it through the host Port; nothing in the
   * portal itself calls this, and nothing here knows where the numbers come
   * from. Passing `null` clears them, which restores "everything allowed".
   */
  setResourceLimits: (limits: ResourceLimitSet | null) => void;
}

const NOOP_SET = () => {
  /* no supplier mounted */
};

/**
 * What every deployment without a supplier gets, and what `useResourceLimits`
 * falls back to outside the provider: nothing capped, nothing read-only, every
 * setter inert. A standalone, on-prem or OSS build never leaves this state.
 */
const NO_LIMITS: ResourceLimitsContextType = {
  readOnly: false,
  readOnlyMessage: '',
  readOnlyBanner: null,
  setReadOnly: NOOP_SET,
  refreshSignal: 0,
  refresh: NOOP_SET,
  canCreate: () => true,
  limitMessage: () => '',
  setResourceLimits: NOOP_SET,
};

const ResourceLimitsContext = createContext<ResourceLimitsContextType>(NO_LIMITS);

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
  const intl = useIntl();
  const [limits, setLimits] = useState<ResourceLimitSet | null>(null);
  const [refreshSignal, setRefreshSignal] = useState(0);
  const [readOnlyState, setReadOnlyState] = useState<{
    readOnly: boolean;
    message: string;
    banner: ReactNode;
  }>({ readOnly: false, message: '', banner: null });

  const setResourceLimits = useCallback(
    (next: ResourceLimitSet | null) => setLimits(next),
    []
  );

  const refresh = useCallback(() => setRefreshSignal((n) => n + 1), []);

  const setReadOnly = useCallback(
    (readOnly: boolean, reason?: string, banner?: ReactNode) =>
      setReadOnlyState({
        readOnly,
        // A supplier that reports read-only with a blank or whitespace-only
        // reason still owes the user an explanation, so fall back to the
        // default message rather than rendering an empty tooltip.
        message: readOnly
          ? reason?.trim() || intl.formatMessage(messages.readOnlyDefault)
          : '',
        banner: readOnly ? (banner ?? null) : null,
      }),
    [intl]
  );

  const value = useMemo<ResourceLimitsContextType>(() => {
    const limitOf = (component: LimitedComponent) => limits?.[component];

    return {
      readOnly: readOnlyState.readOnly,
      readOnlyMessage: readOnlyState.message,
      readOnlyBanner: readOnlyState.banner,
      setReadOnly,
      refreshSignal,
      refresh,
      // Read-only outranks every count: an organization that may not write at
      // all cannot create a component however much room its plan leaves.
      // Answering it here is what makes every page that already gates on
      // `canCreate` honour read-only without a change of its own.
      canCreate: (component) =>
        !readOnlyState.readOnly && !isAtLimit(limitOf(component)),
      limitMessage: (component) => {
        if (readOnlyState.readOnly) return readOnlyState.message;
        const limit = limitOf(component);
        if (!isAtLimit(limit) || !limit) return '';
        // The supplier's own wording wins when it gave one: only it can name
        // the cap that actually binds, which may not be this component's own.
        if (limit.message?.trim()) return limit.message.trim();
        return intl.formatMessage(messages.limitReached, {
          component: intl.formatMessage(COMPONENT_LABELS[component]),
          max: limit.max,
        });
      },
      setResourceLimits,
    };
  }, [
    intl,
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
 * rendered off the app shell (or a test) behaves as it did before limits
 * existed rather than throwing.
 */
export function useResourceLimits(): ResourceLimitsContextType {
  return useContext(ResourceLimitsContext);
}
