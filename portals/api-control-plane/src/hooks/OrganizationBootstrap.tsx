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

import { useEffect, useRef } from 'react';

import { useBootstrapOrganization, useOrganization } from '../api/resources/organizations';
import { ErrorCode, isErrorCode } from '../api/core/errors';
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
 */

/**
 * How many times registration is attempted for one organization before the console
 * stops. A conflict spends them all, because that answer is final.
 */
const MAX_REGISTRATION_ATTEMPTS = 3;

export function OrganizationBootstrap() {
  const { isAuthenticated, user } = useAuth();
  const handle = user?.org?.handle;
  const displayName = user?.org?.name;

  // Only a real absence justifies registering. `isPending` covers the disabled case
  // too, so nothing is read until the lookup has actually answered.
  const existing = useOrganization(isAuthenticated ? handle : undefined);
  const bootstrap = useBootstrapOrganization();
  // Keyed on the handle rather than held as a plain flag, so a session that moves to
  // another organization can still register that one. `tries` is what stops a platform
  // that is failing from turning into an unbounded request loop: the effect re-runs
  // whenever the mutation settles, and the lookup still answers not-found.
  const attempt = useRef<{ handle: string; tries: number; inFlight: boolean } | null>(null);
  const refetchExisting = existing.refetch;

  useEffect(() => {
    if (!isAuthenticated || !handle) return;
    if (existing.isPending || existing.data) return;
    // Not-found is the only answer that means "register it". Anything else — an
    // expired session, a denial, the platform being unreachable — says nothing about
    // whether the organization exists, and registering on those would turn a
    // transient failure into a write attempt. The Platform API answers a missing
    // organization with its domain code; the generic one is kept for older servers.
    if (!isErrorCode(existing.error, ErrorCode.ORGANIZATION_NOT_FOUND, ErrorCode.NOT_FOUND)) return;

    const record =
      attempt.current && attempt.current.handle === handle
        ? attempt.current
        : { handle, tries: 0, inFlight: false };
    attempt.current = record;
    if (record.inFlight || record.tries >= MAX_REGISTRATION_ATTEMPTS) return;

    record.inFlight = true;
    record.tries += 1;
    bootstrap.mutate(
      {
        id: handle,
        displayName: displayName || handle,
        region: runtimeConfig.defaultOrgRegion,
      },
      {
        onSuccess: () => {
          record.inFlight = false;
        },
        onError: (error) => {
          record.inFlight = false;
          // A conflict is the other route having won, which is success. Re-read so the
          // rest of the console sees the organization, and spend the remaining attempts
          // so this does not register again.
          if (isErrorCode(error, ErrorCode.ORGANIZATION_EXISTS, ErrorCode.CONFLICT)) {
            record.tries = MAX_REGISTRATION_ATTEMPTS;
            void refetchExisting();
            return;
          }
          // Anything else may be transient, so another attempt is allowed until the
          // ceiling is reached.
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

  return null;
}
