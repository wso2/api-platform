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

import { BILLING_PROXY_ENABLED } from '../config.env';
import { useAppAuth } from '../contexts/AppAuthContext';
import { BILLING_API_BASE_URL } from '../paths';

// Product code this workspace activates on first login. Must match the billing
// product whose subscription drives APIP gateway provisioning.
const PRODUCT = 'api-platform';

/** How many times to re-try a failed activation, and how long to wait between tries. */
const MAX_ATTEMPTS = 3;
const RETRY_DELAY_MS = 5000;

/**
 * Performs billing first-login activation once the user is authenticated.
 *
 * GET <billing>/organization?product=api-platform both reads the organization's
 * subscription and, as a server-side side effect, activates it if it is currently
 * inactive. That activation is what emits subscription.activated, which is in turn
 * what provisions this organization's gateway — so without this call an organization
 * whose users only ever open the AI Workspace has no gateway to deploy to, and the
 * absence surfaces much later as a payment-required error on the first environment or
 * gateway operation.
 *
 * The same call the console makes, for the same reason: whichever portal a user
 * reaches first is the one that has to activate. Activation is idempotent — it acts
 * only on the inactive transition — so both firing is harmless.
 *
 * Routed through the BFF's same-origin proxy — this component never sees a token or
 * the real billing service URL.
 *
 * Renders nothing. No-op when the BFF has no billing upstream configured (every
 * standalone deployment today).
 */
export function ProductActivation() {
  const { isAuthenticated } = useAppAuth();
  const done = useRef(false);
  // Drives the retry: the effect re-runs when this changes, which a ref alone could
  // not do. `fetch` resolves for a 500 as readily as for a 200, so the response has
  // to be inspected — a silent failure here costs the organization its gateway.
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!BILLING_PROXY_ENABLED || !isAuthenticated || done.current) {
      return undefined;
    }

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const retryOrGiveUp = () => {
      if (cancelled || attempt + 1 >= MAX_ATTEMPTS) return;
      timer = setTimeout(() => {
        if (!cancelled) setAttempt((previous) => previous + 1);
      }, RETRY_DELAY_MS);
    };

    void fetch(`${BILLING_API_BASE_URL}/organization?product=${PRODUCT}`, {
      credentials: 'include',
    }).then(
      (response) => {
        if (cancelled) return;
        if (response.ok) {
          done.current = true;
          return;
        }
        retryOrGiveUp();
      },
      () => {
        retryOrGiveUp();
      },
    );

    // Best-effort throughout: activation must never block the workspace, so a run of
    // attempts that all fail is left alone rather than surfaced.
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [attempt, isAuthenticated]);

  return null;
}
