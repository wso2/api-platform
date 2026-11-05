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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { ReactNode } from 'react';

import type { LimitedComponent, ResourceLimitSet } from './entitlementLimits';
import type { BillingOrganization } from './types';

/**
 * The slice of the host's Port this feature needs, hand-mirrored rather than
 * imported so the same component works in either portal (see the cloud
 * extensions convention: a feature receives a value of this shape as a prop,
 * never a host's own context).
 *
 * Billing is read through the host because that read is not a plain GET: it
 * performs first-login subscription activation as a side effect, so the host
 * owns it and every caller shares one request. `organization` resolves null
 * when the deployment has no billing upstream.
 */
export type TrialStatusHostPort = {
  /**
   * The organization's handle: keys the shared entitlement store, and becomes
   * the upgrade link's `orgId`. Optional so the badge still renders its
   * countdown before the organization resolves — it simply shows no usage
   * panel until it does.
   */
  orgHandle?: string;
  billing: {
    organization: () => Promise<BillingOrganization | null>;
  };
  /**
   * The same counter the gate watches. The badge's usage panel reads the same
   * summary, so it has to re-read on the same signal — otherwise it keeps
   * showing the counts from its first render for the life of the session,
   * while the gate's own view of the limits moves on without it.
   *
   * Optional so the shared AI Workspace host, which supplies no limits port,
   * still satisfies this type.
   */
  resourceLimits?: Pick<ResourceLimitsPort, 'refreshSignal'>;
};

/**
 * The limits half of the host Port — see `api-control-plane/src/hostPort.tsx`'s
 * `ResourceLimitsPort`. The host holds the limits and knows nothing about where
 * they come from; this plugin is what supplies them, via `set`/`setReadOnly`.
 *
 * Product nouns only. Billing metric codes stop at `entitlementLimits.ts`.
 */
export type ResourceLimitsPort = {
  // Narrow, not `string`: the host's own signature takes its `LimitedComponent`
  // union, and a function accepting `string` is not assignable to one accepting
  // the union (parameters are contravariant). Mirror the union exactly.
  canCreate: (component: LimitedComponent) => boolean;
  limitMessage: (component: LimitedComponent) => string;
  set: (limits: ResourceLimitSet | null) => void;
  readOnly: boolean;
  readOnlyMessage: string;
  readOnlyBanner: ReactNode;
  setReadOnly: (readOnly: boolean, reason?: string, banner?: ReactNode) => void;
  /** Ask for a re-read; the gate watches `refreshSignal` for it. */
  refresh: () => void;
  refreshSignal: number;
};

/** What `EntitlementsGate` needs from the host. */
export type EntitlementsHostPort = {
  /**
   * The organization's handle. Used to key the shared store and to build the
   * upgrade link — see `runtimeConfig.upgradeUrl` on why a handle rather than a
   * separate UUID.
   */
  orgHandle: string;
  /** Changes on navigation; a cheap signal that counts may be stale. */
  projectHandle?: string;
  resourceLimits: ResourceLimitsPort;
};
