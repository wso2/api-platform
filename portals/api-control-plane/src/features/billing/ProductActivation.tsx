import { useEffect, useRef } from 'react';

import { runtimeConfig } from '../../config/runtime';
import { useAuth } from '../auth/AuthProvider';

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
