import { useEffect, useRef } from 'react';

import { runtimeConfig } from '../../config/runtime';
import { useAuth } from '../auth/AuthProvider';

// Product code this console activates on first login. Must match the billing
// product whose subscription drives APIP gateway provisioning.
const PRODUCT = 'api-platform';

/**
 * Performs billing first-login activation once the user is authenticated.
 *
 * GET {billingServiceUrl}/organization?product=api-platform both reads the org's
 * subscription and, as a server-side side effect, activates the api-platform
 * subscription if it is currently inactive (first-login activation). That
 * activation emits subscription.activated, which triggers APIP gateway
 * provisioning. Mirrors agent-manager-console, whose trial-info fetch doubles
 * as activation.
 *
 * Renders nothing. No-op when billingServiceUrl is not configured.
 */
export function ProductActivation() {
  const { isAuthenticated, token } = useAuth();
  const activated = useRef(false);

  useEffect(() => {
    const billingUrl = runtimeConfig.billingServiceUrl;
    if (!billingUrl || !isAuthenticated || !token || activated.current) {
      return;
    }
    activated.current = true;
    void fetch(`${billingUrl}/organization?product=${PRODUCT}`, {
      headers: { Authorization: `Bearer ${token}` },
    }).catch(() => {
      // Best-effort: activation must not block the console. Allow a later retry.
      activated.current = false;
    });
  }, [isAuthenticated, token]);

  return null;
}
