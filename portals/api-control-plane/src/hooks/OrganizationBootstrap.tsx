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

import { useEffect, useRef, useState } from 'react';

import { useBootstrapOrganization, useOrganization } from '../api/resources/organizations';
import { ErrorCode, isApiError, isErrorCode } from '../api/core/errors';
import { runtimeConfig } from '../config/runtime';
import { useAuth } from '../contexts/auth/AuthProvider';

/**
 * Registers the signed-in user's organization when the platform does not have it yet.
 *
 * An organization reaches the platform by two independent routes, and either may get
 * there first: the provisioning flow creates it when the organization is set up, and a
 * portal creates it when a user arrives before that has happened. Both are idempotent,
 * so neither has to know whether the other ran, and losing the race is success.
 *
 * This is the console's half of that, matching what the AI Workspace already does, so
 * whichever portal a user opens first is the one that registers. Without it a
 * console-first user faces an organization the platform has never heard of, and every
 * call made against it fails.
 *
 * The IDP organization reference is deliberately not sent: the platform derives it
 * server-side from the token's organization claim, which is what makes an organization
 * registered here identical to a provisioned one.
 *
 * Renders nothing, and never blocks the console — the organization may simply be
 * arriving by the other route.
 *
 * A registration that fails is never retried straight away. A deployment may not let
 * portals register organizations at all (the route is not served, or the caller may
 * not use it); then the console stops trying and only looks again, every
 * {@link RECHECK_INTERVAL_MS}, until provisioning has created the organization. Any
 * other failure may be transient, so the console looks again after the same interval
 * and registers then if the organization is still missing.
 */
export function OrganizationBootstrap() {
  const { isAuthenticated, user } = useAuth();
  const handle = user?.org?.handle;
  const displayName = user?.org?.name;

  // Only a real absence justifies registering. `isPending` covers the disabled case
  // too, so nothing is read until the lookup has actually answered.
  const existing = useOrganization(isAuthenticated ? handle : undefined);
  const bootstrap = useBootstrapOrganization();
  // The organization registration was started for: a session that moves into
  // another organization may need that one registered too.
  const started = useRef<string | undefined>(undefined);
  const refetchExisting = existing.refetch;
  // What to do while the organization is still missing after a failed registration:
  // keep looking ('recheck'), or look once and then register again ('retry').
  const [waiting, setWaiting] = useState<{ handle: string; mode: 'recheck' | 'retry' }>();

  useEffect(() => {
    if (!isAuthenticated || !handle || started.current === handle) return;
    if (existing.isPending || existing.data) return;
    // Not-found is the only answer that means "register it". Anything else — an
    // expired session, a denial, the platform being unreachable — says nothing about
    // whether the organization exists, and registering on those would turn a
    // transient failure into a write attempt.
    if (!isErrorCode(existing.error, ErrorCode.NOT_FOUND)) return;

    started.current = handle;
    bootstrap.mutate(
      {
        id: handle,
        displayName: displayName || handle,
        region: runtimeConfig.defaultOrgRegion,
      },
      {
        onError: (error) => {
          // A conflict is the other route having won, which is success. Re-read so
          // the rest of the console sees the organization, and leave `started` set so
          // this does not register again.
          if (isErrorCode(error, ErrorCode.CONFLICT)) {
            void refetchExisting();
            return;
          }
          // `started` stays set either way, so nothing registers again until the
          // wait below says so.
          setWaiting({ handle, mode: isRegistrationRefused(error) ? 'recheck' : 'retry' });
        },
      },
    );
  }, [
    bootstrap,
    displayName,
    existing.data,
    existing.error,
    existing.isPending,
    handle,
    isAuthenticated,
    refetchExisting,
  ]);

  const waitingFor = waiting && waiting.handle === handle && !existing.data ? waiting.mode : undefined;
  useEffect(() => {
    if (!waitingFor) return;
    if (waitingFor === 'recheck') {
      const timer = setInterval(() => void refetchExisting(), RECHECK_INTERVAL_MS);
      return () => clearInterval(timer);
    }
    const timer = setTimeout(() => {
      started.current = undefined;
      setWaiting(undefined);
      void refetchExisting();
    }, RECHECK_INTERVAL_MS);
    return () => clearTimeout(timer);
  }, [waitingFor, refetchExisting]);

  return null;
}

/** How long the console waits before looking for the organization again. */
export const RECHECK_INTERVAL_MS = 30_000;

/**
 * Whether the platform refused the registration itself, rather than failing it: the
 * route is not served here (404, 405) or the caller may not use it (403). Retrying
 * cannot change either answer. Read from the HTTP status, since a route an
 * intermediary does not serve answers without the platform's error body.
 */
function isRegistrationRefused(error: unknown): boolean {
  return isApiError(error) && error.kind === 'http' && [403, 404, 405].includes(error.status ?? 0);
}
