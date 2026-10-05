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

import { getBillingOrganization } from '../billing/organization';
import { useAuth } from '../contexts/auth/AuthProvider';

/**
 * Performs billing first-login activation once the user is authenticated.
 *
 * Reading the organization's billing record both reads its subscription and, as
 * a server-side side effect, activates the api-platform subscription if it is
 * currently inactive (first-login activation). That activation emits
 * subscription.activated, which triggers APIP gateway provisioning. Mirrors
 * agent-manager-console, whose trial-info fetch doubles as activation.
 *
 * The read itself lives in `billing/organization`, which is the single owner of
 * that call: a cloud extension that also wants the billing record (the trial
 * badge) reaches the same memo through the Port, so the activation side effect
 * fires once rather than racing itself.
 *
 * Renders nothing. No-op when the BFF hasn't configured a billing upstream
 * (every standalone deployment today) — the read resolves null there.
 */
export function ProductActivation() {
  const { isAuthenticated } = useAuth();
  const activated = useRef(false);

  useEffect(() => {
    if (!isAuthenticated || activated.current) {
      return;
    }
    activated.current = true;
    void getBillingOrganization().catch(() => {
      // Best-effort: activation must not block the console. Allow a later retry.
      activated.current = false;
    });
  }, [isAuthenticated]);

  return null;
}
