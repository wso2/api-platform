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

import { runtimeConfig } from '../config/runtime';
import { useAuth } from '../contexts/auth/AuthProvider';

// Product code this console activates on first login. Must match the billing
// product whose subscription drives APIP gateway provisioning.
const PRODUCT = 'api-platform';

/**
 * Performs billing first-login activation once the user is authenticated.
 *
 * GET /proxy/billing/organization?product=api-platform both reads the org's
 * subscription and, as a server-side side effect, activates the api-platform
 * subscription if it is currently inactive (first-login activation). That
 * activation emits subscription.activated, which triggers APIP gateway
 * provisioning. Mirrors agent-manager-console, whose trial-info fetch doubles
 * as activation.
 *
 * Routed through the BFF's same-origin proxy (a "billing" named upstream) —
 * this component never sees a token or the real billing service URL.
 *
 * Renders nothing. No-op when the BFF hasn't configured a billing upstream
 * (every standalone deployment today).
 */
export function ProductActivation() {
  const { isAuthenticated } = useAuth();
  const activated = useRef(false);

  useEffect(() => {
    if (!runtimeConfig.billingProxyEnabled || !isAuthenticated || activated.current) {
      return;
    }
    activated.current = true;
    void fetch(`/proxy/billing/organization?product=${PRODUCT}`, {
      credentials: 'same-origin',
    }).catch(() => {
      // Best-effort: activation must not block the console. Allow a later retry.
      activated.current = false;
    });
  }, [isAuthenticated]);

  return null;
}
